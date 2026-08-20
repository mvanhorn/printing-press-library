# Phase 5 Acceptance — wanderlust-goat (v2 reprint)

**Run:** 20260507-163338
**Level:** Quick Check
**Verdict:** PASS (7/7)

## Auth context

- Spec `auth.type`: `none` (Nominatim is the spec base URL; no auth required for Stage-1 anchor resolution).
- `GOOGLE_PLACES_API_KEY`: **not set** in environment. `near`/`goat`/`why` correctly exit code 4 with a documented setup hint per the brief.
- `ANTHROPIC_API_KEY`: not set (optional, only enables `--llm` criteria match — heuristic path runs by default).
- `HOTPEPPER_API_KEY`: not set (optional, hotpepper Stage-2 falls back to HTML scrape).

## Test matrix

| # | Command | Result |
|---|---------|--------|
| 1 | `doctor` | PASS — reports missing `GOOGLE_PLACES_API_KEY` per spec |
| 2 | `coverage --country JP --json` | PASS — JP region with 3 real Stage-2 sources + 2 stubs |
| 3 | `research-plan kissaten --country JP --json` | PASS — typed query plan returned |
| 4 | `places search --query "Eiffel Tower"` | PASS — Nominatim geocoding |
| 5 | `golden-hour "Tokyo Tower"` | PASS — pure-Go SunCalc, Asia/Tokyo zone |
| 6 | `agent-context` | PASS — JSON describing every command |
| 7 | `near "Tokyo"` (no key, expected exit 4) | PASS — exits 4 with the documented setup hint |

## Notes

- **Wiring invariant test:** `internal/cli/wiring_test.go` asserts every `internal/<source>/` package is imported by either `cli/` or `dispatch/` AND has at least one exported function called. Mutation-tested: a deliberately-unwired source dir fails the test with a clear message.
- **Two-stage funnel:** Stage 1 (Google Places NearbySearch) is wired; Stage 2 fans out to every implemented source per region (`tabelog`, `retty`, `hotpepper` for JP; `navermap`, `naverblog` for KR; `lefooding` for FR); Stage 3 trust-weighted ranking applies the closed-signal kill-gate and walking-time radius.
- **Stage-2 stubs:** 19 packages in `internal/<source>/` are typed Go stubs that satisfy the wiring test; `coverage` reports them with their deferral reason. Promoting any stub to real impl is a single-package edit.
- **Live API verification:** deferred. Stage-2 sources scrape live HTML; running them in CI would be flaky and the request rate would hit anti-bot mitigation. Instead, every source has `httptest`-backed unit tests asserting the parsing contract, plus a runtime sanity-check via `coverage` (regions table + registry consistency) and `sync-city` (in-process Stage-2 prewarm hits every source).

## Known machine-side gaps (not v2 brief items)

- The generator's SKILL canonical install URL uses `library/<category>/` derived from research.json, but `verify-skill` expects `library/other/`. Workaround: install URL is `library/other/` in the shipped SKILL. Filing for retro.
- `mcp_token_efficiency` scored 4/10 — only 2 typed MCP tools (the Nominatim places mirror); Cobra walker exposes the rest of the surface dynamically. v2 brief did not call for MCP enrichment.
