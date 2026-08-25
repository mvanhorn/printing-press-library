# Gmail Absorb Manifest

Sources: googleworkspace/cli (official gws), GongRzhe + ArtyMcLabin Gmail-MCP-Server, cablate/mcp-google-gmail, Gururagavendra/gmail-cleaner, InboxWhiz/gmail-declutter-extension, justjake/gmail-unsubscribe, davidtkeane/gmail-multi-cli, ThomasHabets/cmdg, Inbox Zero (getinboxzero.com, incumbent SaaS).

Hard constraints (build-plan Phase 5 amendment, operator-accepted): spec pruned to 20 ops — send/insert/import/drafts/settings/watch/permanent-delete structurally absent; scope pinned `gmail.modify`; Trash is the deletion ceiling; unsubscribe = HTTP one-click (RFC 8058) only, mailto surfaced never sent; no LLM calls inside the CLI.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Gmail search-syntax passthrough (from:, newer_than:, category:, larger:) | every tool | live `search` / `messages list --q` | offline FTS fallback, --agent JSON |
| 2 | Message read w/ format control | Gmail MCP servers | `messages get --format metadata/full` | headers-first default, attachment refs |
| 3 | Thread read | MCPs, cmdg | `threads get` | conversation-shaped JSON |
| 4 | Multi-account | gmail-multi-cli | named gauth profiles (`--account ads/personal`) | role-scoped tokens, consented-account verification |
| 5 | Bulk ops by query with preview | gmail-cleaner age/size/category filters | `cleanup plan` → `cleanup apply` | preview→confirm→undo ledger, batchModify at scale |
| 6 | Sender analytics | Inbox Zero, InboxWhiz | `senders` (SQLite aggregate: volume, size, unread-rate, last-seen) | offline, cross-account |
| 7 | Bulk unsubscribe | Inbox Zero, justjake/gmail-unsubscribe | `unsub audit` (extract+classify List-Unsubscribe) + `unsub run` | RFC 8058 HTTP one-click ONLY, confirm-gated; mailto→desk list, never sent; ledger |
| 8 | Label CRUD | gws, MCPs | `labels create/update/delete/list` + taxonomy tree view | safe rename, counts |
| 9 | Bulk label apply/remove | MCPs | `messages modify` + `batchModify` via cleanup engine | inverse-op recording (undo) |
| 10 | Trash/untrash | cleanup tools | `messages/threads trash` at query scale via cleanup engine | trash-only ceiling (scope physics), undo |
| 11 | All-category summary | gmail-cleaner category filters | `digest` (per-category counts, unread aging, top senders) + `--category` everywhere | the operator's "summary of ALL my email incl. sub-folders", multi-account |
| 12 | Attachment fetch | cablate/mcp-google-gmail | `attachments get` | size-aware |
| 13 | Profile/quota info | gws | `profile` | multi-account roll-up |
| 14 | Offline store + FTS + incremental sync | nobody | press data layer + historyId cursor sync | the substrate; expired-cursor full-resync fallback |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Unsubscribe compliance check | `unsub verify` | 10/10 | the operator | Local unsub ledger × post-unsubscribe arrivals — one-click is fire-and-forget everywhere else; nothing verifies senders obeyed |
| 2 | Mailbox delta report | `delta` | 9/10 | the assistant, sweep | Checkpoint row + local tables: what's new/changed since last report — no re-list, no double-reporting |
| 3 | Storage attribution report | `storage report` | 9/10 | the operator | sizeEstimate×label×age×sender aggregation with emit-ready cleanup queries; Gmail only exposes a profile total |
| 4 | Saved cleanup rules | `rules add/list/run` | 9/10 | sweep, the operator | Local rulebook replayed through plan→confirm→undo; server-side filters are structurally pruned, this is the compliant recurrence path |
| 5 | Sort suggestions | `sort suggest` | 9/10 | the operator, the assistant | Majority-label concentration per sender (≥80%, min 5) emitting batchModify plans — the LLM-free version of Inbox Zero's AI sort |
| 6 | Trash undo-window watch | `trash report` | 8/10 | the operator, sweep | Mutation ledger × Gmail's 30-day purge clock: last regret-check before undo becomes impossible |
| 7 | Mailbox health score | `score` | 7/10 | the operator, sweep | Local aggregates snapshotted over time — "Promotions down 60% since baseline" campaign progress |

Kill list (9) and customer model: see `2026-08-18-095450-novel-features-brainstorm.md` (audit trail).

Stubs: none — every row above is shipping scope.
