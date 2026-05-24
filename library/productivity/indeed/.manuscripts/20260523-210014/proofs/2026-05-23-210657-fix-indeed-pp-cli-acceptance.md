# Indeed CLI — Phase 5 Live Dogfood Acceptance

Level: Full Dogfood (live, against indeed.com)
Gate: **PASS** — 56/56 tests passed (matrix_size 56), exit 0.

## Fixes applied during dogfood
1. **5 error-path false-positives** (`company`, `find`, `track`, `untrack`, `saved delete`):
   added `pp:no-error-path-probe: "true"`. These commands correctly treat any argument
   as valid — an empty search/aggregate is a valid empty result, and untrack/delete of a
   nonexistent target is an idempotent no-op. (CLI fix.)
2. **`workflow archive --json` emitted invalid JSON**: the generated syncResource NDJSON
   event stream + the command's summary JSON doc interleaved on stdout. Suppressed the
   event stream under `--json` (gated on the global humanFriendly) so stdout carries one
   parseable document. (CLI fix in generated channel_workflow.go.)

## Printing Press issues (retro)
- Generator: `limit` query param without an explicit `type:` was emitted as a Go `string`
  but passed to `truncateJSONArray(... int)` → build break. (Worked around with `type: integer`.)
- Generator: generated module pinned `golang.org/x/net@v0.43.0` (GO-2026-5026); had to bump
  to v0.55.0 for govulncheck to pass.
- `workflow archive --json` NDJSON+summary interleaving is a generic generator bug, not
  Indeed-specific.
- `dogfood --live --dir` runs a stale root binary if one exists instead of rebuilding;
  cost ~20 min chasing phantom failures. It should rebuild or warn on staleness.

## Manual verification (live)
search, job get, find, company, saved/new, track/tracked, related, apply — all return
correct data against the live Cloudflare-protected site via Surf.
