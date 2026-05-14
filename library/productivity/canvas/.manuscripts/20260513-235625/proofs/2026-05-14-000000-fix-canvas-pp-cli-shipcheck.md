# Canvas LMS CLI — Shipcheck Report

## Phase 4 Result: PASS (6/6 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | 5/5 path valid, 0 dead flags, 8/8 novel survived |
| verify | PASS | 100% pass rate (WARN threshold 80%) |
| workflow-verify | PASS | workflow-pass verdict |
| verify-skill | PASS | 0 mechanical mismatches |
| validate-narrative | PASS | 10/10 commands resolved |
| scorecard | PASS | 83/100 Grade A |

## Scorecard Summary
- Total: 83/100 Grade A
- Gaps: mcp_token_efficiency 4/10 (356 raw endpoint tools exposed), type_fidelity 1/5 (Swagger 1.2→OpenAPI3 loses some type info)
- Notable: auth 10/10, local cache 10/10, workflows 10/10, insight 10/10, agent workflow 9/10

## Phase 5 Result: PASS (with blocked fixtures)

| Test | Result |
|------|--------|
| doctor --json | PASS — configured, API reachable |
| sync | PASS — 53 records synced across 8 resources |
| courses list-your | PASS — 10 courses returned |
| pressure --json | PASS — empty (all courses access-restricted, expected) |
| impact --json | PASS — empty (no active enrollments) |
| drift --json | PASS — first-run snapshot created |
| search | PASS — FTS index functional |
| 28 complex-body commands | BLOCKED_FIXTURE — require instructor access to active courses |

## Known Issues
1. **CANVAS_BASE_URL must include `/api`**: `https://canvas.txstate.edu/api` not just the hostname. Auth narrative updated.
2. **MCP surface 356 tools**: large for agent context. Cloudflare pattern (code orchestration) not implemented — would require re-generation.
3. **type_fidelity 1/5**: Swagger 1.2→OpenAPI3 conversion loses some `integer`/`array` type specificity.

## Build Notes
- SQLite driver: `ncruces/go-sqlite3` v0.22.0 (pure Go WASM, no CGo) — required due to Apple Claude Code sandbox blocking CGo compilation
- Build flags: no special tags needed (WASM embedded via `embed` package)
- Binary size: 21MB (includes 3MB WASM SQLite blob)

## Verdict: SHIP
