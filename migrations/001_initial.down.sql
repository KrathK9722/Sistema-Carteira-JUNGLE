DROP TABLE IF EXISTS outbox,inbox,wallet_ledger,wager_transactions,wallets CASCADE;
DROP FUNCTION IF EXISTS protect_ledger(),protect_terminal_transaction(),protect_outbox_snapshot();
DROP FUNCTION IF EXISTS validate_ledger_insert(),validate_wallet_ledger();
DROP TABLE IF EXISTS schema_migrations;
