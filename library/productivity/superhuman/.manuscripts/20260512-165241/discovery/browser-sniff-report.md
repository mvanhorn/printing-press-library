# Superhuman Browser-Sniff Discovery Report

## Method
Authenticated chrome-MCP capture against the user's logged-in Chrome session on `mail.superhuman.com`. User is logged in to two accounts: `user@example.com` and `user2@example.com`. Team ID: `team_11UMCgi2kN68oQRadb`. App version observed: `2026-05-08T19:06:03Z`.

Augmented with deep code-archaeology of `edwinhu/superhuman-cli` (TypeScript reference impl, GitHub) which has documented every internal endpoint Superhuman's web app uses.

## Backend
- **Base URL**: `https://mail.superhuman.com/~backend`
- **Pattern**: dot-RPC. Each endpoint is `POST /v3/<service>.<method>` with a JSON body. Response is JSON. No GraphQL, no persisted queries, no WebSockets.
- **Auth**: `Authorization: Bearer <Firebase ID token>` for every backend call.
- **Content-Type quirks**:
  - `/v3/userdata.*` and `/v3/ai.*` use `text/plain;charset=UTF-8` (NOT JSON, despite the body being JSON)
  - `/v3/attachments.upload` uses `application/json`
  - `/messages/send` and `/messages/send/log` use `application/json; charset=utf-8` plus the `x-superhuman-*` request id headers

## Confirmed endpoints (live capture)
| Method | Path | Purpose | Confirmed |
|--------|------|---------|-----------|
| POST | `/v3/browsers.create` | Register a browser session on app boot | Live |
| GET  | `/v3/teams.suggest?includeSingletons=true` | List the user's teams | Live |
| GET  | `/v3/users.achievements` | Gamification state | Live |
| GET  | `/v3/users.getReferral` | Referral code/state | Live |
| POST | `/v3/metrics.write?email=<email>` | Analytics (no value for our CLI) | Live |
| POST | `/v3/userdata.write` | Firestore-like document mutation | Live, captured body |
| GET  | `/~backend/contact/<email>/photo?name=<name>` | Contact photo | Live |

## Endpoint inventory from edwinhu/superhuman-cli (validated against live state)

### Core read/write surface
- `POST /v3/userdata.writeMessage` — drafts (`writes:[{path:"users/<uid>/threads/<tid>/messages/<msgId>/draft", value:DraftValue}]`), attachment metadata, read/unread fallback (`path:"threads/<tid>/labels"`, `value:{addLabelIds, removeLabelIds}`)
- `POST /v3/userdata.getThreads` — thread listing. Body: `{filter, offset, limit}`. Filter shapes:
  - `{}` → latest threads across all (works — this is our inbox listing)
  - `{type:"snippet"}` → snippets
  - `{type:"reminder"}` → snoozed
  - `{listId:"INBOX"|"SH_IMPORTANT"|"SH_OTHER"}` → 400 (Split-Inbox filtering needs CDP)
- `POST /v3/userdata.sync` — bidirectional sync. Body: `{startHistoryId}`. Returns `{history:{threads:{...}}}`. Incremental delta engine.
- `POST /v3/userdata.write` — generic Firestore-like write. Body: `{writes:[{path, value}], doNotReturnValues?:bool}`. Path is hierarchical: `users/<uid>/threads/<tid>/...`.

### Drafts and send
- `POST /messages/send` — send a draft (with optional 20s undo via `delay`). Body: `{version:3, outgoing_message:OutgoingMessage, delay:20, is_multi_recipient:bool}`. Must be preceded by `/messages/send/log`.
- `POST /messages/send/log` — log `draft_ready` before send.
- `POST /v3/attachments.upload` — upload attachment. Body: `{draftMessageId, threadId, uuid, contentType, content (base64)}`. Returns `{downloadUrl}`. Caller then writes attachment metadata via `userdata.writeMessage`.

### AI / semantic search
- `POST /v3/ai.askAIProxy` — Ask AI semantic search (SSE stream). Body includes `session_id`, `question_event_id` (specific `event_11V<rand4><userPrefix4><rand7>` format), `query`, `chat_history`, `user`, `local_datetime`, `current_thread_id`, `current_thread_messages`, `available_skills`. Response is SSE: `data: {"content":...,"retrievals":[...]}\n` terminated by `data: [DONE]`.
- `POST /v3/ai.compose` (sibling, simpler) — takes `{thread_content, instructions}` and dodges the event-id quirk.

### Snooze (reminders)
- `POST /reminders/create` — body: `{reminder:{reminderId, threadId, messageIds, triggerAt, clientCreatedAt}, markDone, moveToInbox, poll}`
- `POST /reminders/cancel` — body: `{reminderId, threadId, moveToInbox, poll}`

### Portal RPC (CDP-only)
These cannot be hit over HTTP. They require `window.GoogleAccount.portal.invoke(service, method, args)` via Chrome DevTools Protocol `Runtime.evaluate` against a logged-in Superhuman tab.

| Service | Method | Args | Purpose |
|---------|--------|------|---------|
| `threadInternal` | `listAsync` | `[listId, {limit, query, filters?}]` | INBOX/SENT/STARRED/TRASH listing (Split-Inbox filterable) |
| `threadInternal` | `getAsync` | `[threadId, {format:"minimal"}]` | Get message IDs for a thread |
| `threadInternal` | `fetchAsync` | `[threadId]` | Fetch full thread |
| `threadInternal` | `modifyLabels` | `[threadId, {addLabelIds, removeLabelIds}]` | Archive, delete, mark read, star, label |
| `messageInternal` | `getBodyAsync` | `[messageId]` | Fetch message HTML body |
| `searchTable` | `query` | `[sql, params, {limit}]` | Local FTS over the SPA's SQLite cache |
| `modifierInternal` | `getAllUncompletedModifiersAsync` | `[]` | Pending actions |
| `accountBackground` | `ping` | `[]` | Health check |

