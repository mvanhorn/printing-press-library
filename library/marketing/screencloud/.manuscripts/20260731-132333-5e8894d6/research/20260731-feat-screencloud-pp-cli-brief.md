# ScreenCloud CLI Brief

## API Identity
- Domain: Digital-signage content and app management, with a specific deep focus on ScreenCloud Playgrounds.
- Users:
  - A digital-signage developer who builds and revises HTML/CSS/JavaScript Playgrounds for campaigns, keeps Studio and a code editor open, and needs a reproducible pull/diff/preview/push loop.
  - A signage platform administrator who inventories spaces, verifies Playgrounds installation and version state, and checks app instances before deployments across an organization.
  - A content operations lead who starts from Playgrounds templates, connects public JSON data, previews changes, and needs to know exactly which screen content will change before publishing.
  - An automation engineer who uses the Studio GraphQL API in scripts and needs stable, agent-shaped output, pagination, cost visibility, and guarded mutations without hand-writing GraphQL documents each time.
- Data profile: Studio is a relationship-heavy GraphQL graph (organizations, spaces, apps, installs, versions, instances, shares, channels, playlists, and screens). The official v2.103.0 reference publishes 1,903 schema pages: 386 queries, 319 mutations, 579 objects, 503 inputs, 102 enums, one interface, and 13 scalars. Playgrounds content lives in a separate file/data service keyed by `appUuid`, with `lastModified` supporting a likely concurrency boundary. Organization API keys and short-lived management/viewer JWTs are sensitive and must not enter the local store.

## Reachability Risk
- Low for the current Studio API: a bearer-authenticated, read-only `currentOrgId` POST succeeded and matched the supplied organization on July 31, 2026. A generic GET probe returned HTTP 405 because the endpoint requires GraphQL POST; this is not evidence of an outage.
- Low for the Playgrounds service transport: a direct unauthenticated probe returned the expected HTTP 401 JSON response, classifying it as standard HTTP rather than a browser-protected endpoint.
- Contract risk is medium for Playgrounds: its file/data/package API was derived from ScreenCloud's current production editor/viewer bundles, not a published stable reference, and may change without notice.

## Reachability Gate
- Decision: PASS.
- Evidence: on August 1, 2026, a real bearer-authenticated read-only `POST /graphql` returned HTTP 200, contained no GraphQL errors, matched the configured organization guard, and reported query cost 1. No identifiers or credential values were retained.

## Top Workflows
1. **Playgrounds release ritual:** pull the current files and JSON data, compare them with a local project, preview changes, detect `lastModified` drift, and publish only after reviewing the exact target and diff.
2. **Organization inventory ritual:** list spaces, find the Playgrounds installation in each space, join installations to instances and versions, and flag missing, inactive, or outdated configurations.
3. **Create-from-template ritual:** select a Playgrounds template, validate HTML/CSS/JS/data locally, create the Studio app instance, upload service content, and report partial completion safely if either side fails.
4. **App debugging ritual:** inspect the catalog app, stable/latest `AppVersion`, viewer/editor URLs, manifest/store metadata, instance config/state/status, and GraphQL query-cost/errors in one diagnostic report.
5. **Automation ritual:** run bounded, paginated GraphQL queries with predictable JSON/agent output while keeping region, organization, and token context explicit and guarded.
6. **Least-privilege ritual:** compare the current token and user capabilities with the published permission catalog before attempting a command, and report available, missing, and unknown permissions without exposing raw grants.

## Table Stakes
- Secure bearer-token setup from environment/keychain with `currentOrgId` mismatch protection.
- Read-only commands for apps, spaces, installations, versions, and app instances.
- Relay pagination, filtering, ordering, JSON/CSV/selected-field output, and GraphQL `errors` handling even when HTTP is 200.
- Local sync/search for high-gravity metadata, with credentials and short-lived JWTs excluded.
- A complete, regenerated official-schema atlas plus a generic GraphQL request command for published operations outside the typed Playgrounds workflow.
- Read-only capability diagnostics based on `currentToken`, `currentUser`, and `permissionsList`.
- Playgrounds pull/diff/preview/push using exact app/space targets and optimistic drift checks.
- Mutation dry-runs, explicit confirmations, idempotency/partial-failure reporting, and redacted logs.
- Incumbents are ScreenCloud Studio for interactive work and generic curl/Postman/GraphQL clients for automation. No maintained dedicated ScreenCloud CLI or MCP server was found.

