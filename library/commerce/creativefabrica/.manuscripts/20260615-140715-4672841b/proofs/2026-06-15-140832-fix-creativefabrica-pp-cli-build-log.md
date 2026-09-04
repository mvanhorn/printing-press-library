Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Creative Fabrica CLI Build Log

## Architecture
- 100% Algolia catalog surface over standard HTTPS. No Cloudflare, no browser at runtime.
- internal/algolia: hand-written client + runtime credential resolution
  (env -> cache -> best-effort discovery). Public app id default; key never compiled in.
- internal/snapshot: JSON snapshot store for new-since change tracking (the 20M-item
  catalog can't be bulk-synced, so per-tracker objectID sets are the right local state).
- flexString/flexFloat tolerate the index returning price as number-or-string and
  regularPrice as false-or-string.

## Built — Absorbed (13)
find (live search w/ --type --category --designer --sort --max-price --on-sale --page --limit),
free, pod, designer, product, categories, types. Plus auth (set-key/status), doctor.

## Built — Transcendence (7/7)
1. find --format svg,dxf,...  (local FTS over tags/title; not a server facet) ✓
2. deals  (ranks by true regular->sale discount %) ✓
3. designer-stats  (local aggregation: type mix, price band, free/POD counts, newest) ✓
4. designer-compare  (two designers side-by-side) ✓
5. new-since  (snapshot diff of newest objectIDs) ✓
6. find --no-subscription  (local post-filter on outsideSubscription) ✓
7. tags  (local tag/category frequency rollup) ✓

## Verified live
find/free/pod/deals/designer-stats/designer-compare/tags/categories/types/product/new-since
all return correct live data. CleanText applied (HTML entities). Prices rounded. doctor OK.
go vet clean; unit tests pass (algolia, snapshot, cli).

## Deferred (documented, not v1 scope)
Authenticated personal library/favorites/downloads (cookie-auth GraphQL gateway);
curated 24h daily-gifts; content index (prod_items). Addable via /printing-press-amend.

## Intentional
- new-since uses file-based snapshots, not a bulk SQLite mirror (catalog is 20M+ items).
- Credential auto-discovery is best-effort (CF blocks plain HTTP on the homepage);
  env var / 'auth set-key' is the reliable path. Full uTLS zero-setup discovery = polish item.
