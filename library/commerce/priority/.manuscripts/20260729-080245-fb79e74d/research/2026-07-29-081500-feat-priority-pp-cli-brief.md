# Priority Software CLI Brief

## API Identity
- Domain: ERP (Israeli-origin, global). Priority's REST API exposes **every user screen ("form") as an OData v4 entity** — ORDERS, CUSTOMERS, AINVOICES, LOGPART, PORDERS, SUPPLIERS, plus ~3,500 entity sets on a typical tenant. Schema is per-instance (`$metadata` EDMX, often >10MB; custom fields/forms per install).
- Users: (1) integration developers at Priority customers/partners building sync jobs and EDI flows; (2) ops/finance analysts who need AR aging, order status, stock levels but only have the Priority UI or Power BI OData feeds; (3) AI-agent builders wiring Priority into automation (fast-growing: 3 MCP servers, n8n node, Zapier MCP all appeared 2025-2026).
- Data profile: transactional business documents (orders + subform lines), master data (customers, parts, suppliers, accounts), financial documents (invoices, payments), attachments (base64 data URIs in EXTFILES_SUBFORM), HTML text subforms.

## API Contract (exact, from official docs raw captures)
- Base URL: `https://{host}[/ui]/odata/Priority/{tabula.ini}[,{lang}]/{company}` — per-instance. Official public sandbox: `https://t.eu.priority-connect.online/odata/Priority/tabbtd38.ini,3/usdemo` (user `apidemo`, pass `123`, read-only, published in official docs).
- Auth: (1) Basic (username = "API User Name" from Personnel File); (2) **PAT** — sent as Basic with username=token, password=literal `PAT` (v19.1+, recommended); (3) OAuth2 Auth-Code+PKCE Bearer (requires paid External ID module, on-prem focus). Optional per-app license headers `X-App-Id`/`X-App-Key`.
- Query: `$filter` (eq/ne/gt/ge/lt/le, and/or, parens), `$select`, `$orderby`, `$top`, `$skip`, `$expand` (subforms; nested with `;`-separated options, `%3B` if IIS strips `;`), `$since` (changed-records, BPM entities only, UTC Z recommended), `$format=...odata.metadata=full`. **No `$count`.** Case-sensitive UPPERCASE form/field names.
- Write: POST create (headers `OData-Version: 4.0`, `Content-Type: application/json;odata.metadata=minimal`), deep insert (child arrays in POST body), PATCH update (no PUT!), DELETE, subform ops via `PARENT('key')/CHILD_SUBFORM(n)`, composite keys `(IVNUM='T9696',IVTYPE='A')`. Text subforms: `TEXT`/`APPEND`/`SIGNATURE`. Attachments: base64 data-URI in `EXTFILENAME` + optional `SUFFIX`.
- `$batch`: JSON body `{"requests":[...]}` with `id`/`dependsOn`/`atomicityGroup`, `$1/...` new-entity refs, max 100 ops, **no rollback**.
- Meta surfaces: `$metadata` (EDMX XML), `GetMetadataFor(entity='X')` (v25.0), `GetPriorityVersion()`, `GetODataVersion()`, `ClearEntityMetadata` (POST).
- Limits: **100 calls/min/user → 429**; 10 concurrent + 5 queued; 3-min request drop; rows capped by MAXAPILINES (default 2,000); 350MB response cap. Every write = 1 licensed transaction (metered, 10k/month packages).
- Errors: 200/201(delete!)/400/404/409/429/500; screen business-logic errors surface in response (`InterfaceErrors` text); errors JSON since v19.1.
- Dates: OData `DateTimeOffset` strings; **cliutil.ParseODataDate relevant** if any v3-style `/Date()/ ` appears (docs show ISO offsets; treat ISO as primary).

## Reachability Risk
- **None.** Official docs + official public sandbox with published demo credentials. No GitHub issues reporting 403/blocks; failures in the wild are licensing/config ("API Cannot Be Run for This Form" 400 when form not API-licensed) — a doctor-explainable state, not a block.
- Probe-safe endpoint: `GET {serviceRoot}/GetODataVersion()` or `GET {serviceRoot}/ORDERS?$top=1` on sandbox.

