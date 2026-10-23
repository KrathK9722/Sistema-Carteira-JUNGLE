package domain

import (
	"math"
	"strings"
	"time"
)

type Wallet struct {
	id         string
	playerID   string
	balance    Money
	version    int64
	createDate time.Time
	updateDate time.Time
}

// Criação de Carteira com versão inicial 1
// Identificador e hora são registrados ao chamar a função
func NewWallet(
	id string,
	playerID string,
	initialBalance Money,
	createDate time.Time,
) (Wallet, error) {
	if id == "" {
		return Wallet{}, ErrInvalidWalletID
	}

	if strings.TrimSpace(id) != id {
		return Wallet{}, ErrInvalidWalletID
	}

	if playerID == "" {
		return Wallet{}, ErrInvalidPlayerID
	}

	if strings.TrimSpace(playerID) != playerID {
		return Wallet{}, ErrInvalidPlayerID
	}

	err := initialBalance.Validate()

	if err != nil {
		return Wallet{}, err
	}

	if initialBalance.MinorUnits() < 0 {
		return Wallet{}, ErrNegativeInitialBalance
	}

	if createDate.IsZero() {
		return Wallet{}, ErrInvalidWalletTime
	}

	creationTime := createDate.UTC()

	wallet := Wallet{
		id:         id,
		playerID:   playerID,
		balance:    initialBalance,
		version:    1,
		createDate: creationTime,
		updateDate: creationTime,
	}

	return wallet, nil
}

func (wallet Wallet) ID() string {
	return wallet.id
}

func (wallet Wallet) PlayerID() string {
	return wallet.playerID
}

func (wallet Wallet) Balance() Money {
	return wallet.balance
}

func (wallet Wallet) Currency() string {
	return wallet.balance.Currency()
}

func (wallet Wallet) Version() int64 {
	return wallet.version
}

func (wallet Wallet) CreateDate() time.Time {
	return wallet.createDate
}

func (wallet Wallet) UpdateDate() time.Time {
	return wallet.updateDate
}

// Validate verifica se a carteira possui um estado válido.
func (wallet Wallet) Validate() error {
	if wallet.id == "" {
		return ErrInvalidWallet
	}

	if strings.TrimSpace(wallet.id) != wallet.id {
		return ErrInvalidWallet
	}

	if wallet.playerID == "" {
		return ErrInvalidWallet
	}

	if strings.TrimSpace(wallet.playerID) != wallet.playerID {
		return ErrInvalidWallet
	}

	err := wallet.balance.Validate()

	if err != nil {
		return ErrInvalidWallet
	}

	if wallet.balance.MinorUnits() < 0 {
		return ErrInvalidWallet
	}

	if wallet.version < 1 {
		return ErrInvalidWallet
	}

	if wallet.createDate.IsZero() {
		return ErrInvalidWallet
	}

	if wallet.updateDate.IsZero() {
		return ErrInvalidWallet
	}

	if wallet.updateDate.Before(wallet.createDate) {
		return ErrInvalidWallet
	}

	return nil
}

// Adiciona dinheiro ao saldo.
// Crédito zero não altera saldo, versão ou data.
// Qualquer erro preserva o estado anterior da carteira.
func (wallet *Wallet) Credit(amount Money, updatedAt time.Time) error {
	if wallet == nil {
		return ErrInvalidWallet
	}

	err := wallet.Validate()

	if err != nil {
		return err
	}

	err = amount.Validate()

	if err != nil {
		return err
	}

	if amount.MinorUnits() < 0 {
		return ErrNegativeCredit
	}

	if amount.Currency() != wallet.Currency() {
		return ErrCurrencyMismatch
	}

	if updatedAt.IsZero() {
		return ErrInvalidWalletTime
	}

	if updatedAt.Before(wallet.updateDate) {
		return ErrInvalidWalletTime
	}

	if amount.MinorUnits() == 0 {
		return nil
	}

	if wallet.version == math.MaxInt64 {
		return ErrWalletVersionOverflow
	}

	newBalance, err := wallet.balance.Add(amount)

	if err != nil {
		return err
	}

	wallet.balance = newBalance
	wallet.version = wallet.version + 1
	wallet.updateDate = updatedAt.UTC()

	return nil
}
