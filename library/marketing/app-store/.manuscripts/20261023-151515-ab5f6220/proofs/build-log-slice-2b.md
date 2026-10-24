# Slice 2b build log: charts, search, reviews, suggest, keyword rank, featured

Worktree `working/wt-2b`, branch `slice-2b`, base b1786f3 (slice 1 foundation).
Date 2026-10-24 PKT (2026-10-23 UTC). Build green: `go build ./...`, `go vet ./...`,
`go test -count=1 ./...` all pass; `golangci-lint run ./internal/cli/...` reports no
issues in slice-2b files (3 errcheck findings remain in slice-1 files, see foundation notes).

**Live acceptance status: BLOCKED.** The first live Apple request of this slice
(`top --chart top-free --genre 36 --limit 200`, chart page) was answered HTTP 429
by apps.apple.com (Fastly `x-cache: MISS fsly`, `Retry-After: 13`), at
2026-10-23T19:31:46Z. Per the hard rule the slice stopped all live traffic there.
Live requests sent: 1 of 15 (0 of the 1 owner-approved /api call). The governor
behaved as designed: exit 7, a clear message, no retry, no second request, nothing
recorded except the failed capture-log row. All contract checks were then run
offline against the sanitized real-capture fixtures (raw output below). The
remaining live checks are listed at the end, ready for one sequential session.

Manifest rows covered (absorbed): 4 search-store, 5 top, 6 top-grossing, 7 per
platform, 8 Kids age bands, 9 reviews, 13 suggest, 18 categories (kept the generated
commands), 23 top row Editor's Choice flag, 27 keyword-rank, 33 featured, 34
keyword-rank --serp signals. Transcendence built: 0 (not this slice).

## Files (all slice-2b owned)
- `internal/cli/top.go`, `search_store.go`, `reviews.go`, `suggest.go`,
  `keyword_rank.go`, `featured.go`: the six commands (each self-registers via
  `registerNovelCommand` + `addNovelCommandIfAbsent`; `// pp:data-source live` plus the
  matching annotation; `mcp:read-only: "true"`).
- `internal/cli/s2b_common.go`: client seam `s2bNewClient` (defaults to
  `newAppStoreClient`), `s2bOpenLive` (store first, then client, then
  `attachCaptureLog`), Lookup hydration per 150-id batch with per-batch fetch times,
  cache-aware ratings / keyword-rank recording, `{meta, results}` envelope printer,
  search platform/source validation, title-match helpers.
- Tests: `top_test.go`, `search_store_test.go`, `reviews_test.go`, `suggest_test.go`,
  `keyword_rank_test.go`, `featured_test.go`, `s2b_common_test.go`,
  `s2b_acceptance_test.go` (contract checks, offline), `s2b_testutil_test.go` (fake
  Apple transport serving `internal/appstore/testdata` fixtures; synthetic Lookup and
  review feeds use placeholder text only: "reviewer-N", "Sample title N").
  50 slice tests, all passing.
- No foundation file, README, SKILL, research.json or .printing-press.json touched.

