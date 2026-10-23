package domain

import "time"

type TransactionEventData struct {
	TransactionID string     `json:"transactionId"`
	WalletID      string     `json:"walletId"`
	Status        string     `json:"status"`
	FailureCode   string     `json:"failureCode,omitempty"`
	Money         MoneyInput `json:"money"`
}
type BalanceEventData struct {
	WalletID      string     `json:"walletId"`
	TransactionID string     `json:"transactionId"`
	Direction     string     `json:"direction"`
	Money         MoneyInput `json:"money"`
	BalanceBefore MoneyInput `json:"balanceBefore"`
	BalanceAfter  MoneyInput `json:"balanceAfter"`
	WalletVersion int64      `json:"walletVersion"`
}
type Event[T any] struct {
	EventID       string    `json:"eventId"`
	EventType     string    `json:"eventType"`
	AggregateID   string    `json:"aggregateId"`
	CorrelationID string    `json:"correlationId"`
	CausationID   string    `json:"causationId,omitempty"`
	OccurredAt    time.Time `json:"occurredAt"`
	Version       int       `json:"version"`
	Data          T         `json:"data"`
}

func NewTransactionEvent(kind, wallet, correlation string, data TransactionEventData) (Event[TransactionEventData], error) {
	switch kind {
	case "WagerTransactionProcessed", "WagerTransactionRejected", "WagerTransactionPendingReference":
	default:
		return Event[TransactionEventData]{}, ErrInvalidOperation
	}
	id, err := NewID()
	return Event[TransactionEventData]{EventID: id, EventType: kind, AggregateID: wallet, CorrelationID: correlation, OccurredAt: time.Now().UTC(), Version: 1, Data: data}, err
}
func NewBalanceEvent(wallet, correlation string, data BalanceEventData) (Event[BalanceEventData], error) {
	id, err := NewID()
	return Event[BalanceEventData]{EventID: id, EventType: "WalletBalanceChanged", AggregateID: wallet, CorrelationID: correlation, OccurredAt: time.Now().UTC(), Version: 1, Data: data}, err
}
