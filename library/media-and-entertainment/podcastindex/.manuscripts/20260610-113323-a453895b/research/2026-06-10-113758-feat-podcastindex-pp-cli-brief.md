# PodcastIndex CLI Brief

## API Identity
- Domain: PodcastIndex.org API v1.12.1 (OpenAPI 3.0.2). Base `https://api.podcastindex.org/api/1.0`.
- Users: podcast app developers, podcast researchers, value4value/Lightning tooling, agents needing podcast discovery + episode metadata.
- Data profile: feeds (podcasts) and episodes — large, stable, richly-typed JSON. Uniform envelope `{status, feeds|items|episodes, count, query, description}`.

## Reachability Risk
- None. Official, documented, free, high-availability API. Auth confirmed HTTP 200 by user.
- Probe-safe endpoint: `GET /stats/current` (no required params).

## Auth (the load-bearing decision)
- 4 computed request headers per call:
  - `User-Agent: <cli>/<version>`
  - `X-Auth-Key: $PODCASTINDEX_KEY`
  - `X-Auth-Date: <unix-now>`
  - `Authorization: sha1hex($PODCASTINDEX_KEY + $PODCASTINDEX_SECRET + <unix-now>)`
- The generator's `composed` auth emits a *verbatim static* `AuthHeaderVal` and cannot compute per-request date+SHA1. **Therefore the signer is hand-authored** (`internal/podcastindex/signer.go`) and the user-facing commands are built as **novel commands on a signed sibling client** (`internal/podcastindex/client.go` → `Get(ctx, path, params)`), not on the generator's raw endpoint-mirror commands (which cannot sign).
- Creds present: KEY (len 20), SECRET (len 40) in `~/.zshrc`. Read-only API; no mutation risk for core scope.

## Top Workflows
1. Search podcasts by term → `GET /search/byterm?q=&max=&clean=&fulltext=&similar=`
2. Search episodes by person → `GET /search/byperson?q=&max=&fulltext=`
3. Resolve a show to its episodes → `GET /episodes/byfeedid?id=&max=&since=&newest=` (feedId from search or lookup)
4. Look up a podcast by feedId / guid / iTunes id / feedUrl → `GET /podcasts/by*`
5. Discover: trending, recent episodes/feeds, by category/tag/medium.

## Table Stakes (mirror the SDK wrappers — all 50 endpoints exist as functions in Node/Python/Swift/Kotlin/Ruby/C#/Go wrappers)
- search byterm/byperson/bytitle/music-byterm
- podcasts byfeedid/byguid/byfeedurl/byitunesid, trending, bytag, bymedium, dead
- episodes byfeedid/byid/byguid/byitunesid/bypodcastguid, random, live
- recent feeds/episodes/newfeeds/soundbites/data
- categories list; stats current; value (value4value/Lightning) by feed/episode/guid

## Data Layer
- Primary entities: `feeds` (podcast shows, PK feedId), `episodes` (PK episodeId, FK feedId).
- Sync cursor: `since`/`before` unix timestamps on recent + episodes endpoints.
- FTS/search: offline FTS over synced feed titles/authors/descriptions and episode titles/descriptions — the headline differentiator no wrapper offers.

## Why This CLI Instead Of The Incumbents
Every existing tool is a language-specific SDK that mirrors the endpoints and returns raw JSON in-process. None offers: a single binary, an offline SQLite mirror, full-text search over synced data, `--json/--select/--csv/--compact` agent-native output, `--dry-run`, typed exit codes, an MCP server, or compound commands that chain search→resolve→episodes in one call. That is the entire GOAT gap.

## Product Thesis
- Name: PodcastIndex CLI (`podcastindex-pp-cli`)
- Thesis: the only PodcastIndex tool that is agent-native and works offline — search, resolve, and catch up on any podcast from one signed binary, with a local mirror you can FTS and SQL.

## User Vision (from briefing)
Creds set + verified (SHA1 auth proven). Must-haves: search by term, search by person, resolve show → episodes. Everything else is parity + transcendence on top.

## Build Priorities
1. Hand-authored SHA1 signer + signed sibling client (unblocks everything).
2. Core 3: `search term`, `search person`, `episodes by-feed`.
3. Lookup + discovery parity (podcasts by*, trending, recent, categories, stats, value).
4. Offline store + sync + FTS search + SQL.
5. Transcendence compound commands.
