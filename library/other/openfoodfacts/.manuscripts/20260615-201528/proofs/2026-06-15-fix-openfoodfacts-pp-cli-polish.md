# Open Food Facts CLI — Polish (Phase 5.5)

Scorecard 94→94 (MCP desc trim), verify 100%, dogfood PASS, go vet 0, tools-audit 0 pending.
ship_recommendation: ship; further_polish_recommended: no.

Fixes applied by polish:
- pp:data-source annotations on all 8 novel commands (dogfood structural PASS).
- Trimmed find_get-search MCP description 933→~110 tokens (mcp-descriptions.json + mcp-sync); preserved filter-syntax hints.

Post-polish fixes (output-review warnings, applied in-session):
- swap --max-nova now excludes nova_group:0 (unknown) — unclassified products no longer pass a NOVA ceiling.
- rank emits a stderr "run sync" hint on empty store in all modes (JSON stdout stays clean []).

Skipped (not real gaps): mcp_token_efficiency 4/10 (scorer counts handler-body literals as desc weight; real bloat removed; desc quality 10/10), mcp_tool_design/surface_strategy/live_api_verification (omitted from denominator).
