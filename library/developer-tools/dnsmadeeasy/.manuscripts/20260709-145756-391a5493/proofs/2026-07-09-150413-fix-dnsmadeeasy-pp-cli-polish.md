# DNS Made Easy CLI — Polish (Phase 5.5)

Scorecard 94 -> 95. Verify 100% (61->62 cmds). Dogfood FAIL -> PASS. gosec hand-auth 4 -> 0. tools-audit 5 -> 0 pending.

Fixes: registered orphaned `auth set-token`; aligned root Short/Long with narrative; #nosec G505 (API-mandated HMAC-SHA1);
explicit `_ = s.Close()` (G104); 5 MCP-grade tool descriptions (MCP Desc 7->10); bulk-apply/acme-purge --dry-run now
reflect requested params + valid JSON.

Deferred (generator/spec-level, retro candidates): MCP Tool Design 5/10 (spec mcp.intents), Cache Freshness 5/10
(no freshness helper emitted for this API shape), ~30 gosec findings in generator-emitted DO-NOT-EDIT files.

ship_recommendation: ship. further_polish_recommended: no.
Remaining: live matrix not exercised (needs user API token) — parent-owned gate before publish.
