package domain

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// Notas pessoais:
// Os campos são privados porque começam com letra minúscula.
// Money representa um valor monetário.
// minorUnits guarda o valor em centavos.
// currency guarda a moeda.

type Money struct {
	minorUnits int64
	currency   string
}

// NewMoney implementação e criação de um dinheiro:
//   - as moedas aceitas são BRL e USD;
//   - zeros à esquerda são aceitos e normalizados por Amount;
//   - o valor precisa ter exatamente duas casas decimais;
//   - valores negativos não são aceitos neste construtor;
//   - espaços, sinais e notação científica são rejeitados.
//
// Exemplos válidos: "0.00", "10.50" e "1000.00".
func NewMoney(amount string, currency string) (Money, error) {
	if !isSupportedCurrency(currency) {
		return Money{}, ErrInvalidCurrency
	}

	parts := strings.Split(amount, ".")

	if len(parts) != 2 {
		return Money{}, ErrInvalidAmount
	}

	wholePart := parts[0]
	fractionPart := parts[1]

	if len(wholePart) == 0 {
		return Money{}, ErrInvalidAmount
	}

	if len(fractionPart) != 2 {
		return Money{}, ErrInvalidAmount
	}

	if !containsOnlyDigits(wholePart) {
		return Money{}, ErrInvalidAmount
	}

	if !containsOnlyDigits(fractionPart) {
		return Money{}, ErrInvalidAmount
	}

	// Variavel para juntar reais e centavos sem passar por float.
	// "25" + "50" = "2550"
	minorUnitsText := wholePart + fractionPart

	minorUnits, err := strconv.ParseInt(minorUnitsText, 10, 64)

	if err != nil {
		return Money{}, ErrAmountOutOfRange
	}

	money := Money{
		minorUnits: minorUnits,
		currency:   currency,
	}

	return money, nil
}

// containsOnlyDigits verifica se todos os caracteres são de 0 a 9.
func containsOnlyDigits(value string) bool {
	if len(value) == 0 {
		return false
	}

	for index := 0; index < len(value); index++ { //PEGAR CARACTERE DO VALOR MONETÁRIO
		character := value[index]

		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}

// MinorUnits retorna o valor em centavos.
func (money Money) MinorUnits() int64 {
	return money.minorUnits
}

// Currency retorna a moeda.
func (money Money) Currency() string {
	return money.currency
}

// Validate verifica se Money possui uma moeda aceita.
// Valores negativos são permitidos em cálculos internos.
func (money Money) Validate() error {
	if !isSupportedCurrency(money.currency) {
		return ErrInvalidMoney
	}

	return nil
}

// Cria um valor zero para a moeda informada.
func ZeroMoney(currency string) (Money, error) {
	money, err := NewMoney("0.00", currency)

	if err != nil {
		return Money{}, err
	}

	return money, nil
}

// Os valores originais não são alterados.
// O resultado é retornado em um novo Money.
func (money Money) Add(other Money) (Money, error) {
	err := money.Validate()

	if err != nil {
		return Money{}, err
	}

	err = other.Validate()

	if err != nil {
		return Money{}, err
	}

	if money.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}

	if other.minorUnits > 0 {
		if money.minorUnits > math.MaxInt64-other.minorUnits {
			return Money{}, ErrAmountOutOfRange
		}
	}

	if other.minorUnits < 0 {
		if money.minorUnits < math.MinInt64-other.minorUnits {
			return Money{}, ErrAmountOutOfRange
		}
	}

	total := money.minorUnits + other.minorUnits

	result := Money{
		minorUnits: total,
		currency:   money.currency,
	}

	return result, nil
}

// Subtrai outro valor monetário sem alterar os valores originais.
func (money Money) Subtract(other Money) (Money, error) {
	err := money.Validate()

	if err != nil {
		return Money{}, err
	}

	err = other.Validate()

	if err != nil {
		return Money{}, err
	}

	if money.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}

	// Subtrair um valor positivo pode ultrapassar o limite negativo.
	if other.minorUnits > 0 {
		if money.minorUnits < math.MinInt64+other.minorUnits {
			return Money{}, ErrAmountOutOfRange
		}
	}

	// Subtrair um valor negativo pode ultrapassar o limite positivo.
	if other.minorUnits < 0 {
		if money.minorUnits > math.MaxInt64+other.minorUnits {
			return Money{}, ErrAmountOutOfRange
		}
	}

	difference := money.minorUnits - other.minorUnits

	result := Money{
		minorUnits: difference,
		currency:   money.currency,
	}

	return result, nil
}

// Inverte o sinal sem alterar o valor original.
func (money Money) Negate() (Money, error) {
	err := money.Validate()

	if err != nil {
		return Money{}, err
	}

	if money.minorUnits == math.MinInt64 {
		return Money{}, ErrAmountOutOfRange
	}

	result := Money{
		minorUnits: -money.minorUnits,
		currency:   money.currency,
	}

	return result, nil
}

// Compara dois valores monetários da mesma moeda.
// Retorna:
// -1 money menor que other.
// 0 valores iguais.
// 1 money maior que other.
func (money Money) Compare(other Money) (int, error) {
	err := money.Validate()

	if err != nil {
		return 0, err
	}

	err = other.Validate()

	if err != nil {
		return 0, err
	}

	if money.currency != other.currency {
		return 0, ErrCurrencyMismatch
	}

	if money.minorUnits < other.minorUnits {
		return -1, nil
	}

	if money.minorUnits > other.minorUnits {
		return 1, nil
	}

	return 0, nil
}

// Informa se a moeda é aceita
func isSupportedCurrency(currency string) bool {
	if currency == "BRL" {
		return true
	}

	if currency == "USD" {
		return true
	}

	return false
}

// Amount retorna o valor decimal com duas casas, sem usar ponto flutuante.
func (money Money) Amount() (string, error) {
	err := money.Validate()
	if err != nil {
		return "", err
	}

	digits := strconv.FormatInt(money.minorUnits, 10)
	sign := ""

	if strings.HasPrefix(digits, "-") {
		sign = "-"
		digits = digits[1:]
	}

	// Garante pelo menos um dígito inteiro e dois dígitos decimais.
	for len(digits) < 3 {
		digits = "0" + digits
	}

	decimalPosition := len(digits) - 2
	wholePart := digits[:decimalPosition]
	fractionPart := digits[decimalPosition:]

	return sign + wholePart + "." + fractionPart, nil
}

// moneyJSON define o formato de saída. Os valores continuam sendo strings.
type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// MarshalJSON é chamado automaticamente por json.Marshal.
// Valores internos negativos podem ser serializados, como na reconciliação.
func (money Money) MarshalJSON() ([]byte, error) {
	amount, err := money.Amount()
	if err != nil {
		return nil, err
	}

	payload := moneyJSON{
		Amount:   amount,
		Currency: money.currency,
	}

	return json.Marshal(payload)
}
