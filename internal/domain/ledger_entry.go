package domain

import "time"

// WalletLedgerEntry is an immutable financial snapshot.
type WalletLedgerEntry struct {
	id, walletID, transactionID, direction string
	money, balanceBefore, balanceAfter     Money
	createdAt                              time.Time
}

func NewWalletLedgerEntry(id, walletID, transactionID, direction string, money, before, after Money, created time.Time) (WalletLedgerEntry, error) {
	if id == "" || walletID == "" || transactionID == "" || created.IsZero() {
		return WalletLedgerEntry{}, ErrInvalidOperation
	}
	err := money.Validate()
	if err != nil {
		return WalletLedgerEntry{}, err
	}
	err = before.Validate()
	if err != nil {
		return WalletLedgerEntry{}, err
	}
	err = after.Validate()
	if err != nil {
		return WalletLedgerEntry{}, err
	}
	if money.MinorUnits() <= 0 || before.MinorUnits() < 0 || after.MinorUnits() < 0 {
		return WalletLedgerEntry{}, ErrInvalidAmount
	}
	var expected Money
	switch direction {
	case "CREDIT":
		expected, err = before.Add(money)
	case "DEBIT":
		expected, err = before.Subtract(money)
	default:
		return WalletLedgerEntry{}, ErrInvalidOperation
	}
	if err != nil {
		return WalletLedgerEntry{}, err
	}
	comparison, err := expected.Compare(after)
	if err != nil {
		return WalletLedgerEntry{}, err
	}
	if comparison != 0 {
		return WalletLedgerEntry{}, ErrInvalidAmount
	}
	return WalletLedgerEntry{id: id, walletID: walletID, transactionID: transactionID, direction: direction, money: money, balanceBefore: before, balanceAfter: after, createdAt: created.UTC()}, nil
}
func (entry WalletLedgerEntry) ID() string            { return entry.id }
func (entry WalletLedgerEntry) WalletID() string      { return entry.walletID }
func (entry WalletLedgerEntry) TransactionID() string { return entry.transactionID }
func (entry WalletLedgerEntry) Direction() string     { return entry.direction }
func (entry WalletLedgerEntry) Money() Money          { return entry.money }
func (entry WalletLedgerEntry) BalanceBefore() Money  { return entry.balanceBefore }
func (entry WalletLedgerEntry) BalanceAfter() Money   { return entry.balanceAfter }
func (entry WalletLedgerEntry) CreatedAt() time.Time  { return entry.createdAt }
