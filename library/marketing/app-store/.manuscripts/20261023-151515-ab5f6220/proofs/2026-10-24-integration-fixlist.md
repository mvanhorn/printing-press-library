# Integration and fix list (post-merge, main at 93570b2)

## A. Review low-severity defects (foundation review, wf_459eadd2-538)
- [helper API footgun] internal/appstore/genres.go:367-372 — ResolveAgeBand("") returns All Kids (ageBandId 0) with no error. A slice that resolves an unset --age-band unconditionally (the slice notes list appstore.ResolveAgeBand without a caveat) would turn every default `top` into the Kids chart. The 200-row acceptance check would not catch that, and the store would record it as the overall chart (see the chart-key defect).
  Fix: Make an empty input an error (or return a zero band with ok=false), and let only an explicit 'kids'/'all kids' map to 0. Add a CLI wrapper resolveAgeBand(flag) that returns "" when the flag is unset.
- [pacing / jitter] internal/appstore/client.go:375-392 (jitter sleep before limiter Wait); cliutil/ratelimit.go:133-150 — The jitter sleep runs before AdaptiveLimiter.Wait, which measures time since the last request. Whenever response time plus jitter is shorter than the interval, the jitter is fully absorbed. The Search API (3.33 s interval, ~1.6 s responses) therefore fires on a fixed rhythm. The contract asks for ±20% jitter; the documented deviation claims +0-20% and 'never a fixed rhythm', but neither holds.
  Fix: Apply the jitter after Wait returns (sleep Wait plus a random extra), or reserve interval × (1 + U[0, j]) in the limiter, and add a test that the gap spread exceeds ~10% of the interval.
- [review upsert merge] internal/store/app_store_store.go:1583-1590 — The UPDATE path overwrites vote_sum and vote_count unconditionally. /api reviews carry no votes (optin.go:155-163), so `reviews --source api` after RSS zeroes the stored helpfulness counts of every overlapping review.
  Fix: Keep existing votes when the incoming source has none (e.g. update votes only when the source is rss, or use COALESCE(NULLIF(?,0), vote_sum)), as is already done for version and author.
- [change history completeness] internal/store/app_store_store.go:632-637, 839-843 (privacy), 951-954 (IAP) — A privacy label or IAP list that goes from populated to empty is never recorded, so PrivacyHistory/IAPHistory keep showing the old list as current and changes/watch-listing miss the removal.
  Fix: Record an explicit empty snapshot when the page positively shows no label or IAP section (e.g. the annotation is absent but the page otherwise parsed), while still skipping when parsing failed.
- [error typing] internal/appstore/client.go:532-535; cli/appstore_common.go:308-310 — Every 404 becomes NotFoundError and exits 3 'not found', including Search API outages. The research error catalogue records a transient /search 404 on 2026-04-16, so keyword-rank or search-store would report 'not found' instead of an API failure.
  Fix: Map 404 to NotFoundError only for id-addressed resources (product, developer, see-all pages); treat 404 on /search, RSS and hints as an HTTPError (exit 5).
- [lookup input normalisation] internal/appstore/endpoints.go:109-117 — dedupe runs before the 'id' prefix is stripped, so 'id324684580' and '324684580' are both sent and both returned.
  Fix: Strip the 'id' prefix and trim before deduplicating.
- [--lang on pages] internal/appstore/client.go:436 (cache key ignores lang), 464-466 (Accept-Language only) — --lang reaches pages only as Accept-Language. Apple's page CDN varies only on accept-encoding, and research shows the page language is chosen with `?l=`, so --lang likely has no effect on product, chart, Today or developer pages. The page cache key also ignores the language, so a page cached in one language can be served for another for 15 min. This follows the contract, so it is a contract gap.
  Fix: Append `l=<lang>` to apps.apple.com page URLs when --lang is set, which also makes the cache key language-specific, and keep Accept-Language as a hint.

