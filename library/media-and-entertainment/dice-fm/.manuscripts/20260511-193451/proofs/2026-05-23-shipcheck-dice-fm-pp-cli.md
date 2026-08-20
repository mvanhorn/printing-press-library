# DICE FM CLI — Shipcheck Report

**Run:** 20260511-193451 (resumed) · **Binary:** cli-printing-press v4.12.0 · **Date:** 2026-05-23

## Verdict: `ship` (with documented no-token gap)

## Shipcheck umbrella — 6/6 legs PASS
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS (10 narrative commands resolved + full examples) |
| dogfood | PASS (novel_features_check: planned 7 / found 7) |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS — **77/100, Grade B** |

## What was built
GraphQL CLI for the DICE Partners API. The generator's auto-GraphQL path assumes root-level `nodes` connections; DICE uses `viewer { conn { edges { node } } }` with typed `where` inputs and deeply nested objects, so the GraphQL **data layer was hand-authored** against the generated transport (`client.PostQueryWithParams`) and store:
- `internal/cli/dice_query.go` — viewer/edges paginator, per-entity field fragments, where-builders, node-by-id.
- `internal/cli/dice_resources.go` — 8 absorbed live commands (events list/get, tickets, orders, returns, transfers, extras, genres) with `--event`/`--state`/`--fan-phone`/`--separate-barcode` filters.
- `internal/cli/sync.go` (rewritten) — paginated sync of all 7 connections into the store + derived `fans` table; `--since`/`--full`/`--latest-only`/`--dry-run`.
- 7 transcendence commands (`door list`, `revenue summary`, `velocity show`, `fans repeat`, `fans top`, `fans optin`, `returns anomalies`).
- Reused generated scaffolding: bearer-token auth (`DICE_FM_TOKEN`), SQLite store + FTS, `search`/`sql`/`context`/`doctor`, MCP cobratree mirror, README/SKILL.

**Behavioral verification:** 6 store-backed transcendence commands verified with seeded-store unit tests (`dice_transcend_test.go`, exact-value assertions). `door list` + all live read commands are logically correct against the documented SpectaQL schema but **unverified against the live API** — no `DICE_FM_TOKEN` was available (Phase 5 live dogfood skipped per the auth-required-no-credential rule).

## Known gaps (non-blocking)
1. **Live API unverified** — no token. GraphQL query shapes match the documented schema; live correctness should be confirmed once a `DICE_FM_TOKEN` (from MIO/AMP) is available (`doctor`, then `sync --latest-only`).
2. **`path_validity 0/10`** — scorer drag, not a real defect (see retro candidates).
3. **`dead_code 0/5`** — orphaned helper funcs in generated `helpers.go` after deleting the broken generated endpoint commands; polish to address.
4. **PM/write framework commands** (`load`, `stale`, `orphans`, `import`) are generic scaffolding that don't fit a read-only event-ticketing API; candidates to hide in polish.

## Retro candidates (machine improvements)
- **Generator GraphQL shape assumption**: the auto-GraphQL templates (`graphql_queries.go.tmpl`) only emit root-level `field { nodes { flat-scalars } }`. APIs whose connections live under a `viewer`/root wrapper and use Relay `edges { node }` with nested objects + typed `where` inputs (DICE, and many partner GraphQL APIs) cannot use the generated query layer at all — it must be wholly hand-replaced. Consider (a) supporting the viewer/edges connection shape, or (b) a documented "GraphQL scaffold-only" mode that emits transport+store+framework without broken per-endpoint commands.
- **GraphQL SDL parser brittleness**: a well-formed SDL with `viewer`-nested connections + `edges` produced zero resources (parser only walks root `Query` fields for `*Connection` types with `nodes`). 
- **Scorecard `path_validity` is REST-only**: it scans for `path := "..."` literals matching spec paths; GraphQL CLIs (single `/graphql` path, hand-authored data layer) score 0 spuriously. Consider relaxing/​N/A for GraphQL specs.
- **`scorecard` JSON lacks per-dimension detail** (`reason`/`detail`) — hard to diagnose a 0 score programmatically.
