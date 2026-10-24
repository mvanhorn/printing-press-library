Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.

# app-store-pp-cli build log

Contract: proofs/2026-10-23-151515-build-spec.md. Slices: 1 foundation -> {2a app-page family,
2b charts/search/reviews, 3a local history + search, 3b transcendence} in parallel worktrees -> merge.

## Slice log

### Slice 1 (foundation) — 2026-10-23, build green

Manifest rows: no command rows (foundation for all 36 absorbed + 5 transcendence rows;
transcendence built count stays 0/5). Absorbed row 31 (shared client: per-host pacing,
cache headers, governor, tri-state results, capture log) is implemented here.

**Files** (all slice-1 owned):
- `internal/appstore/`: `client.go` (Client, Options, per-class AdaptiveLimiter, governor,
  manual redirects, 5xx single retry, capture hooks), `cache.go` (file cache), `endpoints.go`
  (Lookup, Search, ChartRSS, ReviewsRSS, Hints, Page/PageOfKind, ProductPage, ChartsPage,
  DeveloperPage, TodayPage, SeeAllPage), `optin.go` (WebSearch, APIReviews, APIApp + parsers),
  `page.go` (serialized-server-data extraction, intent kinds), `parse_product.go`,
  `parse_pages.go` (charts, developer, Today, see-all, search), `parse_feeds.go` (lookup body,
  RSS charts, reviews RSS, hints plist), `types.go`, `jsonnav.go`, `urls.go` (URL builders,
  ParseAppRef, platform/chart/sort normalizers), `storefronts.go` (177 rows: 175 active + BD,
  LI legacy), `genres.go` (iOS 36/60xx/70xx/13xxx/16xxx + Mac 39/12xxx, spec aliases, Kids bands).
- Tests: `internal/appstore/{parse,client,tables}_test.go`, `live_test.go` (build tag `live`,
  5 requests, never in `go test ./...`); 21 sanitized fixtures in `internal/appstore/testdata/`
  (all < 100 KB; reviewer names/titles/bodies synthetic; userTokenHash dropped; a leak check
  of 508 real names/titles/bodies from the raw captures found 0 hits).
- `internal/store/app_store_store.go` (schema + typed accessors), `internal/store/extras.go`
  (migrateExtras applies appStoreMigrations), `internal/store/app_store_store_test.go`.
- `internal/cli/appstore_common.go` (+ `_test.go`), `internal/cli/root.go` (rootFlags.country/lang
  and the `--country` (default us) / `--lang` persistent flags only).
- `.printing-press-patches/appstore-client.json` (schema_version 2, files + call_sites guard).

