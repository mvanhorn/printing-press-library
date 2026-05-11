# Phase 5.5 Polish Report — telegram-pp-cli

## Delta

|                    | Before    | After     | Delta    |
|--------------------|-----------|-----------|----------|
| Scorecard          | 79/100    | 79/100    | +0       |
| Verify             | 100%      | 100%      | +0       |
| Dogfood            | WARN      | PASS      | improved |
| Publish-validate   | FAIL      | PASS      | improved |
| Verify-skill       | PASS      | PASS      | +0       |
| Tools-audit        | 0 pending | 0 pending | +0       |
| go vet             | 0         | 0         | +0       |

## Fixes applied
1. Migrated 16 `flags.printJSON(cmd, v)` call sites in `internal/cli/novel_*.go` to `printJSONFiltered(cmd.OutOrStdout(), v, flags)` so `--select`, `--compact`, `--csv`, `--quiet`, `--plain` flow through the novel commands the same way they do for generator-emitted endpoint mirrors.
2. Initialized `auditReport.ByChat` and `auditReport.ByMediaType` to empty slices in `buildAuditReport`, so `audit --json` empty case marshals as `[]` rather than `null` (jq and agent loops iterate cleanly on both).
3. Set `SilenceErrors: true` on the root Cobra command and updated `cmd/telegram-pp-cli/main.go` to emit a single `Error: <msg>` line on failure — eliminates the duplicate error-message double-print on failures like `publish send --body <missing-file>`.
4. Added the missing `printer` field (`1nationlivinthelegend`) to `.printing-press.json` and restored `mcp_binary` to `telegram-pp-mcp` after a stray mcp-sync flipped it to the slug-derived `telegram-bot-pp-mcp`. Required by publish-validate's manifest check.
5. Regenerated `tools-manifest.json` via `printing-press mcp-sync . --force` — publish-validate checks for MCP manifest presence.
6. Fixed stale `telegram-bot-pp-{cli,mcp}` references in `internal/mcp/tools.go`, `internal/mcp/cobratree/cli_path.go`, `manifest.json`, and `tools-manifest.json` (introduced by mcp-sync's spec-title-vs-dir-slug drift). Removed the spurious `cmd/telegram-bot-pp-{cli,mcp}/` directories.
7. Copied `phase5-skip.json` to `<CLI_DIR>/.manuscripts/<run-id>/proofs/` where `publish validate` resolves Phase 5 acceptance markers.

## Skipped findings
- **pii-audit:** command not available in printing-press v4.2.2 (above the v4.0.0 floor though). Environmental skip.
- **Dogfood "Path Validity 0/0 FAIL" header line:** divisor branch artifact; scorecard's `Path Validity 10/10` confirms the real capability. Cosmetic.
- **Scorecard MCP dimensions (Token Efficiency 4/10, Surface Strategy 2/10, Tool Design 5/10, Remote Transport 5/10):** 74 typed endpoints with no `mcp:` block applied to the spec. Fix is spec-edit-driven (`transport: [stdio, http]`, `endpoint_tools: hidden`, `orchestration: code`) plus regen — out of scope for in-place polish. Flag as a retro/reprint candidate.
- **Scorecard Auth Protocol 5/10, Type Fidelity 3/5, Data Pipeline Integrity 7/10, Cache Freshness 5/10:** structural — Telegram's bot-token-in-URL-path auth doesn't fit the scorer's standard archetypes; novel `messages list` and `audit` read directly from `internal/store` rather than the generic search helper. Real but not gameable without scaffolding.

## Ship recommendation
**`ship`** — `further_polish_recommended: no`. All hard gates (verify, dogfood, verify-skill, workflow-verify, publish-validate, tools-audit, go vet, unit tests) pass. Remaining scorecard gaps are spec-edit-driven (need regen with `mcp:` block) or structurally locked (Telegram's URL-path auth), neither of which a re-run resolves.
