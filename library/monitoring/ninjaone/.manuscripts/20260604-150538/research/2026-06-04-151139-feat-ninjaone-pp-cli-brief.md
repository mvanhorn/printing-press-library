# NinjaOne CLI Brief

## API Identity
- **Domain:** RMM / endpoint management for MSPs and internal IT. Device monitoring, patching, alerting, ticketing, scripting, org/client management, billing, KB/docs, custom fields, vulnerability management, backup.
- **Users:** MSP technicians and IT admins managing fleets of hundreds–thousands of devices across many client organizations.
- **Data profile:** Official OpenAPI 3.0.1 spec, 246 paths / 306 operations, 28 tags. Region-sharded (US `app.ninjarmm.com` is our default; EU/CA/OC exist). High-gravity entities: organizations, devices, alerts, OS/software patches, tickets, activities, automation scripts, custom fields, policies.

## Reachability Risk
- **None / Low.** Official, paid, actively-maintained Public API v2 with live docs, Postman workspace, regularly-updated OpenAPI spec. No 403/deprecation/bot-block reports.
- Reachability probe (Phase 1.9): `GET /v2/organizations` → `HTTP 400 {"error":"missing_header"}` — API alive, responds programmatically, demands auth header. PASS (auth required, no creds in probe).
- Probe-safe endpoint used: `GET /v2/organizations`
- **Operational risk:** aggressive **429 throttling** under bulk load (per-minute quota, undocumented numeric limit). Honor `Retry-After` + exponential backoff. No community tool ships this well → differentiator.

## Top Workflows
1. **Cross-org patch triage** — find every device with a failed/missing OS patch across ALL organizations, then scan + apply in bulk. The #1 MSP use case.
2. **Today's critical-alert triage** — list active alerts, group by org/severity, reset resolved ones in bulk, trend analysis.
3. **Bulk device action on a cohort** — reboot offline/stale devices, set/clear maintenance mode before patch windows, run a remediation script across a device filter.
4. **Fleet health / inventory & compliance reporting** — device health, AV status/threats, backup usage, disk/volume capacity, stale-device + patch-compliance reports across the fleet.
5. **Client onboarding / structural ops** — create org + locations, set policies, custom fields, device approval, role/tag management.

## Table Stakes (everyone has these)
- Device list/get/search, reboot, maintenance mode, run script.
- Org list/get/create, locations, contacts.
- Alert list/reset (single + bulk).
- OS/software patch query + scan + apply.
- Ticket CRUD + comments.
- Custom fields query/update (device/org/location).
- System/HW queries (AV, health, OS, disks, volumes, NICs, processors, RAID, services, software).
- Activities feed.
- Cursor pagination + device-filter (`df`) syntax.

## Data Layer
- **Primary entities to persist:** organizations, locations, devices, alerts, os_patches, software_patches, tickets, activities, automation_scripts, policies, custom_field_definitions, software_products, vulnerabilities, users/contacts.
- **Sync cursor:** cursor-based (`pageSize` + `after`; `nextCursor`); activities by `lastActivityId`; devices accept `df` filter.
- **FTS/search:** offline full-text over devices (name/hostname/OS/serial), alerts (message/type), organizations, tickets — so a tech can grep the whole fleet without re-hitting the throttled API.

## Codebase Intelligence (ecosystem)
- **homotechsual/NinjaOne** (PowerShell, ~131★) — de-facto MSP standard. 4 auth flows; `-instance us/eu/oc`; publishes canonical OpenAPI gist. The adoption bar.
- **Lungshot/NinjaOneMCP** (TS, ~34★) — strongest curated MCP: patching + ticketing + scripts + dry-run (`confirm`), STDIO/HTTP/SSE, dual OAuth.
- **jasondsmith72/NinjaRMM-API-Gateway-MCP** (Python) — gateway design: 306 endpoints behind 7 tools (`find_endpoint`/`describe_endpoint`/`call`/`raw_call`/memory). Max coverage, min tools.
- **wyre-technology/ninjaone-mcp** (TS, ~17★) — hierarchical lazy tool loading; regions.
- **ninjapy** (PyPI) — most complete Python client; env vars `NINJA_CLIENT_ID/SECRET/TOKEN_URL/SCOPE/BASE_URL`, scope `"monitoring management control"`, dual pagination.
- **Auth:** OAuth2 `client_credentials` → POST `client_id`/`client_secret`/`grant_type`/`scope` to `https://app.ninjarmm.com/ws/oauth/token`. Bearer token ~3600s; refresh ~60s before expiry. Scopes: `monitoring` (read), `management` (write/scripts), `control` (remote). Canonical env: `NINJAONE_CLIENT_ID` / `NINJAONE_CLIENT_SECRET` (MCP convention) with `NINJA_*` aliases.
- **Rate limiting:** 429 + `Retry-After`; exponential backoff; cache aggressively.
- **Pagination:** cursor (`pageSize`+`after`, `nextCursor`); device filter `df` first-class.

## User Vision (USER_BRIEFING_CONTEXT)
- Region: **US** (`app.ninjarmm.com`).
- Scope: **Everything** — full parity across the NinjaOne surface, not a curated subset.
- Credentials available for live smoke testing (OAuth2 client-credentials).
- Same shape as the user's prior halopsa-mcp and hudu-pp-mcp local MCP servers.

## Product Thesis
- **Name:** NinjaOne CLI (`ninjaone-pp-cli`)
- **Why it should exist:** No existing tool combines (1) full 300+ endpoint coverage, (2) named-command ergonomics, (3) robust rate-limit/pagination handling, and (4) cross-org bulk workflows with a local store. PowerShell module = ergonomics but no agent-native/offline; gateway MCP = coverage but no ergonomics; curated MCPs = ergonomics but partial coverage. This CLI is all four: every endpoint reachable, the high-value ones promoted to named commands + a thin MCP (`ninjaone_search` + `ninjaone_execute`), a local SQLite store for offline fleet-wide search and cross-org rollups, and an adaptive `Retry-After` limiter so bulk ops don't melt under 429s.

## Build Priorities
1. **Foundation:** OAuth2 client-credentials auth + token cache, adaptive rate limiter, cursor pagination auto-follow, `df` device-filter flag, local SQLite store for primary entities, sync/search/SQL.
2. **Absorb:** every device/org/alert/patch/ticket/script/custom-field/system-query command the competitors expose, matched and beaten with `--json`/`--select`/`--dry-run`/typed exits/offline.
3. **Transcend:** cross-org fleet-wide commands only possible with the local store + adaptive limiter (cross-org patch-gap report, fleet drift, alert-storm triage, stale-device sweep, bulk cohort actions from a saved filter).
