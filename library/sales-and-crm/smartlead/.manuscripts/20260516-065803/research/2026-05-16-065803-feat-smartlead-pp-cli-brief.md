# SmartLead CLI Brief

## API Identity
- Domain: Cold email outreach automation. Campaigns, multi-step sequences, sender
  email accounts with inbox warmup, lead management, deliverability analytics.
- Users: Agencies and growth/sales teams running cold outbound at scale. Power
  users script bulk lead uploads, campaign state changes, and reply triage.
- Data profile: Campaigns -> sequences + email-accounts + leads. Leads carry
  categories and per-campaign send statistics. Email accounts carry warmup state.
- Base URL: `https://server.smartlead.ai/api/v1`
- Auth: API key as query parameter `api_key`. Env var `SMARTLEAD_API_KEY`. No
  header/bearer auth. (Confirmed in the official Python SDK source.)

## Reachability Risk
- None. SmartLead ships a fully documented, stable official REST API. No GitHub
  issues report 403/blocked/deprecated against the API itself. The only failure
  mode is HTTP 429 rate limiting (~60 req/60s per key; community CLIs throttle
  to ~10 req/2s with backoff). Generated client should self-throttle and read
  rate-limit response headers.

## Top Workflows
1. Triage a campaign: list campaigns -> pull statistics/analytics -> see which
   leads opened/replied/bounced -> read the message thread for a hot lead.
2. Bulk lead operations: add a lead list to a campaign, recategorize leads,
   unsubscribe a lead globally, block sender domains.
3. Sender hygiene: list email accounts, check warmup stats, reconnect failed
   accounts before they tank a campaign.
4. Campaign lifecycle: create a campaign, attach email accounts, save the
   sequence, set the schedule, start/pause/stop it.
5. Cross-campaign lead lookup: find a lead by email and see every campaign it
   touches (dedupe / do-not-double-contact checks).

## Table Stakes
- Full CRUD on campaigns, leads, email accounts, webhooks.
- Campaign start/pause/stop, schedule and settings updates.
- Sequence get/save, A/B variant support.
- Warmup configuration and warmup stats per account.
- Campaign statistics with email-status filters; date-range analytics.
- Webhook upsert/delete with typed event + category enums.
- Whitelabel client management.
- JSON-first output for agent/script consumption (bcharleson CLI is the bar).

## Data Layer
- Primary entities: campaigns, leads, email_accounts, sequences, webhooks,
  lead_categories, clients, campaign_statistics.
- Sync cursor: campaigns list -> per-campaign leads (offset/limit pagination)
  and statistics. No global "updated since" cursor; sync is full-snapshot.
- FTS/search: leads (email, name, company), campaigns (name), message history
  bodies. Offline search across leads is the highest-value index.

## Codebase Intelligence
- Source: official Python SDK `smartlead-ai/API-Python-Library` (40 methods,
  every one maps to a real documented endpoint -- authoritative path list).
- Auth: `?api_key=` query suffix on every request; env `SMARTLEAD_API_KEY`.
- Data model: campaign-centric. Leads, email-accounts, webhooks, statistics,
  and analytics are all sub-resources reached via `/campaigns/{id}/...`.
- Rate limiting: 429 on exceed; no documented hard number. Throttle defensively.
- Enums worth typing: campaign status (START/PAUSED/STOPPED), webhook event
  types, lead categories, track/stop-lead settings.

## User Vision
- Matt runs SmartLead inside the Ozark link-building machine (campaigns, leads,
  sequences, sender-config). API key pulled from
  `/mnt/c/Coding/ozark-linkbuilding/.env` (`SMARTLEAD_API_KEY`) for read-only
  Phase 5 smoke tests. A clean CLI plugs directly into that LB workflow --
  campaign health checks, reply triage, lead dedupe across campaigns.

## Product Thesis
- Name: smartlead-pp-cli
- Why it should exist: Every existing SmartLead tool is either an MCP server
  (no offline state, no local search, agent-only) or a thin TypeScript CLI with
  no local database. None can answer "which leads went silent", "which sender
  accounts are dragging down deliverability", or "is this lead already in
  another campaign" without re-hitting the API per question. A CLI with a
  local SQLite mirror + FTS turns those into instant offline queries and makes
  cold-outbound auditable.

## Build Priorities
1. Priority 0: SQLite data layer for campaigns, leads, email_accounts,
   sequences, webhooks, statistics + sync + FTS search + sql passthrough.
2. Priority 1: All 41 absorbed endpoints as typed commands with --json,
   --dry-run, typed exit codes, adaptive 429 throttling.
3. Priority 2: Transcendence commands -- offline analytics the API cannot
   answer in one call (silent leads, sender health ranking, cross-campaign
   lead dedupe, reply-rate drift, warmup-readiness gate).