## Commands built
| Command | Flags | Data source | Store writes |
|---|---|---|---|
| `top` | `--chart top-free\|top-paid\|top-grossing` (default top-free), `--genre` id or name (default 36), `--platform iphone\|ipad\|mac`, `--limit` 1-200 (default 100; grossing capped at 100 with a note), `--age-band all\|5-under\|6-8\|9-11` (genre 36, iphone/ipad, free/paid only), `--db` | apps.apple.com chart page via `c.PageOfKind` + `appstore.ParseChartsPage` (1 request), Lookup hydration (1 request per 150 rows); grossing: legacy RSS `ChartRSS` (100 rows) + 1 Lookup | `RecordChart` (every rank: 200 web / 100 rss, RowLimit 200/100, dated at the page's fetch time) + one `lookup` ratings snapshot per hydrated app |
| `search-store <term>` | `--platform iphone\|ipad\|mac`, `--price free\|paid`, `--limit` 1-200 (default 25), `--source itunes\|web` (web opt-in, robots-disallowed), `--db` | iTunes Search API (1 request; `--price` scans Apple's first 200); web: `c.EnableOptInSurfaces()` + `c.WebSearch` + Lookup | `RecordKeywordRank` with no target (app_id ""), full ranked id list |
| `reviews <app>` | `--countries` (default --country), `--sort recent\|helpful\|both` (default both; api defaults to recent), `--pages` 1-10 (default 1), `--limit` (0 = all), `--source rss\|api` (api opt-in, robots-disallowed, adds developerResponse), `--db` | customer-reviews RSS (50/page); api: `c.EnableOptInSurfaces()` + `c.APIReviews` (20/page) | `UpsertReviews` per page (Sort = feed sort, Source rss/api, SeenAt = fetch time) + capture log |
| `suggest <term>` | `--limit` (0 = all), `--db` | search hints (`c.Hints`, X-Apple-Store-Front header); help states the surface is robots-flagged and the command itself is the explicit opt-in | capture log only |
| `keyword-rank <term>` | `--app` (id, id123, URL or bundle id; optional), `--source itunes\|web` (web opt-in), `--platform`, `--serp` 1-200 (default 10), `--db` | Search API, one 200-result request (web: search page + 1 Lookup of the top N) | `RecordKeywordRank` (rank, result count, full ranked id list), skipped when the identical capture (same second) is already stored |
| `featured` | `--platform iphone\|ipad`, `--limit` (0 = all; every story is stored), `--db` | Today page via `c.PageOfKind(..., "today_Intent")` + `appstore.ParseTodayPage` | `RecordFeatured` (all stories), once per observation |
| `categories list` / `categories games` | generated, unchanged | generated client | none |

Output shapes: `top`, `search-store`, `reviews` and `featured` print `{meta, results}`;
`results` rows are exactly the contract rows (`top`: rank, appId, title, subtitle,
developerName, genre, price, averageRating, ratingCount, editorsChoice; `reviews`:
reviewId, storefront, rating, version, title, body, author, reviewedAt, voteSum,
voteCount, developerResponse; `featured`: position, storyId, title, heading, apps[{appId,
title}]; `search-store`: rank, appId, title, developerName, genre, price, averageRating,
ratingCount, bundleId, source). `meta` carries storefront, surface, capture time,
fromCache, counts, `notes` and `fetch_failures`. `suggest` prints `{term, storefront,
suggestions, note?}`; `keyword-rank` prints `{term, storefront, platform, source, app|null,
rank|null, resultCount, scanned, top[], signals{resultCount, topN, topRatingCounts,
titleMatches, titleMatchShare}, capturedAt, fromCache, recorded, notes}`.

## Decisions
1. **Observation time, not run time.** Chart and Today pages are fetched with
   `c.PageOfKind` so the snapshot is dated at `PageModel.FetchedAt`; a page served from
   the 15-minute cache is re-recorded at its original second (RecordChart replaces
   same-second rows, so it is idempotent) and never counted as a new capture. Ratings
   snapshots use each Lookup batch's fetch time and are skipped when the store already
   has that app at that second; keyword ranks and featured stories check for an
   identical same-second capture before writing. Reruns inside a cache window
   therefore never inflate history (tested for top, keyword-rank, featured, reviews).
2. **Snapshots are always full depth.** `top --limit 10` still stores all 200 (or 100)
   ranks from the page/feed, titled where the page or Lookup gave a title, so every
   web capture is comparable (RowLimit 200) regardless of --limit.
3. **Absent beats zero.** Fields a source does not carry are omitted, not zero:
   subtitle/editorsChoice exist only for the 25 chart lockups, ratings only for apps the
   Lookup returned (`meta.lookupMissing` lists the rest), votes only for RSS reviews,
   developer responses only for /api reviews. `keyword-rank` rank is JSON `null` when
   the target is not in the scanned results.
4. **Honest empties.** Empty chart segment, empty RSS feed (Mac grossing), no search
   results, price filter with no match, empty review feeds, no hints and an empty Today
   page all return `[]` (never null) plus a `verified empty` note and record nothing.
5. **Reviews fan-out.** Pages stop at the first short page (fewer than 50 RSS / 20 api
   reviews) or a verified-empty page; a throttle stops everything (exit 7); other page
   errors become `fetch_failures` plus the stderr warning, and a storefront with a
   failed page is never reported as verified empty; if every page failed the command
   exits 5. Duplicates across sorts are merged by (storefront, reviewId); `both` sorts
   rows newest first, a single sort keeps feed order.
6. **Lookups only when needed.** `reviews` with a numeric id skips the Lookup; only
   when every feed is empty does it look the app up, to tell "no such app" (exit 3) from
   "no written reviews". Bundle ids resolve through Lookup first. `keyword-rank` matches
   a bundle-id target against Search results without a Lookup.
7. **Agent output.** The envelope meta uses `surface` (web, rss, itunes, api; the same
   values as the stored `source` columns) because a meta `source` that is not
   live/local/computed overwrites the `--agent` provenance `meta.source: live`. The
   envelope printer keeps every row key under `--compact/--agent`, so sparse fields
   (editorsChoice, developerResponse) and review `body` survive (both tested).
8. **Kids charts** are stored under genre key `36-kids-<ageBandId>` (e.g.
   `36-kids-10001`) so they never mix with the genre-36 overall series.
9. **Opt-in surfaces.** `c.EnableOptInSurfaces()` is called only on `--source web`
   (search-store, keyword-rank) and `--source api` (reviews); tests assert the default
   paths never touch apps.apple.com search or /api, and that `reviews --source api
   --pages 1` sends exactly one /api request.
10. **Dogfood caps** via `capForDogfood` / `countriesFlag`: top limit <= 50, review pages
    <= 1 per sort, countries <= 2; search, suggest, keyword-rank and featured are one
    request each.
11. Test isolation: `--home` sets a process-wide override in `PersistentPreRunE`; the
    slice harness resets it in `t.Cleanup` (without that, generated teach tests failed
    with "test sandbox escaped").

## Deviations from the contract
1. `top`, `search-store`, `reviews`, `featured` emit `{meta, results}` envelopes rather
   than bare arrays (contract rows unchanged inside `results`); `suggest` adds
   `storefront` and `note` to the google-play `{term, suggestions}` shape; `keyword-rank`
   adds platform, scanned, capturedAt, fromCache, recorded, notes and signals.topN /
   titleMatches.
2. Charts and Today use `c.PageOfKind` + exported parsers instead of `c.ChartsPage` /
   `c.TodayPage` (those return no fetch time or cache flag).
3. Lookup hydration is batched in the slice (one `c.Lookup` per 150 ids) so a failed or
   throttled second batch keeps the first batch's results (`c.Lookup` returns no
   results when a later batch fails).
4. Grossing rows are hydrated with one Lookup (exact ratings and ratings snapshots), not
   RSS-only.
5. `reviews --source api` defaults to `--sort recent` (one /api request per page) while
   rss defaults to both; numeric ids skip the validation Lookup unless all feeds are empty.
6. `keyword-rank --app` is optional (no target = SERP capture with rank and app null);
   `top_ids` stores the full ranked list (up to 200), not only the --serp N.
7. Kids charts use the snapshot genre key `36-kids-<id>` (schema has no age-band column).
8. `featured --platform` accepts iphone and ipad only (the Today tab).
9. Every command has a command-local `--db`; envelope meta field is `surface`, not `source`.

## Foundation notes / requests (not edited; slice 1 owns these files)
1. `chart_snapshots` has no age-band column: add `age_band` (or a ChartKey field) or
   ratify the `36-kids-<id>` genre key; slices 3a/3b must not treat `36-kids-*` as 36.
2. `ChartsPage`, `TodayPage` and `WebSearch` should return `FetchMeta` (FetchedAt,
   FromCache). For WebSearch there is no way to learn the fetch time of a cached page
   without bypassing the opt-in gate, so cached web captures are not recorded again.
3. `Lookup` drops all results when a later batch fails (`return resp, err` before
   `byID` is copied into `resp.Results`); return the completed batches.
4. Add one shared HTTP-transport test seam for `newAppStoreClient` so slices do not each
   add their own (slice 2b uses `s2bNewClient`).
5. `PersistentPreRunE` leaves the `--home` override set after `Execute`; tests that pass
   `--home` must reset it (`cliutil.SetHomeOverride("")`).
6. golangci-lint errcheck in slice-1 files: `appstore_common.go:270` and `:339`
   (fmt.Fprint*), `appstore_common_test.go:167` (`defer st.Close()`).
7. Envelope commands should avoid a meta key named `source` (it clobbers the `--agent`
   provenance `meta.source`).
8. The 429 came 2 minutes after slice 2a's 13 live requests (4 to apps.apple.com,
   19:28:54Z-19:29:38Z) from the same IP. Run the remaining live acceptance from one
   session, sequentially, after the parallel slices finish.

