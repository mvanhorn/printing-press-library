-- mt5-pp-cli local mirror schema
-- Migration 0001: initial tables
-- Runner: internal/store will apply pending migrations on first Open().
--
-- Time fields are stored as INTEGER unix epoch milliseconds (UTC) unless
-- otherwise noted. Money fields are REAL (broker reports doubles already).
-- All bar/tick/deal/order tables carry the originating account_login so a
-- single store can mirror multiple accounts.

-- The Go runner wraps each migration in BEGIN IMMEDIATE / COMMIT, owns
-- schema_migrations bookkeeping, and re-checks the applied state inside
-- the transaction. Migrations themselves are pure DDL/DML.

CREATE TABLE accounts (
    login        INTEGER PRIMARY KEY,
    server       TEXT NOT NULL,
    name         TEXT,
    company      TEXT,
    currency     TEXT,
    leverage     INTEGER,
    trade_mode   TEXT,   -- demo | contest | real
    last_synced  INTEGER
);

CREATE TABLE symbols (
    account_login INTEGER NOT NULL,
    symbol        TEXT    NOT NULL,
    description   TEXT,
    digits        INTEGER,
    point         REAL,
    spread        INTEGER,
    contract_size REAL,
    volume_min    REAL,
    volume_max    REAL,
    volume_step   REAL,
    trade_mode    INTEGER,
    trade_calc    INTEGER,
    base_currency TEXT,
    profit_currency TEXT,
    margin_initial REAL,
    last_synced   INTEGER,
    PRIMARY KEY (account_login, symbol)
);

