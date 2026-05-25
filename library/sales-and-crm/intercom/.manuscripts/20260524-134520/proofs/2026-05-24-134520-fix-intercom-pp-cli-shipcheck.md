# intercom-pp-cli — Phase 4 Shipcheck

## Final verdict: **ship**

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | go build / vet / test all green; auto-fix loop did nothing |
| dogfood | PASS | path validity 6/6, dead flags 0, examples 10/10, novel features 4/4 survived, data pipeline GOOD |
| workflow-verify | PASS | no workflow manifest (no `workflow_verify.yaml`); skipped cleanly |
| verify-skill | PASS | after fixing the stale `--since 7d` reference in README cookbook and hiding the `contact` parent |
| validate-narrative | PASS | after splitting the `articles pull && articles push` example into single-command form |
| scorecard | PASS | **89/100, Grade A** |

## Scorecard breakdown

```
Output Modes         10/10
Auth                 10/10
Error Handling       10/10
Terminal UX          10/10
README               10/10
Doctor               10/10
Agent Native         10/10
MCP Quality          10/10
MCP Desc Quality     10/10
MCP Token Efficiency  4/10   ← lowest dimension; addressable via MCP enrichment in polish
MCP Remote Transport  5/10   ← addressable via mcp.transport: [stdio, http]
MCP Tool Design       5/10   ← addressable via mcp.orchestration: code
MCP Surface Strategy  2/10   ← addressable via mcp.endpoint_tools: hidden
Local Cache          10/10
Cache Freshness       5/10   ← addresses on first `sync` run (Phase 5)
Breadth              10/10
Vision                9/10
Workflows            10/10
Insight              10/10
Agent Workflow        9/10

Domain Correctness
Path Validity           10/10
Auth Protocol           10/10
Data Pipeline Integrity 10/10
Sync Correctness        10/10
Live API Verification   N/A
Type Fidelity           2/5
Dead Code               5/5

Total: 89/100 - Grade A
```

## Sample Output Probe (live command sample)

4/4 commands passed. All four novel transcendence commands (`incident-tag`, `articles pull`, `contact 360`, `conversations sla`) returned expected output shapes.

## Fixes applied during shipcheck (2 fix loops)

### Loop 1: 3 issues

1. **`articles pull && articles push` chained example** — validate-narrative failed because it parsed the chained command as one invocation, hitting `articles pull --from` (a flag pull doesn't have). Fixed by splitting into a single-command example with the push reference in the explanation prose. (research.json novel_features + novel_features_built + recipes)

2. **`conversations list --since 7d`** — recipe referenced a flag that doesn't exist on the generated `conversations list` command. The `--since` flag exists on `sync` (which dogfood-sync rendered with `--since 30d`) and on the hand-built `conversations sla`/`incident-tag` commands. Dropped from the recipe in research.json; hand-removed from the stale README cookbook line (dogfood doesn't re-render cookbook content).

3. **`contact` parent's `Use:` clause confused verify-skill** — the validator parses `Use: "contact"` as expecting 0 positional args; the example `contact 360 a placeholder email` has 2 (`360` and `mei@...`). Verify-skill self-flagged the analogous `auth login` shape as a likely false-positive but the 2-positional `contact 360 <key>` case didn't get the same tolerance. Fixed by setting `Hidden: true` and changing `Use:` to `"contact <subcommand> [args...]"`, matching the generator's convention for grouped parents.

### Loop 2: clean

All 6 legs pass.

## What's intentionally deferred to polish (Phase 5.5)

- **MCP surface enrichment.** Four MCP-shape dimensions score 2–5/10 because the generator emitted 133 endpoint mirrors as MCP tools (well over the 50-tool threshold). The recommended fix is to add `x-mcp:` to the spec with:
  ```yaml
  x-mcp:
    transport: [stdio, http]    # remote-capable
    orchestration: code         # thin <api>_search + <api>_execute pair
    endpoint_tools: hidden      # suppress raw per-endpoint mirrors
  ```
  Then re-run generate. Polish will surface this as the top opportunity; deferring rather than adding another regen loop here.
- **Cache Freshness 5/10.** The local DB hasn't been synced yet; first Phase 5 `sync` pass will move this dimension up automatically.
- **Type Fidelity 2/5.** Some endpoints take `oneOf/anyOf` bodies that fall back to `--body-json` (raw JSON). Documented at generation time and out of scope for this run — fixing requires hand-rolling typed body flags per endpoint, ~7 endpoints.

## Ship recommendation

**ship.** Verdict survives all 6 shipcheck legs, scorecard is Grade A at 89/100, all 4 approved novel features built and behaviorally verified. The MCP-surface dimensions are the only weak spots and they're enrichment-shaped, not bug-shaped — polish (Phase 5.5) or a follow-up regen with `x-mcp:` enrichment is the right place to close them.
