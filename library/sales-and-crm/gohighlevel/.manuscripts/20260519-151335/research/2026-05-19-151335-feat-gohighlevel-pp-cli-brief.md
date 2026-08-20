# GoHighLevel CLI Brief

## API Identity
- Domain: SaaS marketing/CRM platform (formerly HighLevel, now leadconnectorhq.com for API endpoints). Widely used by digital marketing agencies and real estate brokerages.
- Users: Agency operators, multi-location brokerages (KWCP runs DC/MD/VA Keller Williams offices on it), in-house operators automating recruiting + lead funnels.
- Data profile: Multi-tenant per "location" (sub-account). Heavy on contacts, opportunities, conversations, calendars, custom fields. Reference data (pipelines, stages, tags, custom fields, users) is high-cardinality but rarely changes. Transactional data (contacts, opportunities, appointments) churns daily.

## Reachability Risk
- [Low] API is well-documented in published help articles and Stoplight portal. PIT auth is straightforward. Connection-drop pattern is well-known and resolved via retry-with-backoff. No bot-detection or rate-limit issues observed in customer deployments.

## Top Workflows
1. **Contact ops** — search (with `searchAfter`), upsert by email/phone, bulk tag/untag, dedup, custom field read/write, add/remove from workflow.
2. **Pipeline/opportunity management** — search across stages, stage advancement, stale-opp reports, pipeline funnel summaries.
3. **Calendar/appointment booking** — list calendars, free slots, create appointments, manage blocked slots.
4. **Conversations** — search, send SMS/email, get messages, recordings + transcriptions.
5. **Custom fields & folders** — create/read/update, name→ID lookup so users never hardcode opaque IDs.

## Data Layer
- Primary entities: contacts, opportunities, pipelines+stages, custom_fields, tags, users, calendars, appointments, workflows, locations, conversations.
- Sync cursor: `searchAfter` cursor (GHL pagination spec). `dateUpdated` per-record drives incremental sync.
- FTS/search: SQLite FTS5 over contact name + email + phone (E.164) + custom field values; opportunities by name + monetaryValue + stage.
- Stage history is synthesized — GHL does NOT expose per-opp stage history endpoint, so the local cache is the only source for "when did this opp move from Stage X to Y."

## Codebase Intelligence
- Source: local MCP server at `~/Documents/ghl-mcp-server-kwcp/` (forked from `mastanley13/GoHighLevel-MCP`).
- Auth: `Authorization: Bearer pit-<uuid>` lowercase prefix mandatory. Capital `Pit-` returns 401 Invalid JWT (saved as user memory `ghl_pit_token_case.md`). Env var convention: `GHL_PIT_TOKEN` for token, `GHL_LOCATION_ID` for sub-account scope.
- Data model: location-scoped multi-tenant. Every endpoint requires `locationId` query param or implicit from token.
- Rate limiting: 100 req / 10s burst, 200k req / day. Mitigation: exponential backoff with 5 retries, 30s timeout (mirrored from user's existing `safe_get`/`safe_post` helpers in `kwcp_config.py`).
- Architecture: REST v2 only (v1 deprecated). Conversations endpoints use `Version: 2021-04-15`; everything else uses `Version: 2021-07-28`. Hardcode the right version per resource at the client layer — operators should never think about it.
- Endpoint surface: 67 distinct paths confirmed from local MCP source — covering associations, blogs, calendars, contacts, conversations, custom-fields, email, invoices, locations, media, objects, opportunities, payments, products, store, surveys, workflows.

## User Vision
- User is Director of Operations & Technology at KW Capital Properties (multi-office KW brokerage). Has 30+ existing Python automation scripts hitting GHL daily via direct REST + the local MCP. Wants a CLI that complements (not replaces) those scripts — terminal-fast contact lookups, bulk ops, dedup, stale-opp reports, custom field name→ID resolution, multi-location support (KWCP + THINK).
- Auth context: `PIT_TOKEN=pit-27acab48-...` hardcoded in `kwcp_config.py` (not exported as env var). CLI must support `GHL_PIT_TOKEN` env var. Two location IDs in play: KWCP (`F9YlSB15qA1pRCrPsTSw`) and THINK (`4LFw3kcvK7JYE0DZYkwm`).

## Product Thesis
- Name: `ghlcli` (binary), library slug `gohighlevel-pp-cli`.
- Tagline: *"The terminal for GoHighLevel. Bulk ops, dedup, and pipeline reports in seconds — local cache, agent-native JSON, multi-location support."*
- Why it should exist:
  - **No GHL CLI exists today.** The `GoHighLevel/ghl-cli` org repo is empty/internal. There are 10+ MCP servers (none cache locally), one official Node SDK, and zero terminal-native tools.
  - **Local SQLite cache** turns bulk ops (tag 5000 contacts) from minutes into seconds and survives the connection-drop bug.
  - **PIT-first auth** with auto-lowercase prefix correction — protects users from the silent 401 trap.
  - **Custom field name→ID resolution** — operators say "Agent Affiliation" not `ilvReXHcDuxetPOZ4wCK`.
  - **Multi-location config** — switch between KWCP + THINK with a flag.
  - **Agent-native --json everywhere** so it pipes into `jq`, Claude scripts, or stdin/stdout workflows.
  - **MCP-server mode** (auto-emitted) makes it competitive with the existing MCPs while being lighter.

## Build Priorities
1. **`ghlcli contact search`** — `--email`, `--name`, `--tag`, `--custom-field name=value`, `--updated-since`, `--json`. Auto-uses `searchAfter` cursor. The backbone command everything else builds on.
2. **`ghlcli contact bulk-tag` / `bulk-untag`** — CSV/stdin of emails or IDs, dedup, chunk + retry. The single highest-leverage daily op.
3. **`ghlcli opportunity report`** — stage distribution, stale opportunities (`--stale-days N`), pipeline funnel. JSON or TSV-for-Sheets-paste. Replaces user's `kwcp_growth_metrics.py` flow.
4. **`ghlcli sync`** — incremental pull of contacts/opportunities/custom-fields/pipelines/tags/users into local SQLite. Nightly cron-friendly.
5. **`ghlcli contact dedup`** — email + phone match, richness scoring, dry-run preview, optional merge. Reproduces `kwcp_dedup_weekly.py` as a first-class command.

## Known Limitations (must surface in CLI)
- Phone search returns 500 — block `?phone=` in `contact search`, route to email/name.
- Cannot create dropdown/multi-select custom fields via API — error early on `field create --type dropdown`.
- "Form Submitted" workflow trigger won't re-fire for already-enrolled contacts — document in `workflow enroll --help`.
- `/contacts/search` 100-page cap — implement `searchAfter` automatically; never expose page numbers.
- PIT prefix case-sensitivity — auto-lowercase on read, warn if uppercase input detected.
- No GET-by-id for email templates — use `previewUrl` round-trip from `/emails/builder?include=html`.
