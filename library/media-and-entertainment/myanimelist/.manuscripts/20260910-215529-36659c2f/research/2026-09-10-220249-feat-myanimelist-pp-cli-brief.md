# MyAnimeList CLI Brief

Target: **https://myanimelist.net/** — the website itself (Phase 0 choice), **anonymous-only** (Phase 0 auth gate: no cookie, no token, no OAuth surface in the printed CLI).
Run: `20260910-215529-36659c2f` · slug `myanimelist` · binary `myanimelist-pp-cli`

## API Identity

- **Domain:** MyAnimeList is the largest English-language anime/manga database. It exposes **no anonymous official read API**: `api.myanimelist.net/v2` requires OAuth2 + `X-MAL-CLIENT-ID`, and the legacy v1 endpoint was retired. The public website is therefore the only credential-free data surface, and it is **server-rendered HTML plus exactly three machine-readable surfaces** (a JSON prefix-search, RSS feeds, and an XML sitemap).
- **Verified machine-readable surfaces (anonymous):**
  | Surface | URL | Verified |
  |---|---|---|
  | Instant/prefix search (JSON) | `/search/prefix.json?type=anime\|manga\|character\|person&keyword=<q>&v=1` | `200 application/json`, rich payload (`media_type`, `start_year`, `aired`, `score`, `status` for anime; `related_works`, `favorites` for characters) |
  | News RSS | `/rss/news.xml` (`/rss.php?type=news` → 301) | `200 application/xml` |
  | Featured-article RSS | `/rss/featured.xml` | `200 application/xml` |
  | Sitemap index | `/sitemap/index.xml` → `main`, `anime-001`, `manga-00N`, `character-00N`, `people-00N`, `news-001`, `featured-001`, `company-001` | `200 text/xml`, robots-advertised |
  | PWA manifest | `/manifest.json` | `200` (no data value) |
- **HTML routes verified `200` with full anonymous content** (47-URL probe matrix + DOM parse):
  `/anime/{id}` (slug optional — `/anime/1/Fullmetal_Alchemist_Brotherhood` serves Cowboy Bebop, i.e. the slug is ignored and the id wins), `/manga/{id}`, `/{anime,manga}/{id}/stats`, `.../characters`, `.../staff`, `.../reviews`, `.../recommendations`, `.../userrecs`, `.../pics`, `.../video`, `.../episode`, `.../episode/{n}`, `.../forum`, `/topanime.php?type={tv,movie,airing,bypopularity,favorite}`, `/topmanga.php?type={manga,novels,lightnovels,oneshots,doujin,manhwa,manhua,bypopularity,favorite}`, `/anime/season` and `/anime/season/{year}/{season}` (+`?page=N`), `/anime/genre/{id}/{slug}`, `/manga/genre/{id}/{slug}`, `/anime/producer/{id}/{slug}` (studios **and** licensors share this route), `/manga/magazine/{id}/{slug}`, `/character/{id}`, `/people/{id}`, `/profile/{username}` (**public**), `/news`, `/news/{id}`, `/forum/`, `/forum/?board=N`, `/forum/?topicid=N`, `/anime.php?q=` (advanced filters), `/manga.php?q=`.
- **Advanced search is GET-parameter driven and verified working** against `/anime.php`: `q`, `type`, `score`, `status`, `p`, `r`, `sy`/`ey` (year range), `genre`, `order`. This is the single richest filter surface on the site and is fully scriptable.
- **Users:** seasonal watchers deciding what to pick up; completionists working a backlog; franchise watchers who need watch order; VA/staff and studio enthusiasts doing filmography cross-reference; data-curious users who want score/status distributions that MAL only renders as bars on a `/stats` page.
- **Data profile:** ~30k+ anime entries, comparable manga volume, characters, people/staff, studios/licensors, magazines, per-title score **and** status distributions, per-episode rows (number, EN+JP title, air date, poll average, reply count), reviews with helpfulness counts, community recommendations, news articles, forum boards/topics, public user profiles.

## Reachability Risk

