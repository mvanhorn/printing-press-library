Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Netlify CLI Build Log

## Priority 0 + 1 (generator)
- 171 endpoint commands emitted from official Swagger 2.0 spec (converted to OpenAPI 3).
- Data layer: SQLite mirror; sync works (verified live: 284 records across 14 top-level resources).
- Cloudflare MCP pattern auto-applied (>50 endpoints): thin search+execute, endpoint tools hidden, transport [stdio,http].

## Generator fixes applied
- Spec: distributed 73 path-level parameter arrays into operations (Swagger→OA3 converter rejected path-level body params). RETRO candidate: generator should handle path-level params during conversion.
- sync.go: removed 2 duplicate switch cases (agent-runners, sites) that broke compile. RETRO candidate: generator emitted duplicate `responsePathForResource` cases.

## Priority 2 (transcendence — all hand-built, all shipping)
1. overview — cross-site health from local mirror (sites + form counts). BUILT.
2. env-drift — env var presence/context drift across owners. BUILT.
3. dns-audit — dangling NETLIFY records + expiring SNI certs (--cert-days). BUILT.
4. since <window> — deploy/build timeline via ParseDurationLoose. BUILT.
5. deploy-diff <a> <b> — side-by-side deploy field diff, mirror-first then live. BUILT.
6. submissions search <query> — offline substring search over submissions, --limit. BUILT.

All 6: verify-friendly RunE (help-only + dry-run + usageErr branches), missing-mirror hint, empty→valid JSON, mcp:read-only, route through printJSONFiltered (--select/--json/--csv). Verified live against empty account: exit 0, valid JSON, graceful.

## Deferred
- None. No stubs.
