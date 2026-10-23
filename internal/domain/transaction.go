package domain

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
