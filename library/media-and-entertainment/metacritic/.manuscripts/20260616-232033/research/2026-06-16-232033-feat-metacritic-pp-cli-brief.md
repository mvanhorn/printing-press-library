# Metacritic CLI Brief

## API Identity
- **Domain:** Critic and user review aggregation (Metascore, user score, summaries, release data, reviews) for games, movies, and TV
- **Users:** Media fans who live in the terminal, developers building review integrations, agents that need structured score lookups
- **Data profile:** Read-only, no auth, single first-party JSON backend, moderate volume, HIGH cross-media lookup need
- **Base URL:** `https://backend.metacritic.com`
- **Auth:** None. A shared public `apiKey` is embedded in metacritic.com's own JavaScript bundle and carried as a parameter default; no user account or token.

## Reachability Risk
- **Low.** `probe-reachability` classifies `backend.metacritic.com` as standard HTTP (0.95 confidence, no bot protection). Live read-only smoke against browse, search, detail, and reviews all returned correct data. (Music album pages are the exception — see Gaps.)

## Top Workflows
1. **Browse and rank by Metascore** — top games/movies/TV for a medium (`finder browse-titles`)
2. **Cross-media search** — one query spanning games, movies, TV, and people (`finder search-titles`)
3. **Title detail** — full composed payload with Metascore, user score, summary, release (`composer`)
4. **Critic reviews** — per-title critic review list with publication and score (`reviews list-critic`)
5. **User reviews** — per-title user review list (`reviews list-user`)

## Table Stakes (from competitors)
- Game scores and detail (melroy89/metacritic_api: PHP, games only)
- Multi-media coverage games/movies/TV (chrismichaelps/metacritic, metacritic-ts npm)
- Search + sentiment (metacritic-ts npm)
- Bulk archival to local store (gofurry/metacritic-harvester: the only existing Go tool, a batch archiver not a quick-lookup CLI)
- MCP exposure (MrCherry/mcp-metacritic: a 0-star thin wrapper, MCP server only)

## Data Layer
- **Primary entities:** Titles (games/movies/TV), Reviews (critic/user)
- **Medium selector:** `mcoTypeId` query param on browse (1 = TV, 2 = movies, 13 = games); `{mediaType}` path segment (`games`, `movies`, `shows`) on detail, filters, and reviews
- **Score fields:** `criticScoreSummary.score` (Metascore) + `userScore.score`
- **Transcend layer (scaffolded):** local SQLite + FTS5 `search`, `analytics`, compound `workflow` — present but `defaultSyncResources` is empty, so it needs a population path to be useful (tracked in Gaps)

## Product Thesis
- **Name:** metacritic-pp-cli (binary), Metacritic CLI (product)
- **Why it should exist:** Metacritic has no official API and no credible terminal-native quick-lookup CLI. The ecosystem is a PHP games-only library (83★), a couple of TS libraries, one Go *bulk archiver* (3★), and a 0-star MCP thin wrapper. None give you fast cross-media score lookup from a terminal with `--json` for agents. This CLI absorbs that surface across games/movies/TV from one host and adds an agent-native MCP surface.

## Build Priorities
1. Absorb: browse, cross-media search, filters, full detail, critic + user reviews across games/movies/TV
2. Agent-native: `--json` / `--select` on every command, doctor, agent-context, MCP server mirroring the Cobra tree
3. Transcend (follow-up): wire `defaultSyncResources` so `sync` populates local SQLite, making `search`/`analytics` fully functional
