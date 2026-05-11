# Unify CLI Brief

## API Identity
- **Domain:** B2B SaaS — Unify (Unify GTM) is a go-to-market outbound automation platform. The Data API exposes Unify's CRM-style object store: companies, people, opportunities, users, plus mirrored Salesforce objects, with custom-attribute extension.
- **Users:** Revenue/sales-ops engineers, AEs, and integration owners running Unify alongside Salesforce/HubSpot. The Gladly team uses it for retail account qualification scoring, outbound campaign sequencing (~3,400/day across inboxes), and SF-field-aligned account coverage frameworks.
- **Data profile:** Relational. Three resource layers: Objects (schemas), Attributes (fields, with options for select-type), Records (instances). Each record has a UUID + arbitrary typed attributes. Records reference other records (e.g., `company.opportunities` → opportunity[]).

## Reachability Risk
- **None.** Spec is at `https://api.unifygtm.com/data/v1/openapi.json` (208 KB, OpenAPI 3.0). HTTPS 200 with valid API key; clean response envelope. No history of 403/rate-limit issues on the Unify GitHub org. Rate limit is 100,000 req per 5-minute window (very generous).

## API surface (what the spec gives us)
- **Spec:** `https://api.unifygtm.com/data/v1/openapi.json`
- **Auth:** `X-Api-Key: <token>` header; env var `UNIFY_API_KEY` (matches Python SDK convention)
- **10 paths / 21 operations / 44 schemas**
- **Endpoints:**
  - Objects: list, create, get, update, delete
  - Attributes: list, create, get, update, delete (scoped to object)
  - Attribute Options: list, create, get, update, delete (for select/multi-select)
  - Records: create, get-by-id, update, delete, **find-unique**, **upsert**
- **What is conspicuously missing:** No `LIST records` endpoint. No `SEARCH records` endpoint. The only ways to retrieve a record are (a) by known UUID or (b) by `find-unique` with a `match` on one or more unique attributes. This is the central product opportunity — see Build Priorities.
- **Validation modes:** `strict` (fails on unknown/invalid attrs) and `ignore_invalid` (strips unknowns, replaces invalid known attrs with `undefined`). Per-request via `?validation_mode=` query param.
- **Response envelope:** `{"status": "success", "data": ...}` on success; `{"status": "<error_code>", "errors": [...], "message": "..."}` on 400/401/404/409/500/503.
- **Workspace inventory (this key):** 12 objects total — 4 Unify standard (company, opportunity, person, user) + 8 Salesforce-mirrored (account, campaign, campaign_member, contact, lead, opportunity, task, user). Company has 20 attributes (domain, industry, employee_count, last_activity_at, opportunities ref, people ref, record_owner ref, ...).

## Top Workflows
1. **Bulk upsert from CSV with idempotency** — Emily owns Salesforce fields, Nate runs the scoring builds. Today this is custom Python; the CLI should make it `unify upsert --object company --csv accounts.csv --match-on domain --validation strict --dry-run`.
2. **Local-first read/search/SQL** — Records have no list endpoint. The CLI syncs everything you have IDs for into SQLite, then `unify sql "SELECT ..."`, `unify search "<text>"`, and rich joins across Unify + Salesforce objects become possible offline. This is the #1 reason to install the CLI.
3. **Live find-by-key lookup** — `unify find company --domain gladly.com --json` (POST find-unique) returns the full record for inline use in agent pipelines, jq chains, scripts.
4. **Schema introspection + drift detection** — When Emily ships new Salesforce-mirrored fields or attribute options, `unify schema diff` against the last snapshot tells you what changed. `unify objects describe company --json` returns the full attribute spec.
5. **Cross-source audit + coverage** — Compare Unify standard `company` with Salesforce `salesforce_account` to find records present in one but not the other, scoring-rule mismatches, and stale `last_activity_at` records. This is exactly the work the Mondays-9:30-CT meeting circles around.

## Table Stakes (any decent CLI must match these)
- Auth via env var (`UNIFY_API_KEY`) plus `--key` flag.
- `--json` everywhere with `--select` field filtering (gravity: company/person/opportunity records are 20–50 attrs each).
- `--dry-run` on every mutation; CSV idempotent upserts; stdin batch support.
- `doctor` command (auth check, base-URL reachability, rate-limit headroom probe).
- Object + attribute schema commands (list/get/create/update/delete).
- Record commands (create/get/update/delete/find-unique/upsert) mirroring the spec 1:1.

## Data Layer
- **Primary entities:** `objects`, `attributes`, `attribute_options`, `records` (one per object type) — flatten records with a `data` JSON blob + indexed common columns (id, object_name, created_at, updated_at, plus high-gravity attrs).
- **Sync cursor:** Records have no list endpoint, so sync is **explicit-IDs**: the CLI keeps a `known_ids` table (added when records are seen via find-unique, upsert, get) and refreshes them. CSV import populates known_ids. Snapshot on every sync for drift.
- **FTS/search:** FTS5 index across all text/url/email attributes in all record tables. One unified `search` command returns hits across every object type.
- **Per-object tables:** `record_company`, `record_person`, etc. — so a typed column for `domain`, `industry`, etc. can be derived from the JSON blob during sync, enabling fast SQL.
- **Schema-aware:** Attribute metadata in a `schema` table refreshed by `objects sync`, so SQL queries know which columns exist per object.

