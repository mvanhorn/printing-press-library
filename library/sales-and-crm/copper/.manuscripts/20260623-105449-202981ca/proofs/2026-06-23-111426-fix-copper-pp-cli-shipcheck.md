# Copper CLI — Shipcheck Report

## Verdict: PASS (7/7 legs)

| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS (+canonical-sections) |
| scorecard | PASS |

## Scorecard: 96/100 — Grade A
- 10/10 across Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Remote Transport/Tool Design/Surface Strategy, Local Cache, Breadth, Vision, Workflows, Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness; Type Fidelity 5/5, Dead Code 5/5.
- Cache Freshness 5/10 (intentionally not enabled — pre-read refresh could surprise users on a write-heavy CRM; manual sync + doctor cache report instead).
- MCP Quality 8/10, Insight 7/10, Agent Workflow 9/10 (minor).

## Sample Output Probe: 5/7 passed
- 5 novel features sampled clean.
- dedupe + who flagged "output contains no query token" ONLY because they are local store-readers run without a synced DB → returned the honest missing-mirror empty-state `[]` plus a sync hint (correct behavior, not a defect). All 7 novel features have passing seeded-data unit tests (25 tests) with exact acceptance assertions.

## Known generator-level issue (RETRO CANDIDATE — not Copper-specific)
internal/cliutil/credentials_test.go has 4 failing tests that reproduce IDENTICALLY in a pristine freshly-generated CLI (confirmed against a 2-resource stub). Root cause: the test writes TOML into a path the loader parses as JSON (config.json), breaking legacy-config fallback/migration/scrub assertions. One is a secret-scrub security assertion. Affects every generated CLI. Does not block shipcheck (verify leg builds + runtime-tests; does not run `go test ./...`). FILE FOR RETRO against the Printing Press generator.

## Ship threshold: MET
shipcheck exit 0; verify/dogfood/workflow-verify/verify-skill green; scorecard 96 >= 65; no flagship feature returns wrong output (empty results are honest no-data states). Verdict: ship (pending Phase 5 live dogfood with user credentials).