**Live acceptance** (5 sequential requests through the client, all HTTP 200, 2026-10-23):
Lookup 324684580 us: "Spotify: Music and Podcasts" 9.1.92, userRatingCount 42,719,156, 44 raw
keys. Chart page us/iphone/36: top-free 25+175 (#1 407558537 Capital One Mobile), top-paid
25+175 (#1 932747118 Shadowrocket $2.99), 26 categories. Spotify product page: totalRatings
42,722,184, histogram [37458149 2996730 950717 373640 942947] sum 42,722,183, 25 versions
(newest 9.1.92 2026-10-19T13:57:56Z), 2 privacy types, accessibility not declared, chart
top-free/6011/1. Reviews RSS 6448311069 p1 mostrecent: 50 entries, ratings {1:7,2:2,3:2,4:1,5:38}.
Hints "insta": 10 suggestions, all start with "insta". Stats: apps-pages 2, itunes 2,
search-hints 1; 5 capture rows.

**Decisions**
- Store accessors are package-level functions `store.X(ctx, st, ...)` (the contract's
  `RecordProductPage(ctx, st, ...)` shape), no raw SQL needed in commands. Series readers are
  oldest first; `Reviews` is newest first; `VersionEntries` newest release first.
- Sources: app_snapshots carry `Source` (`lookup` or `page+lookup`) in fields_json; compare
  like with like. ratings_snapshots source `lookup` (exact userRatingCount) vs `page`
  (productRatings); keep one source per series (traction: prefer `lookup`). chart source
  `web` (200 ranks) vs `rss` (100); `LatestTwoCharts` only pairs same source + row limit.
- `RecordProductPage` writes app_snapshots only when a Lookup row is passed; versions are
  insert-or-ignore; privacy and IAP lists are written only when they change (their History
  readers return change points); events upsert (first/last seen); one `page` ratings row.
- reviews_fts is a standalone FTS5 table (trigram, own title/body copy, rowid = hash of
  review_id+storefront) kept in sync inside `UpsertReviews` (no triggers; VACUUM-safe).
  `SearchReviews` falls back to LIKE for words under 3 characters.
- Annotation titles are localized on non-English storefronts, so the IAP list is detected by
  textPair rows and the age rating by its textEncapsulation badge.
- Opt-in surfaces (web search page, apps.apple.com/api) return `appstore.ErrOptInRequired`
  until `c.EnableOptInSurfaces()`; /api is paced 0.5/s and sends `Authorization: Bearer ` +
  `Origin`. None were called live in this slice.

**Deviations from the contract**
1. Jitter is +0-20% of the class interval (stated rates are ceilings), not +-20%: at 0.3/s a
   -20% gap would let the Search API reach 22.5/min, above Apple's ~20/min.
2. `Lookup` returns `*LookupResponse{Results, Missing, Artist, Requests}` (not a bare slice) so
   missing ids and the developer wrapper row travel with the results.
3. `Page(ctx, url)` has a companion `PageOfKind(ctx, url, kind)`; typed fetchers and parsers
   enforce the expected `$kind`.
4. Bundle-id lookups go one per request (comma lists undocumented); the Lookup/Search `lang`
   parameter is sent only for en_us/ja_jp (others 400) while `--lang` always goes as
   Accept-Language.
5. Redirects are followed manually (each hop paced) and only within itunes.apple.com,
   apps.apple.com and search.itunes.apple.com; the capture log has one row per logical fetch at
   the final URL; cache hits are not logged (not network requests).
6. `--rate-limit` only lowers the per-host rates; auto/0 keep the defaults.
7. Extra fixtures (developer lookup, two opt-in /api samples with synthetic text) and extra
   accessors (RecordVersions/Privacy/IAPs/Events, ChartCaptures, ChartSnapshotAt, ChartRows,
   TrackedApps, SearchApps, SearchReviews, CountReviews) for slices 2a/3a/3b.

**Notes for the next slices** (exact entry points)
- Client: `c, err := newAppStoreClient(flags)`; open the store first and `attachCaptureLog(c, st)`
  to log captures. Classify every client error with `classifyAppStoreErr(err)` (7 throttle /
  3 not found / 2 opt-in / 5 other); in fan-out loops stop and return on `isRateLimited(err)`.
- App args: `ref, err := parseAppRef(arg)`; `cc, err := refCountry(cmd, flags, ref)`;
  `lk, err := lookupAppRef(ctx, c, ref, cc)` (exit 3 with a clear message, e.g. `app 1`).
- Product page: `c.ProductPage(ctx, appstore.ProductPageQuery{URL: lk.CanonicalURL()})` costs 1
  request; `{Country: cc, AppID: id}` costs 2 (slug 301). Persist with
  `store.RecordProductPage(ctx, st, cc, *page, lk)`. Spotify US has no IAP annotation; Candy
  Crush has 10 IAP rows; Pages declares accessibility.
- Charts: `c.ChartsPage(ctx, appstore.ChartsPageQuery{Country, Platform, GenreID, Chart, AgeBandID})`
  -> `seg, _ := page.Segment(chart)`; `seg.Lockups` (25, Position = rank), `seg.RankedIDs()` (200).
  Hydrating 200 ids with `c.Lookup` is 2 requests (batch 150). Grossing:
  `feed, _ := appstore.ChartFeedName("top-grossing", platform)` + `c.ChartRSS(ctx, cc, feed, genre, 100)`.
  Store with `store.RecordChart(ctx, st, store.ChartSnapshot{Source: store.SourceWebChart or
  store.SourceRSS, RowLimit: 200 or 100, ...})`.
- Reviews: `c.ReviewsRSS(ctx, cc, id, page, sort)` (sort recent|helpful); store each page with
  `store.UpsertReviews(ctx, st, store.ReviewUpsert{Storefront, AppID, Sort: feed.Sort, Source:
  store.SourceRSS, Reviews: feed.Reviews})` -> `{Inserted, Updated, Unchanged}`.
  `--source api`: `c.EnableOptInSurfaces()` then `c.APIReviews(...)`, Source `store.SourceAPI`.
- Other live calls: `c.Hints(ctx, cc, term)`, `c.Search(ctx, appstore.SearchQuery{...})`,
  `c.TodayPage(ctx, cc, platform)`, `c.DeveloperPage(ctx, cc, devID, lk.ArtistViewURL)`,
  `c.Lookup(ctx, appstore.LookupQuery{ArtistID: devID})`, `c.SeeAllPage(...)`.
- Args/flags: `parsePlatform`, `parseChartPlatform`, `resolveGenre`, `appstore.ResolveAgeBand`,
  `countriesFlag(flags, raw)` (empty = --country; capped to 2 under dogfood), `capForDogfood(n,
  dogfoodMaxChartLimit|dogfoodMaxReviewPages|dogfoodMaxKeywordScans)`, `printRows(cmd, flags,
  rows)`, `fetchFailure` + `warnFetchFailures(cmd.ErrOrStderr(), failed, total, what)`.
- Local commands: `path, ok := appStoreMirrorExists(cmd)` for the missing-mirror guard,
  `openAppStoreStore(cmd, flags)` (honours a command-local `--db`), then the store readers.
- The generated `keyword_share.go` scaffold declares a local `--country` that shadows the new
  root flag; slice 3b should drop it when replacing the scaffold and use `flags.country`.
- Live budget used by slice 1: 5 of 10 requests.

### Slice 1 fixes — 2026-10-24, build green, commit 06e5ce7 on main

Fixes the five medium review defects in the foundation. No exported function signature changed
(every change is a stricter behaviour, a new exported identifier or a new struct field), so the
slice branches built on b1786f3 merge on top unchanged: a trial merge of slice-2a, 2b, 3a and 3b
onto 06e5ce7's content builds, vets and passes `go test ./...` (internal/cli: 324 PASS, 0 FAIL),
once the pre-existing 2a/3b name collision below is renamed. Parser generation is now
`appstore-parsers/2` (capture rows and cache entries carry it).

**1. Tri-state outcomes need the documented envelope** (`parse_feeds.go`, `optin.go`, `endpoints.go`, `types.go`)
- New `*appstore.EnvelopeError` (+ `IsEnvelopeError`). A wrong-shaped 200 is now a failed capture
  and an error (CLI exit 5 via `classifyAppStoreErr`), never verified-empty, never "missing".
- Lookup/Search: `resultCount` and an array `results` are required; non-empty `errorMessage` is an
  error; `resultCount > 0` with no results is an error. `{}`, `null`, `[]`, other feeds' bodies fail.
- Legacy RSS: the body must be `{"feed": {...}}`. A feed id that names another feed is refused
  (a customer-reviews id for a chart; another chart feed than requested, e.g. top-free for
  top-grossing; for reviews another app id, page or sort than requested). Chart: any entry with
  `im:rating` or entries none of which carries an app id fail. Reviews: more than one entry
  without `im:rating` (a reviews page leads with at most one app-metadata entry) or review
  entries none of which carries an id fail. A feed with no rows is verified-empty only when it
  carries feed metadata (id, updated, author or title).
- Hints: the plist root dict must carry a `hints` array (empty is verified-empty); hints none of
  which has a term fail. /api: `errors` is an error (status/code/title surface in the message), a
  `data` array is required, an app resource must include reviews for ParseAPIReviews, items none
  of which is a user review fail, ParseAPIApp refuses a non-"apps" first item or one without id.
- Chart RSS ranks are now the feed position (a skipped id-less row leaves a gap instead of
  shifting every later rank up); identical for every real capture.
- Tests: `envelope_test.go` (negative tables for {}, null, errorMessage, cross-feed bodies per
  parser, positive controls, endpoint-level capture rows + no caching of failed bodies);
  `appstore_common_test.go` `TestS1LookupAppRefNeedsTheEnvelopeForNotFound` (`{}` -> exit 5, the
  documented empty envelope -> exit 3).

**2. Pages are logged and cached after their typed parse** (`endpoints.go`, `parse_pages.go`)
- One page path `fetchPage`: fetch -> extract model -> kind check -> typed parser for the model's
  kind (product, charts, developer, Today, see-all, search) -> capture row -> cache. `Page`,
  `PageOfKind` (the path slice 2b uses for charts and Today) and every typed page method go
  through it. Parse failure = failed row with the error, not cached; zero rows = verified-empty,
  not cached; entry_count = parsed rows (chart ranks across segments, version entries for a product
  page, apps, stories, see-all reviews/lockups).
- `ParseChartsPage`: a segment with cards but no Lockup (renamed card kind) or with next-page
  entries but no ids is an error (else ranks 26-200 would be reported as 1-175); a segment with no
  cards and no next page is an explicitly empty chart. Developer page: cards but no Lockup is an
  error. Today page: zero TodayCards plus cards of unknown kinds is an error (UnifiedMessage-only
  stays verified-empty, matching slice 2b's test).
- The cache ignores entries stored by another parser generation (bodies the lenient generation 1
  cached are fetched again).
- Tests: `pages_capture_test.go` (RV-P4 replicas incl. the PageOfKind path, RV-P12 renamed card
  kind, id-less next page, renamed cards with intact next page, explicit empty chart, parsed
  counts 400/25/32/4/10, cache hit keeps FetchedAt, old-generation cache entry refetched).

**3. Robots-disallowed surfaces are opt-in for every request** (`client.go`, `optin.go`)
- `appstore.OptInURL(u)`: on apps.apple.com every path under /api, /v1, /WebObjects, /includes
  and every path whose last segment is `search`. `fetch` refuses them before any request and on
  every redirect hop (`refusing redirect from A to B: <ErrOptInRequired>`, logged as a failed row)
  until `EnableOptInSurfaces`. Search hints (search.itunes.apple.com) stay opt-in by command
  (only `suggest` calls them).
- Tests: `optin_test.go` (URL table, Page/PageOfKind of search and other disallowed paths send 0
  requests, opted-in WebSearch works, Today -> search and product -> /api redirects refused).

**4. Cache vs time series** (`client.go`, `types.go`, `parse_pages.go`, store accessors)
- `MaxCacheTTL` is 15 minutes (was 24 h): Apple's max-age=86400 on Lookup/Search no longer keeps
  a day-old answer; every other surface already sends <= 900 s.
- Observation time on every result: `FetchedAt`/`FromCache` on `LookupResult` (per result, from its
  batch), `LookupResponse` (oldest batch / any cached), `SearchResponse` (+ rows), `ChartFeed`,
  `ReviewsFeed`, `HintsResult`, `ChartsPage`, `TodayPage`, `DeveloperPage`, `SeeAllPage`,
  `SearchPage`, `APIReviewsPage`, `APIApp` (all `json:"-"`, output shapes unchanged). A cached
  result keeps its original FetchedAt.
- Store: `RecordLookupSnapshot` with zero `at` uses `lk.FetchedAt`; `RecordProductPage` uses
  page.FetchedAt, else lk.FetchedAt, and records `lookupFetchedAt` in fields_json.
- Tests: `observation_test.go` (RV-P5 replica with a controllable clock: t0 1000, +10 min cached
  1000 labelled FromCache/t0, +16 min and +23 h fresh 1900), `TestSnapshotWritersUseObservationTime`.

**5. Chart series key includes the Kids age band** (`app_store_store.go`, `extras.go`)
- `chart_snapshots.age_band TEXT NOT NULL DEFAULT ''` (fresh stores in CREATE TABLE; older stores
  via `ensureColumn`), backfill of the interim composite key `36-kids-<band>` (slice 2b) into
  genre 36 + band, index `idx_chart_snapshots_series(storefront, platform, genre_id, age_band,
  chart, captured_at)` replacing `idx_chart_snapshots_key`.
- `ChartKey.AgeBand`, `ChartSnapshot.AgeBand`, `ChartRowRecord.AgeBand`, `ChartQuery.AgeBand`,
  `store.AnyAgeBand`. A key without AgeBand selects the overall chart only; RecordChart's
  same-second replace includes the band (an overall and a Kids write in one second both stay);
  the composite genre key is normalized on write and read, so slice 2b's merged code stores
  canonical rows and its own test still passes.
- Tests: `TestKidsChartIsItsOwnSeries` (RV-S2 replica), `TestChartSnapshotsAgeBandMigration` (old
  schema + old index + composite rows -> column, backfill, new index; idempotent on reopen).

**Also (trivial, same files)**: errcheck/noctx/ST1005 lint findings in slice-1 files fixed
(including the three slice 2b listed); `.printing-press-patches/appstore-client.json` lists the new
test files and its call-site guards match the new migration lines (`for _, m := range
appStoreMigrations {`, `for _, c := range appStoreColumns {`) and the opt-in check.

**Decisions / deviations**
1. RSS envelope: the review asked for feed id/updated/author; the parser requires the feed
   object and needs metadata only to prove emptiness, because slice 2b's synthetic feeds carry
   `updated` only. Cross-feed detection uses the feed id when present and the entry shapes
   otherwise; id checks fire only on positive evidence of another feed.
2. Cache window: the global `MaxCacheTTL` cap (15 min) instead of a Fresh option, so commands
   built on b1786f3 get fresh observations without changes. Contract text said "capped at 24 h".
3. `ChartQuery.AgeBand` "" means overall charts only (not "all"), so readers built before the
   column (rank-history, traction, impact) cannot mix a Kids chart into the genre-36 series.
4. Product-page capture entry_count is its version-history row count; outcome ok even at 0.
5. Redirects into an opt-in surface wrap `ErrOptInRequired` (exit 2), as /api was handled.
6. Not changed (style only): golangci-lint `sqlclosecheck` x15 on the contract's drain-first
   pattern (rows closed explicitly on every path).

**Notes for the merge and slices 2a/2b/3a/3b**
- Pre-existing collision (not slice 1): `type ratingsRow` (2a `ratings.go`) vs `func ratingsRow`
  (3b `s3b_seed_test.go`) stops `go vet`/`go test` of internal/cli once both merge; rename the 3b
  test helper (e.g. `s3bRatingsRow`).
- Stamp every snapshot with the result's observation time: `lk.FetchedAt` (per Lookup result),
  `SearchResponse.FetchedAt`, `ChartFeed.FetchedAt`, `ChartsPage.FetchedAt`, `TodayPage.FetchedAt`,
  `ReviewsFeed.FetchedAt`, `page.FetchedAt`. A result with `FromCache` true is the same
  observation as its first fetch (record it once). Zero means now and is right only for a fresh
  network read. 2a's `s2aIDFetchedAt` and 2b's batch FetchedAt keep working; `c.ChartsPage`/
  `c.TodayPage` now carry FetchedAt/FromCache too (PageOfKind stays valid and is now validated).
- 2b `top --age-band`: prefer `store.ChartSnapshot{GenreID: o.Genre.ID, AgeBand: o.AgeBand.ID}`
  over the composite `36-kids-<id>` key (both store the same canonical rows).
- 3a/3b: `ChartQuery{}` and `ChartKey{}` without AgeBand read overall charts only; pass a band or
  `store.AnyAgeBand` (and read `ChartRowRecord.AgeBand`) to include Kids charts.
- A parse/envelope failure is `*appstore.EnvelopeError` or a parse error, classified exit 5; fan-out
  loops record it as a per-item failure (it is not a throttle).

**Raw acceptance output**
```
$ go build ./... && go vet ./... && go test -count=1 ./...        (main, 06e5ce7)
BUILD_OK / VET_OK
ok  app-store-pp-cli/cmd/app-store-pp-mcp  ok  internal/appstore  ok  internal/cli
ok  internal/client  ok  internal/cliutil  ok  internal/learn (+entities, lookups, patterns)
ok  internal/mcp (+bound, cobratree)  ok  internal/platform  ok  internal/store

$ go test -count=1 -v (new and updated tests)
--- PASS: TestCacheTTL                                   --- PASS: TestParseLookupBodyRequiresEnvelope
--- PASS: TestParseChartFeedRequiresChartEnvelope        --- PASS: TestParseReviewsFeedRequiresReviewsEnvelope
--- PASS: TestParseHintsPlistRequiresHintsArray          --- PASS: TestParseAPIBodiesRequireData
--- PASS: TestEndpointsLogWrongShapedBodiesAsFailed      --- PASS: TestLookupIsReobservedAfterTheCacheWindow
--- PASS: TestResultsCarryObservationTime                --- PASS: TestOptInURL
--- PASS: TestRawPageFetchesOfDisallowedPathsNeedOptIn   --- PASS: TestRedirectToDisallowedSurfaceIsRefused
--- PASS: TestPageParseFailureIsLoggedFailedAndNotCached --- PASS: TestChartSegmentWithUnrecognizedCardsIsAnError
--- PASS: TestExplicitlyEmptyChartIsVerifiedEmptyAndNotCached --- PASS: TestPageCapturesCountParsedRows
--- PASS: TestCacheIgnoresEntriesOfAnotherParserVersion --- PASS: TestTodayAndDeveloperPagesRejectUnknownCards
--- PASS: TestKidsChartIsItsOwnSeries                    --- PASS: TestChartSnapshotsAgeBandMigration
--- PASS: TestSnapshotWritersUseObservationTime          --- PASS: TestClassifyAppStoreErr
--- PASS: TestS1LookupAppRefNeedsTheEnvelopeForNotFound

$ the same checks against b1786f3 (old-API-compatible subset, temp worktree): every one FAILS
--- FAIL: TestOldPageOfSearchURLNeedsOptIn: Page(search URL) without opt-in = <nil>, want ErrOptInRequired
--- FAIL: TestOldLookupReobservedAfter15Minutes: 16 minutes later: requests 1 fromCache true
--- FAIL: TestEndpointsLogWrongShapedBodiesAsFailed: all 12 subtests (Lookup {}: "missing=[324684580]:
    a broken body marked the id missing"; Lookup null / errorMessage / resultCount only, Search {},
    ReviewsRSS chart body / {}, ChartRSS reviews body / null, Hints empty dict, APIReviews errors,
    APIApp {}: "wrong-shaped 200 was accepted")
--- FAIL: TestPageParseFailureIsLoggedFailedAndNotCached: all 4 ("call 1: err = <nil>, want \"no chart
    segments\"" on the PageOfKind path; requests 1 / ok rows on the others)
--- FAIL: TestChartSegmentWithUnrecognizedCardsIsAnError: renamed card kind, id-less next page,
    "top-free with renamed cards and 175 next-page ids: err = <nil>"
--- FAIL: TestRedirectToDisallowedSurfaceIsRefused: "Today -> search redirect: err = unexpected App
    Store page model ... got \"SearchPageIntent\", want a refused redirect" (the hop was followed)
(the age-band store test cannot compile against b1786f3: ChartKey/ChartSnapshot had no band)

$ robustness sweep: every parser x 50 non-empty raw captures from the session (53 files under
  probe/, critic-out/, s1/; 3 are 0-byte)
every capture parses with its own parser (lookup 1/5/176, chart RSS 10/25/100/100, reviews RSS
50/50/50, hints 10, /api reviews 8/10, /api app 1, pages: charts 400 x4, product 25 x8, Today 32,
developer 4, see-all 10, story 1); cross-surface: 0 verified-empty results (the reviewer's harness
counted 82 on the old parsers: Lookup 14, chart RSS 15, reviews RSS 16, /api reviews 19, /api app 18)
VERIFIED-EMPTY RESULTS (0): []

$ trial merge (temp worktree): 06e5ce7 content + slice-2a + slice-2b + slice-3a + slice-3b
merged slice-2a / slice-2b / slice-3a / slice-3b (no conflicts); after renaming 3b's ratingsRow:
BUILD_OK VET_OK; go test ./... all ok; internal/cli -v: 324 PASS, 0 FAIL (incl.
TestTopKidsAgeBandUsesItsOwnSnapshotKey, TestTopEmptyChartIsHonestAndRecordsNothing,
TestTopMacGrossingEmptyFeed, TestFeaturedEmptyTodayPageIsHonest, TestFeaturedListsStoriesAndStoresAll,
TestSuggestNoHintsIsHonest, TestReviewsEmptyFeedForRealAppIsHonest, TestReviewsAPISourceIsOptIn...)

$ go test -tags live -count=1 -run '^TestLiveEmptyEnvelopes$' -v ./internal/appstore/   (2026-10-23T20:23:41Z)
LIVE lookup id=1: results=0 missing=[1] outcome=verified-empty fetchedAt=2026-10-23T20:23:42Z
LIVE reviews id=1: reviews=0 outcome=verified-empty fetchedAt=2026-10-23T20:23:43Z
LIVE hints zqxjvkwpqz: suggestions=[] outcome=verified-empty
LIVE requests sent=3 stats=map[itunes:2 search-hints:1]
LIVE capture: status=200 outcome=verified-empty entries=0 cache-control="max-age=72352" url=https://itunes.apple.com/lookup?country=us&id=1
LIVE capture: status=200 outcome=verified-empty entries=0 cache-control="" url=https://itunes.apple.com/us/rss/customerreviews/page=1/id=1/sortby=mostrecent/json
LIVE capture: status=200 outcome=verified-empty entries=0 cache-control="no-transform, max-age=600" url=https://search.itunes.apple.com/WebObjects/MZSearchHints.woa/wa/hints?clientApplication=Software&term=zqxjvkwpqz
--- PASS: TestLiveEmptyEnvelopes (2.10s)
```
Live budget used by the fixes: 3 of 5 requests (all HTTP 200, sequential, no throttle).

## Merge and completion-gate checks (main session, 2026-10-24 01:35-01:50 PKT)
- Merged slice-2a, slice-2b, slice-3a, slice-3b onto main (06e5ce7 foundation fixes) with --no-ff:
  no conflicts; no slice touched foundation files, README, SKILL, manifest, spec or go.mod.
- Merge fix 93570b2: slice 3b test helper `ratingsRow` renamed `s3bRatingsRow` (clashed with
  slice 2a's `type ratingsRow`). go build, go vet, go test -count=1 ./... green.
- Per-row Cobra resolution (scripts/manifest_cobra_check.py): 41/41 approved command paths
  resolve as leaves (36 absorbed incl. behavior rows, 5 transcendence).
- Deterministic backstop: dogfood --json novel_features_check planned=5 found=5 (PASS).
  Structural verdict WARN: 1 dead flag (--max-age, fix-list C4), 9 unused generated helpers;
  source_client_check 22 files clean; wiring 73/73; examples 10/10.
- Priority 1 Review Gate (privacy, reviews, movers): realistic examples, --dry-run envelopes,
  valid JSON (local commands return [] / {meta} with a stderr hint on an empty store).
- Live acceptance for slice 2b was blocked by a Fastly 429 on apps.apple.com (2 min after
  slice 2a's 13 requests); one paced acceptance pass runs after the integration fixes
  (proofs/2026-10-24-integration-fixlist.md, worktree wt-fix2).

### Post-merge integration fixes — 2026-10-24, build green, branch integration-fix (working/wt-fix2), base 93570b2, head ed322dd

Fix list: proofs/2026-10-24-integration-fixlist.md (sections A, B and C). Live Apple requests: **0** (budget 0);
every check runs through the shared httptest/fake-transport seam over the real-capture fixtures. No new Go
dependency; README.md, SKILL.md and research.json untouched; cli-printing-press dogfood not run. One commit per
item, each with `go build ./...`, `go vet ./...` and `go test -count=1 ./...` green.

**Commits**
```
e7da751 A1  empty Kids age band is an error; CLI resolveAgeBand returns no band for an unset flag
db95a0a C6  one shared transport seam (appStoreClientHook; s2aNewClient/s2bNewClient retired); --home scoped to its invocation
ec5e0c5 C1  chart key genre + age_band; movers/rank-history/breakouts --age-band; traction/impact name Kids series; integration test
029e1b1 C2  idempotent snapshot writers on the observation; FetchedAt everywhere (web search, /api reviews); cached second-run test
1a4376d C3  Lookup keeps completed batches on a later batch failure (Unresolved lists the rest)
4a4a1a2 A2  jitter = reserved send slot interval x (1 + U[0, j]) after the previous send, taken after the limiter wait
8823ecb A3  review upserts from a source without votes keep the stored votes
43c3943 A4  disappearing privacy label / IAP list recorded as an explicit empty change point when the page positively shows none
1bb0767 A5  404 is NotFoundError only for id-addressed resources; elsewhere HTTPError (exit 5)
d55ef4e A6  Lookup normalizes ids (id prefix, whitespace) before de-duplicating
437404e A7  --lang adds l=<lang> to apps.apple.com page URLs (language-specific URL and cache key)
1a79ba4 A2  test tolerance for scheduling noise (spread check unchanged)
8d97304 C5  store.AppIDForBundle + store.AppsWithHistory; bundle ids in local commands; changes covers every app with history
7e7dc81 C4  --max-age staleness hint on stderr in the 11 local history commands
ce6f195 C7  golangci-lint clean on hand-built code (22 errcheck fixed; only the 15 drain-first sqlclosecheck notes remain)
6afd75b B1  FetchMeta CacheControl/MaxAge/Age/DataAsOf; ratings_snapshots.data_as_of; traction/impact order counts by data time
cca0d21 B14 integration test keyword-rank/search-store -> store -> keyword-history/keyword-share
ed322dd     patches manifest: new store tests, the two root.go hooks, validation note
```

**C. Main-session items**
- C1 fixed (ec5e0c5). `top` writes `store.ChartSnapshot{GenreID: 36, AgeBand: <band>}` (never `36-kids-<band>`); meta field
  `snapshotGenreKey` (string) is replaced by `snapshotKey` (the store.ChartKey object). `movers`, `rank-history` and `breakouts`
  take `--age-band` (unset = overall chart, validated like top: genre 36, iphone/ipad, top-free/top-paid); rank-history
  `elsewhere` lists Kids charts with their `ageBand`; traction and impact read every chart (`store.AnyAgeBand`) and name a Kids
  series `iphone/36/top-free/kids-10001` (the first three segments are unchanged). Test
  `TestIntegrationTopToMoversRankHistoryBreakouts`: httptest Apple serving charts_us_iphone_36.html and re-ranked copies, `top`
  in us and gb (overall and `--age-band 6-8`) on two days, then movers (overall: only the rank-150 -> 26 climber, entrant,
  exiter; Kids: only its own 100 -> 30 climber), rank-history (overall vs Kids series of the same app), breakouts (2 markets,
  median gain 124 / Kids 70) and traction (`bestRankChart` = `iphone/36/top-free/kids-10001`). Before the fix the readers had no
  way to select the Kids series (unknown flag) and traction folded nothing but the overall chart.
- C2 fixed (029e1b1). Store writers are idempotent on the observation: app snapshots on (app, storefront, source, captured_at)
  via a new `app_snapshots.source` column (backfilled from fields_json, indexed); ratings on (app, storefront, source,
  captured_at) (`RecordRatingsN` reports writes); featured on (storefront, platform, captured_at) (`RecordFeaturedOnce`);
  keyword ranks on (term, storefront, platform, source, target, captured_at) (`RecordKeywordRankOnce`); privacy/IAP change
  points compare with the list in force at the observation; events keep first_seen MIN / last_seen MAX with fields from the
  newest sighting; reviews widen first/last seen and an older batch never overwrites newer text or moves last_seen back. The CLI
  dedupe helpers (s2aRecord* checks, s2bRatingsSnapshotAt, s2bKeywordRankAt, featuredCaptureExists) are retired. Wall-clock
  stamps removed: `search-store --source web` and `keyword-rank --source web` use SearchPage.FetchedAt/FromCache, `reviews
  --source api` uses APIReviewsPage.FetchedAt/FromCache. Tests: `TestWritersAreIdempotentOnTheObservation`,
  `TestAppSnapshotsSourceColumnMigration`, `TestCachedSecondRunRecordsNoNewSnapshots` (11 live commands with a pinned client
  clock: the cached second run sends 0 requests, adds 0 rows to 10 tables, and every stored time equals the observation time;
  with the old writers it fails on a wall-clock keyword_ranks row).
- C3 fixed (1a4376d). `Lookup` returns the completed batches (Results and Missing for those ids, input order) together with the
  error; `LookupResponse.Unresolved` lists the ids (or bundle ids) never asked. Test
  `TestClientLookupKeepsCompletedBatchesOnFailure` (5xx and 429 on batch 2 of 200 ids; bundle-id variant).
- C4 fixed (7e7dc81). `warnStale` prints `hint: the newest stored <capture> is <age> old (captured <t>), older than --max-age <d>;
  refresh with: <live command>` on stderr in movers, rank-history, keyword-history, watch-listing, review-digest, search,
  traction, impact, changes, keyword-share and breakouts (newest relevant capture per command; `store.NewestAppObservation`
  for app-scoped ones). `--max-age 0` disables it; stdout/JSON is byte-identical. Test
  `TestLocalCommandsHintWhenDataIsOlderThanMaxAge` (all 11 commands: hint at 1h, none at 0 or 200000h, stdout unchanged).
- C5 fixed (8d97304). `store.AppIDForBundle` (case-insensitive, index `idx_app_snapshots_bundle`) and `store.AppsWithHistory`
  (union of app_snapshots, iap_prices, privacy_snapshots, app_events, version_entries). rank-history, watch-listing,
  review-digest, `keyword-history --app` and `search --app` now accept bundle ids (resolved after the store opens; an unknown one
  is an empty result with a note, exit 0); s3b's per-app bundle index is replaced by the accessor; `changes` without `--apps`
  covers every app with stored history. Three slice-3a tests that asserted bundle ids exit 2 were updated (the bundle cases moved
  out of their exit-2 lists). Tests: `TestLocalCommandsAcceptBundleIDs`, `TestAppIDForBundleAndAppsWithHistory`.
- C6 fixed (db95a0a). `appStoreClientHook` in newAppStoreClient is the one seam; test helper `useFakeAppStore` (transport, cache
  dir, cached, clock); the s2a and s2b harnesses use it. The `--home` override is scoped: PersistentPreRunE wraps
  `cliutil.SetHomeOverride` in `scopeHomeOverride`, a Cobra finalizer restores it after a directly executed command, and
  `Execute()` defers `executeHomeScope()` first so the journal/receipt still land in --home before the restore (two-line edit in
  the generated root.go, recorded as call-site guards in .printing-press-patches). Tests: `TestHomeOverrideIsScopedToTheInvocation`
  (fails on the old line: "--home leaked past the command that set it"), `TestNewAppStoreClientUsesTheSharedSeam`.
- C7 fixed (ce6f195). See Lint below.

**A. Foundation review defects**
- A1 fixed (e7da751): `appstore.ResolveAgeBand("")` errors; only explicit all/kids/all kids/0 map to band 0; CLI
  `resolveAgeBand(flag)` returns the zero band for an unset flag; `checkChartAgeBand`, `ageBandLabel`, `KidsAgeBandByID`.
  Tests `TestResolveAgeBand`, `TestResolveAgeBandFlag`, `TestTopEmptyAgeBandIsTheOverallChart` (old code: `--age-band ""` fetched
  `ageBandId=0`).
- A2 fixed (4a4a1a2, test tolerance 1a79ba4): pacing waits for the class limiter, then reserves a send slot at least
  interval x (1 + U[0, Jitter]) after the previous slot of the class (under the client lock, so concurrent callers never share a
  slot). The documented +0-20% deviation now holds. `TestClientJitterSpreadsRequestGaps` (20 sends at 20/s): old pacing spread
  1.9 ms (min 49.0, max 50.9 ms) -> fails; new spread about 10 ms, no gap under the interval (10% scheduling tolerance).
- A3 fixed (8823ecb): only the RSS source carries votes; other sources insert NULL votes and update with COALESCE (stored votes
  kept). `TestReviewVotesSurviveASourceWithoutVotes`.
- A4 fixed (43c3943): parser flags `Privacy.NoneShown` (App Privacy section rendered with no items at all) and
  `ProductPage.IAPNoneShown` (Information rendered, no In-App Purchases annotation, offer hasInAppPurchases false); the store then
  records an explicit empty label / an empty IAP list (one marker row: position 0, NULL name and price; IAPHistory returns it as
  an empty list). Unreadable sections (items of unknown kinds) and pages without the section record nothing. No real
  "No Details Provided" capture exists, so the rule is conservative. Tests `TestProductPageShowsNoIAPOrPrivacy`,
  `TestEmptyPrivacyAndIAPAreChangePointsOnlyWhenShown`, `TestChangesReportsRemovedIAPListAndPrivacyLabel` (changes now reports 10
  IAP and 2 privacy removals; before: none).
- A5 fixed (1bb0767): `notFoundMeansMissing` = apps.apple.com /{cc}/app/..., /{cc}/developer/..., /api/apps/v1/catalog/{cc}/apps/{id};
  every other 404 is `*HTTPError` (exit 5). `TestClient404TypingDependsOnTheSurface` (7 surfaces HTTPError, 5 not found),
  `TestSearch404IsAnAPIFailureNotNotFound` (search-store and keyword-rank exit 5).
- A6 fixed (d55ef4e): ids are trimmed and stripped of id/ID before de-duplication. `TestClientLookupNormalizesIDsBeforeDedupe`.
- A7 fixed (437404e): `withPageLang` adds `l=<lang>` to apps.apple.com page URLs (an explicit l= is kept; /api already set it);
  Accept-Language stays as a hint; --lang help updated. `TestClientLangSelectsThePageLanguage` (old code: a ja page served from
  cache to a client without --lang).

**B. Foundation requests**
- B1 fixed (6afd75b): FetchMeta gains CacheControl, MaxAge, Age and DataAsOf (FetchedAt minus the copy's age: the Age header, or for
  itunes /lookup and /search, whose edge sends no Age and counts max-age down from 86,400 s, 86,400 s minus max-age); the cache
  keeps Age; LookupResult, PageModel and ProductPage carry DataAsOf; `ratings_snapshots.data_as_of` (column + migration) and
  fields_json `lookupDataAsOf` record it; traction and impact order and bucket rating counts by `RatingsSnapshot.ObservedAt()`.
  Tests `TestResultsCarryDataAge` (max-age=39985 -> data 46,415 s older; Age 332 page; cached copies keep both),
  `TestRatingsSnapshotsKeepTheDataTime`, `TestTractionOrdersCountsByDataTimeNotCaptureTime` (a stale copy captured 5 minutes after
  a fresh one: old ordering reports a fake reset and 1,300 new ratings, new ordering 300 and no reset).
- B2 fixed = C2. B4 fixed = C5. B8 fixed = C3. B9, B10 fixed = C6. B13 fixed = C5.
- B3 / B11 already-ok: the errcheck lines in appstore_common.go (:270, :339) and appstore_common_test.go (:167) were fixed in 06e5ce7
  (`_, _ = fmt.Fprint*`, deferred close with ignored error); the baseline lint at 93570b2 reported nothing in those files.
- B5 already-ok: same errcheck lines; `lookupAppRef` and `fetchFailure` are used since the merge (app, developer, events,
  keyword-rank, reviews; compare, iap, ratings, top, reviews); the repo lint reports no unused identifier in hand-built code.
- B6 fixed: the age_band column landed in 06e5ce7; the writer and every reader use it since C1.
- B7 fixed: ChartsPage, TodayPage and SearchPage carry FetchedAt/FromCache since 06e5ce7 (including a cached WebSearch, so the
  opt-in gate needs no bypass); C2 switched the web-search writers to them; B1 adds the data time.
- B12 skipped: this task has no live budget. The slice-2b live checks (top 200/150/pk, reviews twice, keyword-rank, featured,
  suggest, one /api reviews call) still need one sequential live session.
- B14 fixed: `TestIntegrationTopToMoversRankHistoryBreakouts` (chart Platform/GenreID/Chart/AgeBand) and
  `TestIntegrationKeywordRankToKeywordShareAndHistory` (cca0d21: Source itunes, Platform iphone, target in AppID, ranked TopIDs).
- B15 already-ok: process note; dogfood was not run in this task.

**Lint** (golangci-lint 2.13.2, repo .golangci.yml, `--max-issues-per-linter=0 --max-same-issues=0`): 37 issues at 93570b2 (22
errcheck in breakouts, changes, impact, keyword_share, s3b_helpers, traction; 15 sqlclosecheck) -> 15 now, all sqlclosecheck on
the drain-first readers in internal/store/app_store_store.go: Captures, scanAppSnapshots, TrackedApps, VersionEntries,
PrivacyHistory, latestIAPHashTx, IAPHistory, EventsHistory, RatingsSeries, ChartCaptures, ChartSnapshotAt, ChartSeries,
ChartRows, FeaturedSince, KeywordSeries (rows are closed explicitly on every path; new code uses deferred closes). Unused
generated helpers: the repo config reports none (generated files are excluded and `unused` treats generated code as used); a
scratch scan with `generated-is-used: false` lists 92 identifiers, all in generator-owned files: helpers.go 47
(paginatedGetMaxPages, formatCLIParamValue, requiredInputWaived, harnessRefusalResult, writeHarnessRefusal,
endpointPositionalArgs, endpointArgsUsageErr, endpointUsageLine, booleanSpaceFormPositionalErr, booleanNoValueFlags,
hasChangedLocalFlags, responsePathPaginatedClient (+IsDryRun, GetWithHeaders), paginatedGetWithResponsePath,
retainCLIQueryParams, retainExplicitQueryParams, paginatedGet, cloneRawObject, deleteRawPath, nextFullPageOffsetCursor,
nextClientSidePaginationCursor, emitTruncationWarning, emitMissingPaginationSignalWarning, emitPaginatedGetMaxPagesWarning,
emitPaginatedGetRepeatedPageWarning, paginatedItemsEqual, emitMissingPaginationCursorWarning, paginationCursorToken,
resolvePaginatedNextCursor, hasMoreReading, readHasMoreField, hasMoreFieldIsNextLink, rawJSONNull, cursorTokenFromMaybeURL,
reportedCollectionTotal, reportedCollectionTotalAtDepth, nextCursorFromLinks, nextCursorFromPaging, paginationLinkURL,
nextCursorFromTopLevelURL, isFollowableNextURL, cursorFromNextURL, responsePayloadParentAtPath, resolvePlatformWindow,
isDryRunResponseForClient, pattern), data_source.go 17, client/client.go 6, store/store.go 4, config/config.go 4,
html_extract.go 3, doctor.go 2, resource_paths.go 2, platform_client.go 1, root.go 1 (registerClientHook), mcp/tools.go 1,
store/candidates.go 1. The fix list's count of 9 could not be reproduced with either method; none of them is hand-built, and
they stay.

**Decisions / deviations**
1. Kids series naming in traction/impact output: a fourth path segment `kids-<ageBandId>`; top meta `snapshotKey` replaces
   `snapshotGenreKey`.
2. Store schema additions (idempotent migrations): `app_snapshots.source` (+ backfill + `idx_app_snapshots_capture`),
   `ratings_snapshots.data_as_of`, `idx_app_snapshots_bundle`. New accessors: RecordRatingsN, RecordFeaturedOnce,
   RecordKeywordRankOnce, AppIDForBundle, AppsWithHistory, NewestAppObservation; existing signatures unchanged.
3. The empty IAP list is a marker row in iap_prices; RecordIAPs (standalone) still records nothing for an empty list because its
   caller cannot tell an app without IAPs from a page that hid them.
4. The Lookup data-age heuristic (86,400 s minus max-age) applies to itunes /lookup and /search only; RSS and hints count as fresh.
5. --max-age keeps its generated default (30m), so the stderr hint appears for most local runs on day-old captures.
6. root.go (generated) carries two call-site hooks for the --home scope; everything else lives in appstore_home_scope.go.

**Notes for the next phase**
- README.md / SKILL.md (not edited here) do not yet mention `--age-band` on movers, rank-history and breakouts, bundle ids in the
  local commands, the `--max-age` hint, `--lang` via `l=`, or top's `meta.snapshotKey`.
- `go test -count=2 ./...`: internal/cliutil TestKindDirRelativeOverridesWarnAndFallThrough and internal/learn
  TestJournal_UnwritableDirWarnsOnceAndCommandSucceeds fail on the second in-process run (warn-once state). Verified identical at
  93570b2 in a throwaway worktree; generator-owned; `-count=1` is green. Every hand-built package passes with -count=2.
- This log's first line still reads "0 built"; slice 3b built all 5 transcendence rows (merged in 88bfd8e).
- B12: live acceptance still pending (one sequential session).

**Raw output**
```
$ go build ./... && go vet ./... && go test -count=1 ./...        (head ed322dd)
BUILD_OK
VET_OK
ok  app-store-pp-cli/cmd/app-store-pp-mcp  ok  internal/appstore  ok  internal/cli  ok  internal/client
ok  internal/cliutil  ok  internal/learn (+entities, lookups, patterns)  ok  internal/mcp (+bound, cobratree)
ok  internal/platform  ok  internal/store

$ golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 ./...
internal/store/app_store_store.go: 15 x "Close should use defer (sqlclosecheck)" (the readers listed above)
15 issues:
* sqlclosecheck: 15

$ go test -count=1 -v (new tests)
--- PASS: TestClientJitterSpreadsRequestGaps          --- PASS: TestClientLookupKeepsCompletedBatchesOnFailure
--- PASS: TestClient404TypingDependsOnTheSurface      --- PASS: TestClientLookupNormalizesIDsBeforeDedupe
--- PASS: TestClientLangSelectsThePageLanguage        --- PASS: TestResultsCarryDataAge
--- PASS: TestProductPageShowsNoIAPOrPrivacy          --- PASS: TestResolveAgeBand
--- PASS: TestAppIDForBundleAndAppsWithHistory        --- PASS: TestWritersAreIdempotentOnTheObservation
--- PASS: TestAppSnapshotsSourceColumnMigration       --- PASS: TestReviewVotesSurviveASourceWithoutVotes
--- PASS: TestEmptyPrivacyAndIAPAreChangePointsOnlyWhenShown --- PASS: TestRatingsSnapshotsKeepTheDataTime
--- PASS: TestResolveAgeBandFlag                      --- PASS: TestHomeOverrideIsScopedToTheInvocation
--- PASS: TestNewAppStoreClientUsesTheSharedSeam      --- PASS: TestChangesReportsRemovedIAPListAndPrivacyLabel
--- PASS: TestIntegrationTopToMoversRankHistoryBreakouts --- PASS: TestIntegrationKeywordRankToKeywordShareAndHistory
--- PASS: TestLocalCommandsAcceptBundleIDs            --- PASS: TestCachedSecondRunRecordsNoNewSnapshots
--- PASS: TestSearch404IsAnAPIFailureNotNotFound      --- PASS: TestLocalCommandsHintWhenDataIsOlderThanMaxAge
--- PASS: TestStaleAgeFormat                          --- PASS: TestTopKidsAgeBandUsesItsOwnSnapshotKey
--- PASS: TestTopEmptyAgeBandIsTheOverallChart        --- PASS: TestTractionOrdersCountsByDataTimeNotCaptureTime

$ the same checks against the pre-fix code (files swapped back temporarily, then restored)
--- FAIL: TestHomeOverrideIsScopedToTheInvocation: --home leaked past the command that set it
--- FAIL: TestCachedSecondRunRecordsNoNewSnapshots: SELECT DISTINCT captured_at FROM keyword_ranks =
    [2026-10-22T08:30:00Z 2026-10-23T21:04:55Z], want only the observation time 2026-10-22T08:30:00Z
--- FAIL: TestClientJitterSpreadsRequestGaps: gap spread 1.9055ms (min 49.043917ms, max 50.949417ms) is under 10% of the
    50ms interval: the jitter was absorbed and requests keep a fixed rhythm
--- FAIL: TestTractionOrdersCountsByDataTimeNotCaptureTime: a stale CDN copy was read as a rating reset
```
Live budget used by the integration fixes: 0 requests.

## Integration, live acceptance and live fixes (2026-10-24 01:50-03:50 PKT)
- Fix list proofs/2026-10-24-integration-fixlist.md (29 items) in worktree wt-fix2: 24 fixed,
  4 already-ok, 1 deferred (B12 = this live acceptance). Key changes: chart key genre +
  age_band (C1), snapshot writers idempotent on Apple's observation time (C2), Lookup keeps
  completed batches (C3), --max-age staleness hints (C4), bundle ids in local commands (C5),
  shared transport test seam and scoped --home (C6), golangci-lint 37 -> 15 (sqlclosecheck
  only, drain-first by design) (C7), FetchMeta/DataAsOf (B1).
- Live acceptance (one sequential session): 18 requests, 16 x 200, 2 x 429 on apps.apple.com
  page fetches (3rd page in 38 s; a page 76 s after the previous one). iTunes host never
  throttled. Stopped by rule after the second throttle; 16/26 checks passed, none failed on
  correctness.
- Live fixes: governor persisted across invocations (per-class send slots; per-host hold of
  max(Retry-After, 5 min) after 403/429; uncached apps.apple.com pages 1 per 20 s + 0-20%
  jitter) (bc0e62b); fresher product-page version wins over stale Lookup CDN copies
  (abe2b0b); categories no longer use the retrying generated client (4f6d045); help text
  (01aaa46). Live re-run: 9 requests, all 200, no throttle at 20-23 s page spacing; 10/10
  re-run assertions passed. Not re-run (budget): events 850417475, developer 324684583,
  reviews dedupe (all pass offline; developer passed live in slice 2a) -> phase 18.
- Main session: merged integration-fix (df4e5f1); restored manifest row 18's live path:
  categories list/games --live (708c4a8), default offline. Full suite green (14 packages).
- Phase 3 completion gate: 41/41 manifest command paths resolve; dogfood novel_features_check
  5/5. Build log first line updated to 5 built.
