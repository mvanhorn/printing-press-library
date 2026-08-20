# Roam (ro.am) CLI Brief

## API Identity
- Domain: Virtual office / async-first remote workspace by Wonder Inventions, distinct from Roam Research (note-taking).
- Users: Distributed teams using Roam HQ for chat, video meetings, transcripts, magicasts (recorded async videos), and on-air events.
- Data profile: Chats, meetings, transcripts, recordings, on-air events, guests/attendance, users, groups, magicasts, webhook subscriptions.

## Reachability Risk
- None. All five OpenAPI specs reachable HTTP 200 at developer.ro.am.
- API base `https://api.ro.am` returns 401 without auth (expected). With provided personal access token: v0/Chat endpoints work; v1/HQ endpoints require an org-level "full access" key (PAT not supported). Captured for Phase 5 scope.

## Top Workflows
1. Pull and search recent transcripts/chat history across meetings ("what did we decide about pricing last week?").
2. Send chat messages and post system notifications from CI/cron into Roam channels.
3. Manage on-air events: create, update guest list, RSVP changes, attendance pull.
4. Compliance/HR: SCIM provision/deprovision users, export message-event archives.
5. Subscribe and manage webhooks for chat-message and event-creation events.

## Table Stakes (matched from Roam's own surfaces)
- Send/list/edit/delete chat; reactions; uploads.
- List/info on transcripts; prompt against a transcript.
- List/create/update/cancel on-air events; manage guests; pull attendance.
- List recordings, magicasts, groups, users.
- Webhook subscribe/unsubscribe.
- SCIM 2.0 user/group CRUD.
- Bearer auth with API key OR personal access token.

## Data Layer
- Primary entities: chats, messages, meetings, transcripts, recordings, magicasts, onair_events, guests, attendance, users, groups, webhooks.
- Sync cursor: cursor-based for most endpoints; date-range for transcripts and meetings.
- FTS5: messages, transcripts (free-text "what was said" queries — agent-grade context retrieval).

## Codebase Intelligence
- 5 official OpenAPI specs at developer.ro.am: openapi.json (v1 HQ, 7), onair.json (v1, 11), chat.json (v0 Alpha, 30), scim.json (SCIM 2.0, 8), webhooks.json (v0 Alpha, 2). Total 58 paths after merge.
- Auth: HTTP Bearer; env var ROAM_API_KEY. Two key tiers: full-access (organization) and personal access token (subset).
- Rate limiting: 10-req burst, 1 req/s sustained, 429 + Retry-After. Adaptive limiter mandatory.
- Architecture: dotted-RPC style paths (e.g., `/chat.post`, `/onair.event.create`). Per-spec base URLs (v0/v1/scim) merged into one CLI via per-operation `servers` overrides.

## Competitive Landscape (Roam HQ specifically)
- **Roam HQ remote MCP** at https://api.ro.am/mcp (official, hosted) — the main competitor; agent-only, no offline cache, no shell composability.
- **No third-party CLIs found** for ro.am (search results conflate with Roam Research note-taking; those are unrelated).
- Clean lane to be the only meaningful local CLI + MCP for Roam HQ.

## Product Thesis
- Name: roam-pp-cli (binary), library slug: roam.
- Why it should exist: Roam ships a remote MCP but no CLI; teams that want shell pipelines, cron jobs, local SQLite cache for offline transcript/chat search, or compliance/SCIM scripting have no good option. We're the only tool that absorbs all five Roam APIs into one binary AND syncs them locally so an agent (or `jq`-pipeline) can search transcripts and chat history without round-tripping the rate-limited API.

## Build Priorities
1. Foundation: SQLite store for chats/messages/transcripts/meetings/recordings/onair_events/guests/users/groups/webhooks; FTS5 on messages and transcript text; sync command per resource family with cursor + date-range modes; adaptive rate limiter with per-tier (free/PAT/full-access) classification.
2. Absorb every endpoint in all 5 specs as typed commands with --json/--select/--dry-run.
3. Transcendence: cross-resource grep ("what did we decide about X?"), stale-meeting reaper, attendance drift, transcript prompt fan-out, chat→Roam relay (stdin pipe → /chat.post), webhook digest (subscribed events shown as a tail), token-tier doctor (which endpoints work with this key?).
