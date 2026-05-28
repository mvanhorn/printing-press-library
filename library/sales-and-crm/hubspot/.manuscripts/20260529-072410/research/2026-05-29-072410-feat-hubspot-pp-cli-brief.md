# HubSpot CRM Contacts CLI Brief

## API Identity
- Domain: HubSpot CRM Contacts v3 (`api.hubapi.com/crm/v3/objects/contacts/*`)
- Users: Sales ops, RevOps, marketing ops, BDRs, sales managers
- Data profile: Contact records with engagement metrics, lifecycle stage, owner, score, source attribution, activity timestamps. Modest entity volume per workspace (10k-1M contacts typical) but rich property surface (~100+ default properties + customs).

## Reachability Risk
- None. `GET /crm/v3/objects/contacts` returns HTTP 401 with no token (expected); auth-required is the documented state.
- Tier/permission hints from 4xx body: omit when absent
- Probe-safe endpoint used: `GET /crm/v3/objects/contacts?limit=1`

## Top Workflows
1. **Detect engagement decay** — "who's gone dark on us in the last N days, especially previously-hot contacts"
2. **Lifecycle hygiene** — "who's stuck in MQL/SQL past SLA, which records are duplicates, who's under-completed"
3. **Source ROI** — "which acquisition channels actually produce revenue vs. tire-kickers"
4. **Owner-load balance** — "which reps are overloaded with active contacts, who has stale unworked queues"
5. **Score-trend tracking** — "whose score dropped meaningfully over the window (early disengagement signal)"

## Table Stakes
- List contacts with pagination
- Get contact by id / by email
- Search with filterGroups (UI filter builder equivalent)
- Properties schema fetch
- Batch read for bulk sync
- Associations (read-only; deals/companies linkage)

## Data Layer
- Primary entities: `contacts` (the API gives them all-in-one shape; engagement metrics, lifecycle, owner, source live on the contact record as properties)
- Sync cursor: `paging.next.after` (cursor-based, no offset)
- Snapshot tables: `contact_score_history` and `contact_engagement_history` for cross-time queries (score-drift, decay)
- FTS/search: contact name, email, company; full-text over `notes_body` if synced

## Codebase Intelligence
- Source: HubSpot's official Python/Node SDKs (`hubspot-api-python` 422★, `hubspot-api-nodejs` 397★) for endpoint discovery
- Auth: Bearer private-app token via `Authorization: Bearer <token>` header
- Rate limiting: 100 req/10s burst, 250k/day soft limit for typical apps; `X-HubSpot-RateLimit-*` headers in responses
- Search endpoint: 6 filterGroups max (AND between, OR within), 200 results per page, `query` ≤3000 chars, sortable

## User Vision
Single-token read-only contact-analytics CLI. The narrow scope (HubSpot CRM Contacts only, not the full HubSpot CRM/Marketing surface) is intentional — ship now, expand later via amend PRs covering Companies, Deals, and Tickets as their own catalog entries appear. Differentiator vs. every existing tool is the cross-time analytics that HubSpot's own UI can't render without Operations Hub Pro: score-drift, engagement-decay, lifecycle-stuck, stale-but-valuable, daily-digest.

## Product Thesis
- Name: `hubspot-pp-cli`
- Why it should exist: Every HubSpot CLI on GitHub is either CMS dev tooling (HubSpot's own 195★ CLI) or generic CRUD wrappers. Every HubSpot MCP server is conversational/write-heavy. Nobody ships an analytics tool — yet sales ops teams export contact data to spreadsheets every week to answer exactly the questions in Top Workflows. A read-only Go CLI with a local SQLite store and a dozen pivots-as-commands fills a clear, repeatedly-validated gap.

## Build Priorities
1. **Spec coverage (Priority 0/1)** — All 11 contacts endpoints (list, get, search, batch read, properties schema) wired with proper pagination, filter language, and `--select` field projection
2. **Engagement & score snapshot store (Priority 0)** — Local tables for score history and last-activity history, populated on every `sync`
3. **Transcendence commands (Priority 2)** — engagement-decay, lifecycle-stuck, stale-but-valuable, source-roi, score-drift, owner-overload, silent-after-first-touch, duplicate-suspects, daily-digest. Drop property-completion-score, activity-cliff, territory-gap to keep scope tight (~9 novel commands, in line with PR #911's 8 and PR #915's 5).
