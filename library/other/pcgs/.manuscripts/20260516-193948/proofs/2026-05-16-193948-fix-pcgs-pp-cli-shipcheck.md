# PCGS-pp-cli shipcheck proof

## Final verdict: PASS (6/6 legs)

| Leg | Result | Elapsed |
|-----|--------|---------|
| dogfood            | PASS | 1.28s |
| verify             | PASS | 4.02s |
| workflow-verify    | PASS | 15ms |
| verify-skill       | PASS | 223ms |
| validate-narrative | PASS | 537ms |
| scorecard          | PASS | 60ms |

## Scorecard 86/100 — Grade A

Dimension scores: Output Modes 10/10, Auth 10/10, Error Handling 10/10, Terminal UX 9/10, README 8/10, Doctor 10/10, Agent Native 10/10, MCP Quality 8/10, MCP Tool Design 5/10, MCP Remote Transport 10/10, MCP Token Efficiency 7/10, Local Cache 10/10, Cache Freshness 5/10, Breadth 7/10, Vision 7/10, Workflows 8/10, Insight 8/10, Agent Workflow 9/10, Path Validity 10/10, Auth Protocol 10/10, Data Pipeline 7/10, Sync 10/10, Type Fidelity 4/5, Dead Code 4/5.

Omitted from denominator: mcp_description_quality, mcp_surface_strategy, live_api_verification.

## Fix loop summary

Two loops of doc-drift fixes (not code fixes):
1. README.md / SKILL.md still referenced renamed-or-killed commands (`quota` standalone → `--quota` flag; `stale` → `refresh --dry-run`; `coin ingest` → `coin batch --list-certs`; `sync --max-age` → `refresh --older`; `coin facts cert` space-form → `coin facts-cert` kebab).
2. validate-narrative needed an in-repo `examples-pcgs-coin-list.csv` so example commands could execute. The bare `pcgs-pp-cli --quota --json` quickstart line had no subcommand-words for the validator; swapped to `pcgs-pp-cli audit --since 24h --json` which exercises a real subcommand AND emits the stderr quota line via PostRunE.

One real code fix: `internal/cli/coin_batch.go` was wrapping nil errors with `fmt.Errorf("batch: %w", ...)` on the success path, producing `Error: batch: %!w(<nil>)` after the actual JSON output. Refactored to `if err != nil { return wrap(err) } return nil`.

## Final shipped surface

- 6 novel Cobra commands: `coin batch`, `coin pop-curve`, `refresh`, `audit`, `order hydrate`, `search`
- 12 absorbed endpoints: all spec-emit (coin facts/apr/images cert/grade/barcode, banknote facts/images cert/grade, order submission/range)
- Universal quota: stderr quota line on every API-touching command; global `--quota` and `--quota-only` flags
- Typed exit codes: 0/2/3/4/5/7 per PCGS error envelope
- Auth: bearer token in `PCGS_AUTH_TOKEN` env var
- Local cache + `lookup_log` audit table + `coin` resource store
- 17 SKILL recipes covering primary workflows + 10 compose-with-other-data patterns (bullion, NGC, Heritage, eBay, FRED CPI, Set Registry, etc.)

## Ship recommendation

`ship`. All shipcheck legs PASS. Phase 5 live dogfood next (the user's $PCGS_AUTH_TOKEN is available so live testing will work).
