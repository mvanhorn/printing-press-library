# Roadside America — Absorb Manifest

No competing CLI/API exists for roadsideamerica.com (only an unrelated random-image repo). "Absorbed" therefore = the website's own feature surface, matched and beaten with agent-native output (`--json/--select/--csv/--compact`, typed exit codes, offline SQLite cache, pipe-friendly). Transcendence = local-data features the site does not offer as commands.

## Absorbed (match or beat the website)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Nearby attractions (map-a-city) | roadsideamerica.com map / `nearbyAttractions.php` | `roadside-america-pp-cli near` | Accepts `<lat,lng>` OR place name (geocoded), `--radius mi` filter, JSON/CSV, cached offline, distance-sorted, source links |
| 2 | Browse by state | `/location/<st>`, `attractionsByState.php` | `roadside-america-pp-cli state` | Full state list (not teaser), `--json/--csv`, FTS-indexed into local cache |
| 3 | Attraction detail / writeup | `/tip/<id>`, `/story/<id>` | `roadside-america-pp-cli show` | Structured fields (name, street, city, state, writeup, source URL), accepts id/slug/name, `--json` |
| 4 | Name / keyword search | `/search/tip` | `roadside-america-pp-cli search` | Offline FTS over cache + live name-search fallback, agent-native output |
| 5 | Source attribution / link-back | the site itself | (behavior in `roadside-america-pp-cli show`) | Every record carries `source_url` + "community-sourced from RoadsideAmerica.com"; honors the polite-citizen policy |
| 6 | Refresh / populate data | the site (manual browse) | (behavior in `roadside-america-pp-cli sync`) | Polite ~1 req/3s sync of a state into local SQLite; fresh-on-read TTL |
| 7 | Health / reachability | n/a | `roadside-america-pp-cli doctor` | robots-aware reachability + cache + rate-limit report |
| 8 | Power query | n/a | `roadside-america-pp-cli sql` | Raw read-only SELECT over the local attraction cache (framework) |

## Transcendence (only possible with our local-cache + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Superlative/keyword categories | `category` | hand-code | Site's theme slugs are internal/undocumented; we classify by name/desc patterns (biggest, smallest, tallest, weird-food, giants, muffler-men, animals, signs) over local data — the site offers no such command | Use for "find the big/small/tall/weird-food things". Pass `--list` to see categories. Operates over the local cache; run `sync`/`state`/`near` first to populate. |
| 2 | Quirk leaderboard / rollups | `stats` | hand-code | Requires aggregation across cached attractions (counts by state and category) the site never exposes | Use to summarize what's cached: counts by state, by category, totals. Do NOT use for a single lookup; use `show`. |
| 3 | Surprise-me | `random` | hand-code | Weighted random pick from local cache, filterable by `--state`/`--category` — the site's "roll the dice" with structured output | Use for serendipity / road-trip inspiration. `--state TX --category giants` to constrain. |
| 4 | Road-trip stops | `trip` | hand-code | Aggregates `near` results for a list of stops (geocoded), dedupes, labels by stop — turns the catalog into a route-planning primitive no UI offers | Use to collect quirky stops across multiple cities/coords in one call. `trip "Austin, TX" "Waco, TX" --radius 15`. |
| 5 | State quirk comparison | `compare` | hand-code | Cross-state counts + top picks in one call (local join) — the site has no compare view | Use to compare two states' offbeat density. `compare TX CA`. Do NOT use for listing; use `state`. |

Minimum 5 transcendence features met (all hand-code).

## Stubs / risks
- **No stubs.** Every listed command ships fully implemented.
- **Geocoding dependency:** `near <place>` and `trip` use keyless OSM Nominatim (cached, rate-limited, disclosed). `near <lat,lng>` needs no geocoder.
- **No per-attraction coordinates** from the site → `--radius` is enforced via the site's own "X mi. away" distances (nearby endpoint), not client-side haversine. Documented honestly.
- **Politeness is load-bearing:** ~1 req/3s adaptive limiter + SQLite cache + robots-path guard + honest UA, per the user-approved source policy.

## Hand-code summary (for Phase Gate)
- Hand-code transcendence rows: **5** (`category`, `stats`, `random`, `trip`, `compare`).
- Hand-built absorbed commands: `near`, `state`, `show`, `search` (+ shared parser/geocoder/cache/classifier).
- Framework-provided: `sql`, `sync` scaffold, `doctor`, `--json/--select/--csv/--compact`, MCP mirror, agent-context.
