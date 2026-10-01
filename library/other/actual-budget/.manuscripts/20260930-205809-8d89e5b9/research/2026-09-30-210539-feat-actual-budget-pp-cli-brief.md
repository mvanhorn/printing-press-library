# Actual Budget CLI Brief

## API Identity
- Domain: personal finance / envelope budgeting. Actual Budget (actualbudget/actual, ~29k★) is a self-hosted, local-first YNAB alternative.
- Users: self-hosters running `actual-server` (Docker), mostly technical, who script imports, backups and reports.
- Data profile: one budget = one SQLite file (`db.sqlite` inside a zip, optionally AES-GCM encrypted). The server only stores and syncs CRDT messages; all logic lives in the client. Amounts are integer minor units, `transactions.date` is int YYYYMMDD, budget months are int YYYYMM, deletes are `tombstone=1`, and merged payees/categories resolve through `payee_mapping`/`category_mapping`.
- **There is no HTTP API.** The official docs (https://actualbudget.org/docs/api/) say so explicitly: the API is the Node package `@actual-app/api`, which downloads the budget and runs the engine locally.

## Transport Decision (user-confirmed: Hybrid)
1. **Native read path (no sidecar):** talk to the sync server directly.
   - `POST /account/login {password}` → `data.token`
   - `GET /sync/list-user-files` (header `X-ACTUAL-TOKEN`)
   - `GET /sync/get-user-file-info` and `GET /sync/download-user-file` (header `X-ACTUAL-FILE-ID`)
   - Unzip → `db.sqlite` → local read-only mirror. Encrypted files: `POST /sync/user-get-key` (salt) → derive key, then AES-GCM decrypt.
   - Every read, search, report and audit command runs on this mirror.
2. **Write path via REST sidecar:** jhonderson/actual-http-api (256★, release 26.9.0 on 2026-09-04, matches the user's server 26.9.0).
   - 82 operations under `/v1/budgets/{budgetSyncId}/...`, header `x-api-key`, optional `budget-encryption-password` header.
   - Spec extracted from swagger-jsdoc annotations at commit e17b5c3 → `research/actual-http-api-openapi.json`.
3. Rejected: native CRDT protobuf writes (too risky to keep correct across monthly releases); sidecar-only (forces a second container even for read-only use).

## Reachability Risk
- Low for the sync server (the user runs it locally; `GET /info` → `{"version":"26.9.0"}`).
- Medium for actual-http-api: recurring client/server version-mismatch breakage (#109 `no such column: bank_sync_status`, #99/#102 backup broke on 26.6.0, #101 `/export` broken). Open: #114 (rate limit on bank sync). Mitigation: a `doctor` command compares the sidecar's `/actualhttpapiversion` with the server's `/info`.
- Native download path risk: schema drift across migrations. Mitigation: read through tolerant SQL (select only known columns; feature-detect with `PRAGMA table_info`).

## Top Workflows
1. "Where did my money go this month?": spending by category/payee for a month or range, budget vs actual, uncategorized transactions.
2. Import bank CSV/QIF/OFX into an account with dedupe (`transactions/import` reconcile path, dry-run first).
3. Cleanup: find uncategorized transactions, duplicate or near-duplicate payees, rules that never fire, stale schedules; then bulk-categorize or merge.
4. Monthly budgeting: show a month's budgeted/spent/balance per category, set amounts, move money between categories, hold for next month.
5. Backup and export: a version-proof zip download straight from the sync server, plus CSV export of transactions or reports.

## Table Stakes (from @actual-app/cli, actualpy CLI, actual-mcp, actual-mcp-server, abctl)
- CRUD: accounts (plus close/reopen/balance), transactions (list/add/import/update/delete), categories, category groups, payees (plus merge, common), rules (plus payee-rules), schedules, tags, notes, account groups.
- Budget months: list, show, set-amount, set-carryover, hold/reset-hold, category transfer.
- ActualQL `query run`; `get-id` by name; bank sync trigger; export/import budget; server version.
- Reports (actual-mcp): spending-by-category, monthly-summary, budget-vs-actual, net-worth, category-trends, spending-by-payee, cash-flow, balance-history.
- Output: `--format json|table|csv`; config via env (`ACTUAL_SERVER_URL`, `ACTUAL_PASSWORD`, `ACTUAL_SYNC_ID`, `ACTUAL_ENCRYPTION_PASSWORD`, `_FILE` variants).

## Data Layer
- Primary entities: transactions, accounts, categories, category_groups, payees, rules, schedules, zero_budgets/reflect_budgets, notes, tags.
- Sync cursor: whole-file snapshot. Re-download when the server's file `groupId`/size changes, or on `sync --force`. Optional later improvement: apply `/sync/sync` message deltas (read-only CRDT apply).
- FTS/search: FTS5 over transaction notes, imported_description, payee name and category name, built in the CLI's own store next to the mirror (never written into the downloaded `db.sqlite`).
- Names resolve through the mapping tables. Treat `v_transactions`-equivalent logic carefully: exclude tombstoned rows, parents vs children of splits, and closed accounts where relevant.

## Codebase Intelligence
- Source: actualbudget/actual `packages/sync-server/src/app-sync.ts`, `app-account.js`, `util/validate-user.ts`; `loot-core/src/server/cloud-storage.ts` (zip packaging, encryption); `packages/crdt/src/proto/sync.proto`.
- Auth: server password → session token (header `X-ACTUAL-TOKEN`); 401 `{reason:"unauthorized"|"token-expired"}`. Sidecar: `x-api-key`.
- Rate limiting: none on the sync server; the sidecar's bank-sync can hit upstream bank provider limits.
- Architecture insight: the server is a dumb file and message store. A Go CLI that owns a SQLite mirror is *structurally faithful* to Actual, not a cache bolted onto a REST API.

## Product Thesis
- Name: `actual-budget-pp-cli`
- Why it should exist: every existing tool needs Node, either `@actual-app/api` in-process or the http-api sidecar, and loads the whole engine for each call. This is a single static binary that reads your budget straight from your own server with no Node and no sidecar, and answers questions the app and existing CLIs can't: ad-hoc SQL and full-text search over the real ledger, cleanup audits (uncategorized, duplicate payees, dead rules), and version-proof backups. When you want to write, it drives the sidecar with dry-run-first imports and bulk operations.

## Build Priorities
1. Native sync-server client: login, list budgets, download/decrypt, local mirror + `doctor`.
2. Offline read commands on the mirror: accounts/balances, transactions (filter/search), categories, payees, budget month view, reports.
3. Generated REST commands from the sidecar spec (writes), with `--dry-run` on import and bulk operations.
4. Novel: cleanup audits, CSV import mapping, backups, spending reports/trends, SQL passthrough.
5. Agent-native output (`--json`, stable exit codes) and an MCP surface.

## Reachability Gate
- Decision: PASS (carve-out)
- Reason: lan-only-no-global-url
- Evidence: spec servers[0] = http://localhost:5007/v1 (self-hosted sidecar); native sync server http://localhost:5006 GET /info → 200 {"version":"26.9.0"}, GET /sync/list-user-files without token → 401 {"reason":"unauthorized","details":"token-not-found"}.
