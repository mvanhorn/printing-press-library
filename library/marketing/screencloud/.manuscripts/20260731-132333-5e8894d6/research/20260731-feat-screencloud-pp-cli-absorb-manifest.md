# ScreenCloud CLI Absorb Manifest

## Scope

This first print combines ScreenCloud's complete published Studio GraphQL v2.103.0 reference, its official JavaScript client and app SDKs, and the production Playgrounds editor/viewer contract. The generated API atlas covers all 1,903 published schema pages; typed commands focus on Playgrounds and its deployment topology, while `graphql request` remains the escape hatch for every published root operation. The deprecated Signage REST API is migration context only. The crowd-sniff candidate `POST /workramp/instant-auth` was rejected as an unrelated integration false positive.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Execute a Studio GraphQL document with variables | `@screencloud/studio-graphql-client` `request` | screencloud-pp-cli graphql request | File/stdin input, structured errors, query-cost output, redacted variables, and agent-shaped JSON |
| 2 | Initialize and reuse a configured Studio client | `@screencloud/studio-graphql-client` `initClient` / `getClient` | (behavior in screencloud-pp-cli graphql request) shared configured client with explicit region and endpoint | One configuration path for interactive commands, scripts, and generated endpoint commands |
| 3 | Recognize and safely inspect Studio token material | `@screencloud/studio-graphql-client` `isStudioGraphQLToken` | screencloud-pp-cli auth inspect | Reports token shape and configuration source without printing the credential |
| 4 | Parse a GraphQL request payload | `@screencloud/studio-graphql-client` `parseGraphQLRequest` | screencloud-pp-cli graphql parse | Validates query/variables locally before a network call and supports stdin |
| 5 | Map a Studio region to its GraphQL endpoint | `@screencloud/studio-graphql-client` `mapStudioGraphQLEndpoint` | screencloud-pp-cli regions endpoint | Makes US/EU routing explicit and permits a user-supplied endpoint override |
| 6 | Supply a fetch implementation in runtimes that lack one | `@screencloud/studio-graphql-client` `polyfillFetch` | (behavior in screencloud-pp-cli graphql request) native Go HTTP transport | Removes the JavaScript runtime dependency while preserving request semantics |
| 7 | Verify the authenticated organization | Studio GraphQL `currentOrgId` | screencloud-pp-cli org current | Fails closed on configured organization mismatch and never reveals credentials |
| 8 | List and inspect catalog apps | Studio GraphQL `allApps` / `App` | screencloud-pp-cli apps list | Relay pagination, filters, selected fields, JSON/CSV, sync, and offline search |
| 9 | List and inspect spaces | Studio GraphQL `allSpaces` / `Space` | screencloud-pp-cli spaces list | Relay pagination, filters, selected fields, JSON/CSV, sync, and offline search |
| 10 | List and inspect app instances | Studio GraphQL `allAppInstances` / `AppInstance` | screencloud-pp-cli app-instances list | Sanitizes config by default and supports cross-entity local joins |
| 11 | List and inspect app installations | Studio GraphQL app-install connection / `AppInstall` | screencloud-pp-cli app-installs list | Exposes per-space installation and status/version context in scriptable output |
| 12 | List and inspect app versions | Studio GraphQL app-version connection / `AppVersion` | screencloud-pp-cli app-versions list | Compares stable/latest/installed versions without opening Studio |
| 13 | Create a Studio app instance | Studio GraphQL `createAppInstance` | screencloud-pp-cli app-instances create | Dry-run, explicit target display, confirmation, and partial-completion receipts |
| 14 | Mint a scoped app-management JWT | Studio GraphQL `createSignedAppManagementJwt` | screencloud-pp-cli tokens management create | Requires explicit space, redacts the token by default, and never stores it |
| 15 | Mint a scoped app-viewer JWT | Studio GraphQL `createSignedAppViewerJwt` | screencloud-pp-cli tokens viewer create | Requires explicit scope, redacts the token by default, and never stores it |
| 16 | Connect a hosted app to ScreenCloud Player | `@screencloud/apps-sdk` `connectScreenCloud` / `getScreenCloud` | screencloud-pp-cli app-runtime inspect | Validates a captured or test runtime context without needing a browser debugger |
| 17 | Exercise app connect/initialize/start lifecycle | `@screencloud/apps-sdk` lifecycle methods | (behavior in screencloud-pp-cli app-runtime inspect) lifecycle-state validation | Converts SDK lifecycle state into deterministic diagnostics for local app development |
| 18 | Inspect app configuration and player context | `@screencloud/apps-sdk` `getConfig` / `getContext` | (behavior in screencloud-pp-cli app-runtime inspect) sanitized context projection | Selectable output for device, screen, app, duration, region, timezone, files, and theme fields |
| 19 | Model the app-start event and local test data | `@screencloud/apps-sdk` `onAppStarted` / test initialization | screencloud-pp-cli app-runtime validate | Validates local fixtures and lifecycle prerequisites without calling an external service |
| 20 | Model editor configuration-update messaging | `@screencloud/apps-editor-sdk` `emitConfigUpdateAvailable` / `onRequestConfigUpdate` | (behavior in screencloud-pp-cli app-runtime validate) editor-message contract validation | Catches malformed editor config fixtures before they reach Studio |
| 21 | List Playgrounds templates | Production Playgrounds editor bundle and live read-only response | screencloud-pp-cli playgrounds templates list | Stable structured output with tags, file types, metadata, and redaction |
| 22 | Read current Playgrounds source files | Production Playgrounds editor `GET /files/{appUuid}` | screencloud-pp-cli playgrounds files get | Scoped-token handling, local directory output, and `lastModified` receipt |
| 23 | Update Playgrounds source files | Production Playgrounds editor `PUT /files/{appUuid}` | screencloud-pp-cli playgrounds files put | Dry-run diff, explicit target, expected `lastModified`, confirmation, and no token persistence |
| 24 | Read Playgrounds application data | Production Playgrounds editor `GET /data/{appUuid}` | screencloud-pp-cli playgrounds data get | Optional user-selected file output; private JSON is excluded from metadata sync by default |
| 25 | Update Playgrounds application data | Production Playgrounds editor `PUT /data/{appUuid}` | screencloud-pp-cli playgrounds data put | Local validation, dry-run diff, explicit target, and confirmation |
| 26 | Use the Playgrounds preview workspace | Production editor `<appUuid>-preview` convention | screencloud-pp-cli playgrounds preview | Keeps preview identifiers separate from published content and makes the target visible |
| 27 | Fetch the assembled viewer package | Production Playgrounds viewer `GET /apps/{appUuid}` | screencloud-pp-cli playgrounds viewer get | Uses a viewer-scoped token, reports content metadata, and avoids dumping package HTML by default |
| 28 | Synchronize high-gravity Studio metadata | Printing Press framework | screencloud-pp-cli sync | Bounded Relay pagination into SQLite with secrets and private Playgrounds content excluded |
| 29 | Search and analyze synchronized metadata | Printing Press framework | screencloud-pp-cli search | Offline FTS, selected output, and composable JSON/CSV across app, space, installation, version, and instance metadata |
| 30 | Check auth, reachability, schema assumptions, and local-store health | Printing Press framework plus live research | screencloud-pp-cli doctor | Understands GraphQL HTTP-200 errors, organization mismatch, disabled introspection, and Playgrounds contract risk |
| 31 | Discover the complete published Studio schema | Official Studio GraphQL v2.103.0 sitemap: 386 queries, 319 mutations, and 1,198 supporting type pages | screencloud-pp-cli graphql atlas | Regenerated searchable index with direct official links, operation/type categories, product-domain summaries, and a version/drift banner |
| 32 | Synchronize actual content-placement topology | Official `AppInstance`, `Channel`, `Playlist`, `Screen`, `Association`, and `ShareAssociation` relationships | (behavior in screencloud-pp-cli sync) bounded channels, playlists, screens, associations, and share-associations | Makes direct casts, nested placement, and cross-space sharing locally joinable while surfacing freshness and query cost |
| 33 | Inspect token, user, and permission metadata | Studio GraphQL `currentToken`, `currentUser`, and `permissionsList` | (behavior in screencloud-pp-cli auth capabilities) generated endpoint behavior | Reads permission structure without printing raw effective grants, user objects, or token material |

