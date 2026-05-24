# Rappi shipcheck proof

## Pass

| Leg                 | Result | Notes                                             |
|---------------------|--------|---------------------------------------------------|
| dogfood             | PASS   | 18/18 commands, 100% pass rate, 0 critical        |
| verify              | PASS   | All runtime checks; auto-fix not required         |
| workflow-verify     | PASS   | No workflow manifest (single-source CLI)          |
| verify-skill        | PASS   | After fixing dropped `--cities` flag on coverage-diff |
| validate-narrative  | PASS   | After fixing `sync --city` → `sync` in quickstart |
| scorecard           | PASS   | 81/100 — Grade A                                  |

## Scorecard breakdown

| Dimension                | Score |
|--------------------------|-------|
| Output Modes             | 10/10 |
| Auth                     | 10/10 |
| Error Handling           | 10/10 |
| Doctor                   | 10/10 |
| Agent Native             | 10/10 |
| MCP Quality              | 10/10 |
| Local Cache              | 10/10 |
| Insight                  | 10/10 |
| Terminal UX              | 9/10  |
| Agent Workflow           | 9/10  |
| README                   | 8/10  |
| Vision                   | 8/10  |
| MCP Token Efficiency     | 7/10  |
| Breadth                  | 7/10  |
| Path Validity            | 7/10  |
| Data Pipeline Integrity  | 7/10  |
| MCP Remote Transport     | 5/10  |
| MCP Tool Design          | 5/10  |
| Cache Freshness          | 5/10  |
| Workflows                | 6/10  |
| Type Fidelity            | 3/5   |
| Dead Code                | 5/5   |

Note: `mcp_description_quality`, `mcp_surface_strategy`, `auth_protocol`, `live_api_verification` omitted from denominator.

## Fixes applied during shipcheck

1. **`stores coverage-diff --cities` example flag mismatch.** The narrative quickstart and SKILL recipe referenced `--cities ciudad-de-mexico,guadalajara` but the shipped command takes `--types` and `--since` only (coverage is per-store-type globally; `/tiendas/tipo/<type>` isn't city-scoped at the SSR layer). Removed `--cities` from research.json's narrative `quickstart` + `recipes` arrays and from the rendered README.md and SKILL.md.

2. **`sync --city ciudad-de-mexico` quickstart.** The narrative quickstart called sync with `--city`, but the generated sync command doesn't accept that flag (the spec's `restaurants.list_city` has `city` as a required path param, so sync skips it). Updated research.json's `narrative.quickstart` and the rendered README.md to plain `rappi-pp-cli sync`.

## Verdict

`ship` — all 6 shipcheck legs pass. No functional bugs in any shipping-scope feature. Ready to proceed to Phase 4.8 (agentic SKILL review), Phase 4.85 (output review), Phase 4.9 (README/SKILL/AGENTS correctness audit), Phase 5 (live dogfood), Phase 5.5 (polish), Phase 5.6 (promote and archive), and Phase 6 (next steps).

## Known gaps that DO NOT block ship

These are surfaced as documented gaps in `research.json` and the README; v1 ships without them by design:
- Menu items + prices (`rappi.com.mx` XHR endpoints unreachable from datacenter IPs)
- Cart, orders, account flows (require login; explicit non-goal)
- Mobile-app signed-request endpoints (off-limits per agent guidance)
- `restaurants near` and `restaurants by-neighborhood` require `--fetch-detail` to be set for accurate geo/address data (acknowledged in command Long help text)

## Scorecard drilldowns for polish targets

- `MCP Remote Transport: 5/10` — spec doesn't enable HTTP transport. Polish can add `mcp.transport: [stdio, http]` to spec, regenerate. Surface is small (7 tools) so this is low-priority.
- `Cache Freshness: 5/10` — sync writes to the store but no explicit freshness signal. Polish can add sync-time TTLs.
- `Workflows: 6/10` — no workflow_verify.yaml manifest. Polish could add a primary-workflow manifest (e.g., "sync → search → top").
- `Type Fidelity: 3/5` — generic html_link/html_extracted_page typing; novel commands use typed structs but generated endpoints don't.
- `Path Validity: 7/10` and `Data Pipeline: 7/10` — sub-areas with room for cleanup that polish can attack.
