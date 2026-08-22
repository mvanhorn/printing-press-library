# Cosmos Browser-Sniff Report

## User Goal Flow

- Goal: search and retrieve saved inspiration across collections, then organize or add elements without opening Cosmos.
- Steps completed:
  1. Loaded the authenticated For You feed and captured the account bootstrap operations.
  2. Searched globally for `brutalist typography` and switched between element and collection results.
  3. Opened the authenticated profile collection index.
  4. Exercised profile-scoped collection search.
  5. Opened one private collection and paged its elements.
  6. Ran collection-scoped search for `onboarding`.
  7. Opened one element detail view and loaded its collection connections, social graph, recommendations, and visually similar elements.
- Steps skipped: create collection, save URL, connect/disconnect, follow, and delete actions were not submitted because the user authorized discovery, not account mutation. Their known GraphQL documents were gathered from public source and Cosmos's shipped web bundles instead.
- Secondary flows attempted: recommendation feed, activity/bootstrap calls, and element similarity.
- Coverage: 7 of 7 planned read-only interaction steps completed.

## Pages & Interactions

1. `https://www.cosmos.so/` — authenticated feed; reload and scroll.
2. `https://www.cosmos.so/search/elements/brutalist%20typography` — submitted a keyword search.
3. Same search route — selected the Collections tab.
4. `https://www.cosmos.so/<authenticated-user>/collections` — opened the profile collection index and entered a collection search term.
5. `https://www.cosmos.so/<authenticated-user>/<private-collection>` — opened one private collection and scrolled its elements.
6. `https://www.cosmos.so/<authenticated-user>/<private-collection>/search/onboarding` — submitted a collection-scoped search.
7. `https://www.cosmos.so/e/<element-id>` — opened an element detail view and allowed recommendation panels to load.

No save, create, edit, disconnect, follow, or delete control was submitted.

## Browser-Sniff Configuration

- Backend: `agent-browser` 0.33.1, isolated headed session.
- HAR mode: response content included; raw HAR held only in the temporary session directory.
- Durable capture: 37 unique GraphQL operations with credential headers removed and all scalar response values replaced by type-preserving placeholders.
- Pacing: 1 second between agent-initiated interactions; the site itself issued concurrent page-bootstrap queries. No 429 response occurred.
- GraphQL BFF detected: yes — `POST https://api.cosmos.so/graphql?q=<OperationName>`.
- Proxy-envelope detected: no; routing is by GraphQL operation name rather than a service/method/path envelope.
- Runtime replay: standard HTTP. A captured authenticated GraphQL call replayed with plain `curl`; bearer auth returned 200 and the same call without auth returned 401.

## GraphQL Operations Discovered

| Operation | Method | Status | Content-Type | Auth |
|---|---:|---:|---|---|
| GetActiveImports | POST | 200 | application/json | auth-required |
| GetActivities | POST | 200 | application/json | auth-required |
| GetAllElements | POST | 200 | application/json | auth-required |
| GetAllElementsCount | POST | 200 | application/json | auth-required |
| GetAllRecentActivity | POST | 200 | application/json | auth-required |
| GetClusterBasic | POST | 200 | application/json | public |
| GetClusterBySlug | POST | 200 | application/json | public |
| GetClusterElements | POST | 200 | application/json | auth-required |
| GetClusterRecommendations | POST | 200 | application/json | public |
| GetConnectableClusters | POST | 200 | application/json | auth-required |
| GetDidSetupProfile | POST | 200 | application/json | auth-required |
| GetElementDetails | POST | 200 | application/json | public |
| GetElementSocialGraph | POST | 200 | application/json | public |
| GetElementTopCounts | POST | 200 | application/json | public |
| GetForYouElements | POST | 200 | application/json | auth-required |
| GetForYouUserConfiguration | POST | 200 | application/json | auth-required |
| GetHasUserSeenNewFeature | POST | 200 | application/json | auth-required |
| GetLoaderState | POST | 200 | application/json | auth-required |
| GetProfileCounts | POST | 200 | application/json | auth-required |
| GetQuickConnectRecommendation | POST | 200 | application/json | auth-required |
| GetRecentGlobalSearches | POST | 200 | application/json | auth-required |
| GetRecentlyViewedElementCount | POST | 200 | application/json | auth-required |
| GetSimilarElements | POST | 200 | application/json | auth-required |
| GetSubclusters | POST | 200 | application/json | public |
| GetUserClusters | POST | 200 | application/json | auth-required |
| GetUserFollowsBasic | POST | 200 | application/json | public |
| GetUserForMemberPage | POST | 200 | application/json | public |
| GetUserPublicClustersCount | POST | 200 | application/json | public |
| GetUserPublicElementsClusterId | POST | 200 | application/json | public |
| GetUserPublicElementsCount | POST | 200 | application/json | public |
| RememberGlobalSearchTerm | POST | 200 | application/json | auth-required |
| SearchGlobal | POST | 200 | application/json | public |
| SearchGlobalClusters | POST | 200 | application/json | public |
| SearchGlobalElements | POST | 200 | application/json | public |
| SearchUserClusters | POST | 200 | application/json | auth-required |
| SearchUserElements | POST | 200 | application/json | auth-required |
| ViewElement | POST | 200 | application/json | auth-required |

