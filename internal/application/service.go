package application

import (
	"context"
	"example.com/financial-service/internal/domain"
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
}
type Service struct{ store Store }

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
	err := operation.Normalize()
	if err != nil {
		return domain.Result{}, err
	}
	return service.store.Process(ctx, operation, correlation, inbox)
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
