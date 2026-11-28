Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

## What Was Built

### Slice 1 — Foundation (`internal/adlibrary/` + `internal/config/config.go`)
- `internal/adlibrary/models.go`: Normalized `Ad`, `CreativeCard`, `Advertiser`, `AdDetails`, `BeneficiaryPayer`, `DemographicEntry`, `SearchParams`, `SearchResult`, `MediaFile`, `MediaDownloadReport`; Meta redirect unwrapper (`UnwrapRedirectURL`), domain/path/UTM extractors, Meta Ad Library URL parser/builder (`ParseAdLibraryURL`, `BuildAdLibraryURL`), mechanical opening hook extractor (`ExtractOpeningHook`), and runtime age calculator (`ComputeDaysRunning`).
- `internal/adlibrary/client.go`: Zero-auth pure-HTTP GraphQL client using `cliutil.AdaptiveLimiter` and `*cliutil.RateLimitError`. Bootstraps anonymous `lsd`/`__spin_r`/`__hsi`/`__hs`/cookies from `GET https://www.facebook.com/`, computes `jazoest`, strips `for (;;);`, handles HTTP 403 and session errors `1357001`/`1357004` via automatic session refresh, handles HTTP 429 and GraphQL error `1675004` with backoff + `*cliutil.RateLimitError`, and streams creative images/videos via `StreamHTTP`.
- `internal/adlibrary/store.go`: SQLite tables (`meta_ads`, `meta_advertisers`, `meta_snapshots`, `meta_snapshot_ads`, `meta_ad_details`) layered on `*store.Store`, committing custom transactions before mirroring into framework `resources_fts` via `store.Upsert` (WAL-safe, drain-first queries throughout).
- `internal/adlibrary/adlibrary_test.go`: Table-driven unit tests covering URL unwrapping, URL parsing/building, opening hook extraction, mock GraphQL bootstrap/search/typeahead/details/media/429/403 refresh, SQLite persistence, and all 7 analytics computations.
- `internal/config/config.go`: Added `META_ADS_ARCHIVE_TOKEN` and `FACEBOOK_ACCESS_TOKEN` fallback env vars alongside `META_ACCESS_TOKEN`.

### Slice 2 — Absorbed Public Commands + Official Archive Enhancement
- `internal/cli/ads.go`: `ads search` (global zero-auth search, `--url` parser, `--page-name` typeahead lookup, `l.facebook.com` unwrapping, SQLite & snapshot persistence), `ads details` (`AdLibraryV3AdDetailsQuery`, auto-resolves `page_id` from SQLite when cached), `ads url` (computed deep-link builder).
- `internal/cli/advertisers.go`: `advertisers search` (`useAdLibraryTypeaheadSuggestionDataSourceQuery`, SQLite persistence).
- `internal/cli/media.go`: `media download` (streaming creative image/video downloader with `--dry-run`, `--images-only`, `--videos-only`).
- `internal/cli/promoted_archive.go`: Official Graph API v27.0 `archive` endpoint enhanced with automatic FULL-to-CORE field fallback when a token lacks EU-only field permissions.
- `internal/cli/sync.go`: Populated `defaultSyncResources()` with `"ads"` and wired `syncPublicAds` so `sync` works zero-auth out of the box.
- `internal/cli/helpers.go`: Pruned unused `endpointPositionalArgs` family (0 dead functions).

### Slice 3 — All 7 Novel Transcendence Commands
1. `internal/cli/winners.go` (`winners [page-or-id]`) — `pp:data-source local`
2. `internal/cli/diff.go` (`diff [page-or-id]`) — `pp:data-source auto` (local snapshot diff + `--refresh` live snapshot capture)
3. `internal/cli/compare.go` (`compare [page1] [page2] [page3...]`) — `pp:data-source local`
4. `internal/cli/hooks.go` (`hooks [page-or-id]`) — `pp:data-source local`
5. `internal/cli/funnels.go` (`funnels [page-or-id]`) — `pp:data-source local`
6. `internal/cli/transparency.go` (`transparency [page-or-id]`) — `pp:data-source local`
7. `internal/cli/sov.go` (`sov [keyword]`) — `pp:data-source auto` (local FTS5 + `--live` GraphQL sweep)
- `internal/cli/meta_ads_behavior_test.go`: End-to-end mock CLI tests for all absorbed commands and both empty-mirror and populated-mirror behavioral assertions for all 7 novel commands.

## Deferred / Skipped / Generator Limitations
- **Intentionally deferred:** None (`0` stubs; all 18 approved features shipped).
- **Skipped body fields:** None (read-only API).
- **Generator notes:**
  1. `tier_routing` with `default_tier: free` (`auth: none`) + credential tier omitted `authHeaderLooksLikePlaceholderCredential` in `cli-printing-press v4.35.4`; resolved cleanly by declaring root `auth` for the spec's official `archive` resource while public GraphQL commands use the zero-auth `internal/adlibrary` client.
  2. `cli-printing-press v4.35.4` emitted `go 1.26.10` in `go.mod`, which flagged `GO-2026-6859` in `govulncheck` (published 2026-11-24, fixed in `go 1.26.11`); bumped `go.mod` to `go 1.26.11` and verified `GOTOOLCHAIN=go1.26.11 govulncheck ./...` passes with 0 vulnerabilities.

## Phase 3 Gate Verification
- `go vet ./...` — PASS
- `go test -count=1 ./...` — PASS (all packages including `internal/adlibrary` and `internal/cli` behavioral tests)
- Priority 1 review gate (`--help`, `--dry-run --json`) — PASS
- Per-row Cobra leaf `[flags]` resolution across all 15 commands — PASS
- `cli-printing-press dogfood --dir "$CLI_WORK_DIR" --research-dir "$API_RUN_DIR" --json` — **PASS** (0 issues, `novel_features_check: 7/7`, `dead_flags: 0`, `dead_functions: 0`, `wiring_check: 70/70`)
