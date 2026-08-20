# OfferUp CLI — Phase 4 Shipcheck Report

**Run:** 20260531-200239 · **Binary:** `offerup-pp-cli`

## Shipcheck umbrella — 6/6 PASS (exit 0)

| Leg | Verdict | Notes |
|---|---|---|
| verify | PASS | 0 failures |
| validate-narrative | PASS | 8 narrative commands resolved + full examples passed (after dropping the broken `sync` quickstart step) |
| dogfood | PASS | novel_features_check 6/6 built; synced README/SKILL/root-help/.printing-press.json from verified set |
| workflow-verify | PASS | |
| verify-skill | PASS | All checks (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command) + canonical-sections (after inlining location flags) |
| scorecard | PASS | **68/100 — Grade B** |

## Fixes applied during shipcheck
1. **validate-narrative**: dropped the quickstart `sync --resources listings --param ...` step — the generic `sync` can't drive the html-search `listings` resource and isn't OfferUp's population path (the price commands self-populate). Quickstart now: doctor → listings search → price-check → deals.
2. **verify-skill**: the location flags (`--zip/--lat/--lon/--city/--state/--category`) were registered via a shared `lf.register(cmd)` helper; verify-skill's static per-command parser couldn't follow the indirection ("declared elsewhere"). Inlined the flag registration into each command (the idiomatic generated pattern). Removed the now-dead helper.
3. **README/SKILL/AGENTS correctness audit (Phase 4.9)**: found stale `--markdowns` claim in `which.go` and old mechanical feature names in `mcp/tools.go` (dogfood doesn't sync those two surfaces — **retro candidate**). Hand-fixed both; corrected the `deals` description.

## Phase 4.95 local code review — NO FINDINGS
Independent reviewer verified: SQL fully parameterized (no injection), all `rows`/`db`/response bodies closed, untrusted `__NEXT_DATA__` parsed via comma-ok helpers (no panic surface), 429 → typed `*cliutil.RateLimitError` (exit 7, never empty), percentile/drop/median math correct, NULL-safe scans throughout. `go vet` clean.

## Simplify pass (ce-simplify-code, 3 reviewers) — applied
- Extracted `searchAndRecord` helper (price-check/deals/listings-search soft store-write path).
- Added `lf.storeKey(query)` method (collapsed 6 call sites).
- digest: reuse `PriceStats.Median` instead of recomputing `Median(stored)`.
- Hoisted two per-call `strings.NewReplacer`s to package vars (`locationCookieReplacer`, `priceReplacer`).
- Removed unreachable `&& l.Lat/Lon != ""` guards; removed redundant comment.
- **Skipped (reviewer misjudgments, would change behavior):** the `if fresh != nil` guards (keep JSON `[]` not `null`), the `storeKeyFor` nil-case (prevents nil-deref), the RecordSearch per-row SELECT (in-process SQLite, ~40 rows — not worth rewrite risk).
- Re-verified: build/vet/test 0, verify-skill 0, validate-narrative 0, live smoke correct (median consistent across price-check/deals/digest).

## Ship threshold
- shipcheck exit 0, all 6 legs PASS ✓
- verify PASS, dogfood wiring clean, workflow-verify not workflow-fail ✓
- verify-skill exit 0 ✓
- scorecard 68 ≥ 65 ✓
- **No flagship feature returns wrong/empty output** — behaviorally verified live (price-check $380 median over 41 listings; deals ranked correctly; listings clean/no-ads; item detail full incl. seller + condition; honest-empty price-drops on first run) ✓

## Scorecard weak dims (Phase 5.5 polish targets)
Workflows 4/10, Dead Code 0/5, Cache Freshness 5/10, Breadth 7/10, Vision 7/10. Path Validity renders 0/10 but is a SKIP (no spec-mirror paths; html endpoints). Total 68/B.

## Verdict: **ship**
All ship-threshold conditions met; no known functional bugs in shipping-scope features. Proceed to Phase 5 dogfood.
