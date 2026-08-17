# Raindrop.io CLI Research Brief

## Product thesis

Build an agent-native Go CLI and MCP server covering the complete documented Raindrop.io REST surface, then add a persistent SQLite mirror for offline retrieval, safe cleanup, durable review workflows, and historical insights unavailable from current-state API calls.

Primary users are knowledge workers with large libraries, automation agents needing deterministic JSON and safe bulk operations, researchers using highlights, and the existing `raindrop-processor` operator who already relies on incremental checkpoints, bounded review batches, retry classification, and confirmed server writeback.

## Sources

- Official REST API documentation: https://developer.raindrop.io/
- Official hosted MCP: https://help.raindrop.io/integrations/mcp
- `kyoji2/raindrop-cli`: https://github.com/kyoji2/raindrop-cli
- `adeze/raindrop-mcp`: https://github.com/adeze/raindrop-mcp
- `hiromitsusasaki/raindrop-io-mcp-server`: https://github.com/hiromitsusasaki/raindrop-io-mcp-server
- Prior local processor: `/home/siju/WORK/GINA/raindrop-processor`

## API contract

- Base URL: `https://api.raindrop.io/rest/v1`
- Auth: `Authorization: Bearer <token>`
- Canonical local env var: `RAINDROP_TOKEN`
- Pagination: zero-based `page`, maximum `perpage=50`
- Important special collections: `0` all bookmarks, `-1` Unsorted, `-99` Trash
- Read-only reachability probe: `GET /user`

## Existing-tool strengths to absorb

`kyoji2/raindrop-cli` provides broad bookmark, collection, tag and batch management; dry-run; JSON/TOON output; account context; suggestions; and Wayback checks. `adeze/raindrop-mcp` adds highlights, collection trees, library audits, duplicate cleanup, diagnostics, rate limiting, and retry behavior. Official MCP adds semantic/full-content search, misplaced/mistagged detection, full highlight CRUD, and bulk organization. Prior local processor contributes incremental checkpoints, bounded queue packets, lifecycle state, retry/manual classification, and confirmed writeback.

## Differentiation

No discovered CLI combines full REST coverage, CLI plus MCP parity, local SQLite/FTS5, incremental sync, historical diffs, metadata-preserving duplicate plans, resumable inbox review, durable triage queues, and offline knowledge workflows.

## Reachability

Official API is documented and token-authenticated. Live gate will use the copied token only for `GET /user` and other read-only smoke tests. Mutation tests remain mock/dry-run unless explicitly safe.
