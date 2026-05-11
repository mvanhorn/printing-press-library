# Chainels CLI Brief

## API Identity
- **Domain:** B2B tenant-experience platform for commercial + residential real estate. Property managers run a community per building/portfolio; tenants/residents communicate, report issues, book resources, file periodic-reporting (turnover) data, receive alarms.
- **Users:** property managers / community managers (primary), retail tenants reporting turnover, residents reporting issues, integrators (Yardi, Entrata) syncing leases/units.
- **Data profile:** OpenAPI 3.1, 164 paths, **236 operations**, 35 tags. Real, current spec at `https://www.chainels.com/openapi.bundle.json` (693KB, etag 2026-05-04). Servers: prod `https://www.chainels.com/api/v2`, demo `https://demo.chainels.com/api/v2`.

## Reachability Risk
- **None.** Spec fetched HTTP 200, no anti-bot. Live OAuth token endpoint `https://www.chainels.com/oauth/access_token` is plain HTTPS. User is bas@chainels.com (insider); will provide client_id/client_secret for Phase 5.

## Top Workflows
1. **Community broadcast** — push timeline post, message, or alarm to a community's members (residents/tenants) and track read receipts.
2. **Issue triage** — list open issues for a community/category, assign, transition through workflow actions, resolve.
3. **Turnover & periodic reporting** — collect, query, and aggregate retail tenants' periodic turnover/reporting submissions (multi-month).
4. **Booking flow** — list bookables, query bookings, create/edit on tenants' behalf.
5. **Member lifecycle** — invite/onboard/offboard accounts to a community, sync entity (company/household) roles, audit who's where.

## Table Stakes
- List + get + create + update + delete on companies, communities, messages, issues, bookings, alarms, accounts, agreements, invoices, request-forms.
- OAuth login (client_credentials for app, authorization_code for user, group_token for "act as a group").
- Filter + paginate every list endpoint (`?since`, `?limit`, pages or offsets — verify against spec at generate-time).
- Read auth scopes (`basic`, `chat`, `read.account.email`, `read.account.notifications`, `read.turnover`, `read.booking`, ~20 total).

## Data Layer
- **Primary entities:** companies (46 ops, huge), communities, accounts (people), messages, issues, bookings, alarms, agreements, invoices, request-form submissions, turnover reports, periodic reports.
- **Sync cursor:** most resources expose `updated_at` / `created_at` semantics via PATCH-able endpoints — sync by listing per community + entity, store last-sync timestamp per resource.
- **FTS/search:** message bodies, issue titles/descriptions, agreement names, request-form payloads. Property managers cannot grep across communities in the web UI today — offline FTS5 over a synced corpus is a real differentiator.

## Codebase Intelligence
- No DeepWiki target (the spec is the source of truth; the only public ecosystem repo is a stale PHP OAuth provider).
- **Auth (from spec):** OAuth2 with three flows.
  - `oauth_client`: client_credentials → userless app token. Scopes: `basic`, `create.account`.
  - `oauth_code`: authorization_code → user (or group) token. ~20 scopes incl. `chat`, `write.messages`, `read.turnover`, `read.invoices`, `send.alarm`.
  - `openid`: OpenID Connect at `/.well-known/openid-configuration`.
  - Token endpoint: `https://www.chainels.com/oauth/access_token`.
  - PHP provider (`Chainels/oauth2-provider-php`, 0 stars, last touched 2017) mentions a third grant "Group Token" — Chainels-specific.
- **Data model (path-root distribution):** `/companies` (46), `/messages` (15), `/reporting` (14), `/issues` (11), `/booking` (9), `/communities` (9), `/requests` (9), `/alarms` (6), `/accounts` (5), `/turnover` (5), `/discounts` (5), `/metrics` (5), `/agreements` (5), `/service-accounts` (5).
- **Rate limiting:** not declared in spec; assume tenant-grade. Surface 429 cleanly.

## User Vision
- Not provided ("Let's go").

## Source Priority
- Single source (Chainels official OpenAPI). Multi-source priority gate skipped.

## Product Thesis
- **Name:** `chainels-pp-cli`
- **Why it should exist:** Today no machine-readable wrapper exists for Chainels. Property managers, integrators (Yardi/Entrata pipelines), and Chainels' own team have no CLI for bulk operations, no cross-community search, no offline reporting query. Building it as a Printing Press CLI gives:
  1. Every endpoint as a typed Cobra command (236 ops) + agent-native `--json --select`.
  2. Local SQLite store with FTS5 over messages/issues/agreements — the cross-community search the web UI lacks.
  3. Transcendence commands powered by joins the API doesn't expose (assignee load, stale issues, turnover variance, broadcast coverage).
  4. MCP server (stdio + http) so agents can pull tenant context, file issues, or pre-compose alarms.
- **Differentiator vs zero existing tools:** parity for free; novel local-store joins are the wedge.

## Build Priorities
1. **Phase 2 — Generate.** Pre-enrich spec auth (OAuth client_credentials scope → CHAINELS_CLIENT_ID + CHAINELS_CLIENT_SECRET; tokens cached in config). Auto-pick `client_credentials` as the default flow (no browser handshake needed for a CLI).
2. **Phase 3 priorities:**
   - **P0 data layer:** companies, communities, accounts, messages, issues, bookings, alarms, agreements, invoices, turnover reports, periodic reports → SQLite + FTS5.
   - **P1 absorb:** every endpoint as a typed command (auto from spec; 236 ops). Re-author terse param descriptions where spec is thin.
   - **P2 transcendence:** cross-community FTS, issue-assignee load, broadcast read-receipt rollup, turnover variance vs trailing N months, stale-issue digest, agreement renewal calendar, alarm-recipient diff, booking utilization, "what changed since" since flag, member-load audit.
3. **MCP:** 236 endpoints + ~10 novel features → 246+ tools. Enrich `mcp:` block with `transport:[stdio,http]`, `orchestration: code`, `endpoint_tools: hidden` (Cloudflare pattern).
