package domain

import (
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
