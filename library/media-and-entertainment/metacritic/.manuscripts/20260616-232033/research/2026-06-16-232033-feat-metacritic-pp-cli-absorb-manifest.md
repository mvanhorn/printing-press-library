# Metacritic CLI Absorb Manifest

## Source Tools Cataloged

| Tool | Type | Stars | Coverage | Notes |
|------|------|-------|----------|-------|
| melroy89/metacritic_api | PHP library | 83 | Games only | Most-starred, but single-medium and not a CLI |
| chrismichaelps/metacritic | TS library (npm `unofficial-metacritic`) | 67 | Games, movies, TV, music | Library, no terminal binary |
| metacritic-ts (npm) | TS library | 0 | Search + detail + sentiment | Active downloads, library only |
| gofurry/metacritic-harvester | Go CLI | 3 | Bulk archive | The ONLY existing Go CLI — a batch archiver, not a quick-lookup tool |
| MrCherry/mcp-metacritic | MCP server (TS) | 0 | Thin wrapper | Only MCP surface in the field; 0 stars, thin |

**No official Metacritic API exists.** Every tool above reverse-engineers the site.

## The Gap
There is no credible terminal-native, cross-media, quick-lookup Metacritic CLI. The closest Go tool is a bulk archiver; the only MCP server is a 0-star thin wrapper. This entry fills that gap from one host (`backend.metacritic.com`) with agent-native output.

## Absorbed (match or beat what exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse / rank by Metascore | melroy89 (games only) | `finder browse-titles` | All three media, sortable, paginated, `--json --select` |
| 2 | Cross-media search | chrismichaelps/metacritic | `finder search-titles` | One query over games/movies/TV/people |
| 3 | Filter facets | (none expose this) | `finder list-filters` | Genres/platforms/networks/product-types per medium |
| 4 | Title detail + scores | metacritic-ts | `composer` | Full composed payload: Metascore, user score, summary, release |
| 5 | Critic reviews | metacritic-ts (sentiment) | `reviews list-critic` | Per-title critic list with publication + score |
| 6 | User reviews | metacritic-ts | `reviews list-user` | Per-title user review list |
| 7 | MCP surface | MrCherry/mcp-metacritic (0★ thin) | `metacritic-pp-mcp` | 6 tools mirroring the Cobra tree at runtime, not a hand-thin wrapper |
| 8 | Health / diagnostics | (none) | `doctor` | API reachability + config validation |
| 9 | Music (albums) | chrismichaelps (music) | **Out of scope** | No `backend.metacritic.com` JSON surface; legacy HTML behind Cloudflare |

## Transcendence (local data layer — scaffolded)

| # | Feature | Command | Status | Note |
|---|---------|---------|--------|------|
| 1 | Full-text title search | `search` | Scaffolded | FTS5 over synced titles; needs `defaultSyncResources` wired to populate |
| 2 | Local analytics | `analytics` | Scaffolded | Group-by/count over synced rows; same dependency on sync |
| 3 | Compound workflows | `workflow` | Present | Chains browse to detail to reviews in one call |
| 4 | Offline sync | `sync` | No-op (gap) | `defaultSyncResources` empty — tracked as the primary follow-up |

**Honest scope note:** the absorb layer (rows 1-8) is complete and live-verified. The transcend layer (sync/search/analytics) is generator-scaffolded but not yet wired to a default population path, so it is a follow-up rather than a shipped capability. This is disclosed rather than hidden.

## Build Plan
- **Priority 0 — Absorb (DONE):** browse, search, filters, detail, critic + user reviews across games/movies/TV
- **Priority 1 — Agent-native (DONE):** `--json`/`--select` everywhere, doctor, agent-context, MCP server
- **Priority 2 — Transcend (FOLLOW-UP):** wire `defaultSyncResources` so sync populates SQLite and search/analytics become live
- **Priority 3 — Polish (FOLLOW-UP):** replace generic analytics examples with Metacritic-specific ones
