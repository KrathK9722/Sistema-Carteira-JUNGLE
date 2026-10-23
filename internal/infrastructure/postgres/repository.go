package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"example.com/financial-service/internal/application"
	"example.com/financial-service/internal/domain"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"strconv"
	"time"
)

type Repository struct{ Pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository    { return &Repository{Pool: pool} }
func (r *Repository) Ready(ctx context.Context) error { return r.Pool.Ping(ctx) }
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

type scanner interface{ Scan(...any) error }

func decimal(units int64) string { return fmt.Sprintf("%d.%02d", units/100, units%100) }
func scanWallet(row scanner) (domain.Wallet, error) {
	var id, player, currency string
	var balance, version int64
	var created, updated time.Time
	err := row.Scan(&id, &player, &currency, &balance, &version, &created, &updated)
	if err != nil {
		return domain.Wallet{}, dbError(err)
	}
	money, err := domain.NewMoney(decimal(balance), currency)
	if err != nil {
		return domain.Wallet{}, err
	}
	return domain.RehydrateWallet(id, player, money, version, created, updated)
}
func (r *Repository) Wallet(ctx context.Context, id string) (domain.WalletView, error) {
	wallet, err := scanWallet(r.Pool.QueryRow(ctx, `SELECT id,player_id,currency,balance,version,created_at,updated_at FROM wallets WHERE id=$1`, id))
	if err != nil {
		return domain.WalletView{}, err
	}
	return domain.View(wallet)
}
func insertOutbox(ctx context.Context, tx pgx.Tx, id, aggregate, kind string, event any) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id,aggregate_id,event_type,payload) VALUES($1,$2,$3,$4)`, id, aggregate, kind, raw)
	return err
}
func transactionEvent(ctx context.Context, tx pgx.Tx, id, wallet, kind, status, failure, correlation string, money domain.MoneyInput) error {
	event, err := domain.NewTransactionEvent(kind, wallet, correlation, domain.TransactionEventData{TransactionID: id, WalletID: wallet, Status: status, FailureCode: failure, Money: money})
	if err != nil {
		return err
	}
	return insertOutbox(ctx, tx, event.EventID, wallet, event.EventType, event)
}
func recordMovement(ctx context.Context, tx pgx.Tx, id string, wallet domain.Wallet, direction string, money domain.MoneyInput, before int64, correlation string) error {
	amount, err := domain.NewMoney(money.Amount, money.Currency)
	if err != nil {
		return err
	}
	entryID, err := domain.NewID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO wallet_ledger(id,wallet_id,transaction_id,direction,amount,currency,balance_before,balance_after,wallet_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, entryID, wallet.ID(), id, direction, amount.MinorUnits(), money.Currency, before, wallet.Balance().MinorUnits(), wallet.Version())
	if err != nil {
		return err
	}
	event, err := domain.NewBalanceEvent(wallet.ID(), correlation, domain.BalanceEventData{WalletID: wallet.ID(), TransactionID: id, Direction: direction, Money: money, BalanceBefore: domain.MoneyInput{Amount: decimal(before), Currency: money.Currency}, BalanceAfter: domain.MoneyInput{Amount: decimal(wallet.Balance().MinorUnits()), Currency: money.Currency}, WalletVersion: wallet.Version()})
	if err != nil {
		return err
	}
	return insertOutbox(ctx, tx, event.EventID, wallet.ID(), event.EventType, event)
}
func (r *Repository) CreateWallet(ctx context.Context, player string, money domain.Money) (domain.WalletView, error) {
	id, err := domain.NewID()
	if err != nil {
		return domain.WalletView{}, err
	}
	wallet, err := domain.NewWallet(id, player, money, time.Now())
	if err != nil {
		return domain.WalletView{}, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain.WalletView{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO wallets(id,player_id,currency,balance,version,created_at,updated_at) VALUES($1,$2,$3,$4,1,$5,$5)`, id, player, money.Currency(), money.MinorUnits(), wallet.CreateDate())
	if err != nil {
		return domain.WalletView{}, dbError(err)
	}
	if money.MinorUnits() > 0 {
		openingID, idErr := domain.NewID()
		if idErr != nil {
			return domain.WalletView{}, idErr
		}
		input := domain.MoneyInput{Amount: decimal(money.MinorUnits()), Currency: money.Currency()}
		_, err = tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,wallet_id,player_id,kind,amount,currency,status,result_balance) VALUES($1,'INTERNAL',$2,$3,'OPENING',$4,$5,'PROCESSED',$4)`, openingID, id, player, money.MinorUnits(), money.Currency())
		if err != nil {
			return domain.WalletView{}, err
		}
		err = recordMovement(ctx, tx, openingID, wallet, "CREDIT", input, 0, id)
		if err != nil {
			return domain.WalletView{}, err
		}
		err = transactionEvent(ctx, tx, openingID, id, "WagerTransactionProcessed", "PROCESSED", "", id, input)
		if err != nil {
			return domain.WalletView{}, err
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return domain.WalletView{}, err
	}
	return domain.View(wallet)
}
func resultRow(row scanner) (domain.Result, error) {
	var result domain.Result
	var balance *int64
	var currency string
	err := row.Scan(&result.TransactionID, &result.Status, &result.FailureCode, &balance, &currency)
	if err != nil {
		return result, dbError(err)
	}
	if balance != nil {
		result.Balance = domain.MoneyInput{Amount: decimal(*balance), Currency: currency}
	}
	return result, nil
}
func (r *Repository) Transaction(ctx context.Context, provider, id, external string) (domain.Result, error) {
	return resultRow(r.Pool.QueryRow(ctx, `SELECT id,status,COALESCE(failure_code,''),result_balance,currency FROM wager_transactions WHERE origin='EXTERNAL' AND provider_id=$1 AND (($2<>'' AND id=$2) OR ($3<>'' AND external_id=$3))`, provider, id, external))
}
func beginInbox(ctx context.Context, tx pgx.Tx, message *application.InboxMessage) error {
	if message == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO inbox(consumer_name,message_id,payload_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, message.Consumer, message.ID, message.Hash)
	if err != nil {
		return err
	}
	var hash string
	err = tx.QueryRow(ctx, `SELECT payload_hash FROM inbox WHERE consumer_name=$1 AND message_id=$2 FOR UPDATE`, message.Consumer, message.ID).Scan(&hash)
	if err != nil {
		return err
	}
	if hash != message.Hash {
		return domain.ErrConflict
	}
	return nil
}
func finishInbox(ctx context.Context, tx pgx.Tx, message *application.InboxMessage) error {
	if message == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE inbox SET completed_at=now() WHERE consumer_name=$1 AND message_id=$2`, message.Consumer, message.ID)
	return err
}
func (r *Repository) Process(ctx context.Context, op domain.Operation, correlation string, message *application.InboxMessage) (domain.Result, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return domain.Result{}, err
	}
	defer tx.Rollback(ctx)
	err = beginInbox(ctx, tx, message)
	if err != nil {
		return domain.Result{}, err
	}
	wallet, err := scanWallet(tx.QueryRow(ctx, `SELECT id,player_id,currency,balance,version,created_at,updated_at FROM wallets WHERE id=$1 FOR UPDATE`, op.WalletID))
	if err != nil {
		return domain.Result{}, err
	}
	if wallet.PlayerID() != op.PlayerID {
		return domain.Result{}, domain.ErrForbidden
	}
	if wallet.Currency() != op.Money.Currency {
		return domain.Result{}, domain.ErrCurrencyMismatch
	}
	hash, err := op.PayloadHash()
	if err != nil {
		return domain.Result{}, err
	}
	id, err := domain.NewID()
	if err != nil {
		return domain.Result{}, err
	}
	raw, err := json.Marshal(op)
	if err != nil {
		return domain.Result{}, err
	}
	money, err := domain.NewMoney(op.Money.Amount, op.Money.Currency)
	if err != nil {
		return domain.Result{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO wager_transactions(id,origin,provider_id,external_id,idempotency_key,payload_hash,wallet_id,player_id,kind,amount,currency,reference_external_id,request,status) VALUES($1,'EXTERNAL',$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),$12,'PENDING') ON CONFLICT DO NOTHING`, id, op.ProviderID, op.ExternalID, op.IdempotencyKey, hash, op.WalletID, op.PlayerID, op.Kind, money.MinorUnits(), op.Money.Currency, op.Reference, raw)
	if err != nil {
		return domain.Result{}, err
	}
	if tag.RowsAffected() == 0 {
		rows, queryErr := tx.Query(ctx, `SELECT id,payload_hash,idempotency_key,external_id FROM wager_transactions WHERE provider_id=$1 AND (idempotency_key=$2 OR external_id=$3)`, op.ProviderID, op.IdempotencyKey, op.ExternalID)
		if queryErr != nil {
			return domain.Result{}, queryErr
		}
		count := 0
		var existingID, existingHash, existingKey, existingExternal string
		for rows.Next() {
			count++
			queryErr = rows.Scan(&existingID, &existingHash, &existingKey, &existingExternal)
			if queryErr != nil {
				rows.Close()
				return domain.Result{}, queryErr
			}
		}
		queryErr = rows.Err()
		rows.Close()
		if queryErr != nil {
			return domain.Result{}, queryErr
		}
		if count != 1 || existingHash != hash || existingKey != op.IdempotencyKey || existingExternal != op.ExternalID {
			return domain.Result{}, domain.ErrConflict
		}
		result, queryErr := resultRow(tx.QueryRow(ctx, `SELECT id,status,COALESCE(failure_code,''),result_balance,currency FROM wager_transactions WHERE id=$1`, existingID))
		if queryErr != nil {
			return domain.Result{}, queryErr
		}
		result.IdempotentReplay = true
		err = finishInbox(ctx, tx, message)
		if err != nil {
			return domain.Result{}, err
		}
		err = tx.Commit(ctx)
		return result, err
	}
	result, err := r.apply(ctx, tx, id, op, wallet, correlation, "PENDING")
	if err != nil {
		return domain.Result{}, err
	}
	err = finishInbox(ctx, tx, message)
	if err != nil {
		return domain.Result{}, err
	}
	err = tx.Commit(ctx)
	return result, err
}
func (r *Repository) apply(ctx context.Context, tx pgx.Tx, id string, op domain.Operation, wallet domain.Wallet, correlation, oldStatus string) (domain.Result, error) {
	var reference *domain.Operation
	var referenceID, referenceStatus string
	if op.Reference != "" {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT id,status,request FROM wager_transactions WHERE provider_id=$1 AND external_id=$2`, op.ProviderID, op.Reference).Scan(&referenceID, &referenceStatus, &raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return domain.Result{}, err
		}
		if err == nil {
			var value domain.Operation
			err = json.Unmarshal(raw, &value)
			if err != nil {
				return domain.Result{}, err
			}
			reference = &value
		}
	}
	direction, failure := domain.Movement(op, reference, referenceStatus)
	status := "PROCESSED"
	eventType := "WagerTransactionProcessed"
	if failure == "REFERENCE_PENDING" {
		var expired bool
		err := tx.QueryRow(ctx, `SELECT expires_at<=now() OR attempts>=30 FROM wager_transactions WHERE id=$1`, id).Scan(&expired)
		if err != nil {
			return domain.Result{}, err
		}
		if expired {
			failure = "REFERENCE_NOT_FOUND"
		} else {
			status = "PENDING_REFERENCE"
			eventType = "WagerTransactionPendingReference"
			failure = ""
		}
	}
	if status == "PROCESSED" && failure == "" && (op.Kind == "REFUND" || op.Kind == "ROLLBACK") {
		var reversed bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM wager_transactions WHERE reference_id=$1 AND kind IN ('REFUND','ROLLBACK') AND status='PROCESSED')`, referenceID).Scan(&reversed)
		if err != nil {
			return domain.Result{}, err
		}
		if reversed {
			failure = "ALREADY_REVERSED"
		}
	}
	before := wallet.Balance().MinorUnits()
	money, err := domain.NewMoney(op.Money.Amount, op.Money.Currency)
	if err != nil {
		return domain.Result{}, err
	}
	if failure == "" && status == "PROCESSED" {
		updated := time.Now().UTC()
		if updated.Before(wallet.UpdateDate()) {
			updated = wallet.UpdateDate()
		}
		if direction == "DEBIT" {
			err = wallet.Debit(money, updated)
		} else if direction == "CREDIT" {
			err = wallet.Credit(money, updated)
		}
		if errors.Is(err, domain.ErrInsufficientBalance) {
			failure = "INSUFFICIENT_BALANCE"
			if op.Kind == "ROLLBACK" {
				failure = "REVERSAL_INSUFFICIENT_BALANCE"
			}
		} else if errors.Is(err, domain.ErrAmountOutOfRange) {
			failure = "AMOUNT_OUT_OF_RANGE"
		} else if err != nil {
			return domain.Result{}, err
		}
	}
	if failure != "" {
		status = "REJECTED"
		eventType = "WagerTransactionRejected"
	}
	state, err := domain.RestoreWagerTransaction(oldStatus)
	if err != nil {
		return domain.Result{}, err
	}
	err = state.Transition(status)
	if err != nil {
		return domain.Result{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE wager_transactions SET status=$2,failure_code=NULLIF($3,''),result_balance=$4,reference_id=NULLIF($5,''),updated_at=now(),attempts=attempts+1,next_attempt_at=now()+make_interval(secs=>LEAST(300,power(2,LEAST(attempts,8)))::double precision) WHERE id=$1`, id, status, failure, wallet.Balance().MinorUnits(), referenceID)
	if err != nil {
		return domain.Result{}, dbError(err)
	}
	if status == "PROCESSED" && direction != "" {
		_, err = tx.Exec(ctx, `UPDATE wallets SET balance=$2,version=$3,updated_at=$4 WHERE id=$1`, wallet.ID(), wallet.Balance().MinorUnits(), wallet.Version(), wallet.UpdateDate())
		if err != nil {
			return domain.Result{}, err
		}
		err = recordMovement(ctx, tx, id, wallet, direction, op.Money, before, correlation)
		if err != nil {
			return domain.Result{}, err
		}
	}
	if status != "PENDING_REFERENCE" || oldStatus != "PENDING_REFERENCE" {
		err = transactionEvent(ctx, tx, id, wallet.ID(), eventType, status, failure, correlation, op.Money)
		if err != nil {
			return domain.Result{}, err
		}
	}
	return domain.Result{TransactionID: id, Status: status, FailureCode: failure, Balance: domain.MoneyInput{Amount: decimal(wallet.Balance().MinorUnits()), Currency: wallet.Currency()}}, nil
}
func (r *Repository) ResumeOne(ctx context.Context) (bool, error) {
	var id, walletID string
	err := r.Pool.QueryRow(ctx, `SELECT id,wallet_id FROM wager_transactions WHERE status='PENDING_REFERENCE' AND next_attempt_at<=now() ORDER BY next_attempt_at LIMIT 1`).Scan(&id, &walletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	wallet, err := scanWallet(tx.QueryRow(ctx, `SELECT id,player_id,currency,balance,version,created_at,updated_at FROM wallets WHERE id=$1 FOR UPDATE SKIP LOCKED`, walletID))
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT request FROM wager_transactions WHERE id=$1 AND status='PENDING_REFERENCE' AND next_attempt_at<=now() FOR UPDATE SKIP LOCKED`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var op domain.Operation
	err = json.Unmarshal(raw, &op)
	if err != nil {
		return false, err
	}
	_, err = r.apply(ctx, tx, id, op, wallet, id, "PENDING_REFERENCE")
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

type Publisher interface {
	Publish(context.Context, string, string, string) error
}

func (r *Repository) PublishOne(ctx context.Context, publisher Publisher) (bool, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id, aggregate string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT event_id,aggregate_id,payload FROM outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY occurred_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &aggregate, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	publishErr := publisher.Publish(ctx, id, aggregate, string(payload))
	if publishErr == nil {
		_, err = tx.Exec(ctx, `UPDATE outbox SET published_at=now(),attempts=attempts+1 WHERE event_id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE outbox SET attempts=attempts+1,next_attempt_at=now()+make_interval(secs=>LEAST(300,power(2,LEAST(attempts,8)))::double precision) WHERE event_id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return false, err
	}
	return true, publishErr
}
func signedDecimal(units string) string {
	sign := ""
	if len(units) > 0 && units[0] == '-' {
		sign = "-"
		units = units[1:]
	}
	for len(units) < 3 {
		units = "0" + units
	}
	return sign + units[:len(units)-2] + "." + units[len(units)-2:]
}
func (r *Repository) Reconcile(ctx context.Context, id string) (any, error) {
	var stored int64
	var calculated, difference, currency string
	var count int64
	err := r.Pool.QueryRow(ctx, `SELECT w.balance,COALESCE(sum(CASE WHEN l.direction='CREDIT' THEN l.amount::numeric ELSE -l.amount::numeric END),0)::text,(w.balance::numeric-COALESCE(sum(CASE WHEN l.direction='CREDIT' THEN l.amount::numeric ELSE -l.amount::numeric END),0))::text,w.currency,count(l.id) FROM wallets w LEFT JOIN wallet_ledger l ON l.wallet_id=w.id WHERE w.id=$1 GROUP BY w.id`, id).Scan(&stored, &calculated, &difference, &currency, &count)
	if err != nil {
		return nil, dbError(err)
	}
	return map[string]any{"walletId": id, "storedBalance": domain.MoneyInput{Amount: decimal(stored), Currency: currency}, "calculatedBalance": domain.MoneyInput{Amount: signedDecimal(calculated), Currency: currency}, "difference": domain.MoneyInput{Amount: signedDecimal(difference), Currency: currency}, "consistent": difference == "0", "checkedEntries": count}, nil
}
func (r *Repository) Ledger(ctx context.Context, id, cursor string, limit int) (any, error) {
	var after int64
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, domain.ErrInvalidOperation
		}
		after, err = strconv.ParseInt(string(raw), 10, 64)
		if err != nil || after < 0 {
			return nil, domain.ErrInvalidOperation
		}
	}
	if limit < 1 || limit > 100 {
		return nil, domain.ErrInvalidOperation
	}
	_, err := r.Wallet(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := r.Pool.Query(ctx, `SELECT sequence,id,transaction_id,direction,amount,currency,balance_before,balance_after,created_at FROM wallet_ledger WHERE wallet_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, id, after, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]map[string]any, 0)
	next := ""
	var last int64
	for rows.Next() {
		var sequence, amount, before, balance int64
		var entryID, transaction, direction, currency string
		var created time.Time
		err = rows.Scan(&sequence, &entryID, &transaction, &direction, &amount, &currency, &before, &balance, &created)
		if err != nil {
			return nil, err
		}
		if len(entries) == limit {
			next = base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(last, 10)))
			break
		}
		entries = append(entries, map[string]any{"id": entryID, "walletId": id, "transactionId": transaction, "direction": direction, "money": domain.MoneyInput{Amount: decimal(amount), Currency: currency}, "balanceBefore": domain.MoneyInput{Amount: decimal(before), Currency: currency}, "balanceAfter": domain.MoneyInput{Amount: decimal(balance), Currency: currency}, "createdAt": created})
		last = sequence
	}
	return map[string]any{"entries": entries, "nextCursor": next}, rows.Err()
}
