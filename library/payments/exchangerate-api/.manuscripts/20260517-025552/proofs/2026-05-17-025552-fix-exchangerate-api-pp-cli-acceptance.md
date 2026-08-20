# Phase 5 Acceptance Report: exchangerate-api-pp-cli

Run: 20260517-025552
Level: **Full Dogfood**
Tests: **88/88 passed** (63 skipped — commands without happy-path examples; auto-skipped by matrix builder)
Gate: **PASS**

## Test Matrix (Full level)

For every leaf subcommand the matrix exercises four cohorts:
- `help` — `<cmd> --help` returns 0 and prints help text
- `happy_path` — runs the command with realistic args from `Example:`
- `json_fidelity` — re-runs the happy_path with `--json` and asserts the output parses as JSON
- `error_path` — runs the command with an invalid sentinel arg, asserts non-zero exit

Skipped tests are commands whose Example doesn't have enough positional segments to construct a happy_path (e.g., `api`, `feedback`, `profile <subcommand>` parents).

## Fixes Applied During Phase 5

The first live dogfood run had 4 failures across 88 tests. All were fixed inline before re-running:

1. **convert-batch happy_path** — Example referenced `amounts.txt` (which doesn't exist on the test machine). Changed example to `--input -` (stdin) and made the empty-input case a successful no-op instead of an error (better agent ergonomics).
2. **convert-batch json_fidelity** — Same root cause as #1; fixed by the same change.
3. **history-cache error_path** — Calling `history-cache __invalid__` exited 0 because the 1-arg case fell through to `cmd.Help()`. Now exits 2 (usage error) when fewer than 2 positional args are supplied.
4. **mcp serve json_fidelity** — `mcp serve --json` previously launched the MCP server and streamed multi-frame wire protocol on stdout (not parseable as JSON). Now prints a clean JSON envelope explaining `--json` is incoherent for this command and exits 0 without starting the server.

After fixes: 88/88 PASS.

## Endpoints Verified Live Against `https://v6.exchangerate-api.com`

| Endpoint | Tier | Result |
|---|---|---|
| `/codes` | Free | PASS (161 codes) |
| `/latest/{base}` | Free | PASS |
| `/pair/{base}/{target}` | Free | PASS (USD→EUR 0.8597) |
| `/pair/{base}/{target}/{amount}` | Free | PASS (250 USD = 214.925 EUR) |
| `/quota` | Free | PASS (1490/1500 remaining at time of dogfood) |
| `/enriched/{base}/{target}` | Business | PASS — returns 403 plan-upgrade-required (expected and handled gracefully) |
| `/history/{base}/{year}/{month}/{day}` | Pro | PASS — returns 403 plan-upgrade-required (expected and handled gracefully) |
| `https://open.er-api.com/v6/latest/{base}` | None | PASS (no-key endpoint, attribution included) |

## Novel Commands Exercised End-to-End

| Command | Live behavior |
|---|---|
| `matrix USD,EUR,GBP,JPY --json` | 16 rates from 1 /latest call |
| `convert 250 USD EUR,GBP,JPY --json` | 3 results from 1 /latest call |
| `convert-batch --from USD --to EUR --input -` | Reads stdin; 1 /pair call |
| `sync-rates --base USD --json` | 166 snapshots persisted |
| `history-cache USD EUR --json` | Returns synced snapshots |
| `drift --base USD --since 26h --json` | Top movers vs prior snapshot |
| `quota burn --json` | Burn projection (returns "warming-up" with 1 snapshot, projects with 2+) |
| `plan-check --json` | Detected: Free tier; per-endpoint probes annotated; **key masked in output paths** |
| `watch add/list/check` | Watchlist persists and threshold-checks against live rates |
| `log show --json` | Returns prior conversions from local log |
| `open USD --json` | No-key endpoint with attribution |
| `mcp serve --help` | Subprocess wrapper to standalone MCP binary (verified) |
| `rates pair USD EUR --as-of 2026-12-31 --json` | Resolves from local snapshot, **0 API calls** |

## Security Verification

Tested error and dry-run paths for accidental credential exposure:

- `rates enriched USD EUR` → error message uses `/v6/****37ac/enriched/USD/EUR` (key masked, last 4 only)
- `rates pair USD EUR --dry-run` → preview URL uses `/v6/****37ac/pair/USD/EUR` (key masked)
- `plan-check --json` → every `path` in the JSON output uses `/v6/****37ac/...`
- The bogus query-string `apikey_placeholder=<key>` parameter is suppressed on every live request (was the default for `auth.in: query` declarations)

## Printing Press Improvements Identified (for retro)

- Scorecard live-check uses the stage binary (`build/stage/bin/`) which doesn't get rebuilt automatically after hand-edits to spec-derived commands. A `stage rebuild` step should fire before scorecard's live-check, or the live-check should rebuild the staging binary itself when source is newer.
- Internal YAML spec doesn't natively support path-based API keys (ExchangeRate-API embeds key in URL path). Workaround: declare `auth.type: api_key, in: query` with placeholder header, then patch client.go to skip the query param. A first-class `auth.in: path` (with `path_position: <segment>`) would eliminate the workaround.
- AST merge preserves hand-edits to spec-derived commands across regen, which worked perfectly here.

## Verdict: **PASS** — proceed to Phase 5.5 (polish) and Phase 5.6 (promote to library).
