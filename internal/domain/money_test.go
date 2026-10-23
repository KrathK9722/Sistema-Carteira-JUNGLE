package domain

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestNewMoneyWithValidAmount(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("25.50", "BRL")

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
			"O valor de %d centavos na moeda %s foi registrado corretamente.",
			money.MinorUnits(),
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
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
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
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
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
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
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
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
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

func TestMoneyNegateWithPositiveAmount(t *testing.T) {
	registerTestSummary(t)

	money, err := NewMoney("25.50", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor: %v", err)
	}

	result, err := money.Negate()

	if err != nil {
		t.Fatalf("não foi possível inverter o sinal: %v", err)
	}

	if result.MinorUnits() != -2550 {
		t.Fatalf(
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
		)
	}

	if result.Currency() != "BRL" {
		t.Fatalf(
			"esperava moeda BRL, mas recebeu: %s",
			result.Currency(),
		)
	}

	if money.MinorUnits() != 2550 {
		t.Fatal("a negação alterou o valor original")
	}

	t.Log("O valor positivo foi convertido em negativo sem alterar o original.")
}

func TestMoneyNegateWithNegativeAmount(t *testing.T) {
	registerTestSummary(t)

	money := Money{
		minorUnits: -2550,
		currency:   "BRL",
	}

	result, err := money.Negate()

	if err != nil {
		t.Fatalf("não foi possível inverter o sinal: %v", err)
	}

	if result.MinorUnits() != 2550 {
		t.Fatalf(
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
		)
	}

	if money.MinorUnits() != -2550 {
		t.Fatal("a negação alterou o valor original")
	}

	t.Log("O valor negativo foi convertido em positivo.")
}

func TestMoneyNegateWithZero(t *testing.T) {
	registerTestSummary(t)

	money, err := ZeroMoney("BRL")

	if err != nil {
		t.Fatalf("não foi possível criar zero: %v", err)
	}

	result, err := money.Negate()

	if err != nil {
		t.Fatalf("não foi possível inverter o sinal de zero: %v", err)
	}

	if result.MinorUnits() != 0 {
		t.Fatalf(
			"resultado inesperado: recebeu %d centavos",
			result.MinorUnits(),
		)
	}

	t.Log("A negação de zero retornou zero.")
}

func TestMoneyNegateWithUninitializedValue(t *testing.T) {
	registerTestSummary(t)

	var money Money

	_, err := money.Negate()

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A negação rejeitou o valor não inicializado.")
}

func TestMoneyNegateWithMinimumInt64(t *testing.T) {
	registerTestSummary(t)

	money := Money{
		minorUnits: math.MinInt64,
		currency:   "BRL",
	}

	_, err := money.Negate()

	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf(
			"esperava ErrAmountOutOfRange, mas recebeu: %v",
			err,
		)
	}

	t.Log("A negação do menor int64 foi rejeitada para evitar overflow.")
}

func TestMoneyCompareWithSmallerAmount(t *testing.T) {
	registerTestSummary(t)

	firstMoney, err := NewMoney("10.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Compare(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível comparar os valores: %v", err)
	}

	if result != -1 {
		t.Fatalf("esperava -1, mas recebeu: %d", result)
	}

	t.Log("A comparação identificou que $10.00 BRL é menor que $25.00 BRL.")
}

func TestMoneyCompareWithEqualAmounts(t *testing.T) {
	registerTestSummary(t)

	firstMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Compare(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível comparar os valores: %v", err)
	}

	if result != 0 {
		t.Fatalf("esperava 0, mas recebeu: %d", result)
	}

	t.Log("A comparação identificou valores iguais.")
}

func TestMoneyCompareWithLargerAmount(t *testing.T) {
	registerTestSummary(t)

	firstMoney, err := NewMoney("100.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o primeiro valor: %v", err)
	}

	secondMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o segundo valor: %v", err)
	}

	result, err := firstMoney.Compare(secondMoney)

	if err != nil {
		t.Fatalf("não foi possível comparar os valores: %v", err)
	}

	if result != 1 {
		t.Fatalf("esperava 1, mas recebeu: %d", result)
	}

	t.Log("A comparação identificou que $100.00 BRL é maior que $25.00 BRL.")
}

func TestMoneyCompareWithUninitializedReceiver(t *testing.T) {
	registerTestSummary(t)

	var uninitializedMoney Money

	validMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor válido: %v", err)
	}

	_, err = uninitializedMoney.Compare(validMoney)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A comparação rejeitou o primeiro valor não inicializado.")
}

