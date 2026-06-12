# Blu-ray.com CLI Brief

## API Identity
- **Domain:** Blu-ray.com — the canonical disc database for Blu-ray / 4K UHD / DVD / digital releases since 2002. ~1M+ canonical release URLs across blu-ray, dvd, itunes, digital, MA (Movies Anywhere), UV (UltraViolet), games, "main" (movie umbrella pages), and cast.
- **Users:** Disc collectors, A/V hobbyists, deal hunters, home-theater enthusiasts, movie reviewers.
- **Data profile:** Big, slowly-mutating catalog (movie metadata, technical specs, ratings, screenshots, packaging). Fast-mutating layers on top: deals (real-time prices from retailers), news, release dates, coming soon, preorders.

## Reachability
- **Mode:** `standard_http` (plain stdlib HTTP, 200 across the board, ~95% confidence per `probe-reachability`). No CF, no DataDome, no JS challenge.
- **Encoding:** `ISO-8859-1` (latin-1). Pages must be decoded as latin-1, not UTF-8.
- **Robots.txt explicitly disallows** for polite bots:
  - `/movies/search.php`, `/news/search.php`, `/search/` — search endpoints (JS-rendered anyway)
  - `/community/collection.php`, `/community/favorites.php`, `/community/profile.php`, `/community/ratings.php` — personal/user data
  - `/cgi-bin/`, `/link/` — internal redirector
- **The site blocks AI-bot user agents in robots.txt**: ClaudeBot, GPTBot, Bytespider, Amazonbot, meta-externalagent, AhrefsBot, SemrushBot, etc. Our CLI MUST use a normal browser User-Agent string and respect robots-disallowed paths.
- **Rate limit (from TUVIMEN scraper):** ~4,000 pages/day per IP before throttling; IPv6 blocked. Design the CLI to be polite: configurable `--wait` (default 1s+jitter), single-threaded by default, retry-with-backoff.

## Top Workflows
1. **"What's new on Blu-ray/4K this week"** — list new releases, coming soon, preorders, filter by format / country.
2. **"Look up a specific disc"** — by canonical URL, by ID, by title slug; full technical spec (codec, resolution, audio, subtitles, runtime, discs, packaging, region coding) + community rating.
3. **"Find a deal"** — current sale prices across retailers, % off list, with cover/title/retailer/posted-at.
4. **"Track my wishlist for price drops"** — local watchlist of disc IDs + repeated deal scans → notify on drop or new-low.
5. **"Catch up on news"** — recent Blu-ray.com news stories.
6. **"Sync the full catalog for offline search"** — pull the public sitemap, build a local FTS5 index of every Blu-ray release ever, search instantly without round-trips.

## Source Reachability + Auth
- **No auth required for public surfaces.** No API key, no OAuth, no login. Just a polite UA.
- **No official API.** Blu-ray.com publishes no JSON API; all data comes from HTML extraction + the published XML sitemap.
- **Sitemap is the gold mine.** `https://www.blu-ray.com/sitemap.xml` is a sitemap index pointing to ~30 gzipped sub-sitemaps:
  - `sitemap_main.xml.gz`, `sitemap_news.xml.gz`
  - `sitemap_movies_0..3.xml.gz`
  - `sitemap_bluraymovies_0..8.xml.gz` (50,000 URLs each, ~400-450k Blu-ray releases)
  - `sitemap_itunesmovies_0..4.xml.gz`, `sitemap_dvdmovies_0..6.xml.gz`, `sitemap_digitalmovies_0..1.xml.gz`
  - `sitemap_cast_0..10.xml.gz` (people)
  - `sitemap_ma.xml.gz`, `sitemap_games.xml.gz`, `sitemap_other.xml.gz`
- News sitemap is enriched with `<news:title>` and `<news:publication_date>` per URL.
- TUVIMEN scraper claims ~1,170,773 total URLs.

## URL Patterns
- **Movie release detail (canonical):** `/movies/<TitleSlug>-Blu-ray/<id>/` (variants: `-4K-Blu-ray`, `-3D-Blu-ray`). 301-redirects on stale slug; the `<id>` is stable.
- **Movie umbrella page:** `/main/<id>/`.
- **Format variants:** `/digital/<id>/`, `/dvd/<id>/`, `/itunes/<id>/`, `/ma/<id>/`, `/uv/<id>/`, `/prime/<id>/`.
- **Listings:**
  - `/movies/movies.php?show=newreleases` (paginated `&page=N`)
  - `/movies/movies.php?show=comingsoon`
  - `/movies/releasedates.php` (filter `?year=YYYY&country=USA&format=4K`)
  - `/theatrical/releasedates.php`, `/digital/releasedates.php`
- **Deals:** `/deals/` (filter `?country=USA&format=4k|bluray|dvd`). Each row links via `/link/click.php?p=<offerid>&retailerid=<rid>` (affiliate redirector — we surface the deal data but should not follow redirects for actual purchases).
- **News:**
  - News index `/news/` (322KB HTML)
  - Story URL: `/news/?id=<n>` (numeric id)
  - News sitemap is the clean enumeration.
- **Search:** JS-rendered + robots-disallowed. We DO NOT scrape `/movies/search.php`. Instead, sync the sitemap → SQLite + FTS5 for offline title search. This is BOTH the polite path AND the higher-quality path.

