# Airbnb CLI Brief

## API Identity
- Domain: Short-term rental marketplace. No public/official API — internal `/api/v3` persisted-query GraphQL + legacy `/api/v2` REST.
- Users: Guests (search, book, message hosts, wishlists, trips) and Hosts (listings, calendar, reservations, payouts). This account = guest ("Jim", isHomeHost=false).
- Data profile: Listings, search results, message threads/messages, wishlists, reservations/trips, price quotes, host profiles.

## Reachability Risk
- **Low (for JSON API), with one caveat.** stdlib + Surf-Chrome curl to `/api/v2` and `/api/v3` returns `application/json` (200/400), NOT DataDome challenge HTML. Public `StaysSearch` GET validated 200 w/ 12 results from plain fetch. DataDome/`datadome` cookie guards HTML pages, not the JSON API surface.
- **Caveat 1 — method:** persisted GraphQL queries MUST be **GET** (URL-encoded variables). POST triggers Airlock v2 (`401 {success,redirect}`). Mutations need POST → mutation transport (CSRF/headers) confirmed during build.
- **Caveat 2 — hash rotation:** persisted-query sha256 hashes change on Airbnb deploys. Mitigated by the self-heal harvester (hashes are extractable from served JS bundles).

## Top Workflows
1. **Search stays** by location + dates + guests + filters → list of listings (id, price, rating, host).  ← public headline
2. **Contact a host / property owner** — open/create a thread, send a message, **send images** (GetSignedUrls → CreateMediaItems → CreateBulkMessages).  ← USER'S PRIMARY BUSINESS CASE
3. **Inbox management** — list threads, read a thread, mark read, track replies.
4. **Listing detail + price quote** — full PDP sections, availability, checkout price breakdown (read-only quote).
5. **Wishlists & trips** — list saved listings and upcoming reservations.

## Data Layer
- Primary entities: listing, search_result, thread, message, wishlist, wishlist_item, trip/reservation, host_profile, contact (host contacted), price_quote.
- Sync cursor: inbox uses opaque base64 cursors (pageInfo.endCursor); search uses pageCursors[].
- FTS/search: local FTS over synced listings + threads + messages → offline search of past conversations & saved listings.

## User Vision
- "einfach alles" — full surface. Emphasis: **search + contact property owners incl. sending images** (outreach/business use). Writes allowed, but money/booking guarded (dry-run default + explicit --confirm; never silent charge).

## Auth
- Public reads: api key `d306zoyjsyarp7ifhu67rjxn52tv0t20` only.
- Authenticated: Chrome cookie import (`auth login --chrome`) — session cookies from logged-in browser. No password handling.
- Required headers on every call: x-airbnb-api-key, x-airbnb-graphql-platform=web, x-airbnb-graphql-platform-client=minimalist-niobe, x-airbnb-supports-airlock-v2=true.

## Product Thesis
- Name: **airbnb-pp-cli** ("nest") — the Airbnb CLI for power outreach + saved-data intelligence.
- Why it should exist: No CLI exists for Airbnb's real (authenticated) surface. This one turns Airbnb into a scriptable outreach + research tool: batch-search listings, bulk-contact hosts/owners with templated messages + photos, and keep a local searchable archive of every conversation and saved listing — none of which the website or any existing tool offers.

## Build Priorities
1. Client foundation: GET-only persisted-query transport, api key + Chrome cookie import, base64 ID codec, operation registry + `ops refresh` self-heal harvester, GET/POST mutation transport, SQLite store.
2. Read surface: search, listing detail, price quote, inbox list, thread read, wishlists, trips, host-status, me.
3. Write surface: contact host (new thread), send message, **send image(s)**, mark-read; booking price-quote read + guarded reserve (dry-run/confirm).
4. Transcendence: bulk-contact hosts from a search (templated), conversation archive + offline FTS, saved-listing price-drop watch, self-heal `ops refresh`, outreach CRM (who I contacted + reply status).
