//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/financial-service/internal/configuration"
	"example.com/financial-service/internal/domain"
	"example.com/financial-service/internal/infrastructure/messaging"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var client = &http.Client{Timeout: 25 * time.Second}
var endpoints = []string{"http://localhost:8080", "http://localhost:8082", "http://localhost:8083"}

func request(method, endpoint, token, key string, body any) (int, []byte, error) {
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	return response.StatusCode, data, err
}
func token(t *testing.T, id, secret string) string {
	t.Helper()
	response, err := client.PostForm("http://localhost:8081/realms/jungle/protocol/openid-connect/token", url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Token string `json:"access_token"`
	}
	err = json.NewDecoder(response.Body).Decode(&payload)
	if err != nil || response.StatusCode != 200 || payload.Token == "" {
		t.Fatalf("token acquisition failed: status %d, error %v", response.StatusCode, err)
	}
	return payload.Token
}
func waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		all := true
		for _, base := range endpoints {
			status, _, err := request("GET", base+"/health/ready", "", "", nil)
			if err != nil || status != 200 {
				all = false
			}
		}
		if all {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("three API instances are not ready; start docker compose --profile multi up --build -d")
}
func createWallet(t *testing.T, internal, amount string) domain.WalletView {
	t.Helper()
	player, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	status, raw, err := request("POST", endpoints[0]+"/wallets", internal, "", map[string]any{"playerId": player, "initialBalance": domain.MoneyInput{Amount: amount, Currency: "BRL"}})
	if err != nil || status != 201 {
		t.Fatalf("create wallet: %d %s %v", status, raw, err)
	}
	var wallet domain.WalletView
	err = json.Unmarshal(raw, &wallet)
	if err != nil {
		t.Fatal(err)
	}
	return wallet
}
func operation(wallet domain.WalletView, id, kind, amount string) domain.Operation {
	return domain.Operation{ProviderID: "provider-a", ExternalID: id, PlayerID: wallet.PlayerID, WalletID: wallet.ID, RoundID: "round-1", GameID: "game-1", Kind: kind, Money: domain.MoneyInput{Amount: amount, Currency: "BRL"}}
}
func send(t *testing.T, provider string, op domain.Operation, expected int) domain.Result {
	t.Helper()
	status, raw, err := request("POST", endpoints[0]+"/wagering/transactions", provider, op.ExternalID, op)
	if err != nil || status != expected {
		t.Fatalf("process %s: expected %d got %d %s %v", op.Kind, expected, status, raw, err)
	}
	var result domain.Result
	err = json.Unmarshal(raw, &result)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func walletBalance(t *testing.T, internal, id, expected string) {
	t.Helper()
	status, raw, err := request("GET", endpoints[1]+"/wallets/"+id, internal, "", nil)
	if err != nil || status != 200 {
		t.Fatalf("wallet query %d %s %v", status, raw, err)
	}
	var wallet domain.WalletView
	err = json.Unmarshal(raw, &wallet)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.Balance.Amount != expected {
		t.Fatalf("expected balance %s got %s", expected, wallet.Balance.Amount)
	}
}

type job struct {
	base, token string
	op          domain.Operation
	output      chan outcome
	wait        *sync.WaitGroup
}
type outcome struct {
	status int
	body   []byte
	err    error
}

func (work job) run() {
	defer work.wait.Done()
	status, raw, err := request("POST", work.base+"/wagering/transactions", work.token, work.op.ExternalID, work.op)
	work.output <- outcome{status, raw, err}
}
func parallel(provider string, operations []domain.Operation) []outcome {
	var wait sync.WaitGroup
	output := make(chan outcome, len(operations))
	for index, op := range operations {
		work := job{base: endpoints[index%len(endpoints)], token: provider, op: op, output: output, wait: &wait}
		wait.Add(1)
		go work.run()
	}
	wait.Wait()
	close(output)
	results := make([]outcome, 0, len(operations))
	for result := range output {
		results = append(results, result)
	}
	return results
}
func queryCount(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(), sql, args...).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}
func TestRealDistributedFinancialFlows(t *testing.T) {
	waitReady(t)
	internal := token(t, "internal-service", "local-internal-secret")
	provider := token(t, "provider-a", "local-provider-a-secret")
	otherProvider := token(t, "provider-b", "local-provider-b-secret")
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://financial:local_development_password@localhost:5432/financial?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	wallet := createWallet(t, internal, "100.00")
	// Authentication and authorization fail before any financial writes.
	for _, credential := range []string{"", "invalid-token", provider} {
		status, _, reqErr := request("POST", endpoints[0]+"/wallets", credential, "", map[string]any{"playerId": "unauthorized", "initialBalance": domain.MoneyInput{Amount: "10.00", Currency: "BRL"}})
		if reqErr != nil || (status != 401 && status != 403) {
			t.Fatalf("unauthorized wallet access: %d %v", status, reqErr)
		}
	}
	id, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	bet := operation(wallet, id, "BET", "25.00")
	operations := make([]domain.Operation, 50)
	for index := range operations {
		operations[index] = bet
	}
	results := parallel(provider, operations)
	for _, result := range results {
		if result.err != nil || result.status != 200 {
			t.Fatalf("duplicate request failed: %d %s %v", result.status, result.body, result.err)
		}
	}
	walletBalance(t, internal, wallet.ID, "75.00")
	if queryCount(t, pool, `SELECT count(*) FROM wallet_ledger WHERE wallet_id=$1`, wallet.ID) != 2 {
		t.Fatal("50 duplicate bets produced duplicate ledger entries")
	}
	status, _, err := request("POST", endpoints[0]+"/wagering/transactions", otherProvider, bet.ExternalID, bet)
	if err != nil || status != 403 {
		t.Fatal("provider impersonation was accepted")
	}
	replay := send(t, provider, bet, 200)
	status, _, err = request("GET", endpoints[0]+"/wagering/transactions/"+replay.TransactionID, otherProvider, "", nil)
	if err != nil || status != 404 {
		t.Fatal("another provider could read a transaction")
	}
	winID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	send(t, provider, operation(wallet, winID, "WIN", "10.00"), 200)
	replay = send(t, provider, bet, 200)
	if replay.Balance.Amount != "75.00" || !replay.IdempotentReplay {
		t.Fatal("replay did not preserve original balance")
	}
	conflicting := bet
	conflicting.Money.Amount = "26.00"
	send(t, provider, conflicting, 409)
	status, _, err = request("POST", endpoints[0]+"/wagering/transactions", provider, "another-key", bet)
	if err != nil || status != 409 {
		t.Fatal("same operation with another key was accepted")
	}
	// Two different 80.00 bets racing against 100.00 across independent processes.
	raceWallet := createWallet(t, internal, "100.00")
	firstID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	results = parallel(provider, []domain.Operation{operation(raceWallet, firstID, "BET", "80.00"), operation(raceWallet, secondID, "BET", "80.00")})
	accepted, rejected := 0, 0
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.status == 200 {
			accepted++
		} else if result.status == 422 {
			rejected++
		} else {
			t.Fatalf("race failure: %d %s", result.status, result.body)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("race accepted %d rejected %d", accepted, rejected)
	}
	walletBalance(t, internal, raceWallet.ID, "20.00")
	if queryCount(t, pool, `SELECT count(*) FROM wallet_ledger WHERE wallet_id=$1 AND direction='DEBIT'`, raceWallet.ID) != 1 {
		t.Fatal("race produced duplicate debit")
	}
	// Different wallets must also progress concurrently across distinct processes.
	independentA := createWallet(t, internal, "100.00")
	independentB := createWallet(t, internal, "100.00")
	aID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	bID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	for _, outcome := range parallel(provider, []domain.Operation{operation(independentA, aID, "BET", "30.00"), operation(independentB, bID, "BET", "30.00")}) {
		if outcome.err != nil || outcome.status != 200 {
			t.Fatalf("independent wallets failed: %d %s %v", outcome.status, outcome.body, outcome.err)
		}
	}
	walletBalance(t, internal, independentA.ID, "70.00")
	walletBalance(t, internal, independentB.ID, "70.00")
	// LOSS does not alter version or ledger.
	lossID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	send(t, provider, operation(raceWallet, lossID, "LOSS", "0.00"), 200)
	if queryCount(t, pool, `SELECT version::integer FROM wallets WHERE id=$1`, raceWallet.ID) != 2 {
		t.Fatal("LOSS changed wallet version")
	}
	// Reference arrives later and is resumed durably by a worker.
	pendingWallet := createWallet(t, internal, "100.00")
	referenceID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	refundID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	refund := operation(pendingWallet, refundID, "REFUND", "20.00")
	refund.Reference = referenceID
	pending := send(t, provider, refund, 202)
	send(t, provider, operation(pendingWallet, referenceID, "BET", "20.00"), 200)
	deadline := time.Now().Add(30 * time.Second)
	completed := false
	for time.Now().Before(deadline) {
		status, raw, queryErr := request("GET", endpoints[2]+"/wagering/transactions/"+pending.TransactionID, provider, "", nil)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		var result domain.Result
		if status == 200 {
			err = json.Unmarshal(raw, &result)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status == "PROCESSED" {
				completed = true
				break
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !completed {
		t.Fatal("pending reference was not resumed")
	}
	walletBalance(t, internal, pendingWallet.ID, "100.00")
	rollbackID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	rollback := operation(pendingWallet, rollbackID, "ROLLBACK", "20.00")
	rollback.Reference = referenceID
	duplicateReversal := send(t, provider, rollback, 422)
	if duplicateReversal.FailureCode != "ALREADY_REVERSED" {
		t.Fatal("double reversal was not rejected")
	}
	// A refund itself can be rolled back, restoring the original debit.
	refundRollbackID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	refundRollback := operation(pendingWallet, refundRollbackID, "ROLLBACK", "20.00")
	refundRollback.Reference = refundID
	send(t, provider, refundRollback, 200)
	walletBalance(t, internal, pendingWallet.ID, "80.00")
	// Reversing a spent WIN must use a distinct insufficient-balance code.
	reversalWallet := createWallet(t, internal, "10.00")
	winToReverseID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	send(t, provider, operation(reversalWallet, winToReverseID, "WIN", "20.00"), 200)
	spendID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	send(t, provider, operation(reversalWallet, spendID, "BET", "25.00"), 200)
	reverseSpentID, idErr := domain.NewID()
	if idErr != nil {
		t.Fatal(idErr)
	}
	reverseSpent := operation(reversalWallet, reverseSpentID, "ROLLBACK", "20.00")
	reverseSpent.Reference = winToReverseID
	rejectedReversal := send(t, provider, reverseSpent, 422)
	if rejectedReversal.FailureCode != "REVERSAL_INSUFFICIENT_BALANCE" {
		t.Fatal("reversal used incorrect failure code")
	}
	walletBalance(t, internal, reversalWallet.ID, "5.00")
	// Database constraints and immutable ledger, using a real PostgreSQL connection.
	_, err = pool.Exec(context.Background(), `UPDATE wallets SET balance=-1 WHERE id=$1`, wallet.ID)
	if err == nil {
		t.Fatal("database accepted negative balance")
	}
	_, err = pool.Exec(context.Background(), `UPDATE wallets SET balance=balance+1 WHERE id=$1`, wallet.ID)
	if err == nil {
		t.Fatal("database accepted balance without ledger")
	}
	_, err = pool.Exec(context.Background(), `UPDATE wallet_ledger SET amount=amount WHERE wallet_id=$1`, wallet.ID)
	if err == nil {
		t.Fatal("ledger UPDATE accepted")
	}
	_, err = pool.Exec(context.Background(), `DELETE FROM wallet_ledger WHERE wallet_id=$1`, wallet.ID)
	if err == nil {
		t.Fatal("ledger DELETE accepted")
	}
	status, raw, err := request("POST", endpoints[0]+"/wallets/"+wallet.ID+"/reconciliation", internal, "", nil)
	if err != nil || status != 200 || !bytes.Contains(raw, []byte(`"consistent":true`)) {
		t.Fatalf("reconciliation failed: %d %s %v", status, raw, err)
	}
	// At-least-once messages: different FIFO dedup IDs exercise application inbox.
	queues := messaging.NewClient(configuration.Config{AWSRegion: "us-east-1", AWSKey: "test", AWSSecret: "test", SQSEndpoint: "http://localhost:4566"})
	err = queues.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sqsWallet := createWallet(t, internal, "100.00")
	sqsID, err := domain.NewID()
	if err != nil {
		t.Fatal(err)
	}
	sqsOp := operation(sqsWallet, sqsID, "BET", "25.00")
	data, err := json.Marshal(sqsOp)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	err = json.Unmarshal(data, &body)
	if err != nil {
		t.Fatal(err)
	}
	body["idempotencyKey"] = sqsID
	envelope := map[string]any{"messageId": sqsID, "type": "WagerTransactionRequested", "occurredAt": time.Now().UTC(), "accessToken": provider, "data": body}
	raw, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		_, err = queues.API.SendMessage(context.Background(), &sqs.SendMessageInput{QueueUrl: aws.String(queues.QueueURL), MessageBody: aws.String(string(raw)), MessageGroupId: aws.String(sqsWallet.ID), MessageDeduplicationId: aws.String(fmt.Sprintf("%s-%d", sqsID, index))})
		if err != nil {
			t.Fatal(err)
		}
	}
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if queryCount(t, pool, `SELECT count(*) FROM inbox WHERE message_id=$1 AND completed_at IS NOT NULL`, sqsID) == 1 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	walletBalance(t, internal, sqsWallet.ID, "75.00")
	if queryCount(t, pool, `SELECT count(*) FROM wallet_ledger WHERE wallet_id=$1`, sqsWallet.ID) != 2 {
		t.Fatal("SQS duplicates moved money twice")
	}
	send(t, provider, sqsOp, 200)
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if queryCount(t, pool, `SELECT count(*) FROM outbox WHERE published_at IS NULL`) == 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if queryCount(t, pool, `SELECT count(*) FROM outbox WHERE published_at IS NULL`) != 0 {
		t.Fatal("outbox events were not published")
	}
	status, raw, err = request("GET", endpoints[0]+"/metrics", internal, "", nil)
	if err != nil || status != 200 || !strings.Contains(string(raw), "outbox_pending_events") || !strings.Contains(string(raw), "sqs_dlq_visible_messages") {
		t.Fatalf("metrics unavailable: %d %s %v", status, raw, err)
	}
	t.Log("real PostgreSQL, Keycloak, SQS and three independent API/worker instances verified")
}

func TestInvalidInputDoesNotUseFloatingPoint(t *testing.T) {
	waitReady(t)
	provider := token(t, "provider-a", "local-provider-a-secret")
	for _, amount := range []string{"NaN", "Infinity", "1e3", "-1.00", "1.001"} {
		op := domain.Operation{ProviderID: "provider-a", ExternalID: "invalid-" + strings.ReplaceAll(amount, ".", "-"), PlayerID: "player", WalletID: "wallet", RoundID: "round", GameID: "game", Kind: "BET", Money: domain.MoneyInput{Amount: amount, Currency: "BRL"}}
		status, _, err := request("POST", endpoints[0]+"/wagering/transactions", provider, op.ExternalID, op)
		if err != nil || status != 400 {
			t.Fatalf("invalid decimal %s: status %d error %v", amount, status, err)
		}
	}
}