## Data Layer
- Primary entities: spaces, apps, app versions, app installations, app instances, channels, playlists, screens, association/share-association edges, and sanitized Playgrounds metadata. Optional local working-copy files belong in a user-selected directory, not the metadata database.
- Sync cursor: GraphQL Relay cursors where available, supplemented by `updatedAt` and bounded full refreshes. Playgrounds `lastModified` is a write-drift guard, not a general catalog cursor.
- FTS/search: app/space/instance name, slug, tags, status, version, app UUID, and sanitized config keys. Do not index source code or private JSON data by default.

## Codebase Intelligence
- Source: ScreenCloud's public `screencloud/developer` repository, official generated Studio GraphQL reference, npm package metadata, and current official production Playgrounds editor/viewer bundles.
- Auth: Studio uses `Authorization: Bearer $SCREENCLOUD_API_KEY`. The editor uses a short-lived app-management JWT; the viewer uses a distinct short-lived app-viewer JWT.
- Data model: `Organization -> Space -> AppInstall -> AppInstance`, with `App -> AppVersion`; placement continues through direct casts, associations, shares, channels, playlists, and screens. Playgrounds instance config points by `appUuid` to content stored in the separate app service.
- Rate limiting: no fixed public limit was found. Responses expose `meta.graphqlQueryCost`, so the client should surface cost and use bounded pagination.
- Architecture: Playgrounds is a coordinated two-service workflow. Studio stores lifecycle metadata and mints scoped JWTs; the Playgrounds service stores files/data and assembles viewer packages.
- Permission model: a live read-only structural probe succeeded for `currentToken`, `currentUser`, and `permissionsList`. The catalog exposed 25 permission domains; relevant domains include app, app instance, channel, playlist, screen, space, token, and permission sets. Effective grant values remain private.
- Cost observation: one bounded live placement-topology query reported `graphqlQueryCost` 1579. The impact workflow therefore needs bounded pagination, a cached local graph, freshness metadata, and visible query cost rather than repeated deep live traversals.

## User Vision
- Create a ScreenCloud project that documents ScreenCloud's API and system.
- Run the Printing Press workflow against ScreenCloud.
- Focus specifically on Playgrounds apps.
- Use the clipboard-provided API context for safe validation, with the user helping where credential or live access is required.

## Product Thesis
- Name: `screencloud-pp-cli`
- Why it should exist: generic GraphQL tools can enumerate objects but cannot safely coordinate Studio app-instance state with Playgrounds files, data, preview, scoped tokens, and drift detection. This CLI should make the real Playgrounds release ritual reproducible, inspectable, and safe for both humans and agents.

## Build Priorities
1. Model read-only Studio inventory, least-privilege capability diagnostics, and Playgrounds diagnostics with correct GraphQL error/cost handling.
2. Implement a redacted local metadata store and cross-entity joins through shares, channels, playlists, and screens.
3. Implement guarded Playgrounds pull/diff/preview/push with `lastModified` drift protection.
4. Implement coordinated create-from-template with validation and partial-failure recovery.
5. Add agent-shaped output, recipes, and live read-only smoke tests before any mutation testing.
6. Keep mutation testing in the separate approval-gated sandbox plan; ordinary generation, dogfooding, and read-only checks do not authorize writes.

## Source Notes
- Primary current management surface: official ScreenCloud Studio GraphQL API and generated reference.
- Complete reference inventory: official v2.103.0 sitemap, regenerated into the project API atlas with all 1,903 published schema pages linked.
- Playgrounds specialization: official production Playgrounds editor/viewer bundles, treated as an internal and potentially changeable contract.
- Legacy context only: deprecated ScreenCloud Signage REST API.
- Official packages: `@screencloud/studio-graphql-client` 1.0.3 and `@screencloud/apps-sdk` 1.2.2 as observed July 31, 2026.
- Ecosystem review: the official developer repository is active but has GitHub issues disabled; no dedicated ScreenCloud MCP server or maintained CLI was found. A zero-star repository named `screencloud-client` was an unrelated interview exercise and contributed no feature or contract evidence.
