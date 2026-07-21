# Immich CLI Brief

## API Identity
- Domain: Self-hosted photo and video management for a personal library.
- Users: a family archivist curating shared events; a privacy-minded self-hoster managing a growing camera roll; an agent helping someone find, organize, and safely clean up their own photos.
- Data profile: assets, EXIF/OCR/embeddings, people, albums, stacks, duplicate groups, memories, partners, tags, activities, shared links, and job queues.

## Reachability Risk
- Low for a configured personal server; this is a user-hosted REST API. The official OpenAPI spec is generated from the server and currently exposes 173 paths.
- Auth: API keys are sent as `x-api-key`; the generated CLI must use `IMMICH_API_KEY` and a user-configured `IMMICH_BASE_URL` rather than assuming a public cloud host.
- Probe-safe endpoint: `GET /server/ping` (a local instance URL is required, so Phase 5 is legitimately auth/instance-gated here).

## Top Workflows
1. Turn photos from a weekend, trip, or family event into a reviewable shared album.
2. Find a person across a time window, including a recurring month such as past Julys, without hand-searching the timeline.
3. Review native duplicate groups safely before applying a destructive resolution.
4. Bring a library in from folders, Takeout, iCloud, or a prior Immich installation while retaining organization.
5. Maintain a healthy personal photo server: check memory/duplicate/job state, partner sharing, stacks, favorites, archive, and queue pressure.

## Table Stakes and Incumbents
- Official `immich` CLI: API-key login, server information, and bulk upload; supports recursion, ignore patterns, per-folder or named albums, dry runs, concurrency, JSON results, deleting local duplicates, and watch mode. It is intentionally upload-focused.
- `simulot/immich-go` (6.5k+ stars, checked 2026-07-21): imports folders, Google Photos Takeout, iCloud, archives, ZIPs, and other Immich servers; preserves albums/tags/metadata; handles duplicate detection, burst/RAW+JPEG stacks, exclusions, and large collections. It is migration/import focused, not an agent-native library-management client.
- `barryw/ImmichMCP` (29 stars, checked 2026-07-21): broad 49-tool asset/search/album/person/tag/shared-link/activity CRUD plus local upload authorization. It covers generic API operations well, but its published tool list omits native memories, stacks, partners, jobs, and duplicate-resolution workflow; it also does not provide safe, compound personal rituals.
- Absorb stance: emit the full official v3 OpenAPI surface and add direct command coverage for the incumbents' upload, asset, search, album, people, tags, sharing, activity, health, and migration-adjacent capabilities. The eight novel commands must be mechanical compound workflows over the real Immich API, never LLM summaries or synthetic data.

## Data Layer
- Primary entities: assets, albums, people, duplicate groups, stacks, memories, partners, tags, activities, jobs.
- Sync cursor: generated pagination-backed resource sync where the upstream endpoint supports it; compound commands use bounded live calls where OpenAPI request bodies provide richer filters.
- FTS/search: generated SQLite local store plus live Smart/Metadata/OCR search endpoint mirrors.

## Product Thesis
- Name: Immich Printing Press CLI.
- Why it should exist: the official CLI and strongest importer solve moving files, while a broad MCP server exposes raw tools. This CLI makes everyday private-library questions and careful maintenance workflows dependable for both shell users and agents, with preview-before-apply safety and MCP parity.

## Build Priorities
1. Full official Immich v3 API coverage with correct `x-api-key` authentication and configurable self-hosted base URL.
2. Binary coverage of every capability claimed by the official CLI, Immich-Go, and ImmichMCP through exact generated or hand-authored command paths.
3. Exact importer parity is an explicit shipping commitment, not a low-level-endpoint claim: `import folder`, `import watch`, `import archive`, `import takeout`, `import icloud`, and `import immich` will implement the corresponding source behavior with explicit cancellation, stabilization, source filtering, bounded concurrency, metadata/album mapping where the source supplies it, and real Immich upload/copy/download calls.
4. Exactly eight novel personal-use rituals: event albums, duplicate planning/apply, recurring-month people search, memory review, favorite/archive review, stack review, partner sharing check, and job health.

## Evidence
- Official OpenAPI: https://docs.immich.app/developer/open-api and https://raw.githubusercontent.com/immich-app/immich/main/open-api/immich-openapi-specs.json (fetched 2026-07-21; SHA-256 `d4160960180c2640834a76236f00853fb873f9bb148957b66ea332035ad7ecef`).
- Official CLI: https://docs.immich.app/features/command-line-interface/ (fetched 2026-07-21).
- Native duplicate behavior: https://docs.immich.app/features/duplicates-utility/ (fetched 2026-07-21).
- Public incumbent READMEs inspected: https://github.com/simulot/immich-go and https://github.com/barryw/ImmichMCP (2026-07-21).
