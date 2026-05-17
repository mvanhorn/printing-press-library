# ServiceTitan Sales/Estimates (Salestech) CLI Brief

## API Identity
- **Domain:** Sales & Estimates module within ServiceTitan — managing customer-facing estimates (quotes), the line items (SKUs/labor/materials) they contain, and the sell/unsell lifecycle that converts a winning estimate into job/invoice flow.
- **Users:** Field-services contractors (HVAC, plumbing, well-drilling, electrical). Inside JKA Well Drilling: dispatchers, sales/CSRs, technicians who present estimates in the field, and ops staff measuring close rates and sold revenue.
- **Data profile:** ~13 endpoints covering 1 primary entity (Estimate), 1 child entity (EstimateItem), 1 audit feed (StatusChanges), and 2 export feeds with `from`/`includeRecentChanges` cursors. Methods skew read+update (6 GET, 5 PUT, 1 POST, 1 DELETE) — Sales-Tech is a state-management surface, not a high-write one.

## Reachability Risk
- **None.** Official ServiceTitan production API at `https://api.servicetitan.io/sales/v2`. JKA tenant `848413091` has live access. Six sibling ST module CLIs (JPM, CRM, Dispatch, Inventory, Pricebook, Memberships) have verified live composed-auth against this same tenant.

## Top Workflows
1. **Pull today's open estimates** — "What's quoted but not sold yet?" Dispatcher/manager wants the list of active estimates that haven't closed, ranked by age (stale-quote risk) and total $. Filters by `soldById` (sales rep), `jobId`/`projectId`, `totalGreater`. JKA equivalent: "Which well-pump replacement quotes have customer approval pending past day 3?"
2. **Audit an estimate's line items + status history** — Tech or CSR pulls one estimate, sees its `items` (SKUs, qty, price, billable totals), and walks the `status changes` timeline (Open → Sold / Dismissed / Unsold) with UTC timestamps to verify what happened when. Cross-references against the job. This is the "why was this estimate dismissed last Tuesday" forensic flow.
3. **Sell or dismiss in bulk during pipeline review** — Sales manager goes through stale or stuck estimates and either marks them sold (with `soldOn` + `soldBy`), dismisses them with a reason, or unsells one that closed prematurely. Today this is one estimate at a time in the ST web UI; a CLI lets the manager batch via `--ids` or pipe from `find`.
4. **Modify a quote in the field** — Tech is at the customer site, customer asks "can you swap the 1-HP pump for the 1.5-HP?" Tech (or back-office on their behalf) uses `estimates items put` to replace a line item, then `estimates get` to re-confirm the new total before printing/emailing the revised quote.
5. **Sync recent estimate changes for offline ops + analytics** — Nightly or hourly: pull the export feed (`/export/estimates` with `from` cursor), persist every estimate + line item locally, then run audits — close-rate by rep, average days-to-sell, dismissed-reason patterns, line-item SKU frequency, soldBy leaderboard. None of this exists in a single ST API call.

## Table Stakes (incumbent surface to absorb)
- Full CRUD on estimates: list (with all filter params), create, get, update, dismiss, sell, unsell.
- Line-item management: get, put (create-or-update), delete.
- Status-change audit feed per estimate.
- Two export feeds (legacy + current) with cursor + `includeRecentChanges`.
- Composed auth: `ST-App-Key` header + OAuth2 bearer (client_credentials). Per-tenant `{tenant}` path templating.
- `--ids` multi-select on list (max 50 per page).
- Date/total/sort filters on `Estimates_GetItems`.

## Data Layer
- **Primary entities:**
  - `estimates` — id, jobId, projectId, name, jobNumber, status (Open/Sold/Dismissed), reviewStatus, summary, total, subtotal, tax, businessUnitId, soldOn, soldBy (employee id), active, createdOn, modifiedOn, isRecommended, externalLinks[].
  - `estimate_items` — id, estimateId, sku (id+name+type+displayName), qty, unitRate, total, itemGroupName, itemGroupRootId, createdOn, modifiedOn, chargeableBillingType, costRate, totalCost, membershipTypeId.
  - `estimate_status_changes` — estimateId, fromStatus, toStatus, changedById, changedAtUTC.
  - `estimate_exports` (sync cursor state) — module=`estimates`, lastCursor, lastSyncedAt.
