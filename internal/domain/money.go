package domain

import ("errors" "strconv" "strings" )

// Erros que podem ser retornados ao criar um valor monetário.
var ErrInvalidAmount = errors.New("valor monetário inválido")
var ErrInvalidCurrency = errors.New("moeda inválida")
var ErrAmountOutOfRange = errors.New("valor monetário fora do limite permitido")

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
//   - a moeda aceita é BRL;
//   - o valor precisa ter exatamente duas casas decimais;
//   - valores negativos não são aceitos neste construtor;
//   - espaços, sinais e notação científica são rejeitados.
//
// Exemplos válidos: "0.00", "10.50" e "1000.00".
func NewMoney(amount string, currency string) (Money, error) {
	if currency != "BRL" {
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