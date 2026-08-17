# Vagaro CLI Absorb Manifest

**Landscape:** First-of-kind — no consumer CLI or public consumer API exists for Vagaro, Fresha, Booksy, or StyleSeat. Apify scrapers confirm the search surface is scrapable at scale. We absorb the full public discovery surface and transcend with cross-business availability + comparison features the website structurally refuses.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search businesses by service + location | vagaro.com /listings SSR + Apify scrapers | `vagaro-pp-cli search <service> <city--state>` | Offline, `--json`, ranked by rating/price, SQL-composable |
| 2 | Business profile detail | vagaro.com /{slug} SSR | `vagaro-pp-cli business get <slug>` | Structured fields (BusinessID, rating, address, phone), offline cache |
| 3 | Service menu for a business | vagaro.com /{slug}/services SSR | `vagaro-pp-cli business services <slug>` | Prices/durations/descriptions as JSON |
| 4 | Read reviews for a business | vagaro.com /{slug} SSR | `vagaro-pp-cli business reviews <slug>` | Sortable, offline |
| 5 | Availability summary for a business | vagaro.com /{slug}/book-now SSR | `vagaro-pp-cli business availability <slug>` | Structured next-available |
| 5b | **Open slots for a service+provider in a window** (PRIMITIVE) | api.vagaro.com booking availability | `vagaro-pp-cli slots <slug> --service <id> [--provider <id>] --from <day> --to <day>` | Per-provider open times in a date window; reused by find/watch/rebook |
| 4b | Staff / providers for a business | vagaro.com /{slug} Staff tab SSR | `vagaro-pp-cli business staff <slug>` | Provider list (booking is per-provider) |
| 6 | Browse deals | vagaro.com /deals/{city--state} SSR | `vagaro-pp-cli deals <city--state>` | Offline, filterable |
| 7 | Browse professionals directory | vagaro.com /professionals/{city--state} SSR | `vagaro-pp-cli professionals <city--state>` | Structured |
| 8 | Upcoming livestream classes | POST /websiteapi/homepage/getupcominglivestreamclasses | `(generated endpoint) classes list` | `--json`, offline cache, OData dates parsed |
| 9 | Local SQL + FTS over synced data | (framework) | `(behavior in vagaro-pp-cli sql)` / `(behavior in vagaro-pp-cli search)` | Cross-entity offline query |
| 10 | (Auth) My appointments | vagaro.com account (cookie) | `vagaro-pp-cli me appointments` | Structured own-bookings read (auth login --chrome) |
| 11 | (Auth) My profile (read-only view) | vagaro.com account (cookie) | `vagaro-pp-cli me profile` | Structured own-profile read; NO editing |
| 12 | (Auth) **Book an appointment** (TABLESTAKES) | vagaro.com booking flow (cookie) | `vagaro-pp-cli book <slug> --service <id> --provider <id> --at <datetime> --confirm` | Real gated mutation: prints "would book" by default, `--confirm` to place, verify short-circuit |
| 13 | (Auth) My past appointment history | api.vagaro.com/us02/api/v2/myaccount/purchases/appointments (pastAppointment:true) | `vagaro-pp-cli me appointments --past` | Where you've actually been (business+service+provider+date) — powers rebook |
| 14 | (Auth) My favorites / bookmarks | api.vagaro.com/us02/api/v2/myaccount (bookmarks) | `vagaro-pp-cli favorites` | Saved businesses (complementary to history) |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Cross-business availability search | `find` | hand-code | 10 | Joins synced businesses+services+availability in SQLite; the website's per-business funnel cannot answer a marketplace-wide slot query | Use to discover which nearby businesses have a service open soonest under filters. Do NOT use for one known business's slot (use 'watch') or to line up named businesses (use 'compare'). |
| 2 | Business comparison | `compare` | hand-code | 8 | Joins businesses+services+reviews across named slugs; no site view offers this | Use for a side-by-side of businesses you already have in mind. Do NOT use it to discover candidates (use 'find') or for metro price spread (use 'price-check'). |
| 3 | Service price distribution | `price-check` | hand-code | 8 | Min/median/max/count over services rows across a metro; no fair-price view exists | Use to judge whether a quoted price is fair and who's cheap. Do NOT use it to compare specific named businesses (use 'compare'). |
| 4 | Market overview | `market` | hand-code | 7 | Aggregates local store into counts, rating distribution, per-category price ranges | Use for a one-shot metro landscape. Do NOT use it to act on one service query (use 'find' or 'price-check'). |
| 5 | Own-bookings rebook (history-driven) | `me rebook` | hand-code | 8 | Cross-source join: reads your PAST appointment history (business+service+provider), matches current availability at that business, prepares/places the rebook. Keyed off history, not favorites | Reads your past appointments to re-run your usual (`--last` or `<appointment-id>`); `--confirm` places it. Do NOT use it to find new providers (use 'find') or browse saved businesses (use 'favorites'). |
| 6 | Service-menu diff over time | `menu-diff` | hand-code | 6 | Diffs two timestamped service-menu snapshots in SQLite for price changes/added/removed | Use to detect price changes at a specific business over time. Do NOT use it for a live current menu (use 'business services'). |
| 7 | Availability watch (single poll) | `watch` | hand-code | 6 | Fetches live /book-now once, diffs vs stored baseline for slot movement | Single poll, not a background monitor. Use for one known business; use 'find' to scan the market. |

**Hand-code count: 7** transcendence features (all `hand-code`). `find` absorbs `--radius`, `--by`, `--before`, `--max-price`, `--min-rating` flags; `classes` absorbs `--category`/`--before`/`--max-price` filters.

## Notes / risks
- **Gate decision (user, 2026-07-01):** Approved full manifest + auth. Booking promoted to **tablestakes** (#12). Out of scope: payment-method management, profile editing. `me profile` is read-only view.
- **Auth (#10, #11, #12, transcendence #5 `me rebook`)**: cookie-based `auth login --chrome`. Authenticated + booking-flow endpoints captured via the user's live Vagaro session (walk booking flow up to — not through — final confirm).
- **Booking side-effect contract**: `book` is a real authenticated mutation. Prints "would book" by default; requires `--confirm` to place; short-circuits under `PRINTING_PRESS_VERIFY=1`; transport-layer gate no-ops the mutating verb under verify. Cannot fully live-test (creating a real appointment is an irreversible side effect) — verified via dry-run + captured request-shape fidelity. Businesses requiring prepayment/deposit are surfaced as "requires prepayment — complete at <book-now URL>" rather than charging a card (payment is out of scope).
- **Provider dimension (user design input):** Booking = business × service × **provider** × datetime; availability is per-provider. `slots`, `book`, `rebook` carry `--provider` (rebook defaults to your prior provider; `find`/`search` default to any provider).
- **Geo scoping (user design input + finding):** `search <service>` scopes to the user's location via their IP (works out-of-box for end users on their own machine). Explicit `search <service> --city <city>`/`--zip` uses the geocoded search API. NOTE: from this generation env (SF egress IP) all SSR listings default to SF — a generation-env artifact, not a shipped-CLI flaw; end users get their own metro. The explicit-city search API endpoint + the availability/booking-submit endpoints are captured-in-progress and finalized during Phase 3 build (browser session reusable).
- No stubs planned. Every absorbed + transcendence row calls a real vagaro.com URL or reads the local store.
