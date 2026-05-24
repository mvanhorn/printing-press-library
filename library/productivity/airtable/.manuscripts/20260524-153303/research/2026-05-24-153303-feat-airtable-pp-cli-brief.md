# Airtable CLI Brief

## API Identity
- Domain: Cloud spreadsheet/database hybrid. REST Web API at `https://api.airtable.com/v0/` (data plane) plus a Meta API for schema (`/v0/meta/bases`, `/v0/meta/bases/{baseId}/tables`).
- Users: Ops, RevOps, marketing, and product teams using Airtable as a system of record; integration developers wiring Airtable into Zapier/Make/n8n/Retool; agents and AI assistants reading/writing records on behalf of users.
- Data profile: Small-to-medium row counts per table (Team tier caps ~50k records/base; Pro/Business higher). Records are JSON objects with typed fields (text, number, singleSelect/multipleSelects, date, attachment, linked-record, formula, rollup, lookup, button, user). Heavy reliance on linked-record relationships across tables. List endpoints paginate at 100 records/page via `offset` cursor.

## Reachability Risk
- None / Low — No 403/Cloudflare-block issues found in `gtalarico/pyairtable` or `airtable/airtable.js` issue trackers. Public HTTPS endpoint, PAT-based auth, no IP allowlisting on free tier. Documented rate limits (see below) are the primary throttle, not access blocks. Evidence: pyairtable issues #427, #313, #142, #92, #27 all relate to 429 throttling, not access denial.

