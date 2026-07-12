# Phase 3 Build Log — passive-indices-pp-cli

Manifest transcendence rows: 9 planned, 9 built.

## Scope

Combo CLI over niftyindices.com (primary, NSE official index data) and
indiapassivefunds.com (secondary, ETF/index-fund data), bseindices.com
explicitly out of scope. 22-feature manifest: 13 absorbed + 9 novel.

## Completion Gate

All 21 manifest commands smoke-tested via `--help` (exit 0) against the built
binary. No unresolvable command paths.

## Fixes made in this CLI (printed-CLI-specific, not machine)

- `fund_rankings.go`: de-claimed the `--command topAUM` example. Upstream
  `indiapassivefunds` marketrankings endpoint does not publish its accepted
  enum values; `topAUM` returns `"Requested ranking doesnt exist"` live. Help
  text and error messaging now say so instead of implying a working example.
- Removed stale "constituent weight" language left over from an earlier
  correction (the constituents CSV has no per-constituent weight field, only
  company/industry/symbol/ISIN). Propagated the fix to:
  `index_constituents.go`, `index_constituents_diff.go`,
  `passive-indices-spec.yaml`, `research.json` (5 fields), `README.md`,
  `SKILL.md`.
- Added `// pp:client-call` markers to 6 novel-feature files
  (`index_funds.go`, `index_cheapest_tracker.go`, `fund_raw.go`,
  `index_tracking_error.go`, `fund_nfo_tracking.go`, `index_tracking.go`) so
  the reimplementation checker recognizes their calls into the hand-written
  sibling client packages (`internal/niftyindices`, `internal/indiapassivefunds`)
  reached via a package-local `newXClient()` helper — a documented gap in the
  checker's structural signal detection, not a reimplementation violation.
- Added `internal/niftyindices/client_test.go` and
  `internal/indiapassivefunds/client_test.go`: stdlib-only unit tests
  (no testify — not a dependency of this CLI) covering the pure-logic parsing
  helpers in both sibling clients (slugify/truncate/cinfo body shape;
  list-envelope decode, taxonomy value lookup, nested section scanning,
  null-vs-zero latest-ratios handling, similar-funds parsing). Closes the
  `test_presence` gap dogfood flagged.

## Machine-level bug found and fixed (out of this CLI's tree)

While running `dogfood`, its auto-doc-sync step reported 0/9 novel features
found and destructively wrote that empty result back into `which.go`,
`root.go` Highlights, README/SKILL Unique Features sections, MCP tool
descriptions, and `.printing-press.json`. Root cause: `commandPath()` in
`internal/pipeline/dogfood.go` stripped `-`-prefixed flag tokens from a
`NovelFeature.Command` string but not `<`-prefixed placeholder tokens (e.g.
`"index funds <index>"`), so it could never resolve to the real registered
Cobra path and always reported a miss.

Fixed in the main `cli-printing-press` repo (uncommitted, pending user
decision on when to commit):
- `internal/pipeline/dogfood.go`: `commandPath()` now also breaks on `<`.
- `internal/pipeline/dogfood_test.go`: added
  `TestCommandPathStripsPlaceholdersAndFlags` (7 subtests).
- `internal/generator/templates/which_test.go.tmpl`: same class of bug in the
  generated `which` command's well-formedness test; added a
  `whichCommandPath()` helper mirroring the fix.

Verified via `go test ./...` (full suite) and `scripts/golden.sh verify`
(32/32) in isolation from two unrelated pre-existing uncommitted files found
in the same working tree (`internal/generator/generator.go`,
`internal/pipeline/live_dogfood.go` — not part of this session's work, left
untouched).

Re-ran `dogfood` against passive-indices-pp-cli after the fix: correctly
found and re-synced all 9 novel features, restoring the wiped docs (now with
the wording corrections above already applied).

## Final dogfood state

- `novel_features_check`: 9 planned, 9 found.
- `reimplementation_check`: 9 checked, 1 exempted via store access, 8
  exempted via `pp:client-call` directive (includes both the 6 fixed above
  and 2 already-marked files).
- `test_presence`: clean.
- Verdict: WARN — `defaultSyncResources empty` is the only remaining note,
  an accepted non-blocking design characteristic (this CLI has no natural
  default sync scope across two unrelated APIs; sync requires explicit
  `--resources`).

## Next

Proceed to Phase 4 `shipcheck`.
