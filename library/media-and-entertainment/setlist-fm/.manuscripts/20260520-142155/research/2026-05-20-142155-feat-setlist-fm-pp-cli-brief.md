# Setlist.fm CLI Brief

## API Identity
- **Domain:** Crowdsourced concert setlist database — every song every artist played at every venue.
- **Users:** Concert collectors, touring musicians, music journalists, statisticians, fans planning what to see next.
- **Data profile:** Strongly relational and analytics-friendly. 33 entity types: artists (MBID-keyed), setlists, songs, sets, venues, cities, countries, users, tours, attended/edited histories. ~7M setlists in the live database; deeply normalized.
- **Spec source:** Official Swagger 2.0 at `https://api.setlist.fm/docs/1.0/ui/swagger.json` (85 KB, 15 endpoints, 33 definitions).

## Reachability Risk
- **None.** Direct probe `GET /rest/1.0/search/artists?artistName=Radiohead` with key → HTTP 200 + 8 KB JSON. Without key → 403 (expected). Spec endpoint serves 85 KB from CloudFront. No bot protection, no Cloudflare challenge.

## API Surface (all GET, read-only)
| Path | Purpose |
|------|---------|
| `/1.0/search/artists` | Search artists by name |
| `/1.0/search/setlists` | Multi-filter search across all setlists (the workhorse) |
| `/1.0/search/venues` | Search venues |
| `/1.0/search/cities` | Search cities |
| `/1.0/search/countries` | List countries |
| `/1.0/artist/{mbid}` | Artist by MusicBrainz ID |
| `/1.0/artist/{mbid}/setlists` | All setlists for one artist (paginated; the engine for analytics) |
| `/1.0/setlist/{setlistId}` | One setlist with full songs/sets/encores |
| `/1.0/setlist/version/{versionId}` | Specific edit revision of a setlist |
| `/1.0/venue/{venueId}` | Venue detail |
| `/1.0/venue/{venueId}/setlists` | All setlists at one venue |
| `/1.0/city/{geoId}` | City detail |
| `/1.0/user/{userId}` | Public user profile |
| `/1.0/user/{userId}/attended` | A user's attended concerts |
| `/1.0/user/{userId}/edited` | A user's edits |

## Auth
- Header `x-api-key: <key>` (required on every call).
- Headers `Accept: application/json` and `Accept-Language: en` (or es/fr/de/pt/tr/it/pl).
- Env var: `SETLISTFM_API_KEY`. SDK-wrapper precedent: `setlist-fm-client` uses `SETLIST_FM_API_KEY` — we'll accept both for compatibility.
- Spec `securityDefinitions` is empty; **must enrich before generation** so the generator emits the header in client + doctor + README + auth command.

## Rate Limits (CRITICAL — drives architecture)
- **2 req/sec, 1440 req/day** on default keys (16/s and 50K/day on upgraded keys, but upgrades are slow).
- Token-bucket throttling; bursting triggers 429.
- **Implication:** Live-only CLIs burn the daily budget in minutes for any analytics workload. The local SQLite store + sync-once pattern is a 1000× efficiency win, not a nice-to-have. Throttle client to 2 RPS by default; emit `--burst` opt-in.

## Top Workflows
1. **Artist tour intelligence.** "What is Radiohead playing on this tour? What did they play last night? What song is overdue?"
2. **Venue intelligence.** "What setlists came out of Madison Square Garden last month?"
3. **Concert collector log.** "My attended shows, my unique artists/songs/venues count, my biggest gap."
4. **Setlist prediction.** "If I'm seeing Phoenix at Forest Hills tomorrow, what songs are most likely to appear?"
5. **Song trivia / journalist research.** "When was the last time Hey Jude was played live? By whom?"

## Table Stakes (every SDK wrapper has these — we must match all)
- Search artists / venues / cities / countries / setlists with all documented filters
- Get artist / setlist / venue / city / user by id
- List setlists by artist (paginated)
- List setlists by venue (paginated)
- User attended + edited histories
- Setlist version retrieval

## Data Layer (the moat)
- **Primary entities (SQLite tables):** `artists` (mbid PK), `venues` (id PK), `cities` (geoId PK), `countries` (code PK), `setlists` (id PK, artist_mbid FK, venue_id FK, event_date, tour_name, last_updated), `sets` (setlist_id FK, position, is_encore), `songs` (setlist_id FK, set_position, position, name, info, is_cover, cover_artist_mbid, with_artist_mbid, tape), `users` (id PK), `attended` (user_id, setlist_id), `tours` (artist_mbid, name) derived.
- **Sync cursor:** Use `lastUpdated` timestamps on setlists + per-resource ETag-style "seen" tracking. Fan out from a watched-artist list rather than a global firehose (no global feed exists).
- **FTS:** FTS5 over `songs.name + setlists.tour_name + setlists.info`, joined on artist.name / venue.name / city.name for cross-entity search.

## Codebase Intelligence
- No GitHub repo to DeepWiki against (no official server). Skip.
- Source-of-truth wrappers: `terhuerne/setlistfm-js` (JS), `zschumacher/setlist-fm-client` (Python, has pydantic models), `jtmolon/repertorio` (Python), `nucleos/setlistfm` (PHP), `MolinRE/SetlistNet` (.NET), `fusionet24/SetListR` (R). All are thin wrappers — none expose offline storage or analytics.
- One scraper-style MCP exists on Apify (`hoholabs/setlistfm-scraper`) but it scrapes the HTML site, not the official API. Not a competitive analytics surface.

## Competitive Landscape
- **Zero CLIs exist.** Six SDK wrappers across 5 languages, none a CLI.
- **Closest analytics tool** is the official website itself (search + browse + stats pages per artist).
- This is **wide-open ground** for a GOAT CLI. Anything beyond "thin wrapper" is novel-feature territory.

## Product Thesis
- **Name:** `setlist-fm-pp-cli` (binary), branded "Setlist.fm" in README/SKILL.
- **Headline:** *Every Setlist.fm endpoint, plus offline analytics no API call can return — tour shape, song frequency, "what's overdue", setlist prediction, attendance stats.*
- **Why it should exist:** The API serves data; nothing analyzes it. The 1440/day rate limit makes a local SQLite store mandatory for any real workflow. Once synced, an artist's complete history is queryable in milliseconds with offline FTS and SQL.

## Build Priorities
1. **P0 — Foundation:** Full SQLite store for all 9 entity types, `sync` with rate-aware throttling, FTS5, `search`/`sql`/`context` plumbing, auth + doctor + README + SKILL.
2. **P1 — Absorb:** All 15 GET endpoints as `<resource> <action>` commands with `--json`/`--csv`/`--select`/`--limit`/`--dry-run`, paginated list helpers, ID resolvers (`artist resolve <name>` → mbid).
3. **P2 — Transcend:** Tour-shape analytics, song-frequency stats, "overdue" / "due-soon" prediction, setlist prediction (bingo), gap finder, cover finder, attendance dashboard, venue loyalty, debut tracker, since-watcher.
