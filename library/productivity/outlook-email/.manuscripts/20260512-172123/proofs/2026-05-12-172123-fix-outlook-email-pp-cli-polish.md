# Polish Pass — outlook-email-pp-cli

Run: 20260512-172123. Invoked via Skill tool with absolute working-dir path; STANDALONE_MODE=false; main SKILL retained the publish flow.

## Delta

|                  | Before  | After   | Delta |
|------------------|---------|---------|-------|
| Scorecard        | 78/100  | 78/100  | +0    |
| Verify           | 100%    | 100%    | +0    |
| Dogfood          | FAIL\*  | FAIL\*  | -     |
| Go vet           | 0       | 0       | +0    |
| Go tests         | pass    | pass    | -     |
| Tools-audit      | 0       | 0       | +0    |
| Live-check       | 12/12   | 12/12   | -     |

\* dogfood FAIL is a false-positive in the auth-heuristic detector — see "Skipped findings" below.

## Fixes applied

1. **`myAddress()` heuristic 2 rewritten** in `internal/cli/novel_messages.go`. Was returning the loudest external sender as the authenticated user when no Sent folder data had been synced yet (because heuristic 1 looked only at messages where `parent_folder_id` matched a `sentitems`-named folder). New SQL counts `toRecipients` addresses across distinct senders — the mailbox owner is the consistent recipient across many unrelated senders. **Verified live**: `me` field in `followup`, `waiting`, `digest`, `conversations` now correctly returns `aas2018.brennaman@outlook.com` (was `account-security-noreply@accountprotection.microsoft.com`). Downstream `conversations.last_is_from_me` is now correctly `false` for external senders.

## Skipped findings (classified as not actionable in printed CLI)

- **dogfood "Auth Protocol: MISMATCH"** + **scorecard auth_protocol 3/10** — false-positive in the heuristic detector. Runtime emits `Bearer <token>` via `applyAuthFormat`; dogfood only pattern-matches literal string concatenation. Retro candidate against the Press's auth heuristic.
- **attachments-stale `{}` output under `--select`** — not a handler bug. Handler emits a full envelope (`count`, `items`, `total_bytes`, `total_mb`, `cutoff`, `min_mb`); the empty output comes from `--select sender,received_at,size_mb,name` stripping envelope keys when `items[]` is empty. Broader `--select`/`filterFields` behavior, not specific to this command.
- **scorecard `insight 4/10`, `mcp_token_efficiency 7/10`, `mcp_remote_transport 5/10`, `mcp_tool_design 5/10`, `workflows 6/10`, `cache_freshness 5/10`** — structural for a small personal-mailbox CLI. Fixes would require spec-level `mcp:` block changes (`transport: [stdio, http]`, `endpoint_tools: hidden`, `orchestration: code`, named `intents`) — feature-shaped decisions for a future regen, not polish work.
- **`publish-validate manifest/transcendence/phase5` FAIL** — expected mid-pipeline; `.printing-press.json` is created by the main SKILL during promote.

## Ship recommendation: **ship**

## Further polish recommended: **no**

Phase 4.85 output-review's one substantive identity-resolution bug is fixed and verified live. Remaining low-scoring dimensions are structural (small API surface, no remote transport in spec) or known scorer/heuristic false positives that another polish pass cannot move.
