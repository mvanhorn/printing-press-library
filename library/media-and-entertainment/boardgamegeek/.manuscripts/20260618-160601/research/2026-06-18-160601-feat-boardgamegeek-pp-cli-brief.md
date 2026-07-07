# BoardGameGeek CLI Brief

## API Identity
- **Domain:** Board game database — games, expansions, accessories, hot rankings, user collections, logged plays, families, and guilds, from BoardGameGeek's official XMLAPI2
- **Users:** Board gamers who live in the terminal, collection/stat trackers, developers building game-data integrations, agents that need structured game lookups
- **Data profile:** Read-only, Bearer-token auth, single first-party XML backend, moderate volume, HIGH lookup/collection need
- **Base URL:** `https://boardgamegeek.com/xmlapi2` (no leading `www`)
- **Auth:** Bearer token. As of 2025-07-02 BGG requires a registered application (`https://boardgamegeek.com/applications`, non-commercial licenses free) and an `Authorization: Bearer <token>` header on every request.

## Reachability Risk
- **Medium (auth-gated).** The XMLAPI2 host is standard HTTP behind Cloudflare with no bot challenge, but anonymous requests now return `401 Unauthorized`; a registered-application Bearer token is required. Approval is a human review that can take a week or more, so the live read-only smoke test (search, thing, hot) is gated on token approval. The CLI is otherwise complete and passes all build/verify gates offline.

## Top Workflows
1. **Search for a game** — full-text name search across the BGG database (`searches`)
2. **Game detail with stats** — full record plus rating, rank, and ownership stats (`thing --stats 1`)
3. **Hot list** — the live BoardGameGeek Hot rankings (`hot`)
4. **User collection** — a user's owned/rated/wishlisted games (`collection`)
5. **Logged plays** — play sessions for a user or a game (`plays`)

## Table Stakes (from competitors)
- Search + thing detail with stats (lcosmin/boardgamegeek: Python, 230★ — the reference library; no CLI)
- Collection / plays / user / guild coverage (philsstein/bgg_api: Python, 90★; no CLI)
- Go client (beefsack/go-bgg: 22★ — a library, not a CLI)
- A terminal CLI (Nuztalgia/bgg: Python, 5★ — the only existing CLI, narrow scope)
- MCP exposure (MarkusFox/bgg-mcp: 1★ TS MCP server only)

## Data Layer
- **Primary entities:** Things (games/expansions/accessories), Collections, Plays, Users, Families, Guilds
- **ID model:** BGG returns ids in XML *attributes* (`<item id="13">`), normalized to `@id` by the XML→JSON client. Numeric thing ids (e.g. Catan = 13) are stable.
- **Response shape:** Every endpoint returns XML; the generated client normalizes it to JSON (attributes → `@name`, text → `#text`, repeated elements → arrays) so `--json`/`--select` and MCP tools behave like any JSON API. This rides on the generator's new `xml` response_format.
- **Transcend layer (scaffolded):** local SQLite + FTS5 `search`, `analytics`, compound `workflow` — present but offline caching is incomplete because synced rows lack an extractable id (the `@id`-vs-`id` gap; tracked in Gaps). Live queries are unaffected.

## Product Thesis
- **Name:** boardgamegeek-pp-cli (binary), BoardGameGeek CLI (product)
- **Why it should exist:** BGG's ecosystem is libraries, not finished tools — a 230★ Python library, a 90★ Python library, a Go client, one narrow 5★ Python CLI, and a 1★ MCP wrapper. None give you fast, agent-native game lookup from a terminal with `--json` and a full MCP surface across search, detail, hot, collections, and plays. This CLI absorbs that surface from one host and adds the agent-native layer.

## Build Priorities
1. Absorb: search, thing detail (+stats/versions/videos), hot, user, collection, plays, family, guild
2. Agent-native: `--json` / `--select` on every command, doctor, agent-context, MCP server mirroring the Cobra tree, Bearer auth via `BGG_TOKEN`
3. Transcend (follow-up): map `@id` → `id` so `sync` populates local SQLite with addressable rows, making `search`/`analytics` fully functional offline