## Top Workflows
1. **Records list + filter** — `GET /v0/{baseId}/{tableIdOrName}?filterByFormula=...&view=...&fields[]=...&pageSize=100`. The 95% call. Agents need this with paging + jq-friendly output.
2. **Schema dump** — `GET /v0/meta/bases/{baseId}/tables` returns all tables, fields (with types and options), and views in one call. Critical for any non-trivial workflow because field types determine valid write payloads.
3. **Record upsert** — `PATCH /v0/{baseId}/{tableIdOrName}` with `performUpsert.fieldsToMergeOn` (Airtable's native upsert, up to 10 records/request). High-traffic for sync use cases.
4. **Bulk create/update/delete** — `POST/PATCH/DELETE /v0/{baseId}/{tableIdOrName}` with arrays of up to 10 records. Required to stay within the 5 req/sec/base limit when moving more than a handful of rows.
5. **Webhook lifecycle** — `POST /v0/bases/{baseId}/webhooks`, `GET /v0/bases/{baseId}/webhooks/{webhookId}/payloads`. The only first-party way to get change events without polling.

## Table Stakes
- PAT auth via `Authorization: Bearer pat...` header; env var convention `AIRTABLE_API_KEY` (or `AIRTABLE_PAT`).
- List bases (`GET /v0/meta/bases`), list tables (`GET /v0/meta/bases/{baseId}/tables`), describe table (fields + views).
- Records: list (with `filterByFormula`, `view`, `fields[]`, `sort`, `maxRecords`, `pageSize`, `offset`, `returnFieldsByFieldId`, `cellFormat`, `timeZone`, `userLocale`), get by ID, create (single + batch ≤10), update (PATCH partial, PUT replace, batch ≤10), delete (single + batch ≤10), search.
- Native upsert via `performUpsert.fieldsToMergeOn` (≤10 records/request).
- Schema mutations: create/update table, create/update field (Meta API).
- Comments: list, create, update, delete on a record.
- Webhooks: create, list, delete, refresh, list payloads (with cursor), enable/disable notifications.
- Attachments: upload via URL on record write; download via signed URL from `attachment.url`.
- Field ID vs name addressing (`returnFieldsByFieldId=true`) — critical because field renames break name-based code.
- 429 handling with 30-second backoff on rate limit hit; retry with jitter.
- Output formats: JSON (machine), TSV/CSV (human + spreadsheet pipe), pretty table (terminal).
- Multiple PAT profiles (config file or env-var switching) so users can hop between workspaces.

## Data Layer
- Primary entities: **Records** (95% of traffic), **Tables**, **Bases**, **Fields**, **Views**, **Webhooks**, **Comments**, **Collaborators**, **Workspaces**, **Attachments**.
- Sync cursor: Two options.
  1. **Webhook payloads** — `GET /v0/bases/{baseId}/webhooks/{webhookId}/payloads?cursor={n}` returns ordered change events with a monotonic cursor. This is the canonical incremental-sync mechanism.
  2. **Last-modified-time poll** — `LIST records?sort[0][field]=Last Modified Time&sort[0][direction]=desc&filterByFormula=IS_AFTER({Last Modified Time}, '{lastSync}')` requires a `Last Modified Time` field configured on the table. Less reliable than webhooks but works without webhook setup.
- FTS/search: Index locally for `(baseName, tableName, fieldName, recordId, primaryFieldValue, all-text-field-concat)`. Airtable's `filterByFormula` supports `SEARCH()` and `FIND()` but only server-side and only one base at a time — local SQLite FTS5 over a synced mirror is the differentiator vs every existing CLI.

## Codebase Intelligence
- Source: `domdomegg/airtable-mcp-server` (444 stars, last release v1.13.0 on 2026-03-07, TypeScript 95.7%).
- Auth: PAT in `AIRTABLE_API_KEY` env var, passed as `Authorization: Bearer ${token}`. No OAuth2 path in the MCP server; PAT only.
- Data model: 15 MCP tools across Discovery (`list_bases`, `list_tables`, `describe_table`), Records (`list_records`, `search_records`, `get_record`, `create_record`, `update_records`, `delete_records`), Schema (`create_table`, `update_table`, `create_field`, `update_field`), and Comments (`create_comment`, `list_comments`). No webhook or attachment tools — gap to absorb.
- Rate limiting: Not documented in the server; appears to rely on caller-side throttling. Likely 429-vulnerable under burst load.
- Architecture: Stateless. No local cache, no SQLite, no incremental sync. HTTP transport mode (`MCP_TRANSPORT=http`) for remote use. **Key insight: every existing MCP/CLI is a thin pass-through. None of them solve the "5 req/sec/base + paginate-by-100" problem with a local mirror, which is the obvious gap.**

## Source Priority
(Skip — single-source CLI.)

## Product Thesis
- Name: airtable-pp-cli
- Why it should exist: Airtable's official tooling stops at the JS/Python SDKs and a Blocks dev CLI; the ~10 community CLIs and ~8 MCP servers in the wild are all thin REST wrappers — no local store, no incremental sync, no offline query, no cross-base joins. Every agent or pipeline that touches Airtable burns the 5 req/sec/base budget re-fetching the same records, and `filterByFormula` is too anemic to replace SQL. This CLI absorbs every Airtable SDK method (records, schema, webhooks, comments, attachments), adds a SQLite-backed local mirror driven by webhook-cursor incremental sync, and lets agents run jq + SQL over a synced base without round-tripping every call. The unique value: offline-queryable Airtable, with the full surface area in one binary.

## Build Priorities
1. **P0 — Auth + base/table/record read path.** PAT config (env var + profile file), `airtable bases list`, `airtable tables list <baseId>`, `airtable schema dump <baseId>` (Meta API), `airtable records list <baseId> <tableId>` with pagination, `filterByFormula`, `view`, `fields[]`, JSON + TSV output. Built-in 429 backoff with jitter. This alone clears the table-stakes bar.
2. **P0 — Record write path.** `airtable records create|update|delete|upsert` with batch ≤10 enforced client-side; native upsert via `performUpsert.fieldsToMergeOn`. Idempotency keys for retries.
3. **P1 — Local SQLite mirror + incremental sync.** `airtable sync init <baseId>` creates schema-typed tables in a local SQLite file; `airtable sync pull <baseId>` runs webhook-payload pull (or Last-Modified-Time fallback) and applies deltas. `airtable query "SELECT ..."` runs SQL against the mirror. This is the differentiator.
4. **P1 — Webhooks.** `airtable webhooks create|list|delete|payloads`. Required to power the sync cursor and to expose change-feed for downstream tooling.
5. **P2 — Schema mutations.** `airtable tables create|update`, `airtable fields create|update`. Useful but lower-frequency; most users design schema in the Airtable UI.
6. **P2 — Comments + attachments.** `airtable comments list|create|update|delete`, `airtable attachments upload|download`. Round out parity with the SDK surface.
7. **P3 — Multi-base/multi-profile + cross-base SQL.** Sync N bases into one SQLite file under separate schemas; allow `JOIN` across bases in `airtable query`. No existing tool does this.
