//go:build integration

package integration

import (
	"context"
	"errors"
	"example.com/financial-service/internal/configuration"
	"example.com/financial-service/internal/domain"
	"example.com/financial-service/internal/infrastructure/messaging"
	"example.com/financial-service/internal/infrastructure/postgres"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os/exec"
	"testing"
	"time"
)

func compose(arguments ...string) error {
	base := []string{"compose", "--project-directory", "../..", "--profile", "multi"}
	command := exec.Command("docker", append(base, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose %v: %s: %w", arguments, output, err)
	}
	return nil
}
func restoreServices(t *testing.T) {
	t.Helper()
	_ = compose("unpause", "api", "api2", "api3")
	if err := compose("up", "-d", "api", "api2", "api3"); err != nil {
		t.Error(err)
	}
}

type interruptedPublisher struct {
	queues      *messaging.Client
	cancel      context.CancelFunc
	id, payload string
}

func (publisher *interruptedPublisher) Publish(ctx context.Context, id, aggregate, payload string) error {
	err := publisher.queues.Publish(ctx, id, aggregate, payload)
	if err != nil {
		return err
	}
	publisher.id = id
	publisher.payload = payload
	publisher.cancel()
	return context.Canceled
}

func TestRecoveryOfPendingReferencesAndOutbox(t *testing.T) {
	waitReady(t)
	internal := token(t, "internal-service", "local-internal-secret")
	provider := token(t, "provider-a", "local-provider-a-secret")
	wallet := createWallet(t, internal, "100.00")
	pool, err := pgxpool.New(context.Background(), "postgres://financial:local_development_password@localhost:5432/financial?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := postgres.NewRepository(pool)
	queues := messaging.NewClient(configuration.Config{AWSRegion: "us-east-1", AWSKey: "test", AWSSecret: "test", SQSEndpoint: "http://localhost:4566"})
	err = queues.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = compose("pause", "api", "api2", "api3")
	if err != nil {
		t.Fatal(err)
	}
	defer restoreServices(t)
	// No API process can publish the newly committed outbox while paused.
	balance, err := domain.NewMoney("1.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	player, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.CreateWallet(context.Background(), player, balance)
	if err != nil {
		t.Fatal(err)
	}
	publishContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	publisher := &interruptedPublisher{queues: queues, cancel: cancel}
	_, err = repository.PublishOne(publishContext, publisher)
	cancel()
	if err == nil || publisher.id == "" {
		t.Fatalf("failed to simulate interruption after real SQS publication: %v", err)
	}
	var storedPayload string
	var published *time.Time
	err = pool.QueryRow(context.Background(), `SELECT payload::text,published_at FROM outbox WHERE event_id=$1`, publisher.id).Scan(&storedPayload, &published)
	if err != nil {
		t.Fatal(err)
	}
	if published != nil {
		t.Fatal("interrupted publication was acknowledged in database")
	}
	// Retry the same durable row; its event ID and snapshot must remain stable.
	for index := 0; index < 100; index++ {
		_, err = repository.PublishOne(context.Background(), queues)
		if err != nil {
			t.Fatal(err)
		}
		if queryCount(t, pool, `SELECT count(*) FROM outbox WHERE event_id=$1 AND published_at IS NOT NULL`, publisher.id) == 1 {
			break
		}
	}
	if queryCount(t, pool, `SELECT count(*) FROM outbox WHERE event_id=$1 AND published_at IS NOT NULL`, publisher.id) != 1 {
		t.Fatal("interrupted event was not recovered")
	}
	var replayPayload string
	err = pool.QueryRow(context.Background(), `SELECT payload::text FROM outbox WHERE event_id=$1`, publisher.id).Scan(&replayPayload)
	if err != nil {
		t.Fatal(err)
	}
	if storedPayload != replayPayload || storedPayload != publisher.payload {
		t.Fatal("outbox retry changed event snapshot")
	}
	originalID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	refundID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	refund := operation(wallet, refundID, "REFUND", "20.00")
	refund.Reference = originalID
	refund.IdempotencyKey = refundID
	err = refund.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	pending, err := repository.Process(context.Background(), refund, "recovery", nil)
	if err != nil || pending.Status != "PENDING_REFERENCE" {
		t.Fatalf("pending reference not committed: %+v %v", pending, err)
	}
	bet := operation(wallet, originalID, "BET", "20.00")
	bet.IdempotencyKey = originalID
	err = bet.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.Process(context.Background(), bet, "recovery", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Kill every process after the database commits, before workers can resume.
	err = compose("kill", "-s", "SIGKILL", "api", "api2", "api3")
	if err != nil {
		t.Fatal(err)
	}
	err = compose("up", "-d", "api", "api2", "api3")
	if err != nil {
		t.Fatal(err)
	}
	waitReady(t)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		result, queryErr := repository.Transaction(context.Background(), "provider-a", pending.TransactionID, "")
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if result.Status == "PROCESSED" {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	result, err := repository.Transaction(context.Background(), "provider-a", pending.TransactionID, "")
	if err != nil || result.Status != "PROCESSED" {
		t.Fatalf("pending reference did not survive process termination: %+v %v", result, err)
	}
	walletBalance(t, internal, wallet.ID, "100.00")
	replay := send(t, provider, bet, 200)
	if !replay.IdempotentReplay || replay.Balance.Amount != "80.00" {
		t.Fatal("persistent replay lost its original result after restart")
	}
	_, err = pool.Exec(context.Background(), `DELETE FROM wager_transactions WHERE id=$1`, replay.TransactionID)
	if err == nil {
		t.Fatal("terminal financial transaction deletion was accepted")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("unexpected canceled database context")
	}
	t.Log("commit-before-publication, publication-before-ack, durable references and idempotency survived recovery")
}