## Explicit exclusions

- The deprecated ScreenCloud Signage REST API is documented but not promoted as a new-integration command surface.
- The crowd-sniff WorkRamp endpoint is unrelated to ScreenCloud administration or Playgrounds and contributes no feature.
- No public ScreenCloud MCP server or maintained dedicated CLI was found to absorb.
- Private Playgrounds source code and JSON data are not synchronized into the default metadata database.
- Live mutations are not authorized by this manifest, generation, or read-only dogfooding. They require the separate mutation-sandbox prerequisites and a fresh approval for each write stage.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Release impact map | `playgrounds impact <app-uuid> --dir <path>` | 10/10 | hand-code | This uses Playgrounds file/data reads or a user-selected working-copy baseline plus a bounded, freshness-stamped local graph of app instances, direct casts, associations, share associations, channels, playlists, and screens to compute the actual display blast radius of a reviewed change with no external dependencies. It distinguishes direct versus nested placement and marks permission-incomplete or stale results instead of claiming false completeness. | Maya's weekly publish ritual; official placement/share topology; successful bounded live read-only traversal; official Playgrounds file/data contract | Use this command to preview which spaces, channels, playlists, and screens could display a specific local Playgrounds change. Do NOT use it for organization-wide configuration health; use `playgrounds readiness` instead. |
| 2 | Fleet readiness audit | `playgrounds readiness [--app-uuid <id>]` | 10/10 | hand-code | This uses synced local SQLite records for spaces, apps, app versions, app installations, app instances, and sanitized Playgrounds metadata to compute missing, inactive, outdated, dangling, and inconsistent deployment findings with no external dependencies. | Andre's weekly inventory ritual; official `Space`, `AppInstall`, `AppInstance`, `App`, and `AppVersion` relationships | Use this command for organization-wide Playgrounds lifecycle and configuration health. Do NOT use it to calculate the deployment scope of a local file change; use `playgrounds impact` instead. |
| 3 | Privacy-preserving config drift | `playgrounds config-drift --app-uuid <id>` | 9/10 | hand-code | This uses sanitized configuration keys from synced app-instance records plus local space and instance mappings to compute key-set fingerprints and divergent-instance groups without storing values or using external dependencies. | Andre/Ravi configuration checks; live instances expose config keys while project policy excludes private values | Use this command to compare configuration structure across instances without reading private values. Do NOT use it for broader installation or version health; use `playgrounds readiness` instead. |
| 4 | Interrupted-create reconciler | `playgrounds create-reconcile --receipt <path>` | 9/10 | hand-code | This uses a redacted operation receipt, Studio app-instance queries, and Playgrounds file/data reads to compute idempotent resume, cleanup, or no-op actions after a partial create-from-template operation with no external dependencies. | Maya/Lena/Ravi create-from-template ritual; live architecture proves instance creation and content upload cross two services | none |
| 5 | Playgrounds contract check | `playgrounds contract-check --app-uuid <id>` | 9/10 | hand-code | This uses Studio's scoped-JWT operation, Playgrounds file/data reads, and the viewer-package read to compute pass/fail assertions for authentication boundaries, required response fields, `lastModified`, and package availability with no external dependencies. | Ravi's automation ritual; current Playgrounds contract is bundle-derived and medium-risk despite successful live read-only probes | none |
| 6 | Preview drift queue | `playgrounds preview-drift [--older-than 7d]` | 9/10 | hand-code | This uses app-instance-to-`appUuid` mappings and sanitized `lastModified` metadata for `<appUuid>` and `<appUuid>-preview` workspaces to compute unpublished, production-ahead, and aging-preview findings with no external dependencies. | Lena/Maya preview ritual; official production editor establishes the preview suffix and `lastModified` boundary | Use this command to find preview-versus-production timestamp drift and abandoned preview work. Do NOT use it for full installation, version, or configuration health; use `playgrounds readiness` instead. |
| 7 | Least-privilege capability matrix | `auth capabilities [--for <command>]` | 10/10 | hand-code | This reads `currentToken`, `currentUser`, and `permissionsList`, maps command families to the published domain/action catalog, and computes available, missing, or unknown capability states without mutations, external dependencies, or disclosure of raw grants. Unknown and partially visible states fail closed. | Ravi's automation ritual; official API-key permission selection; successful live read-only structural probe covering 25 permission domains | Use this command before automation or a guarded mutation to explain whether the current identity appears capable of the requested command. Do NOT use it as proof that a mutation will succeed or as a substitute for the mutation sandbox approval gate. |

