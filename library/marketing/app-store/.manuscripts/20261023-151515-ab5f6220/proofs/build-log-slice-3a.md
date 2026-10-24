# Slice 3a build log: local history (absorbed)

Run 20261023-151515-ab5f6220 · 2026-10-24 · worktree `working/wt-3a` · branch `slice-3a` ·
commit `bc4d1cb` (on foundation `b1786f3`). Build green: `go build ./...`, `go vet ./...`,
`go test -count=1 ./...` all pass. Live Apple requests: **0** (budget 0). Lock heartbeat
`build-p1-progress` updated after commands 3 and 6.

## Manifest rows completed (absorbed; transcendence count unchanged at 0/5)
| Row | Feature | Command |
|---|---|---|
| 25 | Chart movers between snapshots | `movers` |
| 26 | Rank history for one app | `rank-history <app>` |
| 28 | Keyword rank history | `keyword-history <term>` |
| 35 | SERP snapshot history | `keyword-history <term> --serp N` |
| 29 | Listing change detection | `watch-listing <app>` |
| 30 | Review analytics digest | `review-digest <app>` |
| 32 | Offline full-text search | `search <query>` |

## Files (all slice-3a owned)
- `internal/cli/movers.go`, `rank_history.go`, `keyword_history.go`, `watch_listing.go`,
  `review_digest.go`, `search.go` (one command each, self-registered through
  `registerNovelCommand` + `addNovelCommandIfAbsent`; `// pp:data-source local` directive).
- `internal/cli/s3a_helpers.go`: chart-key resolution (`--chart/--genre/--platform/--source`
  + root `--country`), local app-ref parsing, `--countries`/`--since` parsing, missing-mirror
  guard, list/object emitters (machine JSON via `printJSONFiltered`, human output buffered and
  written once), table renderer, dotted version compare, complaint-term tokenizer + stopwords.
- Tests: `movers_test.go`, `rank_history_test.go`, `keyword_history_test.go`,
  `watch_listing_test.go`, `review_digest_test.go`, `search_test.go`,
  `s3a_contract_test.go` (empty-store contract, output modes, `--agent` compaction, annotations),
  `s3a_fixtures_test.go` (in-process runner, synthetic seed through the store accessors, and
  `TestS3ASeedAcceptanceStore`, skipped unless `S3A_SEED_DB` is set).
- No foundation file edited; no new dependency; README/SKILL/research.json/.printing-press.json untouched.

## Commands built
Every command: help-only branch, `dryRunOK` -> `writeDryRun`, usage errors exit 2, `boundCtx`,
missing-mirror guard before opening the store (a read never creates the store file),
`--db` (command-local store path), annotations `mcp:read-only=true`, `pp:data-source=local`,
`strings.Trim` examples, `pp:happy-args` on every positional command.

| Command | Flags | Data source | Output |
|---|---|---|---|
| `movers` | `--chart` (top-free) `--genre` (36) `--platform` (iphone) `--source` web\|rss `--limit` (per movement kind, 0 = all) `--min-change` (1) `--db`; storefront = `--country` | `store.LatestTwoCharts` (+ `ChartCaptures` for the not-comparable note) | bare array `[{movement climbed\|dropped\|entered\|exited, appId, title, developerName, rank, previousRank, delta, capturedAt, previousCapturedAt}]`; `delta = previousRank - rank`; comparison summary on stderr; `[]` + stderr note when < 2 comparable captures or no movement |
| `rank-history <app>` | `--chart` `--genre` `--platform` `--source` `--since` (7d/4w/date) `--limit` (newest N points) `--db`; storefront = URL storefront or `--country` | `ChartCaptures` + `ChartRows` (app filter) | `{appId, title, storefront, platform, genreId, genre, chart, captures, charted, firstSeen, peak, lastSeen, latest, series[{capturedAt, rank\|null, inChart, source, rowLimit}], elsewhere[], note}`; never-charted app -> `series: []` + note + `elsewhere` (other stored charts holding the app) |
| `keyword-history <term>` | `--app` `--serp N` (0-200) `--platform` `--source` itunes\|web `--since` `--limit` `--db`; storefront = `--country` | `KeywordSeries` (+ `TrackedApps` titles for `--serp`) | `{term, storefront, appId, captures, foundIn, bestRank, latestRank, change, series[{capturedAt, platform, source, appId, rank\|null, found, depth, resultCount, top[]}], note}`; with `--app` the rank is the stored rank for targeted captures, else the position in the stored result list; never found -> `series: []` + note; mixed sources/platforms -> note |
| `watch-listing <app>` | `--source` page\|lookup `--db`; storefront = URL storefront or `--country` | `AppSnapshots` (+ `EventsHistory`, `TrackedApps`) | `{appId, title, storefront, source, snapshots, previous, latest, changed, changes[{field, from, to, detail}], note}`; fields: title, subtitle, version (+releasedAt/notes), releaseNotes, price, editorsChoice, hasInAppPurchases, inAppPurchases (added/removed/repriced), privacy, screenshots, events (added/removed with titles), description, sellerName, primaryGenre, contentRating, minimumOsVersion; equal content hash -> `changes: []` + note |
| `review-digest <app>` | `--since-version` (that version or newer) `--countries` (default URL storefront or `--country`; `all`) `--terms` (20) `--db` | `store.Reviews` | `{appId, storefronts, total, averageRating\|null, stars{"1".."5"}, byVersion[{version, reviews, averageRating, stars}], byStorefront[], lowStarReviews, complaintTerms[{term, reviews, share}], oldestReviewAt, newestReviewAt, excludedUnknownVersion, note}` |
| `search <query>` | `--type` all\|apps\|reviews `--app` `--storefront` `--limit` (per type, 20) `--db` | `store.SearchApps` + `store.SearchReviews` (per table, never `db.Search()`) | bare array of hits `{type app\|review, appId, storefront, title, subtitle, version, rating, averageRating, ratingCount, reviewId, snippet, body, reviewedAt, capturedAt}`; apps first, then reviews by relevance; no match -> `[]` + stderr note |

## Decisions
- **Output shapes.** List commands (`movers`, `search`) print a bare JSON array so an empty
  result is exactly `[]`, with the honest note on stderr (human mode prints it on stdout).
  The four object commands carry a `note` field. Empty lists inside objects are `[]`, absent
  summary values are `null`, never zero placeholders.
- **No fabricated rows.** rank-history and keyword-history return `series: []` when the app
  never appears (counts still say how many captures were checked); once it appears, every
  capture is a point and absences are `rank: null`. movers omits unchanged apps.
