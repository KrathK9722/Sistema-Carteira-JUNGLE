package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewWalletWithValidData(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	wallet, err := NewWallet(
		"wallet-123",
		"player-456",
		initialBalance,
		creationTime,
	)

	if err != nil {
		t.Fatalf("não foi possível criar a carteira: %v", err)
	}

	if wallet.ID() != "wallet-123" {
		t.Errorf("identificador inesperado: %s", wallet.ID())
	}

	if wallet.PlayerID() != "player-456" {
		t.Errorf("jogador inesperado: %s", wallet.PlayerID())
	}

	if wallet.Balance().MinorUnits() != 10000 {
		t.Errorf(
			"esperava saldo de 10000 centavos, recebeu %d",
			wallet.Balance().MinorUnits(),
		)
	}

	if wallet.Currency() != "BRL" {
		t.Errorf("esperava moeda BRL, recebeu %s", wallet.Currency())
	}

	if wallet.Version() != 1 {
		t.Errorf("esperava versão inicial 1, recebeu %d", wallet.Version())
	}

	if !wallet.CreateDate().Equal(creationTime) {
		t.Error("a data de criação está incorreta")
	}

	if !wallet.UpdateDate().Equal(creationTime) {
		t.Error("a data de atualização deveria ser igual à de criação")
	}
}

func TestNewWalletWithEmptyID(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err = NewWallet(
		"",
		"player-456",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidWalletID) {
		t.Fatalf(
			"esperava ErrInvalidWalletID, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de carteira sem identificador foi rejeitada.")
}

func TestNewWalletWithPlayerID(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err = NewWallet(
		"wallet-473",
		"",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidPlayerID) {
		t.Fatalf(
			"esperava ErrInvalidPlayerID, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de carteira sem identificador foi rejeitada.")
}

func TestNewWalletWithInitBalanceZero(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := ZeroMoney("BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo zero: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	wallet, err := NewWallet(
		"wallet-473",
		"player-586",
		initialBalance,
		creationTime,
	)

	if err != nil {
		t.Fatalf("saldo inicial zero deveria ser aceito: %v", err)
	}

	if wallet.Balance().MinorUnits() != 0 {
		t.Fatalf(
			"esperava saldo zero, mas recebeu %d centavos",
			wallet.Balance().MinorUnits(),
		)
	}

	if wallet.Version() != 1 {
		t.Fatalf(
			"esperava versão inicial 1, mas recebeu %d",
			wallet.Version(),
		)
	}

	t.Log("A carteira foi criada com saldo zero e versão 1.")
}

func TestNewWalletWithNegativeInitialBalance(t *testing.T) {
	registerTestSummary(t)

	initialBalance := Money{
		minorUnits: -100,
		currency:   "BRL",
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err := NewWallet(
		"wallet-473",
		"player-586",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrNegativeInitialBalance) {
		t.Fatalf(
			"esperava ErrNegativeInitialBalance, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de carteira com saldo inicial negativo foi rejeitada.")
}

func TestNewWalletWithUninitializedMoney(t *testing.T) {
	registerTestSummary(t)

	var initialBalance Money

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err := NewWallet(
		"wallet-473",
		"player-586",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de carteira com Money não inicializado foi rejeitada.")
}

func TestNewWalletWithUninitializedTime(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	var creationTime time.Time

	_, err = NewWallet(
		"wallet-473",
		"player-586",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidWalletTime) {
		t.Fatalf(
			"esperava ErrInvalidWalletTime, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de carteira sem data válida foi rejeitada.")
}

func TestNewWalletWithSpacesAroundID(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err = NewWallet(
		" wallet-473 ",
		"player-586",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidWalletID) {
		t.Fatalf(
			"esperava ErrInvalidWalletID, mas recebeu: %v",
			err,
		)
	}

	t.Log("O identificador da carteira com espaços nas extremidades foi rejeitado.")
}

func TestNewWalletWithSpacesAroundPlayerID(t *testing.T) {
	registerTestSummary(t)

	initialBalance, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o saldo inicial: %v", err)
	}

	creationTime := time.Date(
		2026,
		time.October,
		23,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	_, err = NewWallet(
		"wallet-473",
		" player-586 ",
		initialBalance,
		creationTime,
	)

	if !errors.Is(err, ErrInvalidPlayerID) {
		t.Fatalf(
			"esperava ErrInvalidPlayerID, mas recebeu: %v",
			err,
		)
	}

	t.Log("O identificador do jogador com espaços nas extremidades foi rejeitado.")
}
