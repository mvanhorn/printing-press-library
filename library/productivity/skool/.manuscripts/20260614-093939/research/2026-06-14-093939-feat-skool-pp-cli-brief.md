# skool CLI Brief

## API Identity
- **Domain:** Skool community management for solo admins/creators.
- **Primary source:** SkoolAPI.com (`https://api.skoolapi.com`) — unofficial REST API, full OpenAPI 3.1 reconstructed from official docs. Auth: `X-Api-Secret` header + a stateful `session_id` (created from Skool email+password).
- **Secondary source:** Apify actor `cristiantala/skool-all-in-one-api` (33 actions) — covers write/admin ops SkoolAPI lacks (member approve/reject/ban, classroom publish, group description, Auto DM, image upload). Auth: `APIFY_API_TOKEN`; called via `POST /v2/acts/cristiantala~skool-all-in-one-api/run-sync-get-dataset-items`.
- **Users:** Solo community admins (the requester runs a Skool community alone and wants to spend time only on content).
- **Data profile:** Posts, chats/messages, members, webhooks, sessions. Small per-call payloads; high value in *aggregating* across them locally.

## Reachability Risk
- **[Medium]** `api.skoolapi.com` returns Cloudflare **503** to unauthenticated probes (stdlib + Surf-Chrome both blocked; `probe-reachability` → `browser_clearance_http`, conf 0.6). Evidence this is an auth-edge artifact, not a true block: SkoolAPI is a paid product with n8n templates and live customers; the probe carried no valid `X-Api-Secret` or session. Treated as the "401 / auth-required, no key" PASS case. Apify actor metadata endpoint returns **200**. Decision: proceed with standard HTTP transport + retry/backoff; do NOT ship browser-clearance transport (this is a server API with secret auth, not a scraped website). Re-validate when a real `SKOOL_API_SECRET` is available.

## Source Priority
- Primary: **skoolapi** — official complete OpenAPI 3.1 — auth: API secret + session (user-supplied, free tier of the wrapper service / their own key).
- Secondary: **apify-skool-actor** — documented action reference — auth: Apify token (paid usage).
- **Economics:** Native SkoolAPI commands form the headline surface. Apify-backed commands (member admin, course publish, autodm, group status, unanswered-post detection) are gated on `APIFY_API_TOKEN` and degrade gracefully with an actionable error when it is absent.
- **Inversion risk:** None — SkoolAPI has the complete spec and is the primary; Apify only fills documented gaps.

## Native Endpoint Surface (SkoolAPI)
- `POST /v1/sessions/` (email,password) → {id,status}; statuses: pending|active|refreshing|authentication_error|internal_error
- `GET /v1/sessions/{id}` → status (poll until `active`)
- `DELETE /v1/sessions/{id}`
- `GET /v1/posts/` (group_id*, page, sort_by[newest|activity], category_id, session_id*) → [PostOut{id,content,title,author:UserOut}]  — **NOTE: no comment count field**
- `GET /v1/chats/` (page*, session_id*) → [ChatOut{id,last_message_id,user:UserOut}]  — **NOTE: no explicit unread flag**
- `GET /v1/chats/{chat_id}` (message_id*, limit≤50*, session_id*) → [MessageOut{id,author_id,content}]
- `POST /v1/chats/{chat_id}` (session_id*, body{content}) → send message
- `POST /v1/chats/{chat_id}/read`, `POST /v1/chats/{chat_id}/unread` (session_id*)
- `GET /v1/webhooks/` (session_id*) → [WebhookOut{id}]
- `POST /v1/webhooks/` (session_id*, body{url,group,events[post|comment|group_stats|chat_update]}) → {id}
- `DELETE /v1/webhooks/{webhook_id}` (session_id*)

## Top Workflows
1. **Inbox zero on chats** — list chats newest-first, reply, mark read (native).
2. **Draft & publish posts from markdown** — write content locally, publish (Apify, since native API is read-only for posts), optionally queue/schedule.
3. **Onboard member waves** — approve all pending, set the welcome Auto DM (Apify).
4. **Find unanswered posts** — surface posts with zero comments to reply (native list + Apify comment trees).
5. **Webhook-driven automation** — register a webhook and tail events in the terminal.

## Table Stakes (must match)
- Session lifecycle with local caching + auto-refresh on `authentication_error`.
- Posts list (paginate, sort, category filter).
- Chat inbox / read / reply / read-all.
- Webhook CRUD.
- Apify: member approve/reject/ban, batch-approve, CSV export, course publish, group description, Auto DM, cover upload.
- Clean table output (gh-style) + `--json` + `--quiet` everywhere; errors to stderr.

## Data Layer (local SQLite mirror)
- **Primary entities:** posts, chats, messages, members (synced via Apify), webhooks, sessions.
- **Sync cursor:** posts by page+sort; chats by page; messages by `message_id` cursor (limit 50).
- **FTS/search:** full-text over posts.content/title and messages.content.
- **Enables transcendence:** offline queries, `skool sql`, unanswered detection, response-time stats, member-since analytics, content calendar.

## Session Handling (hand-wired over generated endpoints)
- Native endpoints all require a `session_id` query param. Cache the active session in `~/.skool/session.json`; auto-inject into every native call; poll-to-active after create; `skool session refresh`; auto-recreate on `authentication_error`.

## Product Thesis
- **Name:** skool
- **Why it should exist:** Existing tools are either the Apify actor (powerful but cloud-bound, no local state, no offline querying) or raw SkoolAPI (no ergonomics). `skool` unifies both behind one binary, adds a local SQLite mirror nobody else has (offline + `sql` + compound analytics), ships agent-native output and an MCP server, and bakes the 12 solo-admin workflows into first-class commands. A solo admin can run their whole community from the terminal or let Claude drive it.

## Build Priorities
1. **P0 foundation:** generate native endpoints from OpenAPI; session manager (cache/poll/refresh/auto-inject); config (`~/.skool/config.yaml` + `.env`); SQLite store for all entities; `sync`; `sql`; `search`.
2. **P1 absorb:** all 12 compound commands — `post draft/list[--unanswered]`, `member approve-all/export/ban/approve/reject`, `chat inbox/reply/read-all`, `webhook watch/list/create/delete`, `autodm set`, `course publish`, `group status`. Apify client wrapper with retry/backoff + structured-error handling.
3. **P2 transcend:** offline analytics over the local mirror (response-time, unanswered queue, content calendar, member onboarding funnel, chat-leaderboard) — only possible because everything is mirrored locally.
4. **P3 polish:** MCP server (cobratree mirror), Claude Code skill describing every compound command, `--help` examples, doctor.

## User Vision (verbatim intent)
Solo Skool admin wants one binary they can run themselves AND let Claude Code drive as an agent, to handle every admin/content task programmatically and focus exclusively on creating content. Explicitly requested: 12 compound "magic moments", local SQLite mirror, `skool sql`, `skool sync`, MCP server, Claude skill, retry/backoff, `--json`/`--quiet`/`--help` everywhere, config via `~/.skool/config.yaml`/`.env`, default `SKOOL_GROUP_ID`.