## B. Foundation requests raised by the slices
- (from slice with app) Apple's Lookup CDN keeps a separate cached copy per URL, up to 24 hours old. Live example: the single-id Spotify lookup came back with max-age=39985, about 12.9 hours old, showing 9.1.92 and 42,719,156 ratings. A batched lookup 43 seconds later (max-age=86400, fresh) showed 9.1.94 and 42,722,184, and the product page already listed 9.1.94. Please put Age/max-age on FetchMeta so snapshots can record when Apple's data actually dates from. Slice 3b (traction) should not put a jump between two lookup snapshots minutes apart into that window.
- (from slice with app) store.RecordProductPage, RecordRatings and RecordLookupSnapshot are not idempotent on (app, storefront, source, captured_at). A cached response keeps its original fetch time, so recording it again duplicates a snapshot at the same instant. Slice 2a removes these duplicates in the CLI layer (s2aRecord* helpers); making the store idempotent would cover every slice.
- (from slice with app) golangci-lint errcheck flags two slice-1 lines: unchecked fmt.Fprint* at internal/cli/appstore_common.go:270 and :339, and defer st.Close() at internal/cli/appstore_common_test.go:167. Slice 2a did not edit these files.
- (from slice with movers) Add store.AppIDForBundle(ctx, st, bundleID), or a bundle-id filter on store.TrackedApps, so local history commands can accept bundle ids without a live lookup.
- (from slice with movers) Optional: golangci-lint (the repo's .golangci.yml) reports foundation-owned findings in internal/cli/appstore_common.go: errcheck on fmt.Fprintf (line 270) and fmt.Fprintln (line 339), and lookupAppRef/fetchFailure unused until the live slices merge. Slice 3a files are lint-clean.
- (from slice with top) chart_snapshots has no age-band column: add age_band (or a ChartKey field) or ratify slice 2b's '36-kids-<id>' genre key. Slices 3a/3b must not read 36-kids-* as genre 36.
- (from slice with top) ChartsPage, TodayPage and WebSearch should return FetchMeta (FetchedAt, FromCache). For WebSearch, the fetch time of a cached page cannot be learned without bypassing the opt-in gate.
- (from slice with top) appstore.Client.Lookup returns no results when a later batch fails (it returns resp before byID is copied into Results). It should return the completed batches.
- (from slice with top) Add one shared HTTP-transport test seam for newAppStoreClient. Slice 2b added its own var, s2bNewClient.
- (from slice with top) PersistentPreRunE leaves the --home override (cliutil.SetHomeOverride) set after Execute. Tests that pass --home must reset it, or the generated teach tests fail with 'test sandbox escaped'.
- (from slice with top) golangci-lint errcheck findings in slice-1 files: appstore_common.go:270 and :339 (fmt.Fprint*), appstore_common_test.go:167 (defer st.Close()).
- (from slice with top) The live acceptance for all slices should run from one session, sequentially, after the parallel slices finish. The first slice-2b request got a 429 on apps.apple.com 2 minutes after slice 2a's 13 live requests from the same IP.
- (from slice with traction) (Low priority) A store accessor that lists every app id with any stored history (the union of app_snapshots, iap_prices, privacy_snapshots, app_events and version_entries). With it, 'changes' without --apps would also cover apps captured only through commands that do not write a listing snapshot. Today it uses store.TrackedApps.
- (from slice with traction) (Merge-time integration check, no code change) traction, impact and breakouts read chart rows with Platform from parseChartPlatform (iphone|ipad|mac), GenreID as the numeric id string and Chart from appstore.NormalizeChart (top-free|top-paid|top-grossing). keyword-share and impact read keyword_ranks with Source itunes|web and the tracked target in AppID. Slice 2b's top, keyword-rank and search-store writers need to store the same values.
- (from slice with traction) (Process note) Slices should not run 'cli-printing-press dogfood --research-dir'. It rewrites research.json and syncs .printing-press.json, README.md and SKILL.md even without --live.

## C. Main-session items (merge review, 2026-10-24 01:45 PKT)
- C1. `top --age-band` must write the new chart key (genre 36 + `age_band`), not slice 2b's
  `36-kids-<band>` genre string; movers, rank-history, breakouts, traction and impact read
  `age_band` ('' = overall chart). Add an integration test: `top` (httptest seam over the
  real chart fixtures) -> store -> `movers` / `rank-history` / `breakouts` read the rows.
- C2. Every snapshot writer records the observation time (`FetchedAt`), not the wall clock,
  and Record* accessors are idempotent on (app, storefront, source, fetched_at), so a
  cache-served response never creates a duplicate snapshot. Test with a cached second run.
- C3. Lookup partial failure: return the completed batches plus the error (do not drop them).
- C4. `--max-age` (dead flag): local history commands print a stderr staleness hint when the
  newest relevant capture is older than --max-age (0 disables); JSON stays unchanged.
- C5. Bundle ids in local commands (store.AppIDForBundle) and an accessor listing every app
  with any stored history so `changes` without `--apps` covers all tracked apps.
- C6. One shared HTTP transport test seam for newAppStoreClient (retire `s2bNewClient`), and
  reset the `--home` override after Execute so tests cannot leak it.
- C7. golangci-lint (repo `.golangci.yml`) clean on hand-built code: errcheck in
  appstore_common.go and its test; unused helpers removed. The 9 unused generated helpers
  in helpers.go and the 15 sqlclosecheck notes (drain-first by design) may stay; list them.
