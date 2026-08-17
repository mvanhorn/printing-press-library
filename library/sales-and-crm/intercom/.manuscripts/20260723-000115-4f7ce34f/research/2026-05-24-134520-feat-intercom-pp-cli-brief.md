# Intercom CLI Brief

## API Identity
- **Domain:** Customer messaging / support / help-center platform (paid SaaS, customer-engagement category)
- **Users:** support teams, customer-success ops, growth/marketing teams running in-app messaging campaigns, AI/agent teams routing conversations through Fin
- **Data profile:** REST 2.13 (latest stable 2.15 as of Oct 2025), Bearer-token auth, JSON request/response, ~150+ operations across ~30 resource groups. Workspace-scoped tokens. Regional base URLs for US / EU / AU. Cursor pagination + a legacy scroll API (1-min idle expiry) for >10k companies. Async job pattern for Data Export. Hard rate limit ~1000 req/10s per app.

## Reachability Risk
**None.** Paid SaaS with a heavily monitored public API. The `intercom-node` issue tracker has zero open issues for 403/401/blocked endpoints; the only rate-limit issue is a config-error report. The 2.13 spec is Intercom-maintained at `intercom/Intercom-OpenAPI` (last touched October 2025).

## Top Workflows
1. **Search and triage conversations** — open by tag/assignee/state, filter by date range, find by substring. Every existing MCP server prioritizes this.
2. **Sync contacts + conversations to a local store for analytics** — `fast-intercom-mcp` exists exclusively for this; data-warehouse teams replicate Intercom to Snowflake/BigQuery on a recurring cadence.
3. **Bulk tag/assign/close conversations** — incident-response pattern: "tag every conversation mentioning X with `outage-2026-05`." No native bulk endpoint; orchestrated client-side.
4. **Help-center article CRUD and export to git** — docs teams version-control articles outside Intercom. `kaosensei/intercom-mcp` exists for this single workflow.
5. **Upsert contacts + custom attributes from external systems** — CRM sync, identity stitching, populating custom attributes from a warehouse.

## Table Stakes
- Every resource group reachable as a subcommand (`conversations list`, `contacts search`, `tickets create`, `articles update`, …)
- Bearer-token auth with `INTERCOM_ACCESS_TOKEN` env fallback
- `--region us|eu|au` (or `INTERCOM_REGION`) — official Intercom MCP is US-only; this is a real gap
- Cursor pagination + scroll-API support
- Structured search ("contacts where role=user AND custom.signup_date > 2025-01-01")
- Bulk operations: tag/assign/close many conversations from stdin or file
- Async-job polling for Data Export
- Output formats: JSON (default), JSONL, CSV, table
- Rate-limit-aware backoff respecting `X-RateLimit-Remaining`
- Local SQLite mirror with `sync`, `sql`, `search`

## Data Layer
- **Primary entities (sync targets):**
  - `contacts` — 100k–500k typical, sync by `updated_at` descending
  - `companies` — 1k–50k, scroll API
  - `conversations` — 200k–2M, sync by `updated_at` descending; parts fetched lazily
  - `conversation_parts` — messages within conversations; 5–20× conv count; fetched on demand
  - `tickets` — 5k–100k, sync by `updated_at`
  - `articles` — 50–5,000, sync by `updated_at`
  - `notes` — fetched per parent (contact/company)
- **Low-cardinality reference data (full-reload):** `tags`, `segments`, `admins`, `teams`, `data_attributes`, `subscription_types`, `away_status_reasons`, `brands`
- **Sync cursor:** persisted `last_synced_at` per resource; incremental run uses `search` with `updated_at > $cursor`
- **FTS/search:** FTS5 on conversation parts (`body`), articles (`title + body`), contacts (`name + email`), companies (`name + custom_attributes`)
- **Cardinality constraint:** at 150 results/page, a 500k-conversation full sync ≈ 3,300 requests; throttle to ~30 req/s sustained to stay under the 1000 req/10s app limit

## Codebase Intelligence
- **Source:** [intercom/Intercom-OpenAPI](https://github.com/intercom/Intercom-OpenAPI) (vendor-maintained), official Node SDK [intercom/intercom-node](https://github.com/intercom/intercom-node), Python SDK [intercom/python-intercom](https://github.com/intercom/python-intercom)
- **Auth:** Bearer; header `Authorization: Bearer <token>`; required `Intercom-Version: 2.13` (or `2.15`); env var `INTERCOM_ACCESS_TOKEN`
- **Data model:** workspace-scoped tokens; PAT-style (no expiry until rotated); search-vs-list split for contacts and conversations (search supports nested AND/OR predicates, excludes merged contacts)
- **Rate limiting:** 1000 req / 10s per app; signaled by `X-RateLimit-Remaining` and `X-RateLimit-Reset`; backoff sane, no cliffs
- **Architecture:** REST + a small set of async jobs (`/jobs/<id>` polling) for Data Export; Fin AI agent endpoints (`/fin/start`, `/fin/reply`) stream events via webhook only; multi-brand workspaces gated by `brand_id` query param on many endpoints

## Product Thesis
- **Name:** `intercom-pp-cli`
- **Display name:** Intercom
- **Why it should exist:**
  - The official Intercom MCP server exists but exposes only **6 tools** (search/fetch + 2 paired typed endpoints) and is **US-region only**. EU and AU workspaces — a significant share of Intercom's customer base — have no agent surface at all.
  - The Node and Python SDKs are excellent libraries but require writing code for every workflow.
  - No existing CLI offers a local SQLite mirror; analytics teams currently either pay for the Data Export API and write custom ingestion, or build their own pipeline.
  - Community CLIs are all single-digit-star and scoped to a niche or stale.
  - One published differentiator: a recent supply-chain note flags `intercom-client@7.0.4` (npm) as compromised — a single static Go binary sidesteps the entire npm dep tree.

## Build Priorities
1. Full 2.13 spec coverage — every endpoint as a subcommand, generated from the official OpenAPI
2. Region-aware base URL handling (`--region us|eu|au` / `INTERCOM_REGION`) — wedge against the official MCP
3. Local SQLite mirror with `sync` for the four big entities (contacts, conversations, conversation_parts, tickets) plus articles, companies, and the low-cardinality reference tables
4. Search-API translation: agent-friendly `--filter` syntax that compiles to Intercom's nested predicate AST
5. Bulk operations: `conversations bulk-tag`, `conversations bulk-assign`, `conversations bulk-close` reading IDs/URLs from stdin
6. Async-job polling helper for Data Export
7. MCP surface — every Cobra command auto-mirrored as an MCP tool (the runtime walker handles this; verify it covers all generated typed endpoints)

## Specs

- **Primary spec URL:**
  - `https://raw.githubusercontent.com/intercom/Intercom-OpenAPI/main/descriptions/2.13/api.intercom.io.yaml`
- **Alternative (latest):** `descriptions/2.15/api.intercom.io.yaml` — pinning to 2.13 reduces churn risk and aligns with the user-floor convention
