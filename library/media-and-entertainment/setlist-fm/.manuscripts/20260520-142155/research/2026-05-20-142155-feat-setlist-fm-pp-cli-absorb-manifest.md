# Setlist.fm Absorb Manifest

## Tools surveyed
- **terhuerne/setlistfm-js** (JS, npm `setlistfm-js`) — 15 methods, 1:1 API mirror, promise-based.
- **zschumacher/setlist-fm-client** (Python, `pip install setlist-fm-client`) — 15 methods, sync + async, pydantic models.
- **jtmolon/repertorio** (Python) — 15 methods, requests wrapper.
- **nucleos/setlistfm** (PHP, composer) — 15 methods.
- **MolinRE/SetlistNet** (.NET) — 15 methods.
- **fusionet24/SetListR** (R) — analytics wrapper (closest to our thesis but R-only and no offline store).
- **hoholabs/setlistfm-scraper** (Apify MCP) — HTML scraper, not API client; out of scope as a peer.
- **DataFire integration**, **APIs-guru directory** — mirror of the same Swagger spec.

**Pattern:** every wrapper is a 1:1 surface mirror. None offer offline storage, analytics, or compound queries. The bar is high in *coverage* (every endpoint) and low in *transcendence* (nobody competes).

---

## Absorbed (match or beat every wrapper)

| #  | Feature                              | Best Source                            | Our Implementation                                                       | Added Value |
|----|--------------------------------------|----------------------------------------|--------------------------------------------------------------------------|-------------|
| 1  | Search artists by name               | setlistfm-js `searchArtists()`         | `search artists --name <q>` (auto-paginates, `--json/--csv/--select`)   | Offline-cached, agent-native flags |
| 2  | Search venues by name                | setlistfm-js `searchVenues()`          | `search venues --name <q> [--city <name>] [--country <code>]`            | Cross-filter, offline FTS after sync |
| 3  | Search cities by name                | setlistfm-js `searchCities()`          | `search cities --name <q> [--country <code>]`                            | `--json` agent surface |
| 4  | List countries                       | setlistfm-js `searchCountries()`       | `search countries`                                                       | Cached once, no API cost on repeat |
| 5  | Search setlists (multi-filter)       | setlistfm-js `searchSetlists()`        | `search setlists --artist <q> --year YYYY --venue <q> --city <q> ...`   | 17 filters mapped, paginates, `--max-pages` budget |
| 6  | Artist by MBID                       | setlistfm-js `getArtist()`             | `artist get <mbid>`                                                      | `--select` for compact agent output |
| 7  | Artist's setlists (paginated)        | setlistfm-js `getArtistSetlists()`     | `artist setlists <mbid> [--page N] [--all]`                              | `--all` auto-paginates; stores to SQLite |
| 8  | Setlist detail                       | setlistfm-js `getSetlist()`            | `setlist get <id>`                                                       | Returns parsed song list, encore-aware |
| 9  | Setlist version (revision)           | setlistfm-js `getSetlistByVersion()`   | `setlist version <versionId>`                                            | `setlist history <id>` to diff revisions (transcend) |
| 10 | Venue detail                         | setlistfm-js `getVenue()`              | `venue get <id>`                                                         | |
| 11 | Venue setlists (paginated)           | setlistfm-js `getVenueSetlists()`      | `venue setlists <id> [--all]`                                            | Auto-paginates |
| 12 | City detail                          | setlistfm-js `getCity()`               | `city get <geoId>`                                                       | |
| 13 | User profile                         | setlistfm-js `getUser()`               | `user get <userId>`                                                      | |
| 14 | User attended history                | setlistfm-js `getUserAttended()`       | `user attended <userId> [--all]`                                         | Stored to local `attended` table |
| 15 | User edited history                  | setlistfm-js `getUserEdited()`         | `user edited <userId> [--all]`                                           | |
| 16 | ID resolver (name → MBID)            | none — manual                          | `artist resolve <name>` (top match + confidence)                         | Removes copy-paste-MBID friction |
| 17 | Full sync of an artist               | none — manual                          | `sync --artist <name|mbid> [--max-pages N]`                              | Rate-aware; respects 2 RPS / 1440 day budget |
| 18 | Offline FTS across cached data       | none                                   | `search "<term>"` (FTS5 across songs+setlists+artists+venues)            | Zero API cost after sync |
| 19 | Raw SQL escape hatch                 | none                                   | `sql "<query>"`                                                          | Power user / agent analytics |
| 20 | Auth / health / config plumbing      | partial                                | `auth set-token`, `auth status`, `doctor`, `config show`                 | One-command setup; explicit rate-limit health check |