- **Comparability.** movers relies on `LatestTwoCharts` (same source + row limit) and says
  when other captures exist but are not comparable; watch-listing compares snapshots of the
  same source, preferring the full `page+lookup` pair over later lookup-only snapshots (from
  `prices`/`compare`), with `--source` to pin and a note on fallback.
- **Equal hashes = nothing.** watch-listing short-circuits on equal `content_hash`, so rating
  drift (not hashed) can never appear as a listing change; every hashed field is diffed so a
  changed hash always explains itself.
- **Local app refs.** Numeric id, `id123` and store URLs (the URL storefront wins unless
  `--country` is set). Bundle ids are a usage error with a hint, because local history is
  keyed by numeric id and there is no stored bundle-id accessor (see requests).
- **review-digest.** Complaint terms come only from 1-2 star titles+bodies, lower-cased words
  of 3+ letters, counted once per review, English stopwords plus app/apps/game/games/please,
  need 2 mentioning reviews. `--since-version` uses numeric dotted comparison (9.1.10 > 9.1.9);
  reviews without a version are excluded and counted. No LLM.
- **search.** `<query>` placeholder so dogfood treats it as a search (no-match exits 0).
  Review hits carry `snippet` (survives `--agent`/`--compact`) and the full `body`.
- **keyword-history** carries `pp:no-error-path-probe` (any term is a valid query; a never
  captured term is an empty history). The `<app>` commands need no opt-out: an invalid ref
  such as `__printing_press_invalid__` exits 2.
- **Lint.** Slice files are clean under the repo's `.golangci.yml` (golangci-lint v2, errcheck
  included): human output is rendered into a `bytes.Buffer` and written once with the error
  returned; store closes are explicit.

## Deviations from the contract
1. `search` app-listing matching is a case-insensitive substring match over each app's latest
   snapshot title/subtitle via the foundation's `store.SearchApps` (the schema has no
   app-snapshot FTS table); reviews use `reviews_fts` (FTS5 trigram) as specified.
2. `search --limit` caps each type (apps, reviews) rather than the combined list, so listings
   cannot crowd out reviews.
3. `movers --limit` caps each movement kind (climbed/dropped/entered/exited); counts in the
   stderr summary are taken before the cap.
4. `review-digest --since-version` means "this version or newer" (numeric compare), not an
   exact match as in the google-play sibling, to match the flag name.
5. No `--age-band` on movers/rank-history: slice 2b's storage key for Kids charts is not known
   to this slice (see merge notes).

## Foundation requests / merge notes
- Request: `store.AppIDForBundle(ctx, st, bundleID)` (or a bundle filter on `TrackedApps`) so
  local history commands can accept bundle ids without a live lookup.
- Merge note: if slice 2b stores Kids age-band charts under a distinct genre/chart key, add
  `--age-band` to `movers` and `rank-history` with the same encoding.
- Merge note: slice 3a registers `search`; slice 2b must not alias `search-store` as `search`.
- Merge note: golangci-lint still reports foundation-owned findings in
  `internal/cli/appstore_common.go` (errcheck on two `fmt.Fprint*`; `lookupAppRef` and
  `fetchFailure` unused until the live slices merge).

## Raw acceptance output
Acceptance used the built binary (`go build -o ./app-store-pp-cli ./cmd/app-store-pp-cli`)
with `--home <wt>/.acceptance-home` (store `<wt>/.acceptance-home/data/data.db`), seeded by
`S3A_SEED_DB=<path> go test -run TestS3ASeedAcceptanceStore ./internal/cli/` through the store
accessors. Phase A = no store file, B = empty tables, C = synthetic seed; several checks
cross-verify the command output against independent `sqlite3` queries on the same file.
Script: `scratchpad/scripts/s3a_acceptance.sh`. Result: **71 passed, 0 failed**.

### go test (slice tests, -v summary)
```text
--- PASS: TestKeywordHistoryHelpWires (0.00s)
--- PASS: TestKeywordHistoryAppSeries (0.01s)
--- PASS: TestKeywordHistorySerpReplay (0.01s)
--- PASS: TestKeywordHistoryNotFoundIsEmpty (0.01s)
--- PASS: TestKeywordHistoryMixedSourcesAndFilters (0.01s)
--- PASS: TestKeywordHistoryEmptyAndErrors (0.01s)
--- PASS: TestMoversHelpWires (0.00s)
--- PASS: TestMoversTwoSnapshotsEnteredExited (0.01s)
--- PASS: TestMoversOneSnapshotReturnsEmpty (0.01s)
--- PASS: TestMoversEmptyStoreAndMissingMirror (0.00s)
--- PASS: TestMoversIgnoresIncomparableCaptures (0.01s)
--- PASS: TestMoversMinChangeLimitAndSelect (0.01s)
--- PASS: TestMoversUsageErrors (0.00s)
--- PASS: TestComputeMoversPure (0.00s)
--- PASS: TestRankHistoryHelpWires (0.00s)
--- PASS: TestRankHistorySeriesWithGap (0.01s)
--- PASS: TestRankHistoryDroppedOutNote (0.01s)
--- PASS: TestRankHistoryNeverChartedIsEmptyWithElsewhere (0.01s)
--- PASS: TestRankHistorySinceWindow (0.01s)
--- PASS: TestRankHistoryEmptyAndErrors (0.00s)
--- PASS: TestReviewDigestHelpWires (0.00s)
--- PASS: TestReviewDigestCountsMatchInsertedRows (0.01s)
--- PASS: TestReviewDigestAllStorefrontsAndSinceVersion (0.01s)
--- PASS: TestReviewDigestEmptyAndErrors (0.01s)
--- PASS: TestDigestTermsAndVersions (0.00s)
--- PASS: TestS3AEmptyStoreContract (0.01s)
--- PASS: TestS3AOutputModes (0.05s)
--- PASS: TestS3AAgentCompactKeepsPayload (0.01s)
--- PASS: TestS3ACommandsAnnotated (0.00s)
--- SKIP: TestS3ASeedAcceptanceStore (0.00s)
--- PASS: TestS3ASeedIsDeterministic (0.01s)
--- PASS: TestSearchHelpWires (0.00s)
--- PASS: TestSearchReturnsOnlyMatchingRows (0.01s)
--- PASS: TestSearchFilters (0.01s)
--- PASS: TestSearchNoMatchAndEmpty (0.01s)
--- PASS: TestSearchSnippet (0.00s)
--- PASS: TestWatchListingHelpWires (0.00s)
--- PASS: TestWatchListingVersionChange (0.01s)
--- PASS: TestWatchListingEqualHashesReportNothing (0.01s)
--- PASS: TestWatchListingSingleSnapshotAndOtherStorefront (0.01s)
--- PASS: TestWatchListingPrefersFullSnapshots (0.01s)
--- PASS: TestWatchListingEmptyAndErrors (0.00s)
PASS
ok  	app-store-pp-cli/internal/cli	0.646s
```

