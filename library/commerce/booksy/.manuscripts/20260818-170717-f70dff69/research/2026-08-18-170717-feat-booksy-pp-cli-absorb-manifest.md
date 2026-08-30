# Booksy CLI — Absorb Manifest

## Landscape
No consumer Booksy CLI exists. The only programmatic Booksy access is third-party **scrapers/wrappers** (Apify "Booksy Scraper", Parse.bot, Supergood) that expose read-only business/availability data — none book, none are agent-native, none run locally. Booksy's own value is the app's booking funnel. So this CLI's edge is: **be the first agent-native Booksy booking CLI**, matching the app's read surface and adding a safe, scriptable booking path plus local comparison intelligence.

Discovery: authenticated DevTools HAR from the logged-in pl.booksy.com session (66 real API calls). Base `https://pl.booksy.com/core/v2/customer_api/`. Transport `standard_http`.

## Absorbed (match the app + every scraper wrapper)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search businesses by query/city/category | Booksy app; Apify scraper | (generated endpoint) businesses search | --json/--select/--csv, offline cache, sort flags |
| 2 | Business profile (services, staff, hours) | Booksy app; Parse.bot business info | (generated endpoint) businesses get | typed, agent-native, cached |
| 3 | Reviews | Booksy app | (generated endpoint) businesses reviews | paginated, --json |
| 4 | Query suggestions | Booksy app search | (generated endpoint) discover suggest | scriptable autocomplete |
| 5 | Location resolution (city -> id) | Booksy app search | (generated endpoint) discover locations | needed to scope search |
| 6 | Popular treatments | Booksy app | (generated endpoint) discover treatments | seed a search |
| 7 | Own profile | Booksy app account | (generated endpoint) me profile | token-gated |
| 8 | Own home / appointments / favorites | Booksy app "my booksy" | (generated endpoint) me home | booking_box + favorites_visited |
| 9 | Availability / open time slots | Booksy app calendar; Parse.bot slots | booksy-pp-cli availability | clean flags over the nested time_slots POST |
| 10 | Book an appointment | Booksy app checkout | booksy-pp-cli book | dry-run-by-default + --confirm + harness refusal |

## Transcendence (only possible with our local + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Guided haircut booking | book | hand-code | Composes business+variant+staffer+slot+dry_run+confirm into one safe step the app never exposes | Use to actually book. Dry-run previews; --confirm commits. Refuses under any test harness. |
| 2 | Open slots | availability | hand-code | Wraps the nested subbookings time_slots POST as simple flags | Use to answer "when can I get in?"; needs BOOKSY_ACCESS_TOKEN. |
| 3 | Services table | services | hand-code | Flattens 3-level nested service_categories->services->variants to expose the bookable variant id + price | Use to pick the --service-variant id for availability/book. |
| 4 | Soonest slot | earliest | hand-code | Scans day-grouped time_slots and returns the first open slot | Use for "soonest haircut here?"; do NOT use for a full calendar — use availability. |
| 5 | Compare barbers | compare | hand-code | Holds multiple business profiles together locally; API returns one at a time | Use to compare rating + cheapest matching service across businesses. |
| 6 | Cheapest haircut nearby | cheapest | hand-code | Joins search results with per-business service prices; Booksy never sorts by service price | Use for "cheapest decent haircut near me"; a query Booksy's UI cannot express. |

**Hand-code count: 6** (book, availability, services, earliest, compare, cheapest). All scored ≥5/10 on customer value. Auto-emitted read commands: 8 (from spec). Booking-safety: `book` never commits during verification/dogfood.

## Stubs
None. Every listed feature ships fully. `book --confirm` is real but is only exercised interactively by the user, never by automated tests.