## User Vision
The user (Nate) works directly with the Unify team at Gladly. The CLI should serve:
- **Operational audits:** "which retail accounts have scoring mismatches between SF lead_score and Unify scoring fields?"
- **Sales-ops field ops:** "Emily added 4 new SF-mirrored fields last week — what changed?"
- **Local rich querying:** "show me companies in industry='Retail' with employee_count >= 200 and at least one opportunity created in the last 30 days" (impossible via raw API).
- **Coverage reporting:** "% of SF accounts that exist as Unify companies, by industry segment."
Voice rule from the Unify meeting context: **operational, data-forward, specific**. Avoid marketing/AI-first framing in the README and SKILL.

## Product Thesis
- **Name:** `unify-pp-cli` (binary `unify-pp-cli`, slug `unify`)
- **Why it should exist:** Unify's REST API gives you write/upsert/lookup but **no way to list, scan, or search records**. Every serious read workflow has to be invented client-side. The CLI ships that read layer: SQLite-backed sync, FTS5 search, SQL across objects, snapshot/diff for schema and data drift, and bulk CSV upserts with idempotency. Plus the agent-native plumbing (`--json --select --dry-run`, typed exit codes, MCP tree) the official SDKs don't ship.
- **The differentiator in one sentence:** "Unify's API is write-only by design; this CLI is the read layer."

## Build Priorities

### Priority 0 — Foundation
1. SQLite store with: `objects`, `attributes`, `attribute_options`, per-object record tables + a unified `records_fts` FTS5 index.
2. `sync` command: refreshes objects + attributes for all object types, then refreshes records currently in `known_ids` via parallel `find-unique` or by-id GETs.
3. `doctor`, `objects list/get`, `attributes list/get` working end-to-end.

### Priority 1 — Absorb (match every existing tool)
- All 21 spec endpoints mapped to subcommands (objects/attributes/records CRUD + find-unique + upsert + options).
- Validation mode flag (`--validation strict|ignore_invalid`) on all mutations.
- `--json`, `--select`, `--csv`, `--compact`, `--dry-run` everywhere applicable.
- CSV import for `upsert` with `--match-on <attr>` and `--create-or-update <field=value>` mapping.
- Schema introspection (`objects describe <name>`, `attrs list <object>`).

### Priority 2 — Transcend (only possible because we have a local store)
1. `search "<text>"` — FTS5 across every synced record table; the read primitive the API can't give you.
2. `sql "<query>"` — read-only SQL on the local store; joins across `record_company` and `record_salesforce_account` in one query.
3. `find` (live) — wraps find-unique with a clean flag interface: `unify find company --domain gladly.com`.
4. `schema diff` — diffs current spec/workspace attributes vs. last snapshot; flags new Salesforce-mirrored fields, deleted attributes, changed types.
5. `coverage --left salesforce_account --right company --key domain` — set-difference report (missing-in-Unify, missing-in-SF, matched-but-stale by `last_activity_at`).
6. `audit-scores --object company --field unify_score --field salesforce_lead_score --threshold 50` — flag records where two score-fields diverge beyond a threshold (directly serves Nate's auto-deduct-50pts use case).
7. `import-csv --object company --file accounts.csv --match-on domain --plan` — preview the upsert plan (count of creates vs updates vs no-ops) before sending.
8. `references trace --from <record_id>` — walks reference attributes locally; "show every opportunity attached to this company" without N+1 API calls.

### Priority 3 — Polish
- Naming cleanup (operationId-derived commands like `find_unique_object_record` → `records find-unique`).
- Enrich flag descriptions with domain context (`--validation-mode "strict fails on unknown attrs (recommended); ignore_invalid strips unknowns silently"`).
- `--key`/env-var auth onboarding in `doctor` and README.

## Codebase Intelligence
- **Official Python SDK** (`unifygtm-sdk` on PyPI, `unifygtm/sdk-python` on GitHub): Pydantic-based, full coverage of the 21 endpoints. Confirms: auth env var is `UNIFY_API_KEY`, header is `X-Api-Key`. SDK has automatic retries (2x for 5xx + rate limits), streaming responses, raw-response access. Notably absent from SDK: any list-records or search helper — confirming the gap.
- **Official TypeScript SDK** (`unifygtm/sdk-typescript`): parallel coverage.
- **No competing CLI exists** for the Unify Data API.
- **No MCP server exists** for the Unify Data API.
- **No claude-plugin / no Anthropic-hosted skill** for Unify.
- **intent-js-client** (`@unifygtm/intent-client`): browser SDK for the *Intent* API (a separate surface — pixel/JS tracking, not the Data API). Out of scope.
- **unify-hackathon-demo**: a sample multi-agent Python app — interesting in that it shows Unify themselves recommend the Python SDK for AI/agent integrations; reinforces the agent-native angle for our CLI.

## Notes on the build
- The spec doesn't declare pagination on list endpoints, and there are no list endpoints for records anyway — so generated paginators aren't needed.
- Upsert is the richest endpoint: 6 distinct merge clauses (`match`, `create`, `update`, `update_if_empty`, `create_or_update`, `create_or_update_if_empty`). Worth making the CLI's `--mode create-only|update-only|both|both-if-empty` flag map onto these instead of forcing users to remember the JSON shape.
- Reference values can be supplied three ways (by-id, by-match, by-upsert). Worth a `--ref <field>=<lookup>` shorthand that picks the right shape based on what you pass.
- Validation mode defaults aren't documented, so we should print the effective mode in `--dry-run` output and `doctor`.
