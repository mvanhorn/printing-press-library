# Keenable Shipcheck

## Results

- Final shipcheck: **PASS**; all canonical legs passed.
- Verify: **PASS**, live mode, 27/27 checks, 100% pass rate, zero critical failures.
- Validate narrative: **PASS**, all narrative command paths and full examples resolved.
- Dogfood structural: **WARN only** for the expected generic sync diagnostic; novel feature coverage is 7 planned / 7 found and dead-function count is zero.
- Workflow verify: **PASS**.
- Apify audit: **PASS**.
- Verify-skill: **PASS**.
- Scorecard: **89/100**, grade A. Live API verification is now scored 10/10 after exact-key live verification. MCP tool design/surface are 10/10; MCP description/token dimensions are N/A for the code-orchestration surface. Remaining score deductions are structural for a stateless two-operation read API: cache freshness 3/10, data pipeline integrity 7/10, sync correctness 7/10, and vision 6/10.
- Scorecard live feature samples: 7/7 passed.
- Full live dogfood: **86/86 passed**, 100% pass rate.

## Fixes applied

- Implemented all seven approved novel commands.
- Corrected generated endpoint examples to use real public URLs and valid command paths.
- Added local-vs-live output metadata.
- Added flexible decoding for Keenable's numeric `published_at` responses.
- Removed unused generated pagination/no-op helpers.
- Added canonical auth env metadata and code-oriented MCP configuration.
- Preserved a README/SKILL surface with Quick Start, Agent Usage, Health Check, Troubleshooting, Cookbook, Unique Features, and anti-triggers.

## Live evidence

- Authenticated `POST /v1/search` with the supplied key returned HTTP 200 and ranked results.
- Public `POST /v1/search/public` and `GET /v1/fetch/public` returned HTTP 200 with expected shapes.
- Public missing-title error returned HTTP 400 with `Missing app identifier`.
- No remote write endpoints exist; no destructive API operation was attempted.

## Recommendation

**ship**. The requested 95 score was not reached; 89 is the measured score after all legitimate MCP and live-verification improvements. The remaining deductions reflect capabilities Keenable does not expose (syncable collections, remote mutations, automatic cache refresh), not unimplemented approved features. No known functional bug remains in the seven novel commands.
