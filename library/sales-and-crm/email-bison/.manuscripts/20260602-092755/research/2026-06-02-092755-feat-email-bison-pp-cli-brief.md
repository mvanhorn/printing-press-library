# Email Bison CLI Brief

## API Identity
- Domain: Cold-email outreach / private email sequencer (self-hosted "dedicated instance" SaaS). Competes with Instantly, Smartlead, Lemlist. Aimed at agencies and the "top 10%" sending high volume from private infrastructure.
- Users: Cold-email agencies, lead-gen operators, growth teams running multi-workspace outreach (one workspace per client).
- Data profile: Leads/contacts (unlimited storage, ESP-tagged), campaigns (settings + schedule + multi-step sequences with A/B variants), sender emails (Google/Microsoft/SMTP), replies (master inbox with interest categorization), tags, custom variables, webhooks, workspaces.

## Reachability Risk
- Low. REST API, Bearer token auth, documented. Default base host `https://dedi.emailbison.com`. The catch: it is **self-hosted with a per-tenant dedicated domain**, so each customer has their own base URL. An unauthenticated `GET /api/users` returns 401 (expected; user declined key gate). No bot-protection, no reverse-engineering needed. Rate limit ~10 req/s, HTTP 429 carries `retry_after`.

## Top Workflows
1. **Build and launch a campaign end-to-end**: create campaign -> update settings (sending caps, tracking, unsubscribe) -> create schedule (days/window/timezone) -> create sequence steps (subject/body with `{VARIABLE}` merge tags, wait days, A/B variants, thread replies) -> attach sender emails -> attach leads -> `resume` to launch.
2. **Lead ingestion + enrichment**: create single lead or bulk CSV import, attach custom variables, tag leads, push into campaigns by ID or lead-list.
3. **Master-inbox triage**: list replies (filter by status/folder/campaign/sender/tags/read), mark interested, reply in-thread (cc/bcc/inject previous body), push interested replies into a follow-up campaign.
4. **Sender-email / deliverability ops**: list sender accounts, bulk-connect SMTP/IMAP, tag accounts, patch account settings.
5. **Multi-workspace agency management**: list workspaces, mint per-workspace `api-user` tokens (super-admin), per-workspace tag/variable management.

## Table Stakes (must match the existing MCP + UI)
- Campaigns: list, get, create, update settings, pause, resume, schedule (create/get/templates/from-template), sequence steps (list/create/send-test/delete).
- Leads: list (paginated), get replies/sent-emails for a lead, create, update, bulk CSV, custom variables.
- Replies: list, mark interested, reply, push to follow-up campaign, attach scheduled email.
- Sender emails: list, patch, bulk import.
- Tags: list, create, delete, attach/remove on leads/campaigns/sender-emails.
- Custom variables: list, create.
- Webhooks: create, send test event.
- Workspaces: list, create API token.

## Data Layer
- Primary entities: leads, campaigns, sequence_steps, sender_emails, replies, tags, custom_variables, workspaces, scheduled_emails.
- Sync cursor: cursor pagination (`pagination_type=cursor`, `next_cursor`) on all index routes — ideal for full local sync (page pagination is capped at 1000 pages).
- FTS/search: leads (name/email/company/title/custom vars), replies (message text/status/folder), campaigns (name/settings).

## Auth
- Bearer token in `Authorization: Bearer <token>`. Env var: `EMAIL_BISON_API_KEY`.
- Two token kinds: `api-user` (workspace-scoped, recommended) and `super-admin` (impersonates creator; required only for workspace-token minting). Every request is scoped to exactly one workspace.
- **Base URL is per-tenant** (self-hosted dedicated domain). Default `https://dedi.emailbison.com`; the CLI MUST allow override via `EMAIL_BISON_BASE_URL`. This is the single most important non-default requirement — agencies on their own instance cannot use a hardcoded host.

## Ecosystem (for absorb)
- `Sirkunle001/email-bison-claude-mcp` (Python MCP, single-binary): list_campaigns, analyze_campaign, campaign_performance_summary, analyze_replies, dump_replies_json, lead_engagement_analysis, add_leads_to_campaign, stop_future_emails, sequence_optimization_insights, campaign_events_stats, list_email_accounts, list_warmup_accounts, warmup_account_details, warmup_enable/disable/update_limits, raw_request. Uses `EMAILBISON_API_KEY` + `EMAILBISON_BASE_URL`.
- Cargo, Clay, n8n, Zapier, Make integrations (lead create/update/upsert + webhook listeners + blocklist enrichments).
- No existing Go CLI. No official CLI. This is greenfield for a CLI.

## Reachability / Quirks for Build
- `remove-sender-emails`: docs prose says DELETE but the concrete curl example uses POST with a body. Use POST (matches the working example); expose as a `campaigns remove-sender-emails` command.
- Tag remove uses the SAME `/api/tags/attach-to-*` paths with HTTP DELETE + body.
- Bulk endpoints are multipart/form-data (no Content-Type header set manually).
- Warmup endpoints exist (MCP exposes them) but exact paths are undocumented; do NOT invent typed endpoints for them — leave to the generic `raw` escape hatch if needed.
- Merge-tag syntax in sequences: `{VARIABLE}` uppercase, must pre-exist as a custom variable in the workspace.

## Product Thesis
- Name: **Email Bison CLI** (`email-bison-pp-cli`).
- Why it should exist: The only programmatic surface today is a Python MCP and the raw REST API. There is no agent-native CLI with a local store. Agencies running many workspaces need scriptable, offline-queryable campaign/lead/reply state: "which campaigns are below their daily cap", "which sender accounts are tagged disconnected", "show every interested reply across campaigns since yesterday" — none of which a single REST call answers. A SQLite-backed CLI with cursor sync, FTS, and `--json/--select` turns Email Bison into a composable building block for outreach automation.

## Build Priorities
1. Data layer + cursor sync + FTS for leads, campaigns, sequence_steps, sender_emails, replies, tags (Priority 0).
2. Full absorbed endpoint surface across campaigns, leads, replies, sender-emails, tags, custom-variables, webhooks, workspaces, scheduled-emails (Priority 1).
3. `EMAIL_BISON_BASE_URL` override (non-negotiable for self-hosted).
4. Transcendence: cross-entity analytics only possible with a local join (interested-reply roll-up, sender-health board, campaign-cap headroom, sequence-variant performance, stale-lead detection).
