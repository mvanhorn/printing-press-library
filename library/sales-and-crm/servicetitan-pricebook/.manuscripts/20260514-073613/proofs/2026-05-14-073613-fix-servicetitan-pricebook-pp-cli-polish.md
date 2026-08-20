# servicetitan-pricebook-pp-cli — Phase 5.5 Polish

Polish skill failed once with a transient "API Error: Internal server error"; succeeded on retry.

## Result

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 87/100 | 87/100 |
| Verify | 100% | 100% |
| Dogfood | PASS | PASS |
| go vet | 0 | 0 |
| Tools-audit | 0 pending | 0 pending |
| PII-audit | 0 pending | 0 pending |
| Publish-validate | PASS | PASS |

## Fixes applied
- Rewrote boilerplate/truncated root `Short` to a clean capability one-liner.
- Replaced truncated root `Long` first line with the full headline from `research.json` narrative.
- Fixed truncated CLI description in `agent_context.go` (was cut at "...and...").
- Fixed truncated SKILL.md frontmatter description (was cut at "...and...").
- Removed a stray double blank line in root help text (mcp-sync re-render artifact).
- Ran `mcp-sync` to refresh the MCP surface (tools.go / manifest); migrated cleanly.
- `gofmt -w` fixed pre-existing alignment drift in `sync.go`, `audits.go`, `match.go`, `skus.go`.

Build, `go vet`, and `go test ./internal/pricebook/` all clean after polish.

## Skipped findings (retro candidates — not fixable from within the CLI dir)
- **`mcp_token_efficiency 0/10`** — the spec's `x-mcp` block already has the prescribed config (`endpoint_tools: hidden`, `orchestration: code`, `transport: [stdio, http]`) and the runtime `tools.go` already implements the 2-tool code-orchestration surface, but `tools-manifest.json` still catalogs all 40 endpoints and the scorecard scores off that count. `mcp-sync` does not collapse the manifest; hand-deleting 38 entries would game the scorer. Generator-side fix needed: the manifest emitter should honor `endpoint_tools: hidden`.
- **root.go Highlights `quote-reconcile` line truncated with `…`** — dogfood re-syncs the Highlights block from `research.json` on every run and the sync renderer truncates long descriptions. Source data is clean; any direct edit is clobbered on the next dogfood. Cosmetic (one help-text line; the command's own `--help` is full).
- **scorecard `--live-check` `unable: true` / Phase 4.85 SKIP** — environmental: the Windows binary is built without a `.exe` extension so the live-check probe path isn't executable. Not a CLI defect.

## Ship recommendation: `ship`
`further_polish_recommended: no` — all fixable defects fixed; the three skipped findings are structural generator/scorer gaps and an environmental probe issue, none durably resolvable by another polish pass.
