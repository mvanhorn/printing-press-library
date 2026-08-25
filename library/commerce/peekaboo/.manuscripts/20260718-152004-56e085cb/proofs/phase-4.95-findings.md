# Phase 4.95 Local Code Review — findings

Reviewer: general-purpose subagent. No error-severity (crash/security/race) issues.
Concurrency, partial-failure accounting, nil-deref, token bootstrap, dryRunOK ordering all verified clean.

## Autofixed in-place (6)
1. top_deals.go: removed duplicate partial-failure stderr print + unused `os` import.
2. peekaboo_ext.go extractGuestToken: json.Decoder tolerates trailing JS after the object.
3. peekaboo_ext.go fanOutCityDeals: return post-truncation attempted count so partial-failure denominator + "scanned N" note are honest.
4. nearest.go: surface a failed --city resolution instead of silently swallowing it.
5. peekaboo_ext.go ensureGuestToken: thread cmd.Context() so auto-bootstrap honors cancellation.
6. nearest.go parseLatLong: reject out-of-range lat/long; skip (0,0) branches when picking nearest; expiring sorts by parsed time.

## Retro candidates (generator/template — not patched in generated files)
- SKILL.md learn-loop hint references `peekaboo-pp-cli sync` and README/SKILL "Offline-friendly: sync/search commands use the local SQLite store" — generic framework boilerplate emitted even when the CLI has no sync command (this API's resources aren't auto-syncable). verify-skill did not flag it (not a broken invocation), but the hint is misleading for sync-less CLIs. File against the machine.
- Generator drops defaulted bool body params (only sends when flag Changed) — broke `deals list` until associatedDeals was modeled as a string. File against the machine.
- Hand-added top-level AddCommand (bootstrap, not in research.json novel_features) was dropped by one regen pass's lost-registration merge. File against the machine.
