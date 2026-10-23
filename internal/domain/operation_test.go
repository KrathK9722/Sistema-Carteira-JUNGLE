package domain

import (
	"errors"
	"testing"
	"time"
)

func sampleOperation() Operation {
	return Operation{ProviderID: "provider-a", ExternalID: "bet-1", IdempotencyKey: "key-1", PlayerID: "player-1", WalletID: "wallet-1", RoundID: "round-1", GameID: "game-1", Kind: "BET", Money: MoneyInput{Amount: "25.00", Currency: "BRL"}}
}
func TestOperationHashNormalization(t *testing.T) {
	registerTestSummary(t)
	first := sampleOperation()
	second := sampleOperation()
	second.Money.Amount = "00025.00"
	second.IdempotencyKey = "another-key"
	err := first.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	err = second.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	a, err := first.PayloadHash()
	if err != nil {
		t.Fatal(err)
	}
	b, err := second.PayloadHash()
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("equivalent business payloads must have the same hash")
	}
	second.Money.Amount = "26.00"
	b, err = second.PayloadHash()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("different business payloads must have different hashes")
	}
}
func TestExternalOpeningRejected(t *testing.T) {
	registerTestSummary(t)
	op := sampleOperation()
	op.Kind = "OPENING"
	if !errors.Is(op.Normalize(), ErrInvalidOperation) {
		t.Fatal("external OPENING accepted")
	}
}
func TestOperationZeroPolicy(t *testing.T) {
	registerTestSummary(t)
	for _, kind := range []string{"BET", "WIN", "REFUND", "ROLLBACK"} {
		op := sampleOperation()
		op.Kind = kind
		op.Reference = "original"
		op.Money.Amount = "0.00"
		if !errors.Is(op.Normalize(), ErrInvalidAmount) {
			t.Fatalf("zero accepted for %s", kind)
		}
	}
	op := sampleOperation()
	op.Kind = "LOSS"
	op.Money.Amount = "0.00"
	if err := op.Normalize(); err != nil {
		t.Fatal(err)
	}
}
func TestTerminalTransactionCannotTransition(t *testing.T) {
	registerTestSummary(t)
	for _, status := range []string{"PROCESSED", "REJECTED", "FAILED"} {
		transaction := NewWagerTransaction()
		if err := transaction.Transition(status); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(transaction.Transition("PROCESSED"), ErrInvalidOperation) {
			t.Fatalf("terminal %s accepted a transition", status)
		}
	}
}
func TestReferenceRules(t *testing.T) {
	registerTestSummary(t)
	original := sampleOperation()
	refund := original
	refund.Kind = "REFUND"
	refund.Reference = original.ExternalID
	direction, failure := Movement(refund, &original, "PROCESSED")
	if direction != "CREDIT" || failure != "" {
		t.Fatal("valid refund rejected")
	}
	original.RoundID = "another-round"
	_, failure = Movement(refund, &original, "PROCESSED")
	if failure != "REFERENCE_MISMATCH" {
		t.Fatal("cross-round reference accepted")
	}
	_, failure = Movement(refund, nil, "")
	if failure != "REFERENCE_PENDING" {
		t.Fatal("missing reference must become pending")
	}
}
func TestLedgerEntryRejectsWrongBalance(t *testing.T) {
	registerTestSummary(t)
	amount, err := NewMoney("25.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	before, err := NewMoney("100.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	after, err := NewMoney("90.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewWalletLedgerEntry("entry", "wallet", "transaction", "DEBIT", amount, before, after, time.Now())
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatal("inconsistent ledger balance accepted")
	}
}
func TestWalletRehydrationPreservesState(t *testing.T) {
	registerTestSummary(t)
	balance, err := NewMoney("75.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)
	wallet, err := RehydrateWallet("wallet", "player", balance, 7, created, updated)
	if err != nil {
		t.Fatal(err)
	}
	if wallet.Version() != 7 || wallet.Balance().MinorUnits() != 7500 || !wallet.UpdateDate().Equal(updated) {
		t.Fatal("rehydration changed persisted financial state")
	}
}
