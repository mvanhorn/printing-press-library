# GoHighLevel (GHL) CLI Brief

## API Identity
- **Domain:** marketing/CRM platform. Contacts, conversations (SMS/email/voice), calendars, opportunities (sales pipelines), workflows (automation), tags, custom fields, custom values.
- **Users (this run):** i2 Fitness operator (Alex). GHL is the system of record across
  [[i2-ai-nurture]] (Riley SMS/voice), [[trainer-dashboard]] (coach UI),
  [[business-dashboard]] (KPI cockpit). The CLI must be safe for an AI agent (Riley)
  to read while never bypassing the documented kill-switch tags.
- **Data profile:** medium-volume CRM (thousands of contacts, hundreds of
  conversations/day, dozens of pipelines & stages). Reads vastly outnumber writes.
  Phone is a top-gravity lookup key.

## Reachability
- **PASS.** Reachability probe: `GET https://services.leadconnectorhq.com/locations/search?limit=1`
  with `Authorization: Bearer pit-test` + `Version: 2021-07-28` returns
  `HTTP 401 {"statusCode":401,"message":"Invalid Private Integration token"}` —
  spec is correct and the API rejects only on auth, not on shape.
- Risk level: **None.** Official spec, official MCP just shipped, active vendor.

## Auth (load-bearing)
- **PIT (Private Integration Token), location-scoped.** Generated in the
  sub-account UI under Settings → Private Integrations.
- Sent as `Authorization: Bearer <pit>`.
- Every request also requires `Version: 2021-07-28` — stripped from the spec
  and injected by the printed CLI's HTTP client so it doesn't pollute every command.
- Stored via the CLI's own `auth` subcommand (per user spec; not pasted into
  the OpenAPI doc, not written to git-tracked files).
- Two PIT modes exist in HighLevel: Sub-Account (location) and Agency. v1
  targets Sub-Account only — that matches every i2 Fitness use case.

## Top Workflows (priority order from user)
1. **Contacts.** List with rich filters (tag, date range, custom field, phone match);
   get; update; add/remove tags; search by phone/email. *Kill-switch tags must be
   visible in `contact-get` default output, not hidden behind `--full`.*
2. **Conversations.** List; get full thread; send SMS; send email; search messages
   by keyword.
3. **Calendars.** List calendars; list appointments by date range; get availability
   (free slots); create/cancel appointment.
4. **Opportunities.** List by pipeline + stage; get; update stage; update monetary
   value.
5. **Workflows.** List; list contacts currently in a workflow; trigger a contact
   into a workflow.
6. **Custom fields & values.** Read + update on contacts.
7. **Tags.** List all tags in a location; count contacts per tag.
8. **Activity.** Recent contact activity; recent conversation activity (24h / 7d).

