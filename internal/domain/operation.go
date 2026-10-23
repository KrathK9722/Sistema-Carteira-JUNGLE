package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrConflict = errors.New("conflict")
var ErrNotFound = errors.New("not_found")
var ErrForbidden = errors.New("forbidden")
var ErrInvalidOperation = errors.New("invalid_operation")

// MoneyInput preserves decimal text at every external boundary.
type MoneyInput struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
type Operation struct {
	ProviderID     string     `json:"providerId"`
	ExternalID     string     `json:"externalTransactionId"`
	IdempotencyKey string     `json:"-"`
	PlayerID       string     `json:"playerId"`
	WalletID       string     `json:"walletId"`
	RoundID        string     `json:"roundId"`
	GameID         string     `json:"gameId"`
	Kind           string     `json:"kind"`
	Money          MoneyInput `json:"money"`
	Reference      string     `json:"referenceExternalTransactionId,omitempty"`
}

func (operation *Operation) Normalize() error {
	fields := []string{operation.ProviderID, operation.ExternalID, operation.IdempotencyKey, operation.PlayerID, operation.WalletID, operation.RoundID, operation.GameID}
	for _, value := range fields {
		if value == "" || strings.TrimSpace(value) != value || len(value) > 256 {
			return ErrInvalidOperation
		}
	}
	if strings.TrimSpace(operation.Reference) != operation.Reference || len(operation.Reference) > 256 {
		return ErrInvalidOperation
	}
	money, err := NewMoney(operation.Money.Amount, operation.Money.Currency)
	if err != nil {
		return err
	}
	switch operation.Kind {
	case "BET", "WIN", "REFUND", "ROLLBACK":
		if money.MinorUnits() <= 0 {
			return ErrInvalidAmount
		}
	case "LOSS":
		if money.MinorUnits() != 0 {
			return ErrInvalidAmount
		}
	default:
		return ErrInvalidOperation
	}
	if (operation.Kind == "REFUND" || operation.Kind == "ROLLBACK") && operation.Reference == "" {
		return ErrInvalidOperation
	}
	if (operation.Kind == "BET" || operation.Kind == "LOSS") && operation.Reference != "" {
		return ErrInvalidOperation
	}
	operation.Money.Amount, err = money.Amount()
	return err
}

// PayloadHash uses an ordered JSON object. All its values are strings or ordered
// objects with string values; encoding/json sorts map keys deterministically.
func (operation Operation) PayloadHash() (string, error) {
	data := map[string]any{"providerId": operation.ProviderID, "externalTransactionId": operation.ExternalID, "playerId": operation.PlayerID, "walletId": operation.WalletID, "roundId": operation.RoundID, "gameId": operation.GameID, "kind": operation.Kind, "money": map[string]string{"amount": operation.Money.Amount, "currency": operation.Money.Currency}, "referenceExternalTransactionId": operation.Reference}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func NewID() (string, error) {
	var value [16]byte
	_, err := rand.Read(value[:])
	if err != nil {
		return "", err
	}
	value[6] = (value[6] & 15) | 64
	value[8] = (value[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
func RehydrateWallet(id, player string, balance Money, version int64, created, updated time.Time) (Wallet, error) {
	wallet := Wallet{id: id, playerID: player, balance: balance, version: version, createDate: created.UTC(), updateDate: updated.UTC()}
	err := wallet.Validate()
	return wallet, err
}

type Result struct {
	TransactionID    string     `json:"transactionId"`
	Status           string     `json:"status"`
	Balance          MoneyInput `json:"balance"`
	FailureCode      string     `json:"failureCode,omitempty"`
	IdempotentReplay bool       `json:"idempotentReplay"`
}
type WalletView struct {
	ID       string     `json:"id"`
	PlayerID string     `json:"playerId"`
	Balance  MoneyInput `json:"balance"`
	Version  int64      `json:"version"`
}

func View(wallet Wallet) (WalletView, error) {
	amount, err := wallet.Balance().Amount()
	return WalletView{wallet.ID(), wallet.PlayerID(), MoneyInput{amount, wallet.Currency()}, wallet.Version()}, err
}

// WagerTransaction validates terminal transitions independently of transport.
type WagerTransaction struct{ status string }

func NewWagerTransaction() WagerTransaction { return WagerTransaction{status: "PENDING"} }
func RestoreWagerTransaction(status string) (WagerTransaction, error) {
	switch status {
	case "PENDING", "PENDING_REFERENCE", "PROCESSED", "REJECTED", "FAILED":
		return WagerTransaction{status: status}, nil
	}
	return WagerTransaction{}, ErrInvalidOperation
}
func (transaction WagerTransaction) Status() string { return transaction.status }
func (transaction *WagerTransaction) Transition(next string) error {
	if transaction == nil || (transaction.status != "PENDING" && transaction.status != "PENDING_REFERENCE") {
		return ErrInvalidOperation
	}
	switch next {
	case "PENDING_REFERENCE", "PROCESSED", "REJECTED", "FAILED":
		transaction.status = next
		return nil
	}
	return ErrInvalidOperation
}

// Movement validates reference semantics and determines the financial direction.
func Movement(operation Operation, reference *Operation, referenceStatus string) (string, string) {
	if operation.Reference != "" {
		if reference == nil || referenceStatus == "PENDING" || referenceStatus == "PENDING_REFERENCE" {
			return "", "REFERENCE_PENDING"
		}
		if referenceStatus != "PROCESSED" {
			return "", "REFERENCE_NOT_PROCESSED"
		}
		if operation.ProviderID != reference.ProviderID || operation.PlayerID != reference.PlayerID || operation.WalletID != reference.WalletID || operation.RoundID != reference.RoundID || operation.Money.Currency != reference.Money.Currency {
			return "", "REFERENCE_MISMATCH"
		}
		if operation.Kind == "WIN" {
			if reference.Kind != "BET" {
				return "", "INVALID_REFERENCE_KIND"
			}
		}
		if operation.Kind == "REFUND" || operation.Kind == "ROLLBACK" {
			if operation.Money.Amount != reference.Money.Amount {
				return "", "REFERENCE_AMOUNT_MISMATCH"
			}
			if operation.Kind == "REFUND" && reference.Kind != "BET" {
				return "", "INVALID_REFERENCE_KIND"
			}
			if operation.Kind == "ROLLBACK" && reference.Kind != "BET" && reference.Kind != "WIN" && reference.Kind != "REFUND" {
				return "", "INVALID_REFERENCE_KIND"
			}
		}
	}
	switch operation.Kind {
	case "BET":
		return "DEBIT", ""
	case "WIN", "REFUND":
		return "CREDIT", ""
	case "LOSS":
		return "", ""
	case "ROLLBACK":
		if reference != nil && reference.Kind == "BET" {
			return "CREDIT", ""
		}
		return "DEBIT", ""
	}
	return "", "INVALID_KIND"
}