All 15 wrapper endpoints absorbed and beaten with offline caching, agent flags, and rate-aware client behavior. Nothing shipped as a stub.

---

## Transcendence (only possible with our local store)

| #  | Feature                | Command                                                  | Score | Why Only We Can Do This |
|----|------------------------|----------------------------------------------------------|-------|------------------------|
| T1 | Setlist prediction     | `predict <artist> [--venue <id>] [--last N]`             | 10/10 | Computes per-song probability from last N setlists; weighted by recency. Needs full artist history in store. |
| T2 | Per-song stats         | `song stats <artist> "<song>"`                           | 10/10 | Total plays, first/last date, longest gap, average set position, % of shows played. Single SQL aggregate over the songs table. |
| T3 | Overdue songs          | `overdue <artist> [--top N]`                             | 9/10  | Songs sorted by shows-since-last-played. The question every fan asks before a show. |
| T4 | Tour shape             | `tour shape <artist> [--tour <name>]`                    | 9/10  | Median set length, encore frequency, song-position histogram, opener/closer top-10 — all for one tour. |
| T5 | Tour comparison        | `compare <artist> --tour A --tour B`                     | 9/10  | Overlap %, dropped songs, debuts, position shifts. Shows how a band evolves. |
| T6 | Attended dashboard     | `attended stats <userId>`                                | 9/10  | Total shows, unique artists/songs/venues/cities, biggest streak, longest gap, decade breakdown. |
| T7 | Encore intelligence    | `encore <artist>`                                        | 8/10  | Top encore openers, top encore closers, % of shows with encore. |
| T8 | Cover finder           | `covers <artist>`                                        | 8/10  | All cover songs, ranked by frequency, with original artist (the API marks `cover` per song). |
| T9 | Setlist diff           | `setlist diff <setlistId-A> <setlistId-B>`               | 8/10  | Side-by-side song diff; `--json` for agent consumption. |
| T10| Gap finder             | `song gap <artist> "<song>"`                             | 7/10  | Biggest dry spells for one song; when the comeback happened. |
| T11| Rare debuts            | `debut <artist>`                                         | 7/10  | Songs played exactly once live (rare-bookings hunt). |
| T12| Bingo card generator   | `bingo <artist> --songs 25`                              | 7/10  | Generates a printable bingo card of N most-likely-to-play songs for the upcoming show. Fun + fan-festival use case. |
| T13| Venue loyalty          | `venue-loyalty <artist>`                                 | 7/10  | Top venues by play count, "home venue" detection. |
| T14| Since-cursor digest    | `since <iso-timestamp> [--artist <name>]`                | 8/10  | Setlist updates since last run; pair with `sync` for delta refresh. Daily-digest workflow. |
| T15| Tour route             | `tour route <artist> [--tour <name>]`                    | 6/10  | Sequence of city/venue stops, gaps highlighted. (Lower because no built-in geo distance; future enhancement.) |
| T16| Spotify playlist       | `playlist <artist> [--last N] [--output csv\|m3u\|spotify-search] [--create]` | 10/10 | Reads songs for the artist's most recent setlist (or the merged last N setlists) from the local store and exports as CSV/M3U/Spotify-search URIs. With `--create` and `SPOTIFY_TOKEN` set, uses the Spotify Web API to search-resolve each track and create a real playlist named after the show. User-requested. |

16 transcendence features, all backed by the same SQLite store, all impossible without it.

---

## Status
- **All features in shipping scope.** Zero stubs.
- The rate-limit reality (2 RPS / 1440/day) is what makes this CLI obviously superior to live-only SDK wrappers: one `sync` populates the store; every transcendence query runs offline forever.