## Live acceptance (raw)
```
$ app-store-pp-cli suggest "" --json --home .acceptance-home
rc=2, stdout 0 bytes
stderr: (usage) ... Error: a search term is required, e.g. suggest insta
network requests: 0

$ app-store-pp-cli top --chart top-free --genre 36 --limit 200 --json --home .acceptance-home
rc=7, stdout 0 bytes
stderr: Error: rate limited by Apple: HTTP 429 from Apple; the governor stopped this run (no retry) (url: https://apps.apple.com/us/iphone/charts/36?chart=top-free); retry after 13s
hint: wait a few minutes, then retry with fewer storefronts, pages or apps

captures table (sqlite3 .acceptance-home/data/data.db):
id|captured_at|url|host|status|outcome|entry_count|cache_control|x_cache|error
1|2026-10-23T19:31:46Z|https://apps.apple.com/us/iphone/charts/36?chart=top-free|apps.apple.com|429|failed|0|no-store, private, max-age=0|MISS fsly|rate limited: HTTP 429 for https://apps.apple.com/us/iphone/charts/36?chart=top-free; retry after 13s: HTTP 429 from Apple; the governor stopped this run (no retry)
chart_snapshots rows: 0
```
Stopped here (hard rule: stop at the first 403/429). Not run, in this order, 15
requests total including the one /api call (the second reviews run is served from the
15-minute cache, 0 requests; a numeric-id reviews run needs no Lookup when reviews
exist). This slice has 14 of its 15 left, so the rerun needs a fresh budget:
1. `top --chart top-free --genre 36 --limit 200 --json` (3: page + 150 + 50 Lookup)
2. `top --chart top-grossing --limit 150 --json` (2: RSS + Lookup)
3. `top --country pk --genre 6014 --chart top-paid --json` (2) - open question: rows use
   the Lookup `formattedPrice`; the chart page shows `Rs 99.00`; confirm Lookup's PK
   format also contains "Rs".
