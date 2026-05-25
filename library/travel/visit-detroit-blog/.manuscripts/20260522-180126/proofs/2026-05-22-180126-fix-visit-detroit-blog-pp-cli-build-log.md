# Inside the D CLI — Build Log

## What was built

**Foundation (P0)**
- Generated the module + foundation (root, config, client, store, MCP cobratree, doctor, analytics, sync, tail, import) from the internal YAML spec via `printing-press generate` (all 7 quality gates passed).
- Hand-built `internal/cli/algolia.go`: Algolia query/page/sync helpers. `syncBlogsAlgolia` pages the `prod-visit-detroit-listings` index filtered to `sectionName:Blogs`, upserting all hits into the store as `resource_type=blogs`. Uses `PostQueryWithParamsAndHeaders` (read-only POST → reaches transport under verify). Public search-only Algolia key isolated here with an `VISIT_DETROIT_BLOG_ALGOLIA_API_KEY` env override.
- Patched generated `sync.go`: `defaultSyncResources()` → `["blogs"]`; worker routes `blogs` through `syncBlogsAlgolia`.

**Absorbed (P1)** — `internal/cli/blogs.go`
- `search` (offline FTS via store.Search, `--no-sponsored`/`--sponsored-only`, `--limit`)
- `blogs list` (cross-axis: `--category`/`--region`/`--since`/`--until`/`--no-sponsored`/`--limit`)
- `blogs get <slug|uri|id>` (full article body offline)
- `categories`, `regions` (facet counts from the corpus)
- `recent` (newest by postDate, `--since`/`--until`)

**Transcendence (P2)** — `internal/cli/blogs.go`
- `blogs related <slug>` — ranks by shared categories (×2) + shared regions
- `blogs coverage` — category × region cross-tab (default / `--category` / `--region`)
- `blogs reading-list` — ordered md/json/csv export, `--output` file, editorial-only via `--no-sponsored`
- Editorial-only flag shared across list/search/reading-list

All read commands query `internal/store` (offline); `sync` is the only outbound API call (anti-reimplementation: store-access carve-out satisfied). Read commands annotated `mcp:read-only`; `reading-list` is not (writes `--output` files) and short-circuits file writes under `PRINTING_PRESS_VERIFY=1`.

## Verified live (against the real Algolia index)
- `sync` → **748 blogs** in ~1.4s.
- `categories` → 28 categories, **Dining=288** (matches the live facet distribution exactly → proof all 748 load).
- `search "ethiopian"` → correct top hit; `--select`/`--agent`/`--json` work.
- `blogs get donuts`, `blogs related donuts` (scored), cross-axis `blogs list`, `coverage --category Outdoors` (37) all correct.
- Error paths: not-found → exit 3, usage (bad date / mutually-exclusive flags) → exit 2, no-arg → help. Dry-run probes → exit 0.

## Bug found & fixed during build
- `store.List(type, 0)` defaults to **LIMIT 200**, not "all" — `loadAllBlogs` was silently truncating the 748-row corpus (got/related/coverage/categories undercounted; `get donuts` failed). Fixed by querying the store directly with no LIMIT. **Systemic note (retro candidate):** `List(.., 0)` defaulting to 200 is a footgun for "load all" use cases.

## Machine gaps observed (retro candidates)
- The generator emitted **no `search` and no `sql` command** for a POST-search-only spec, even though a store is created. Had to hand-build `search`. A store-backed CLI should always get `search`/`sql`.
- The generated promoted endpoint command (`blogs`) did a raw empty-body POST returning all content unfiltered; replaced entirely.

## Intentionally deferred
- `sql` command (not built; the typed novel commands cover the SQL-composition value; `analytics` covers ad-hoc counts). Low priority — not in quickstart/recipes.
- Live-fetch of full HTML article pages (Algolia `content` already carries clean full bodies, so unnecessary).
- `related` weighting: broad county tags (Wayne/Oakland/Macomb/Downtown) appear on most articles and slightly inflate overlap scores; acceptable for v1 (category weighted 2×, recency tiebreak), candidate for polish.

## Tests
- `internal/cli/blogs_test.go`: table-driven tests for blogSlug, blogURL, containsFold, regionMatch, intersectIn, parseDateFlag, formatFromOutput, csvField, resolveSponsored, blogFilter.match, blogRecord.id.
- `go fmt`, `go vet`, `go test ./...` all clean/pass.
