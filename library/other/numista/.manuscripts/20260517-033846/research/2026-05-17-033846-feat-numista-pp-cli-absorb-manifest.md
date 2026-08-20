# Numista CLI Absorb Manifest

## Source Tools Surveyed
| Tool | Surface | What they expose |
|------|---------|------------------|
| **namachieli/numista-api-sdk** (Python) | SDK | All 17 free endpoints + OAuth label management (myToken, myTokenRefresh, myUserId, myTokenExp) + `validateGrade` + `schemaFind`/`schemaGenerateBody` |
| **numistalib** (Python) | SDK + cache | 10 service modules covering all read endpoints + RFC 9111 HTTP caching (SQLite) + exponential backoff (tenacity) + rate-limiting (pyrate-limiter) |
| **@leopiccionia/numista-sdk** (npm, TS) | SDK | Catalogue + Collection + OAuth surface for browser+server |
| **MihajloNesic/Numista** (Windows GUI) | Legacy | Pre-v3 catalogue browse only — superseded |

**No competing CLI. No MCP server. No Claude plugin. Greenfield.**

## Absorbed (match or beat everything that exists)

Every read+write surface from the active SDKs, mirrored as Cobra commands and re-exposed via MCP through the cobratree mirror. All commands support `--json`, `--select`, `--quota` (root flag), `--dry-run` where mutating, `--db` for offline cache reuse.

| # | Feature | Best Source | Our Command | Added Value | Status |
|---|---------|-------------|-------------|-------------|--------|
| 1 | Search types by text | namachieli `searchTypes`, numistalib `types.search` | `types search` | Local FTS5 fallback when cache is fresh; `--cached` forces zero-call | ship |
| 2 | Get type by ID | namachieli `getType` | `types get N#` | Cache-first; `--refresh` to force live; SQL composable | ship |
| 3 | List issues of a type | namachieli `getIssues` | `types issues N#` | Persists to local `issues` table | ship |
| 4 | Get prices for an issue | namachieli `getPrices` | `types prices N# issue-id` | Stores snapshot to `prices` table with timestamp | ship |
| 5 | List issuers | namachieli `getIssuers` | `issuers list` | Cache-first; FTS over issuer names | ship |
| 6 | List mints | namachieli (extends) | `mints list` | Cache-first | ship |
| 7 | Get mint by ID | spec only | `mints get id` | Cache-first | ship |
| 8 | List reference catalogues | namachieli `getCatalogues` | `catalogues list` | Cache-first; FTS over catalogue titles | ship |
| 9 | Get publication by ID | spec | `publications get id` | Cache-first | ship |
| 10 | Get user details | namachieli `getUser` | `users get user-id` | — | ship |
| 11 | List user collections (folders) | namachieli `getUserCollections` | `users collections user-id` | — | ship |
| 12 | List collected items | namachieli `getCollectedItems` | `users items user-id` | Streams pages; persists to `collected_items` | ship |
| 13 | Get one collected item | namachieli `getCollectedItem` | `users item user-id item-id` | — | ship |
| 14 | Add a collected item | namachieli `addCollectedItem` | `users items add user-id` | `--dry-run` shows request; `--from-file` for batch import | ship |
| 15 | Edit a collected item | namachieli `editCollectedItem` | `users items edit user-id item-id` | PATCH semantics; `--dry-run` | ship |
| 16 | Delete a collected item | namachieli `deleteCollectedItem` | `users items delete user-id item-id` | `--dry-run`; `--force` required | ship |
| 17 | OAuth token (client_credentials) | namachieli `myToken` | `auth login` | Stores token in `~/.numista-pp-cli/auth.json` (mode 0600); `auth status` shows expiry; `auth refresh` rotates | ship |
| 18 | OAuth token (authorization_code) | spec | `auth login --authorization-code --scope view_collection` | Local callback server prints redirect URL; tokens persisted | ship |
| 19 | Grade validation | namachieli `validateGrade` | `grades list` | Lookup helper; no API call | ship |
| 20 | Schema introspection | namachieli `schemaFind`/`schemaGenerateBody` | `agent-context spec` | Already provided by Printing Press base | ship |
| 21 | HTTP caching | numistalib (RFC 9111) | typed tables + `lookup_log.cache_hit` | SQLite-backed; cache hits are free, never count against quota | ship |
| 22 | Doctor / connectivity check | (table stakes) | `doctor` | Verifies `NUMISTA_API_KEY`, network reachability, current quota | ship |

## Transcendence (only possible because we cache and run locally)

