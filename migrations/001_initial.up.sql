CREATE TABLE IF NOT EXISTS wallets (
 id text PRIMARY KEY,
 player_id text NOT NULL CHECK (length(trim(player_id)) > 0),
 currency text NOT NULL CHECK (currency IN ('BRL','USD')),
 balance bigint NOT NULL CHECK (balance >= 0),
 version bigint NOT NULL CHECK (version >= 1),
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL CHECK (updated_at >= created_at),
 UNIQUE(player_id,currency), UNIQUE(id,currency)
);
CREATE TABLE IF NOT EXISTS wager_transactions (
 id text PRIMARY KEY,
 origin text NOT NULL CHECK(origin IN ('INTERNAL','EXTERNAL')),
 provider_id text, external_id text, idempotency_key text, payload_hash text,
 wallet_id text NOT NULL, player_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('OPENING','BET','WIN','LOSS','REFUND','ROLLBACK')),
 amount bigint NOT NULL CHECK(amount >= 0), currency text NOT NULL,
 reference_external_id text, reference_id text REFERENCES wager_transactions(id),
 request jsonb,
 status text NOT NULL CHECK(status IN ('PENDING','PENDING_REFERENCE','PROCESSED','REJECTED','FAILED')),
 failure_code text, result_balance bigint CHECK(result_balance >= 0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours',
 FOREIGN KEY(wallet_id,currency) REFERENCES wallets(id,currency),
 CHECK ((origin='INTERNAL' AND kind='OPENING' AND provider_id IS NULL AND external_id IS NULL AND idempotency_key IS NULL AND payload_hash IS NULL AND request IS NULL AND reference_external_id IS NULL AND reference_id IS NULL AND status='PROCESSED' AND amount>0)
 OR (origin='EXTERNAL' AND kind<>'OPENING' AND provider_id IS NOT NULL AND external_id IS NOT NULL AND idempotency_key IS NOT NULL AND payload_hash IS NOT NULL AND request IS NOT NULL)),
 CHECK ((kind='LOSS' AND amount=0) OR (kind<>'LOSS' AND amount>0)),
 CHECK (status NOT IN ('PROCESSED','REJECTED') OR result_balance IS NOT NULL),
 CHECK (kind NOT IN ('REFUND','ROLLBACK') OR reference_external_id IS NOT NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS external_operation_identity ON wager_transactions(provider_id,external_id) WHERE origin='EXTERNAL';
CREATE UNIQUE INDEX IF NOT EXISTS external_idempotency_identity ON wager_transactions(provider_id,idempotency_key) WHERE origin='EXTERNAL';
CREATE UNIQUE INDEX IF NOT EXISTS one_opening_per_wallet ON wager_transactions(wallet_id) WHERE kind='OPENING';
-- One direct reversal of any kind prevents REFUND + ROLLBACK returning a BET twice.
CREATE UNIQUE INDEX IF NOT EXISTS one_successful_reversal ON wager_transactions(reference_id) WHERE kind IN ('REFUND','ROLLBACK') AND status='PROCESSED';
CREATE INDEX IF NOT EXISTS pending_reference_schedule ON wager_transactions(next_attempt_at) WHERE status='PENDING_REFERENCE';
CREATE TABLE IF NOT EXISTS wallet_ledger (
 sequence bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 id text PRIMARY KEY, wallet_id text NOT NULL, transaction_id text NOT NULL REFERENCES wager_transactions(id),
 direction text NOT NULL CHECK(direction IN ('DEBIT','CREDIT')),
 amount bigint NOT NULL CHECK(amount>0), currency text NOT NULL,
 balance_before bigint NOT NULL CHECK(balance_before>=0), balance_after bigint NOT NULL CHECK(balance_after>=0),
 wallet_version bigint NOT NULL CHECK(wallet_version>=1),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(wallet_id,transaction_id), UNIQUE(wallet_id,wallet_version),
 FOREIGN KEY(wallet_id,currency) REFERENCES wallets(id,currency),
 CHECK ((direction='CREDIT' AND balance_after::numeric=balance_before::numeric+amount::numeric)
 OR (direction='DEBIT' AND balance_after::numeric=balance_before::numeric-amount::numeric))
);
CREATE INDEX IF NOT EXISTS ledger_wallet_cursor ON wallet_ledger(wallet_id,sequence);
CREATE OR REPLACE FUNCTION validate_ledger_insert() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE operation wager_transactions; previous_balance bigint; previous_version bigint;
BEGIN
 PERFORM 1 FROM wallets WHERE id=NEW.wallet_id FOR UPDATE;
 SELECT * INTO operation FROM wager_transactions WHERE id=NEW.transaction_id;
 IF operation.wallet_id IS DISTINCT FROM NEW.wallet_id OR operation.currency IS DISTINCT FROM NEW.currency OR operation.amount IS DISTINCT FROM NEW.amount OR operation.status IS DISTINCT FROM 'PROCESSED' OR operation.result_balance IS DISTINCT FROM NEW.balance_after OR operation.kind='LOSS' THEN RAISE EXCEPTION 'ledger must match processed operation'; END IF;
 SELECT balance_after,wallet_version INTO previous_balance,previous_version FROM wallet_ledger WHERE wallet_id=NEW.wallet_id ORDER BY sequence DESC LIMIT 1;
 IF NEW.balance_before<>COALESCE(previous_balance,0) THEN RAISE EXCEPTION 'ledger chain broken'; END IF;
 IF operation.kind='OPENING' THEN
  IF previous_version IS NOT NULL OR NEW.wallet_version<>1 THEN RAISE EXCEPTION 'invalid opening ledger'; END IF;
 ELSIF NEW.wallet_version<>COALESCE(previous_version,1)+1 THEN RAISE EXCEPTION 'ledger version chain broken'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS ledger_validate ON wallet_ledger;
CREATE TRIGGER ledger_validate BEFORE INSERT ON wallet_ledger FOR EACH ROW EXECUTE FUNCTION validate_ledger_insert();
CREATE OR REPLACE FUNCTION validate_wallet_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE wallet_identity text; stored_balance bigint; stored_version bigint; total numeric; expected_version bigint;
BEGIN
 IF TG_TABLE_NAME='wallets' THEN wallet_identity:=NEW.id; ELSE wallet_identity:=NEW.wallet_id; END IF;
 SELECT balance,version INTO stored_balance,stored_version FROM wallets WHERE id=wallet_identity;
 SELECT COALESCE(sum(CASE WHEN l.direction='CREDIT' THEN l.amount::numeric ELSE -l.amount::numeric END),0),1+count(*) FILTER(WHERE t.kind<>'OPENING') INTO total,expected_version FROM wallet_ledger l JOIN wager_transactions t ON t.id=l.transaction_id WHERE l.wallet_id=wallet_identity;
 IF stored_balance::numeric<>total OR stored_version<>expected_version THEN RAISE EXCEPTION 'wallet balance/version must agree with ledger'; END IF;
 RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS wallet_ledger_consistency ON wallets;
CREATE CONSTRAINT TRIGGER wallet_ledger_consistency AFTER INSERT OR UPDATE ON wallets DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_wallet_ledger();
DROP TRIGGER IF EXISTS ledger_wallet_consistency ON wallet_ledger;
CREATE CONSTRAINT TRIGGER ledger_wallet_consistency AFTER INSERT ON wallet_ledger DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION validate_wallet_ledger();
CREATE OR REPLACE FUNCTION protect_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'ledger is append-only'; END $$;
DROP TRIGGER IF EXISTS ledger_immutable ON wallet_ledger;
CREATE TRIGGER ledger_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON wallet_ledger FOR EACH STATEMENT EXECUTE FUNCTION protect_ledger();
CREATE OR REPLACE FUNCTION protect_terminal_transaction() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'transaction deletion forbidden'; END IF;
 IF OLD.status IN ('PROCESSED','REJECTED','FAILED') THEN RAISE EXCEPTION 'terminal transaction is immutable'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS transaction_terminal ON wager_transactions;
CREATE TRIGGER transaction_terminal BEFORE UPDATE OR DELETE ON wager_transactions FOR EACH ROW EXECUTE FUNCTION protect_terminal_transaction();
CREATE TABLE IF NOT EXISTS inbox (
 consumer_name text NOT NULL, message_id text NOT NULL, payload_hash text NOT NULL,
 received_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 PRIMARY KEY(consumer_name,message_id)
);
CREATE TABLE IF NOT EXISTS outbox (
 event_id text PRIMARY KEY, aggregate_id text NOT NULL, event_type text NOT NULL, payload jsonb NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now(), attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz
);
CREATE INDEX IF NOT EXISTS outbox_schedule ON outbox(next_attempt_at) WHERE published_at IS NULL;
CREATE OR REPLACE FUNCTION protect_outbox_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.event_id IS DISTINCT FROM OLD.event_id OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id OR NEW.event_type IS DISTINCT FROM OLD.event_type OR NEW.payload IS DISTINCT FROM OLD.payload OR NEW.occurred_at IS DISTINCT FROM OLD.occurred_at THEN RAISE EXCEPTION 'outbox snapshot is immutable'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS outbox_snapshot ON outbox;
CREATE TRIGGER outbox_snapshot BEFORE UPDATE ON outbox FOR EACH ROW EXECUTE FUNCTION protect_outbox_snapshot();
