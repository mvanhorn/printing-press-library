# MyAnimeList CLI — Absorb Manifest

Run: `20260910-215529-36659c2f` · slug `myanimelist` · binary `myanimelist-pp-cli`
Source: the public **myanimelist.net** website, **anonymous-only** (no OAuth, no cookie, no API key).
Spec: internal YAML, 38 typed endpoints across 15 resources (`research/myanimelist-spec.yaml`).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Instant/title search across anime, manga, characters, people | brunolm/mal-mcp `search_anime`/`search_manga`; trackma search | `(generated endpoint) instant search` + `(generated endpoint) anime list --query` / `manga list --query` + framework `search` | Zero-credential; the site's own suggest JSON *and* the full advanced-filter search; FTS5 over everything cached; `--json`/`--select` |
| 2 | Full anime detail | brunolm/mal-mcp `get_anime_details`; cyanheads `anime_get_media` | `(behavior in myanimelist-pp-cli anime show) parses the detail page into a typed record; `anime get` remains the raw page plumbing` | Fields no MCP exposes: premiered season, broadcast slot, licensors, streaming platforms, themes, demographics, favorites, weighted score, external links |
| 3 | Full manga detail | brunolm/mal-mcp `get_manga_details` | `(behavior in myanimelist-pp-cli manga show) parses the detail page into a typed record` | Volumes, chapters, serialization magazine, authors, published window |
| 4 | Anime rankings (all/airing/upcoming/tv/ova/ona/special/bypopularity/favorite) | brunolm/mal-mcp `get_anime_ranking`; cyanheads `anime_get_rankings` | `(generated endpoint) ranking anime` | Every site `?type=` variant plus limit/page control |
| 5 | Manga rankings (manga/novels/lightnovels/oneshots/doujin/manhwa/manhua/bypopularity/favorite) | brunolm/mal-mcp `get_manga_ranking` | `(generated endpoint) ranking manga` | Full site variant set |
| 6 | Seasonal anime by year+season | brunolm/mal-mcp `get_seasonal_anime`; cyanheads `anime_get_schedule` | `(behavior in myanimelist-pp-cli season list) parses the chart into typed per-format records` / `(generated endpoint) season current` | Parsed by format category (TV/ONA/OVA/Movie/Special) with pagination, no client id |
| 7 | Airing schedule with upcoming-episode window | cyanheads `anime_get_schedule` (upcoming mode) | `myanimelist-pp-cli airing --days N` | MAL-native: broadcast slot (JST) + episode-level air dates, converted to the user's timezone |
| 8 | Characters + voice actors for a title | brunolm/mal-mcp; cyanheads `anime_find_characters` | `(behavior in myanimelist-pp-cli anime characters) parses cast rows with role and JP/EN voice actors` | Role (main/supporting) + JP/EN VA split with favorites counts, cached |
| 9 | Character or person lookup | cyanheads `anime_find_characters`; brunolm/mal-mcp | `(behavior in myanimelist-pp-cli person) parses animeography plus staff credits` / `(generated endpoint) character get` | Full animeography with role type plus staff credits — absent from every MAL MCP |
| 10 | Staff/crew for a title | (none) | `(generated endpoint) anime staff` | No existing tool exposes MAL staff credits |
| 11 | Recommendations for a title | cyanheads `anime_get_recommendations` | `(behavior in myanimelist-pp-cli anime recommendations) parses typed recommendation rows with counts` | MAL's own community graph with recommendation counts, not a mirror |
| 12 | Reviews with scores | (none) | `(behavior in myanimelist-pp-cli anime reviews) parses typed review records` | Typed review records: score, helpful counts, spoiler flag, reviewer |
| 13 | Studio filmography | cyanheads `anime_get_studio` | `(behavior in myanimelist-pp-cli studio) parses the filmography into typed ranked rows` | MAL producer/studio pages (also licensors), sortable, cached |
| 14 | Episode list | (none) | `(behavior in myanimelist-pp-cli anime episodes) parses the episode table into typed rows` | Episode number, EN+JP title, air date, poll average, reply count |
| 15 | Statistics for a title | (none) | `(behavior in myanimelist-pp-cli anime stats) and (behavior in myanimelist-pp-cli manga stats) parse both distributions into typed JSON` | 1–10 score distribution with vote counts + full status breakdown |
| 16 | Filtered search (type, score, status, year range, genre, sort) | brunolm/mal-mcp search (title only) | `(generated endpoint) anime list` / `manga list` filter flags | The site's advanced-search parameter set as real flags with enums |
| 17 | Genre / theme / demographic browse | (none) | `(generated endpoint) genre anime` / `genre manga` | Themes and demographics share the route |
| 18 | Magazine browse | (none) | `(generated endpoint) magazine get` | Serialization magazines the official API does not model |
| 19 | News feed + article reading | (none) | `(generated endpoint) news list` / `(generated endpoint) news get` / `(generated endpoint) feed news` | RSS + full articles, offline-searchable |
| 20 | Featured articles | (none) | `(generated endpoint) feed featured` | From the featured RSS feed |
| 21 | Forum boards and topics | (none) | `(generated endpoint) forum boards` / `forum topics` / `forum topic` | Public discussion surface no MAL tool covers |
| 22 | Public user profile | brunolm/mal-mcp `get_current_user` (OAuth only) | `(generated endpoint) member get` | Works with no credential for public profiles |
| 23 | Title pictures / promo videos | (none) | `(generated endpoint) anime pictures` / `anime videos` | Gallery routes |
| 24 | ID enumeration / catalog dump | (none) | `(generated endpoint) sitemap index` / `(generated endpoint) sitemap shard` | robots-advertised sitemap for polite explicit enumeration |
| 25 | "Play next" / continue watching | trackma "play next"; curd `-c` | `myanimelist-pp-cli next` | No account: next episode from the local library with filters |
| 26 | Local list management (status, score, progress, notes) | trackma; brunolm/mal-mcp (server mutations) | `myanimelist-pp-cli track add\|progress\|rate\|list\|drop\|note` | Fully local — no OAuth, offline, FTS over notes |
| 27 | One-way list export into MAL | trackma/curd (direct sync) | `myanimelist-pp-cli export --format mal-xml` | Emits MAL's import XML; no token ever held |
| 28 | Personalized suggestions | brunolm/mal-mcp `get_anime_suggestions` (OAuth) | `myanimelist-pp-cli suggest` | Eligibility/scoring from the local library + public stats, no account |
| 29 | Franchise relation graph → watch order | cyanheads `anime_get_relations` | `myanimelist-pp-cli watch-order` | MAL's own relation graph with every edge labelled and ordered |
| 30 | Watch/read the title (playback) | curd; anipy-cli; ani-cli | *(not absorbed — out of scope)* | Deliberate: this is a data/tracking CLI; playback belongs to those tools |
| 31 | Multi-site aggregation (AniList/Kitsu/Shikimori) | trackma; cyanheads | *(not absorbed — out of scope)* | User chose myanimelist.net as the single source |
| 32 | Agent-facing typed tool surface | cyanheads; brunolm/mal-mcp (MCP) | `(behavior in myanimelist-pp-cli agent-context) the agent surface is a generated bundled MCP server, not a CLI subcommand` | Every user-facing command becomes an MCP tool automatically |

