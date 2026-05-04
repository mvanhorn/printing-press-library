# Movie CLI Brief

## API Identity
- Domain: Entertainment metadata — movies, TV shows, people, ratings, streaming availability
- Users: Movie enthusiasts, developers, cinephiles, anyone choosing what to watch
- Data profile: Dual-API — TMDb (primary, 900K+ movies) + OMDb (enrichment, RT/Metacritic ratings)

## User Vision
Multi-source "movie goat" CLI powered by TMDb + OMDb. TMDb for discovery, streaming info, trending, credits. OMDb for Rotten Tomatoes, Metacritic, and IMDb ratings aggregation. Local SQLite cache for personal data only — not bulk imports.

## Data Sources

### Primary: TMDb REST API v3
- Base URL: https://api.themoviedb.org/3
- Auth: Bearer token via TMDB_API_KEY
- Rate limit: ~40 req/10s
- Endpoint groups: search, movies, TV, people, discover, trending, watch providers, genres, configuration, images, videos, credits, recommendations, similar, collections

### Enrichment: OMDb REST API
- Base URL: https://www.omdbapi.com/
- Auth: API key via OMDB_API_KEY (free tier: 1000 req/day)
- Unique data not in TMDb: **Rotten Tomatoes scores, Metacritic scores, Awards text, IMDb ratings** (the canonical ones from imdb.com)
- Graceful degradation: if no OMDB_API_KEY set, show TMDb rating only

### Cross-Source Architecture
- TMDb is the engine: all search, discovery, streaming, trending, credits, images
- OMDb is the enricher: when displaying a title, optionally fetch RT/Metacritic/Awards
- Match via IMDb ID: TMDb provides imdb_id on movie/tv details → OMDb accepts IMDb IDs
- Local SQLite: watchlist, favorites, ratings, search history, cached lookups

## Reachability Risk
- **None** — Both APIs stable, well-documented, free tiers sufficient for CLI use.

## Top Workflows
1. **Search & discover**: Rich filtering by genre, year, rating, cast, streaming provider
2. **Deep dive**: Full details + aggregated ratings from 4 sources + where to watch
3. **Filmography**: Actor/director complete career with ratings
4. **Where to watch**: Streaming/rent/buy for any title
5. **Trending & upcoming**: What's hot, in theaters, coming soon

## Table Stakes
- Search movies/TV/people by title
- Title details with cast, crew, ratings
- Popular / top-rated / upcoming / now-playing / on-the-air / airing-today
- Discover with rich filters (genre, year, rating, cast, provider)
- Recommendations and similar titles
- Watch providers / streaming availability
- Trending (daily/weekly)
- Episode guide for TV
- Person filmography
- Multi-source ratings (TMDb + IMDb + RT + Metacritic)
- Images and trailers
- --json output, --select field filtering

## Data Layer
- Primary entities: movies, tv_shows, people, genres (all from TMDb API on demand)
- Local SQLite: watchlist, favorites, personal ratings, search history, cached title details
- FTS: on cached titles for quick re-lookup
- NO bulk dataset import

## Product Thesis
- Name: movie-pp-cli
- Why: The only CLI that combines TMDb's rich discovery engine with OMDb's multi-source ratings. One command shows TMDb rating + IMDb rating + Rotten Tomatoes + Metacritic + where to watch. Current TMDb CLIs are demos. OMDb CLIs are search-only. Nobody combines them.

## Build Priorities
1. TMDb API — search, details, discover, trending, credits, watch providers
2. OMDb enrichment — RT, Metacritic, IMDb ratings, awards (optional, graceful degradation)
3. Local SQLite — watchlist, favorites, search history
4. Smart output — aggregated ratings card, streaming provider grouping
5. Transcendence — career timelines, tonight picker, versus, marathon planner

---

## Revalidation Pass (printing-press 1.3.2 → 3.8.0)

Triggered by Phase 0 version-bump prompt. The prior brief is reused verbatim above; this section captures targeted machine-delta checks.

### Reachability re-probe
- TMDb (`/3/configuration?api_key=…`): **200 OK** with provided v3 key (32-char hex).
- OMDb (`/?apikey=…&i=tt0111161`): **200 OK** with provided key.

### Auth correction
- Prior spec declared `auth.type: bearer_token, format: "Bearer %s"`. v3 TMDb keys are **rejected** with 401 when sent as a Bearer header (probed: returns `status_code: 7, "Invalid API key: You must be granted a valid key."`).
- v3 keys MUST be sent as the `api_key` query parameter. The prior CLI's `client.go` worked around this with a length-check hack (`< 64` chars → query, else Bearer).
- For this reprint the spec auth block becomes:
  ```yaml
  auth:
    type: api_key
    in: query
    header: api_key
    env_vars: [TMDB_API_KEY]
    key_url: https://www.themoviedb.org/settings/api
  ```
  This eliminates the post-generation client.go patch.

### MCP surface tier (new pre-generation enrichment)
- Typed endpoints in spec: 25
- Cobratree-walked tools at runtime: ~13 framework + 6 novel ≈ 19
- **Total estimated MCP tool count: ~44** — in the 30-50 ask-the-user band (per Phase 2 Pre-Generation MCP Enrichment).
- Recommendation: opt into `mcp.transport: [stdio, http]` for remote agent reach. Endpoint count is below the >50 threshold that triggers code orchestration; intents could help if multi-step workflows (career, marathon, versus) score well in the absorb gate.

### Scoring rubrics
- Multi-source CLI pattern still classifies cleanly under v3.8.0 (`internal/source/<name>/` rate-limit rule applies to OMDb client).
- Cobratree tools now count toward MCP token scoring (#552); prior brief's 6 novel commands remain a positive contribution.

### Discovery
- TMDb has well-documented OpenAPI specs (vendor + community). Phase 1.7 browser-sniff and 1.8 crowd-sniff both eligible to skip silently — current internal YAML spec covers the documented surface.

### Build priorities — unchanged
The prior brief's priorities (TMDb engine → OMDb enrichment → SQLite → smart output → 5 transcendence commands) remain valid. The reprint adds: corrected auth shape, opt-in HTTP MCP transport, and any current-machine quality patches (NoCache=true defaults, codeOrch stopword filter, narrative validation).
