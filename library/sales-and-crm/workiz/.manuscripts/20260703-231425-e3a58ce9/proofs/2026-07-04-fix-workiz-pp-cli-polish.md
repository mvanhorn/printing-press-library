## Polish Results for workiz-pp-cli

```
                    Before    After     Delta
  Scorecard:        92/100    92/100    +0
  Verify:           100%      100%      +0%
  Live matrix:      exercised -> exercised
  Tools-audit:      2 pending 0 pending -2 pending findings
  Gosec (hand-authored): 1    0         -1
```

**Fixes applied:**
- Fixed a mis-cited `#nosec` suppression rule ID in `internal/cli/novel_shared.go:138` (G201 -> G202) so gosec's exact rule-ID match actually suppresses the false positive (table name drawn from a fixed internal set, never user input).
- Added agent-grade MCP description overrides for `team_list` and `timeoff_list` via `mcp-descriptions.json` (previously thin spec-derived boilerplate). `MCP Desc Quality` 7/10 -> 10/10.
- `gofmt -w .` struct-field alignment cleanup.

**Skipped (generator-owned retro candidates):** 26 gosec findings in generator-emitted files (client.go, config.go, cobratree/shellout.go, store.go, cache.go, profile.go, import.go, feedback.go, deliver.go) — not printed-CLI-owned.

Ship recommendation: **ship**. Further polish not recommended — all hard gates clean, remaining_issues empty.

Rebuilt, re-tested (`go build`/`go vet`/`go test ./...` all clean), and reran the full shipcheck umbrella after polish's changes: still 6/6 legs PASS, Grade A (92/100), 100% sample-output-probe pass rate.