4. `reviews 6448311069 --countries us --sort both --pages 2 --json` (4) then the same
   command again (0) - expect `meta.stored.inserted: 0`.
5. `keyword-rank spotify --app 324684580 --json` (1)
6. `featured --json` (1)
7. `suggest insta --json` (1)
8. `reviews 6448311069 --source api --pages 1 --json` (1, the owner-approved /api call)

## Offline acceptance (raw, `go test -count=1 -v -run TestS2bContractAcceptance ./internal/cli/`)
Real-capture fixtures (2026-10-23, trimmed, synthetic review text) through the real
client, parsers, store and output pipeline; Lookup bodies synthetic where no fixture
covers the ids.
```
=== RUN   TestS2bContractAcceptance/top_free_genre_36_limit_200
    s2b_acceptance_test.go:38: rows=200 ranks_1_to_200=true nonzero_ratingCount=200 chart_page_requests=1 lookup_requests=2 snapshotRows=200 ratingsStored=200
=== RUN   TestS2bContractAcceptance/top_grossing_limit_150
    s2b_acceptance_test.go:52: rows=100 surface=rss notes=["top-grossing comes from Apple's legacy iTunes RSS feed, which returns at most 100 rows" "--limit 150 was capped at 100: Apple's grossing feed has no deeper ranks"]
=== RUN   TestS2bContractAcceptance/top_pk_6014_top-paid_prices_contain_Rs
    s2b_acceptance_test.go:82: rows=100 with_Rs=25 (the 25 chart-page lockups; the fake Lookup returned no apps, so ranks 26-100 carry no price) first_prices=["Rs 99.00" "Rs 1,300.00" "Rs 100.00"]
=== RUN   TestS2bContractAcceptance/reviews_both_sorts_pages_2_and_a_second_run
    s2b_acceptance_test.go:113: run1 unique=150 fetched=200 bad_ratings=0 stored={Inserted:150 Updated:50 Unchanged:0} | run2 stored={Inserted:0 Updated:0 Unchanged:200} | reviews table=150
=== RUN   TestS2bContractAcceptance/suggest_insta
    s2b_acceptance_test.go:126: suggestions=["instagram" "instacart" "instacart shopper" "instawork" "insta360" "instagram unfollowers" "instant cash advance" "instants" "instacart driver app" "instants, an instagram app"]
=== RUN   TestS2bContractAcceptance/suggest_empty_term_exits_2
    s2b_acceptance_test.go:139: exit=2 err=a search term is required, e.g. suggest insta requests=0
=== RUN   TestS2bContractAcceptance/keyword-rank_spotify_app_324684580
    s2b_acceptance_test.go:162: rank=1 app={AppID:324684580 BundleID:com.spotify.client Title:Spotify: Music and Podcasts} resultCount=5 topRatingCounts=[42719156 49546355 1515966 29628149 47347] titleMatchShare=0.400 recorded=true
=== RUN   TestS2bContractAcceptance/featured_stories
    s2b_acceptance_test.go:181: stories=32 distinctApps=70 first=["Play Roblox’s Biggest Events" "5 Quick Tips for Clash of Clans" "Essential iPhone Games"] stored_rows=74
=== RUN   TestS2bContractAcceptance/reviews_source_api_pages_1_is_one_opt-in_request
    s2b_acceptance_test.go:206: api_requests=1 urls=[https://apps.apple.com/api/apps/v1/catalog/us/apps/6448311069/reviews?l=en-US&limit=20&offset=0&platform=web&sort=recent] rows=10 with_developer_response=1
--- PASS: TestS2bContractAcceptance (0.11s)
```
Behavioural, absence and negative coverage beyond the contract (all PASS): rerun from
cache stores no duplicate chart/ratings/keyword/featured rows; `--limit` never shrinks a
snapshot; empty chart segment, empty Mac grossing feed, empty search, price filter with
no match, empty review feeds, no hints and an empty Today page all print `[]` plus a
verified-empty note and record nothing; Lookup-missing apps keep no invented fields;
Kids charts land under `36-kids-10001`, not `36`; mismatching search term returns
nothing; missing keyword target gives `"rank": null`; unknown app (`reviews 1`) exits
3; a Today URL serving a chart model exits 5; throttles exit 7 on every command with no
further requests and nothing recorded; partial review failures warn
`warning: 1 of 2 fetches failed` and fill `fetch_failures`; bad flags exit 2 with zero
requests (11 top cases, 6 search-store, 9 reviews, 3 suggest, 6 keyword-rank, 4
featured); `--agent` keeps review bodies and editorsChoice; `--help` resolves
`app-store-pp-cli <leaf> [flags]` for all six commands and both categories commands.
