# Nylas CLI Brief

## API Identity
- **Domain:** Unified API for Email, Calendar, Contacts, Scheduling, Notetaker (meeting bots), Webhooks, Admin (apps/keys/domains), Migration (v2→v3) across Google, Microsoft, IMAP, EWS, Yahoo.
- **Users:** B2B SaaS developers embedding email/calendar/contacts; AI agent builders needing typed inbox tools; meeting-intelligence platforms; healthcare/finance teams needing audit-logged communications.
- **Data profile:** Per-grant resources (messages, threads, drafts, folders, attachments, signatures, templates, calendars, events, contacts, notetakers, workflows, scheduling configs) plus tenant-wide resources (applications, connectors, redirect URIs, API keys, domains, webhooks, channels, rules, policies, lists, workspaces).

## Reachability Risk
- **None.** Public OpenAPI 3.1 spec at `https://developer.nylas.com/_spec-files/nylas-api.yaml` (1.6 MB, 121 paths). Both US/EU API endpoints respond HTTP 200. Auth is bearer (API key). Nylas publishes `llms.txt`, `.well-known/agent-skills`, `.well-known/api-catalog`, and `.well-known/mcp/server-card.json` — they have done first-class agent-readiness work.

## Top Workflows
1. **"What changed in my inbox/calendar since X?"** — Incremental sync of messages/events/contacts against the local store, then `<since 2h>` style time-windowed queries.
2. **"Find every thread with <person> across all my grants"** — Cross-grant FTS5 search joined to contacts, which the upstream API cannot do in a single call.
3. **"Schedule a meeting with X across timezones, find the slot"** — Availability + find-time + book + RSVP, often paired with Notetaker bot dispatch.
4. **"Draft and send transactionally with smart-compose, audit the action"** — Smart-compose → preview → confirm → send → audit log entry, with idempotent-send guarantees.
5. **"Webhook plumbing in dev"** — Create webhook, send test event, rotate secret, replay events locally without a public URL.
6. **"Notetaker bot for an upcoming meeting"** — Invite the bot, monitor history, fetch transcript + audio + video media after the call.

## Top Table Stakes (from incumbent `nylas-cli` v3.1.1)
- Auth: login, whoami, grants list/show/switch/add/remove/revoke, OAuth + BYO, scopes, providers detect, v2→v3 migrate
- Email: list/read/send/search/mark, draft CRUD, threads, folders, attachments, smart-compose, templates, signatures, scheduled-send
- Calendar: calendars CRUD, events CRUD, free-busy, availability, find-time, RSVP, AI conflicts/reschedule
- Contacts: CRUD, search, sync, groups
- Webhooks: CRUD, triggers, test events, local webhook server, rotate secret
- Notetaker: invite/list/show/history/media/cancel/leave
- Admin: applications, API keys, redirect URIs, domains (custom-domain send), connectors + creds
- MCP: install for Claude Code/Desktop/Cursor/Windsurf/VS Code; serve local MCP
- Scheduling v3: configurations, sessions, bookings, group events
- Audit logs

## Data Layer
- **Primary entities to persist:** messages, threads, drafts, folders, attachments (metadata only), calendars, events, contacts, notetakers, scheduling configurations, bookings, grants, webhooks, templates, signatures, workflows, rules, policies.
- **Per-grant scoping:** every resource is namespaced under a `grant_id`. The store key is `(grant_id, resource_type, id)`.
- **Sync cursor:** Nylas exposes `next_cursor` pagination on list endpoints and webhook deltas (`message.created`, `event.updated`, `grant.expired`, etc.). The CLI maintains per-(grant, resource_type) cursor state plus a `last_sync_at` timestamp.
- **FTS5:** virtual tables on messages (subject + body_plain + from + to), threads (subjects + participants), events (title + description + location + participants), contacts (display_name + emails + phones).
- **High-gravity fields:** messages — id, thread_id, subject, from, to, snippet, date, unread, starred, folder_id, has_attachments. Events — id, calendar_id, title, when (start/end), participants, location, status, organizer. Contacts — id, emails, phones, given_name, surname, company.

## Codebase Intelligence
- **Source:** Direct OpenAPI 3.1 spec + Nylas-published `nylas-cli` v3.1.1 SKILL ([nylas/skills](https://github.com/nylas/skills)) + Nylas MCP server-card (17 typed tools, two-step confirm-then-send safety pattern).
- **Auth:** `Authorization: Bearer <NYLAS_API_KEY>` for application-level; `Bearer <ACCESS_TOKEN>` for per-user; `Bearer <SCHEDULER_SESSION_TOKEN>` for public Scheduler endpoints. Env: `NYLAS_API_KEY`. Grant ID is a request path parameter for all per-user resources, NOT an auth header.
- **Data model:** Per-grant resources (`/v3/grants/{grant_id}/...`, 54 paths) carry the bulk of email/calendar/contacts/notetaker. Tenant resources (`/v3/applications`, `/v3/connectors`, `/v3/webhooks`, etc.) are global.
- **Rate limiting:** Per-app, per-endpoint, with `Retry-After` headers. Compression (`Accept-Encoding: gzip`) recommended for messages list.
- **Architecture insight:** Two-tier auth (API key + grant_id) gives application-level access to every grant, which makes cross-grant offline aggregation straightforward — exactly the gap in the incumbent CLI which is grant-at-a-time and stateless.

## Source Priority
- Single source: official Nylas v3 OpenAPI spec. No combo.

## Product Thesis
- **Name:** `nylas-pp-cli`
- **Why it should exist:** The incumbent `nylas-cli` is feature-rich but **stateless** — every command hits the live API, single-grant-at-a-time. Power users and AI agents need a CLI that builds a **local SQLite mirror** of inbox/calendar/contacts across **all grants at once**, can answer "what changed since 2h ago?" without N round-trips, lets agents compose **SQL** on synced data, and exposes a **uniform `--json --select --agent` surface** with typed exit codes. Every Nylas feature, plus a local store no other Nylas CLI has.

## Build Priorities
1. **P0 — Data layer:** SQLite store with `resources` table keyed by `(grant_id, resource_type, id)`, plus typed FTS5 tables for messages/threads/events/contacts. Cursor state per (grant, resource_type).
2. **P1 — Absorb the incumbent's full feature set:** Auth + grants management, Email (read/send/search/mark/drafts/threads/folders/attachments/smart-compose/templates/signatures/scheduled-send), Calendar (CRUD/free-busy/availability/find-time/RSVP), Contacts (CRUD/groups), Webhooks (CRUD/rotate/test events), Notetaker (invite/history/media), Admin (applications/keys/redirect URIs/domains/connectors), Scheduling v3 (configs/sessions/bookings/group events), Audit logs (read-only stream).
3. **P2 — Transcend with local-store-only features:** see absorb manifest.
4. **P3 — Polish:** smart-compose with `--dry-run` preview, MCP server with proper read-only/destructive annotations matching Nylas's own safety pattern.

## Risks / Notes
- Smart-compose, send, draft-send, RSVP, and notetaker invite are **destructive/external-side-effect** commands. They must short-circuit under `PRINTING_PRESS_VERIFY=1` and default-print under verify, matching the incumbent's confirm-before-send pattern from the Nylas MCP server-card.
- The spec is large (1.6 MB, 121 paths). Generation will produce a substantial command tree; absorb scope is real work. Plan to run polish at least once.
- Nylas's own MCP server (`mcp.us.nylas.com`) is the obvious comparison point. Our printed-CLI MCP must justify its existence by exposing the local-store and cross-grant tools their server doesn't have.