- **Sync cursor:** `from` (UTC ISO 8601) + `includeRecentChanges`; standard ST export-feed pattern (same shape as CRM/JPM siblings).
- **FTS/search:** name, summary, jobNumber, sku.name, sku.displayName, itemGroupName. Power-user search like "find every estimate that mentions 'well pump' and is over $5k and still Open."

## Codebase Intelligence
- Sibling pattern verified across 6 prior ST CLIs (`servicetitan-{jpm,crm,dispatch,inventory,pricebook,memberships}`). All use:
  - Composed auth: `ST_APP_KEY` apiKey header + OAuth2 bearer; `Load()` does `strings.TrimSpace` on all four env vars (JKA env-newline gotcha).
  - Generator v4.8.0 closed #1303 (apiKey wiring), #1305 (x-tenant-env-var sync wiring), #1304 (.git in publish), #1150 (.exe probe). Verify mid-run rather than pre-patch.
  - Still open: #1332 (sync tenant-templated paths empty + no fill), #1333 (dual-scheme apiKey env-var aliasing — supersedes #1303 broader), #1334 (mcp-sync auth_type drift), #1208 (mcp-sync spurious cmd dirs from info.title slug). Patches recorded in `.printing-press-patches.json` with `// PATCH:` markers per the publish-CI contract.
  - Hand-built novel commands live in `internal/<module>/` with table-driven tests; thin Cobra wrappers in `internal/cli/<command>.go` short-circuit on `dryRunOK(flags)` and annotate read-only commands with `mcp:read-only`.

## User Vision
- Pierce explicitly named this `servicetitan-salestech` and provided the pre-enriched spec — this is the 7th of 25 planned per-module ST CLIs. The shared goal is to replace the 400+-tool general ST MCP with N lean per-module pp-mcp binaries, collapsing the per-turn token tax.
- For this module specifically, the Sales/Estimates surface is small (13 endpoints) but the workflows around it are state-heavy: knowing the lifecycle of every estimate, who sold what, what got dismissed and why, and how the estimate-to-job conversion is performing. The transcendence is in cross-entity audits the API can't answer in one call.

## Product Thesis
- **Name:** `servicetitan-salestech-pp-cli` (binary), `servicetitan-salestech-pp-mcp` (MCP intent surface).
- **Why it should exist:** Every other estimate audit today either lives in the ST web UI (one estimate at a time, no cross-cutting queries) or is a hand-written report. A local SQLite mirror + FTS gives close-rate, days-to-sell, dismissal-reason, soldBy-leaderboard, and stale-quote queries that compose with `jq` / pipes / agents. The MCP collapses 13 endpoint mirrors into 2 intent tools (the proven Cloudflare pattern) so cost-per-turn stays small even when the agent has the whole CLI in reach.

## Build Priorities
1. **Foundation (generator-emitted Priority 0/1):** all 13 endpoints as typed commands; data layer for `estimates`, `estimate_items`, `estimate_status_changes`; sync wired via `defaultSyncResources()` + `syncResourcePath()` with `{tenant}` substitution; composed auth + ST_APP_KEY wiring (verify generator handles this in v4.8.0; if not, port from pricebook).
2. **Absorb (Priority 1, hand-finished after generation if needed):** any spec-derived command that ends up dead-flagged, ugly, or missing examples gets a domain-realistic example (no `12345` literals — use a no-flag smoke pattern first, then a literal id second; pattern lives in pricebook's `wbs.go`/`projects_rollup.go`).
3. **Transcendence (Priority 2 — hand-built):** the 8-12 cross-entity commands that justify the CLI. Detailed in the absorb manifest below.
4. **Polish (Priority 3):** flag-description enrichment for auth/id/filter params, README's `## Known Gaps` if anything ships gated, MCP intent-tool surface verified via `tools-audit`.