func TestMoneyCompareWithUninitializedArgument(t *testing.T) {
	registerTestSummary(t)

	validMoney, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor válido: %v", err)
	}

	var uninitializedMoney Money

	_, err = validMoney.Compare(uninitializedMoney)

	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf(
			"esperava ErrInvalidMoney, mas recebeu: %v",
			err,
		)
	}

	t.Log("A comparação rejeitou o segundo valor não inicializado.")
}

func TestNewMoneyWithUSD(t *testing.T) {
	registerTestSummary(t)

	money, err := NewMoney("25.50", "USD")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em USD: %v", err)
	}

	if money.MinorUnits() != 2550 {
		t.Fatalf(
			"esperava 2550 centavos, mas recebeu: %d",
			money.MinorUnits(),
		)
	}

	if money.Currency() != "USD" {
		t.Fatalf(
			"esperava moeda USD, mas recebeu: %s",
			money.Currency(),
		)
	}

	err = money.Validate()

	if err != nil {
		t.Fatalf("o valor em USD deveria ser válido: %v", err)
	}

	t.Log("O valor em USD foi criado e validado corretamente.")
}

func TestMoneyAddWithDifferentCurrencies(t *testing.T) {
	registerTestSummary(t)

	moneyBRL, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em BRL: %v", err)
	}

	moneyUSD, err := NewMoney("10.00", "USD")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em USD: %v", err)
	}

	_, err = moneyBRL.Add(moneyUSD)

	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf(
			"esperava ErrCurrencyMismatch, mas recebeu: %v",
			err,
		)
	}

	t.Log("A soma entre BRL e USD foi rejeitada corretamente.")
}

func TestMoneySubtractWithDifferentCurrencies(t *testing.T) {
	registerTestSummary(t)

	moneyBRL, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em BRL: %v", err)
	}

	moneyUSD, err := NewMoney("10.00", "USD")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em USD: %v", err)
	}

	_, err = moneyBRL.Subtract(moneyUSD)

	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf(
			"esperava ErrCurrencyMismatch, mas recebeu: %v",
			err,
		)
	}

	t.Log("A subtração entre BRL e USD foi rejeitada corretamente.")
}

func TestMoneyCompareWithDifferentCurrencies(t *testing.T) {
	registerTestSummary(t)

	moneyBRL, err := NewMoney("25.00", "BRL")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em BRL: %v", err)
	}

	moneyUSD, err := NewMoney("25.00", "USD")

	if err != nil {
		t.Fatalf("não foi possível criar o valor em USD: %v", err)
	}

	_, err = moneyBRL.Compare(moneyUSD)

	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf(
			"esperava ErrCurrencyMismatch, mas recebeu: %v",
			err,
		)
	}

	t.Log("A comparação entre BRL e USD foi rejeitada corretamente.")
}

func TestMoneyAmountWithPositiveValue(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("25.50", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "25.50" {
		t.Fatalf("esperava 25.50, recebeu %s", amount)
	}
}

func TestMoneyAmountWithNegativeCent(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: -1, currency: "BRL"}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "-0.01" {
		t.Fatalf("esperava -0.01, recebeu %s", amount)
	}
}

func TestMoneyAmountWithZero(t *testing.T) {
	registerTestSummary(t)
	money, err := ZeroMoney("BRL")
	if err != nil {
		t.Fatal(err)
	}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "0.00" {
		t.Fatalf("esperava 0.00, recebeu %s", amount)
	}
}

func TestMoneyAmountWithMinimumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MinInt64, currency: "BRL"}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "-92233720368547758.08" {
		t.Fatalf("formatação incorreta do limite negativo: %s", amount)
	}
}

func TestMoneyAmountWithMaximumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MaxInt64, currency: "BRL"}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "92233720368547758.07" {
		t.Fatalf("formatação incorreta do limite positivo: %s", amount)
	}
}

func TestMoneyAmountWithUninitializedValue(t *testing.T) {
	registerTestSummary(t)
	var money Money
	_, err := money.Amount()
	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("esperava ErrInvalidMoney, recebeu %v", err)
	}
}

func TestMoneyMarshalJSON(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("25.50", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(money)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"amount":"25.50","currency":"BRL"}`
	if string(data) != expected {
		t.Fatalf("esperava %s, recebeu %s", expected, string(data))
	}
}

func TestMoneyMarshalJSONWithNegativeValue(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: -1550, currency: "BRL"}
	data, err := json.Marshal(money)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"amount":"-15.50","currency":"BRL"}`
	if string(data) != expected {
		t.Fatalf("esperava %s, recebeu %s", expected, string(data))
	}
}

