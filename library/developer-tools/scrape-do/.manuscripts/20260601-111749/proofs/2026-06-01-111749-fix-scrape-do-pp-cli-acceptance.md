# Scrape.do CLI — Phase 5 Acceptance Report

Level: **Full Dogfood** (binary-owned live matrix) · Run: 20260601-111749

## Result
- Tests: **68/68 passed** (verdict PASS). Acceptance marker: `phase5-acceptance.json` status `pass`.
- Credits: ~40 spent across the matrix (live Google verticals + scrape); ~895 of 1000 remaining.

## First-pass failures (4) — all fixed inline before the passing re-run
1. `batch [happy_path]` & `[json_fidelity]` — the Example used `--input urls.txt` (nonexistent file). **Fix:** `batch` now accepts inline positional targets (`batch <url...>`) and the Example uses real URLs; `--input`/stdin retained as fallbacks.
2. `budget set [json_fidelity]` — `--dry-run --json` returned bare nil (no JSON). **Fix:** dry-run now emits a `would_set_*` JSON object.
3. `scrape [error_path]` — invalid input reached the API (HTTP 400, exit 0). **Fix:** `scrape` validates the target URL locally (must be a fully-qualified host) and returns a usage error (no credit spent) on garbage input.

All four re-ran green; full matrix 68/68.

## Behavioral spot-checks (live)
- `google search "best crm software" --agent --select organic_results.position,...` → correct narrowed SERP, 11 organic rows persisted, cost=10 (header).
- `scrape https://example.com` → cost=1 (header), ledgered; invalid URL → exit 2, no call.
- `batch https://example.com https://example.org --json` → dispatched 2, succeeded 2, total_cost 2.
- `budget` → live `/info` remaining + concurrency cap 5 + spend by mode.
- `cost`/`drift`/`movers`/`sql` → correct offline (honest empty-state).

## Fixes applied this phase
- 4 dogfood fixes above (CLI-specific).
- (Phase 4.95) token-leak scrub, sql read-only hardening, ledger detached-ctx (CLI-specific).

## Printing Press issues (for retro)
1. One-line description truncation on dotted brands / multi-clause headlines (v4.20.0).
2. Nested-subcommand `--help` exits 2 instead of 0 (v4.20.0 regression).
3. verify data-pipeline false-negative for no-syncable-resource CLIs with hand domain tables.
4. `cliutil.SanitizeErrorBody` redaction omits the `token=` query-param pattern.

## Gate: PASS
No PII in live outputs (SERP results are public web data; account state is numeric). Proceed to Phase 5.5 (polish) and promotion.
