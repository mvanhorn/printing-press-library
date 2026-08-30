# Cosmos CLI Brief

## API Identity

- Domain: `https://www.cosmos.so/`; undocumented GraphQL service at `https://api.cosmos.so/graphql`.
- Users: designers, photographers, creative directors, researchers, and teams collecting visual references.
- Data profile: users own collections (also called clusters), elements, media, source attribution, connection records, search results, cursors, follows, and visibility metadata.
- Official product promise: a connected, searchable space for inspiration with keyword/visual search, AI-content controls, and source/artist attribution.
- Capture extension: images, links, selected text, and supported social posts can be saved from the browser.

## Reachability Risk

- Low for HTTP transport: `probe-reachability` returned `standard_http` with 200 responses for the Cosmos website. A GET to the GraphQL endpoint returned 415, which is expected for a POST-only GraphQL service and did not show a WAF challenge.
- API stability risk is medium: Cosmos publishes no public API contract. Community clients state that GraphQL shapes were reverse-engineered and may drift.
- The most complete wrapper found (`jpoindexter/cosmos-mcp`) has no open or closed issues reporting 403, blocking, deprecation, or rate-limit failures as of 2026-08-01.

## Top Workflows

1. Search Cosmos for elements and collections, filter results, page through them, and preserve source attribution.
2. List and inspect personal collections, then fetch every element from one collection.
3. Save a URL or an existing Cosmos element into one or more collections.
4. Create and organize collections, including private collections and nested/subcollections.
5. Export or sync collection media and metadata locally for moodboards, archives, or downstream creative tooling.

## Table Stakes

- Keyword search across elements and collections, plus a combined global search.
- Featured/trending elements and collections.
- Collection lookup by ID and by owner/slug; cursor-based element pagination.
- Authenticated profile and personal-collection listing.
- Save URL, connect/disconnect existing elements, create collections, and discover visually similar elements.
- Structured JSON and stable IDs/URLs suitable for agents and shell pipelines.
- Local export/download with a manifest, deduplication, and resumable operation.
- Offline cache/search and reproducible snapshots, which competing web-only clients do not provide.

## Data Layer

- Primary entities: users, collections, elements, media, sources, collection-element connections, and sync runs.
- Relationships: user owns collections; collections may have a parent; elements connect to one or more collections; elements point to media and source attribution.
- Sync cursor: GraphQL `nextPageCursor` per collection/search result, with count and last-sync metadata stored locally.
- FTS/search: captions, website titles/descriptions, collection names/descriptions, owners, source URLs, and local notes/tags.
- Files: optionally download media by stable Cosmos CDN URL while retaining element ID, dimensions, media type, provenance, and checksum.

## Codebase Intelligence

- Source: direct source analysis of `jpoindexter/cosmos-mcp`, plus focused analysis of public Cosmos scraper/export scripts.
- Protocol: `POST https://api.cosmos.so/graphql?q=<OperationName>` with JSON `{operationName, query, variables}` and `x-client-name`.
- Auth: `Authorization: Bearer <JWT>`; the community MCP supports `COSMOS_ACCESS_TOKEN`, `COSMOS_REFRESH_TOKEN`, login, and token refresh. The live web app also refreshes through a credentialed web route.
- Rate limiting: the MCP serializes calls with a 350 ms gap and handles 429 explicitly. Discovery will begin at a 1 second delay and adapt only after successful calls.
- Proven operations include `SearchElements`, `SearchClusters`, `GetFeaturedElements`, `GetFeaturedClusters`, `GetCluster`, `GetClusterBySlug`, `GetClusterElements`, `GetMe`, `GetMyClusters`, `GetConnectableClusters`, `GetSimilarElements`, `CreateElement`, `SaveToCluster`, `CreateCluster`, `Login`, and `RefreshAccessToken`.
- Public scripts prove unauthenticated collection pagination and CDN media download. They also show two currently observed collection element shapes: `clusterConnections` and `elements(filters: {clusterId})`.

## User Vision

- Build for `cosmos.so` what the Printing Press Library provides for other services: a comprehensive, agent-native, scriptable CLI rather than a thin one-off scraper.
- The user confirmed an authenticated Cosmos browser session for discovery.

## Product Thesis

- Name: `cosmos-pp-cli`
- Thesis: make a visual inspiration library searchable, automatable, exportable, and locally auditable from the terminal while preserving Cosmos attribution and using the undocumented web API only through replayable HTTP.

## Build Priorities

1. Reliable GraphQL client, browser-assisted auth import, profile/collection/search/read commands, pagination, and JSON output.
2. Safe organization and capture commands: create collection, save URL, connect/disconnect elements, with dry-run and confirmation semantics.
3. Local sync/export: SQLite cache, incremental cursor tracking, FTS, media manifests, resume, and deduplication.
4. Compound creative workflows that compare, audit, diff, and assemble collections without losing source provenance.

## Sources

- Cosmos official site: https://www.cosmos.so/
- Save to Cosmos extension: https://chromewebstore.google.com/detail/save-to-cosmos/mgjneceglphcpbbfbhjplkpgfapebmdg
- Cosmos MCP: https://github.com/jpoindexter/cosmos-mcp
- Cosmos scraper: https://github.com/rclaycock/cosmos-scraper-mk-3
- Public collection downloader: https://github.com/rawpage/suggaplay
- Are.na API (competitor reference): https://dev.are.na/documentation
- Raindrop.io API (competitor reference): https://developer.raindrop.io/
