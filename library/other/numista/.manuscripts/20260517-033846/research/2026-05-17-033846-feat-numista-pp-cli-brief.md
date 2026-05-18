# Numista CLI Brief

## API Identity
- **Domain:** Numismatics — coins, banknotes, exonumia (tokens, medals). Numista is the dominant community catalogue with ~530K type records and ~6M collected items spread across hobbyist collectors and dealers.
- **Users:** Coin collectors managing personal collections; numismatic researchers cross-referencing rulers/mints/years; dealers checking grade/price estimates before listings; cataloguers contributing data.
- **Data profile:** Heavy reference data (issuers, mints, catalogues, types, issues, prices) + per-user mutable collection data (collected_items, collections). Prices and mintage data carry the most value-decay; issuer/mint reference data is effectively static.
- **API:** OpenAPI 3.0 spec at `https://en.numista.com/api/doc/swagger.yaml`. Base URL `https://api.numista.com/v3`. Auth via `Numista-API-Key:` request header; user-scoped read/write needs OAuth 2.0 (authorization-code or client-credentials) with `view_collection` / `edit_collection` scopes.

## Reachability Risk
- **None.** Direct probe against `/v3/types/95420` with `NUMISTA_API_KEY` returns 200. Spec is publicly hosted and accessible (Cloudflare-fronted but no challenge, only UA-shape filtering — already handled).

## Free vs Paid (User Constraint: Exclude Paid)
- **Free (include — 17 endpoints):**
  - **Catalogue (read):** GET /types, /types/{id}, /types/{id}/issues, /types/{id}/issues/{issue_id}/prices, /issuers, /mints, /mints/{id}, /catalogues
  - **Literature:** GET /publications/{id}
  - **OAuth flow:** GET /oauth_token
  - **User (OAuth-gated, but the API access itself is free):** GET /users/{id}, GET/POST /users/{id}/collected_items, GET/PATCH/DELETE /users/{id}/collected_items/{item_id}, GET /users/{id}/collections
- **Paid (exclude):** POST /search_by_image — spec says "Search by image is a paid feature… minimum €100/month."
- **Special permission (exclude):** POST /types, POST /types/{id}/issues (Catalogue Edition tag) — even though not strictly paid, these need a special permission users will not have by default; ship-clean CLI excludes them.
- **Deprecated (exclude):** All 7 `/coins/*` endpoints + `/users/{id}/collected_coins` — superseded by `/types/*` and `/users/{id}/collected_items`.

## Quota Model (verbatim from pricing page)
- **Free plan: 2000 requests / month.** Hard cap, monthly UTC reset, server-side enforced; HTTP 429 on exhaustion ("You sent too many simultaneous requests or you reached the limit of your monthly quota"). No quota headers in responses — purely client-tracked.
- **Paid (search-by-image):** no hard quota with usage-balance constraint; ignored — we exclude this endpoint.
- **PCGS analog:** PCGS uses `lookup_log` table + 1000/day cap + UTC daily reset; we adapt to **monthly** reset (`called_at >= datetime('now','start of month')` equivalent via `strftime('%Y-%m', called_at) = strftime('%Y-%m','now','utc')`) and 2000 ceiling. The pattern (one row per live API call + 0-cost cache hits + `--quota` short-circuit + `--dry-run` cost forecast + audit subcommand) carries over directly.

