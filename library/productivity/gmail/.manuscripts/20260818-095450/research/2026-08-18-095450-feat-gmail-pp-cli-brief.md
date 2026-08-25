# Gmail CLI Brief

## API Identity
- Domain: Gmail API v1 (`gmail.googleapis.com`) — official OpenAPI via apis.guru, **pruned to 20 operations before generation** (read + labels CRUD + modify/trash + batchModify + history; send/insert/import/drafts/settings/watch/permanent-delete structurally absent per the operator's assistant build plan Phase 5 amendment).
- Users: (1) an always-on personal-assistant agent doing **mailbox cleanup** on two accounts (multi-account named profiles, per-profile tokens, `gmail.modify` scope pinned); (2) the operator at the desk; (3) any agent needing safe scriptable Gmail hygiene.
- Data profile: messages/threads (headers-first, metadata format), labels (user folders + system categories), sender aggregates, List-Unsubscribe headers, historyId cursors. Two consented accounts; a third (readonly utility) optional later.

## Reachability Risk
- None — official Google API, stable, generous free quota (250 quota units/user/sec; list=5u, get=5u, modify=5u). 401 without token is the expected unauth probe result.

## Top Workflows (operator-stated, verbatim intent)
1. **Summarize all my email** — across every category/label (Primary, Promotions, Social, Updates, Forums, custom), not just inbox: counts, unread aging, top senders, biggest items.
2. **Unsubscribe** — find subscription senders, execute HTTP one-click (RFC 8058) unsubscribes confirm-gated; mailto-only unsubscribes surface as a desk list (never sent — transitive-barrier rule).
3. **Delete (trash) in bulk** — by sender/age/size/category query, previewed first, recoverable (trash-only ceiling is scope physics).
4. **Sort into folders** — bulk label apply/remove by query; batchModify for scale.
5. **Create/manage folders** — label CRUD, taxonomy view.

## Table Stakes (from landscape)
- Full Gmail search-syntax passthrough (`from:`, `newer_than:`, `has:attachment`, `category:`, `larger:`) — every tool has it
- Message/thread read with format control (metadata vs full), attachment fetch
- Multi-account (gmail-multi-cli's headline; ours is role-scoped like the calendar print)
- Bulk ops by query with preview counts (gmail-cleaner's age/size/category filters)
- Label CRUD + apply/remove (gws, MCP servers)
- Sender grouping/analytics (Inbox Zero, InboxWhiz)
- Agent-native JSON everywhere (`gws` ships agent skills; we match with --agent + SKILL.md + MCP mirror)

## Data Layer
- Primary entities: messages (metadata: id, threadId, labelIds, from, subject, date, sizeEstimate, snippet, List-Unsubscribe), labels, senders (aggregate view), unsubscribe ledger, mutation ledger (inverse ops)
- Sync cursor: historyId per account (users.history.list); full resync fallback on 404-expired cursor
- FTS/search: subjects + snippets + senders offline; live search passthrough for body-level queries

## Codebase Intelligence
- GongRzhe/ArtyMcLabin Gmail-MCP (most-installed MCP): auth via OAuth client + token file, tools read/search/send/label — we absorb its read/label surface, structurally exclude its send tools
- Auth: installed-app OAuth loopback, per-profile token store, scope pinned `gmail.modify` ONLY (no readonly+modify dual-mint; modify ⊃ read)
- Rate limiting: 250 units/user/sec; batchModify (50 ids/call) for bulk; AdaptiveLimiter per house rule

## User Vision
Operator's briefing, verbatim intent: "She will be helping me clean up <ads-account> & <personal-account> — summary of all my emails (not just Primary, sub-folders too); unsubscribing; deleting; sorting into folders; creating new folders; there may be other things as we go." Send-capable-scope residual formally accepted at adoption (build-plan register item 6). Enforcement: structural pruning + trash-only + confirm-gated bulk + HTTP-only unsubscribe.

## Product Thesis
- Name: gmail-pp-cli (slug: gmail)
- Why it should exist: nothing combines **safe bulk cleanup** (preview → confirm → undo, trash-only by construction) with **agent-native output** and an **offline sender ledger**. The official `gws` is a thin dynamic API mirror (no local state, no cleanup workflows, no structural send-exclusion); cleanup GUIs aren't scriptable; MCP servers can send (ours cannot, by construction).

## Build Priorities
1. Data layer: message-metadata sync w/ historyId cursor + senders aggregate + FTS
2. Read surface: search/list/get/digest across categories, multi-account merge
3. Cleanup engine: plan→apply→undo (batchModify/trash), label ops, taxonomy
4. Unsubscribe: audit (extract+classify List-Unsubscribe) + run (RFC 8058 one-click, confirm-gated) + ledger
5. Sender intelligence: volume/unread-rate/stale-subscription detection
