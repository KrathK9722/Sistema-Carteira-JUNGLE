package application

import (
	"context"
	"example.com/financial-service/internal/domain"
	"fmt"
	"sync/atomic"
	"time"
)

type InboxMessage struct{ Consumer, ID, Hash string }
type Store interface {
	CreateWallet(context.Context, string, domain.Money) (domain.WalletView, error)
	Wallet(context.Context, string) (domain.WalletView, error)
	Process(context.Context, domain.Operation, string, *InboxMessage) (domain.Result, error)
	Transaction(context.Context, string, string, string) (domain.Result, error)
	Ledger(context.Context, string, string, int) (any, error)
	Reconcile(context.Context, string) (any, error)
	Ready(context.Context) error
	Metrics(context.Context) (string, error)
}
type Service struct {
	store                                                                                                      Store
	requests, elapsedNanos, replays, processed, rejected, pending, sqsRetries, outboxRetries, referenceRetries atomic.Uint64
}

func NewService(store Store) *Service { return &Service{store: store} }
func (service *Service) CreateWallet(ctx context.Context, player string, input domain.MoneyInput) (domain.WalletView, error) {
	money, err := domain.NewMoney(input.Amount, input.Currency)
	if err != nil {
		return domain.WalletView{}, err
	}
	return service.store.CreateWallet(ctx, player, money)
}
func (service *Service) Wallet(ctx context.Context, id string) (domain.WalletView, error) {
	return service.store.Wallet(ctx, id)
}
func (service *Service) Process(ctx context.Context, operation domain.Operation, correlation string, inbox *InboxMessage) (domain.Result, error) {
	started := time.Now()
	defer service.observe(started)
	err := operation.Normalize()
	if err != nil {
		return domain.Result{}, err
	}
	result, err := service.store.Process(ctx, operation, correlation, inbox)
	if err == nil {
		if result.IdempotentReplay {
			service.replays.Add(1)
		}
		switch result.Status {
		case "PROCESSED":
			service.processed.Add(1)
		case "REJECTED":
			service.rejected.Add(1)
		case "PENDING_REFERENCE":
			service.pending.Add(1)
		}
	}
	return result, err
}
func (service *Service) observe(started time.Time) {
	service.requests.Add(1)
	service.elapsedNanos.Add(uint64(time.Since(started).Nanoseconds()))
}
func (service *Service) RecordRetry(source string) {
	switch source {
	case "sqs":
		service.sqsRetries.Add(1)
	case "outbox":
		service.outboxRetries.Add(1)
	case "reference":
		service.referenceRetries.Add(1)
	}
}
func (service *Service) Metrics(ctx context.Context) (string, error) {
	persisted, err := service.store.Metrics(ctx)
	if err != nil {
		return "", err
	}
	elapsed := service.elapsedNanos.Load()
	return persisted + fmt.Sprintf("wager_results_total{status=\"PROCESSED\"} %d\nwager_results_total{status=\"REJECTED\"} %d\nwager_results_total{status=\"PENDING_REFERENCE\"} %d\nwager_duplicates_total %d\nwager_retries_total{source=\"sqs\"} %d\nwager_retries_total{source=\"outbox\"} %d\nwager_retries_total{source=\"reference\"} %d\nwager_processing_duration_seconds_count %d\nwager_processing_duration_seconds_sum %d.%09d\n", service.processed.Load(), service.rejected.Load(), service.pending.Load(), service.replays.Load(), service.sqsRetries.Load(), service.outboxRetries.Load(), service.referenceRetries.Load(), service.requests.Load(), elapsed/1000000000, elapsed%1000000000), nil
}
func (service *Service) Transaction(ctx context.Context, provider, id, external string) (domain.Result, error) {
	return service.store.Transaction(ctx, provider, id, external)
}
func (service *Service) Ledger(ctx context.Context, id, cursor string, limit int) (any, error) {
	return service.store.Ledger(ctx, id, cursor, limit)
}
func (service *Service) Reconcile(ctx context.Context, id string) (any, error) {
	return service.store.Reconcile(ctx, id)
}
func (service *Service) Ready(ctx context.Context) error { return service.store.Ready(ctx) }