All seven rows require hand-written Cobra commands and `root.go` wiring after generation. No transcendence row is marked `spec-emits`.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Target explainer | The absorbed app-debugging diagnostic already traces the same entities, so a one-instance variant is a thin wrapper. | Fleet readiness audit |
| Orphan scanner | Orphaned content and dangling instances are readiness finding types, not an independent weekly workflow. | Fleet readiness audit |
| Rollout cohorts | Reorganizes readiness/config-drift fields without producing a distinct decision. | Privacy-preserving config drift |
| GraphQL cost profiler | Relay pagination and GraphQL query-cost visibility are already absorbed into the core client. | Playgrounds contract check |
| Preview package parity | The researched contract provides no reversible mapping from assembled viewer HTML to its source files/data. | Preview drift queue |
| Atomic preview promotion | Studio and Playgrounds expose no shared transaction; safe push and partial-failure reporting are already required. | Release impact map |
| One-command rollback | Requires private snapshots or server-side revision history that the contract does not provide. | Interrupted-create reconciler |
| Template recommender | Requires semantic classification and has no useful mechanical version in the template metadata. | Interrupted-create reconciler |
| Release receipt builder | Duplicates mandatory redaction, targeting, drift, and partial-failure behavior. | Interrupted-create reconciler |

## Stub approvals

None. Every absorbed and transcendence row is shipping scope; any later inability to implement one requires returning to the Phase 1.5 gate rather than silently downgrading it.
