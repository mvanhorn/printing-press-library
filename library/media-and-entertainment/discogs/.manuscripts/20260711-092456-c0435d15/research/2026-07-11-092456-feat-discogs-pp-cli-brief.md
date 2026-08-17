# Discogs CLI Brief

## API Identity
- **Domain:** Discogs — the largest crowd-sourced music (vinyl/CD) database + a marketplace + per-user collection & wantlist. API v2 at `https://api.discogs.com`.
- **Users:** vinyl collectors, record sellers/flippers, DJs, catalogers. Power users track collection value, hunt marketplace deals, and manage listings.
- **Data profile:** releases, masters, artists, labels (database); per-user collection (folders, instances, custom fields, value), wantlist, marketplace inventory/listings/orders, price suggestions, marketplace stats, user lists, identity/profile. Read-heavy with meaningful write surface (collection add/remove/rate, wantlist add/remove, listing CRUD, order updates).

## Reachability Risk
- **LOW.** API is reliably reachable with a personal access token. Probe-confirmed 2026-07-11: `GET /releases/{id}` returns JSON; `x-discogs-ratelimit: 25` unauth / 60 auth.
- **Two hard requirements (both confirmed):**
  1. **Mandatory descriptive `User-Agent`** — empty/generic/missing UA → hard **HTTP 403**. This is the #1 avoidable failure. Client MUST send `discogs-pp-cli/<version>`.
  2. **Rate limiting** — 60/min authenticated, 25/min unauth, 60s moving average via `X-Discogs-Ratelimit[-Used/-Remaining]`. Pulling price suggestions across a whole collection routinely hits the ceiling. Client must respect the remaining header and back off.
- Cloudflare occasionally throws false-positive 403 bursts (resolve on Discogs' side). Image asset URLs are Cloudflare-bot-gated — do not rely on server-side image fetch.
- No mutation probing beyond documented GETs.

## Top Workflows
1. **Search the database** (release/master/artist/label) with rich filters, then drill into a release. (The single most-used path; `agent-discogs` is built entirely around this.)
2. **Manage a collection** — list by folder, add/remove releases, see collection value.
3. **Hunt marketplace deals** — check lowest asking (`stats`) and condition-matched value (`price_suggestions`) for a release; watch wantlist items for listings at/below a target price.
4. **Sell** — list inventory, manage listings, track orders, compute fees.
5. **Value & track over time** — collection valuation with cost basis, price trend, undervalued detection.

## Table Stakes (from cswkim/discogs-mcp-server = full-surface incumbent + python3-discogs-client)
- Database: search, release, master, master versions, artist, artist releases, label, label releases.
- Ratings: community rating, per-user rating get/set/delete.
- Collection: folders CRUD, items by folder, add/remove instance, rate instance, custom fields, collection value, find release in collection.
- Wantlist: list, add, remove, edit note/rating.
- Marketplace: inventory, listing get/create/update/delete, orders list/get/update, order messages, **fee**, **price_suggestions**, **marketplace stats**.
- Identity: identity, profile get/edit, submissions, contributions.
- Lists: user lists, list detail.
- Inventory export: request export, list exports, get export, download export.
- Media: fetch image (CF-gated — best-effort).

## Data Layer (local SQLite mirror — the differentiator substrate)
- **Primary entities:** releases, masters, artists, labels, collection_items, collection_folders, collection_fields, wantlist_items (+ local `max_price`), marketplace_listings, orders, lists.
- **Keystone table: `price_snapshots`** — (release_id, captured_at, lowest_price, currency, condition, suggestion_by_condition, num_for_sale). The Discogs API keeps **NO price history** (confirmed collector complaint); the CLI persisting snapshots over time is what unlocks undervalued detection, portfolio history, and comps. This is the moat.
- **Sync cursor:** collection/wantlist by `date_added`; price snapshots by capture timestamp.
- **FTS:** offline full-text over release/artist/label titles + synced collection.

## Codebase Intelligence (from ecosystem research)
- **Auth:** personal access token. Canonical header `Authorization: Discogs token=<TOKEN>` (custom scheme prefix — model via `auth.format: "Discogs token={token}"`). Discogs also accepts `?token=<TOKEN>` query param. Env var `DISCOGS_TOKEN`. OAuth 1.0a exists for multi-user apps — OUT OF SCOPE.
- **Rate limiting:** `X-Discogs-Ratelimit`, `-Used`, `-Remaining`; 60s window.
- **Reference implementations:** `irlndts/go-discogs` (Go client, 52⭐) for HTTP/parse patterns; `cswkim/discogs-mcp-server` TOOLS.md for the exhaustive tool surface; `python3-discogs-client` (409⭐) for full method coverage incl. price suggestions.

## User Vision (from prepared briefing prompt — 2026-07-11-discogs-pp-cli-briefing-prompt.md)
Standalone agent-native Discogs API CLI (local SQLite mirror + MCP server), NOT a wrapper on the existing Railway web app. Discogs-side discovery + valuation layer for a vinyl collector; pairs loosely (agent-as-glue) with an eBay CLI + a Gixen sniping CLI but contains no eBay/Gixen logic. **Flagship novel feature: wantlist-as-limit-order-book** — each wantlist entry carries a local `max_price`; watch marketplace lowest-asking/stats against each threshold; surface "fills" with a diff since last check. Personal token only, no OAuth. Transcendence set: limit-order book, undervalued detection, portfolio value+history, condition-matched comps, fee-aware sell router, catalog-number/barcode identity spine, liquidity score, what-changed watch. Out of scope: OAuth 1.0a, eBay/Gixen logic, writes to other apps' DBs.

## Product Thesis
- **Name:** discogs-pp-cli ("Discogs")
- **Why it should exist:** No existing tool combines (a) full Discogs API breadth, (b) agent-native ergonomics (`--json`, `--select`, typed exits, MCP), and (c) a single static Go binary with an offline SQLite mirror. `agent-discogs` has the ergonomics but is read-only (no account features). `cswkim` has the breadth but needs a Node runtime + MCP host and **lacks price suggestions**. `DiscoDOS` is a heavyweight Python collector tool. And crucially, **the API keeps no price history** — so a CLI that snapshots prices locally unlocks undervalued detection, portfolio trend, and condition-matched comps that no incumbent offers, plus the wantlist-as-limit-order-book bridge (only `discogs-alert` is adjacent, and it's a standalone price-drop alert daemon).

## Build Priorities
1. **Foundation:** SQLite mirror for all primary entities + `price_snapshots`; sync/search/SQL; **descriptive User-Agent (non-negotiable)**; rate-limit-header-aware backoff + caching.
2. **Parity (absorb everything):** the full cswkim/python3-discogs-client surface — database, collection, wantlist, marketplace, orders, fee, price_suggestions, stats, identity, lists, inventory export. Every command agent-native, `--dry-run` on writes.
3. **Transcendence (the moat):** wantlist-as-limit-order-book (flagship), undervalued detection, portfolio value+history, condition-matched comps, fee-aware sell router, catalog-number/barcode identity spine, liquidity score, what-changed watch.

## Spec Source Decision
No trustworthy OpenAPI exists (community `wyattowalsh/discogs-api-spec` is unvetted LLM-generated). **Hand-author an internal YAML spec** grounded in the Discogs Postman collection (`leopuleo/Discogs-Postman`, authoritative endpoint inventory — fetched) + cswkim TOOLS.md + official docs. Auth: `api_key` with `format: "Discogs token={token}"`, `DISCOGS_TOKEN`. `required_headers` sets the mandatory User-Agent.
