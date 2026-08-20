# Outlook Email CLI Brief

## API Identity
- Domain: Microsoft Graph mail surface (`/me/messages`, `/me/mailFolders`, `/me/sendMail`, `/me/messages/delta`, `/me/inferenceClassification`, `/me/outlook/masterCategories`, `/me/mailboxSettings`)
- Users: Personal Microsoft 365 / Outlook.com / MSA owners and small businesses on M365. Power users running script-driven triage, agents driving inbox-zero loops, "what did I miss" workflows. **Not** the COM-Outlook-desktop audience — that is a different product (`gmickel/outlookctl`).
- Data profile: Mid-volume, high-cardinality. A typical mailbox: 10k–500k messages, 20–200 folders, 50–500 distinct senders, 20–80 attachments per active day. Per-message payload is large (HTML body, attachments collection). Delta-sync is mandatory; full re-pull is wasteful and rate-limited.

## Reachability Risk
- None. Microsoft Graph is a first-party Microsoft API with a documented OpenAPI master spec. The outlook-calendar CLI shipped against this exact authority (`/common`) and continues to work; we have a known-good auth playbook to mirror.

## Auth model (the load-bearing decision)
- OAuth 2.0 device-code against `https://login.microsoftonline.com/common` so personal MSAs work without an Azure tenant.
- Default client id: Microsoft Graph PowerShell (`14d82eec-204b-4c2f-b7e8-296a70dab67e`) — works for personal accounts out of the box; users can BYO Azure AD app via `--client-id`.
- Default scopes: `Mail.ReadWrite Mail.Send User.Read offline_access` (read/write the user's own mail; send-on-behalf; persistent refresh).
- Spec auth declared as `bearer_token` with `OUTLOOK_EMAIL_TOKEN` env var; hand-built `auth login --device-code` persists access+refresh tokens in `~/.config/outlook-email-pp-cli/config.toml`; `auth refresh` rotates before expiry.

## Top Workflows
1. **Triage sweep** — "what came in since I last looked, who from, what's flagged, what's unread". Today: Outlook UI scroll, Copilot $30/seat, or one-shot scripts. Our pitch: one command, focused/other classified, sender-grouped, agent-pipeable JSON.
2. **Send / reply / forward from script** — compose, reply-all, forward with attachments. `outlook-email-pp-cli send --to ... --subject ... --body-md ...` and `<id> reply`/`<id> replyAll`/`<id> forward`.
3. **Search across folders / time windows** — Graph `$search` is OK; combined with our local SQLite FTS5 we can do regex, AND-NOT, sender+window combinations Graph doesn't support.
4. **Follow-up tracking** — "emails I sent more than N days ago to person X that they never replied to". This is purely a local-store operation; Graph has no endpoint for it.
5. **Unsubscribe / sender pruning** — group inbox by sender, count, surface high-volume senders for bulk archive / inbox rule creation.
6. **Inbox-zero loop** — flagged messages with due dates, snooze suggestions, "old unread" report, "attachments older than 90 days" report.

## Table Stakes (must match what others have)
- list/get/send/reply/forward/delete messages
- list/get/create/delete mail folders, move messages between folders
- list/get attachments, download attachment bytes
- focused/other (`inferenceClassification`) read + override
- categories (color labels) read + apply
- mailbox settings (timezone, signature, automaticReplies) read
- `$filter`, `$search`, `$top`, `$orderby`, `$select` on list/get
- delta-sync messages and mailFolders
- JSON output, `--select` field narrowing
- Agent-first (`--agent`/`--json`, non-interactive, exit codes, dry-run)
- MCP server with read-only annotations on safe tools

## Data Layer
- Primary entities: `messages`, `mail_folders`, `attachments` (metadata only by default), `categories`, `inference_overrides`, `mailbox_settings`.
- Sync cursor: messages/delta() per folder when scoped, or per-mailbox; folder delta tracks folder hierarchy.
- Indexes: `(received_at desc)`, `(from_email)`, `(is_read)`, `(folder_id, received_at desc)`, `(conversation_id)`, FTS5 over `subject+body_preview+from+to`.
- Stored fields per message: id, conversation_id, parent_folder_id, subject, body_preview, body_full (lazy), from{name,email}, to/cc/bcc (joined), received_at, sent_at, is_read, importance, flag{status,due,start,complete_at}, has_attachments, categories[], inference_classification, web_link, internet_message_id, change_key.

## Codebase Intelligence
- Reuse: existing `/Users/paul/printing-press/manuscripts/outlook-calendar/20260510-094714/research/msgraph-master.yaml` (37MB) is the Microsoft Graph beta+v1.0 master and contains every mail endpoint we need. We will author a curated internal-YAML spec by lifting these paths, dropping bulk admin endpoints, and tightening params (like the calendar CLI did).
- Auth wiring: `outlook-calendar-pp-cli` already has the device-code-flow boilerplate (`internal/oauth`, `auth_login.go`, `auth_refresh.go`, `config.SaveTokens`). Mirror exactly, just with different env-var name and default scopes.

## Source Priority
- Single source: Microsoft Graph v1.0. No combo CLI.

## User Vision
- Mirror the `pp-outlook-calendar` shape: personal MSA via device-code OAuth, offline conflict-aware operations, agent-first surface. No proprietary auth flows, no work-account-only features, no COM bridge.

## Product Thesis
- **Name:** `outlook-email-pp-cli` (binary), `pp-outlook-email` (skill), display name **Outlook Email**.
- **Headline:** "Drive your personal Microsoft 365 inbox from agents — read, send, sync, and run offline triage analytics that Outlook's own UI never exposes."
- **Why it should exist:** Other Outlook CLIs are read-only, work-account-only, or single-purpose. None of them ship a persisted local store, which means none can answer "who hasn't replied in 7 days," "which senders blew up my last month," or "old unread by folder" without paying for Copilot.
- **Companion to outlook-calendar:** Both CLIs share the device-code/MSA story; agents can drive both with one auth playbook.

## Build Priorities
1. **Foundation (P0):** spec authoring (lift mail endpoints from msgraph master), generator run, hand-built device-code auth login mirroring calendar CLI, store schema for messages + folders + attachments + categories, sync command using messages/delta.
2. **Absorbed (P1):** every Graph mail endpoint + every feature from msgcli/outpost/outlook-cli. list/get/send/reply/forward/delete; list/get/create folders; move/copy; categories apply/clear; focused/other read+override; mailboxSettings read; attachments list/download; search with `$search` and offline FTS; agent-context.
3. **Transcendence (P2):** local-store novel features — `followup` (sent without reply), `senders` (volume rollup), `quiet` (no inbox traffic since timestamp), `since` (what arrived since N), `flagged` (open todos with due-date diff), `stale-unread`, `attachments stale`, `dedup` (likely duplicate threads), `digest` (daily summary), `rules-suggest` (heuristic inbox-rule proposals).
4. **Polish (P3):** rename ugly Graph names (`mailFolders/{id}/messages` → `folder messages`), tighten flag descriptions, sample examples with realistic args.
