# Movie CLI Absorb Manifest

## Sources Analyzed
1. **tmdb-mcp** (XDwanj) — MCP server: search, details, discover, trending, recommendations
2. **imdb-mcp-server** (uzaysozen) — MCP server: search, details, top 250, popular, upcoming, cast/directors, regional
3. **mcp-server-imdb** (mrbourne777) — MCP server: search, poster, trailer (FM-DB API, no auth)
4. **tmdb-cli** (degerahmet, Go) — CLI: popular, top-rated, upcoming, now-playing
5. **tmdb-cli** (letsmakecakes, Go) — CLI: popular, top-rated, upcoming, now-playing
6. **tmdb-cli** (che1nov, Go) — CLI: popular, top-rated, upcoming, now-playing + Redis cache
7. **TMDB_CLI** (illegalbyte, Python) — CLI: movie/TV by ID, IMDB↔TMDB ID convert, JSON output
8. **tmdb-cli** (bhantsi) — CLI: search, trending, upcoming, movie details
9. **mediascore** (dkorunic, Go, archived) — CLI: multi-source ratings from OMDb/IMDB/RT/Metacritic
10. **BurntSushi/imdb-rename** (Rust) — Fuzzy search via BM25 across IMDb datasets
11. **tmdbv3api** (Python wrapper) — popular, details, search, similar, recommendations, discover, season
12. **TMDB-Trakt-Syncer** (Python) — Watchlist/ratings sync between TMDb and Trakt

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search movies by title | tmdb-mcp search | `movie search <query>` | Multi-type (movie/TV/person), --type filter, --year filter, --json |
| 2 | Get movie details by ID | TMDB_CLI --movie | `movie get <id>` | Rich card display: rating, cast, where to watch, trailer link. --json, --select |
| 3 | Get TV show details | TMDB_CLI --television | `movie tv <id>` | Season/episode count, status, networks, next air date. --json |
| 4 | Get person details | tmdbv3api Person.details | `movie person <id-or-name>` | Full bio + filmography inline. --json |
| 5 | Popular movies | tmdb-cli popular | `movie popular` | Movies AND TV, --type filter, --page, --json |
| 6 | Top-rated movies | tmdb-cli top-rated | `movie top-rated` | Movies AND TV, --type filter, --genre, --json |
| 7 | Upcoming movies | tmdb-cli upcoming | `movie upcoming` | With streaming date info where available. --json |
| 8 | Now playing in theaters | tmdb-cli now-playing | `movie now-playing` | With runtime and certification. --json |
| 9 | Trending content | tmdb-mcp get_trending | `movie trending` | Daily or weekly, movies/TV/people/all. --window day\|week, --type |
| 10 | Discover movies with filters | tmdb-mcp discover_movies | `movie discover` | Genre, year, rating, votes, certification, cast, crew, keywords, providers. --json |
| 11 | Discover TV with filters | tmdb-mcp discover_tv | `movie discover --type tv` | Same rich filtering for TV shows |
| 12 | Recommendations | tmdb-mcp get_recommendations | `movie recommend <id>` | For any movie or TV show. --json |
| 13 | Similar titles | tmdbv3api similar | `movie similar <id>` | For any movie or TV show. --json |
| 14 | Watch providers | TMDb API | `movie watch <id>` | Stream/rent/buy grouped by provider with region. --region flag |
| 15 | Cast & crew | imdb-mcp-server get_cast | `movie credits <id>` | Full cast + crew, sorted by billing order. --role filter |
| 16 | Episode guide | tmdbv3api Season.details | `movie episodes <tv-id> [--season N]` | All seasons or specific, with ratings per episode |
| 17 | IMDB↔TMDB ID convert | TMDB_CLI --imdbidconvert | `movie id-convert <id>` | Auto-detect IMDB (tt*) vs TMDB format |
| 18 | Poster/image URLs | mcp-server-imdb poster | `movie images <id>` | Posters, backdrops, profiles. --type filter |
| 19 | Trailer/video URLs | mcp-server-imdb trailer | `movie videos <id>` | Trailers, teasers, clips, featurettes. YouTube links |
| 20 | JSON output | TMDB_CLI --json | `--json` on every command | Global flag, valid JSON, pipeable to jq |
| 21 | Genre listing | TMDb API | `movie genres` | Movie + TV genres with IDs. --json |
| 22 | On the air (TV) | TMDb API | `movie on-the-air` | Currently airing TV shows |
| 23 | Airing today (TV) | TMDb API | `movie airing-today` | TV shows with episodes airing today |
| 24 | Popular people | TMDb API | `movie popular --type person` | Popular actors/directors |

## Transcendence (only possible with our compound approach)

Per Phase 1.5c.5 novel-features subagent. Reprint reconciliation against prior `novel_features`: 4 keep, 1 reframe (`get` → `ratings` because absorb #2 owns `movie get`), 1 drop (`watch` is now table-stakes absorb #14). 3 net-new survivors.

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Tonight Picker | `movie tonight [--mood <genre>] [--max-runtime N] [--providers <csv>] [--region US]` | 9/10 | Maya | TMDb `/trending` + `/discover` + per-title `/watch/providers` filtered to user's `--providers` set; mechanical rank by composite rating. |
| 2 | Multi-Source Ratings Card | `movie ratings <id>` | 9/10 | Maya, Priya | TMDb `/movie/{id}` or `/tv/{id}` + OMDb `?i=<imdbid>`; rendered as a single card. Graceful degradation when OMDB_API_KEY unset. |
| 3 | Marathon Planner | `movie marathon <title-or-collection-id> [--order release\|inuniverse] [--breaks-every N]` | 8/10 | Devon | Resolves title→`belongs_to_collection`→`/collection/{id}`, normalizes order, sums runtimes from per-title TMDb details, inserts breakpoints every N minutes. |
| 4 | Career Timeline | `movie career <person-id-or-name> [--since YEAR] [--role actor\|director\|dp]` | 8/10 | Priya | TMDb `/person/{id}/combined_credits` + per-title detail fan-out for ratings + OMDb (when key set) via IMDb ID; chronological. |
| 5 | Watchlist | `movie watchlist add/list/remove [--available --providers <csv> --region US]` | 8/10 | Sam | SQLite `watchlist(id, kind, title, added_at)`; `list --available` re-checks `/watch/providers` per row, filters to `--providers`, emits flagged rows. |
| 6 | Head-to-Head Versus | `movie versus <id-a> <id-b> [--region US]` | 7/10 | Maya | Composes Ratings Card across two titles + cast-name intersection from `/credits` + provider lists per region; aligned columnar output. |
| 7 | Recommendation Queue | `movie queue [--limit 20] [--providers <csv> --region US]` | 7/10 | Sam, Maya | For every row in local watchlist: union `/recommendations` + `/similar`, dedupe by id, rank by TMDb vote_average × log(vote_count). |
| 8 | Recurring Collaborators | `movie collaborators <person> [--min-count 2] [--role actor\|crew]` | 6/10 | Priya | TMDb `/person/{id}/combined_credits` + per-title `/credits`, group-by name + count, filters by `--min-count`. |

### Reprint drops

The previous run's `Where to Watch` (`watch`) novel feature is now table-stakes (absorb #14). Phase 1.5 gate review may reinstate it as a transcendence row if the user prefers — surfaced here per the reprint reconciliation rule.
