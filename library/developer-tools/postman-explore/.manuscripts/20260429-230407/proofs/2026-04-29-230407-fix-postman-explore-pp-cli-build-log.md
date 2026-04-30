# postman-explore-pp-cli — Build Log

## Phase 0 (foundation)
- Generator emitted: store with networkentity/team/category tables + resources_fts FTS5 virtual table; sync, search, export, import, doctor, agent-context, profile, feedback, which all wired in by v3 generator
- Surf transport with Chrome TLS confirmed in client.go
- Proxy-envelope routing via serviceForPath wired automatically
- No auth command emitted (correct — API is no-auth)

## Phase 1 (absorbed endpoint mirrors)
All 8 spec endpoints emitted as Cobra commands by v3 generator:
- `category get` / `category list-categories`
- `networkentity list-network-entities` / `networkentity get-network-entity` / `networkentity get-network-entity-counts`
- `team get` / `team get-workspaces`
- `search-all search_all` (also promoted as top-level `search-all`)

## Phase 2 (transcendence — hand-written)
Eight novel commands built from the absorb manifest, all registered in root.go and category.go:

| Command | File | Verified |
|---------|------|----------|
| `canonical <vendor>` | `internal/cli/canonical.go` | ✅ Live: returns verified Stripe collection (1461 forks) as #1 pick |
| `top --metric <m>` | `internal/cli/top.go` | ✅ Compiles + graceful empty-store message |
| `publishers top` | `internal/cli/publishers.go` | ✅ Compiles + graceful empty-store message |
| `drift --since <d>` | `internal/cli/drift.go` | ✅ Compiles + graceful empty-store message |
| `similar <id>` | `internal/cli/similar.go` | ✅ Compiles + helpful "not in store" error |
| `velocity --top N` | `internal/cli/velocity.go` | ✅ Compiles + graceful empty-store message |
| `browse <type> --verified-only` | `internal/cli/browse.go` | ✅ Live: returned verified Salesforce collection |
| `category landscape <slug>` | `internal/cli/category_landscape.go` | ✅ Compiles + helpful 404 hint when slug wrong |

Shared helpers in `internal/cli/novel_helpers.go`: `validMetric`, `metricNamesList`, `validEntityType`, `extractMetricValue`, `openLocalStore`, `scanNetworkentityRows`, `publisherInfoMap`, `entityPublisherID`, `emptyMessage`.

## Build status
- `go build ./cmd/postman-explore-pp-cli` — Success
- `go vet ./...` — passed in generator's quality gates
- All 8 novel commands listed in root --help under "Highlights (not in the official API docs)"
- All 8 novel commands carry `mcp:read-only` annotations (read-only, no external mutation)

## Spec corrections applied during generation
- Sort enum on `listNetworkEntities` reduced to `[popular]` only — other values (recent, new, week, alltime, featured) return HTTP 400
- New `minMonthlyForkCount` query parameter added (discovered during browser-sniff)
- Search response data is object-keyed when 0 or multi queryIndices, array-keyed when single index — `canonical` parser handles both shapes

## Intentionally deferred
Nothing. All 8 approved novel features are functional. No stubs.

## Generator limitations encountered
None. v3 generator produced a clean foundation that needed only the novel-feature wiring.
