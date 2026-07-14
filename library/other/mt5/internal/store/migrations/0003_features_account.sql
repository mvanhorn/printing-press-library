-- Migration 0003: account-scope features.
--
-- The features table originally keyed on (symbol, tf, time_ms) and was thus
-- account-blind: building features for account B silently overwrote account
-- A's rows for the same symbol/timeframe. Add account_login to both the
-- columns and the primary key.
--
-- SQLite can't ALTER the primary key in place, so we rebuild via the
-- standard table-swap pattern. Existing rows are preserved with
-- account_login = 0 (legacy / unknown); users who care should re-run
-- 'mt5-pp-cli features build --account <login> ...' to populate the new
-- per-account rows.

-- Wrapped in BEGIN IMMEDIATE / COMMIT by the Go migration runner.

CREATE TABLE features_new (
    account_login   INTEGER NOT NULL DEFAULT 0,
    symbol          TEXT    NOT NULL,
    tf              TEXT    NOT NULL,
    time_ms         INTEGER NOT NULL,
    ret             REAL,
    log_ret         REAL,
    atr_14          REAL,
    rsi_14          REAL,
    realized_vol_20 REAL,
    PRIMARY KEY (account_login, symbol, tf, time_ms)
) WITHOUT ROWID;

INSERT INTO features_new (account_login, symbol, tf, time_ms, ret, log_ret, atr_14, rsi_14, realized_vol_20)
SELECT 0, symbol, tf, time_ms, ret, log_ret, atr_14, rsi_14, realized_vol_20 FROM features;

DROP TABLE features;
ALTER TABLE features_new RENAME TO features;
