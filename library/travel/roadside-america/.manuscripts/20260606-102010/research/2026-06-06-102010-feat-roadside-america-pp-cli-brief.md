# Roadside America CLI Brief

## API Identity
- **Domain:** roadsideamerica.com — long-running (since 1996) community guide to offbeat / quirky US & Canada tourist attractions ("World's Largest …", muffler men, mystery spots, folk art, oddities).
- **Users:** road-trippers, travel writers, "weird America" enthusiasts; agents planning quirky stops along a route.
- **Data profile:** ~scraped/community-sourced. Each attraction has: numeric id, name, street, city, state, a writeup/directions blurb, a detail link (`/tip/<id>` or `/story/<id>`). No coordinates exposed on pages; distance is computed server-side for nearby queries. ~70+ editorial "themes" exist but their slug vocabulary is undocumented/uncrackable.

## Source Policy (decided with user 2026-06-06)
- robots.txt allows generic agents (`User-agent: *`) on all content paths; disallows only `/travel/`, `/tipSubmission.html`, `/mailus.html`, `/mailthisexit.html`, `/shared/randomSite.php`, `/admin/`, `/uploader/`.
- robots.txt **blocks model-training crawlers** (`GPTBot`, **`ClaudeBot`** → `Disallow: /`) but **allows user-initiated / search-retrieval bots** (`OAI-SearchBot`, `ChatGPT-User`, Googlebot, Bingbot): *"Allow AI search/retrieval bots that can drive referral traffic and citations back to this site."*
- **User chose: proceed as a polite, attributing, user-initiated tool.** Build-time obligations: honest descriptive User-Agent (no browser/Googlebot impersonation), respect robots disallow paths, ~1 req/3s adaptive rate limit, SQLite cache to minimize refetch, attribute + link back to every source page, never use data for training.

## Reachability Risk
- **None.** All data endpoints return HTTP 200 with parseable content via plain `curl` (Apache/PHP behind CloudFront). No bot wall, no JS requirement for the data endpoints, no auth. (Probe-safe GET; no mutation probing.)
- Provisional runtime: `standard_http` (plain HTTP confirmed; no Surf/clearance/browser needed).

## Data Layer (discovered via static JS analysis of mapFunctions.js + direct-HTTP confirmation)
The site's map UI calls these `XMLHttpRequest` endpoints; all confirmed working:

| Purpose | Endpoint | Notes |
|---|---|---|
| **By state** | `GET /map/attractionsByState.php?state=<st>` | Returns `<ul class="attrlist">` of ALL attractions in a state (e.g. TX = 542 KB). No coords/distance. |
| **Nearby (radius)** | `GET /map/nearbyAttractions.php?long=<lng>&lat=<lat>&delta=<deg>&id=0` | Same attrlist WITH distance (`<div class="location">(<1 mi. away)</div>`), distance-sorted. `delta` = bounding half-size in degrees. |
| **Detail** | `GET /tip/<id>` and `GET /story/<id>` | Full page: `<title>` = "City, ST - Name", `Address:` (street), `Directions:` prose, writeup body, `og:description`. |
| **Name search** | `POST /search/tip` (`tip_AttractionName`, `tip_Town`, `tip_State`) | "Search Results" page with `/tip/<id>` links. Powers name→id resolution. |

**attrlist record shape** (per `<li id="attr-<ID>-li">`):
`<a class="attractname" href="javascript:openInfo(<ID>)">NAME</a>`, `<div class="street">…</div>`, `<div class="cityState">City, ST</div>`, `<div class="location">(X mi. away)</div>` (nearby only), `<a class="mapmorelink" href="/tip/<ID>">More…</a>` (or `/story/<ID>` or a `/shared/redirectFeatureLink.php` redirect).

- **Sync cursor:** none server-side; use per-record `fetched_at` for fresh-on-read TTL.
- **FTS/search:** local FTS over cached attraction name/city/state/writeup.

## Geocoding (for `near "City, ST"` / place names)
- Pages expose no coordinates; the map center is fetched at runtime. So place→lat/lng needs a geocoder.
- **Decision:** keyless **OpenStreetMap Nominatim** (`https://nominatim.openstreetmap.org/search`), cached in SQLite + rate-limited (Nominatim usage policy: ≤1 req/s, descriptive UA). `near <lat>,<lng>` bypasses geocoding entirely.

## Existing tools / competitors
- No real data API/CLI for this site. The only "Roadside-America-API" repo on GitHub is a random-image endpoint (irrelevant). Atlas Obscura (`atlasobscura.com/categories/roadside-attractions`) is the conceptual competitor; the official paid iOS app is the incumbent. **This CLI would be the first agent-native data tool for roadsideamerica.com.**

## Top Workflows
1. "What quirky stuff is near me / near these coords / near this city?" → `near`
2. "Show me everything weird in <state>" → `state`
3. "Find the big/small/tall/weird-food superlatives" → `category` (local classification)
4. "Give me the full writeup + where exactly is it" → `show`
5. Plan a route's worth of stops; cache for offline; pipe to other tools (`--json`/`--select`/`--csv`).

## Table Stakes (match the website)
- Browse by state; nearby search; attraction detail with address + writeup; name search; link back to source.

## category design (theme slugs are uncrackable)
`attractionsByTheme.php?theme=<slug>` works but the slug vocabulary is internal (display names like "Big"/"Atomic"/"Animals" all return "No Attractions Found"; `themes.php` 404s). **`category` will be local superlative/keyword classification** over fetched/cached attractions — directly serving the user's examples (biggest, smallest, tallest, weird-food) plus more (giants, muffler-men, animals, signs, etc.). This is stronger than the undocumented endpoint and is a transcendence feature.

## Product Thesis
- **Name:** roadside-america (binary `roadside-america-pp-cli`)
- **Why it should exist:** The web's best catalog of offbeat Americana has no API and a paywalled app. An agent-native CLI with local SQLite cache, offline search, superlative classification, and pipe-friendly output turns "find quirky stops" into a scriptable, route-planning primitive — while staying a polite, attributing, user-initiated citizen of the source site.

## Build Priorities
1. **P0 foundation:** HTTP client (polite UA + ~1 req/3s adaptive limiter + robots-path guard), attrlist HTML parser, SQLite store (attractions + detail + geocode cache) with fresh-on-read TTL, attribution/source-URL on every record.
2. **P1 commands:** `near <place|lat,lng> [--radius mi]`, `state <ST>`, `show <id|slug|name>`. Name search resolution.
3. **P2 transcendence:** `category` (local classification), route/trip stops, offline `search`/`sql`, "biggest things" leaderboards, agent-native `--json/--select/--csv/--compact`, MCP mirror.