### Gates (go build / go vet / go test ./...)
```text
go build ./...: ok
go vet ./...: ok
?   	app-store-pp-cli/cmd/app-store-pp-cli	[no test files]
ok  	app-store-pp-cli/cmd/app-store-pp-mcp	1.018s
ok  	app-store-pp-cli/internal/appstore	0.557s
?   	app-store-pp-cli/internal/cache	[no test files]
ok  	app-store-pp-cli/internal/cli	2.139s
?   	app-store-pp-cli/internal/cli/playbooks	[no test files]
ok  	app-store-pp-cli/internal/client	1.314s
ok  	app-store-pp-cli/internal/cliutil	12.207s
?   	app-store-pp-cli/internal/cliutil/testenv	[no test files]
?   	app-store-pp-cli/internal/config	[no test files]
ok  	app-store-pp-cli/internal/learn	1.030s
ok  	app-store-pp-cli/internal/learn/entities	0.211s
ok  	app-store-pp-cli/internal/learn/lookups	0.560s
ok  	app-store-pp-cli/internal/learn/patterns	0.289s
ok  	app-store-pp-cli/internal/mcp	0.455s
ok  	app-store-pp-cli/internal/mcp/bound	0.278s
ok  	app-store-pp-cli/internal/mcp/cobratree	9.693s
ok  	app-store-pp-cli/internal/platform	0.357s
ok  	app-store-pp-cli/internal/store	1.178s
?   	app-store-pp-cli/internal/types	[no test files]
```

