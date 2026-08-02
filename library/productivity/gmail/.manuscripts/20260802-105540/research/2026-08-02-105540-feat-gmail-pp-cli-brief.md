# Gmail CLI Brief

## API Identity
- Domain: Personal + Workspace email (Gmail API v1, official Google REST API, 79 endpoints)
- Users: (1) agent operators (Claude Code / MCP users) driving email from AI sessions; (2) inbox-zero power users who triage/bulk-clean programmatically; (3) developers/founders doing outreach who need send, reply-threading, and scheduled sends; (4) data hoarders/analysts who want offline mailbox search and analytics
- Data profile: messages (id, threadId, labelIds, snippet, sizeEstimate, internalDate, headers From/To/Subject/Date/Message-ID), threads, labels, drafts, filters, sendAs, history records. High-volume (10k-1M messages), incremental-sync-friendly via history.list cursors.

## Reachability Risk
- None. Official stable Google API; live probe returned 200 with real data (16,629 messages in test account). Policy-side only: OAuth restricted-scope verification for published apps; testing-mode 7-day token expiry.

## Top Workflows
1. Agent-driven triage: unread summary, read decoded bodies, archive/label in bulk (gws +triage, MCP servers)
2. Search -> read -> act: Gmail query syntax search, decoded body read, reply with correct threading headers
3. Bulk cleanup: query old/large/promotional mail, batchModify/batchDelete at 1000 ids/call
4. Outreach: send with attachments/HTML, reply-all, forward; scheduled sends (NO tool has this — API gap, UI-only feature)
5. Offline mailbox intelligence: sync to SQLite via history.list, FTS search, sender-frequency and storage analytics

## Table Stakes
- Full search with Gmail query operators; read with base64url + recursive MIME decode
- send/reply/reply-all/forward with In-Reply-To/References threading (gws parity)
- Labels CRUD by NAME (not just ID — simplegmail's top pain), get-or-create
- Drafts lifecycle; batch modify/delete; attachment download
- Filters CRUD (+ gmailctl-style diff/apply is differentiator territory)
- Settings: vacation, sendAs, forwarding, delegates, imap/pop
- Profile/quota; watch/history for new-mail streaming

## Data Layer
- Primary entities: messages (metadata + snippet + decoded text body), threads, labels, senders (derived), drafts, filters, scheduled_sends (novel)
- Sync cursor: historyId (history.list incremental, 404 => full resync); messages.list q-scoped for first sync
- FTS/search: subject/from/to/snippet/body_text FTS5; joins for sender analytics, storage analytics

## Codebase Intelligence
- Source: GongRzhe/Gmail-MCP-Server + shinzo-labs/gmail-mcp source read
- Auth: OAuth2 loopback (localhost:3000, access_type=offline), ecosystem convention ~/.gmail-mcp/{gcp-oauth.keys.json,credentials.json}; env CLIENT_ID/CLIENT_SECRET/REFRESH_TOKEN headless mode; scopes gmail.modify+compose+send+settings.basic (+sharing for delegates/sendAs)
- Data model: high-gravity fields = id, threadId, snippet, From/To/Subject/Date/Message-ID/In-Reply-To/References headers, payload.parts recursion, body.data (base64url), attachmentId
- Rate limiting: 6,000 quota units/min/user (messages.get=20, send=100, list=5, batchModify=50 for 1000 ids); 429 => exponential backoff mandatory
- Architecture: every wrapper reimplements base64url+MIME walk; NOBODY does HTML-to-text; reply threading = Message-ID/References header construction; batch endpoints are the bulk-op key

## User Vision
- User explicitly requested: SCHEDULED EMAIL SENDING. Gmail API has no native schedule-send
  (UI-only feature). Build as local SQLite queue: schedule send --at/--in, schedule list/cancel/edit,
  schedule run (cron/launchd-friendly processor) + --watch mode, idempotent (never double-send).
  This is shipping-scope user vision.

## Product Thesis
- Name: gmail-pp-cli
- Why it should exist: The Gmail tool landscape is fragmented — gws has great helpers but no local store and no analytics; MCP servers mirror the API but decode poorly (no HTML-to-text) and have zero offline capability; gmailctl only does filters; GYB only does backup. Nobody has: offline SQLite mailbox mirror with FTS + sender/storage analytics, schedule-send, or agent-native structured output across the full 79-endpoint surface. One binary that absorbs all of them and adds what the API itself lacks.

## Build Priorities
1. Data layer: messages/threads/labels sync via history.list incremental; FTS5 on decoded bodies
2. Core mail verbs: find (live search), read (full MIME decode + HTML-to-text), send/reply/forward (threading headers), triage
3. Schedule-send queue (user vision): send --at/--in, list/cancel/edit, run/--watch, idempotent
4. Bulk ops: archive/clean via batchModify/batchDelete with dry-run
5. Full settings surface (generated endpoints): filters, sendAs, vacation, forwarding, delegates