### Calendar (CDP-only for now)
- `window.GoogleAccount.di.get('gcal').<method>(...)` for Google Calendar
- `window.GoogleAccount.backend.requestMicrosoftCalendar({account,url,endpoint,method,body})` for MS Graph

### Contacts
`edwinhu`'s CLI defers contacts to the official MCP. For HTTP-only, hit `people.googleapis.com` / `graph.microsoft.com` directly with the user's cached OAuth `accessToken`.

## Auth flow
The CLI keeps two tokens per account:
1. **Superhuman backend JWT** — Firebase ID token (`securetoken.googleapis.com`). The only header the backend cares about.
2. **Provider OAuth access token** — Google or Microsoft. Used for attachment download and direct Gmail/Graph passthrough.

Extraction (one-time): CDP `Runtime.evaluate` on the running Superhuman tab to call `window.GoogleAccount.<account>.credential.getIDTokenAsync()` for a fresh Firebase refresh. Reads `credential._authData.accessToken` for the OAuth token. Also harvests `userPrefix` (chars 7-10 of `ga.labels._settings._cache.userId` after stripping `user_`), `userExternalId`, `deviceId`.

Token store path: `~/.config/superhuman-pp-cli/tokens.json` (schema below).

Refresh: 5-minute buffer before expiry, reconnect via CDP and re-run `getIDTokenAsync()`. On 401/403 from any backend call, force one refresh and retry.

CDP discovery: `CDP.List({host, port:9250})`, filter targets where `url.includes("mail.superhuman.com") && type === "page"`, prefer the tab whose URL contains the target email. Port 9250 chosen to avoid VS Code's 9222.

## Token store schema
```json
{
  "version": 1,
  "accounts": {
    "user@gmail.com": {
      "type": "google",
      "accessToken": "...",
      "expires": 1715526400000,
      "userId": "...",
      "userPrefix": "4sKP",
      "userExternalId": "user_...",
      "deviceId": "<uuid>",
      "superhumanToken": { "token": "<jwt>", "expires": 1715526400000 }
    }
  },
  "lastUpdated": 1715522800000
}
```

## ID formats (CRITICAL — wrong formats return 400)
- `draftId`: `draft00` + exactly 14 hex chars (`draft0012ab34cd56ef78`)
- `rfc822Id`: `<random>.<uuid>@we.are.superhuman.com>` (with angle brackets)
- `superhuman_id`: `<tsBase36>.<uuid>` where tsBase36 is `Date.now()` clamped to 8 base36 chars
- AI `question_event_id`: `event_11V<rand4><userPrefix4><rand7>` — base62

## TypeScript types to mirror in Go
- `InboxThread` — `{id, subject, from:{email,name}, date, snippet, labelIds, messageCount}`
- `ThreadMessage` — `{id, threadId, subject, from, to[], cc[], date, snippet, body?}`
- `DraftValue` — full draft envelope (to/cc/bcc are STRINGS here)
- `OutgoingMessage` — for `/messages/send` (to/cc/bcc are OBJECTS here)
- `SuperhumanAttachment` — `{uuid, name, type, inline, downloadUrl}`
- `Snippet` — `{id, threadId, name, body, subject, snippet, to[], cc[], bcc[], sends, lastSentAt}`
- `Label` — `{id, name, type?}`
- `CalendarEvent` — `{id, summary, description, start, end, location, attendees[], organizer, isAllDay, status, calendarId}`
- `Attachment` — `{id, attachmentId, name, mimeType, extension, messageId, threadId, inline}`
- `AIRetrieval` — `{thread_id, message_id, subject, from, to?, date, index}`

## Replayability assessment
- **HTTP backend**: fully replayable from a captured JWT. Token refresh is via Firebase `securetoken.googleapis.com` (works with provider OAuth refresh token), or via CDP re-extraction if Chrome is alive. Standard HTTPS, no Cloudflare/WAF challenge observed.
- **CDP transport**: NOT replayable as HTTP. Requires Chrome running on `--remote-debugging-port=9250` with a logged-in Superhuman tab. This is the constraint for inbox-list-by-category, message-body fetching, and local FTS.
- **AI SSE**: replayable as `text/event-stream` HTTP POST. Standard SSE chunked transfer.

## Recommendation
Ship a v1 CLI that uses **HTTP backend only** for ~80% of feature coverage. Hand-build the **CDP auth bootstrap** as a one-time setup command (`superhuman auth login --chrome`). Defer Split-Inbox-category listing, message body HTML fetch, and local-SQLite-FTS-on-app-cache to v1.1 or as opt-in `--via cdp` flags. Treat **durable token refresh** as the headline value prop — the user's stated #1 pain is the MCP losing auth, and a native Go CLI managing Firebase refresh-token lifecycle solves it cleanly.

## Reachability
- `mail.superhuman.com/~backend/*` → 401 without JWT, 200 with valid JWT. No bot challenge. `probe-reachability` would return `standard_http`.
- `mcp.mail.superhuman.com/mcp` → 401 (OAuth-gated). Useful as fallback transport for Business/Enterprise users.
