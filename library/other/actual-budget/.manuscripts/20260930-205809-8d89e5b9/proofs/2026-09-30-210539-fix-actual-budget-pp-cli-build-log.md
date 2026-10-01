Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

## Slice 1 — foundation (build green)
- internal/actual: native sync-server client (login, list-user-files, get-user-file-info, user-get-key, download-user-file, /info), AES-256-GCM + PBKDF2-SHA512 decryption (verified against loot-core encryption-internals.electron.ts), zip extraction (root or single dir), mirror pull/manifest (0600 files, XDG data dir/mirrors/<syncId>), read-only open (mode=ro, query_only), Ledger query layer resolving payee_mapping/category_mapping, tombstones, splits, integer dates/amounts.
- internal/actual/actualtest: realistic fixture budget (merged payee, deleted mapped category, split, transfers, starting balances, templates, rules incl. dead/dangling/shadowed, schedules, CRDT log).
- Tests: plain + encrypted pull (missing key, wrong key), auth error, deleted budget, nested zip, ledger resolution, uncategorized semantics, leaf balances, merged payee counts, amount/date helpers. All pass.
- internal/config/actual_config.go: ACTUAL_SERVER_URL/PASSWORD/SESSION_TOKEN/SYNC_ID/ENCRYPTION_PASSWORD (+_FILE).
- Commands: mirror pull (--keep-zip backups), mirror status, budgets-remote. Absorbed rows 1, 2, 34.
- doctor: actual_server / actual_login / actual_budget / actual_mirror / sidecar_version_skew (row 24). One-line hook added to generated doctor.go.
- Decision: mark the sidecar x-api-key optional (x-auth-optional) at next regen so read-only users don't see doctor FAIL.
- Next slice: ledger (search, uncategorized, duplicates, sql) + report (spending, cashflow, budget-vs-actual, net-worth, trends, recurring).

## Slice 2 — ledger + reports (build green; verified by orchestrator)
- ledger search / uncategorized / duplicates (novel #2) / sql (novel #5); report spending / cashflow / budget-vs-actual / net-worth / trends / recurring. Absorbed rows 25-31, 33.
- Logic in internal/actual/reports.go with table tests; cli tests assert fixture answers (Aug spending total 173723, dup pair dup-a/dup-b, uncategorized set of 5, net worth 4000000 at 2026-06-30, rent recurring scheduled=true).
- Search is a Go substring AND-scan over leaf txns (not FTS5) — deliberate: correctness over index, budgets are small.
- Transcendence built: 2/8 (ledger duplicates, ledger sql).

## Slice 3 — audits, categorize, templates, changes, import-csv (build green; verified by orchestrator)
- audit payees (#1), categorize suggest (#3, --apply PATCHes sidecar; partial failures surfaced), audit rules (#4), audit schedules (#6), templates status (#7, notes #template/#goal + goal_def JSON), changes since (#8, messages_crdt decode). import-csv (absorbed row 32) POSTs to sidecar transactions/import with reconcile.
- Verified sidecar PATCH /transactions/{id} accepts partial bodies (validateTransactionBody only checks non-empty) → category-only PATCH is valid.
- Rule matching compares resolved (post-merge) payee/category ids; documented in help.

## Regen 2 (spec overlay fixes)
- PATCH request bodies: dropped nested `required` (update = partial per official docs) → `transactions update` no longer forces --transaction-account/--transaction-date.
- `cleared` typed boolean (was string).
- Sidecar key x-auth-optional → doctor reports INFO, not FAIL, for read-only users.
- Cross-spec regen dropped the doctor.go hook line; re-applied (2-line edit: addActualDoctorChecks call + checkKeys rows).
- gofmt: report_test.go formatted.

## Phase 3 completion gate
- Per-row Cobra resolution: 20/20 approved paths resolve with `<path> [flags]` usage.
- dogfood novel_features_check: planned 8, found 8, missing none (depth_mismatches are leaf-name heuristic false positives).
- Deferred: generated doctor "Sidecar API" check still prints FAIL when the optional sidecar is down (cosmetic; revisit in polish).
- Skipped body fields: none blocking.
