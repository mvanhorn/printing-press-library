# ServiceTitan Pricebook CLI Brief

## API Identity
- **Domain:** ServiceTitan Pricebook v2 — categories, client-specific pricing (rate sheets), discounts & fees, equipment, materials, materials markup, pricebook bulk operations, pricebook images, services, and export feeds.
- **Users:** Field-service operations teams running ServiceTitan as their ERP. Primary persona: JKA Well Drilling ops/automation owner who runs Claude Code on Windows and maintains the pricebook by hand today. The pricebook module is the **center of gravity** for this user's documented workflows (2M pricebook ingestion, vendor-quote → pricebook updates, markup discipline, warranty attribution).
- **Data profile:** Tenant-scoped (`/tenant/{tenant}/...` on every path). Composed auth (static `ST-App-Key` header + OAuth2 client-credentials bearer). 40 endpoints across 10 resources — list + targeted CRUD + 4 export feeds + 2 bulk operations.
- **Servers:** `https://api.servicetitan.io/pricebook/v2` (Production), `https://api-integration.servicetitan.io/pricebook/v2` (Integration).

## Reachability Risk
- **None.** ServiceTitan is a tier-1 SaaS API. Four sibling CLIs (`servicetitan-crm`, `servicetitan-dispatch`, `servicetitan-inventory`, `servicetitan-jpm`) are already shipped in `~/printing-press/library/` against this exact base host. All four ST auth env vars (`ST_APP_KEY`, `ST_CLIENT_ID`, `ST_CLIENT_SECRET`, `ST_TENANT_ID`) are present and confirmed in the user's environment. Phase 5 live read-only testing will succeed.

## Top Workflows
1. **2M pricebook ingestion** — search Materials + Equipment, apply hold-markup when a cost changes, **always sync the 2M Part #** into `primaryVendor.vendorPart`, put warranty text in the description. (memory: `project_pricebook_2m_workflow`)
2. **Vendor quote → pricebook update** — ingest a vendor PDF/Excel quote, self-healing part-# match against existing SKUs, multi-vendor primary logic, daily approval digest, then write costs back. (memory: `project_vendor_quote_ingestion`)
3. **Markup discipline** — keep `(price − cost)/cost` aligned to the `MaterialsMarkup` tier ladder; when a vendor cost moves, the price must move with it.
4. **Warranty attribution audit** — every warranty must be prefixed `Manufacturer's` so it is clear JKA is not providing it; JKA's own 1-year parts & labor offering lives alongside it in the description. (memory: `feedback_warranty_attribution`, `project_jka_service_warranty`)
5. **Category taxonomy management** — list/create/update/delete categories, keep the hierarchy clean, find orphaned SKUs in inactive categories.

## Table Stakes
- Every endpoint produces a working Cobra command (40 commands minimum after complex-body skips). Base CRUD must work.
- Composed auth works first try: `auth` exchanges `ST_CLIENT_ID`/`ST_CLIENT_SECRET` for a bearer token, persists it, every call adds both `ST-App-Key` and `Authorization` headers; refresh on 401.
- `--json` / `--select` / `--csv` / `--limit` on every list command; `--dry-run` on every mutation.
- Local SQLite store for materials, equipment, services, categories, discounts-and-fees, materials-markup, rate sheets + `meta` table for sync cursor.
- `doctor` confirms `ST_APP_KEY` + bearer token + `ST_TENANT_ID` + base URL.
- MCP surface (Cobra-tree mirror) for Claude Desktop / Code agents — the reason this per-module project exists.

## Data Layer
- **Primary entities (sync target):** `materials`, `equipment`, `services`, `categories`, `discounts-and-fees`, `materials-markup`, `client-specific-pricing` (rate sheets), `material-cost-types` (small lookup). Export feeds (categories/equipment/services/materials) are continuation-token feeds, not sync targets.
- **Sync cursor:** `modifiedOnOrAfter` query param (standard ST pattern); `modifiedOn` is on every SKU response. Store last-sync timestamp in `meta`.
- **FTS/search:** material/equipment/service `code` + `displayName` + `description`; category `name` + `description`; vendor part numbers (`primaryVendor.vendorPart`, `otherVendors[].vendorPart`).
- **Snapshot table:** `sku_cost_history` — append `(sku_type, sku_id, cost, price, vendor_part, modifiedOn, snapshot_at)` on every sync. This is what makes cost-drift, markup-drift, and stale-price transcendence features one-shot.

