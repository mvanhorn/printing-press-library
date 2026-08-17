# ScreenCloud Crowd-Sniff Report

## npm Packages Analyzed

| Package | Version | Downloads, Jul 24–30 2026 | Package release | Result |
|---|---:|---:|---|---|
| `@screencloud/studio-graphql-client` | 1.0.3 | 3 | March 9, 2022 | Official GraphQL transport/auth helpers; no fixed operation catalog |
| `@screencloud/apps-sdk` | 1.2.2 | 281 | February 10, 2022 | Official Player lifecycle/config/context SDK; no HTTP endpoints |
| `@screencloud/apps-editor-sdk` | 1.0.2 | 9 | May 18, 2022 | Official editor config-update lifecycle SDK; no HTTP endpoints |

The npm registry metadata was modified in 2026, but the latest package versions themselves were published in 2022. All three remain useful as contract evidence; none provides an OpenAPI schema.

## GitHub Repos Searched

- GitHub search ran authenticated, which enabled broader code-search results.
- Queries included `screencloud MCP server`, `screencloud CLI`, exact imports of `@screencloud/studio-graphql-client` and `@screencloud/apps-sdk`, `graphql.us.screencloud.com`, and `api-playgrounds.screencloudapps.com`.
- `screencloud/developer` — official JavaScript/TypeScript app SDK and documentation repository; 20 stars, 5 forks, not archived, pushed July 30, 2026.
- `Poafs1/screencloud-client` — zero-star 2023 ScreenCloud interview exercise, unrelated to the management API; excluded.
- No public ScreenCloud MCP server, maintained command-line client, official Claude plugin, or relevant PyPI client was found.
- The official developer repository has GitHub issues disabled, so no public `403`, deprecation, breakage, or rate-limit issue corpus exists.

## Endpoints Discovered

| Method | Path | Source Tier | Source Count |
|---|---|---|---:|
| POST | `https://graphql.{region}.screencloud.com/graphql` | official-sdk | 1 |

The automated crawler also proposed `POST /workramp/instant-auth` from an official-source tier. That route belongs to an unrelated WorkRamp integration and was rejected. It is not merged into the ScreenCloud specification.

## Base URL Resolution

- Selected Studio base: the region-specific endpoint displayed in ScreenCloud Studio and mapped by `@screencloud/studio-graphql-client`, with US and EU shorthands.
- Selected Playgrounds base: `https://api-playgrounds.screencloudapps.com`, confirmed by live editor/viewer bundles and read-only replay.
- The automated crowd spec chose the supplied GraphQL URL but cannot represent the two-host Playgrounds lifecycle by itself; it remains provenance only.

## Auth Patterns Detected

- `@screencloud/studio-graphql-client` accepts a developer token and constructs GraphQL requests against a region endpoint. The live contract uses `Authorization: Bearer $SCREENCLOUD_API_KEY`.
- `@screencloud/apps-sdk` exposes `appViewerToken` through Player context.
- The editor flow uses a separate short-lived app-management token.
- Canonical local environment variable for the printed CLI: `SCREENCLOUD_API_KEY`; region endpoint in `SCREENCLOUD_GRAPHQL_URL`; expected-org guard in `SCREENCLOUD_ORGANIZATION_ID`.

## Parameter Name Evidence

- Studio client: `endpoint`, `token`, GraphQL `query`, and `variables`.
- Region aliases: `us` and `eu`.
- Player SDK context: `appId`, `appInstanceId`, `orgId`, `spaceId`, optional `screenId`, `device`, `filesByAppInstanceId`, `durationMs`, `durationElapsedMs`, `theme`, `screenData`, `region`, `timezone`, and `appViewerToken`.
- Editor SDK lifecycle: `connect`, `initialize`, `start`, `getConfig`, `getContext`, `onAppStarted`, `emitConfigUpdateAvailable`, and `onRequestConfigUpdate`.
- Playgrounds service: `appUuid`, `files`, `html`, `css`, `js`, `scriptType`, `data`, `lastModified`, and `location`.

## Coverage Summary

- Automated result: one proposed endpoint across one resource, plus one crawler warning from the npm-download API.
- Accepted crowd contribution: one official GraphQL transport endpoint and the public SDK method/lifecycle surface.
- Rejected: one unrelated WorkRamp endpoint.
- Gaps relative to the brief: community code adds no Playgrounds file/data/package endpoints, no mutation examples, no optimistic-concurrency behavior, no error catalog, and no rate-limit details. Browser capture, official generated GraphQL docs, and official production bundles remain the authoritative discovery sources.
