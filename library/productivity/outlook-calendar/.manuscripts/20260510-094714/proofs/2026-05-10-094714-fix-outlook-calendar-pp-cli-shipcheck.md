# Phase 4 Shipcheck — outlook-calendar-pp-cli

## Run order
- Initial run: 5/6 legs passed; `validate-narrative` failed on a stale recipe.
- Fix loop 1:
  1. Removed dead helper `extractResponseData` (`internal/cli/helpers.go`)
  2. Added `cliutil.AdaptiveLimiter` and typed `*cliutil.RateLimitError` to `internal/oauth/device.go` for all three Microsoft-Identity HTTP calls (devicecode, token poll, refresh)
  3. Renamed lingering `stale` → `pending` in `research.json` `narrative.value_prop` and `narrative.recipes`
  4. Verified all 8 transcendence commands return `[]` (not `null`) on empty stores
- Re-run: **6/6 legs PASS, Verdict: PASS**

## Final shipcheck verdict

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | 8/8 novel features survived; 0 dead helpers; sources rate-limited |
| verify | PASS | 26/26 commands PASS, 100% pass rate |
| workflow-verify | PASS | No workflow manifest declared |
| verify-skill | PASS | Flag names, commands, positional args, sections all clean |
| validate-narrative | PASS | 11 narrative commands resolved + full examples passed under `PRINTING_PRESS_VERIFY=1` |
| scorecard | PASS | Total 79/100 (Grade B) |

## Scorecard breakdown
```
  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX           9/10
  README                8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          10/10
  MCP Token Efficiency  7/10
  MCP Remote Transport  5/10
  MCP Tool Design       5/10
  Local Cache          10/10
  Cache Freshness       5/10
  Breadth               9/10
  Vision                8/10
  Workflows            10/10
  Insight              10/10
  Agent Workflow        9/10

  Domain Correctness
  Path Validity         10/10
  Auth Protocol          3/10  ← gap
  Data Pipeline          7/10
  Sync Correctness      10/10
  Type Fidelity          1/5   ← gap
  Dead Code              5/5
```

## Lingering gaps (not blockers)

### auth_protocol 3/10
The spec declares `auth.type: bearer_token` so the scorer's pattern-match on `auth.type == oauth2` fails, but the runtime flow IS OAuth 2.0 device-code (hand-built in `internal/oauth/device.go` + `internal/cli/auth_login.go`), and the README/SKILL document it honestly. Fix paths:
- Switch the spec to `auth.type: oauth2` with `oauth2_grant: authorization_code` + a token URL — the generator will then try to emit OAuth scaffolding that we'd need to override; the device-code-vs-authorization-code mismatch makes this risky.
- Status: leave as-is. Auth WORKS; the scorer line is misleading.

### type_fidelity 1/5
The internal-spec `Event` type is intentionally flat (id, subject, etc) — the deep nesting (start/end/location/attendees/responseStatus) is parsed by hand in `internal/cli/novel_events.go` because the generator's typed-struct emit doesn't model nested objects well in internal YAML. Fix paths:
- Migrate to OpenAPI spec (the upstream Microsoft Graph spec covers nested types) — risky given 35MB master and the personal-account `/me/*` filter we'd lose.
- Status: leave as-is. The CLI's actual behavior is correct; agents see the full payload via `--json`.

### MCP_remote_transport / MCP_tool_design 5/10
Default endpoint-mirror MCP surface. Could be enriched with `mcp.transport: [stdio, http]` + `mcp.orchestration: code` for richer tooling, but adds complexity for a 31-tool surface that isn't huge. Polish phase can revisit.

## Ship recommendation: **ship**

All ship-threshold conditions met:
- shipcheck exits 0
- verify 100%
- dogfood passes wiring checks
- verify-skill exits 0
- validate-narrative resolves
- scorecard 79 ≥ 65
- all 8 novel features built and behave (return `[]` on empty store; expected output shapes)

Two scorecard gaps documented as known but not blocking (auth_protocol pattern-match miss, type_fidelity from intentionally-flat internal spec).
