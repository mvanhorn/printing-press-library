# Polish (Phase 5.5) — ship
Scorecard 94 -> 96 (MCP Desc Quality 0 -> 10); verify 100% -> 100%; dogfood PASS; gosec (hand-authored) 51 -> 0 (5 narrow #nosec with reasons; 31 remaining in generated files); tools-audit 5 -> 0 pending (2 accepted, generated thin shorts); go vet 0; PII audit clean.
Fixes: filepath.Clean on profile paths; explicit discards for read-only Close; sync table flush error returned; prefs \u parsing bitSize 16; mcp-descriptions.json overrides for 6 manifest tools via mcp-sync; stats Short verb-led.
Skipped (retro): generated-file gosec (incl. migration.go/testenv.go missing Generated header); sync_correctness/cache_freshness scoring HTTP template on local-sqlite CLI; mcp-sync pagination_undeterminable; mcp-sync truncates the awaiting-reply insight in internal/mcp/tools.go (restored by hand).
Live check sampled an empty sandbox store (5/5 pass); plausibility verified manually against the real store.
ship_recommendation: ship
