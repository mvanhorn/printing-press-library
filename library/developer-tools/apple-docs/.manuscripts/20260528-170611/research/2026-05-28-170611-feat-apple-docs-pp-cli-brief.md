# Apple Developer Docs (DocC) CLI Brief

## API Identity
- **Domain:** Apple Developer Documentation — the JSON layer that powers developer.apple.com's docs SPA (Swift, SwiftUI, Foundation, UIKit, every Apple framework, plus sample code and WWDC video metadata).
- **Users:** Swift/iOS/macOS/visionOS engineers; AI agents that author Swift code on behalf of users; technical writers and educators citing Apple APIs; researchers studying API deprecation trends.
- **Data profile:** Public, unauthenticated, structured JSON at `/tutorials/data/documentation/<framework>[/<path>...].json` and `/tutorials/data/index/<framework>.json`. Each framework root is ~20–40 KB, each symbol page is 50 KB–500 KB, each framework index is ~1 MB.

## Reachability Risk
- **None** — confirmed 200 OK across 40+ framework slugs probed today (`swiftui`, `foundation`, `uikit`, `appkit`, `combine`, `coredata`, `swiftdata`, `arkit`, `realitykit`, `metal`, `coreml`, `mapkit`, `webkit`, `avfoundation`, `healthkit`, `storekit`, `widgetkit`, `appintents`, `observation`, `vision`, `visionkit`, `visionos`, `speech`, `photos`, `photokit`, `screencapturekit`, `xcode`, `passkit`, `usernotifications`, `network`, `security`, `coregraphics`, `corelocation`, `corebluetooth`, `eventkit`, `notificationcenter`, `accessibility`, `driverkit`, `endpointsecurity`, `gamekit`, `quartzcore`, `accelerate`, `localauthentication`, `audiotoolbox`, `spritekit`, `scenekit`, `contacts`). Served by Apple's CDN; no anti-bot, no auth, no rate limit observed in light probing.
- 404s confirm strict slug spelling: `photoskit` 404 / `photokit` 200; `applepay` 404; `xcodecloud` 404; `persistentcollection` 404. Lowercase, no separators, matches the URL path component after `/documentation/`.
- Tier hints: none.
- Probe-safe endpoint used: `GET /tutorials/data/documentation/swift.json` (200 OK, 19 KB).

## Top Workflows
1. **Cross-framework symbol lookup** — "where is `View.onAppear` documented?" → fetch index, resolve symbol, render abstract + declaration + see-also.
2. **Deprecation cliff detection** — "show every SwiftUI API deprecated in iOS 18" → walk `metadata.platforms[].deprecatedAt` across a framework's index.
3. **Markdown for LLM context** — "convert the View protocol page to Markdown" — sosumi.ai already proves the demand; we ship it offline.
4. **Symbol diff between releases** — "what changed in `Task` between Swift 5.10 and 6.0" — fetch archived snapshots, diff declaration fragments and platforms.
5. **Sample-code grep** — "find SwiftUI sample-code projects using `@Observable`" — `references{}` carries `kind:"sampleCode"` entries; full-text on titles/abstracts.
6. **WWDC ↔ API map** — references include `kind:"video"`; reverse-index video → symbols cited.

## Table Stakes (from competitors)
Drawn from kimsungwhee/apple-docs-mcp (1.3k stars, 18 MCP tools — the dominant competitor), Ahrentlov/appledeepdoc-mcp (15 tools), sosumi.ai (4 MCP tools), Abdullah4AI/apple-developer-toolkit (8 Go subcommands), Dash.app / Zeal (offline docset browsers):

- search apple docs by keyword (kimsungwhee, sosumi, Abdullah, Dash)
- get a single doc page by URL or symbol identifier (kimsungwhee, sosumi, Abdullah)
- list all frameworks / technologies (kimsungwhee, Abdullah)
- list symbols inside a framework (kimsungwhee `search_framework_symbols`, Abdullah `symbols`)
- get related APIs / see-also (kimsungwhee `get_related_apis`)
- get platform compatibility (kimsungwhee `get_platform_compatibility`)
- find similar/alternative APIs (kimsungwhee `find_similar_apis`)
- WWDC video search + transcripts (kimsungwhee — 1,260+ videos indexed)
- sample-code search (kimsungwhee `get_sample_code`, Abdullah `samples`)
- recent documentation updates feed (kimsungwhee `get_documentation_updates`)
- Swift Evolution / HIG access (Ahrentlov)
- offline docset (Dash, Zeal) — but these are GUI-only
- Markdown rendering of docs (sosumi)

