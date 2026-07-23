# Zoho Campaigns CLI Brief

## API Identity
- Domain: Email marketing — Zoho Campaigns (campaigns.zoho.com), the email-marketing product, NOT the CRM "Campaigns" module.
- Users: Marketing operators and AI agents pulling campaign performance, list health, and send history; occasionally subscribing/unsubscribing contacts from automations.
- Data profile: Campaigns (~44 in the Kontur org), mailing lists, subscribers, per-recipient engagement actions, tags. Small-to-medium volume, report-heavy reads.

## Reachability Risk
- Low. Live-verified 2026-07-23 against the Kontur org: `GET /api/v1.1/recentcampaigns` and `GET /api/v1.1/getmailinglists` returned 2xx with real data using `Authorization: Zoho-oauthtoken <token>`.
- Known hazards (from docs + community, not blockers):
  - Rate limit: 500 calls per 5 minutes; exceeding it locks the API for 30 minutes. Batch report pulls must be throttled.
  - Errors return HTTP 200 with `{"status":"error","code":...}` — HTTP-status checks alone silently pass failures.
  - Path scheme is inconsistent: reads at root (`/recentcampaigns`), some writes under `/json/` (`/json/listsubscribe`), tags under `/tag/...`.
  - The prior (May 2026) CLI failed because BOTH earlier specs used nonexistent `/json/get*` read paths. This run authors a corrected spec, live-verified per endpoint.

## Top Workflows
1. Campaign performance pull: recentcampaigns → campaignreports per key (opens, clicks, bounces, unsubs, geo) → feed the Kontur Marketing Dashboard.
2. Mailing-list health: getmailinglists → per-list contact/bounce/unsub counts, growth over time.
3. Send-history summary: recentsentcampaigns / sent-status filters for "what went out this month."
4. Recipient drill-down: getcampaignrecipientsdata (opened/clicked/bounced/optout contacts per campaign).
5. Contact ops from automations: listsubscribe / listunsubscribe / contactdonotmail (writes; secondary).

## Table Stakes (best incumbents: MCPBundles hosted MCP ~20 tools; keepsuit Laravel SDK for contacts/tags)
- List/create/send/schedule/clone/delete campaigns; campaign details, reports, recipients data
- Mailing list CRUD; subscribers with status filter + pagination; subscriber counts
- Subscribe/unsubscribe/do-not-mail; bulk add contacts
- Tag CRUD + associate/deassociate
- Segments (details + contacts); contact field schema

## Data Layer
- Primary entities: campaigns (campaignkey), mailing lists (listkey), subscribers, campaign reports (point-in-time snapshots), tags.
- Sync cursor: fromindex/range pagination; campaigns by created/updated time.
- FTS/search: campaign name/subject, list name, subscriber email.
- Report snapshots over time are the transcendence enabler: Zoho only shows current-state reports; a local history enables trend/delta analysis nothing else offers.

## Codebase Intelligence
- Source: keepsuit/laravel-zoho-campaigns (read directly) + dltHub recipe + official docs.
- Auth: OAuth2 self-client; `Authorization: Zoho-oauthtoken <access_token>`; refresh at `https://accounts.zoho.com/oauth/v2/token`; scopes `ZohoCampaigns.campaign.*` (campaigns/reports) and `ZohoCampaigns.contact.*` (lists/subscribers/tags). Region-variant hosts (.com/.eu/.in) — token host must match API host. Kontur org verified on .com.
- Data model: campaignkey is the join key from recentcampaigns into details/reports/recipients; listkey from getmailinglists into all list ops.
- Rate limiting: 500/5min with 30-min lockout; no usage-inspection endpoint.
- Error envelope: HTTP 200 + `status:"error"` + numeric `code` (e.g., 6101 no campaigns, 6301 invalid key, 6601 insufficient privilege).

## User Vision
- Kent (Global Marketing Director, Kontur) needs durable no-human-in-the-loop auth: claude.ai Zoho connector artifacts keep hitting recertification; this CLI holds self-client OAuth creds that auto-refresh. Credentials already captured and verified this run (full ALL scopes).
- Primary consumers: Claude Code agents feeding the Kontur Marketing Dashboard note, daily brief, scheduled/headless runs, and data baked into artifacts at generation time. Agent-first.

## Product Thesis
- Name: zoho-campaigns-pp-cli
- Why it should exist: There is NO open-source CLI, MCP server, npm, or PyPI package for Zoho Campaigns — every existing MCP is closed hosted middleware with interactive auth (the exact recert pain), and the two PHP SDKs cover only fragments. This CLI is first-mover: offline SQLite mirror, report-history snapshots, agent-native output, and self-refreshing headless auth.

## Build Priorities
1. Corrected, live-verified spec (read endpoints first: recentcampaigns, campaignreports, getmailinglists, getlistsubscribers, recipientsdata).
2. Local store + sync for campaigns/lists/subscribers + report snapshots (the trend/delta foundation).
3. Report/dashboard-feeding commands with `--json`/`--select`/`--compact`.
4. Contact write ops (subscribe/unsubscribe/do-not-mail/bulk add) with `--dry-run`.
5. Tags + segments parity.
