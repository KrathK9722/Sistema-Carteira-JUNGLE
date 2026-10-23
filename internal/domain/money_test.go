package domain

import (
	"errors"
	"testing"
)

func TestNewMoneyWithValidAmount(t *testing.T) {
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
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
	registerTestSummary(t)
	_, err := ZeroMoney("INVALID")

	if !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf(
			"esperava ErrInvalidCurrency, mas recebeu: %v",
			err,
		)
	}

	t.Log("A criação de zero com moeda inválida foi rejeitada corretamente.")
}

func TestMoneyAddWithValidAmounts(t *testing.T) {
	registerTestSummary(t)
	firstMoney, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Add(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível somar os valores: %v", err)
	}

	if result.MinorUnits() != 3550 {
		t.Fatalf(
			"esperava $35.50 BRL, mas recebeu: $%d.%02d BRL",
			result.MinorUnits()/100,
			result.MinorUnits()%100,
		)
	}

	if result.Currency() != "BRL" {
		t.Fatalf(
			"esperava moeda BRL, mas recebeu: %s",
			result.Currency(),
		)
	}

	if firstMoney.MinorUnits() != 2550 {
		t.Fatal("a soma alterou o primeiro valor original")
	}

	if secondMoney.MinorUnits() != 1000 {
		t.Fatal("a soma alterou o segundo valor original")
	}

	t.Log("A soma retornou $35.50 BRL sem alterar os valores originais.")
}

func TestMoneyAddWithZero(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor: %v", err)
	}

	zero, err := ZeroMoney("BRL")

	if err != nil {
		t.Fatalf("não foi possível criar zero: %v", err)
	}

	result, err := money.Add(zero)

	if err != nil {
		t.Fatalf("não foi possível somar com zero: %v", err)
	}

	if result.MinorUnits() != 2550 {
		t.Fatalf(
			"esperava $25.50 BRL, mas recebeu: $%d.%02d BRL",
			result.MinorUnits()/100,
			result.MinorUnits()%100,
		)
	}

	t.Log("A soma com zero preservou o valor corretamente.")
}

func TestMoneyAddWithUninitializedReceiver(t *testing.T) {
	registerTestSummary(t)
	var uninitializedMoney Money

	validMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor válido: %v", err)
	}

	_, err = uninitializedMoney.Add(validMoney)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A soma rejeitou o primeiro valor não inicializado.")
}

func TestMoneyAddWithUninitializedArgument(t *testing.T) {
	registerTestSummary(t)
	validMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor válido: %v", err)
	}

	var uninitializedMoney Money

	_, err = validMoney.Add(uninitializedMoney)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A soma rejeitou o segundo valor não inicializado.")
}

func TestMoneyAddWithAmountAboveLimit(t *testing.T) {
	registerTestSummary(t)
	maximumMoney, err := NewMoney("92233720368547758.07", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor máximo: %v", err)
	}

	oneCent, err := NewMoney("0.01", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar um centavo: %v", err)
	}

	_, err = maximumMoney.Add(oneCent)

	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf(
			"esperava ErrAmountOutOfRange, mas recebeu: %v",
			err,
		)
	}

	t.Log("A soma que ultrapassaria o limite de int64 foi rejeitada.")
}

func TestMoneySubtractWithValidAmounts(t *testing.T) {
	registerTestSummary(t)

	firstMoney, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Subtract(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível subtrair os valores: %v", err)
	}

	if result.MinorUnits() != 1550 {
		t.Fatalf(
			"esperava $15.50 BRL centavos, mas recebeu: $%d.%02d BRL",
			result.MinorUnits()/100,
			result.MinorUnits()%100,
		)
	}

	if result.Currency() != "BRL" {
		t.Fatalf(
			"esperava moeda BRL, mas recebeu: %s",
			result.Currency(),
		)
	}

	if firstMoney.MinorUnits() != 2550 {
		t.Fatal("a subtração alterou o primeiro valor original")
	}

	if secondMoney.MinorUnits() != 1000 {
		t.Fatal("a subtração alterou o segundo valor original")
	}

	t.Log("A subtração retornou $15.50 BRL sem alterar os valores originais.")
}

func TestMoneySubtractWithNegativeResult(t *testing.T) {
	registerTestSummary(t)

	firstMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Subtract(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível subtrair os valores: %v", err)
	}

	if result.MinorUnits() != -1550 {
		t.Fatalf(
			"esperava $-15.50 BRL, mas recebeu: $%d.%02d BRL",
			result.MinorUnits()/100,
			result.MinorUnits()%100,
		)
	}

	err = result.Validate()

	if err != nil {
		t.Fatalf("o resultado negativo deveria ser válido: %v", err)
	}

	t.Log("A subtração produziu um resultado negativo válido para cálculo interno.")
}

func TestMoneySubtractWithUninitializedArgument(t *testing.T) {
	registerTestSummary(t)

	money, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor: %v", err)
	}

	var uninitializedMoney Money

	_, err = money.Subtract(uninitializedMoney)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A subtração rejeitou o valor não inicializado.")
}
