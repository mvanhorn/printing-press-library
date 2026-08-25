# Patch: 2026-08-17 — `accounts reconcile` searches full history by default

Hand-edit applied to a hand-authored novel command. Recorded here so a
reprint's reconciliation context carries the intent forward instead of
silently dropping it.

## Root cause

`accounts reconcile` compares the account's real all-time
`cleared_balance_currency` against a user-supplied statement balance, then
fetches the account's transaction list to surface the candidate(s) most
likely responsible for any difference. When `--since-date` was left unset,
the transaction fetch omitted the `since_date` query parameter entirely —
and per the bundled OpenAPI spec, YNAB's transactions endpoint "Defaults to
one year ago when not specified."

The all-time difference was always computed correctly, but the candidate
search silently only covered the trailing year. A prior patch
(`ynab-novel-command-freshness`) added a note to `result.Note` explaining
this caveat when `--since-date` was omitted — but a note doesn't recover the
transaction; it only tells the user their evidence might be incomplete.
Greptile's follow-up review of PR #1725 (confidence 4/5) flagged this as the
reason the PR wasn't yet safe to merge: any account with more than a year of
cleared history could have its actual discrepancy-causing transaction never
appear in the ranked candidates at all.

## Fix

Added `reconcileDefaultSinceDate = "2000-01-01"` and used it whenever
`--since-date` is not supplied, so the transaction fetch always passes an
explicit `since_date` covering the account's practical full history instead
of relying on the API's own one-year default. `result.Note`'s caveat was
inverted: it now only appears when the *user* has deliberately narrowed the
window with `--since-date`, since that's the only remaining case where
candidates might miss an older transaction.

Verified `go build ./...`, `go vet ./...`, and
`go test ./internal/cli/... -run Reconcile` all clean.

## Not touched

`export balances` and `payees profile` were not affected — neither depends
on `since_date` defaults in the same way, and Greptile's review scoped this
finding to `accounts_reconcile.go` only.
