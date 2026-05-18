# numista-pp-cli — Phase 5.5 Polish Report

## Delta
|              | Before | After | Δ |
|--------------|--------|-------|---|
| Scorecard    | 85/100 | 85/100 | 0 |
| Verify       | 100%   | 100%   | 0 |
| Tools-audit  | 5 pending | 0 pending | -5 |
| PII-audit    | 0      | 0      | 0 |
| Dogfood      | PASS   | PASS   | — |
| go vet       | 0      | 0      | 0 |
| Workflow-verify | workflow-pass | workflow-pass | — |
| Verify-skill | 0 findings | 0 findings | — |

## Fixes applied
- **Tools-audit:** 5 thin-short findings on `mints`, `types`, `types issues`, `users collected-items`, `users collections` parent groupers — accepted with per-finding rationales noting they're generator-emitted category containers (`Short: "Manage <plural>"`) whose leaf subcommands carry the actionable Shorts. Parent-grouper detection in tools-audit doesn't currently recognize the `parentNoSubcommandRunE` helper.

## Skipped findings (out of polish scope)
- **mcp_remote_transport (5/10)** — Structural; requires spec.yaml `mcp.transport: [stdio, http]` + regenerate.
- **mcp_token_efficiency (7/10)** — Structural; spec.yaml `mcp.endpoint_tools: hidden` + `mcp.orchestration: code` + regenerate.
- **mcp_tool_design (5/10)** — Structural; requires `mcp.intents` block in spec for multi-step intent composition.
- **Verify score-2 commands** (collection, crawl, oauth-token, profile, refresh, watchlist, workflow): scorer behavior — these pass help+dry-run but lack exec samples in the verify harness. Not a CLI defect.
- **Dogfood Data Pipeline PARTIAL**: novel-feature search hand-writes SQL against FTS5 by design (transcendence feature). Heuristic doesn't apply.

## Ship verdict
**ship** — remaining_issues empty, further_polish_recommended: no.

> Polish converged with zero remaining issues and every hard gate passing; the remaining scorecard headroom is structural (MCP spec-extension edits + regeneration), which falls outside polish's no-regenerate contract.

## Retro candidate filed
Generator emits `Short: "Manage <plural>"` for parent commands that use `parentNoSubcommandRunE`. Because that helper sets `RunE`, tools-audit's parent-grouper exemption doesn't apply, surfacing the parent as a thin-short finding. Two clean retro fixes: (a) generator emits a richer composite Short from the subcommand list; (b) tools-audit recognizes the `parentNoSubcommandRunE` sentinel and exempts those commands.