## Data Layer
- **Primary entities:**
  - `framework` (e.g. SwiftUI) — slug, role, modules, abstract, topic-section structure
  - `symbol` (e.g. SwiftUI.View) — identifier URL, kind (protocol/struct/class/func/var), role (symbol/article/sampleCode), title, abstract, declaration fragments, platforms[], topicSections, references, deprecatedAt, introducedAt
  - `topic` (e.g. "Creating a view") — parent symbol, anchor, identifiers (links to symbols)
  - `reference` (the cross-link target) — identifier URL, kind, title, role, abstract, url (web path), images[]
  - `platform_availability` (per-symbol-per-platform) — platform name (iOS/macOS/watchOS/tvOS/visionOS), introducedAt, deprecatedAt
  - `sample_code` — referenced under `references{}` with `kind:"sampleCode"`
  - `video` — referenced under `references{}` with `kind:"video"` (WWDC sessions)
- **Sync cursor:** none server-side. Each fetch is a stateless GET. For "what changed today" we keep ETags / Last-Modified per URL and re-fetch.
- **FTS:** SQLite FTS5 on symbol title + abstract + declaration fragments + topic titles, scoped by framework + role + platform.

## Codebase Intelligence
- **Source:** `apple/swift-docc-render` (Vue.js SPA that consumes the same JSON our CLI will). The `RenderNode`/`RenderReference` Swift types in `apple/swift-docc` are the authoritative schema definitions. `schemaVersion: {major:0, minor:3, patch:0}`.
- **Confirmed top-level keys on framework/symbol JSON:** `sections`, `schemaVersion`, `hierarchy.paths`, `abstract`, `topicSections[{title, identifiers[], anchor}]`, `primaryContentSections`, `variants`, `kind`, `identifier`, `metadata.{role, symbolKind, modules, platforms[{name, introducedAt, deprecatedAt?}]}`, `references{<doc://...>: {identifier, title, kind, role, type, abstract[], url}}`, `legalNotices`.
- **Symbol identifier format:** `doc://com.apple.<Module>/documentation/<Module>/<Symbol>[/<Member>...]`.
- **Auth:** none. The endpoints are served by the public Apple CDN.
- **Rate limiting:** none observed (light probing). Apple's CDN absorbs traffic; respect normal HTTP caching headers (`Cache-Control: max-age=300, public` observed on root).

## User Vision
- (User chose "Let's go" at the briefing — no upfront vision captured beyond the docs-target choice.)

## Product Thesis
- **Name:** `apple-docs-pp-cli` (slug: `apple-docs`)
- **Why it should exist:** No mature Go CLI ships both a binary and an MCP server with deprecation-diff and cross-framework-grep as first-class commands. The leader (kimsungwhee, 1.3k stars) is MCP-only Node.js with read-shaped tools. Dash/Zeal are GUIs. Apple's own `swift-docc` only *produces* archives. There is a white space for: (a) a real CLI you can pipe to `jq`, (b) offline SQLite FTS so search works on a plane, (c) deprecation/availability analytics that mine `metadata.platforms[]` programmatically, (d) Markdown emission for feeding agents context, (e) a single binary that wraps all of the above plus an MCP server you can plug into Claude Desktop.
- **Differentiator:** The transcendence features below (cross-framework grep, deprecation-cliff report, symbol-diff between releases, conformance/inheritance graphs) are only practical when every framework's index lives in local SQLite. Network-bound MCP servers can't do them.

## Build Priorities
1. Foundation: HTTP client against `/tutorials/data/`, SQLite store for `frameworks`, `symbols`, `references`, `platforms`, `topics`, sync command that walks `/tutorials/data/index/<framework>.json` per framework.
2. Absorbed: search, get, list frameworks, list symbols, related-apis, platform-compatibility, sample-code list, WWDC video search (subset), Markdown emission, recent-updates feed, find-similar-apis.
3. Transcendence (hand-coded, see Phase 1.5): cross-framework grep, deprecation-cliff report, snapshot-diff between two saved snapshots, conformance graph, port-to (cross-platform replacement walker), token-lean doc-get projection, WWDC reverse index, agent-shape doc bundle.

## Reachability Gate
- Decision: PASS
- Reason: probe-confirmed-200
- Evidence: `GET /tutorials/data/documentation/swift.json` → HTTP 200, 19 KB. 40+ framework slugs (`swiftui`, `foundation`, `uikit`, `appkit`, `combine`, `coredata`, `swiftdata`, `arkit`, `realitykit`, `metal`, `coreml`, `mapkit`, `webkit`, `avfoundation`, `healthkit`, `storekit`, `widgetkit`, `appintents`, `observation`, `vision`, `visionkit`, `visionos`, `speech`, `photos`, `photokit`, `screencapturekit`, `xcode`, `passkit`, `usernotifications`, `network`, `security`, `coregraphics`, `corelocation`, `corebluetooth`, `eventkit`, `notificationcenter`, `accessibility`, `driverkit`, `endpointsecurity`, `gamekit`, `quartzcore`, `accelerate`, `localauthentication`, `audiotoolbox`, `spritekit`, `scenekit`, `contacts`) all returned 200. `technologies.json` returned 256 KB master listing. No anti-bot, no auth, no rate limit observed.
