# SimpleFIN CLI Brief

## API Identity
- Domain: Read-only personal financial data sharing (SimpleFIN Protocol v2.0.0). "Like RSS, but for financial data."
- Users: Developers and power users who want programmatic, privacy-respecting access to their own bank/brokerage balances + transactions without handing bank credentials to an app. Big overlap with the plaintextaccounting / self-hosted-finance crowd (Actual Budget, Lunch Money, Beancount, Firefly III users).
- Data profile: One data endpoint (`GET /accounts`) returns a JSON "Account Set" spanning ALL the user's connected institutions at once: connections[], accounts[] (balance, available-balance, currency, balance-date), each account's transactions[] and (Bridge extension) holdings[]. Rich, multi-institution, point-in-time.

## Auth Model (UNUSUAL — drives design)
- No API key. Flow: user creates a **Setup Token** (base64 of a claim URL) -> app POSTs to claim URL -> receives an **Access URL** of the form `https://user:pass@host/simplefin` (HTTP Basic creds + base URL baked into one secret) -> GET `{ACCESS_URL}/accounts`.
- The Access URL is the single long-lived credential AND encodes the server host. Most users use the SimpleFIN Bridge (`bridge.simplefin.org`); a user's bank could run its own server (rare).
- CLI design: single secret `SIMPLEFIN_ACCESS_URL`, parsed at runtime into base_url + `Authorization: Basic base64(user:pass)`. A `claim` command exchanges a Setup Token for an Access URL and saves it. Public demo token available for testing (regenerates per page load at beta-bridge.simplefin.org/info/developers).

## Reachability Risk
- None. Live-verified 2026-06-29: claimed a fresh demo Setup Token -> Access URL -> `GET /accounts?version=2` returned 200 with 3 demo accounts (Demo Savings w/ AAPL holding, Demo Checking, Demo Empty); `start-date` window returned 171 transactions/account. `/info` returns `{"versions":["1","2"]}`.
- Probe-safe endpoint used: GET /accounts (read-only by protocol; SimpleFIN is read-only — no mutations exist).
- Rate limit is the real constraint: ~24 requests/day expected; exceeding it triggers warnings then Access-URL disablement. Date range per /accounts call capped at 90 days.

## Endpoints (complete surface)
- `GET /info` -> {versions:[...]} (no auth, server root)
- `GET /create` -> interactive web page (NOT a CLI command; the `claim` flow replaces it)
- `POST /claim/:token` -> Access URL (the claim flow)
- `GET /accounts` -> Account Set. Params: `start-date`, `end-date` (unix epoch; <=90d span), `pending` (=1), `account` (repeatable id filter), `balances-only` (=1), `version` (2).

## Data Layer (the heart of this CLI)
- Primary entities: connection, account, transaction, holding (Bridge ext), + balance-snapshot (OUR construct: each sync stamps balances over time -> time series the protocol never returns).
- Sync cursor: date-range based (start-date/end-date); idempotent upsert by (account_id, txn_id). Must dedupe overlapping windows.
- FTS/search: full-text over transaction description/payee/memo across all institutions.
- Categorization: SimpleFIN has NO native categories -> local rule/keyword-based categorization is a value-add.

## Codebase Intelligence
- (DeepWiki/MCP source analysis: see absorb manifest. Protocol is tiny + fully documented at simplefin.org/protocol.html, so ground truth is the spec itself, live-verified above.)

## Top Workflows
1. Connect once (`claim <setup-token>`) then `sync` to pull all institutions into a local store.
2. Cross-institution net worth + balance trajectory over time (snapshots).
3. Cash flow / spending analysis: income vs outflow, top merchants, by month.
4. Recurring/subscription detection across all accounts.
5. Export to ledger/beancount/CSV/QIF for plaintextaccounting tools.

## Table Stakes (to be confirmed by ecosystem research)
- list accounts + balances; list/search transactions; net worth; sync into local store; respect rate limit.

