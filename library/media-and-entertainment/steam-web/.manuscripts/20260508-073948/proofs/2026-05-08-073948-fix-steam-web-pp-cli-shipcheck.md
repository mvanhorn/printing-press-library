# Steam Web — Shipcheck Proof

## Result

**Verdict: ship**

Shipcheck umbrella: **5/5 legs PASS** after one fix loop. Sample-probe: **11/11**.

| Leg | Result |
|---|---|
| dogfood | PASS — 77/77 tests, 100% pass rate |
| verify | PASS — auto-fix loop converged |
| workflow-verify | workflow-pass (no manifest, skipped) |
| verify-skill | PASS (canonical sections, all flag-paths resolve) |
| scorecard | PASS — 84/100, Grade A |

## Scorecard breakdown

```
  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          8/10
  README               8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          8/10
  MCP Token Efficiency 4/10  ← gap
  MCP Remote Transport 10/10
  MCP Tool Design      10/10
  MCP Surface Strategy 10/10
  Local Cache          10/10
  Cache Freshness      5/10
  Breadth              10/10
  Vision               8/10
  Workflows            8/10
  Insight              9/10
  Agent Workflow       9/10

  Path Validity        10/10
  Data Pipeline Integ.  7/10
  Sync Correctness     10/10
  Type Fidelity         1/5  ← gap (generator limitation)
  Dead Code             5/5

  Total: 84/100  (Grade A)
```

## Fixes applied

**Fix loop 1 (after first shipcheck pass):**

1. **`news search` FTS5 syntax error.** Sample-probe failure: `news search 'patch notes'` raised `SQL logic error: fts5: syntax error near "'"`. Root cause: free-form user queries can contain FTS5 special characters (single/double quotes, parens, colons, operators) that the FTS5 parser rejects. Fix: added `sanitizeFTSQuery(q)` in `internal/cli/novel_app_data.go` that strips FTS5 special characters and wraps multi-token input as a phrase. Also degrades gracefully — if the FTS5 parser still rejects (e.g., user types only special chars), the command returns an empty hit list with an `fts_query_invalid` note instead of a hard error.

**Fix loop 1 (Phase 4.8 SKILL review findings):**

2. **`iauthentication-service` cited as agent-mode demo (error finding).** Reviewer caught two examples in SKILL.md (`Agent Mode --select` example at line 534 and the `Named Profiles` recipe at line 586) that demoed `iauthentication-service begin-auth-session-via-credentials` — but the SKILL itself flags that namespace as "NOT for Web API auth" and "hidden from the MCP surface." Fix: replaced both with user-facing demos (`library audit … --select never_launched.appid,never_launched.name` and `--profile daily-flex rare-achievements --steamid …`).
3. **Truncated frontmatter description (warning).** SKILL.md `description:` field cut mid-sentence (`"…friend playtimes, achievement progress, and..."`) before the trigger phrases. Fix: shortened the lead-in to fit `"Steam Web CLI with throttled fan-out, achievement intelligence, and a local SQLite store for cross-library queries."` before the trigger phrases.
4. **"200 tools" / "first Go-native MCP" marketing-copy smell (warning).** README + SKILL headline cited specifics that aren't substantiated and don't age well. Fix: rewrote to describe the actual MCP shape (stdio + http transports, `steam_web_search` + `steam_web_execute` orchestration pair).
5. **IAuthenticationService visibility narrative (warning).** Auth section claimed those endpoints are "hidden from the MCP surface" — accurate for the MCP catalog but the CLI subcommand is still reachable. Fix: clarified to "remain reachable via the CLI subcommand for completeness but are hidden from the MCP tool catalog and should not be used as part of normal Web API workflows."

## Remaining gaps (non-blocking)

- **`mcp_token_efficiency 4/10`.** Even after applying the Cloudflare pattern (`endpoint_tools: hidden`), the scorecard's MCP token efficiency dimension still reads low. The orchestration `steam_web_search`/`steam_web_execute` pair plus the 11 novel commands plus the framework default tools still surface as a non-trivial catalog. This is a polish target — possibly `mcp:hidden` annotations on a few framework tools that don't add value in MCP context (e.g., `feedback`, `which`, `import`, `tail`, `analytics`) would push this up. Polish skill will tackle this.
- **`type_fidelity 1/5`.** Known generator limitation — the data layer is the generic `resources(id, resource_type, data JSON)` table with FTS5 over the JSON blob, not typed columns per entity. Brief proposed 9 typed entities; novel commands work around it via `resource_type:<scope>` keys and JSON parsing on read. Retro candidate.
- **`MCP: 169 tools (0 public, 169 auth-required)`.** Steam's `auth.type: api_key` is global, so every endpoint mirror is flagged auth-required even when (e.g., `GetServerInfo`) it isn't. OpenAPI's per-operation `no_auth: true` tagging convention isn't fully wired for `apiKey` schemes; retro candidate.

## Verdict rationale

All ship-threshold conditions met:
- Shipcheck umbrella exit 0, all 5 legs PASS.
- Verify pass rate 100% with 0 critical failures.
- Dogfood 77/77 tests, no spec parse / binary path / skipped example failures, no command/config wiring bugs.
- Workflow-verify `workflow-pass`.
- Verify-skill exit 0 — every flag and command path in SKILL.md resolves to the shipped CLI.
- Scorecard 84/100 (>= 65), no flagship feature returning wrong/empty output (sample probe 11/11).

No `Known Gaps` README block needed (verdict is `ship`, not `ship-with-gaps`).
