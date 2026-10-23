package domain

import (
	"errors"
	"testing"
)

func TestNewMoneyWithValidAmount(t *testing.T) {
	money, err := NewMoney("25.50", "BRL")
	reais := money.MinorUnits() / 100
	centavos := money.MinorUnits() % 100

	if err != nil {
		t.Fatalf("não esperava erro, mas recebeu: %v", err)
	}

	if money.MinorUnits() != 2550 {
		t.Errorf(
			"esperava 2550 centavos, mas recebeu: %d",
			money.MinorUnits(),
		)
	}

	if money.Currency() != "BRL" {
		t.Errorf(
			"esperava moeda BRL, mas recebeu: %s",
			money.Currency(),
		)
	}

	if !t.Failed() {
		t.Logf(
			"O valor de $%d,%02d %s foram registrados corretamente.",
			reais,
			centavos,
			money.Currency(),
		)
	}
}

func TestNewMoneyWithZeroAmount(t *testing.T) {
	money, err := NewMoney("0.00", "BRL")

	if err != nil {
		t.Fatalf("zero deveria ser aceito, mas recebeu erro: %v", err)
	}

	if money.MinorUnits() != 0 {
		t.Errorf(
			"esperava zero centavos, mas recebeu: %d",
			money.MinorUnits(),
		)
	} else {
		t.Log("O valor zero foi aceito corretamente.")
	}
}

func TestNewMoneyWithNegativeAmount(t *testing.T) {
	_, err := NewMoney("-10.00", "BRL")

	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf(
			"esperava ErrInvalidAmount, mas recebeu: %v",
			err,
		)
	} else {
		t.Log("O valor negativo foi rejeitado corretamente.")
	}
}

func TestNewMoneyWithTooManyDecimalPlaces(t *testing.T) {
	_, err := NewMoney("10.001", "BRL")

	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf(
			"esperava ErrInvalidAmount, mas recebeu: %v",
			err,
		)
	} else {
		t.Log("O valor com mais de duas casas decimais foi rejeitado corretamente.")
	}
}

func TestNewMoneyWithInvalidCurrency(t *testing.T) {
	_, err := NewMoney("10.00", "INVALID")

	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf(
			"esperava ErrInvalidCurrency, mas recebeu: %v",
			err,
		)
	} else {
		t.Log("A moeda inválida foi rejeitada corretamente.")
	}
}

func TestNewMoneyWithAmountAboveLimit(t *testing.T) {
	_, err := NewMoney("92233720368547758.08", "BRL")

	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf(
			"esperava ErrAmountOutOfRange, mas recebeu: %v",
			err,
		)
	} else {
		t.Log("O valor acima do limite de representação em int64 foi rejeitado corretamente.")
	}
}

func TestMoneyValidateWithUninitializedValue(t *testing.T) {
	var money Money

	err := money.Validate()

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("O dinheiro não inicializado foi rejeitado corretamente.")
}

func TestMoneyValidateWithValidValue(t *testing.T) {
	money, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf(
			"não foi possível criar o dinheiro: %v",
			err,
		)
	}

	err = money.Validate()

	if err != nil {
		t.Fatalf(
			"o dinheiro deveria ser válido, mas recebeu erro: %v",
			err,
		)
	}

	t.Log("O dinheiro foi validado corretamente.")
}

func TestZeroMoneyWithValidCurrency(t *testing.T) {
	money, err := ZeroMoney("BRL")

	if err != nil {
		t.Fatalf(
			"não foi possível criar o dinheiro com valor zero: %v",
			err,
		)
	}

	if money.MinorUnits() != 0 {
		t.Fatalf(
			"esperava zero centavos, mas recebeu: %d",
			money.MinorUnits(),
		)
	}

	if money.Currency() != "BRL" {
		t.Fatalf(
			"esperava moeda BRL, mas recebeu: %s",
			money.Currency(),
		)
	}

	err = money.Validate()

	if err != nil {
		t.Fatalf(
			"zero em BRL deveria ser válido, mas recebeu erro: %v",
			err,
		)
	}

	t.Log("O dinheiro com valor zero foi criado e validado corretamente.")
}

func TestZeroMoneyWithInvalidCurrency(t *testing.T) {
	_, err := ZeroMoney("INVALID")

	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf(
			"esperava ErrInvalidCurrency, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de zero com moeda inválida foi rejeitada corretamente.")
}