## Transcendence (only possible with our approach)

Produced by the Phase 1.5c.5 brainstorm subagent (customer model → 15 candidates → adversarial cut to 7). Full audit trail, including the eight killed candidates, is in `research/<stamp>-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|---------|--------------|------------------------|------------------|
| 1 | Divisiveness index | `anime divisive` | 10/10 | Dev, Kenji | hand-code | Requires parsing the `/stats` 1–10 vote bars the site only renders as a chart, then computing polarization (9–10 vs 1–2 share + weighted spread) — no MAL tool exposes distributions at all | Use this command for the shape of a title's 1–10 score distribution (polarization, love-it/hate-it share). Do NOT use this command for whether a title's score is rising or falling; use 'drift' instead. Do NOT use this command for the raw vote table; use 'anime stats' instead. |
| 2 | Reception drift and movers | `drift` | 9/10 | Kenji, Dev | hand-code | Requires local SQLite snapshots over time; MAL publishes no score/member history anywhere, so only a local store can diff it | Use this command for how a title's score, members, favorites, or rank changed over time from local snapshots. Do NOT use this command for the static shape of the current score distribution; use 'anime divisive' instead. |
| 3 | Weekly viewing grid | `week` | 9/10 | Marisol | hand-code | Requires intersecting the local library with cached JST broadcast slots plus episode air dates and converting to the user's timezone, flagging same-hour collisions | Use this command for a timezone-correct weekly viewing grid built from titles in your local library, including broadcast-slot collisions. Do NOT use this command for a flat list of everything airing in the next N days; use 'airing' instead. |
| 4 | Drop-risk | `anime drop-risk` | 8/10 | Dev | hand-code | Requires the `/stats` status distribution (dropped/on-hold share) joined with episode count — invisible on the detail page and absent from every tool | Use this command for how likely MyAnimeList users are to abandon a title (dropped/on-hold share from the status distribution). Do NOT use this command for how divisive its scores are; use 'anime divisive' instead. Do NOT use this command for raw status counts; use 'anime stats' instead. |
| 5 | Episode reception curve | `anime consistency` | 8/10 | Marisol, Dev | hand-code | Requires the episode table's poll averages and reply counts, then per-episode deltas and a first-3 vs last-3 slope; no tool surfaces episode-level data | Use this command for episode-by-episode reception (poll averages, reply counts, late-season drop-off). Do NOT use this command for the raw episode list; use 'anime episodes' instead. Do NOT use this command for the overall score distribution; use 'anime divisive' instead. |
| 6 | Source coverage | `adaptation` | 7/10 | Priya | hand-code | Requires joining a cached anime entry (episodes, airing status) with its related manga entry (chapters, volumes, publication status) — a cross-entity join the site never presents | Use this command for whether an anime's adaptation covered its source manga and how much source remains. Do NOT use this command for other entries in the same franchise (sequels, movies, side stories); use 'franchise gap' instead. |
| 7 | Franchise gap | `franchise gap` | 7/10 | Priya | hand-code | Requires traversing relation edges for every franchise touched by the local library and diffing against library status — a local-only join | Use this command for entries missing from franchises you have already started in your local library. Do NOT use this command for the ordering of franchise entries; use 'watch-order' instead. Do NOT use this command for anime-to-manga source coverage; use 'adaptation' instead. |

**Hand-code commitment:** 7 of 7 transcendence rows are `hand-code` (0 are `spec-emits`). In addition, the absorbed rows above require typed parsers behind `anime show`, `manga show`, `anime cast`, `anime episodes`, `anime stats`, `season show`, `person credits`, `studio filmography`, plus the local-library commands (`track`, `next`, `suggest`, `export`) and `airing`/`watch-order`. Those typed/local commands are the Phase 3 build scope; the 38 generated endpoint commands ship from the spec.

## Stubs

None. Every approved feature ships implemented — nothing in this manifest is a placeholder.

## Risks the user should know before approving

- **HTML parsing is the load-bearing risk.** MAL is server-rendered with no JSON-LD and no embedded state, so every typed command depends on the site's markup idioms (`dark_text` labels, `.leftside`, `span.score-label`, the episode table's `data-sort-key`). A MAL redesign breaks parsers — this is exactly how `ryukinix/mal` died. Mitigation: one parser package with table-driven tests over captured fixtures, plus graceful degradation (the raw-HTML endpoint command still works when a parser fails).
- **Politeness is enforced in the client.** robots.txt blocks AI-training crawlers and pure scrapers by name. The CLI sends a descriptive UA, defaults to 1 request/second with a cache TTL, and has no crawl-everything command; sitemap consumption is explicit and paginated.
- **Anonymous scope means no MAL-side writes.** Local tracking + MAL-import XML export is the bridge; there is no list sync, by the user's own choice at the auth gate.
- **`adaptation` reports a coverage band, not a chapter number.** MAL does not store where an anime stopped in its source; the command states its estimate as a band and never fabricates precision.