`GetClusterElements` also supports public collections; it is marked auth-required here because the captured instance was private.

## Traffic Analysis

- Protocols: GraphQL (0.92 confidence), SSR embedded data (0.85), REST JSON (0.75). The analyzer also flagged an RPC-like envelope from generic JavaScript evidence, but the replayable API surface is GraphQL.
- Auth signals: bearer token in `Authorization`; no credential values are preserved. Public operations also work without the header.
- Parameter evidence: `searchTerm`, `searchOrigin`, `pageCursor`, `pageSize`, `ownerId`, `userId`, `clusterId`, `elementId`, `elementIds`, time windows, and boolean feed filters.
- Protection signals: the raw asset traffic contained generic Cloudflare/Akamai/PerimeterX strings in shipped JavaScript and CDN headers. Direct website probing and authenticated GraphQL replay both worked through standard HTTP, so these are not runtime blockers.
- Generation hints: GraphQL BFF, bearer auth, cursor pagination, browser-derived undocumented contract, and weak evidence only for empty/binary non-API responses.
- Candidate commands: search elements/collections/global, list personal collections, collection show/elements/search, element show/similar/connections, activity, feed, and import status.
- Warnings: empty payload warnings correspond to preflight or optional bootstrap calls; binary warnings correspond to media segments, not GraphQL response bodies.

## Coverage Analysis

- Exercised: authenticated bootstrap, feed, global search, profile metadata, personal collections, private collection lookup, collection pagination, scoped search, element detail, social graph, connection status, recommendations, and similarity.
- Captured 37 operations across 19 retained resources after removing CORS preflight, notification transport, analytics, media streams, and unrelated web routes.
- Likely missed interactively: collection and subcollection mutation results, URL/media creation, connect/disconnect mutations, imports from Pinterest/Are.na/Tumblr, collaboration, following, profile edits, and deletion. Source analysis provides the proven mutation documents needed for the absorb manifest; live mutation tests remain excluded unless separately authorized.

## Response Samples

Durable samples preserve keys and container shapes while redacting scalar values:

```json
{"operation":"GetAllElements","top_level_keys":["data"],"data_keys":["allElementsV2"]}
{"operation":"SearchGlobalElements","top_level_keys":["data"],"data_keys":["searchElements"]}
{"operation":"GetClusterElements","top_level_keys":["data"],"data_keys":["clusterConnections"]}
{"operation":"GetElementDetails","top_level_keys":["data"],"data_keys":["elementQuickConnectRecommendation","elementView"]}
{"operation":"GetConnectableClusters","top_level_keys":["data"],"data_keys":["areSavedToLibrary","connectableClusters"]}
```

Binary media responses were excluded from the durable capture; their URLs and response content are not required to generate the API client.

## Rate Limiting Events

- No 429 responses.
- Seven agent-initiated interaction rounds used a 1 second pacing delay.
- The browser emitted page-owned concurrent queries; no artificial replay loop was run against all captured operations.

## Authentication Context

- Authenticated session used: yes.
- Transfer method: headed login in an isolated `agent-browser` session.
- Auth scheme: `Authorization: Bearer <token>`.
- Validation: `GetAllElements` replay without auth returned HTTP 401 with `AUTHENTICATION`; replay with the captured bearer returned HTTP 200 with data.
- Session state and the unredacted HAR live outside the run and manuscript directories. Neither is included in the sanitized capture or generated spec.

## Bundle Extraction

- API base discovered: `https://api.cosmos.so`.
- Client: `x-client-name: cosmos-web`; GraphQL endpoint `/graphql` with `?q=<OperationName>`.
- Bundle/source-only operations include login/token refresh, `CreateCluster`, `CreateSubcluster`, `CreateElement`, `EditElementsConnectionsToClusters`, follow/unfollow, and import workflows for Pinterest, Are.na, and Tumblr.
- The bundle operation inventory supplemented live read-only capture; only replayable HTTP operations are eligible for the printed CLI.
