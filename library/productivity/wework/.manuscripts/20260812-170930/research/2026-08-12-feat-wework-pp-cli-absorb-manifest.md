# WeWork Desk-Bookings CLI — Absorb Manifest

## Ecosystem scan result
- **No competing CLI, MCP server, or SDK exists** for WeWork WorkplaceOne desk booking. It is a proprietary members portal with no public API. (Searches for a WeWork CLI / MCP / npm wrapper return nothing usable.)
- **Incumbent = the WeWork web app itself** (`members.wework.com` "Book a desk"). The absorb target is the web app's read features; we match them, then beat them with offline persistence, agent-native output, and city-name-driven search that the map UI can't script.

## Absorbed (match or beat the web app's read surface)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List bookable cities | web app city dropdown | `wework-pp-cli cities` | Offline after sync, FTS by name/country, `--json`, exposes marketgeo lat/lng |
| 2 | Search desks by city + date | web "Book a desk" search | `wework-pp-cli desks --city <c> --date <d>` | Scriptable, `--json`/`--select`, credits+price+availability in one row, bounding box derived from city geo automatically |
| 3 | Browse buildings/locations in a city | web map + list | `wework-pp-cli locations --city <c>` | Availability count, hours, amenities as structured rows; offline store |
| 4 | Filter by capacity | web capacity filter | `(behavior in wework-pp-cli desks)` `--capacity` flag | Composable with other filters, works headless |
| 5 | Filter by location type | web "Location Type" filter | `(behavior in wework-pp-cli desks)` `--location-type` flag | — |
| 6 | Show desk price / credits | web desk card | `(behavior in wework-pp-cli desks)` price+credits columns | Both credits and currency amount surfaced; sortable |
| 7 | Show amenities per location | web amenities filter | `wework-pp-cli amenities` | Structured amenity list, offline |
| 8 | View my upcoming bookings | web "Bookings" tab | `wework-pp-cli bookings` | `--json`, verify-safe, no geo needed |
| 9 | Auth via WeWork login | web auth0 login | `wework-pp-cli auth login --chrome` / token import | Import bearer+uuid+member-type from Chrome session or env; local storage; `doctor` reports expiry |

## Transcendence (only possible with our local-store + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Offline city/location search (FTS) | `search <term>` | hand-code | Requires local SQLite FTS over synced cities+locations; web UI has no offline mode |
| 2 | Cheapest desk near a city | `desks --city <c> --sort credits` | hand-code | Requires ranking the full desk result set by credits/price locally; web UI only sorts visually |
| 3 | Availability-only filter | `desks --city <c> --available-only` | hand-code | Requires filtering on `seat.available`/`spaceAvailabilityCount` across the result set |
| 4 | City geo → bounding box search | (behavior powering `desks`/`locations`) | hand-code | Derives the bounds the API requires from a city name via cached marketgeo lat/lng — the web app only gets bounds from the live map viewport |
| 5 | Local SQL over synced data | `sql "<select>"` | spec-emits | Framework SQL command over the local store of cities/locations for arbitrary agent queries |
| 6 | Agent-native compact desk view | `desks --agent --select ...` | spec-emits | Bounded high-gravity fields (name, credits, available, address) for low-token agent consumption |

## Stubs / gaps to disclose
- **No booking mutation in v1.** `book` / `cancel` are intentionally NOT shipped (not even stubs) per approved read-first scope. The confirm POST was never triggered during capture to avoid placing a real reservation. Documented as a Known Gap; can be added later with explicit consent + a targeted capture of the confirm flow.

## Auth-expiry disclosure (must surface at gate)
- The CLI's bearer token is a short-lived auth0 JWT imported from the browser. Live commands work until it expires, then `auth login --chrome` (or re-export of `WEWORK_TOKEN`) is required. `doctor` will report token presence/expiry. Live smoke testing (Phase 5) is only possible while a fresh token is available and may be time-limited.

## Scope summary
- **~9 absorbed read features + 6 transcendence features.** Of the transcendence set, **4 are hand-code** (offline FTS search, credits-sort ranking, availability filter, city→bounds derivation) and 2 are framework/spec-emitted (sql, agent output). No mutation commands.
