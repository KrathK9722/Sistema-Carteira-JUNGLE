package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"example.com/financial-service/internal/application"
	"example.com/financial-service/internal/configuration"
	"example.com/financial-service/internal/domain"
	"example.com/financial-service/internal/infrastructure/authentication"
	"example.com/financial-service/internal/infrastructure/messaging"
	"example.com/financial-service/internal/infrastructure/postgres"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

type Runner struct {
	repository      *postgres.Repository
	service         *application.Service
	queues          *messaging.Client
	auth            *authentication.Validator
	cancel          context.CancelFunc
	wait            sync.WaitGroup
	done            chan struct{}
	providerSenders map[string]string
}

func NewRunner(repository *postgres.Repository, service *application.Service, queues *messaging.Client, auth *authentication.Validator, config configuration.Config) *Runner {
	return &Runner{repository: repository, service: service, queues: queues, auth: auth, done: make(chan struct{}), providerSenders: config.ProviderSenders}
}
func (r *Runner) Start(ctx context.Context) error {
	background, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wait.Add(3)
	go r.consume(background)
	go r.publish(background)
	go r.references(background)
	go r.await()
	return nil
}
func (r *Runner) await() { r.wait.Wait(); close(r.done) }
func (r *Runner) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func waitTick(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func (r *Runner) publish(ctx context.Context) {
	defer r.wait.Done()
	for ctx.Err() == nil {
		workContext, cancel := context.WithTimeout(ctx, 15*time.Second)
		work, err := r.repository.PublishOne(workContext, r.queues)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			r.service.RecordRetry("outbox")
			slog.Warn("outbox retry", "error", err)
		}
		if !work || err != nil {
			if !waitTick(ctx, time.Second) {
				return
			}
		}
	}
}
func (r *Runner) references(ctx context.Context) {
	defer r.wait.Done()
	for ctx.Err() == nil {
		workContext, cancel := context.WithTimeout(ctx, 15*time.Second)
		work, err := r.repository.ResumeOne(workContext)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			r.service.RecordRetry("reference")
			slog.Warn("reference retry", "error", err)
		}
		if !work || err != nil {
			if !waitTick(ctx, time.Second) {
				return
			}
		}
	}
}

type envelope struct {
	MessageID   string    `json:"messageId"`
	Type        string    `json:"type"`
	OccurredAt  time.Time `json:"occurredAt"`
	AccessToken string    `json:"accessToken"`
	Data        struct {
		domain.Operation
		IdempotencyKey string `json:"idempotencyKey"`
	} `json:"data"`
}

func (r *Runner) handle(ctx context.Context, message types.Message) error {
	body := aws.ToString(message.Body)
	if len(body) > 65536 {
		return domain.ErrInvalidOperation
	}
	var input envelope
	err := json.Unmarshal([]byte(body), &input)
	if err != nil {
		return domain.ErrInvalidOperation
	}
	if input.MessageID == "" || len(input.MessageID) > 256 || input.Type != "WagerTransactionRequested" || input.OccurredAt.IsZero() {
		return domain.ErrInvalidOperation
	}
	provider := r.providerSenders[message.Attributes["SenderId"]]
	if input.AccessToken != "" {
		claims, verifyErr := r.auth.Verify(ctx, input.AccessToken)
		if verifyErr != nil || claims.Access != "provider" {
			return domain.ErrForbidden
		}
		provider = claims.ProviderID
	}
	if provider == "" || provider != input.Data.ProviderID {
		return domain.ErrForbidden
	}
	input.Data.Operation.IdempotencyKey = input.Data.IdempotencyKey
	// Transport credentials are not persisted or included in the inbox hash.
	input.AccessToken = ""
	canonical, err := json.Marshal(input)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	inbox := application.InboxMessage{Consumer: "wager-transactions-v1", ID: input.MessageID, Hash: hex.EncodeToString(hash[:])}
	result, err := r.service.Process(ctx, input.Data.Operation, input.MessageID, &inbox)
	if err != nil {
		return err
	}
	slog.Info("message committed", "messageId", input.MessageID, "transactionId", result.TransactionID, "providerId", provider, "walletId", input.Data.WalletID, "status", result.Status)
	return nil
}
func (r *Runner) consume(ctx context.Context) {
	defer r.wait.Done()
	for ctx.Err() == nil {
		batch, err := r.queues.API.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{QueueUrl: aws.String(r.queues.QueueURL), MaxNumberOfMessages: 1, WaitTimeSeconds: 10, VisibilityTimeout: 60, MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount, types.MessageSystemAttributeNameSenderId}})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("sqs receive retry", "error", err)
			if !waitTick(ctx, time.Second) {
				return
			}
			continue
		}
		for _, message := range batch.Messages {
			workCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err = r.handle(workCtx, message)
			cancel()
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if err == nil {
				_, err = r.queues.API.DeleteMessage(cleanupCtx, &sqs.DeleteMessageInput{QueueUrl: aws.String(r.queues.QueueURL), ReceiptHandle: message.ReceiptHandle})
				if err != nil {
					slog.Warn("sqs delete retry", "error", err)
				}
			} else {
				r.service.RecordRetry("sqs")
				attempt, _ := strconv.Atoi(message.Attributes["ApproximateReceiveCount"])
				if attempt < 1 {
					attempt = 1
				}
				if attempt > 5 {
					attempt = 5
				}
				visibility := int32(1 << attempt)
				if ctx.Err() != nil {
					visibility = 0
				}
				_, releaseErr := r.queues.API.ChangeMessageVisibility(cleanupCtx, &sqs.ChangeMessageVisibilityInput{QueueUrl: aws.String(r.queues.QueueURL), ReceiptHandle: message.ReceiptHandle, VisibilityTimeout: visibility})
				slog.Warn("message not acknowledged", "error", err, "visibilityError", releaseErr)
			}
			cleanupCancel()
		}
	}
}
