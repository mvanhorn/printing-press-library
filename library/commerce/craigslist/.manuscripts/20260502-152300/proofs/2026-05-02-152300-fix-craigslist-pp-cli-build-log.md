# craigslist-pp-cli Build Log

## Phase 2 — Generate

- Spec: `internal YAML kind=synthetic`, single resource `postings.search` against `sapi.craigslist.org/web/v8`.
- All 7 generation quality gates passed: `go mod tidy`, `go vet`, `go build`, binary build, `--help`, `version`, `doctor`.
- Bundled MCPB at `build/craigslist-pp-mcp-darwin-arm64.mcpb`.

## Phase 3 — Build

### Foundation packages (in-session)

- `internal/source/craigslist/`
  - `client.go` — typed HTTP client. cl_b cookie jar, Mozilla UA, AdaptiveLimiter, 3-attempt 429/503 retry, RateLimitError surface for 403 anti-bot and hCaptcha responses.
  - `sapi.go` — typed `Search(ctx, site, SearchQuery)` wrapping `sapi.craigslist.org/web/v8/postings/search/full`. Decodes the positional-array `data.items` shape with the `[type_code, value]` typed entries (codes 4=images, 6=slug, 10=priceDisplay, 13=uuid). Computes canonical URL from site/slug/PID. PostedAt computed as `maxPostedDate - postedDelta`.
  - `rapi.go` — typed `GetListing(ctx, uuid)` wrapping `rapi.craigslist.org/web/v8/postings/<uuid>?lang=en`. HTML body stripped to plain text.
  - `reference.go` — typed `GetCategories(ctx)` (178 categories) and `GetAreas(ctx)` (707 areas with subareas) from `reference.craigslist.org`.
  - `sitemap.go` — typed `FreshListings(ctx, site, ymd, metaCat)` and `FreshListingsWindow` for the day+category sitemap surfaces. `MetaCategoryFromAbbr` mapping fine-grained abbrs to top-level meta-cats (sss/hhh/jjj/etc).
  - `sapi_test.go` — 4 unit tests covering positional decode against live ipad fixture (360 items, UUID/title/slug present, >50% with images), default-value vs explicit-flag query params, and location-encoding parser.
  - `testdata/` — saved live fixtures from 2026-05-02 probe (sapi-search, rapi-detail, areas, categories).

- `internal/store/`
  - `cl_tables.go` — `EnsureCLTables(ctx)` migration runner. Creates 9 tables: `listings` (+ FTS5 `listings_fts`), `listing_images`, `listing_snapshots`, `cl_areas`, `cl_categories`, `saved_searches`, `seen_listings`, `favorites` plus indexes on hot lookups (site/cat, uuid, posted_at, price, hostname).
  - `cl_listings.go` — typed `CLListing` shape; `UpsertListing` (single transaction: listings UPSERT + snapshot INSERT-OR-IGNORE + image rewrite + FTS5 upsert with delete-on-conflict fallback); `GetListing`, `GetSnapshots`; `SaveArea`, `SaveCategory`; `CountListings/Areas/Categories` helpers; SHA-1 body hash for snapshot deltas.

### Commands (sub-agent delegation)

17 new top-level commands wired into `internal/cli/root.go`:

| Group | Command(s) | File |
|-------|-----------|------|
| Reference | `categories list`, `areas list`, `catalog refresh` | `categories.go`, `areas.go`, `catalog.go` |
| Search | `search [query]` (cross-city, --negate, --posted-since), `listing get|get-by-pid|images`, `filters show <cat>` | `search.go`, `listing.go`, `filters.go` |
| Sync | `cl-sync --site --category` | `cl_sync.go` |
| Watch | `watch save|list|show|delete|run|tail` | `watch.go` |
| Drift / dedup / score | `drift <pid>`, `dupe-cluster`, `scam-score <pid>` | `drift.go`, `dupecluster.go`, `scamscore.go` |
| Aggregations | `median`, `since`, `reposts`, `cities heat` | `median.go`, `since.go`, `reposts.go`, `cities.go` |
| Geo / faves | `geo within|bbox`, `favorite add|list|remove` | `geo.go`, `favorite.go` |
| Helpers | `cl_storehelp.go` (shared `openCLStore`, `parseDuration`) | — |

### Tests added (new files)

`scamscore_test.go` (7 funcs), `dupecluster_test.go` (6 funcs, simhash + clustering), `median_test.go` (5 funcs, percentile math), `search_test.go` (7 funcs, splitNegate/applyNegate/applyPostedSince/siteList), `since_test.go` (3 funcs).

### Verification

- `go build -o ./craigslist-pp-cli ./cmd/craigslist-pp-cli` → success.
- `go test ./...` → 133 passed, 0 failed across 12 packages.
- `go vet ./...` → clean.
- `gofmt -w` → applied; idempotent.
- All 18 spec-required dry-run probes exit 0:
  `search`, `listing get`, `scam-score`, `drift`, `median`, `since`, `dupe-cluster`, `favorite list`, `watch list`, `watch save`, `categories list`, `areas list`, `catalog refresh`, `geo within`, `reposts`, `cities heat`, `filters show`, `cl-sync`.
- `printing-press validate-narrative` → 11/11 narrative quickstart + recipe commands resolve in the binary.

### Helper reuse

- `cliutil.FanoutRun` powers cross-site fanout in `search`, `cities heat`, `watch run`.
- `cliutil.CleanText` normalizes titles in table output.
- `cliutil.IsVerifyEnv` short-circuits `watch run` and `watch tail` (side-effect convention).
- `cliutil.AdaptiveLimiter` is in use transitively through every `craigslist.New(1.0)` call (default 1 req/s, ramped up on success, halved on 429/403 anti-bot).

### MCP / annotation hygiene

Every new command has `Annotations: map[string]string{"mcp:read-only": "true"}`. No `Args: cobra.MinimumNArgs(...)`, no `MarkFlagRequired`, no `os.Exit` in `RunE`. Empty-arg invocations fall through to `cmd.Help()`.

### No new go.mod deps

Pure-Go SimHash via FNV-1a + `math/bits`. Percentile sort via stdlib.

### Intentionally deferred / known limits

- `since --with-detail` flag is parsed but not wired (the URL-only contract is the v1 spec; rapi hydration on demand can be added later).
- `geo` zero-value detection: `lat==0 && lng==0 && radius==0` falls through to `cmd.Help()`. Equator coverage is impossible with that gate; documented and tested.
- `applyNegate` does substring matching, not word-boundary. "unfurnished" matches "furnished" — documented; full regex word-boundary would need a follow-up.
- `scam-score`'s "below 50% of category median" rule needs a populated `cl_categories` + `listings` store to fire; a missing-store invocation surfaces `listing %d not in local store; run cl-sync first` as the explicit hint.
- v1 is read-only by design. No `auth login`, no posting, no renew. `accounts.craigslist.org` is the most-monitored CL surface and out of scope per the brief.

## Heartbeat

Lock heartbeats updated through generate, build-p1-progress phases. Lock active under scope `hidden-brewing-turtle-68cd6401`.