## Top Workflows
1. **Identify a coin** — user has a coin in hand; searches by query (`q="Australia 3 pence George VI"`) or by issuer/year/material to narrow down to a type ID, then fetches details + issues + prices.
2. **Track a collection** — sync, browse, search, and update the user's personal collected_items list. Add new finds, update grades, mark for sale, set storage location.
3. **Value a collection** — for every collected_item, fetch its issue prices and sum to a total estimated value; flag items without recent price data.
4. **Series scan** — for one coin type (e.g., N#11013), pull every year-of-issue + prices into local store and visualize price/mintage trends across the full series.
5. **Cross-reference** — given an issuer (e.g., "Australia") or ruler (e.g., "George VI"), enumerate all types issued, optionally filtered by year range or material.
6. **Catalogue browsing** — explore reference catalogues (Krause, Bruce, etc.), publications, and mint metadata for research.

## Table Stakes (must match every competitor)
- All catalogue read endpoints — searchTypes, getType, getIssues, getPrices, getIssuers, getMints, getMint, getCatalogues, getPublication (namachieli, numistalib, leopiccionia all cover these)
- All collection management endpoints — getCollectedItems, getCollectedItem, addCollectedItem, editCollectedItem, deleteCollectedItem, getUserCollections, getUser (namachieli covers these with token labels; numistalib has them)
- OAuth helpers — token generation and refresh (namachieli has myToken/myTokenRefresh/myUserId/myTokenExp helpers)
- Grade validation — Numista uses 0–10 + half-grade modifiers; namachieli has `validateGrade`
- Schema introspection — namachieli's `schemaFind` / `schemaGenerateBody` (we expose the spec via `agent-context spec`)
- HTTP caching — numistalib's RFC 9111 caching (we use SQLite-backed lookup_log + dedicated `types`, `issues`, `prices`, `issuers`, `mints`, `catalogues` tables)

## Data Layer
- **Primary entities** (each gets its own typed table with FTS5 index on text fields):
  - `types` — N# id, title, issuer code, ruler, category (coin/banknote/exonumia), min_year, max_year, composition, mass_g, diameter_mm, obverse/reverse description, url
  - `issues` — id, type_id, year, mintage, mint, comment
  - `issuers` — code (e.g., "australia"), name, currency, parent_issuer
  - `mints` — id, name, country, latitude, longitude
  - `catalogues` — id, code (e.g., "krause"), title, author, publisher
  - `publications` — id, title, author, year
  - `prices` — type_id, issue_id, grade, price, currency, date_fetched (snapshot table; multiple snapshots per issue over time)
  - `collected_items` — user_id, item_id, type_id, issue_id, grade, comment, private_comment, public_collection, for_swap, for_sale, ask_price, location, acquired_at
  - `collections` — user_id, collection_id, name, type (folder)
- **Sync cursors:** no native cursors in API; manual driver — sync by user-owned type IDs, or "all types matching predicate X". Watchlist-style sync.
- **FTS:** `types_fts(title, obverse_description, reverse_description, ruler_name)`, `issuers_fts(name)`, `mints_fts(name, country)`, `catalogues_fts(title, author)`. SQL composability via `sql` subcommand.
- **Quota log:** `lookup_log(id, called_at, endpoint, method, request_hash, type_id, http_status, duration_ms, cache_hit)` — identical shape to PCGS, schema docs at `references/lookup-log-schema.md` in source.

## Codebase Intelligence
- **namachieli/numista-api-sdk** (Python, MIT, ~stars TBD): Full SDK with OAuth label-keyed token management (`myToken`, `myTokenRefresh`, `myUserId`), grade validation, schema-driven body generation. Confirms `/v3` base URL, `Numista-API-Key` header, OAuth bearer for user endpoints, JSON request/response.
- **numistalib** (Python, RFC 9111 HTTP caching with SQLite, exponential backoff via `tenacity`, rate-limiting via `pyrate-limiter`). Validates caching as a clear win for any quota-bound API.
- **@leopiccionia/numista-sdk** (npm, TypeScript): browser+server. Validates request/response shapes match across two language ecosystems.
- **MihajloNesic/Numista** (legacy Windows GUI, "unofficial" — pre-v3 API; not authoritative).

## User Vision (from briefing args)
> "auth key is $NUMISTA_API_KEY. 2K calls/month restriction for free plan. Take a look at pcgs-pp-cli for pattern on handling quota. Exclude endpoints that require paid access. Do a code review after you complete dogfood, scoring, and polish. Fix issues then run dogfood through polish to get ready for publishing."

- **Quota pattern:** Mirror `pcgs-pp-cli/internal/cliutil/quota.go` and `lookup_log` table; adapt to monthly reset + 2000 ceiling
- **Paid exclusion:** `/search_by_image` not emitted. `/types` POST and `/types/{id}/issues` POST also excluded (special permission).
- **Workflow:** Phase 4.95 native code review runs after Phase 4 shipcheck per skill default; user explicitly opted into it. Polish runs after that.

## Product Thesis
- **Name:** `numista-pp-cli` (binary), Numista PP CLI (display)
- **Why it should exist:** No CLI or MCP server exists for Numista today; SDKs (namachieli, numistalib, leopiccionia) require Python/JS toolchains and don't expose a command-line surface or local searchable store. A Go single-binary CLI with offline SQLite cache and monthly-quota-aware orchestration covers every workflow above with one install and never exceeds the 2K/month free-plan ceiling silently.
- **Differentiators no one else has:**
  - `--quota` short-circuit + monthly reset timer (Numista exposes no quota endpoint)
  - Offline FTS5 search over 530K-type catalogue once synced (vs every-search-burns-a-call)
  - `coin batch` style cost-forecast against current month's remaining quota before spending a call
  - `series` command pulling every issue + every grade's price for one type in one call-budget-aware command
  - Quota-aware `collection value` that refuses to start if remaining quota < collection size
  - SQL composability via local store — answer "all my Australian coins from 1940–1950" with zero API calls

## Build Priorities
1. **Foundation (P0):** Data layer with typed tables + FTS5 + lookup_log + monthly quota subsystem + sync command. Without this nothing transcendent works.
2. **Absorb (P1):** All 17 free endpoints exposed as Cobra commands (read + write where applicable). Match namachieli's full surface area. OAuth flow for user-scoped operations.
3. **Transcend (P2):** quota-aware compound commands (see Phase 1.5 manifest).
4. **Polish (P3):** README rewrite focused on numismatic vocabulary; rich examples; SKILL.md tuned to "identify a coin," "value a collection," "browse a series."