CREATE TABLE ticks (
    account_login INTEGER NOT NULL,
    symbol        TEXT    NOT NULL,
    time_ms       INTEGER NOT NULL,
    bid           REAL,
    ask           REAL,
    last          REAL,
    volume_real   REAL,
    flags         INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;

CREATE INDEX idx_ticks_symbol_time ON ticks(symbol, time_ms);

-- One bars table per timeframe (smaller indexes, fewer index lookups per query).
-- Schema is identical across timeframes; generated for the canonical MT5 set.

CREATE TABLE bars_M1 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_M1_symbol_time ON bars_M1(symbol, time_ms);

CREATE TABLE bars_M5 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_M5_symbol_time ON bars_M5(symbol, time_ms);

CREATE TABLE bars_M15 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_M15_symbol_time ON bars_M15(symbol, time_ms);

CREATE TABLE bars_M30 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_M30_symbol_time ON bars_M30(symbol, time_ms);

CREATE TABLE bars_H1 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_H1_symbol_time ON bars_H1(symbol, time_ms);

CREATE TABLE bars_H4 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_H4_symbol_time ON bars_H4(symbol, time_ms);

CREATE TABLE bars_D1 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;
CREATE INDEX idx_bars_D1_symbol_time ON bars_D1(symbol, time_ms);

CREATE TABLE bars_W1 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;

CREATE TABLE bars_MN1 (
    account_login INTEGER NOT NULL, symbol TEXT NOT NULL,
    time_ms INTEGER NOT NULL, o REAL, h REAL, l REAL, c REAL,
    tick_volume INTEGER, spread INTEGER, real_volume INTEGER,
    PRIMARY KEY (account_login, symbol, time_ms)
) WITHOUT ROWID;

-- Orders / positions / deals
CREATE TABLE positions (
    account_login INTEGER NOT NULL,
    ticket        INTEGER NOT NULL,
    symbol        TEXT,
    type          TEXT,        -- buy | sell
    volume        REAL,
    price_open    REAL,
    price_current REAL,
    sl            REAL,
    tp            REAL,
    profit        REAL,
    swap          REAL,
    magic         INTEGER,
    comment       TEXT,
    time_open_ms  INTEGER,
    time_update_ms INTEGER,
    PRIMARY KEY (account_login, ticket)
);

CREATE TABLE orders (
    account_login INTEGER NOT NULL,
    ticket        INTEGER NOT NULL,
    symbol        TEXT,
    type          TEXT,        -- buy_limit | sell_limit | buy_stop | sell_stop
    volume_initial REAL,
    volume_current REAL,
    price_open    REAL,
    sl            REAL,
    tp            REAL,
    time_setup_ms INTEGER,
    time_expiration_ms INTEGER,
    state         TEXT,
    magic         INTEGER,
    comment       TEXT,
    PRIMARY KEY (account_login, ticket)
);

CREATE TABLE history_orders (
    account_login INTEGER NOT NULL,
    ticket        INTEGER NOT NULL,
    symbol        TEXT,
    type          TEXT,
    state         TEXT,
    volume_initial REAL,
    volume_current REAL,
    price_open    REAL,
    sl            REAL,
    tp            REAL,
    time_setup_ms INTEGER,
    time_done_ms  INTEGER,
    magic         INTEGER,
    comment       TEXT,
    PRIMARY KEY (account_login, ticket)
);

CREATE TABLE deals (
    account_login INTEGER NOT NULL,
    ticket        INTEGER NOT NULL,
    order_ticket  INTEGER,
    position_id   INTEGER,
    symbol        TEXT,
    type          TEXT,
    entry         TEXT,        -- in | out | inout | out_by
    volume        REAL,
    price         REAL,
    commission    REAL,
    swap          REAL,
    profit        REAL,
    fee           REAL,
    time_ms       INTEGER,
    magic         INTEGER,
    comment       TEXT,
    PRIMARY KEY (account_login, ticket)
);

CREATE INDEX idx_deals_time   ON deals(account_login, time_ms);
CREATE INDEX idx_deals_symbol ON deals(account_login, symbol);
CREATE INDEX idx_deals_magic  ON deals(account_login, magic);
CREATE INDEX idx_deals_posid  ON deals(account_login, position_id);

-- Economic calendar (Phase 9)
CREATE TABLE calendar_events (
    id           INTEGER PRIMARY KEY,
    time_ms      INTEGER NOT NULL,
    currency     TEXT,
    importance   INTEGER, -- 0=low 1=med 2=high
    event_name   TEXT,
    actual       REAL,
    forecast     REAL,
    previous     REAL
);
CREATE INDEX idx_calendar_time ON calendar_events(time_ms);

-- Derived features table (Phase 9)
CREATE TABLE features (
    symbol     TEXT    NOT NULL,
    tf         TEXT    NOT NULL,
    time_ms    INTEGER NOT NULL,
    ret        REAL,
    log_ret    REAL,
    atr_14     REAL,
    rsi_14     REAL,
    realized_vol_20 REAL,
    PRIMARY KEY (symbol, tf, time_ms)
) WITHOUT ROWID;

-- Backtest results (Phase 9)
CREATE TABLE backtests (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    strategy      TEXT NOT NULL,
    symbol        TEXT NOT NULL,
    tf            TEXT NOT NULL,
    from_ms       INTEGER,
    to_ms         INTEGER,
    deposit       REAL,
    net_profit    REAL,
    profit_factor REAL,
    sharpe        REAL,
    max_dd_pct    REAL,
    trades        INTEGER,
    win_rate      REAL,
    params_json   TEXT,
    metrics_json  TEXT,
    created_at_ms INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000)
);

-- Append-only audit log for writes (Phase 6 mirrors this into audit.jsonl
-- as well — DB form is for fast queries, JSONL is for line-tail tools).
CREATE TABLE audit (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    time_ms     INTEGER NOT NULL DEFAULT (strftime('%s', 'now') * 1000),
    command     TEXT NOT NULL,
    request     TEXT NOT NULL,   -- canonical JSON of request
    hash        TEXT NOT NULL,
    confirmed   INTEGER NOT NULL,
    response    TEXT,             -- canonical JSON of response (broker's)
    error       TEXT,
    account_login INTEGER,
    mode        TEXT              -- paper | live | dry-run
);
CREATE INDEX idx_audit_time ON audit(time_ms);