func TestMoneyMarshalJSONWithUninitializedValue(t *testing.T) {
	registerTestSummary(t)
	var money Money
	_, err := json.Marshal(money)
	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("esperava ErrInvalidMoney, recebeu %v", err)
	}
}

func TestNewMoneyNormalizesLeadingZeros(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("00025.50", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	amount, err := money.Amount()
	if err != nil {
		t.Fatal(err)
	}
	if amount != "25.50" {
		t.Fatalf("esperava normalização para 25.50, recebeu %s", amount)
	}
}

func TestNewMoneyWithOneCent(t *testing.T) {
	registerTestSummary(t)
	money, err := NewMoney("0.01", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if money.MinorUnits() != 1 {
		t.Fatalf("esperava 1 centavo, recebeu %d", money.MinorUnits())
	}
}

func TestMoneyAddBelowMinimumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MinInt64, currency: "BRL"}
	negativeCent := Money{minorUnits: -1, currency: "BRL"}
	_, err := money.Add(negativeCent)
	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf("esperava ErrAmountOutOfRange, recebeu %v", err)
	}
}

func TestMoneyAddExactlyAtMaximumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MaxInt64 - 1, currency: "BRL"}
	oneCent := Money{minorUnits: 1, currency: "BRL"}
	result, err := money.Add(oneCent)
	if err != nil {
		t.Fatal(err)
	}
	if result.MinorUnits() != math.MaxInt64 {
		t.Fatalf("esperava o limite positivo, recebeu %d", result.MinorUnits())
	}
}

func TestMoneyAddExactlyAtMinimumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MinInt64 + 1, currency: "BRL"}
	negativeCent := Money{minorUnits: -1, currency: "BRL"}
	result, err := money.Add(negativeCent)
	if err != nil {
		t.Fatal(err)
	}
	if result.MinorUnits() != math.MinInt64 {
		t.Fatalf("esperava o limite negativo, recebeu %d", result.MinorUnits())
	}
}

func TestMoneySubtractBelowMinimumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MinInt64, currency: "BRL"}
	oneCent := Money{minorUnits: 1, currency: "BRL"}
	_, err := money.Subtract(oneCent)
	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf("esperava ErrAmountOutOfRange, recebeu %v", err)
	}
}

func TestMoneySubtractAboveMaximumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MaxInt64, currency: "BRL"}
	negativeCent := Money{minorUnits: -1, currency: "BRL"}
	_, err := money.Subtract(negativeCent)
	if !errors.Is(err, ErrAmountOutOfRange) {
		t.Fatalf("esperava ErrAmountOutOfRange, recebeu %v", err)
	}
}

func TestMoneySubtractExactlyAtMinimumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MinInt64 + 1, currency: "BRL"}
	oneCent := Money{minorUnits: 1, currency: "BRL"}
	result, err := money.Subtract(oneCent)
	if err != nil {
		t.Fatal(err)
	}
	if result.MinorUnits() != math.MinInt64 {
		t.Fatalf("esperava o limite negativo, recebeu %d", result.MinorUnits())
	}
}

func TestMoneySubtractExactlyAtMaximumInt64(t *testing.T) {
	registerTestSummary(t)
	money := Money{minorUnits: math.MaxInt64 - 1, currency: "BRL"}
	negativeCent := Money{minorUnits: -1, currency: "BRL"}
	result, err := money.Subtract(negativeCent)
	if err != nil {
		t.Fatal(err)
	}
	if result.MinorUnits() != math.MaxInt64 {
		t.Fatalf("esperava o limite positivo, recebeu %d", result.MinorUnits())
	}
}

func TestMoneySubtractWithUninitializedReceiver(t *testing.T) {
	registerTestSummary(t)
	var money Money
	validMoney, err := NewMoney("1.00", "BRL")
	if err != nil {
		t.Fatal(err)
	}
	_, err = money.Subtract(validMoney)
	if !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("esperava ErrInvalidMoney, recebeu %v", err)
	}
}

func TestNewMoneyRejectsEmptyAmount(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsNaN(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("NaN", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsInfinity(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("Infinity", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsScientificNotation(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("1e3", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsLeadingSpace(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney(" 10.00", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsPositiveSign(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("+10.00", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsCommaSeparator(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("10,00", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}

func TestNewMoneyRejectsSingleDecimalPlace(t *testing.T) {
	registerTestSummary(t)
	_, err := NewMoney("10.0", "BRL")
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperava ErrInvalidAmount, recebeu %v", err)
	}
}
