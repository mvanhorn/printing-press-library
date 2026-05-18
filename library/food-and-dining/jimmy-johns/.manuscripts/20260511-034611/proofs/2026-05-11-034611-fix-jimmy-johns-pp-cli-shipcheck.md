# jimmy-johns-pp-cli Shipcheck

## Verdict: ship-with-gaps

### Shipcheck Summary
- dogfood: PASS
- verify: PASS (19/19, 100%)
- workflow-verify: workflow-pass
- verify-skill: PASS (0 findings)
- validate-narrative: PASS
- scorecard: 80/100, Grade A

### Scorecard Highlights
- Path Validity 10/10, Sync Correctness 10/10, MCP Token Efficiency 10/10, Local Cache 10/10
- Auth Protocol 5/10 (cookie type partially templated as OAuth)
- Insight 4/10 (no transcendence features hand-built)
- MCP Remote Transport 5/10 (no mcp.transport stanza in spec)

### Known Gaps
1. **Live API blocked by PerimeterX.** JJ uses PerimeterX bot protection that fingerprints TLS handshake and JS execution context. Even with 33 valid session cookies extracted from a logged-in real Chrome browser, replay via Surf+Chrome impersonation returns 403. Reachability probe predicted browser_clearance_http; observed behavior is browser_required.
2. **No novel features built.** Brief listed transcendence ideas (`freaky-fast` ETA predictor, `unwich-mode`, `office-lunch --people N`) but Phase 3 wasn't completed.
3. **`auth login --chrome` flow needs adapter.** Press generated a flow expecting pycookiecheat/cookies/cookie-scoop-cli; on this machine those couldn't read Chrome's running cookie DB. Working around requires either closing Chrome first OR adapting to read from browser-use's exported cookies.json.

### What Works
- All 16 endpoints declared, typed, and surfaced as CLI subcommands
- MCP server bundled (jimmy-johns-pp-mcp-darwin-arm64.mcpb)
- `--dry-run` shows correct request shape for every endpoint
- `--help`, `--json`, `--agent`, `--select` plumbing all wired
- Local SQLite store schema for stores, products, orders, rewards
- Framework commands (sync, search, doctor, agent-context, etc.) all functional

### Polish Delta
- Manifest fixed (added printer field, ran mcp-sync)
- Typo "Jimmy Johns" → "Jimmy John's" corrected in README + SKILL
- gofmt clean

### Recommendation
Promote to library as ship-with-gaps. Document PerimeterX limitation in README's Known Gaps section. Future work: hand-build transcendence features, wire browser-use cookie adapter into auth login --chrome.