| # | Feature | Command | Buildability | Why Only We Can Do This | Score |
|---|---------|---------|--------------|------------------------|-------|
| 1 | **Monthly quota tracker + short-circuit** | `numista-pp-cli --quota` / `--quota-only` (root flags) | hand-code | Numista exposes no quota endpoint; the 2K/month cap is enforced server-side but only observable from the client. Tracking it locally in `lookup_log` lets the CLI forecast batch cost vs current remaining quota before spending a call. **Adapted from PCGS to monthly reset.** | 10 |
| 2 | **Lookup-log audit** | `audit --by day` / `--by endpoint` / `--by type` | hand-code | Direct SQL over `lookup_log` answers "what burned my quota this month?", "which endpoints did I hit most," "which type IDs am I repeatedly looking up — should I cache them?" Zero API cost. | 9 |
| 3 | **Quota-aware batch lookup** | `types batch --file ids.txt --resumable --checkpoint ck.json` | hand-code | Parse a CSV/text/JSONL list of type IDs, look up each with cache reuse. `--dry-run` forecasts cost (live vs cache hits, %-of-quota). `--resumable` splits a list larger than the monthly budget across UTC months. Same shape as PCGS `coin batch`. | 10 |
| 4 | **Series scan** (price/mintage curve across all years of one type) | `types series N#` | hand-code | Pulls every issue + every issue's prices for one type into local store in one quota-aware command, then prints year-by-year mintage and price evolution. PCGS analog: `coin pop-curve`. | 9 |
| 5 | **Collection valuation** | `collection value` | hand-code | For every locally-cached `collected_item`, pull its issue prices and sum at current grade. Refuses to start when remaining quota < items missing price data. Prints total + per-item delta from last valuation. Annotates items with stale price data. | 10 |
| 6 | **Stale-price refresh** | `refresh --older 30d [--field prices]` | hand-code | Refresh only the fields Numista actually changes — prices, mintage updates — leaving cataloger-set identity fields (title, composition) untouched. `--dry-run --older 30d` lists which cached types need refresh without spending a call. PCGS analog: `refresh`. | 8 |
| 7 | **Issuer-scoped crawl** | `issuers crawl issuer-code --years 1900-1950` | hand-code | Crawl all types from one issuer matching a year range, persist to local store, print summary table. Sets the up-front conditions (e.g., "Australia 1900-1950 has 218 types; estimated cost 219 calls = 11% of monthly quota"); user confirms before crawl starts. | 8 |
| 8 | **Watchlist** | `watchlist add N# / list / check` | hand-code | Track price changes for a set of types over time. `check` refreshes prices, persists per-call snapshot to `prices`, prints diff vs last snapshot. Surfaces material price moves an active collector cares about. | 8 |
| 9 | **Cross-issuer search via SQL** | `sql "SELECT ..."` (already from Printing Press) | spec-emits | SQL-composable local store answers "all 1940-1950 silver Australian coins worth >$50" in zero API calls once synced. The generator emits `sql` as a stock command; we just need the schema. | 7 |
| 10 | **Collection import** | `users items add --from-file collection.csv --dry-run` | hand-code | Bulk-import a collected_items list from CSV/JSONL with `--dry-run` cost forecast. Idempotent on (user_id, type_id, issue_id) tuple. | 8 |
| 11 | **Order/folder hydrate** (sync one collection folder) | `users collection hydrate collection-id` | hand-code | Given a collection-id, fan out get-item for every item in the folder, then optionally fan out get-prices. Refuses to start when remaining quota < item count. PCGS analog: `order hydrate`. | 7 |

**11 transcendence rows. Hand-code count: 10. Spec-emits count: 1 (sql, already shipped by base generator).**

Every transcendence feature passes the >=5/10 threshold and will appear in README/SKILL. The themes:

- **"Quota economics"** — 1, 2, 3 (`--quota`, `audit`, `types batch`)
- **"Local state that compounds"** — 4, 5, 6, 7, 8 (`series`, `collection value`, `refresh`, `issuers crawl`, `watchlist`)
- **"Agent-native plumbing"** — 9 (`sql`), 10 (`users items add --from-file`)
- **"Hydration"** — 11 (`users collection hydrate`)

## Hand-Code Commitment (Phase Gate 1.5 readout)
Of 11 transcendence features, **10 will require hand-written Go after `generate`**, each ~50-150 LoC + `root.go` wiring. The remaining 1 (`sql`) is generator-emitted from the base template. The 10 hand-code features by name:

1. `--quota` / `--quota-only` root flags + `quota.go` (PCGS adaptation, monthly)
2. `audit` (SQL views over lookup_log)
3. `types batch` (CSV/JSONL parse + --dry-run + --resumable)
4. `types series` (fan-out issues + prices for one type)
5. `collection value` (fan-out prices for collected_items; sum/delta)
6. `refresh` (selective field refresh + --older filter)
7. `issuers crawl` (issuer-scoped type crawl + cost forecast)
8. `watchlist add/list/check` (price-change tracker + snapshots)
9. `users items add --from-file` (CSV bulk import, idempotent)
10. `users collection hydrate` (sync one folder's items + optional price hydration)

## Stub items
**None.** Every transcendence feature is shipping-scope. The user explicitly excluded paid endpoints (`/search_by_image`) and special-permission endpoints (Catalogue Edition POSTs), so there are no "stub for paid" rows.

## Spec/Auth notes for generation
- Internal YAML or OpenAPI? The Numista spec is clean OpenAPI 3.0 — use directly with `printing-press generate --spec`.
- Auth enrichment: add `x-auth-env-vars: [NUMISTA_API_KEY]` on the ApiKeyAuth security scheme to lock in canonical env var name. (Slug-derived would be `NUMISTAAPIKEY_API_KEY` which is wrong.)
- OAuth bearer: spec already declares OAuth2 client_credentials and authorization_code flows. The auth login command flow handles this separately.
- Endpoint exclusions for generation:
  - POST `/search_by_image` — paid; remove from spec or annotate with `x-hidden: true`
  - POST `/types` and POST `/types/{type_id}/issues` — Catalogue Edition; remove from spec
  - All Deprecated tag operations — remove from spec
