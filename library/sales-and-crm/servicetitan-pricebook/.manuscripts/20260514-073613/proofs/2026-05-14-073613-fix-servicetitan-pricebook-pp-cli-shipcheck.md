# servicetitan-pricebook-pp-cli — Shipcheck

## Result: PASS (6/6 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | After fixing 1 dead helper (see below). novel_features_check: planned 12 / found 12. |
| verify | PASS | Runtime command matrix clean. |
| workflow-verify | PASS | `workflow-pass`. |
| verify-skill | PASS | After fixing stale command paths (see below). |
| validate-narrative | PASS | 10 narrative commands resolved, full examples pass under PRINTING_PRESS_VERIFY=1. |
| scorecard | PASS | **86/100 — Grade A.** |

## Blockers found and fixed

### 1. validate-narrative + verify-skill: stale command paths in the narrative
The `research.json` narrative (and the generated SKILL.md / README.md) referenced commands that don't exist on the shipped CLI:
- `markup-audit --tolerance 0.05` — `--tolerance` is valid, but `0.05` was a nonsense value (the flag is in percentage points; default 5). Changed to `--tolerance 5` everywhere (markup-audit + reprice examples, quickstart, recipe).
- `materials list` — the generated subcommand is `materials get-list`, not `materials list`. The recipe also omitted the required `<tenant>` positional. Fixed to `materials get-list 42 --active True ...` (matching the generator's own `42` example-tenant convention).
- A `sync && markup-audit ...` compound recipe — `&&` does not survive the validate-narrative single-command walk. Split into a single `markup-audit` command; the explanation notes the sync prerequisite.
Fixed in `research.json`, `SKILL.md`, and `README.md`. Both legs re-run clean.

### 2. dogfood WARN: 1 dead helper function
`extractResponseData` in the generated `internal/cli/helpers.go` was emitted but never called. Removed; `go build` clean, dogfood re-run shows 0 dead functions, verdict PASS.

## Scorecard 86/100 — Grade A

Strong: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10, Local Cache 10, Breadth 10, Insight 10, MCP Remote Transport / Tool Design / Surface Strategy all 10, Path Validity 10, Sync Correctness 10.

## Known gap (not a blocker — deferred to Phase 5.5 polish)

**`mcp_token_efficiency 0/10`** — `.printing-press.json` records `mcp_tool_count: 40`. The spec's `x-mcp.endpoint_tools: hidden` correctly suppressed the *typed endpoint-mirror* tools, and code orchestration (`<api>_search` + `<api>_execute`) is wired — but the runtime **cobratree mirror** (`cobratree.RegisterAll`) still walks every user-facing Cobra command, re-exposing the 40 endpoint commands as MCP tools. The intended fix is `cmd.Annotations["mcp:hidden"] = "true"` on the generated endpoint commands so the mirror skips them — a ~40-file change that is polish-scoped, not a Phase 4 blocker. This is also a genuine generator gap (`endpoint_tools: hidden` should propagate to the cobratree walker) and is a retro candidate.

Sample Output Probe reported "binary is not executable" — a Windows `.exe` path quirk in the scorecard probe, not a CLI defect; the binary runs fine.

## Verdict: ship

All six shipcheck legs PASS, scorecard 86/100 Grade A, novel_features 12/12 built, every flagship novel command verified to return correct output against the live JKA pricebook (markup-audit math, warranty-lint rules, vendor-part-gaps, health rollup). The single known gap (`mcp_token_efficiency`) is documented and routed to polish; it does not affect CLI or MCP correctness, only the size of the MCP tool surface.