## Top Workflows
1. **AR / collections review**: pull open invoices per customer, aging buckets (0-30/31-60/61-90/90+), top debtors — the assafch MCP dedicates 7 of 72 tools to this.
2. **Order desk**: list/filter orders by status/customer/date, drill into ORDERITEMS_SUBFORM lines + status log, create/amend orders (deep insert), attach documents.
3. **Inventory checks**: part lookup (LOGPART), stock by warehouse (WARHSBAL), low-stock triage, price lookups.
4. **Bulk data sync / EDI**: nightly export of changed records (`$since` + `$expand`) into local systems; bulk load via `$batch` under the 100/min cap — today everyone hand-rolls throttling.
5. **Schema archaeology**: discovering which forms/fields/subforms exist on THIS tenant (per-install variance, custom `ORGT_`-prefixed fields), which forms are API-licensed, what changed after an upgrade.

## Table Stakes
- Generic entity CRUD on any form (all 6 competitors have it): list/get/create/update/delete + subform navigation.
- OData query passthrough: filter/select/orderby/top/skip/expand/since.
- $batch execution with dependencies + atomicity groups (n8n node, optima CLI).
- Attachments upload/download (MCP, n8n node); text subform read/write.
- Metadata: entity list, per-entity schema, version probes (MCP `list_entities`/`get_entity_schema`; optima `metadata fingerprint/diff`).
- Rate-limit awareness: built-in 100/min limiter, 429 retry, typed rate-limit exit code (optima exit 7; aviranbenmoshe MCP retry).
- Domain conveniences from assafch's 72-tool map: customer balance/aging, open debts, unpaid invoices, pending orders, stock levels, low-stock alerts, top customers/products, cash position.

## Data Layer
- Primary entities: ORDERS (+ORDERITEMS_SUBFORM), CUSTOMERS, AINVOICES, LOGPART, PORDERS, SUPPLIERS — synced into SQLite.
- Sync cursor: `$since` (BPM entities) falling back to full/windowed pulls with `$top`/`$skip` pagination under a built-in rate limiter.
- FTS/search: full-text over synced customers/parts/orders — **no Priority tool offers offline search today; clearest gap in the ecosystem** given the 100/min cap makes ad-hoc exploration painful.
- Metadata cache: parsed `$metadata` per tenant in SQLite (forms, fields, keys, subforms, mandatory flags) → powers schema search, field validation before writes, tenant diffing.

## User Vision
- (from goal statement) "With the CLI I want to be able to do everything you can with the API" — full surface coverage: query, CRUD, subforms, batch, attachments, text, metadata, version probes. Generic escape hatches (raw OData query, arbitrary entity CRUD) are mandatory, not optional.

## Product Thesis
- Name: priority-pp-cli
- Why it should exist: Priority has no official CLI and no official REST SDK. The ecosystem is fragmented (an MCP with 72 tools but no PAT support, a dead TS SDK, a rough n8n node) and every tool re-implements throttling and schema discovery. A single Go binary with generic OData coverage + offline SQLite sync/FTS + metadata intelligence + built-in fair-use throttling beats every existing tool simultaneously, and is agent-native out of the box.

## Build Priorities
1. Generic entity surface: `entity list/get/create/update/delete/subform` against ANY form, with full OData query flags — this is "everything the API can do."
2. Metadata intelligence: cached `$metadata` parse; `forms list/describe/search`, mandatory-field awareness, `GetMetadataFor`, version probes, ClearEntityMetadata.
3. Local store: sync ORDERS/CUSTOMERS/AINVOICES/LOGPART/PORDERS/SUPPLIERS with `$since` cursors; FTS search; offline analytics (aging, top debtors, stock).
4. Batch + writes: `$batch` builder with dependsOn/atomicityGroups; deep insert; text subforms; attachments up/down.
5. Fair-use protection: adaptive limiter under 100/min, 429 typed errors, 3-min timeout guard, MAXAPILINES pagination helper.
