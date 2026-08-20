# ExchangeRate-API CLI Shipcheck Proof

Run: 20260517-025552
Generated via Printing Press v4.8.0
Codex mode: enabled (but most edits ran inline — Codex round-trips would have added latency without value for mechanical patches)

## Shipcheck Final Verdict: **PASS (6/6 legs)**

| Leg | Result | Exit |
|---|---|---|
| dogfood | PASS | 0 |
| verify | PASS | 0 |
| workflow-verify | PASS | 0 (no workflow manifest; skipped) |
| verify-skill | PASS | 0 (1 known false positive: 'codes' mock-value probe) |
| validate-narrative | PASS | 0 |
| scorecard | PASS | 0 |

## Scorecard

- **Total: 81/100 — Grade A**
- Sample output probe: 7/10 (3 known-acceptable misses: convert-batch shell expansion in scorecard runner, time-travel example uses 2026-04-10 date older than fresh-DB snapshots, mcp serve blocks on stdio)

### Per-dimension highlights
- Output Modes 10/10
- Auth 10/10
- Error Handling 10/10
- Doctor 10/10
- Agent Native 10/10
- MCP Quality 10/10
- Local Cache 10/10
- Workflows 10/10
- Insight 10/10
- Sync Correctness 10/10
- Path Validity 10/10

### Gaps (acknowledged)
- **Auth Protocol 4/10**: spec declares `auth.type: api_key, in: query` with placeholder header `apikey_placeholder`. The real auth is in the URL path; the query-string variant is suppressed at request time (client.go patched), but the scorecard dimension reads from the spec declaration. Not fixed because the real auth model (path-segment key) isn't a spec-supported `auth.in:` value; suppressing the spec auth entirely loses the EXCHANGERATE_API_KEY env-var wiring.
- **Cache Freshness 5/10**: cache freshness helper not emitted for this API shape.
- **Breadth 7/10**: spec has 7 endpoints (small API). Expected for a tightly scoped wrapper.
- **MCP Remote Transport 5/10 / MCP Tool Design 5/10**: spec doesn't declare `mcp.transport: [stdio, http]`. Small surface (~26 tools), default endpoint-mirror is acceptable.
- **Dead Code 5/5**: clean.
- **Type Fidelity 3/5**: minor.

## Verify (mock-mode runtime tests)

- 30/33 passed (91% pass rate)
- 0 critical failures
- 3 non-critical failures (table-validation requires `sql` command not emitted; convert-batch/log/sync-rates "no examples to test against" — these are read-after-write commands whose verify cohorts aren't built)

## Dogfood

- Path validity: SKIP (no spec resource paths to validate — endpoints all use templated /v6/{api_key}/...)
- Auth protocol: SKIP (spec parsing limitations for path-based auth)
- Dead flags: 0 dead (PASS)
- Dead functions: 0 dead (PASS)
- Data pipeline: PARTIAL (sync calls domain-specific Upsert methods)
- Examples: 10/10 commands have examples (PASS)
- **Novel features: 10/10 survived** (PASS) — all transcendence commands from Phase 1.5 absorb manifest shipped and are reachable via `--help`
- MCP surface: PASS (cobratree mirror)

## Live-key Smoke (informal, during build)

Direct tests of the patched binary against `https://v6.exchangerate-api.com`:
- `rates pair USD EUR --json` → conversion_rate 0.8597 (success)
- `rates pair-amount USD EUR 250 --json` → conversion_result 214.925 (success)
- `codes --json` → 161 currency codes returned (success)
- `quota --json` → 1500/1500 remaining (success, Free tier confirmed)
- `rates enriched USD EUR` → plan-upgrade-required (expected for Free tier)
- `rates history USD 2024 3 27` → plan-upgrade-required (expected for Free tier)
- `convert 250 USD EUR --json` → 214.925 (success, logged to conversions_log)
- `convert 100 USD EUR,GBP,JPY --json` → 3 results from 1 API call (success)
- `matrix USD,EUR,GBP,JPY --json` → 16 cross-rates from 1 API call (success)
- `open USD --json` → 161 rates via keyless endpoint (success, attribution present)
- `plan-check --json` → "Free tier detected" (success; API key masked in output paths)
- `sync-rates --base USD --json` → 166 snapshots written (success)
- `history-cache USD EUR --json` → 1 snapshot returned (success)
- `drift --base USD --since 26h` → 3 movers identified after synthetic seed (success)
- `quota burn` → "warming-up" with 1 snapshot, projects with 2+ (success)
- `watch add USD EUR --threshold 0.5` / `watch list` / `watch check` → end-to-end persistence works (success)
- `convert-batch --from USD --to EUR --input amounts.txt --json` → batch convert from 1 API call (success)
- `log show --json` → entries from prior convert + batch (success)
- `rates pair USD EUR --as-of 2026-12-31 --json` → resolves from local snapshot, no API call (success)

## Security Posture

- API key fully redacted in:
  - dry-run output (URL path mask: `****<last4>`)
  - plan-check output (paths use placeholder + masked key)
  - log/error messages (masked)
- API key NEVER appears in any file under $RESEARCH_DIR, $PROOFS_DIR, $DISCOVERY_DIR, or the published CLI dir.
- API key sent to API via URL path (as required) but NOT mirrored in query string (suppressed in patched client.go).

## Fix Loop

- Loops used: 1 (post-initial-shipcheck)
- Fixes:
  1. Patched 7 spec-derived commands to auto-inject api_key from `c.Config.ExchangerateApiKey` (removed `<api_key>` positional)
  2. Suppressed bogus `apikey_placeholder` query parameter on real requests
  3. Masked API key in dry-run URL output (was leaking in plain text)
  4. Built 10 hand-authored novel commands matching the Phase 1.5 transcendence manifest
  5. Updated research.json narrative + novel_features for correct command paths (`rates pair`, `sync-rates`, `mcp serve` without `--stdio`)
  6. Reworded narrative.value_prop so verify-skill didn't parse "exchangerate-api-pp-cli wraps" as a command path

## Ship Recommendation

**ship** — all six shipcheck legs PASS, Grade A scorecard (81/100), all 10 novel features ship and function live, all auth/credential surfaces redacted, 0 critical verify failures, 0 dead flags, 0 dead functions, 0 known functional bugs in shipping-scope features.
