---
api_name: table-reservation-goat
sources: [opentable, tock]
primary: opentable
generated_at: 2026-05-09
---

# Absorb Manifest — Table-Reservation-GOAT

This manifest enumerates every reservation feature found across competing tools and how this CLI matches OR beats each. After absorption, the GOAT extends with 9 transcendence features (bottom table) made possible only by the local SQLite store + cross-network joins + agent-native plumbing.

**Sources tallied (12 wrappers + 1 reverse-engineered Swagger):** 21Bruce/resolved-bot (Go), xunhuang/yumyum-v2 (TS), whoislewys/opentable-reservation-bot (TS+Puppeteer), spudtrooper/opentable (Go), singlepatient/tablehog (Rust), Henrymarks1/Open-Table-Bot (Python), jrklein343-svg/restaurant-mcp (TS, OT+Resy MCP, 12 tools), chrischall/opentable-mcp (browser-extension capture), duaragha/opentable-mcp (Playwright MCP), bedheadprogrammer/reservationserver (Playwright MCP), azoff/tockstalk (Cypress sniper), michel-adelino/scrapping (Selenium Tock), Tock /api/docs/latest reservation Swagger (50+ data-model fields).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---|---|---|---|---|
| 1 | Restaurant search by name (autocomplete) | OT Autocomplete GraphQL (5 wrappers) | Surf+CSRF GraphQL with bootstrapped persisted-query hashes; FTS5 mirror in local SQLite | Offline regex search, --json/--csv/--select, agent-native | shipping |
| 2 | Restaurant search by location (lat/lng, metro, neighborhood) | OT /s SSR + multiSearch state | Surf-fetch /s and parse `__INITIAL_STATE__.multiSearch` | --json structured output, dotted --select | shipping |
| 3 | Browse metro/region/neighborhood pages | OT /metro/<slug>-restaurants | Surf-fetch + SSR extraction; cross-network: include Tock metro pages | Cross-network metro listings | shipping |
| 4 | Restaurant detail (hours, cuisine, price, address, phone) | OT /r/<slug> SSR (`restaurantProfile.restaurant` 30 fields); Tock /<venue> SSR + /api/business/<int> REST | Surf-fetch + state extraction; cache to local store | Network-agnostic queries: `pp restaurants get` works for both | shipping |
| 5 | Restaurant photos | OT `restaurantProfile.gallery`; Tock business response | Cache photo URLs; --json includes them | Single command across both networks | shipping |
| 6 | Restaurant reviews & ratings | OT `mostRecentReview`, `topReviews`; Tock accolades | Sync to local store | Offline review search via FTS | shipping |
| 7 | Restaurant menus | OT `restaurantProfile.menus`; Tock `business.menuUrl` | Sync menu metadata | Cross-network menu listing | shipping |
| 8 | Awards / accolades (Michelin, World's 50 Best) | OT `editorialLists`; Tock `accolades` | Local store + queryable | `pp restaurants list --michelin >= 2` | shipping |
| 9 | Single-restaurant availability check | OT `RestaurantsAvailability`; Tock SSR `calendar.offerings` | Surf-fetch + GraphQL or SSR extract | --json with slot tokens, attributes | shipping |
| 10 | Multi-restaurant availability comparison | OT `RestaurantsAvailability` with `restaurantIds:[]` | Single call with array; cross-network parallel for Tock | Cross-network compare | shipping |
| 11 | Multi-day availability (single restaurant) | OT `multiDayAvailabilityModal` slice | Compose 7 daily calls (or single GraphQL) | --json calendar grid for agents | shipping |
| 12 | Experience / prepaid availability | OT `ExperienceAvailability`; Tock SSR experience offerings | GraphQL + SSR | Unified across both networks | shipping |
| 13 | Slot attributes (patio / bar / highTop / standard) | OT slot.attributes; Tock seatingArea | Pass-through | --filter attributes=patio | shipping |
| 14 | Make standard reservation | OT `/dapi/booking/make-reservation` | Real POST with slot tokens; --dry-run prints request; --launch opt-in | Verifier-safe; idempotent body | shipping |
| 15 | Make experience / prepaid reservation | OT `BookDetailsExperienceSlotLock`; Tock checkout flow | Slot lock then book; --dry-run, --launch opt-in | Cross-network booking | shipping |
| 16 | Modify reservation | OT `make-reservation` with `isModify=true` | Same endpoint, modify path | --dry-run preview | shipping |
| 17 | Cancel reservation | Community-documented cancel endpoint | REST POST + --confirm gate | Verifier-safe | shipping |
| 18 | List upcoming reservations | OT `UserNotifications`+my-reservations; Tock /api/patron + /history | Cross-network local query | `pp reservations list --upcoming --json` | shipping |
| 19 | List past reservations | OT past-reservations; Tock `pastPurchase` slice | Local store sync | Aggregations + history search | shipping |
| 20 | View reservation detail | UserNotifications detail GraphQL; Tock /history detail | Local store + live fetch | --json with all fields | shipping |
| 21 | Cancellation watch (per-network) | restaurant-mcp `snipe_reservation`, Henrymarks1 bot, azoff tockstalk | Local SQLite-backed watcher; cron + adaptive polling per source | Cross-network watch in one command (NEW vs. wrappers) | shipping |
| 22 | List active watches | restaurant-mcp `list_snipes` | Local store query | --json, --select | shipping |
| 23 | Cancel a watch | restaurant-mcp `cancel_snipe` | Local store delete | --dry-run | shipping |
| 24 | Notify on slot found | restaurant-mcp Slack webhook; Henrymarks1 console | Local notification + optional webhook | --slack-webhook, --pushover, --webhook | shipping |
| 25 | Saved / wishlisted restaurants | OT `UserWishlist` GraphQL | Authenticated GraphQL + local mirror | `pp wishlist list --json`, search wishlisted | shipping |
| 26 | Add / remove from wishlist | OT `UserWishlist` mutation | GraphQL mutation | --dry-run | shipping |
| 27 | View profile (email, phone) | OT `HeaderUserProfile`; Tock /api/patron | Authenticated REST/GraphQL | --json output | shipping |
| 28 | View loyalty / points balance | OT `pointsForDiscount`, `pointsRedemptions`; Tock `loyaltyProgram` | Authenticated + local cache | Trend tracking via local store snapshots | shipping |
| 29 | Available redemption options | OT `pointRedemptionRewards` | Authenticated GraphQL | --json discount catalog | shipping |
| 30 | Payment methods (read-only) | OT `payment`, `paymentProfile`; Tock `paymentCard` | Authenticated read | --json, never auto-modify | shipping |
| 31 | Gift cards balance | Tock `tockGiftCard`, `giftCard`; OT `wallet` | Authenticated read | Cross-network gift-card tracking | shipping |
| 32 | View tonight's options | restaurant-mcp tonight (implicit) | Composed search "today/now/+30min" | `pp tonight 4` | shipping (subsumed by `goat --when tonight`) |
| 33 | Best Restaurants by category | OT `bestRestaurantsByCategory` | SSR extraction + local store | `pp top --category italian` | shipping |
| 34 | Editorial lists / curated | OT `editorialContent`, `editorialLists` | SSR + local | `pp lists` | shipping |
| 35 | Concierge / AI suggestions | OT `conciergeAi`, `conciergeAiV3` | Authenticated GraphQL | Pass-through; --json | shipping |
| 36 | Private dining / events | OT `privateDining`, `privateDiningForm` | Authenticated read | --json space details | shipping |
| 37 | Walkin / waitlist | Tock `walkinWaitlist` slice | Authenticated read | `pp waitlist join` | shipping |
| 38 | Delivery / pickup options | Tock `delivery` slice | SSR extraction + REST | `pp delivery list` | shipping |
| 39 | Restaurant memberships / VIP | OT `restaurantMemberships`, `restaurantVip` | Authenticated read | --json | shipping |
| 40 | Geo metadata (timezone, coordinates) | Both networks restaurant detail | Local store enrichment | Used by `pp goat` for ranking | shipping |
| 41 | Cuisines list (taxonomies) | OT `facets`; Tock `business.cuisine` | Local store enum | `pp cuisines list` | shipping |
| 42 | Pricing band (per-restaurant + per-experience) | OT `priceBand`; Tock `price.partyRangeConfigs` | Pass-through | `pp restaurants list --max-price 4` | shipping |
| 43 | Hours of operation | OT `hoursOfOperation`; Tock `business.hoursOfOperation` | Local store | `pp hours alinea` | shipping |
| 44 | Dress code / parking / payment options | OT `restaurantProfile.restaurant` 30 fields | Local store | `pp restaurants get --select dressCode,parkingInfo` | shipping |
| 45 | Local SQLite store + sync + FTS5 | NEW (no wrapper has this) | Built-in | --offline search; cross-network joins; SQL composability | shipping |
| 46 | --json / --select / --csv / --quiet on every command | NEW (no wrapper agent-native uniformly) | Built-in via `cliutil` | --json structured output everywhere | shipping |
| 47 | Auth login --chrome (cookie import) | NEW (wrappers require manual cookie copy) | Built-in via Chrome profile parsing | Zero-friction auth | shipping |
| 48 | Doctor / health check | Standard | Generic + per-source | --json status | shipping |
| 49 | Per-source rate limiting | NEW | `cliutil.AdaptiveLimiter` for both opentable + tock | Surfaces `*cliutil.RateLimitError` instead of silent empty | shipping |

**Stub items:** none. All 49 absorbed features ship as full implementations. Authentication-gated commands degrade to clear "not authenticated" errors when no Chrome cookie session is imported (instead of stubbing).

## Transcendence (only possible with our approach)

These are the 9 GOAT features. They are NOT thin renames of API endpoints — every one requires either local SQLite, a cross-network join, agent-shaped output, or a service-specific content pattern (or several).

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|-------------------------|
| T1 | Cross-network unified search-and-rank | `goat <query> [--when ...] [--party N] [--neighborhood ...]` | **10/10** | Marcus | Single command hits OpenTable + Tock concurrently and merges via local `restaurants` + `availability_slots`. No wrapper covers both networks. Ranking fuses relevance × earliest-slot × price band — a join across two synced tables. |
| T2 | Local cross-network cancellation watcher | `watch add/list/cancel/tick` | **10/10** | Daniela | Local SQLite `watches` table; `tick` polls both networks per `cliutil.AdaptiveLimiter`; persists hits; fires local notification + webhook. Resy's Notify works on Resy only; tockstalk works on Tock only; restaurant-mcp's snipe works on Resy+OT only. None covers both, none persists state, none does cancellation across two networks. |
| T3 | Multi-venue earliest-available | `earliest <slug1>,<slug2>,... --party N --within Nd` | **9/10** | Aaron, Marcus | Compose-and-rank across N parallel availability calls (OT GraphQL) + N SSR fetches (Tock); soonest slot per venue across both networks. No single API call answers "of these 5 venues across both networks, which has the soonest table?" |
| T4 | Trip-planner availability matrix | `matrix <slug1>,<slug2>,... --nights <date-range> --party N` | **8/10** | Aaron | (venues × nights × 2 networks) availability grid; CSV/JSON output; agent-pipeable. Multi-day modal in OT is single-restaurant; matrix is N×M×2. |
| T5 | Cross-network dining-history insights | `insights [--year YYYY] [--by cuisine\|neighborhood\|venue]` | **8/10** | Priya | Local GROUP BY across merged `reservations` (OT `UserNotifications` + past + Tock `pastPurchase`+`history`). Two networks × multi-year history merged into one rollup. Neither network shows the other's history. |
| T6 | New-experiences drop feed | `experiences drops --since 7d --within 30d [--cuisine ...]` | **9/10** | Priya, Daniela | Diffs latest synced `experiences` snapshot against `--since`; emits new arrivals with availability join. Tock-specific content type (PRIX_FIXE / EXPERIENCE / PACKAGE) + OT `ExperienceAvailability`; corpus-wide drop surfacing across both is impossible without the local store. |
| T7 | Per-restaurant change feed | `drift <restaurant> [--since <ts>]` | **8/10** | Daniela | Snapshot diff of one venue's `availability_slots` + `experiences` + restaurant fields between prior and current sync. Hot-target deep-watch; surfaces price moves, new experiences, hours changes. |
| T8 | My-saved-list × live availability | `wishlist availability [--party N] [--within Nd]` | **9/10** | Daniela, Priya | Joins merged wishlist (OT `UserWishlist` + Tock saved venues) against live availability calls; emits venues from MY list with ≥1 matching slot. No wrapper does "what on my saved list opened up?" — and no single network even sees the other's wishlist. |
| T9 | Unified loyalty-and-credits status | `loyalty status` | **7/10** | Priya | Authenticated read of OT `pointsForDiscount` + `pointsRedemptions` + Tock `loyaltyProgram` + `giftCard`; merged with computed expiry buckets. Two separate authenticated APIs become one rollup. |

## Auth scoping (free vs paid)

| Tier | Source | Auth required | Commands |
|---|---|---|---|
| free / anonymous | OpenTable consumer | CSRF token only (extracted at runtime) | search, autocomplete, restaurant detail, availability read, metro listings |
| free / anonymous | Tock consumer | None (Surf clears Cloudflare) | venue detail, search, experience listings (anonymous SSR) |
| free / authenticated | OpenTable | Cookie session (chrome import) | book, modify, cancel, my-reservations, wishlist, points, payment-methods (read), gift-cards |
| free / authenticated | Tock | Cookie session (chrome import) | book, my-reservations, walkin/waitlist, loyalty, gift-cards |
| paid / partner | OpenTable Booking API | OAuth2 client_credentials (rare) | OUT OF SCOPE — not used |

Both sources are free at the user level; no paid keys required. The `auth login --chrome` flow imports the user's Chrome cookies for both sites in one step.

## Build priorities (mirrors brief Build Priorities)

1. Surf-based clients: `internal/source/opentable/` (GraphQL + REST + SSR) and `internal/source/tock/` (REST + SSR)
2. Local store: `restaurants`, `availability_slots`, `experiences`, `reservations`, `watches`, `wishlist`, `loyalty_snapshots`, `restaurants_fts`
3. Anonymous read commands first (rows 1-13, 32-44, T1, T3, T4)
4. Authenticated commands behind `auth login --chrome` (rows 14-31, T2, T5, T8, T9)
5. Transcendence (T1-T9) — built explicitly in Phase 3, not the generator
6. Per-source rate limiting via `cliutil.AdaptiveLimiter` for both clients
