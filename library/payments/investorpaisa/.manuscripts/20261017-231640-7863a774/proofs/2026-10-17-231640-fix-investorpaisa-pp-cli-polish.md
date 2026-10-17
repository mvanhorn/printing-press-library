# Polish — investorpaisa-pp-cli
Scorecard 87 -> 87, verify 100% -> 100%, dogfood PASS -> PASS, go vet 0 -> 0, tools-audit 0 pending, ship recommendation: ship.
Fixes: health_get MCP description rewritten (mcp-descriptions.json override); breadth and brief help clarified (breadth counts 52-week highs/lows from end-of-day screener rows, brief from the intraday snapshot — different definitions, not a bug).
Skipped: gosec 71 findings all in generator-emitted files (retro); scorecard live-check 8/8 401 because the harness's staged binary has no stored session (verified directly instead); mcp-sync side effects reverted; auth_protocol/cache_freshness structural.
Retro candidates: dotted cookie names rejected by generated cookie jar; generated set-token demands JWT; dogfood config-consistency heuristic false positive; generated auth.go gosec findings; mcp-sync lossy tools.go.
Follow-up in this run: source edits invalidate the phase5 acceptance fingerprint, so tests, shipcheck and the full live dogfood were re-run afterwards.
