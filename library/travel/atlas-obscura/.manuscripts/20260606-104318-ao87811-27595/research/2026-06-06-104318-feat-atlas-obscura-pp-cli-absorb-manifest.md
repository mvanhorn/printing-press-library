# Atlas Obscura CLI — Absorb Manifest

CLI: `atlas-obscura-pp-cli` | Data labeled **community-sourced, not an official API**.
Every competitor is a thin LIVE scraper with no local cache. Our differentiator: a local
**SQLite** mirror (offline, joinable, persistent), road-trip **corridor routing**, and
agent-native output. All four headline commands + transcendence are hand-coded (AO has no
clean spec); the generator provides the scaffold, SQLite store, output helpers, MCP mirror,
and framework commands (sync/sql/search/doctor).

## Absorbed (match or beat every competitor)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search | atlas-obscura-api, Apify `search` | `atlas-obscura-pp-cli search <q>` | `kind=keyword` relevance, pagination, caches to SQLite, `--json/--select`, offline re-query |
| 2 | Nearby by coords | atlas-obscura-api `search({lat,lng})`, travel-hacking-toolkit | `atlas-obscura-pp-cli near <lat,lng>` | distance (mi) sort, client-side radius, cached |
| 3 | Nearby by place name | (none geocode it) | `(behavior in atlas-obscura-pp-cli near <place>)` Open-Meteo geocode | accepts city/place names, not just coords |
| 4 | Place detail | atlas-obscura-api `placeShort/placeFull` | `atlas-obscura-pp-cli show <id-or-slug>` | JSON-LD parse: coords, address, categories, Know-Before-You-Go; cached offline |
| 5 | Browse by category | Apify `byCategory` | `atlas-obscura-pp-cli browse category <slug>` | `/categories/<slug>` list, cached, `--json` |
| 6 | Browse by destination | Apify `byCountry`, martin0925 | `atlas-obscura-pp-cli browse place <city-slug>` | `/things-to-do/<slug>` list, cached |
| 7 | Trending / recently added | Apify `trending` | `atlas-obscura-pp-cli trending` | cached, `--json` |
| 8 | Category filter on nearby | Apify category+location | `(behavior in atlas-obscura-pp-cli near --category)` client-side tag filter | location×category join, bounded scan |
| 9 | Radius filter | Apify `radius` | `(behavior in atlas-obscura-pp-cli near --radius)` | client-side mile filter on distance |
| 10 | Interestingness scoring / filter mundane | travel-hacking-toolkit | `(behavior in near/route/gaps/surprise --min-score / --sort score)` | reusable score woven across commands |
| 11 | Optional images | travel-hacking-toolkit `--images` | `(behavior in atlas-obscura-pp-cli show/near --images)` | image URLs on demand, lean by default |
| 12 | Offline full-text search | (none — all live) | `(generated endpoint) framework search` + `sql` | FTS5 over cached title/subtitle/description |
| 13 | Warm the cache | (none) | `(generated endpoint) framework sync` | populate local mirror; fresh-on-read TTL |

## Transcendence (only possible with local store + corridor routing)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-----------------|
| 1 | Road-trip corridor (flagship) | `route <cityA> <cityB> [--min-score] [--detour-budget min] [--limit]` | hand-code | Geocode + sample corridor + near-search each + dedupe + score; no tool offers route-based wonder discovery | Use to find wonders along a drive A→B. Not for a single city — use `near` for that. |
| 2 | Saved trips / itineraries | `trip add|remove|list|show [--trip NAME]` | hand-code | Durable cross-session store; live scrapers have no memory | Use to accumulate places into a named trip across sessions. |
| 3 | Visited tracking | `visited mark|list [--export]` | hand-code | Session-spanning personal state in SQLite | Use to record places you've seen; feeds `gaps`/`surprise`. |
| 4 | Unvisited gaps near X | `gaps <place-or-latlng> [--radius] [--min-score]` | hand-code | Joins cached places × local visited table offline | Use to find good unvisited wonders near a point. Not for raw nearby — use `near`. |
| 5 | Walkable clusters | `cluster <place-or-latlng> [--radius] [--min]` | hand-code | Spatial clustering over the full local result set | Use to group nearby wonders into a walkable day. |
| 6 | Daily surprise | `surprise [--near] [--category] [--exclude-visited]` | hand-code | Date-seeded pick excluding visited via local join | Use for a stable daily/heartbeat pick that avoids repeats. |
| 7 | Export trip | `export <trip> --format gpx|geojson|md [--out FILE]` | hand-code | Serializes from local cache (incl KBYG) with zero network | Use to turn a saved trip into a GPX track, GeoJSON, or markdown log. |

Hand-code transcendence rows: **7** (route, trip, visited, gaps, cluster, surprise, export).
spec-emits rows: framework `sync`/`search`/`sql`/`doctor` (generator-provided).

## Stubs
None. Every row above ships fully implemented.