### Acceptance transcript (binary, isolated --home)
```text
=============== PHASE A: no store file at <wt>/.acceptance-home/data/data.db

### A1 movers, no store
$ app-store-pp-cli movers --chart top-free --genre 36 --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli top --chart top-free --genre 36 --platform iphone --country us   (twice, at different times)
note: no local chart snapshots yet

ASSERT PASS: A1 exit 0 (exit 0)
ASSERT PASS: A1 stdout is []
ASSERT PASS: A1 stderr names the missing mirror

### A2 rank-history, no store
$ app-store-pp-cli rank-history 324684580 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "324684580",
  "storefront": "us",
  "platform": "iphone",
  "genreId": "36",
  "genre": "All Apps",
  "chart": "top-free",
  "captures": 0,
  "charted": 0,
  "firstSeen": null,
  "peak": null,
  "lastSeen": null,
  "latest": null,
  "series": [],
  "note": "no local chart snapshots yet; run: app-store-pp-cli top --chart top-free --genre 36 --platform iphone --country us   (repeatedly over time)"
}

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli top --chart top-free --genre 36 --platform iphone --country us   (repeatedly over time)

ASSERT PASS: A2 exit 0 (exit 0)
ASSERT PASS: A2 empty object with note

### A3 keyword-history, no store
$ app-store-pp-cli keyword-history spotify --app 324684580 --json --home .acceptance-home
exit=0
--- stdout
{
  "term": "spotify",
  "storefront": "us",
  "appId": "324684580",
  "captures": 0,
  "foundIn": 0,
  "bestRank": null,
  "latestRank": null,
  "change": null,
  "series": [],
  "note": "no local keyword captures yet; run: app-store-pp-cli keyword-rank \"spotify\" --country us --app 324684580   (repeatedly over time)"
}

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli keyword-rank "spotify" --country us --app 324684580   (repeatedly over time)

ASSERT PASS: A3 exit 0 (exit 0)
ASSERT PASS: A3 empty object with note

### A4 watch-listing, no store
$ app-store-pp-cli watch-listing 324684580 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "324684580",
  "storefront": "us",
  "snapshots": 0,
  "previous": null,
  "latest": null,
  "changed": false,
  "changes": [],
  "note": "no local listing snapshots yet; run: app-store-pp-cli app 324684580 --country us   (twice, at different times)"
}

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli app 324684580 --country us   (twice, at different times)

ASSERT PASS: A4 exit 0 (exit 0)
ASSERT PASS: A4 empty object with note

### A5 review-digest, no store
$ app-store-pp-cli review-digest 6448311069 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "storefronts": [
    "us"
  ],
  "total": 0,
  "averageRating": null,
  "stars": {
    "1": 0,
    "2": 0,
    "3": 0,
    "4": 0,
    "5": 0
  },
  "byVersion": [],
  "byStorefront": [],
  "lowStarReviews": 0,
  "complaintTerms": [],
  "note": "no local reviews yet; run: app-store-pp-cli reviews 6448311069 --countries us"
}

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli reviews 6448311069 --countries us

ASSERT PASS: A5 exit 0 (exit 0)
ASSERT PASS: A5 zero digest with note

### A6 search, no store
$ app-store-pp-cli search crash --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
no local mirror at <wt>/.acceptance-home/data/data.db
run: app-store-pp-cli reviews <app> or app-store-pp-cli app <app>   (to store reviews and listings first)
note: nothing stored yet, so nothing to search

ASSERT PASS: A6 exit 0 (exit 0)
ASSERT PASS: A6 stdout is []
ASSERT PASS: A6 stderr names the missing mirror
ASSERT PASS: A7 no command created the store file (absent)

=============== PHASE B: empty tables at <wt>/.acceptance-home/empty/data.db
empty store tables: 4 app tables, 0 rows

### B1 movers, empty tables
$ app-store-pp-cli movers --genre 6014 --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
note: no stored captures of us iphone 6014 Games top-free (any source); run: app-store-pp-cli top --chart top-free --genre 6014 --platform iphone --country us   (twice, at different times)

ASSERT PASS: B1 exit 0 (exit 0)
ASSERT PASS: B1 stdout is []
ASSERT PASS: B1 note

### B2 rank-history, empty tables
$ app-store-pp-cli rank-history 324684580 --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "324684580",
  "storefront": "us",
  "platform": "iphone",
  "genreId": "36",
  "genre": "All Apps",
  "chart": "top-free",
  "captures": 0,
  "charted": 0,
  "firstSeen": null,
  "peak": null,
  "lastSeen": null,
  "latest": null,
  "series": [],
  "note": "no stored captures of us iphone 36 All Apps top-free (any source); run: app-store-pp-cli top --chart top-free --genre 36 --platform iphone --country us"
}

--- stderr


ASSERT PASS: B2 empty series + note

### B3 keyword-history, empty tables
$ app-store-pp-cli keyword-history spotify --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
{
  "term": "spotify",
  "storefront": "us",
  "captures": 0,
  "foundIn": 0,
  "bestRank": null,
  "latestRank": null,
  "change": null,
  "series": [],
  "note": "no stored keyword captures for \"spotify\" in us; run: app-store-pp-cli keyword-rank \"spotify\" --country us   (repeatedly over time)"
}

--- stderr


ASSERT PASS: B3 empty series + note

### B4 watch-listing, empty tables
$ app-store-pp-cli watch-listing 324684580 --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "324684580",
  "storefront": "us",
  "snapshots": 0,
  "previous": null,
  "latest": null,
  "changed": false,
  "changes": [],
  "note": "no stored snapshots of app 324684580 in us; run: app-store-pp-cli app 324684580 --country us   (twice, at different times)"
}

--- stderr


ASSERT PASS: B4 no changes + note

### B5 review-digest, empty tables
$ app-store-pp-cli review-digest 6448311069 --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "storefronts": [
    "us"
  ],
  "total": 0,
  "averageRating": null,
  "stars": {
    "1": 0,
    "2": 0,
    "3": 0,
    "4": 0,
    "5": 0
  },
  "byVersion": [],
  "byStorefront": [],
  "lowStarReviews": 0,
  "complaintTerms": [],
  "note": "no stored reviews of app 6448311069 in us; run: app-store-pp-cli reviews 6448311069 --countries us"
}

--- stderr


ASSERT PASS: B5 zero digest + note

### B6 search, empty tables
$ app-store-pp-cli search crash --db <wt>/.acceptance-home/empty/data.db --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
note: no stored app listings or reviews match "crash"

ASSERT PASS: B6 stdout is []

=============== PHASE C: synthetic seed at <wt>/.acceptance-home/data/data.db
    s3a_fixtures_test.go:319: seeded <wt>/.acceptance-home/data/data.db
--- PASS: TestS3ASeedAcceptanceStore (0.01s)
PASS
ok  	app-store-pp-cli/internal/cli	0.386s
independent row counts (sqlite3): charts=25 snapshots=7 reviews=14 keywords=6

### C1 movers: two web captures of us iphone 6014 top-free
$ app-store-pp-cli movers --chart top-free --genre 6014 --country us --json --home .acceptance-home
exit=0
--- stdout
[
  {
    "movement": "climbed",
    "appId": "1621328561",
    "title": "MONOPOLY GO!",
    "developerName": "Scopely, Inc.",
    "rank": 3,
    "previousRank": 5,
    "delta": 2,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  },
  {
    "movement": "climbed",
    "appId": "1482155847",
    "title": "Royal Match",
    "developerName": "Dream Games",
    "rank": 1,
    "previousRank": 2,
    "delta": 1,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  },
  {
    "movement": "dropped",
    "appId": "1094591345",
    "title": "Pokémon GO",
    "developerName": "Niantic, Inc.",
    "rank": 5,
    "previousRank": 3,
    "delta": -2,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  },
  {
    "movement": "dropped",
    "appId": "553834731",
    "title": "Candy Crush Saga",
    "developerName": "King",
    "rank": 2,
    "previousRank": 1,
    "delta": -1,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  },
  {
    "movement": "entered",
    "appId": "6446308106",
    "title": "Sample New Game",
    "developerName": "Sample Studio",
    "rank": 4,
    "previousRank": null,
    "delta": null,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  },
  {
    "movement": "exited",
    "appId": "431946152",
    "title": "Roblox",
    "developerName": "Roblox Corporation",
    "rank": null,
    "previousRank": 4,
    "delta": null,
    "capturedAt": "2026-10-21T08:00:00Z",
    "previousCapturedAt": "2026-10-20T08:00:00Z"
  }
]

--- stderr
movers: us iphone 6014 Games top-free (web), 200 ranks: 2026-10-20T08:00:00Z -> 2026-10-21T08:00:00Z: 2 climbed, 2 dropped, 1 entered, 1 exited

ASSERT PASS: C1 entered = [6446308106] at #4
ASSERT PASS: C1 exited = [431946152] from #4
ASSERT PASS: C1 climbed with deltas
ASSERT PASS: C1 dropped with deltas
ASSERT PASS: C1 no unchanged rows
ASSERT PASS: C1 entered matches raw SQL set difference (6446308106)
ASSERT PASS: C1 exited matches raw SQL set difference (431946152)

### C2 movers: one capture of gb iphone 36 top-paid
$ app-store-pp-cli movers --chart top-paid --genre 36 --country gb --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
note: only one comparable capture of gb iphone 36 All Apps top-paid (any source) (2026-10-20T08:00:00Z, web, 200 ranks); movers needs two; run: app-store-pp-cli top --chart top-paid --genre 36 --platform iphone --country gb

ASSERT PASS: C2 exit 0 (exit 0)
ASSERT PASS: C2 stdout is []
ASSERT PASS: C2 honest note

### C3 movers --agent (envelope)
$ app-store-pp-cli movers --chart top-free --genre games --limit 1 --agent --home .acceptance-home
exit=0
--- stdout
{
  "meta": {
    "source": "local"
  },
  "results": [
    {
      "appId": "1621328561",
      "capturedAt": "2026-10-21T08:00:00Z",
      "delta": 2,
      "developerName": "Scopely, Inc.",
      "movement": "climbed",
      "previousCapturedAt": "2026-10-20T08:00:00Z",
      "previousRank": 5,
      "rank": 3,
      "title": "MONOPOLY GO!"
    },
    {
      "appId": "1094591345",
      "capturedAt": "2026-10-21T08:00:00Z",
      "delta": -2,
      "developerName": "Niantic, Inc.",
      "movement": "dropped",
      "previousCapturedAt": "2026-10-20T08:00:00Z",
      "previousRank": 3,
      "rank": 5,
      "title": "Pokémon GO"
    },
    {
      "appId": "6446308106",
      "capturedAt": "2026-10-21T08:00:00Z",
      "delta": null,
      "developerName": "Sample Studio",
      "movement": "entered",
      "previousCapturedAt": "2026-10-20T08:00:00Z",
      "previousRank": null,
      "rank": 4,
      "title": "Sample New Game"
    },
    {
      "appId": "431946152",
      "capturedAt": "2026-10-21T08:00:00Z",
      "delta": null,
      "developerName": "Roblox Corporation",
      "movement": "exited",
      "previousCapturedAt": "2026-10-20T08:00:00Z",
      "previousRank": 4,
      "rank": null,
      "title": "Roblox"
    }
  ]
}

--- stderr
movers: us iphone 6014 Games top-free (web), 200 ranks: 2026-10-20T08:00:00Z -> 2026-10-21T08:00:00Z: 2 climbed, 2 dropped, 1 entered, 1 exited

ASSERT PASS: C3 agent envelope, source local, 4 rows

### C4 movers human
$ app-store-pp-cli movers --chart top-free --genre 6014 --human-friendly --home .acceptance-home
exit=0
--- stdout
us iphone 6014 Games top-free (web), 200 ranks: 2026-10-20T08:00:00Z -> 2026-10-21T08:00:00Z: 2 climbed, 2 dropped, 1 entered, 1 exited
MOVEMENT  RANK  PREV  DELTA  APP ID  TITLE
climbed           3             5             +2             1621328561      MONOPOLY GO!
climbed           1             2             +1             1482155847      Royal Match
dropped           5             3             -2             1094591345      Pokémon GO
dropped           2             1             -1             553834731       Candy Crush Saga
entered           4             -             -              6446308106      Sample New Game
exited            -             4             -              431946152       Roblox

--- stderr



### C5 rank-history 553834731 top-grossing (rss) with a gap
$ app-store-pp-cli rank-history 553834731 --chart top-grossing --genre 36 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "553834731",
  "title": "Candy Crush Saga",
  "storefront": "us",
  "platform": "iphone",
  "genreId": "36",
  "genre": "All Apps",
  "chart": "top-grossing",
  "captures": 3,
  "charted": 2,
  "firstSeen": {
    "capturedAt": "2026-10-19T08:00:00Z",
    "rank": 5
  },
  "peak": {
    "capturedAt": "2026-10-21T08:00:00Z",
    "rank": 3
  },
  "lastSeen": {
    "capturedAt": "2026-10-21T08:00:00Z",
    "rank": 3
  },
  "latest": {
    "capturedAt": "2026-10-21T08:00:00Z",
    "rank": 3,
    "inChart": true,
    "source": "rss",
    "rowLimit": 100
  },
  "series": [
    {
      "capturedAt": "2026-10-19T08:00:00Z",
      "rank": 5,
      "inChart": true,
      "source": "rss",
      "rowLimit": 100
    },
    {
      "capturedAt": "2026-10-20T08:00:00Z",
      "rank": null,
      "inChart": false,
      "source": "rss",
      "rowLimit": 100
    },
    {
      "capturedAt": "2026-10-21T08:00:00Z",
      "rank": 3,
      "inChart": true,
      "source": "rss",
      "rowLimit": 100
    }
  ]
}

--- stderr


ASSERT PASS: C5 series ranks [5,null,3]
ASSERT PASS: C5 first/peak/last

### C6 rank-history 6446308106 in All Apps (never charted)
$ app-store-pp-cli rank-history 6446308106 --genre 36 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6446308106",
  "storefront": "us",
  "platform": "iphone",
  "genreId": "36",
  "genre": "All Apps",
  "chart": "top-free",
  "captures": 0,
  "charted": 0,
  "firstSeen": null,
  "peak": null,
  "lastSeen": null,
  "latest": null,
  "series": [],
  "elsewhere": [
    {
      "storefront": "us",
      "platform": "iphone",
      "genreId": "6014",
      "chart": "top-free",
      "source": "web",
      "captures": 1,
      "bestRank": 4,
      "lastSeenAt": "2026-10-21T08:00:00Z"
    }
  ],
  "note": "no stored captures of us iphone 36 All Apps top-free (any source); run: app-store-pp-cli top --chart top-free --genre 36 --platform iphone --country us; it appears in 1 other stored chart(s), listed under elsewhere"
}

--- stderr


ASSERT PASS: C6 empty series, note, elsewhere = 6014

### C7 rank-history human
$ app-store-pp-cli rank-history 553834731 --chart top-grossing --human-friendly --home .acceptance-home
exit=0
--- stdout
553834731 Candy Crush Saga in us iphone 36 All Apps top-grossing
captures: 3, charted in: 2
first seen: #5 on 2026-10-19 08:00
peak: #3 on 2026-10-21 08:00
last seen: #3 on 2026-10-21 08:00
CAPTURED  RANK  SOURCE
2026-10-19 08:00  5             rss/100
2026-10-20 08:00  -             rss/100
2026-10-21 08:00  3             rss/100

--- stderr



### C8 keyword-history spotify --app 324684580
$ app-store-pp-cli keyword-history spotify --app 324684580 --json --home .acceptance-home
exit=0
--- stdout
{
  "term": "spotify",
  "storefront": "us",
  "appId": "324684580",
  "captures": 3,
  "foundIn": 3,
  "bestRank": {
    "capturedAt": "2026-10-20T08:00:00Z",
    "rank": 1
  },
  "latestRank": 1,
  "change": 1,
  "series": [
    {
      "capturedAt": "2026-10-20T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "appId": "324684580",
      "rank": 1,
      "found": true,
      "depth": 50,
      "resultCount": 50
    },
    {
      "capturedAt": "2026-10-21T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "appId": "324684580",
      "rank": 2,
      "found": true,
      "depth": 50,
      "resultCount": 50
    },
    {
      "capturedAt": "2026-10-22T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "appId": "324684580",
      "rank": 1,
      "found": true,
      "depth": 3,
      "resultCount": 48
    }
  ]
}

--- stderr


ASSERT PASS: C8 ranks [1,2,1], best 1, change +1

### C9 keyword-history spotify --serp 2
$ app-store-pp-cli keyword-history spotify --serp 2 --json --home .acceptance-home
exit=0
--- stdout
{
  "term": "spotify",
  "storefront": "us",
  "captures": 3,
  "foundIn": 0,
  "bestRank": null,
  "latestRank": null,
  "change": null,
  "series": [
    {
      "capturedAt": "2026-10-20T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "appId": "324684580",
      "rank": 1,
      "found": true,
      "depth": 50,
      "resultCount": 50,
      "top": [
        {
          "position": 1,
          "appId": "324684580",
          "title": "Spotify: Music and Podcasts"
        },
        {
          "position": 2,
          "appId": "1017492454"
        }
      ]
    },
    {
      "capturedAt": "2026-10-21T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "appId": "324684580",
      "rank": 2,
      "found": true,
      "depth": 50,
      "resultCount": 50,
      "top": [
        {
          "position": 1,
          "appId": "1017492454"
        },
        {
          "position": 2,
          "appId": "324684580",
          "title": "Spotify: Music and Podcasts"
        }
      ]
    },
    {
      "capturedAt": "2026-10-22T08:00:00Z",
      "platform": "iphone",
      "source": "itunes",
      "rank": null,
      "found": false,
      "depth": 3,
      "resultCount": 48,
      "top": [
        {
          "position": 1,
          "appId": "324684580",
          "title": "Spotify: Music and Podcasts"
        },
        {
          "position": 2,
          "appId": "284035177"
        }
      ]
    }
  ]
}

--- stderr


ASSERT PASS: C9 replayed top-2 with stored titles

### C10 keyword-history 'podcast app' --app 553834731 (never found)
$ app-store-pp-cli keyword-history podcast app --app 553834731 --json --home .acceptance-home
exit=0
--- stdout
{
  "term": "podcast app",
  "storefront": "us",
  "appId": "553834731",
  "captures": 1,
  "foundIn": 0,
  "bestRank": null,
  "latestRank": null,
  "change": null,
  "series": [],
  "note": "app 553834731 was not found in any of the 1 stored captures of \"podcast app\" in us (looked up to 200 results deep)"
}

--- stderr


ASSERT PASS: C10 empty series + note

### C11 watch-listing 324684580 (version change)
$ app-store-pp-cli watch-listing 324684580 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "324684580",
  "title": "Spotify: Music and Podcasts",
  "storefront": "us",
  "source": "page+lookup",
  "snapshots": 3,
  "previous": {
    "capturedAt": "2026-10-21T08:00:00Z",
    "version": "9.1.90",
    "contentHash": "7a5be683e61abade025bfd33"
  },
  "latest": {
    "capturedAt": "2026-10-22T08:00:00Z",
    "version": "9.1.92",
    "contentHash": "17dbfd5545b345fc88e0d2c3"
  },
  "changed": true,
  "changes": [
    {
      "field": "version",
      "from": "9.1.90",
      "to": "9.1.92",
      "detail": {
        "previousReleasedAt": "2026-10-12T10:00:00Z",
        "releaseNotes": "New: smarter shuffle.",
        "releasedAt": "2026-10-19T13:57:56Z"
      }
    },
    {
      "field": "inAppPurchases",
      "from": [
        "Premium Individual: $11.99",
        "Premium Duo: $16.99"
      ],
      "to": [
        "Premium Individual: $12.99",
        "Premium Duo: $16.99"
      ],
      "detail": {
        "added": [],
        "removed": [],
        "repriced": [
          {
            "name": "Premium Individual",
            "from": "$11.99",
            "to": "$12.99"
          }
        ]
      }
    },
    {
      "field": "events",
      "from": [],
      "to": [
        "6740000001"
      ],
      "detail": {
        "added": [
          {
            "eventId": "6740000001",
            "title": "Sample Live Session",
            "startDate": "2026-10-25T18:00:00Z",
            "endDate": "2026-10-25T20:00:00Z"
          }
        ],
        "removed": []
      }
    }
  ]
}

--- stderr


ASSERT PASS: C11 fields version,inAppPurchases,events
ASSERT PASS: C11 version 9.1.90 -> 9.1.92
ASSERT PASS: C11 IAP repriced

### C12 watch-listing 553834731 (equal hashes)
$ app-store-pp-cli watch-listing 553834731 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "553834731",
  "title": "Candy Crush Saga",
  "storefront": "us",
  "source": "page+lookup",
  "snapshots": 2,
  "previous": {
    "capturedAt": "2026-10-20T08:00:00Z",
    "version": "1.300.0",
    "contentHash": "a967638e00831199823ed7eb"
  },
  "latest": {
    "capturedAt": "2026-10-21T08:00:00Z",
    "version": "1.300.0",
    "contentHash": "a967638e00831199823ed7eb"
  },
  "changed": false,
  "changes": [],
  "note": "no listing change between 2026-10-20T08:00:00Z and 2026-10-21T08:00:00Z (same content hash)"
}

--- stderr


ASSERT PASS: C12 nothing reported
ASSERT PASS: C12 raw SQL: 1 distinct hash over 2 snapshots (1/2)

### C13 watch-listing 6448311069 (one snapshot)
$ app-store-pp-cli watch-listing 6448311069 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "title": "ChatGPT",
  "storefront": "us",
  "source": "lookup",
  "snapshots": 1,
  "previous": null,
  "latest": {
    "capturedAt": "2026-10-20T08:00:00Z",
    "version": "1.2026.290",
    "contentHash": "7da34ef250ffd2bdc9f116e5"
  },
  "changed": false,
  "changes": [],
  "note": "only one lookup snapshot of app 6448311069 in us (2026-10-20T08:00:00Z); watch-listing needs two; run: app-store-pp-cli app 6448311069 --country us   (twice, at different times)"
}

--- stderr


ASSERT PASS: C13 previous null + note

### C14 watch-listing human
$ app-store-pp-cli watch-listing 324684580 --human-friendly --home .acceptance-home
exit=0
--- stdout
324684580 Spotify: Music and Podcasts in us (page+lookup snapshots: 3)
2026-10-21 08:00 -> 2026-10-22 08:00
  version: 9.1.90 -> 9.1.92
  inAppPurchases: Premium Individual: $11.99, Premium Duo: $16.99 -> Premium Individual: $12.99, Premium Duo: $16.99
  events: (none) -> 6740000001

--- stderr



### C15 review-digest 6448311069 (us)
$ app-store-pp-cli review-digest 6448311069 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "storefronts": [
    "us"
  ],
  "total": 10,
  "averageRating": 3.2,
  "stars": {
    "1": 2,
    "2": 2,
    "3": 1,
    "4": 2,
    "5": 3
  },
  "byVersion": [
    {
      "version": "1.2026.290",
      "reviews": 6,
      "averageRating": 3,
      "stars": {
        "1": 2,
        "2": 1,
        "3": 0,
        "4": 1,
        "5": 2
      }
    },
    {
      "version": "1.2026.280",
      "reviews": 4,
      "averageRating": 3.5,
      "stars": {
        "1": 0,
        "2": 1,
        "3": 1,
        "4": 1,
        "5": 1
      }
    }
  ],
  "byStorefront": [
    {
      "storefront": "us",
      "reviews": 10,
      "averageRating": 3.2
    }
  ],
  "lowStarReviews": 4,
  "complaintTerms": [
    {
      "term": "crash",
      "reviews": 3,
      "share": 0.75
    },
    {
      "term": "login",
      "reviews": 3,
      "share": 0.75
    }
  ],
  "oldestReviewAt": "2026-10-19T23:00:00Z",
  "newestReviewAt": "2026-10-20T08:00:00Z"
}

--- stderr


ASSERT PASS: C15 total 10, mean 3.2
ASSERT PASS: C15 star histogram matches raw SQL (1:2,2:2,3:1,4:2,5:3)
ASSERT PASS: C15 per-version counts match raw SQL (1.2026.290:6,1.2026.280:4)
ASSERT PASS: C15 'crash' low-star review count matches raw SQL (3)
ASSERT PASS: C15 complaint terms exclude stopwords

### C16 review-digest --countries all
$ app-store-pp-cli review-digest 6448311069 --countries all --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "storefronts": [
    "gb",
    "us"
  ],
  "total": 14,
  "averageRating": 3.07,
  "stars": {
    "1": 4,
    "2": 2,
    "3": 1,
    "4": 3,
    "5": 4
  },
  "byVersion": [
    {
      "version": "1.2026.290",
      "reviews": 9,
      "averageRating": 3.11,
      "stars": {
        "1": 3,
        "2": 1,
        "3": 0,
        "4": 2,
        "5": 3
      }
    },
    {
      "version": "1.2026.280",
      "reviews": 4,
      "averageRating": 3.5,
      "stars": {
        "1": 0,
        "2": 1,
        "3": 1,
        "4": 1,
        "5": 1
      }
    },
    {
      "version": "unknown",
      "reviews": 1,
      "averageRating": 1,
      "stars": {
        "1": 1,
        "2": 0,
        "3": 0,
        "4": 0,
        "5": 0
      }
    }
  ],
  "byStorefront": [
    {
      "storefront": "us",
      "reviews": 10,
      "averageRating": 3.2
    },
    {
      "storefront": "gb",
      "reviews": 4,
      "averageRating": 2.75
    }
  ],
  "lowStarReviews": 6,
  "complaintTerms": [
    {
      "term": "crash",
      "reviews": 4,
      "share": 0.67
    },
    {
      "term": "login",
      "reviews": 3,
      "share": 0.5
    },
    {
      "term": "history",
      "reviews": 2,
      "share": 0.33
    }
  ],
  "oldestReviewAt": "2026-10-19T19:00:00Z",
  "newestReviewAt": "2026-10-20T08:00:00Z"
}

--- stderr


ASSERT PASS: C16 total matches raw SQL count (14)

### C17 review-digest --countries all --since-version 1.2026.290
$ app-store-pp-cli review-digest 6448311069 --countries all --since-version 1.2026.290 --json --home .acceptance-home
exit=0
--- stdout
{
  "appId": "6448311069",
  "storefronts": [
    "gb",
    "us"
  ],
  "sinceVersion": "1.2026.290",
  "total": 9,
  "averageRating": 3.11,
  "stars": {
    "1": 3,
    "2": 1,
    "3": 0,
    "4": 2,
    "5": 3
  },
  "byVersion": [
    {
      "version": "1.2026.290",
      "reviews": 9,
      "averageRating": 3.11,
      "stars": {
        "1": 3,
        "2": 1,
        "3": 0,
        "4": 2,
        "5": 3
      }
    }
  ],
  "byStorefront": [
    {
      "storefront": "us",
      "reviews": 6,
      "averageRating": 3
    },
    {
      "storefront": "gb",
      "reviews": 3,
      "averageRating": 3.33
    }
  ],
  "lowStarReviews": 4,
  "complaintTerms": [
    {
      "term": "crash",
      "reviews": 3,
      "share": 0.75
    },
    {
      "term": "login",
      "reviews": 2,
      "share": 0.5
    }
  ],
  "oldestReviewAt": "2026-10-19T20:00:00Z",
  "newestReviewAt": "2026-10-20T08:00:00Z",
  "excludedUnknownVersion": 1,
  "note": "1 stored review(s) without a version were left out by --since-version"
}

--- stderr


ASSERT PASS: C17 9 reviews, 1 unknown-version excluded

### C18 review-digest human
$ app-store-pp-cli review-digest 6448311069 --countries all --human-friendly --home .acceptance-home
exit=0
--- stdout
app 6448311069, gb,us: 14 stored review(s), mean 3.07 stars
  5 star       4  ############
  4 star       3  #########
  3 star       1  ###
  2 star       2  ######
  1 star       4  ############
VERSION  REVIEWS  MEAN  1  2  3  4  5
1.2026.290       9                3.11          3          1          0          2          3
1.2026.280       4                3.50          0          1          1          1          1
unknown          1                1.00          1          0          0          0          0
complaint terms in 6 low-star review(s):
  crash                   4   67%
  login                   3   50%
  history                 2   33%

--- stderr



### C19 search crash
$ app-store-pp-cli search crash --json --home .acceptance-home
exit=0
--- stdout
[
  {
    "type": "review",
    "appId": "6448311069",
    "storefront": "gb",
    "title": "Crash",
    "version": "1.2026.290",
    "rating": 1,
    "reviewId": "90000000013",
    "snippet": "Crash when uploading a photo",
    "body": "Crash when uploading a photo",
    "reviewedAt": "2026-10-19T20:00:00Z"
  },
  {
    "type": "review",
    "appId": "6448311069",
    "storefront": "us",
    "title": "Crash again",
    "version": "1.2026.290",
    "rating": 1,
    "reviewId": "90000000009",
    "snippet": "Every crash wipes my chat history",
    "body": "Every crash wipes my chat history",
    "reviewedAt": "2026-10-20T00:00:00Z"
  },
  {
    "type": "review",
    "appId": "6448311069",
    "storefront": "us",
    "title": "Crash on start",
    "version": "1.2026.290",
    "rating": 2,
    "reviewId": "90000000007",
    "snippet": "Crash right after login, subscription lost",
    "body": "Crash right after login, subscription lost",
    "reviewedAt": "2026-10-20T02:00:00Z"
  },
  {
    "type": "review",
    "appId": "6448311069",
    "storefront": "us",
    "title": "Login loop",
    "version": "1.2026.280",
    "rating": 2,
    "reviewId": "90000000008",
    "snippet": "Login fails then a crash",
    "body": "Login fails then a crash",
    "reviewedAt": "2026-10-20T01:00:00Z"
  }
]

--- stderr


ASSERT PASS: C19 4 hits, all reviews containing crash
ASSERT PASS: C19 hit count matches raw SQL LIKE count (4)

### C20 search zzqxv (no match)
$ app-store-pp-cli search zzqxv --json --home .acceptance-home
exit=0
--- stdout
[]

--- stderr
note: no stored app listings or reviews match "zzqxv"

ASSERT PASS: C20 exit 0 (exit 0)
ASSERT PASS: C20 []
ASSERT PASS: C20 honest note

### C21 search spotify --type apps
$ app-store-pp-cli search spotify --type apps --json --home .acceptance-home
exit=0
--- stdout
[
  {
    "type": "app",
    "appId": "324684580",
    "storefront": "us",
    "title": "Spotify: Music and Podcasts",
    "subtitle": "Music, podcasts and audiobooks",
    "version": "9.1.92",
    "averageRating": 4.5,
    "ratingCount": 1000,
    "capturedAt": "2026-10-22T08:00:00Z"
  }
]

--- stderr


ASSERT PASS: C21 one listing hit from the latest snapshot

### C22 search crash --csv
$ app-store-pp-cli search crash --type reviews --storefront gb --csv --home .acceptance-home
exit=0
--- stdout
appId,body,rating,reviewId,reviewedAt,snippet,storefront,title,type,version
6448311069,Crash when uploading a photo,1,90000000013,2026-10-19T20:00:00Z,Crash when uploading a photo,gb,Crash,review,1.2026.290

--- stderr



### C23 search human
$ app-store-pp-cli search login --human-friendly --home .acceptance-home
exit=0
--- stdout
TYPE  APP ID  SF  STARS  TITLE   TEXT
review        6448311069      us          2              Login loop      Login fails then a crash
review        6448311069      us          1              Too slow        Slow answers and the login keeps timing out
review        6448311069      us          2              Crash on start  Crash right after login, subscription lost

--- stderr



=============== NEGATIVE / PROBES

### N1 rank-history bundle id
$ app-store-pp-cli rank-history com.spotify.client --json --home .acceptance-home
exit=2
--- stdout


--- stderr
Error: local history is keyed by the numeric App Store id and "com.spotify.client" is a bundle id; run 'app-store-pp-cli app com.spotify.client' to see its id

ASSERT PASS: N1 usage error (exit 2)

### N2 watch-listing dogfood sentinel
$ app-store-pp-cli watch-listing __printing_press_invalid__ --home .acceptance-home
exit=2
--- stdout


--- stderr
Error: unrecognized app reference "__printing_press_invalid__"; pass a numeric id (324684580), id324684580, an App Store URL or a bundle id (com.spotify.client)

ASSERT PASS: N2 usage error (exit 2)

### N3 search bad --type
$ app-store-pp-cli search crash --type users --home .acceptance-home
exit=2
--- stdout


--- stderr
Error: --type: unknown type "users"; use all, apps or reviews

ASSERT PASS: N3 usage error (exit 2)

### N4 movers bad --chart
$ app-store-pp-cli movers --chart top-weekly --home .acceptance-home
exit=2
--- stdout


--- stderr
Error: --chart: unknown chart "top-weekly"; use top-free, top-paid or top-grossing

ASSERT PASS: N4 usage error (exit 2)

### N5 review-digest missing <app> with --json
$ app-store-pp-cli review-digest --json --home .acceptance-home
exit=2
--- stdout


--- stderr
Usage:
  app-store-pp-cli review-digest <app> [flags]

Examples:
  app-store-pp-cli review-digest 6448311069 --countries us,gb --agent
  app-store-pp-cli review-digest 324684580 --since-version 9.1 --terms 10
  app-store-pp-cli review-digest 553834731 --countries all --json --select stars,complaintTerms

Flags:
      --countries string       Storefronts to include: us,gb,... or all (default: --country)
      --db string              Local snapshot database path (default: the CLI data directory)
  -h, --help                   help for review-digest
      --since-version string   Only reviews written for this app version or a newer one (e.g. 9.1)
      --terms int              Maximum complaint terms to list (0 = none) (default 20)

Global Flags:
      --agent                    Set agent-friendly output defaults (--json --compact --no-input --no-color)
      --audit-dir string         Aggregate the receipt and index under this audit directory
      --ca-cert string           PEM CA bundle added to the system trust roots
      --client-profile string    Select the tenant-gated client profile (env: PRINTING_PRESS_CLIENT_PROFILE)
      --compact                  Return only key fields (id, name, status, timestamps) for minimal token usage
      --config string            Config file path
      --country string           App Store storefront as a two-letter country code (us, gb, jp, pk ...) (default "us")
      --csv                      Output as CSV (table and array responses)
 
ASSERT PASS: N5 usage error (exit 2)

### N6 keyword-history --serp 500
$ app-store-pp-cli keyword-history spotify --serp 500 --home .acceptance-home
exit=2
--- stdout


--- stderr
Error: --serp must be between 0 (off) and 200, got 500

ASSERT PASS: N6 usage error (exit 2)

### N7 movers --data-source live (local command)
$ app-store-pp-cli movers --genre 36 --data-source live --json --home .acceptance-home
exit=1
--- stdout


--- stderr
Error: no live equivalent for this command (requested "live"); use --data-source local or --data-source auto

(exit 1: a local-only command refuses --data-source live)

### D-movers --dry-run --json
$ app-store-pp-cli movers --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"movers","would":"run movers; no changes made"}

--- stderr


ASSERT PASS: D-movers dry-run envelope

### D-rank-history --dry-run --json
$ app-store-pp-cli rank-history --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"rank-history","would":"run rank-history; no changes made"}

--- stderr


ASSERT PASS: D-rank-history dry-run envelope

### D-keyword-history --dry-run --json
$ app-store-pp-cli keyword-history --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"keyword-history","would":"run keyword-history; no changes made"}

--- stderr


ASSERT PASS: D-keyword-history dry-run envelope

### D-watch-listing --dry-run --json
$ app-store-pp-cli watch-listing --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"watch-listing","would":"run watch-listing; no changes made"}

--- stderr


ASSERT PASS: D-watch-listing dry-run envelope

### D-review-digest --dry-run --json
$ app-store-pp-cli review-digest --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"review-digest","would":"run review-digest; no changes made"}

--- stderr


ASSERT PASS: D-review-digest dry-run envelope

### D-search --dry-run --json
$ app-store-pp-cli search --dry-run --json --home .acceptance-home
exit=0
--- stdout
{"dry_run":true,"action":"search","would":"run search; no changes made"}

--- stderr


ASSERT PASS: D-search dry-run envelope

=============== RESULT: 71 passed, 0 failed
```
