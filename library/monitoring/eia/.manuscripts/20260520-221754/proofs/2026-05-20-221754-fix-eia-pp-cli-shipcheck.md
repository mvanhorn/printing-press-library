# EIA CLI Shipcheck

**Run:** 20260520-221754 | **API:** EIA APIv2 | **Binary:** `eia-pp-cli`

## Gates (all PASS)

| Gate | Result |
|---|---|
| `go mod tidy` | PASS |
| `govulncheck ./...` | PASS (gate during generate) |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | PASS — 6 packages green, no failures |
| binary build (`eia-pp-cli`) | PASS — 18 MB darwin/arm64 |
| `--help` | PASS — 23 top-level commands |
| `version` | PASS — eia-pp-cli 1.0.0 |
| `doctor` | PASS — config ok, env auth detected, API reachable |

## Dogfood (mock mode)

```
Path Validity:    3/3 valid (PASS)
Auth Protocol:    MATCH (apiKey query)
Dead Flags:       1 dead (WARN) — allowPartialFailure (generic boilerplate)
Dead Functions:   3 dead (WARN) — partialFailure helpers (generic boilerplate)
Data Pipeline:    PARTIAL — series_fts is direct SQL (intentional FTS5 path)
Examples:         10/10 commands have examples (PASS)
Novel Features:   12/12 survived (PASS)
MCP Surface:      PASS
Verdict:          WARN — dead-code is generic generated boilerplate
```

The dead-code warnings are unused partial-failure helpers from the
generator's standard set. EIA's data endpoints don't have a partial-
failure envelope, so the helpers were emitted but never invoked. They
do not affect correctness and are common across docs-derived CLIs.

## Scorecard

```
Output Modes         10/10
Auth                 10/10
Error Handling       10/10
Terminal UX           8/10
README                8/10
Doctor               10/10
Agent Native         10/10
MCP Quality          10/10
MCP Token Efficiency  7/10
MCP Remote Transport  5/10
MCP Tool Design       5/10
Local Cache          10/10
Cache Freshness       5/10
Breadth               7/10
Vision                7/10
Workflows             6/10
Insight               8/10
Agent Workflow        9/10

Domain Correctness
  Path Validity           10/10
  Auth Protocol            4/10
  Data Pipeline Integrity  7/10
  Sync Correctness        10/10
  Type Fidelity            3/5
  Dead Code                3/5

Total: 77/100 — Grade B
```

## Behavioral verification (real EIA API)

Each priority command was exercised against `https://api.eia.gov/v2/`
with a live API key.

| Command | Result |
|---|---|
| `doctor` | env auth detected; api reachable |
| `electricity retail-price TX --latest` | 2026-02 TX ALL 10.39 ¢/kWh |
| `electricity rto ERCO --fuel-mix --hours 6` | NG 32204.7 MWh leading, WND 12682.5, COL 6887.7, SUN 4996.5, NUC 3844.8, BAT 2720.5 |
| `electricity generation TX --fuel-type natural-gas --months 3` | 12 sector rows for 2026-02 returned |
| `natgas price henry-hub --last 5` | 5 daily Henry Hub prints; 2026-05-18 = $3.07/MMBtu |
| `natgas price spot --state TX --months 3` | multi-process rows returned (PRS, PG1, PIN, etc.) |
| `petroleum price crude wti --length 5` | 5 daily WTI Cushing prints; 2026-05-18 = $112.25/bbl |
| `petroleum price crude brent` | works against `RBRTE` series |
| `steo forecast --series oil --months 3` | WTIPUUS forecast 2026-05..2026-07 returned |
| `co2 TX --sector electric-power --years 1` | EC sector by fuel: CO 88.7, NG 97.2, PE 0.2, TO 186.2 MMT |
| `series sync --meta-only` | 9 routes synced into ~/.local/share/eia-pp-cli/series.db |
| `series search 'henry hub'` | natural-gas-futures returned with FTS snippet |
| `series list` | 9 series-meta rows printed |

## Gaps logged for v0.2

- Multi-value facets: the underlying generic client takes
  `map[string]string`, so each command pins one facet value per call. A
  future client variant could accept `map[string][]string` for the
  full EIA facets[xxx][] semantics.
- `series sync` data-points pull tops out at 50 rows per route; an
  `--since` parameter would unblock larger backfills.
- Auth protocol classification: the generator records "unknown prefix"
  because the spec uses an `apiKey` query scheme rather than Bearer
  or HTTP Basic. Real auth works (api_key is injected); only the
  scorecard's auth-protocol detector is partial. Improvement requires
  a generator-side change, not a CLI-side change.
