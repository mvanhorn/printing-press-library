# Expensify CLI Reprint — Build Log

Manifest transcendence rows: 10 planned, 10 built. (novel_features_check: planned 10, found 10)

## Built on the fresh 4.25.0 base
Priority 0 (foundation): internal/store/store.go (816-line SQLite store, WAL + busy_timeout 15s + SetMaxOpenConns(1)), sync (ReconnectApp ingest, handles both onyxData-envelope and spec-typed array shapes; --db/--full/--resources flags for the verify data-pipeline contract).
Priority 1 (absorbed): all generated endpoint commands (expense/report/admin/workspace/export_resource/category/me/recon/tag) from the collision-fixed spec.
Priority 2 (transcendence, 10 claimed): expense quick, expense from-line, expense search, expense rollup, expense dupes, expense missing-receipts, damage, report draft, expense bulk, report submit --wait. Plus honest stubs (watch/undo/close/admin policy-diff) shipped but unclaimed.

## Hand patches re-applied (.printing-press-patches.json)
created-not-date (HEADLINE), expensify-form-wire (form-encoded authToken-in-body), surface-jsoncode-407, dry-run-form-body, delete-resolve-refs, auth-from-chrome, novel-layer.

## Generator bug fixed (retro candidate)
internal/config AuthHeader(): "ExpensifyToken {authToken}" placeholder had no matching map key -> AuthHeader always "" -> doctor "not configured" + 4 failing internal/cliutil credential tests on the CLEAN 4.25.0 base. Added "authToken" key. doctor + cliutil tests now pass.

## Deferred / notes
- report submit --wait: added with waitForSubmitExit poll helper.
- from-chrome cookie token can be a stale classic-session token for NewDot-primary users (follow-up: mint from device credentials).
