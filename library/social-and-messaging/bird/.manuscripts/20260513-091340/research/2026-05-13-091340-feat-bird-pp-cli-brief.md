# Bird CLI Brief

## API Identity
- **Domain:** Bird (formerly MessageBird) — omnichannel messaging + customer engagement platform.
  Brand rebranded from MessageBird ~2023; modern stack lives at `api.bird.com`. The legacy
  MessageBird endpoints (`rest.messagebird.com`, `conversations.messagebird.com`) still work
  but the product surface, docs, and dashboard are on Bird.
- **Users:** Customer-support teams, marketing/notification senders, fintech and e-commerce
  apps that send transactional and conversational SMS/WhatsApp/email/RCS at scale, plus AI
  agent vendors that hook into the Conversations stream.
- **Data profile:** Conversations (1 per contact thread, hundreds–thousands per workspace per
  day), Messages (10–100x conversations), Channels (one per platform: SMS, WhatsApp, email,
  RCS, FB, IG, etc.), Participants (contacts + agents + bots + flows), Workspace settings
  (anti-spam, allow/block lists). Numbers, contacts, and templates are nearby surfaces but
  out of scope per user briefing.

## Scope (per user briefing)
- **In scope:** Bird Conversations API (full surface) + Channels API but **only the SMS
  pieces** (sending SMS, listing/getting SMS messages, programmable-SMS templates, channel
  management for SMS channels, allow/block / compliance keywords for SMS).
- **Excluded:** Voice API, Verify API, WhatsApp/Email/RCS-specific channel features, Numbers
  inventory, Contacts API (used as participant identifiers but not exposed as commands).

## Reachability Risk
- **Low.** Direct HTTPS to `api.bird.com` returns 401 (auth required) — expected for a
  paid commercial API. No 403/Cloudflare/bot-protection signals. The MessageBird Go SDK
  (~3.5y maintained) and the published Postman collection both indicate the surface is
  fully programmatic. No GitHub issues flagging breakage.

## Top Workflows
1. **Send a transactional SMS** — pick the SMS channel, target a phone number, post a body
   (with optional templates and media). Today's incumbents ship this as one of dozens of
   methods inside a multi-product SDK.
2. **Triage open conversations** — list active conversations on a channel, filter by status
   ("active"/"snoozed"/"closed"), dive into messages, reply or assign.
3. **Audit message delivery** — fetch a message by ID, inspect the delivery `interactions`
   trail (sent, delivered, read, failed), and correlate with the originating conversation.
4. **Block-list / compliance management** — bulk-add phone numbers to allow or block rules,
   manage SMS compliance keywords (STOP, HELP, START), monitor anti-spam settings.
