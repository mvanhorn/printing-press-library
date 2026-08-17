# Patch: 2026-08-15 — YNAB `{"data": {...}}` envelope double-nesting

Hand-edits applied to generator-owned (`DO NOT EDIT`) files. Recorded here so
a reprint carries the intent forward instead of silently dropping it.

## Root cause

Every YNAB API response is wrapped in a top-level `{"data": {...}}` envelope
(e.g. `GET /user` returns `{"data": {"user": {...}}}`, `GET
/plans/{id}/accounts` returns `{"data": {"accounts": [...], ...}}`). This
print run's spec absorption never classified that wrapper, so all
generator-emitted read and write commands passed the raw API body straight
through into this CLI's own `results` envelope, producing
`results.data.<field>` instead of `results.<field>` — doubly nested and
inconsistent with the CLI's 3 hand-written novel commands (`export
balances`, `accounts reconcile`, `payees profile`), which build their own
output and were never affected.

`internal/cli/data_source.go`'s local-cache/store write-through logic
(`writeThroughNestedEnvelopeKeys`) already correctly expects and strips this
wrapper when populating the local SQLite mirror — only the live JSON
display path was affected. Confirmed no test fixture hardcoded the buggy
shape (`go test ./internal/cli/...` was green both before and after).

## Fix

Added `unwrapYNABEnvelope` in `internal/cli/helpers.go`, next to the
existing `unwrapSingleKeyArray` (which only flattens a single-key wrapper
whose value is an array — YNAB's `data` key almost always wraps an object,
which that helper intentionally leaves alone). `unwrapYNABEnvelope` strips
exactly a top-level, single-key `"data"` wrapper; any other shape passes
through unchanged.

Wired in at the **display layer only**, after any local-cache write has
already happened against the original wrapped bytes, so store/offline
reads and the learn loop are unaffected:

- `internal/cli/helpers.go`'s `wrapWithProvenance` — covers all 28
  generated read commands (`accounts get`, `plans get`, `user`, etc.), which
  all funnel through this one shared function.
- 16 generated write-command files (`accounts_create.go`,
  `categories_create-category.go`, `category-groups_create.go`,
  `category-groups_update.go`, `categories_update-category.go`,
  `months_categories_update-month-category.go`, `payees_create.go`,
  `payees_update.go`, `scheduled-transactions_create.go`,
  `scheduled-transactions_update.go`, `scheduled-transactions_delete.go`,
  `transactions_create.go`, `transactions_update.go`,
  `transactions_update-plans.go`, `transactions_delete.go`,
  `transactions_import.go`) — each had its own `filtered := data` line
  changed to `filtered := unwrapYNABEnvelope(data)`, ahead of `--select`/
  `--compact` filtering so those flags also operate on the corrected shape.

Verified live against the real API (`user`, `plans get`, `accounts get`)
post-fix: clean `results.user` / `results.plans` / `results.accounts`
shapes. Verified a real `accounts create --dry-run` still produces its
synthetic `{"dry_run": true}` marker untouched (confirms the unwrap is
scoped to exactly the `"data"` key and doesn't over-fire on unrelated
single-key objects). `go build ./...`, `go vet ./...`, and `go test
./internal/cli/...` all clean; `verify` unchanged at 78/79 (same
pre-existing, unrelated `resource-path:export` finding).

## Not touched

The fetch/cache layer (`resolveReadWithStrategyAndResponsePath`,
`applyResponsePath`, `writeThroughCache` in `data_source.go`) was
deliberately left alone. That function's existing `responsePath` parameter
could in principle unwrap `"data"` earlier (before the local-cache write),
but doing so would change what the store-population logic receives, and
that logic's own `writeThroughNestedEnvelopeKeys` handling already
correctly expects the wrapped shape — changing it risked breaking offline
reads/sync for no added benefit over the display-layer fix. Recommend the
generator's spec-absorption step learn to classify this API's envelope
convention (e.g. an `x-response-envelope: data` spec annotation feeding
`responsePath`) so future prints don't need this patch at all — retro
candidate.
