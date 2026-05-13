# HubSpot CLI Brief

## API Identity
- **Domain:** Sales / Marketing CRM. Pre-sale and post-sale customer record system.
- **Users (this CLI's scope):** SMB automation operators and AI agents driving pre-sale workflows — solo operators, n8n flows, Apollo enrichment pipelines, Google Maps prospect scrapers, Claude Code sessions.
- **Data profile:** Polymorphic CRM objects (contacts, deals, companies, leads, custom objects) with thousands of properties (standard + custom). High-volume engagement events (calls, emails, meetings, notes, tasks) attached as one-to-many associations. Pipelines with stages drive deal lifecycle. List membership drives segmentation.

## Reachability Risk
- **None.** Public REST API at `api.hubapi.com` with documented OpenAPI 3.0 specs per resource. 99.9% SLA. Stable v3 endpoints for years. Rate limits per private app token: 110 req/10s burst, 250k/day for Pro/Ent. No bot protection on programmatic access.

## Scope (Pre-Sale Only)
**In:** contacts, deals, companies, leads, owners, pipelines, properties, lists, associations, engagements (calls/emails/meetings/notes/tasks).
**Out (per user briefing):** Marketing Hub email sends, sequences, CMS, blog, landing pages, forms, workflow creation.

## Top Workflows (from user briefing, exact)
1. **Stale leads** — contacts in stage X untouched N days, sorted by last activity
2. **Pipeline health** — deals by stage with age, weighted value, days-since-last-activity
3. **Recent intake** — new contacts last 24/48h with source + UTM attribution
4. **Dedup check** — potential duplicates across email/phone/domain
5. **Closed Won handoff** — every contact moved to Closed Won this week, full property bundle, ClickUp-import-ready
6. **Engagement decay** — contacts whose email open/click rate has dropped off

## Table Stakes (from competitor matrix)
- list / get / search / batch-read for every CRM object (Contacts, Deals, Companies, Leads, Tickets-omit, Products-omit, Quotes-omit)
- list / get / search for every engagement (Calls, Emails, Meetings, Notes, Tasks)
- list / search for Owners, Pipelines, Properties, Lists, Associations
- Filter-based search via `/crm/v3/objects/{type}/search` (filterGroups, sorts, properties projection)
- Batch read up to 100 IDs at once
- Property discovery (`/crm/v3/properties/{objectType}`)
- Pipeline + stage discovery (`/crm/v3/pipelines/{objectType}`)
- Owner directory (`/crm/v3/owners`)
- Auth via private app token (`Authorization: Bearer <token>`)
- Structured JSON output, exit codes by error class, machine-readable errors
- Rate-limit-aware backoff

## Data Layer (SQLite local store)
**Primary entities (sync targets):**
- `contacts` (lastmodifieddate cursor)
- `deals` (hs_lastmodifieddate cursor)
- `companies` (hs_lastmodifieddate cursor)
- `leads` (hs_lastmodifieddate cursor)
- `owners` (cached; rarely change)
- `pipelines` (cached; rarely change)
- `properties` (cached per objectType; metadata)
- `lists` (cached)
- `engagements` (each type: calls, emails, meetings, notes, tasks; hs_lastmodifieddate cursor)
- `associations` (join table contacts↔deals↔companies)

**Sync cursor strategy:** `/crm/v3/objects/{type}/search` with filter `hs_lastmodifieddate > <last_sync>`. Properties projection to keep payload small. Paginate via `after` cursor.

**FTS index:** Full-text search across contacts (firstname, lastname, email, company, jobtitle), deals (dealname, description), companies (name, domain, description).

## Codebase Intelligence
- **shinzo-labs/hubspot-mcp** (TypeScript): exhaustive endpoint-mirror MCP. Confirms full CRM resource list + engagement types + association types. Auth: `Authorization: Bearer ${HUBSPOT_ACCESS_TOKEN}`.
- **baryhuang/mcp-hubspot** (Python): FAISS vector store on retrieved data, SentenceTransformer embeddings, duplicate prevention on create. Confirms semantic search as compelling pattern.
- **dipankar/hubspot-cli** (Go): batch operations capped at 100, exit-code semantics (0/3/5/6/7/8), discovery commands (`discover objects|properties|rate-limits`), Claude skill integration.
- **HubSpot/hubspot-api-nodejs** (official): Confirms v3 endpoint paths, auth header, query param shapes. `/crm/v3/objects/{type}/search` is the universal filter endpoint.
- **HubSpot official MCP** (remote): Full CRUD over the canonical object set including engagement objects. Sensitive data properties excluded.

## User Vision (verbatim from briefing)
Simple Path Media is SMB automation agency. HubSpot is canonical CRM for pre-sale lead-gen (contractors, HVAC, realtors, home services). Used solo + by n8n flows pushing leads from Apollo and a daily Google Maps realtor scraper.

**Architectural rule:** Pre-client work lives in HubSpot only. The moment a deal hits Closed Won, the contact moves to ClickUp. All hot queries are pre-sale: lead status, deal stage, recent engagement, stale leads, source attribution.

**Token economy:** Default to `--compact` + auto-JSON on every list command. Token cost matters more than terminal pretty-print. CLI will be called from n8n flows and Claude Code sessions thousands of times.

**Auth:** Private app token, `$HUBSPOT_TOKEN`. Read scopes: contacts, deals, companies, engagements, owners, properties.

## Product Thesis
- **Name:** `hubspot-pp-cli`
- **Why it should exist:** Every existing HubSpot CLI/MCP is a 1:1 endpoint mirror. None of them have a local SQLite cache, none of them can answer the six compound queries above in one shot, and none of them default to `--compact` JSON for agent consumption. dipankar/hubspot-cli has clean ergonomics but no offline store and no compound analytics. The official MCP has full coverage but loads ~80 tools per session and burns context, has no offline cache, and cannot compute "stale leads over 14 days" in one tool call.
- **Position:** "Every CRM endpoint, plus a local SQLite mirror that powers six pre-sale compound queries no other HubSpot tool can answer in one call."

## Build Priorities
1. **Generator-produced surface (Phase 2):** Contacts, Deals, Companies, Leads, Owners, Pipelines, Properties, Lists, Associations, Calls, Emails, Meetings, Notes, Tasks — list/get/search/batch-read absorbed from official OpenAPI specs.
2. **Data layer (Phase 3 P0):** SQLite store, sync command per resource with `hs_lastmodifieddate` cursor, FTS index, `sql`/`search`/`context`/`stale`/`orphans`/`reconcile` framework commands.
3. **Transcendence (Phase 3 P2):** stale-leads, pipeline-health, recent-intake, dedup, closed-won-handoff, engagement-decay.
4. **Token economy polish:** `--compact` + `--select` defaults for list commands; ensure auto-JSON when not on TTY.

## Auth
- **Type:** `bearer_token`
- **Header:** `Authorization: Bearer <token>`
- **Env var:** `HUBSPOT_TOKEN`
- **Scopes (read-only for this CLI):** `crm.objects.contacts.read`, `crm.objects.deals.read`, `crm.objects.companies.read`, `crm.objects.leads.read`, `crm.objects.owners.read`, `crm.schemas.contacts.read`, `crm.schemas.deals.read`, `crm.schemas.companies.read`, `tickets.read` (omitted), `crm.lists.read`, `crm.associations.read`, plus engagement-specific read scopes.

## Spec Sources (multi-spec generate)
Base: `https://raw.githubusercontent.com/HubSpot/HubSpot-public-api-spec-collection/main/PublicApiSpecs/CRM/`

| Resource | Path |
|---|---|
| Contacts | `Contacts/Rollouts/424/v3/contacts.json` |
| Deals | `Deals/Rollouts/424/v3/deals.json` |
| Companies | `Companies/Rollouts/424/v3/companies.json` |
| Leads | `Leads/Rollouts/424/v3/leads.json` |
| Owners | `Crm%20Owners/Rollouts/146888/v3/crmOwners.json` |
| Properties | `Properties/Rollouts/145899/v3/properties.json` |
| Pipelines | `Pipelines/Rollouts/145896/v3/pipelines.json` |
| Lists | `Lists/Rollouts/144891/v3/lists.json` |
| Associations | `Associations/Rollouts/130902/v4/associations.json` |
| Calls | `Calls/Rollouts/424/v3/calls.json` |
| Emails | `Emails/Rollouts/424/v3/emails.json` |
| Meetings | `Meetings/Rollouts/424/v3/meetings.json` |
| Notes | `Notes/Rollouts/424/v3/notes.json` |
| Tasks | `Tasks/Rollouts/424/v3/tasks.json` |
