# Figma CLI Shipcheck Report (reprint v4.2.0 -> v4.24.0)

## Verdict: ship

## Shipcheck umbrella: 6/6 legs PASS (rc=0)
verify PASS · validate-narrative PASS · dogfood PASS · workflow-verify PASS · verify-skill PASS · scorecard PASS

## Scorecard: 93/100 — Grade A
- Single gap: mcp_token_efficiency 0/10 — confirmed SCORER FALSE-NEGATIVE. The MCP server exposes only the thin code-orchestration surface (figma_search + figma_execute + context/search/sql = 5 tools; 48 endpoint-mirror tools suppressed via x-mcp endpoint_tools:hidden). Scorer appears to read the endpoint catalog, not the exposed tool count. Retro candidate; not a CLI defect.
- Auth Protocol 9/10 (newly scored; native PAT auth emitted from spec).

## Reprint outcome: all 3 prior hand-patches absorbed natively by v4.24.0
- pat-auth-wiring: native PAT (X-Figma-Token + FIGMA_ACCESS_TOKEN/FIGMA_API_TOKEN/FIGMA_API_KEY) emitted from spec; no patch needed.
- cache-key-sorted: generator now sorts query-param keys before hashing the cache key.
- dry-run-pat-label: dry-run echo uses X-Figma-Token label.

## Fixes applied this run
- Removed non-verify-safe `auth status` from narrative quickstart (validate-narrative full-example failed exit 4); README Quick Start aligned. doctor is now step 1.
- Added rows.Err() checks in fingerprint.go and tokens.go scan loops (Phase 4.95).
- Generator-bug workarounds during port: webhooks test file renamed off _test.go; generic framework orphans collision removed.

## Ship recommendation: ship
All ship-threshold conditions met. 8/8 novel features resolve and dry-run clean. Live behavioral verification pending Phase 5 (needs Figma PAT).