- **None for the read paths the CLI uses.** Every route in the probe matrix returned `200` with full content under a standard Chrome UA, and real data parsed out of the DOM (`span.score-label` → `8.75` on `/anime/1`).
- **`probe-reachability` reports `mode: browser_clearance_http`, and that verdict is a FALSE POSITIVE.** Its only evidence is `body_evidence: ["recaptcha"]` — the string `recaptcha` appears in every page because MAL embeds a reCAPTCHA widget in its *login/registration* form (`window.GRECAPTCHA_SITE_KEY` is present in the page head). It is not a challenge: a real browser capture of `/anime/1` reported `challenge: false` with a 373,843-byte rendered DOM. **Runtime is `standard_http`** with a browser-like `User-Agent`; no clearance cookie, no Surf requirement, no resident browser.
- **Browser-sniff result (pre-approved via the Phase 0 website choice): the site has NO hidden JSON/XHR API.** Captured with `browser-use` on `/anime/1`, `/anime/season/2026/fall`, and `/anime.php?q=frieren`: the only first-party resource requests are `/ux/s` (analytics beacon) and `/manifest.json`. Seasonal category blocks (5: TV/ONA/OVA/Movie/Special) and search results are **server-rendered**. Everything except `prefix.json` is HTML, RSS, or XML.
- **robots.txt contract (fetched verbatim):** `Disallow` covers only `/admin/`, `/log/`, `/includes/`, `/comtocom.php`, `/comments.php`, `/ad/`, `/about`, `/dbchanges.php`, `/clubs.php/*`, `/_Incapsula_Resource`, `/mymessages.php`, `/sns/*/*`. **No `/anime/`, `/manga/`, `/search/`, `/rss`, or `/forum/` disallow.** `Sitemap: https://myanimelist.net/sitemap/index.xml`.
- **Compliance posture (deliberate design constraint):** the same robots.txt carries a long `User-agent`-specific `Disallow: /` block list aimed at AI-training crawlers and pure scrapers (`ClaudeBot`, `Claude-Web`, `GPTBot`, `CCBot`, `DeepSeekBot`, `Scrapy`, `Spider`, `FirecrawlAgent`, `Bytespider`, `YisouSpider`, `SleepBot`, …), with the comment "Block pure scrapers with no search referral value". The printed CLI must therefore be a **polite, user-invoked client, not a crawler**: descriptive `User-Agent`, on-disk cache with per-entity TTLs, low default concurrency (1), no crawl-all-by-default command, a real `--rate-limit`/`--delay` knob, and sitemap consumption only for explicit id enumeration.
- **Third-party mirror note:** Jikan (`api.jikan.moe/v4`) is an unofficial, community-run JSON mirror of MAL. It is *not* used as the runtime primary (it is a separate service with its own rate limits and no official blessing); it is recorded only as a fallback/annotation source.

## Top Workflows

