# SculptOK CLI — Polish Pass

Scorecard 78 -> 79 | Verify 100% -> 100% | Tools-audit 1 -> 0 pending | gosec hand-authored -> 0 | go vet 0
Ship recommendation: ship | Further polish: no

Fixes:
- Cleared hand-authored gosec findings (store.go, sculptok/client.go, sculptok_shared.go): narrow #nosec on provably-safe SQL-fragment switch + parameterized LIMIT, explicit Close() error handling, #nosec with reasons on user-chosen file/output paths.
- Fixed truncated root Short/Long (ended mid-word); restored full value statement.
- Added mcp-descriptions.json override for credits_balance; ran mcp-sync.
- Removed incorrect mcp:read-only on sync (writes to local store).

Skipped (retro candidates / structural): dead generated pagination helpers; dogfood cost.go false positive (uses sibling client); profile use mcp annotation (generated); 16 gosec findings in generated files; search live-check empty-store environmental; scorecard insight/sync_correctness structural for a small read-mostly async API.
