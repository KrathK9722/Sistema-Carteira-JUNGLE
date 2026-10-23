package domain

import (
	"errors"
)

// Erros referentes a Wallet
var ErrInvalidWalletID = errors.New("identificador da carteira inválido")
var ErrInvalidPlayerID = errors.New("identificador do jogador inválido")
var ErrNegativeInitialBalance = errors.New("saldo inicial não pode ser negativo")
var ErrInvalidWalletTime = errors.New("data da carteira inválida")

// Erros referentes a Money
var ErrInvalidAmount = errors.New("valor monetário inválido")
var ErrInvalidCurrency = errors.New("moeda inválida")
var ErrAmountOutOfRange = errors.New("valor monetário fora do limite permitido")
var ErrInvalidMoney = errors.New("valor monetário não inicializado ou inválido")
var ErrCurrencyMismatch = errors.New("não é possível operar valores de moedas diferentes")
