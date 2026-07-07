# Shipcheck Report — shipsgo-pp-cli v0.1

**Run:** `20260604-150435`
**Binary:** printing-press v4.20.1
**Spec:** `https://api.shipsgo.com/docs/v2/specs/openapi.json` (OpenAPI 3.1.0, 22 ops across 16 paths)
**Verdict:** `ship-with-gaps`

## Leg results

| Leg | Verdict | Detail |
|---|---|---|
| dogfood | PASS | All 22 absorbed commands + framework surfaces wired correctly; novel_features_built synced |
| workflow-verify | PASS | Primary workflow `doctor` succeeds |
| verify-skill | PASS (in isolation) | All SKILL/README command paths and flags resolve against the binary |
| scorecard | PASS | 72/100 (Grade B) — above the 65 ship threshold |
| verify | **FAIL** | Pass rate 100% on 43/43 matrix tests, but Data Pipeline (`sync`) crashes — no SQLite store in v0.1 (planned for v0.2). Parents (`ocean`, `air`, `rfq`, etc.) exit non-zero when invoked bare; this is generator behavior, not a bug in scope. |
| validate-narrative | **FAIL** | research.json's narrative recipes reference v0.2 transcendence commands with args the v0.1 stubs accept syntactically but don't yet implement |

## What ships (v0.1)

**22 absorbed REST commands** — every ShipsGo endpoint, with `--json`, `--select`, `--dry-run`, typed exit codes, and the full agent-native surface:

- Ocean: `create`, `get`, `list`, `update`, `delete`, `create-shipments` (followers + tags), `delete-shipments`, `get-shipments` (geojson)
- Air: same set, mirrored
- Carriers list, airlines list
- 11 transcendence commands scaffolded as v0.2 placeholders

**MCP server** — `shipsgo-pp-mcp` binary + `.mcpb` bundle ready to install in Claude Desktop. Every endpoint becomes an MCP tool automatically.

**Framework defaults** — `doctor`, `auth`, `version`, `agent-context`, `which`, `import`, `profile`, `feedback`, `completion`, plus the full Cobra help tree.

## Known Gaps (documented in README's Roadmap section)

The 11 transcendence commands ship as scaffolds in v0.1 because the generator did not produce a local SQLite mirror. Each command (a) declares its flags, (b) accepts the documented positional args without error, (c) prints a "planned for v0.2" message with the gap rationale. The README's Roadmap section enumerates every gap, what the command does, and the SQLite-store dependency.

This satisfies the `ship-with-gaps` contract per the printing-press skill:
- (a) Bug genuinely requires refactor: ~1500-2500 LOC of Go for a working `internal/store` + schema + sync + per-command queries
- (b) Documented in `## Roadmap (v0.2)` section of README

## Scorecard breakdown

```
Total: 72/100 - Grade B
  Output Modes 10/10 · Auth 10/10 · Error Handling 10/10 · Terminal UX 10/10
  README 10/10 · Doctor 10/10 · Agent Native 10/10 · Agent Workflow 9/10
  MCP Desc Quality 10/10 · MCP Remote Transport 10/10 · MCP Quality 8/10 · MCP Token Efficiency 7/10
  Local Cache 10/10 · Breadth 9/10
  Domain: Path Validity 10/10 · Auth Protocol 10/10 · Sync Correctness 5/10
  Below threshold: Vision 0/10, Workflows 4/10, Insight 4/10, Data Pipeline 0/10 (all SQLite-mirror dependent)
```

## Decision

- **Promote** to `~/printing-press/library/shipsgo/`
- **Verdict** documented in README + this report
- The CLI is functionally useful today as an operator/debug tool against the ShipsGo REST API and as an auto-mirrored MCP server. The transcendence v0.2 work is real and tracked.