## Competitors / Ecosystem
Entire ecosystem is hobbyist (top stars ~12-110). **No polished Go CLI, no TUI.** Bar to beat:
- **csells/simplefin_rust** (Rust, richest CLI, 15 subcmds): claim, info, collect(sync), add-balance, status, stale, query, summary(net worth+changes), spending, spending-rules, recurring, trends, configure, schema, cleanup. Global --format json|text, --raw.
- **arjungandhi/money** (Go, new, 0 stars): init, fetch, balance(trend), budget, transactions, accounts, categories, LLM categorization, SQLite.
- **jeeftor/simplefin-cli** (Go, 12 stars): balances-only table. No transactions.
- **npm simplefin / @dilllxd/simplefin-cli** (Node): accounts/transactions/balances/info, --format csv|json, relative dates (7d/30d), --pending, --account, --raw, --json-first.
- **finance-reconcile-mcp** (MCP): reconcile audit, find-missing-txns, check-stale, balance-mismatch, find-duplicates.
- **JithendraNara cloudflare-finance-mcp**: finance_overview, detect_subscriptions, detect_recurring_obligations, merchant_summary, find_unusual_transactions, weekly briefing, scheduled sync w/ 3-day overlap.
- **sfin2ledger** (official org): pipe -> Ledger format. **beangulp-simplefin** aspirational. GnuCash/YNAB/Tiller: no native support.
- Consumers (workflow tells): Actual Budget (90-day cap, ~24h refresh, per-account map), Firefly III importer (content-based dedup), Lunch Money (paid sync backend), duplaja/actual-simplefin-sync (5-day lookback overlap dedup), SparkyBudget (110 stars).
- Holdings (investment positions): protocol supports symbol/shares/cost_basis/market_value; **almost nobody implements it** -> ecosystem-wide gap, big differentiator.

## Pain Points (each = a feature)
1. **Rate limit 24 req/day, DISABLES tokens on abuse.** "/accounts returns everything in one call; never loop per-account." -> rate-limit-aware sync + quota guard.
2. **90-day date cap, SILENT truncation** (Firefly UI offers 10yr, backend caps at 90). -> warn + chunked backfill into local store that survives the window.
3. **Dedup failures** — SimpleFIN txn IDs unstable/mirrored across accounts ($3,157 wire seen 3x). -> content-based dedup (amount+date+description), NOT id-based. #1 pain.
4. **Pending transactions cause balance mismatch** (pending that never posts lingers). -> pending lifecycle handling.
5. **Silent stale connections** (green dot while data 6wk old). -> stale-account detection by balance-date age.
Bonus: no native categories in protocol -> local categorization is pure value-add. Access URL = unscoped full-read credential -> careful storage (chmod 600 config, no logging).

## Product Thesis
- Name: simplefin-pp-cli
- NOI: SimpleFIN isn't just a bank-data connector. It's a cross-institution net-worth time machine. One synced pull turns scattered balances and transactions into a queryable local ledger that reveals subscriptions, cash-flow drift, and balance trajectories no single bank app can show — offline, agent-native, and rate-limit-aware.
- Why it should exist: Every existing SimpleFIN tool is an *importer into someone else's app*. None give you a fast, scriptable, offline CLI + local SQLite you own, with cross-institution analytics and agent-native (--json/--select) output.

## Build Priorities
1. Data layer: SQLite schema for connections/accounts/transactions/holdings + balance snapshots; idempotent sync from /accounts; rate-limit awareness.
2. Absorbed: claim/auth, list accounts, list/search transactions, balances, holdings, raw /accounts passthrough, info.
3. Transcend: net worth + trajectory, cash flow, recurring detection, spending-by-merchant, balance-history snapshots, drift/since, anomalies, portfolio gain/loss, export (ledger/beancount/csv/qif), quota/rate-limit doctor.
