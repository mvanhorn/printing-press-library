# SimpleFIN CLI Build Log

Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Built
### Foundation (hand-authored, durable separate files)
- internal/config/simplefin_access.go — parses SIMPLEFIN_ACCESS_URL into BaseURL + Basic auth header (the access URL encodes its own host + creds). One-line Load() hook.
- internal/simplefin/ — protocol domain pkg: AccountSet/Account/Transaction/Holding/Connection types, ClaimSetupToken (base64->claim POST->access URL), ParseAmount, NormalizePayee, ContentHash (content-based dedup key), ParseDate (epoch/relative/YYYY-MM-DD). 6 unit tests.
- internal/store/simplefin_schema.go — dedicated tables: balance_snapshots, categorization_rules, transaction_categories. Accounts/transactions/holdings/connections live in the generic resources table (free framework search/sql/provenance).
- internal/cli/simplefin_common.go — store helpers, missing-mirror guard, typed loaders.

### Absorbed (12)
- auth claim (simplefin_auth.go), transactions (ledger, --account/--since/--category), holdings, stale, sync (nested extraction + balance snapshots + rule application + sync_state), accounts/info (generated promoted endpoints), search/sql/doctor (framework).

### Transcendence (8, all hand-code, all shipping)
1. networth [--at] [--trend] — cross-institution net worth + snapshot trajectory. Treats credit/loan as liabilities.
2. cashflow [--month|--by-merchant] — income/outflow GROUP BY month + merchant ranking.
3. recurring [--min-occurrences] — subscription detection by payee+amount-CV+cadence (interval rules, not LLM).
4. portfolio [--gain] — holdings market_value - cost_basis per position + aggregate.
5. categorize [--add|--rules|--apply|--list-rules] — deterministic regex rule engine; runs during sync.
6. export --format ledger|beancount|qif|csv|json — balanced double-entry plaintextaccounting export.
7. reconcile [--fix] — content-based dedup (amount+day+normalized-desc) catching mirrored/unstable-ID duplicates.
8. since [date] — neutral "recent activity since" digest (new txns + balance change), renamed from the alarmist "drift" per user feedback.

## Replaced
- Framework generic sync -> hand sync (the nested single /accounts shape defeats the generic resource-walker).
- Framework JSONL export -> hand export (adds ledger/beancount/qif for the SimpleFIN audience; keeps json).
- Framework newSyncCmd/newExportCmd remain defined but unregistered (Go-legal; shipcheck dead-code will confirm).

## Verified live (against public demo Access URL)
- claim->sync: 3 accounts, 230-340 txns, 1 AAPL holding, 1 connection; 90-day cap warning surfaced from errlist.
- networth $137,457.78; cashflow monthly; portfolio AAPL 550sh; categorize 112 txns -> Groceries; export beancount/ledger/qif valid; all dry-run probes exit 0.

## User feedback folded in (from ~/repo/claude/toddfinances)
- Schema mirrors proven toddfin design (idempotent upsert preserving categories, balance snapshots).
- Elevated `transactions` ledger command with --account filter.
- Added `since` (neutral framing, not "drift").
- Prioritized content dedup (reconcile) + rate-limit/quota awareness (sync curtails under dogfood; doctor reports).

## Deferred / notes
- reconcile --fix deletes from resources but not resources_fts (minor: search may show a removed dup until next sync).
- Multi-currency net worth is a naive sum (flagged in output note); no FX conversion.
