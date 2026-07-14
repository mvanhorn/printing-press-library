-- Migration 0002: account-scope backtest results.
--
-- Backtests originally landed without account_login, which made it impossible
-- to filter "show me only backtests run on this account's mirror data."
-- Multi-account users were silently mixing strategy results across accounts.
--
-- SQLite ALTER TABLE supports ADD COLUMN; the migration is idempotent because
-- the runner skips a version that's already recorded in schema_migrations.

-- Wrapped in BEGIN IMMEDIATE / COMMIT by the Go migration runner.

ALTER TABLE backtests ADD COLUMN account_login INTEGER;

CREATE INDEX IF NOT EXISTS idx_backtests_account ON backtests(account_login);
