# AdminByRequest CLI Brief

## API Identity
- **Domain:** Endpoint Privilege Management (EPM). AdminByRequest (AbR) is a SaaS that lets standard-user employees request just-in-time local-admin elevation, with audit logging, scanning, and policy controls.
- **Users:** IT/security admins. The public API exists primarily to (a) feed SIEM/observability tooling (Splunk, Sentinel, Power BI), (b) automate approve/deny in ticketing systems (ServiceNow, Jira, generic), (c) generate offline elevation PIN codes for disconnected endpoints.
- **Data profile:** Audit log entries, security events, hardware/software inventory per endpoint, elevation requests (pending/approved/denied), generated PIN codes. Tenant data is segregated across 6 data centers (dc1api–dc6api.adminbyrequest.com), pinned at provisioning. This tenant lives on **dc3api**.

## Reachability Risk
- **None.** Live probes against `dc3api.adminbyrequest.com` returned HTTP 200 with real data for `/auditlog`, `/events`, `/inventory`, `/inventory?wantsoftware=1`, `/inventory?wanthardware=1`, `/requests?status={pending,approved,denied}`, `/auditlog/delta`. Other DCs return 500 for this tenant (correct multi-tenant isolation).

## Top Workflows
1. **SIEM/observability ingest** — pull audit log + events on an interval into Splunk/Sentinel/Power BI. Requires reliable pagination and delta sync.
2. **Approve/deny elevation requests from ticketing** — list pending requests, take action via PUT, record decision back into ticket.
3. **Compliance reporting** — query auditlog by user/time window to demonstrate who got admin and why.
4. **Offline elevation** — generate PIN codes for endpoints that lost connectivity; admin reads PIN to user over phone.
5. **Inventory discovery** — find what hardware/software is installed where; spot endpoints missing AbR agent or running old client versions.

## Table Stakes (from competing tools / SIEM connectors)
- Pagination via `startid` + `take` cursors.
- Delta sync via `/auditlog/delta` (timeNow/deltaTime ticks).
- Filter by status, type, code, user, computer.
- JSON output for piping into pipelines.
- API-key + Basic auth options.
- Daily quota enforcement (100k calls; auto-block until next business day if exceeded).

## Data Layer
- **Primary entities:** `audit_log_entries`, `events`, `inventory_devices`, `requests`, `pin_codes` (transient).
- **Sync cursor:** `id` per stream (monotonic). Delta endpoint exists for auditlog only.
- **FTS/search:** computer name, user account, application name, vendor — these are the high-gravity searchable fields.

## Codebase Intelligence
- **Source:** direct API probes against live tenant + AbR public docs (https://docs.adminbyrequest.com and https://www.adminbyrequest.com/en/docs/{auditlog,events,inventory,pin-code,requests}-api).
- **Auth:** API key. Two modes — header `apikey: <key>` OR HTTP Basic (any username, key as password). Env var convention: `ADMINBYREQUEST_API_KEY`.
- **Data model:** Each tenant has one or many endpoints (devices). Each device has inventory + audit/event history. Requests are a separate lifecycle (pending → approved/denied).
- **Rate limiting:** 100,000 calls/day hard quota; over-quota = tenant blocked till next business day. Tooling should track call count locally and warn before exhaustion.
- **Architecture:** REST + JSON; ASP.NET backend (IIS server header); ID-based cursor pagination, no Link headers.

## Product Thesis
- **Name:** `adminbyrequest-pp-cli` (binary), library slug `adminbyrequest`.
- **Why it should exist:** AbR ships a portal UI and several pre-built SIEM/ticketing integrations (Splunk, Sentinel, ServiceNow, Jira, Teams). What's missing is a *terminal-native, scriptable* surface that (a) syncs everything into local SQLite so analysts can ad-hoc SQL/FTS against months of data without re-hitting the API, (b) lets agents/scripts approve/deny in one command, (c) generates offline PINs in one line, and (d) gives unified context across audit + events + inventory + requests through a single offline data layer. No existing tool combines the local store + offline search + scripting surface; the pre-built integrations are vertical (one product → one SIEM), not a general-purpose analyst tool.

## Build Priorities
1. **Data layer + sync** — schema for auditlog/events/inventory/requests with FTS5 on computer/user/application/vendor; sync command with delta support for auditlog.
2. **Per-resource list/get + approve/deny + pin-generate** — match every documented endpoint.
3. **Transcendence** — quota usage forecasting, cross-resource correlation (who was elevated when this event fired?), offline-friendly inventory drift detection, agent-version compliance, denied-reason word-cloud, repeat-requestor detection.
