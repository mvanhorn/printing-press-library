# Algolia CLI Brief

## API Identity
- Domain: Search-and-discovery platform (hosted full-text search, AI/NeuralSearch, recommendations, merchandising, analytics)
- Users: Developers and operators managing Algolia applications — e-commerce, media, SaaS (17,000+ companies)
- Data profile: Indices (up to 25k+ records), records, settings, rules, synonyms, dictionaries, API keys, clusters, logs. High-gravity entities: indices, records, API keys, rules, synonyms.

## Reachability Risk
- None. Official bundled OpenAPI spec resolved from `algolia/api-clients-automation` `specs/bundled/search.yml` (298KB, 60 paths, 78 operations). Live API verified with the user's credentials: App ID `RUO253QETS`, list-indices returns `algolia_movie_sample_dataset` (25,224 records); live search returns hits.
- **Reachability Gate: PASS** — probe `GET /1/indexes` returned HTTP 200 with valid credentials (via `-dsn.algolia.net` and `.algolia.io` hosts).
- **Host quirk:** primary host `{appId}.algolia.net` had intermittent DNS failures; `{appId}-dsn.algolia.net` and `{appId}.algolia.io` are stable. The generated CLI should prefer the `-dsn` host (or implement retry across fallback hosts per Algolia's documented retry strategy).

## Top Workflows
1. Search an index — query with filters, facets, pagination (`search --index <idx> --query "foo"`)
2. Manage indices — create/delete/list, copy/move, clear, settings export/import
3. Manage records — save/update/delete/browse objects, batch operations
4. Manage API keys — list/get/create/delete/rotate with ACLs and restrictions
5. Manage relevance — rules, synonyms, dictionaries, query suggestions

## Table Stakes (from official `@algolia/cli` v5.16.0)
- `apikeys create|delete|get|list|rotate`
- `application create|current|downgrade|list|planchange|plans|selectapp|update|upgrade`
- `auth login|logout|signup|status|get|crawler`
- `compositions delete|get`
- `crawler` (manage crawler)
- `dictionary entries browse|clear|delete|import`, `dictionary settings get|set|languages`
- `events tail`
- `factory default`
- `indices analyze|clear|config export|config import|copy|delete|list|move`
- `objects browse|delete|import|operations|update`
- `open` (open dashboard)
- `profile add|application|list|remove|setdefault`
- `rules browse|delete|import`
- `search`
- `settings get|list|import|set`
- `synonyms browse|delete|import|save`
- `describe`

## Data Layer
- Primary entities: indices, records, api_keys, rules, synonyms, dictionaries, tasks, logs, secured_api_keys
- Sync cursor: indices list (`GET /1/indexes`), records browse (`POST /1/indexes/{indexName}/browse`), rules/synonyms browse
- FTS/search: local SQLite FTS over synced records (titles, objectIDs, attribute values)

## User Vision
- User wants a fully-functional Algolia CLI tested deeply (live dogfood). No specific feature direction given beyond the API surface.

## Product Thesis
- Name: `algolia-pp-cli`
- Why it should exist: The official CLI is a Go binary but has no offline capability, no local search over synced data, no `--json`/`--select` agent-native output across all commands, no SQLite data layer, no typed exit codes, and no compound cross-resource workflows (e.g., "find stale rules in an index", "diff settings between two indices", "search across all indices"). A Printing Press CLI matches every official command and beats it with offline search, local state, and agent-native output.

## Build Priorities
1. Absorb the official CLI command surface (indices, objects, search, keys, rules, synonyms, settings, dictionaries, logs, tasks)
2. Local SQLite store synced from live API (records, rules, synonyms, keys) with offline FTS search
3. Transcendence: cross-index search, settings diff, stale-rule detection, key ACL report, task wait
4. Agent-native: `--json`, `--select`, `--compact`, `--dry-run`, typed exit codes on every command

## Competition / Ecosystem
- Official CLI: `@algolia/cli` (npm, Go binary) — the incumbent to match
- Official MCP: Algolia Productivity MCP at `mcp.algolia.com` (query Algolia data from AI assistants)
- SDKs: official clients for JS, Python, Go, PHP, Ruby, Swift, Java, Kotlin, C#, Scala, Dart
- No official Algolia MCP server repo on GitHub; community MCP servers are low-quality (<=9 stars)
- `algolia/algoliasearch-client-go` official Go client (reference for transport patterns)