## Table Stakes (must match or beat)
Inventoried against the **official HighLevel MCP** (gold standard, ships 2026-Q1 with
~35 tools) plus the top community SDK
([@gohighlevel/api-client](https://www.npmjs.com/package/@gohighlevel/api-client)) and the
top community MCP ([mastanley13/GoHighLevel-MCP](https://github.com/mastanley13/GoHighLevel-MCP)).

Surface every competitor has:
- Get/update/upsert contact
- Add / remove tags on contact
- Get tasks on contact
- Search conversation, send a message, fetch messages by ID
- Get/search opportunity, list pipelines, update opportunity
- Get calendar events
- Get location, get custom fields, get appointment notes

Gaps in all of them (our absorb-and-beat):
- **Workflows** — not in the official MCP at all. Trigger / list contacts in workflow
  is exactly the surface i2-ai-nurture needs.
- **Tag analytics** — official MCP can add/remove but cannot list all tags in a
  location or count contacts per tag.
- **Activity windowing** (`last 24h`, `last 7d`) — none expose this as a first-class
  surface.
- **Custom value writes** — MCP is read-only on custom fields and ignores custom
  values entirely.
- **Phone/email search ergonomics** — the API supports it (`/contacts/search`) but
  no tool exposes it as a one-liner.
- **Kill-switch awareness** — no tool surfaces `ai off` / `human handover` as
  first-class signals. This is unique to i2's workflow.

## Data Layer
- **Primary entities:** contacts, conversations, messages, calendars, appointments,
  opportunities, pipelines, stages, workflows, tags, custom_fields, custom_values,
  locations, users.
- **Sync cursor:** GHL uses cursor-style pagination (`startAfter` timestamp +
  `startAfterId`). Sync state stores per-resource cursor.
- **FTS / search:** SQLite FTS5 over `contacts.name`, `contacts.email`,
  `contacts.phone`, `messages.body`, `opportunities.name`, `appointments.title`,
  `tags.name`.

## Codebase Intelligence
- Source: GoHighLevel/highlevel-api-docs (the official Stoplight repo on GitHub).
  Per-resource OpenAPI 3.0 JSON files. Merged for this run into a single 103-path,
  258-schema document at `$RESEARCH_DIR/ghl-merged-openapi.json`.
- Auth pattern (from spec): `bearer` http scheme, PIT JWT in `Authorization` header.
- Required header pattern: `Version: 2021-07-28` on every request — defaulted by
  the printed client.
- Data model: location-scoped. Most list endpoints accept a `locationId` query
  param; some endpoints embed it in the path (`/locations/{locationId}/tags`,
  `/locations/{locationId}/customFields`, `/locations/{locationId}/customValues`).
- Rate limiting: per the GHL docs, ~10 req/sec per location; the CLI should respect
  this and surface a typed retry-after error (`*cliutil.RateLimitError`).
- Pagination shape: `startAfter` (epoch ms timestamp) + `startAfterId` cursor;
  defaults `limit=20`, max `limit=100`.

## User Vision
- "GHL is the system of record across [[i2-ai-nurture]] (Riley SMS/voice),
  [[trainer-dashboard]], [[business-dashboard]]."
- "Kill-switch tags `ai off` (silent skip) and `human handover` (transfer to Alex
  at +1-555-0100) MUST be readable in contact-get output by default — don't
  hide them behind --full."
- "contact-list default: one line per contact (id, name, phone, top 3 tags).
   conversation-list default: one line per conversation (id, contact name,
   last message timestamp, unread). Full payloads behind --full or --select."
- "After Phase 1 research, show me the feature catalog before generating.
   I want to confirm coverage of kill-switch tags + custom fields before you build."

## Product Thesis
- **Name:** `ghl-pp-cli` (binary), `pp-ghl` (skill).
- **Why it should exist:** every other GHL tool is either a code-heavy SDK
  (developer-only) or an MCP server with no offline state, no fan-out search,
  no activity windowing, no workflow trigger, no kill-switch awareness, and
  no terse list mode. This CLI absorbs the official MCP's surface, beats it
  with a local SQLite cache, adds the four missing categories (workflows /
  tag analytics / activity / custom values), and reads kill-switch tags as a
  first-class signal so Riley and the trainer dashboard can rely on it.

## Build Priorities
1. **Generator handles foundation + most absorb** — typed endpoint mirrors for
   all 103 paths, store schema for the 8 primary entities, sync/search/SQL path,
   auth handshake, MCP surface (Cloudflare pattern given >100 tools).
2. **Hand-build the i2-specific defaults** — terse contact-list and
   conversation-list; kill-switch tag visibility in contact-get; default phone
   formatting (E.164 normalization on lookup).
3. **Hand-build the four missing transcendence categories** — workflow trigger,
   tag analytics, activity window, custom-value writes.
4. **Polish + dogfood with the user's real PIT** (read-only smoke first).

## Source Catalog (alternatives credited in README)
- [GoHighLevel/highlevel-api-docs](https://github.com/GoHighLevel/highlevel-api-docs) — official OpenAPI source (origin of merged spec)
- [Official HighLevel MCP Server](https://marketplace.gohighlevel.com/docs/other/mcp/) — vendor's own MCP, ~35 tools
- [mastanley13/GoHighLevel-MCP](https://github.com/mastanley13/GoHighLevel-MCP) — community MCP, broader coverage
- [basicmachines-co/open-ghl-mcp](https://github.com/basicmachines-co/open-ghl-mcp) — OAuth-based MCP, contact-heavy
- [BusyBee3333/Go-High-Level-MCP-2026-Complete](https://github.com/BusyBee3333/Go-High-Level-MCP-2026-Complete) — 520+ tools, exhaustive surface
- [@gohighlevel/api-client (npm)](https://www.npmjs.com/package/@gohighlevel/api-client) — official TS/JS SDK
- [M2KDevelopments/gohighlevel](https://github.com/M2KDevelopments/gohighlevel) — community Node SDK
