# GoHighLevel (HighLevel) CLI Brief

## API Identity
- **Domain:** All-in-one CRM, marketing automation, agency platform. White-label SaaS sold to agencies and resellers.
- **Users:** Agencies managing 10-1000s of sub-accounts (locations); marketers building funnels; sales teams running pipelines; SaaS-on-top resellers; ops/automation engineers; support teams handling SMS/email/voice conversations.
- **Data profile:** High-cardinality. Each agency runs N sub-accounts (locations). Each location holds contacts, conversations, opportunities, calendars, invoices, products, campaigns, workflows, social posts. Daily change rate is high (new leads, message threads, pipeline movement). Read-heavy for reporting; write-heavy for messaging/automation.
- **Base URL:** `https://services.leadconnectorhq.com`
- **API surface:** 409 documented REST endpoints across 41 resource groups. Every endpoint requires a `Version` header (e.g., `2021-07-28`) plus an `Authorization: Bearer <token>` header.

## Reachability Risk
- **Low.** Probe `GET https://services.leadconnectorhq.com/contacts/` returned `401 {"statusCode":401,"message":"version header was not found."}` in 118ms. API is alive, fast, and expects standard headers. No anti-bot layer detected.
- Token gating is the only access barrier. Marketplace OAuth or Private Integration Token (PIT) both work; PIT is the simpler path for a CLI.

## Top Workflows
1. **Search and triage contacts/conversations.** `contacts search "name OR email"`, list unread/unanswered messages by location, filter by tag/source/funnel, find contacts assigned to me.
2. **Move opportunities through pipeline stages.** List opportunities by pipeline+stage, bulk-update stage, look at stale opportunities, calculate stage velocity.
3. **Send and read messages (SMS/email/conversation threads).** Send SMS to contact, read inbound thread, reply, schedule a follow-up, mark conversation as read/closed.
4. **Manage calendar appointments.** List today's bookings, find free slots for a calendar, create an appointment for a contact, reschedule, cancel.
5. **Invoice / payment review.** List unpaid invoices, see paid this week, send a reminder, mark as paid, create an invoice for a contact.
6. **Multi-location ops (agency).** Operate across all sub-accounts: rollups of leads/revenue/sent messages by location, switch context between locations, list all locations.

## Table Stakes (every competing tool has these)
- contacts: search, list, get, create, update, delete, tag/untag, custom field set
- conversations: list threads, get messages in thread, send SMS/email
- calendars: list calendars, list events, list free slots, create/cancel event
- opportunities: list (filter by pipeline/stage/assignee), update stage, get
- invoices: list, get, send, mark paid, void
- payments: list transactions, list subscriptions
- workflows: list, trigger workflow for contact (add/remove)
- locations (sub-accounts): list, get, switch
- forms / surveys: list submissions
- products / store: list, get
- social posting: list scheduled, list accounts
- voice AI / phone: list numbers, list call recordings
- users: list staff, list teams
- custom-fields, tags, custom-objects, snapshots, knowledge-base, blogs, links

## Data Layer
- **Primary entities:** contacts, conversations, messages, opportunities, pipelines, stages, calendars, calendar_events, invoices, payment_transactions, products, prices, locations, users, tags, custom_fields, custom_objects, workflows, forms, form_submissions, social_accounts, social_posts.
- **Sync cursor:** GHL endpoints largely use `startAfter` / `startAfterId` cursors plus `updatedAt` filtering. Contact search supports complex filters; sync via `contacts/search` with `dateUpdated` ordering.
- **FTS/search:** SQLite FTS5 over contacts (name, email, phone, tags, custom field text), opportunities (name, contact, monetaryValue, stage), conversation messages (body, fromName, attachments), invoices (number, contact, status, amount). Cross-entity search is the killer offline feature.
- **Local schema rationale:** Power users juggle 10-100 locations. A local SQLite per agency cuts API calls 10-100x for daily triage, makes regex/SQL queries possible, and powers the cross-cutting transcendence commands.

## Codebase Intelligence (DeepWiki / source-read summary)
- Source: github.com/GoHighLevel/highlevel-api-docs (official spec repo, 111 stars). Each app group (`apps/contacts.json`, `apps/conversations.json`, ...) is a complete OpenAPI 3.0 document with `info`, `servers`, `paths`, `components.schemas`, `components.securitySchemes`. Merged into one canonical spec for generation.
- Auth: `Authorization: Bearer <jwt>` plus `Version: 2021-07-28` header on every request. Security schemes scoped: `bearer` (general), `Location-Access` (sub-account), `Agency-Access` (parent company). PITs map to `bearer` with location-scoped or company-scoped scopes.
- Data model: agency → locations → (contacts, conversations, opportunities...). The `locationId` query param threads through almost every list endpoint.
- Rate limiting: documented as 100 requests / 10 seconds per resource per location, with daily quotas. Burst tolerated. We need typed rate-limit handling.
- Architecture insight: "Search Contacts" (`POST /contacts/search`) is more powerful than `GET /contacts` and supports complex compound filters; CLI should expose it as the canonical contact list.

## User Vision
(no upfront briefing context provided — proceeding with workflow-derived defaults; user said "work without stopping")

## Source Priority
- Single source: `gohighlevel` (official OpenAPI spec). No combo CLI; merged spec is canonical.

## Product Thesis
- **Name:** `gohighlevel-pp-cli` (binary), slug `gohighlevel`.
- **Why it should exist:** Every existing GHL tool today is a Claude-Desktop MCP server (mastanley13: 269 tools, BusyBee3333: 520 tools, basicmachines: open-source, tenfoldmarc: 70). They cover the API surface but offer no offline state, no cross-cutting analysis, no scriptable terminal workflow, no multi-location rollup, no agent-native exit codes. A real CLI with a local SQLite gives ops/agency-side users:
  - **Real triage in one shell command** ("`ghl unread --location all --since 1h`") instead of 5 MCP roundtrips
  - **Cross-location analytics** ("`ghl roster --metric revenue-30d`") that no MCP server offers because they're per-call only
  - **Bulk ops with `--dry-run`** that the marketplace UI makes painful (re-tag 500 contacts, move 30 opportunities)
  - **Agent-native shape** (`--json --select`, typed exit codes, `agent-context`) that wraps every endpoint

## Build Priorities
1. **Foundation (Priority 0):** Generated client + auth (Bearer + Version header), config (PIT in env), data layer for the 8 highest-value entities (contacts, conversations, opportunities, calendars, invoices, locations, users, tags).
2. **Absorbed surface (Priority 1):** All 409 endpoints from the merged spec, MCP-exposed via Cloudflare pattern (search+execute orchestration tools, hidden endpoint-mirror tools, stdio+http transport). Endpoint-mirror tools must inject the `Version` header automatically.
3. **Transcendence (Priority 2):** Cross-location triage, stale opportunities, conversation freshness, pipeline velocity, contact dedup, multi-location roster, message-volume rollup, "since I was last here" agent context, contact graph (custom-object joins), bulk-tag + bulk-stage with dry-run.
4. **Polish (Priority 3):** Naming cleanup for ugly operationIds, tests for store/triage logic, narrative+recipes wired to the SKILL.

