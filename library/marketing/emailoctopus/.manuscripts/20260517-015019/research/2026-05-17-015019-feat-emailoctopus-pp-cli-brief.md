# EmailOctopus CLI Brief

## API Identity
- **Domain:** Email marketing platform (Mailchimp/ConvertKit/MailerLite peer). Hosted SaaS, REST API v2 at `https://api.emailoctopus.com`.
- **Surface:** 25 endpoints across Lists (5), Contacts (7), Campaigns + reports (5, read-only), Tags (4), custom Fields (3), Automations (1, trigger only).
- **Users:** Indie newsletter operators, small-SaaS marketers, hobbyist newsletter writers — the API is **included on the free tier** (2,500 subscribers), which broadens the addressable base significantly compared to peers that gate API behind paid plans.
- **Auth:** HTTP Bearer (`Authorization: Bearer <api_key>`). Keys at `https://api.emailoctopus.com/developer/api-keys/create`. v1 keys (pre-Oct 2024) do not work with v2 — meaningful migration cohort.
- **Data profile:** Cursor-paginated (100/page, opaque `starting_after` cursor). Rate limit 10 req/sec sustained, 100-burst, `X-RateLimit-Retry-After` header on 429. Hosted dashboard handles bulk CSV import outside the API.

## Reachability Risk
**None.** Hosted commercial SaaS, documented current API, rate limits clearly described. No 403/blocked complaints in SDK issue spot-checks.

## Top Workflows
1. **Sync contacts from app/CRM into a list** (upsert via `PUT /lists/{id}/contacts` or `/batch`) — dominant use case across every wrapper README.
2. **Bulk-tag and segment contacts** based on app-side events (purchases, plan tier, engagement).
3. **Trigger automations programmatically** for transactional flows (onboarding sequences, post-purchase drips) — only API write path that triggers a send.
4. **Pull campaign reports for BI / dashboards** — summary, per-contact filtered (opened/clicked/bounced), per-link click counts.
5. **Export contacts for analytics or backup** — no native export endpoint; users paginate the whole list.
6. **Cross-list dedupe / migrate / merge** — manual today; surfaced repeatedly in help center.
7. **Webhook-fed local sync** for fast queries the API can't answer directly (engagement filtering, cross-list lookup).

## Table Stakes
- Full CRUD on Lists, Contacts, Tags, Fields.
- Read access to Campaigns + all three report endpoints.
- Tag operations (add/remove/list).
- Automation trigger.
- Cursor pagination handled (`--limit`, auto-page on `--all`).
- Auth env var `EMAILOCTOPUS_API_KEY`.
- Rate-limit aware (back off on `X-RateLimit-Retry-After`).
- `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes.

## Data Layer
- **Primary entities:** lists, contacts (list-scoped FK → list), tags (list-scoped), fields (list-scoped), campaigns, campaign_reports (per-campaign summary), campaign_link_reports.
- **Sync cursor:** opaque per-resource `starting_after`. Snapshot wholesale per list when syncing contacts (no `updated_after` filter exists in the API, so full pagination on each sync is the only option).
- **FTS/search:** contacts by email + tags + custom field values; campaigns by name/subject; tags + fields by name.
- **High-gravity entities:** contacts are the volume driver (free tier 2,500 / Pro scales to hundreds of thousands). Local store is what makes cross-list dedupe, bulk-delete loops, offline filtering, and report joins viable without burning the 10 req/sec rate-limit budget.

## Codebase Intelligence
- **OpenAPI 3.1 spec** provided directly by user (`~/Downloads/openapi.json`, 386 KB, 25 endpoints). Title `EmailOctopus v2 API`, version `2.0.0`. Security: single `api_key` http+bearer scheme. Auth detection should produce `bearer_token` with env var `EMAILOCTOPUS_API_KEY`.
- **Pagination shape:** cursor-only (`limit`, `starting_after`). Generator's pagination handling should detect this automatically.
- **Mutation shapes:** PUT for upsert + update; POST for create; DELETE plain; PATCH absent. Batch endpoint at `PUT /lists/{list_id}/contacts/batch`.
- **Response envelopes:** standard `{data, paging}` shapes typical of cursor-paginated APIs.

## Product Thesis
- **Name:** `emailoctopus-pp-cli` (binary), slug `emailoctopus`.
- **Why it should exist:** No actively-maintained v2-native CLI or SDK exists in any language. Every general-purpose wrapper on GitHub/npm/PyPI targets the deprecated v1 API (which stopped working for keys minted after Oct 2024). The closest "MCP-like" tool is Zapier's hosted server with 6 contact-only tools that each burn 2 Zapier tasks. This CLI is genuinely first-mover and unblocks:
  - Indie operators who want shell-scriptable list management.
  - SaaS teams who want to wire EmailOctopus into deploy pipelines / cron jobs without writing a wrapper.
  - Agents (Claude Desktop, Codex, etc.) that want the full v2 surface, not 6 hand-picked tools.
- **Differentiation:** local SQLite store unlocks the workflows the API itself cannot serve — cross-list dedupe, list-diff over time, offline report analysis, rate-limit-budgeted bulk delete, and SQL-shaped questions over contact + tag + field data.

## Build Priorities
1. **Priority 0 (foundation):** Data layer for lists, contacts, tags, fields, campaigns, campaign reports. Sync command with cursor-pagination across all entities. FTS over email + tags + custom field values + campaign names.
2. **Priority 1 (absorb):** Every endpoint as a typed command. `--json`, `--select`, `--csv`, `--dry-run`, `--limit`, `--all` (auto-paginate). Rate-limit-aware HTTP client. Auth + doctor + login flow.
3. **Priority 2 (transcend):** Cross-list dedupe, bulk-delete with budget, list-diff over time, offline campaign report joins, contact-engagement aggregation, tag taxonomy reports, field-value queries, email-domain breakdown, churn detection.

## Sources
- `~/Downloads/openapi.json` (OpenAPI 3.1 spec, user-provided)
- https://emailoctopus.com/api-documentation/v2
- https://help.emailoctopus.com/article/91-api-limits
- https://help.emailoctopus.com/article/93-sdks-and-libraries
- https://emailoctopus.com/pricing
- https://zapier.com/mcp/emailoctopus (closest existing agent surface — 6 tools, contact-only)
- https://github.com/tubbo/email_octopus (v1 Ruby SDK, ~8 stars, alpha)
- https://github.com/goran-popovic/email-octopus-php (v1 PHP SDK, ~5 stars)
- https://github.com/kartoffelkraft/email-octopus-ts (TS SDK, ~14 stars, scope unclear v1 vs v2)
- https://github.com/wthomsen/email-octopus (Node, v1)
- https://www.emailtooltester.com/en/reviews/emailoctopus/ (2026 review)