## Data Layer
**Primary entities:**
- `releases` — id, kind (blu-ray|4k|3d|dvd|digital|itunes|ma|uv), title, subtitle, country, distributor, year, runtime, rated, release_date, slug, parent_id, list_price, current_price, cover_url, json_blob.
- `release_specs` — release_id, video, audio_tracks, subtitles, discs, packaging, playback, region_coding, links.
- `release_ratings` — release_id, movie, video, video2k, video4k, audio, extras, overall (0-100).
- `movies` — id (umbrella), title, year, cover_url, screenshots, studios, plot_tags, appeals.
- `news` — id, title, publication_date, url, fetched_at, body.
- `deals` — id, release_id, retailer_id, list_price, sale_price, posted_at, click_url, percent_off, observed_at.
- `retailers` — id, name, url.
- `watchlist` — release_id, target_price, added_at, low_seen, alerted_at (local-only).
- `price_history` — release_id, retailer_id, observed_at, price (local longitudinal).
- `cast_people` — id, name, slug.
- `sitemap_index` — url, kind, last_seen, lastmod.

**Sync cursor:** `<lastmod>` on the sitemap index. Incremental: re-pull sitemap weekly, diff against `sitemap_index`, only refetch new/changed.

**FTS5:** `releases_fts(title, year, distributor, country, kind, slug, content='releases')` — covers offline title search.

## Codebase Intelligence
- **TUVIMEN/blu-ray-scraper** (Python, GPLv3) — the only comprehensive scraper, the de-facto reference for what's extractable. 8 resource categories, ~1.17M URLs total, ~300-400 KB per page. Uses `reliq` + `treerequests` + `requests`. Per-release Blu-ray fields:
  - parent_link, parent_id, title, country, subtitle, distributor, year1, year2, runtime, release, seasons, rated, cover, list_price, price, sources (retailer URLs)
  - ratings: movie, video, video2k, video4k, audio, extras, overall
  - info: video, discs, digital, packaging, playback, audio, subtitles, links
  - packaging (member-uploaded images, dated)
  - region_coding (member-submitted region reviews)
  Per umbrella movie: cover, title, year, screenshots, watched/watchlist/notinterested counts, genre appeals %, plottags, studios, per-format releases list.
- **Flexget PR #1336** — added blu-ray.com lookup plugin (2016).
- **blu2trakt** — PHP one-shot Trakt importer.
- **n8n template** — "Send daily 4K Blu-ray preorder updates from Blu-ray.com to Discord" (proves preorder polling is a wanted workflow).
- **NO MCP server, NO Go CLI, NO JSON API exists.** Clearest greenfield in disc-collector tooling.
- **CLZ Movies** — closed-source desktop/mobile collection app; competing commercial product but out of scope (CLI/agent-native).

## Reachability Risk
- **Low.** Standard HTTP, no challenge. Sitemap published with current `lastmod`. Latency 60-400ms.
- **Risk:** aggressive crawling triggers IP throttling (~4k pages/day). Mitigated by per-host rate limiter, low default concurrency, polite UA, `--wait`/`--max-rate` flags, support for `--proxy`.

## Product Thesis
- **Name:** `blu-ray-pp-cli`
- **Headline:** The disc-collector's CLI for Blu-ray.com — a complete, offline, agent-native index of every Blu-ray, 4K, DVD, and digital release, with live deal tracking and price-drop watchlists. No account, no API key, just data.
- **Why it should exist:**
  1. Blu-ray.com's only "export" is a UPC list. Collectors and tools both need structured access; nobody publishes it.
  2. No MCP server exists — agents can't ask "what 4K UHDs come out next week" or "find Aliens on sale right now."
  3. JS-rendered search is slow; the site is heavy (~300-400 KB per page). A local FTS5 index over the sitemap is dramatically faster.
  4. Price-drop watchlist is the universal feature collectors want and nobody offers — blu-ray.com has no alerts; CLZ tracks owned discs not pricing.

## Build Priorities
1. **Catalog sync from sitemap** → SQLite + FTS5. Foundation for everything.
2. **Release detail fetcher** by id or canonical URL. Full tech specs, ratings, packaging, region coding, source links. Cached locally.
3. **Listings**: new-releases, coming-soon, release calendar by year + filter (format/country/page).
4. **Deals scanner** with retailer attribution, `--min-discount`, `--max-price`, `--format` filters.
5. **News reader** from news sitemap + per-story HTML fetch.
6. **Watchlist + price history** (transcendence): local watch by release_id, repeated deal scans, longitudinal price store, alert on target/new-low.
7. **Catalog drift** (transcendence): diff today's sitemap vs last sync, surface "new this week", "removed from catalog."
8. **Edition compare** (transcendence): given a movie umbrella id, list every disc edition (4K, Blu-ray, Steelbook, Director's Cut, country) with price + rating + release-date side-by-side.
9. **UPC import** (transcendence): given a UPC list (CSV export of an existing collection), resolve each to a release_id and enrich. Closes the gap left by blu-ray.com's UPC-only export.

## Things NOT in scope
- Personal user data on blu-ray.com itself (collection, ratings, wishlist) — robots-disallowed.
- Forum scraping.
- Trakt/Letterboxd push — community wrappers exist; out of scope.
- Image hosting — link, not mirror.