1. **"What should I watch tonight / this season?"** — seasonal chart + broadcast day/time + filters (score floor, genre, episode count, not-already-seen).
2. **"Is this show good, or just popular?"** — score distribution, status distribution, drop rate, episode-poll consistency. (FMAB's `/stats` shows 52.9% tens against 3.1% ones — a shape that no score average can express.)
3. **"What order do I watch this franchise in?"** — related-anime graph (prequel/sequel/side story/summary/movie) turned into a watch order.
4. **"What else has this VA/studio made?"** — people/studio pages cross-referenced with score, year, and role.
5. **"Track my watching without a MAL login"** — a local library (progress, rating, notes, backlog) since the user declined an auth surface, plus a MAL-importable XML export.
6. **"Where does the anime leave off in the manga?"** — manga↔anime adaptation lookup between a title's anime and manga entries.

## Table Stakes

Everything the public site does, in scriptable form:
- instant search (`prefix.json`) and filtered search (`anime.php`/`manga.php` params)
- anime/manga detail: alt titles (JP/EN/synonyms), type, episodes/volumes/chapters, status, aired/published, premiered/broadcast, producers, licensors, studios, source, genres, themes, demographics, duration, rating, score + weighted score + scorer count, rank, popularity, members, favorites, synopsis, background, external links, streaming platforms
- characters (with VA JP/EN) and staff (roles)
- reviews (score, helpful counts, spoiler flag) and community recommendations (counts)
- rankings: top anime/manga in every `?type=` variant, currently-airing ranking
- seasonal charts with category grouping and pagination
- genre/theme/demographic browse; studio/licensor and magazine pages
- episode lists: number, EN+JP title, air date, poll average, reply count
- news + featured articles (RSS and article pages), forum boards/topics
- public user profiles; title pictures and promo videos
- `/stats`: score distribution (1–10 with vote counts) and status distribution (watching/completed/on-hold/dropped/plan-to-watch)

## Data Layer

- **Primary entities:** `anime`, `manga`, `character`, `person`, `studio` (producer/licensor), `magazine`, `episode`, `review`, `recommendation`, `news_article`, `season_entry`, plus local-only `library_entry` (status/progress/rating/notes) and `snapshot` (score, members, favorites, rank over time).
- **Sync cursor:** per-entity `fetched_at` + TTL classes — detail pages 7d, `/stats` 1d, season chart 6h, rankings 6h, news RSS 15m, `prefix.json` not cached (it is the live lookup). `stale --days N` re-fetches on demand; `--refresh` bypasses.
- **FTS/search:** SQLite FTS5 over anime/manga titles + English/Japanese/synonym alternative titles + character and person names + news headlines and bodies, so offline `search` works against everything already cached. Local `library_entry.notes` is indexed too.
- **History is the moat:** MAL publishes no score/member history anywhere. Persisting a snapshot row per title per day makes drift, momentum, and rank-change queries possible offline — a capability no existing MAL tool has.

## Codebase Intelligence

Omitted — no DeepWiki analysis was performed for this target (the service is a website, not a single repo).

## Source Priority

Single source (`myanimelist.net`); the Multi-Source Priority Gate was correctly skipped. No `source-priority.json` is written.

## Product Thesis

- **Name:** MyAnimeList CLI (`myanimelist-pp-cli`).
- **Why it should exist:** MAL is the category's largest dataset with the weakest programmatic access: no anonymous official API, no history, no CLI worth using, and community tools that either require OAuth or only sync a local file into a list. This CLI turns the whole public site into a cached, typed, agent-native dataset and then answers questions the website structurally cannot: what airs tonight **in your timezone**, how **divisive** (not just how high) a score is, how likely a show is to be **dropped**, what is **missing from the franchises you started**, how a score and its membership have **drifted** since you last looked, and how this season compares with the last one.
- **Positioning:** offline-first (`sqlite` + FTS5), JSON-everywhere, one static Go binary, zero credentials, polite by construction.

## Build Priorities

1. **Parity surface (spec-driven):** `anime`, `manga`, `season`, `ranking`, `genre`, `studio`, `magazine`, `character`, `person`, `episode`, `review`, `recommendation`, `news`, `forum`, `search`, `profile`, `stats`, `picture`, `sitemap`, `feed` resources over the verified routes, with `response_format: html|json|xml` per endpoint.
2. **Structured extraction layer:** one parser per page family (detail, stats, season, ranking, character, person, studio, review, recommendation, episode, news, forum, profile) producing typed JSON records from MAL's stable `dark_text` / `spaceit_pad` / `leftside` markup idioms.
3. **Local store:** SQLite schema + FTS5 + TTL-aware sync + snapshot history.
4. **Novel analytics:** divisiveness index, drop-risk, episode consistency, score/member drift, season-over-season diff, taste profile from the local library.
5. **Local library + export:** `track`/`progress`/`backlog`/`franchise-gap` and a MAL-compatible XML export for one-way import into MAL.
6. **Airing schedule:** broadcast day/time (JST) → local timezone "tonight"/"this week", with episode-level air dates.
7. **Agent-native polish:** `--json` on every command, `doctor`, `stale`, MCP tool surface, README + SKILL.

## Evidence Appendix

- Probe matrix: 47 URLs, status + `Content-Type` + byte size + body signature recorded (`/tmp/pp_probe/results.tsv` during the run; findings reproduced above).
- robots.txt and RSS/sitemap bodies fetched raw via `fetch-docs.sh` / curl with a Chrome UA.
- Browser capture logs: `browser-use` DOM + Performance-API resource enumeration on three page types; challenge field `false`.
- Detail-page markup verified for parseability: `dark_text` labels (`Japanese:`, `English:`, `Type:`, `Episodes:`, `Status:`, `Aired:`, `Premiered:`, `Broadcast:`, `Producers:`, `Licensors:`, `Studios:`, `Source:`, `Genres:`, `Themes:`, `Duration:`, `Rating:`, `Score:`, `Ranked:`, `Popularity:`, `Members:`) inside `.leftside`; `span.score-label`; episode table rows with `data-sort-key="episode-aired"`; `/stats` "Summary Stats" + "Score Stats" + "Recently Updated By" blocks.
- Slug-ignored routing verified: `GET /anime/1/Fullmetal_Alchemist_Brotherhood` → Cowboy Bebop page.
- `probe-reachability` raw JSON captured, including the `recaptcha` false-positive evidence, for the record.

## Top Competitors (Phase 1 finding, verified via repo/README inspection)

| Tool | Type | Stars | State | What it covers | What it cannot do |
|---|---|---|---|---|---|
| [`z411/trackma`](https://github.com/z411/trackma) (ex-wMAL) | Python list manager (TUI) | — | maintained | Multi-site list sync: MAL, AniList, Kitsu, Shikimori; episode progress; "play next" | Requires OAuth login; no stats/analytics, no schedule, no franchise order, no offline dataset |
| [`cyanheads/anime-mcp-server`](https://github.com/cyanheads/anime-mcp-server) | MCP server (TypeScript) | 1 | active (2026-08) | 8 tools: search, detail, **relations → suggested watch order**, **schedule with countdown**, characters, recommendations, rankings, studio filmography | AniList-primary with Jikan fallback — **does not use myanimelist.net at all**; no scores distributions, no drop/status data, no local store, no offline search |
| [`brunolm/mal-mcp`](https://github.com/brunolm/mal-mcp) | MCP server (TypeScript) | 1 | active (2026-04) | MAL **v2 OAuth API** surface: search, details, rankings, seasonal, user lists, list mutations | Needs a client id + OAuth for most value; no HTML-only data (stats/reviews/recs/episodes/news/forum), no local store |
| [`ryukinix/mal`](https://github.com/ryukinix/mal) | Python CLI | 110 | **archived, BROKEN** ("BLAME MyAnimeList") | Scraped MAL for list + search | Dead: MAL markup/auth changes killed it — proof that website-derived MAL tooling needs a maintained parser layer |
| [`Wraient/curd`](https://github.com/Wraient/curd) | Go CLI | 324 | active (2026-08) | Play anime in the terminal; AniList auth + optional MAL integration + Discord RPC | Playback, not data; no metadata analytics, no offline dataset |
| [`sdaqo/anipy-cli`](https://github.com/sdaqo/anipy-cli) | Python CLI | 515 | active (2026-08) | Watch/download anime in terminal, reusable API | Streaming links only; no MAL data model |
| [`pystardust/ani-cli`](https://github.com/pystardust/ani-cli) | Shell CLI | 13.7k | active | Stream anime | No metadata, no tracking, no analytics |
| [`AdityaJ7/myanimelist-cli`](https://github.com/AdityaJ7/myanimelist-cli) | C CLI | 1 | stale (2023) | Minimal user-anime lookup | Single-purpose, unmaintained |
| [`zun43d/mal-lookup`](https://www.claudemarket.ai/skills/zun43d/mal-lookup) | Claude skill | — | — | Agent-side MAL lookups | Skill wrapper, not a CLI/dataset |

**Read of the market:** every existing tool either (a) needs OAuth and therefore covers only the official v2 surface, or (b) is a playback/search client with no persistence. **Nothing uses the anonymous website surface as a full dataset, nothing stores history, and the best-developed MAL CLI is archived as broken.** That is the opening.

## User Pain Points (grounding for transcendence features)

1. **"My CLI broke because MAL changed."** `ryukinix/mal` is archived with "BROKEN: BLAME MyAnimeList" — website-derived MAL tooling needs a *maintained* extraction layer, not one-off regexes. → the CLI must own a tested parser per page family, plus an offline cache so breakage degrades gracefully instead of failing.
2. **"Is it actually good, or just popular?"** MAL gives one average score. Power users know a 8.5 with 50% tens and 3% ones is a different experience from a flat 8.5. The site *has* the distribution — but only as a rendered bar chart on `/stats`, unusable in a pipeline/none of the tools expose it. → divisiveness/polarity analytics.
3. **"What airs tonight for me?"** MAL publishes broadcast times in JST only; there is no per-user timezone view, and no CLI/MCP exposes episode-level air dates at all. → timezone-correct schedule from broadcast + episode tables.
4. **"What order do I watch this in?"** Relations exist per-title, but no website view flattens the franchise graph; only the AniList-based MCP attempts this. → franchise traversal that names each relation and produces a watch order.
5. **"I have 400 plan-to-watch titles and no idea where to start."** No tool uses drop-rate/status distributions to predict what a user will actually finish. → drop-risk scoring and starter picks.
6. **"Does the anime cover the manga?"** Answering this requires joining a title's anime and manga entries and comparing episode/chapter counts — nothing does it.
7. **"Did the score move?"** MAL has *no* history surface anywhere, and no tool records it. → local snapshot drift.
