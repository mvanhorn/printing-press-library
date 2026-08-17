# ScreenCloud Browser-Sniff Report

## User Goal Flow

- Goal: inspect an existing Playgrounds instance and verify the Studio-to-editor-to-viewer read contract without saving or publishing.
- Steps completed:
  1. Opened ScreenCloud Studio's Developer page in a fresh authenticated in-app-browser tab.
  2. Navigated to Apps, filtered the installed catalog to Playgrounds, and opened the Playgrounds details page.
  3. Opened one existing Playgrounds instance and confirmed the editor was embedded through `studio-player.screencloud.com` into `playgrounds-editor.apps.screencloud.com`.
  4. Opened the Application Data panel, observed the `/data/{orgId}/{spaceId}/{appUuid}` location contract, and canceled without editing.
  5. Replayed read-only GraphQL and Playgrounds service requests with environment-held credentials and short-lived scoped JWTs, retaining only sanitized response shapes.
- Steps skipped: Preview was not clicked because it may write a `-preview` artifact. Save & Close remained disabled and was never clicked. No create, update, publish, force-overwrite, or delete flow was attempted.
- Secondary flows attempted: Studio app catalog/version/instance inventory and management/viewer JWT minting were validated to support the Playgrounds lifecycle map.
- Coverage: 5 of 7 planned read-oriented steps completed; preview and publish were intentionally excluded as write-capable actions.

## Pages & Interactions

1. `https://studio.us.screencloud.com/developer` — confirmed authenticated Developer surface and region-specific GraphQL endpoint presentation.
2. `https://studio.us.screencloud.com/screens` — entered the main Studio shell.
3. `https://studio.us.screencloud.com/apps-store/category/discover/` — searched for Playgrounds.
4. `https://studio.us.screencloud.com/apps/{playgroundsAppId}` — inspected installed instances without changing them.
5. `https://studio.us.screencloud.com/apps/{appInstallId}/{appInstanceId}` — opened an existing editor, opened Application Data, then canceled.

Private organization, space, installation, instance, app UUID, names, application data, and source-code values are omitted.

## Browser-Sniff Configuration

- Backend: Codex in-app browser for authenticated UI discovery, supplemented by read-only direct HTTP replay in an isolated shell.
- Pacing: one-second delay between Playgrounds service calls; approximately 1 request/second effective rate.
- Proxy pattern: not detected. Studio uses a conventional GraphQL endpoint; Playgrounds uses distinct REST-style paths.
- Capture format: enriched JSON with credentials, private identifiers, source code, and application data redacted before disk write.

## Endpoints Discovered

| Method | Path | Status Code | Content-Type | Auth |
|---|---|---:|---|---|
| POST | `https://graphql.us.screencloud.com/graphql` | 200 | `application/json; charset=utf-8` | organization bearer token |
| GET | `https://api-playgrounds.screencloudapps.com/templates` | 200 | `application/json` | app-management JWT |
| GET | `https://api-playgrounds.screencloudapps.com/files/{appUuid}` | 200 | `application/json` | app-management JWT |
| GET | `https://api-playgrounds.screencloudapps.com/data/{appUuid}` | 200 | `application/json` | app-management JWT |
| GET | `https://api-playgrounds.screencloudapps.com/apps/{appUuid}` | 200 | `text/html` | app-viewer JWT |

The browser-sniff analyzer emitted four generated endpoint clusters across four resources. It recognized the viewer response as HTML evidence but did not emit it as an ordinary generated endpoint.

## Traffic Analysis

- Protocols: `graphql` confidence 0.92, `rest_json` confidence 0.75, and `html_scrape` confidence 0.55.
- Auth signals: organization `Authorization` bearer token; short-lived app-management and app-viewer JWTs. Values were never written.
- Parameter evidence: GraphQL uses `query` and optional `variables`; JWT inputs use `spaceId` and viewer input optionally supports `screenId`; Playgrounds service paths use `appUuid`; writes observed in the official editor bundle use `files`, `data`, and `lastModified`.
- Protection signals: none. The Playgrounds service returned structured `401` without auth and standard `200` responses with scoped tokens.
- Generation hints: standard HTTP is replayable. No resident browser transport is required.
- Candidate commands: organization verify, apps/spaces/instances inventory, Playgrounds inspect, pull, diff, preview, push, and create-from-template.
- Warnings: the generated sniff spec collapses multiple GraphQL operations into one generic POST and chooses a single base URL despite the multi-host contract. It is discovery scaffolding only and must not be treated as the final build spec. GraphQL introspection is unavailable on the live endpoint.

## Coverage Analysis

Covered entities and flows: organization identity check, app catalog lookup, installed Playgrounds instances, current app-version/editor metadata, management-token minting, viewer-token minting, templates, files, application data, and rendered viewer package.

Likely missed: exact `PUT /files/{appUuid}` and `PUT /data/{appUuid}` live error/envelope behavior, preview write traffic, publish transition, stale-`lastModified` conflict response, rollback behavior, deletion, screen-bound viewer JWT behavior, and rate-limit thresholds. Those are mutation-oriented or destructive surfaces and were not exercised without a separate approval.

## Response Samples

All samples below are structural and redacted before persistence.

### GraphQL success

```json
{"data":{"currentOrgId":"REDACTED"},"meta":{"graphqlQueryCost":1}}
```

### Templates

```json
{"templates":[{"name":"REDACTED","description":"REDACTED","thumbnailUrl":"REDACTED","files":{},"lastModified":0,"data":{},"tags":[]}]}
```

The live response contained 12 templates.

### Files

```json
{"files":{"html":"REDACTED","css":"REDACTED","js":"REDACTED","scriptType":"javascript"},"lastModified":"REDACTED","location":"REDACTED"}
```

### Data

```json
{"data":{},"lastModified":"REDACTED","location":"REDACTED"}
```

### Viewer package

Binary/HTML response metadata: `text/html`, 8,021 bytes in the inspected instance. Body omitted because it contains private Playgrounds source and rendered content.

## Rate Limiting Events

No HTTP `429` responses occurred. Playgrounds service replay ran at approximately one request per second with a one-second delay between calls.

## Authentication Context

The UI flow used the existing authenticated in-app-browser session in a fresh tab. Direct replay used the organization API key from an isolated environment, then minted one short-lived app-management JWT and one short-lived app-viewer JWT using the documented GraphQL mutations. Authorization headers, cookies, JWTs, organization ID, space ID, instance IDs, app UUIDs, source code, and application data were excluded from capture artifacts. No session state file was archived.

## Bundle Extraction

- Editor entry: `https://playgrounds-editor.apps.screencloud.com/`
- Editor bundle: `https://playgrounds-editor.apps.screencloud.com/static/js/main.c0184561.chunk.js`
- Viewer entry: `https://playgrounds.apps.screencloud.com/`
- Viewer bundle: `https://playgrounds.apps.screencloud.com/static/js/main.b7684be1.chunk.js`
- API base discovered: `https://api-playgrounds.screencloudapps.com`
- Bundle-only write contracts: `PUT /files/{appUuid}` with `{files,lastModified}`, `PUT /data/{appUuid}` with `{data}`, and the `{appUuid}-preview` convention.
- Extracted configuration: editor returns Studio config `{appUuid,lastUpdated}`; viewer fetches `/apps/{appUuid}` and passes ScreenCloud context plus the API host to the rendered iframe.
