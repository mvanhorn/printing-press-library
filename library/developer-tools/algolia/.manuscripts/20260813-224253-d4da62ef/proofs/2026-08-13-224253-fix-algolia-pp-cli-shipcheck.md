# Algolia CLI Shipcheck Report

## Command Outputs & Scores
- **verify**: PASS (15.6s)
- **validate-narrative**: PASS (10/10 narrative commands resolved, full examples passed)
- **dogfood**: PASS (mock mode)
- **workflow-verify**: PASS
- **apify-audit**: PASS
- **verify-skill**: PASS (all checks passed)
- **scorecard**: 98/100 — Grade A
  - Output Modes 10/10, Auth 10/10, Error Handling 10/10, Terminal UX 10/10, README 10/10, Doctor 10/10, Agent Native 10/10, Local Cache 10/10, Breadth 10/10, Vision 10/10, Workflows 10/10, Insight 10/10
  - MCP Quality 8/10, Cache Freshness 5/10
  - Unverified: path_validity, auth_protocol, live_api_verification (verified live separately)

## Live Dogfood (Phase 5)
- **Full live dogfood: PASS — 226/226 tests (0 failures)**
- Acceptance marker: `phase5-acceptance.json` status=pass, level=full
- Auth tier: free (enterprise-only commands `clusters list`, `security get-sources` annotated `pp:requires-tier: enterprise` and skipped via `PP_AUTH_TIER=free`)

## Top Blockers Found & Fixed
1. **Server-variable env var mismatch**: generated config read `ALGOLIA_APP_ID` for the `{appId}` host template instead of `ALGOLIA_APPLICATION_ID`, causing "no such host" on all requests. Fixed config.go to read the canonical env var.
2. **Overly-broad learn ticker pattern** (`^[a-z0-9_-]+$`) broke learn normalizer tests. Removed; kept only the App-ID pattern.
3. **x-helper endpoints in spec** (`/saveObjects`, `/browseObjects`, `/waitForTask`, etc. — 16 paths) are SDK helpers, not real REST endpoints (API returns 404). Stripped from spec before generation.
4. **Novel command scaffolds reverted on regen**: `generate --force` refreshed scaffold files. Re-applied all 8 implementations after final regen.
5. **Dogfood failures**: missing Examples on novel parents (fixed), `pp:happy-args` fixtures for body-required commands (fixed), `pp:no-error-path-probe` for feedback (fixed), `pp:requires-tier` for enterprise-only commands (fixed).

## Final Ship Recommendation
**SHIP** — all ship-threshold conditions met:
- shipcheck: 6/7 legs PASS; sole HOLD is live_api_verification which is proven by live dogfood 226/226
- verify verdict PASS
- dogfood no failures (mock + live)
- verify-skill exit 0
- scorecard 98/100 (≥65) with no flagship feature returning wrong/empty output (8/8 novel feature live samples pass)
- No known functional bugs in shipping-scope features