5. **Bulk SMS dispatch + reconcile** — send a batch from a CSV of recipients, capture each
   response message ID, and re-poll to confirm delivery (post-mortem when callback webhooks
   weren't set up).

## Table Stakes (from competing tools and the docs)
- Send SMS message (single + batch).
- List / get / update / delete conversations.
- List / get / send / delete conversation messages.
- List participants in a conversation; add/remove participants.
- Get/update channel conversations configuration.
- Workspace anti-spam read/update; allow/block rules CRUD + bulk import.
- Pre-signed media upload for MMS attachments.
- List messages by channel and by workspace, with paging.
- Get a message by ID; list message-interaction events.

## Data Layer
- **Primary entities:** `conversations`, `messages`, `channels`, `participants`,
  `allow_block_rules`, `interactions` (per-message delivery events).
- **Sync cursor:** `pageToken` returned in list responses — Bird's standard pagination.
  Use `updatedAt` for incremental syncs of conversations/messages.
- **FTS/search:** index conversation messages (text body) and conversation IDs / participant
  identifiers for `search` and `sql`. SMS bodies are short, so FTS5 over `messages.body` is
  cheap and high-leverage for "find that order confirmation I sent yesterday."

## Codebase Intelligence
- Source: messagebird/openapi-specs (SMS legacy spec) and the Bird docs Markdown surface
  (queryable via `?ask=` parameter on every page).
- **Auth:** `Authorization: AccessKey <access-key>`. Same scheme on legacy MessageBird and
  new Bird. Env var convention in the wild: `MESSAGEBIRD_ACCESS_KEY` (legacy SDKs); we'll
  use `BIRD_API_KEY` (current branding).
- **Workspace scoping:** Every modern Bird path is prefixed with `/workspaces/{workspaceId}`.
  Best practice: bake `{workspaceId}` into the spec's `base_url` and resolve it at runtime
  from `BIRD_WORKSPACE_ID` via `endpoint_template_vars`. No more `--workspace-id` on every
  command.
- **Channel scoping:** SMS-specific endpoints take `{channelId}` as a per-call path param.
  The CLI keeps it as a flag/positional; users typically pin one SMS channel and pass it
  via `BIRD_CHANNEL_ID` env or config.
- **Rate limiting:** 50 retrieval calls/sec burst, 2,000/min steady. Respect 429 backoff.
- **Architecture:** Bird splits responsibilities between **Channels API** (low-level
  send/receive, per-platform) and **Conversations API** (cross-channel thread aggregation,
  participants, statuses). The same SMS message exists in both surfaces.

## User Vision
- "only the Conversations / SMS endpoints, not Voice or Verify" — narrow the absorb scope
  to those two product areas. Voice/Verify endpoints are out even if SDKs/wrappers ship them.

## Product Thesis
- **Name:** `bird-pp-cli` (binary), library slug `bird`.
- **Why it should exist:** Every existing Bird/MessageBird client is a multi-language SDK
  embedded inside an app. There is no terminal-native Bird tool. Engineers debugging an SMS
  failure today have to paste a curl command into Slack or write a one-off script. A CLI
  with offline search of the conversation log, FTS over message bodies, and a SQLite mirror
  collapses every "did that message deliver?" question into one shell command.

## Build Priorities
1. **Foundation (Priority 0):** SQLite store for conversations, messages, channels,
   participants; `sync`, `search`, `sql`. This is what unlocks every transcendence command.
2. **Absorb (Priority 1):** Every Conversations API endpoint + every SMS-relevant Channels
   API endpoint. Match the docs surface 1:1, expose `--json`, `--dry-run`, `--select`,
   typed exit codes, and stable JSON output everywhere.
3. **Transcend (Priority 2):** Local-store features the SDKs can't offer — delivery audits,
   conversation timeline merges across channels, allow/block bulk diff, message search by
   recipient or content fragment. Defer scoping until the absorb subagent runs.

## Sources
- [Bird API docs welcome page](https://docs.bird.com/api)
- [Bird Conversations API](https://docs.bird.com/api/conversations-api)
- [Bird Channels API messaging](https://docs.bird.com/api/channels-api/api-reference/messaging)
- [Bird programmable-SMS](https://docs.bird.com/api/channels-api/supported-channels/programmable-sms/sending-sms-messages)
- [Common API usage / auth + rate limits](https://docs.bird.com/api/api-access/common-api-usage)
- [MessageBird OpenAPI specs (SMS)](https://github.com/messagebird/openapi-specs)
- [Legacy MessageBird Conversations API](https://developers.messagebird.com/api/conversations/)
- [MessageBird Postman collection](https://www.postman.com/messagebird-official/messagebird-official/collection/akk02ux/messagebird-api)

## Reprint Note (v4.5.2)
This is a reprint with printing-press v4.5.2 (prior CLI was generated with v4.2.2). Notable
machine deltas since v4.2.2 that affect this CLI:
- envelope unwrap before agent provenance (#1095) — fixes a prior open retro finding
- bearer + root-security filter in scheme selection (#1238) — not used (api_key auth)
- Accept header default application/json (#1229)
- gofmt rendered .go output (#1100)
- skip dogfood --live error_path probe for mutating commands (#1225)

Reachability profile unchanged: api.bird.com returns 401 (auth required). Auth model
unchanged: AccessKey in Authorization header via BIRD_API_KEY. No browser-sniff,
no crowd-sniff — Bird API is a documented commercial REST API.