## Codebase Intelligence
- Source: 4 sibling generated CLIs in `~/printing-press/library/` (`servicetitan-crm`, `servicetitan-dispatch`, `servicetitan-inventory`, `servicetitan-jpm`) + their manuscripts.
- Auth: composed (`apiKey` header + OAuth2 bearer). The enriched spec already carries `components.securitySchemes` with `x-auth-vars`: `ST_APP_KEY` (`per_call`), `ST_CLIENT_ID` + `ST_CLIENT_SECRET` (`auth_flow_input`), each noting whitespace is stripped defensively — a known JKA gotcha (`feedback_credential_diagnostics`, `feedback_powershell_vs_bash_env`).
- Tenant: `info.x-tenant-env-var = ST_TENANT_ID`. Every path is `/tenant/{tenant}/...` — the generated client must inject the tenant ID from config.
- MCP: spec already carries `x-mcp` = `{ transport: [stdio, http], orchestration: code, endpoint_tools: hidden }` — the Cloudflare large-surface pattern is pre-applied. 40 endpoints + ~13 framework tools + ~12 novel commands ≈ 65 tools, which warrants code orchestration.
- Rate limiting: ServiceTitan documents ~7,000 req/hr per environment per app key; `cliutil.AdaptiveLimiter` handles 429 backoff (sibling CLIs already use this).
- **Retro carry-forward (from `servicetitan-inventory` retro, 2026-05-13):** v4.5.0 shipped three generator bugs against ST module specs — (1) composed-auth OAuth half not wired (#1303, fixed in v4.5.1), (2) `defaultSyncResources()` + `syncResourcePath()` emitted as empty stubs (#1305), (3) `{tenant}` placeholder not auto-substituted from `ST_TENANT_ID` (#1332). This run uses the freshly-upgraded **v4.6.1** binary. **Phase 2 MUST verify** `internal/client` wires both auth headers and `internal/cli/sync.go` has a non-empty resource registry with tenant substitution; patch using the sibling template if any regressed.

## User Vision
From environment + auto-memory:
- **Replace the heavy general ST MCP** with per-module CLIs/MCPs so an agent doing pricebook work loads only pricebook tools.
- **Cut tokens** by exposing ~65 focused MCP tools (code-orchestrated) instead of the 600+ the general ST MCP advertises.
- **Same auth/store pattern** as the existing sibling CLIs so the JKA agent stack stays coherent.
- The pricebook CLI should make the documented JKA workflows — 2M ingestion, vendor-quote reconcile, markup discipline, warranty attribution — into one-shot agent-native commands instead of manual UI clicking.

## Source Priority
- Single source (ServiceTitan Pricebook v2 OpenAPI, user-provided enriched spec). No priority gate needed.

## Product Thesis
- **Name:** `servicetitan-pricebook` (binary: `servicetitan-pricebook-pp-cli`, MCP: `servicetitan-pricebook-pp-mcp`)
- **Why it should exist:** The pricebook is where JKA's margin lives — costs, prices, markup tiers, vendor part numbers, warranty text. Today it is maintained by hand in the ST UI. A per-module agent-native CLI with a local SQLite cache turns "is our markup still right after this vendor cost change?" and "which SKUs are missing a 2M part number?" into one command, and slots into the existing per-module ST CLI family without loading the 600+-tool general ST MCP.

## Build Priorities
1. **Foundation** — config (tenant + composed auth), client (auto-inject `ST-App-Key` + bearer, refresh on 401, tenant substitution), store (materials/equipment/services/categories/discounts/markup/ratesheets/cost-types + `meta` + `sku_cost_history`).
2. **Absorbed** — all 40 endpoints as Cobra commands. `sync` pulls cacheable entities into the store and snapshots cost/price history. FTS5 search across cached SKUs.
3. **Transcendence** — novel features only this CLI can do because everything is in SQLite, composed-auth-aware, ST-pricebook-shape-aware, and grounded in JKA's documented pricebook workflows. (See absorb manifest.)
