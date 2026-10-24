# Slice 3b (transcendence) build log

Branch `slice-3b`, worktree `working/wt-3b`, commit `9c1ba69` (parent `b1786f3`, slice 1).
Date 2026-10-24 (PKT). Live Apple requests: **0** (budget 0). All data in acceptance runs is synthetic.

Manifest transcendence rows built in this slice: **5 of 5** (traction, impact, changes,
keyword-share, breakouts). Gate inside the worktree: `go build ./...`, `go vet ./...`,
`go test -count=1 ./...` all green; `gofmt -l` clean.

## Files (all slice-3b owned)
- `internal/cli/traction.go`, `impact.go`, `changes.go`, `keyword_share.go`, `breakouts.go`: the five
  scaffolds replaced. Constructor names kept (`newNovelXxxCmd`, root.go calls them); each file also
  self-registers via `registerNovelCommand` + `addNovelCommandIfAbsent` (duplicate-safe, contract rule 7).
  Scaffold header, `pp:novel-scaffold` annotation, TODO flag text and keyword-share's local
  `--country` removed. Each has `// pp:data-source local` + annotation, `mcp:read-only: true`,
  `pp:happy-args` with real ids, the manifest Long Description verbatim as Cobra `Long`, and the
  research.json example as the first `Example` line.
- `internal/cli/s3b_helpers.go`: missing-mirror guard + store open, `--since`/`--window` parsing
  (`cliutil.ParseDurationLoose`, dates, RFC3339), storefront scope, local app-ref resolution (ids,
  id123, URLs, bundle ids via stored snapshots), one-source-per-storefront ratings series, daily
  collapse, the reset rule, period buckets, featured-capture grouping, table printing.
- `internal/cli/s3b_snapshots.go`: known versions (version_entries + Lookup release dates),
  consecutive snapshot pairs, listing content-change detection and changed-field diff.
- Tests: `traction_test.go`, `impact_test.go`, `changes_test.go`, `keyword_share_test.go`,
  `breakouts_test.go` (scaffold smoke test kept, marker header removed), `s3b_helpers_test.go`,
  `s3b_seed_test.go` (synthetic scenarios written through the typed store accessors, times relative
  to a base; `TestS3BSeedAcceptanceStore` seeds stores for binary runs only when `S3B_SEED_DB` /
  `S3B_SEED_SINGLE_DB` are set, otherwise skipped).
- No foundation, root.go, research.json, .printing-press.json, README.md or SKILL.md edits (see the
  incident below for research.json).

## Commands built
| Command | Flags | Data source | Output |
|---|---|---|---|
| `traction <app> [app...]` | `--countries`, `--period daily\|monthly\|annual\|all-time` (default monthly), `--since`, `--db` | local: ratings_snapshots, chart_snapshots (+ChartCaptures), reviews | `{meta{proxy:true, download_counts:false, period, apps, apps_without_data, since, first_capture_at, storefronts_with_data, storefronts_without_data, metrics, note}, results[{appId, title, storefront, period, newRatings, ratingDeltas, reset_detected, ratingCount, ratingsSource, bestRank, bestRankChart, daysCharted, chartDaysCaptured, reviewVolume, review_cap_truncated}]}` |
| `impact <app>` | `--at latest\|version:<v>\|listing\|featured\|YYYY-MM-DD\|RFC3339` (default latest), `--window` (default 7d), `--countries`, `--db` | local: version_entries, app_snapshots, featured_snapshots, chart_snapshots, ratings_snapshots, reviews, keyword_ranks | `{meta{app, app_id, title, at, event{kind, at, version, storefront, changed_fields, detail, resolved_from}, window, window_days, before{from,to}, after{from,to}, after_complete, storefronts, storefronts_without_data, co_movement_only:true, direction, insufficient_data_rule, note}, results[{storefront, metric, series, before, after, delta, pointsBefore, pointsAfter, insufficient_data, detail}]}` |
| `changes` | `--since` (default 7d), `--apps`, `--kind version,title,subtitle,price,iap,privacy,event,editors_choice,featured`, `--countries`, `--limit` (default 200, 0 = all), `--db` | local: app_snapshots, version_entries, iap_prices, privacy_snapshots, app_events, featured_snapshots | `{meta{since, window, apps, apps_without_data, kinds, storefronts, comparisons, total_changes, limit, truncated, note}, results[{date, previousCapturedAt, appId, title, storefront, kind, change, field, from, to, detail}]}` newest first |
| `keyword-share [term...]` | root `--country`, `--top` (default 10, 1-200), `--app`, `--platform`, `--source itunes\|web`, `--limit` (default 50), `--db` | local: keyword_ranks (+ titles from app_snapshots) | `{meta{storefront, platform, source, top, terms_counted, comparable_terms, terms[{term, platform, source, lastCapturedAt, previousCapturedAt, topStored, resultCount}], terms_missing, total_apps, limit, truncated, unweighted:true, target{app_id, terms_held, held_terms, gap_terms[{term, targetRank, leaders}]}, note}, results[{appId, title, target, termsHeld, termsHeldPrevious, change, avgPosition, bestPosition, terms[{term, position, previousPosition}]}]}` |
| `breakouts` | `--genre` (id or name, default 36), `--chart` (default top-free), `--platform` (default iphone), `--since` (default 7d), `--min-markets` (default 2), `--min-gain` (default 10), `--countries`, `--limit` (default 50), `--db` | local: chart_snapshots (ChartCaptures + ChartSnapshotAt per storefront) | `{meta{platform, genre_id, genre_name, chart, since, window, min_markets, min_gain, storefronts_compared[{storefront, from, to, source, rowLimit, capturesInWindow}], storefronts_skipped[{storefront, reason, capturesInWindow}], below_min_markets, total_breakouts, limit, truncated, entered_gain_rule, note}, results[{appId, title, developerName, markets, medianGain, bestRank, storefronts[{storefront, fromRank, toRank, gain, entered, from, to}]}]}` |

pp:happy-args: traction `app=324684580;--countries=us,gb;--period=monthly`; impact
`app=324684580;--at=latest;--window=7d`; changes `--since=7d;--apps=324684580,553834731`;
keyword-share `term=spotify;--top=10;--app=324684580`; breakouts
`--genre=6014;--chart=top-free;--since=7d;--min-markets=2`. Each exits 0 with `results: []` and a
note on an empty store (verified with the binary under `PRINTING_PRESS_DOGFOOD=1`).

## Semantics and decisions
- **Common shape.** Every command prints one `{meta, results}` envelope (results = typed rows,
  `make(...,0)` so empty is `[]`). Meta carries the contract fields and an honest `note` for empty or
  partial input. Under `--agent` the framework merges it into one envelope (`meta.source: local`,
  command meta kept); `--select` projects rows and keeps meta; `--csv`/`--plain` print the rows. Human
  tables only inside `wantsHumanTable`. Missing store file: stderr hint plus the empty envelope, exit 0.
  Bad input is `usageErr` (exit 2). `--data-source live` is rejected by the framework (local strategy).
- **Naming.** Row metrics camelCase as in the contract (newRatings, bestRank, daysCharted,
  reviewVolume); the contract's flags keep their snake_case names (`reset_detected`,
  `review_cap_truncated`, `insufficient_data`); meta keys snake_case (`proxy`, `download_counts`,
  `first_capture_at`, `storefronts_compared` ...).
- **Storefront scope** (traction, impact, changes, breakouts): `--countries` list, else an explicitly
  set `--country`, else every storefront with stored data; `--countries all` also means every
  storefront with data.
- **traction.** Ratings: one source per storefront (lookup when present, else page), collapsed to the
  last count per UTC day. newRatings = sum of day-over-day increments credited to the later capture
  day; a drop of more than 1% is a reset whose post-reset count is counted as new from that day
  (`reset_detected`); a smaller drop counts 0; never negative. A period with no measured change has
  `newRatings: null` (not a fabricated 0). bestRank = lowest stored rank across the app's charts in the
  period (`bestRankChart` = platform/genre/chart); daysCharted vs chartDaysCaptured (days with a capture
  of a chart the app appears in). reviewVolume = stored reviews by review date;
  `review_cap_truncated` when a storefront has 500+ captured reviews in one sort. `--since` filters
  every stream before computing (the first in-window day is a baseline).
- **impact.** `latest` = newest stored version release (version_entries merged with Lookup
  `currentVersionReleaseDate` carried by snapshots); `version:<v>` = its release date, else first
  capture showing it; `listing` = latest content-hash change between consecutive snapshots of the same
  source (with `changed_fields`); `featured` = first capture of the latest consecutive featuring run;
  dates/RFC3339 accepted. Windows are half-open `[T-W, T)` / `[T, T+W)`; `after_complete` false while
  the after window is still open. Metrics per storefront: bestRank per chart series (points = chart
  captures in the window), newRatingsPerDay (points = capture days; rate between first and last day),
  reviewStars (points = reviews dated in the window), keywordPosition per tracked term (keyword-rank
  captures targeting the app; mean rank where found). delta = after - before only when both windows
  have >= 2 points; otherwise `insufficient_data: true` and `delta: null`. Placeholder rows say which
  metric has no data. Wording is co-movement only (`co_movement_only: true`; no causal terms, tested).
- **changes.** Fields both sources carry (version, title, price) are diffed over the merged
  lookup+page snapshot series (tightest bracket, no duplicates); subtitle and Editor's Choice only over
  page snapshots; IAP lists (stored per change) diffed by name: changed/added/removed, or `reordered`;
  privacy labels diffed per type (added/removed/changed categories, data-type changes); events are
  `new` when first seen after the storefront's first capture (baseline events are not changes);
  featuring gained/lost between consecutive Today captures per storefront/platform. Rows are dated by
  the capture that first showed the change, with `previousCapturedAt`. `meta.comparisons` counts only
  comparisons for the requested kinds. One snapshot gives `results: []` and a note.
- **keyword-share.** Per term, the latest capture's top N vs the previous capture with the same
  platform and source. Per app: termsHeld, termsHeldPrevious, change (counted only over terms with a
  previous capture), avg/best position, per-term positions. Counts unweighted. `--app` marks the
  target row (kept even past `--limit`) and lists `meta.target.gap_terms` (term, the target's own
  latest tracked rank, top-3 leaders). Annotated `pp:no-error-path-probe` because any text is a valid
  term (an untracked term is an empty result, exit 0, listed in `terms_missing`).
- **breakouts.** Per storefront, the first and last comparable capture (same source and row limit)
  inside `--since`; storefronts with < 2 captures in the window or no comparable pair are listed in
  `storefronts_skipped` with the reason; never-captured storefronts are listed only when named in
  `--countries`. An app qualifies in a storefront when it entered (gain = rowLimit+1-rank) or gained
  >= `--min-gain`; kept when it qualifies in >= `--min-markets` storefronts; ranked by market count,
  median gain, best rank. `below_min_markets` counts the single/few-market climbers excluded.

## Deviations from the contract
1. Output is a `{meta, results}` envelope for all five; "changes returns []" is `results: []`
   (with `meta.comparisons: 0` and a note), not a bare top-level `[]`.
2. Extra flags beyond the manifest examples: traction `--since`, `--db`; impact `--countries`, `--db`;
   changes `--kind`, `--countries`, `--limit`, `--db`; keyword-share `--platform`, `--source`,
   `--limit`, `--db`; breakouts `--platform`, `--min-gain` (from the brainstorm), `--countries`,
   `--limit`, `--db`. Defaults: traction `--period monthly`, breakouts `--min-markets 2` (the example
   uses 3).
3. traction reports `newRatings: null` (not 0) for periods without a measured day-over-day change.
4. The impact error-path/`latest` rules above are interpretations where the contract was silent
   (latest = newest release; points for ratings are capture days).
5. `changes` default app set is apps with stored listing snapshots (`store.TrackedApps`); `--apps`
   reaches apps that only have IAP/privacy/event history (see foundation request 1).

## Foundation requests
1. (Low priority) A store accessor listing every app id with any stored history (union of
   app_snapshots, iap_prices, privacy_snapshots, app_events, version_entries), so `changes` without
   `--apps` also covers apps captured only through commands that do not write a listing snapshot.
2. (Integration check at merge, no change requested) traction/impact/breakouts read chart rows by
   `Platform` from `parseChartPlatform` (iphone|ipad|mac), `GenreID` as the numeric id string and
   `Chart` from `appstore.NormalizeChart` (top-free|top-paid|top-grossing); keyword-share/impact read
   keyword rows with `Source` `itunes`/`web` and the target in `AppID`. Slice 2b's writers must use the
   same values.

## Incident: research.json rewritten by a structural dogfood run (restored)
Running `cli-printing-press dogfood --dir <wt-3b> --research-dir <run dir> --json` (non-live; it only
runs `--help`/agent-context, no network) to check `novel_features_check` (result: planned 5, found 5;
`reimplementation_check` 5/5 exempted via store; examples 10/10 valid) had side effects: it synced
`.printing-press.json`, README.md and SKILL.md in the worktree, wrote `dogfood-results.json`, and
re-wrote the run's `research.json` (added `novel_features_built`, re-serialized through the press's
typed `ResearchResult` struct). Actions: worktree files restored with `git checkout`,
`dogfood-results.json` deleted (never committed); `research.json` restored by removing the
`novel_features_built` block. Verified afterwards: no `novel_features_built`, `novel_features` has the
same 5 entries with all fields (name, command, description, rationale, example, why_it_matters,
group). Residual difference: the bytes are now the press's canonical Go formatting (11,141 bytes vs
the original 10,190; default fields such as `install_method: ""` re-added to `alternatives` entries).
No copy of the original existed to restore byte-for-byte. The press's own completion gate performs
the same re-serialization. Other worktrees and the main working dir were not touched (checked with
`git status`). Copy of the post-dogfood file:
`<scratchpad>/research.json.after-dogfood`.

## Acceptance (raw)

### Unit tests (`go test -count=1 -v -run 'Traction|Impact|Changes|KeywordShare|Breakouts|S3B' ./internal/cli/`)
```
--- PASS: TestNovelBreakoutsHelpWires (0.00s)
--- PASS: TestBreakoutsExcludesSingleMarketClimbers (0.02s)
--- PASS: TestBreakoutsWindowsCountriesAndNegative (0.02s)
--- PASS: TestNovelChangesHelpWires (0.00s)
--- PASS: TestChangesSingleSnapshotReturnsEmpty (0.01s)
--- PASS: TestChangesFeedCoversEveryKind (0.01s)
--- PASS: TestChangesFiltersAndLimits (0.01s)
--- PASS: TestChangesNoStoreAndDiffHelpers (0.00s)
--- PASS: TestNovelImpactHelpWires (0.00s)
--- PASS: TestImpactLatestReleaseWindows (0.01s)
--- PASS: TestImpactInsufficientDataWithOneCapture (0.01s)
--- PASS: TestImpactEventKinds (0.01s)
--- PASS: TestImpactEmptyAndNegative (0.01s)
--- PASS: TestNovelKeywordShareHelpWires (0.00s)
--- PASS: TestKeywordShareDropsLocalCountryFlag (0.00s)
--- PASS: TestKeywordShareCountsAndTargetGaps (0.00s)
--- PASS: TestKeywordShareAllTermsStorefrontAndNegative (0.01s)
--- PASS: TestS3BParseSinceAndWindow (0.00s)
--- PASS: TestS3BStorefrontScope (0.00s)
--- PASS: TestS3BChangedFieldsMedianAndFeatured (0.00s)
--- PASS: TestS3BDogfoodProbes (0.00s)
--- PASS: TestBreakoutsCountriesAllMeansStoredStorefronts (0.01s)
--- SKIP: TestS3BSeedAcceptanceStore (0.00s)
--- PASS: TestNovelTractionHelpWires (0.00s)
--- PASS: TestS3BRatingIncrements (0.00s)
--- PASS: TestS3BBucketsAndPeriods (0.00s)
--- PASS: TestTractionResetNeverNegativeDaily (0.02s)
--- PASS: TestTractionPeriodRollups (0.03s)
--- PASS: TestTractionSinceWindowAndAgentEnvelope (0.02s)
--- PASS: TestTractionEmptyAndNegative (0.02s)
PASS
ok  	app-store-pp-cli/internal/cli	0.549s
```

### Binary runs (52/52 PASS)
Built with `go build -o ./app-store-pp-cli ./cmd/app-store-pp-cli`; stores seeded by
`S3B_SEED_DB=$H/s3b-all.db S3B_SEED_SINGLE_DB=$H/s3b-single.db go test -run TestS3BSeedAcceptanceStore ./internal/cli/`
(base 2026-10-23T19:00:00Z); every run passes `--home $H` (`$H` = `<wt-3b>/.acceptance-home`).
Script: `scratchpad/scripts/s3b_acceptance.sh`; full log: `scratchpad/s3b_acceptance.log`.
```
[PASS] traction daily: never negative across a reset, reset_detected on the reset day
  $ traction 324684580 --period daily --countries us --json --db $H/s3b-all.db
  raw: [{"period":"2025-12-20","newRatings":null,"reset_detected":false,"ratingCount":null},{"period":"2025-12-28","newRatings":null,"reset_detected":false,"ratingCount":90000},{"period":"2026-10-13","newRatings":10000,"reset_detected":false,"ratingCount":100000},{"period":"2026-10-14","newRatings":500,"reset_detected":false,"ratingCount":100500},{"period":"2026-10-15","newRatings":0,"reset_detected":false,"ratingCount":100400},{"period":"2026-10-16","newRatings":2000,"reset_detected":true,"ratingCount":2000},{"period":"2026-10-17","newRatings":600,"reset_detected":false,"ratingCount":2600}]
[PASS] traction monthly: deltas summed per month, baseline month null
  raw: [{"period":"2025-12","newRatings":null,"ratingDeltas":0,"reset_detected":false,"ratingCount":90000,"bestRank":null,"daysCharted":0,"chartDaysCaptured":0,"reviewVolume":1},{"period":"2026-10","newRatings":13100,"ratingDeltas":5,"reset_detected":true,"ratingCount":2600,"bestRank":3,"daysCharted":3,"chartDaysCaptured":4,"reviewVolume":3}]
[PASS] traction annual: raw: [{"period":"2025","newRatings":null},{"period":"2026","newRatings":13100}]
[PASS] traction all-time: raw: [{"storefront":"gb","period":"all-time","newRatings":100,"ratingsSource":"page","reviewVolume":500,"review_cap_truncated":true},{"storefront":"jp","period":"all-time","newRatings":null,"ratingsSource":"lookup","reviewVolume":0,"review_cap_truncated":false},{"storefront":"us","period":"all-time","newRatings":13100,"ratingsSource":"lookup","reviewVolume":4,"review_cap_truncated":false}]
[PASS] traction meta: raw: {"proxy":true,"download_counts":false,"first_capture_at":"2025-12-28T09:00:00Z","storefronts_with_data":["gb","jp","us"],"period":"all-time"}
[PASS] traction --agent (--countries us,gb,pk): raw: {"meta":{"source":"local","proxy":true,"download_counts":false,"storefronts_with_data":["gb","us"],"storefronts_without_data":["pk"]},"rows":2,...}
[PASS] traction negative (unknown app 999999999): raw: {"results":[],"apps_without_data":["999999999"]}
[PASS] traction empty store: exit 0, raw: {"results":[],"note":"Public demand proxies, not downloads: ... No local App Store snapshot store at $H/s3b-empty-does-not-exist.db yet; ..."}
[PASS] traction usage: no app -> exit 2; --period weekly -> exit 2 ("--period \"weekly\": use daily, monthly, annual or all-time")
[PASS] traction human table: "us  all-time  13100  reset  2600  3  3/4  4" + "Public demand proxies, not downloads"
[PASS] impact latest release (--countries us):
  raw: {"event":{"kind":"version","version":"1.301.0","at":"2026-10-16T10:00:00Z"},"complete":true,"rows":[{"metric":"bestRank","series":"iphone/7012/top-free","before":15,"after":8,"delta":-7,"pointsBefore":2,"pointsAfter":2,"insufficient_data":false},{"metric":"newRatingsPerDay","series":"lookup","before":75,"after":150,"delta":75,"pointsBefore":3,"pointsAfter":3,"insufficient_data":false},{"metric":"reviewStars","series":"stored reviews","before":3,"after":4.67,"delta":1.67,"pointsBefore":2,"pointsAfter":3,"insufficient_data":false},{"metric":"keywordPosition","series":"candy (itunes, iphone)","before":5,"after":2,"delta":-3,"pointsBefore":2,"pointsAfter":2,"insufficient_data":false}]}
[PASS] impact insufficient_data with one capture ($H/s3b-single.db):
  raw: {"event":"9.1.92","rows":[{"metric":"bestRank","pointsBefore":0,"pointsAfter":0,"insufficient_data":true,"delta":null},{"metric":"newRatingsPerDay","pointsBefore":0,"pointsAfter":1,"insufficient_data":true,"delta":null},{"metric":"reviewStars","pointsBefore":0,"pointsAfter":0,"insufficient_data":true,"delta":null},{"metric":"keywordPosition","pointsBefore":0,"pointsAfter":0,"insufficient_data":true,"delta":null}]}
[PASS] impact gb (one rating capture): raw: [{"storefront":"gb","pointsBefore":0,"pointsAfter":1,"insufficient_data":true,"delta":null}]
[PASS] impact --at featured: raw: {"kind":"featured","at":"2026-10-17T09:00:00Z","storefront":"us","detail":"featured in: Game of the Day","resolved_from":"featured"}
[PASS] impact --at listing: raw: {"kind":"listing","at":"2026-10-21T08:00:00Z","storefront":"us","changed_fields":["title","version","price"],...}
[PASS] impact --at version:9.9.9: raw: {"event":null,"results":[],"note":"... Version 9.9.9 is not in the stored history of app 553834731; stored versions (newest first): 1.301.0, 1.300.0, 1.299.0."}
[PASS] impact wording: raw: {"co_movement_only":true,"note":"Co-movement only: before and after values show what moved around the event, not why it moved. A window with fewer than 2 captures is insufficient_data and has no delta."}
[PASS] impact empty store: raw: {"results":[],"event":null}; usage: --at yesterday -> exit 2; two apps -> exit 2
[PASS] changes with a single snapshot returns []: raw: {"results":[],"comparisons":0,"note":"No consecutive captures to compare in the stored history: ..."}
[PASS] changes every kind, newest first (13 rows): raw: [{"date":"2026-10-21T08:00:00Z","kind":"price","change":"changed","field":"price","from":"Free","to":"$0.99"},{"date":"2026-10-20T09:00:00Z","kind":"featured","change":"lost",...},{"date":"2026-10-17T09:00:00Z","kind":"featured","change":"gained",...},{"kind":"version","from":"1.300.0","to":"1.301.0"},{"kind":"title","from":"Candy Crush Saga","to":"Candy Crush Saga!"},{"kind":"subtitle","from":"Sweet puzzle","to":"Sweetest puzzle"},{"kind":"iap","change":"added","field":"Booster Pack","to":"$4.99"},{"kind":"iap","change":"changed","field":"Gold Bars","from":"$0.99","to":"$1.29"},{"kind":"iap","change":"removed","field":"Lives","from":"$1.99"},{"kind":"privacy","change":"changed","field":"Data Linked to You","from":"Identifiers","to":"Identifiers, Location"},{"kind":"privacy","change":"added","field":"Data Used to Track You","to":"Identifiers"},{"kind":"event","change":"new","field":"Halloween Party"},{"kind":"editors_choice","change":"gained"}]
[PASS] changes --kind iap: raw: [{"change":"added","field":"Booster Pack","from":"","to":"$4.99"},{"change":"changed","field":"Gold Bars","from":"$0.99","to":"$1.29"},{"change":"removed","field":"Lives","from":"$1.99","to":""}]
[PASS] changes --apps 850417475 (single snapshot): raw: {"results":[],"comparisons":0}
[PASS] changes --select kind,date: raw: {"rows":13,"keys":["date","kind"],"total":13}
[PASS] changes empty store: raw: {"results":[]}; usage: --kind bogus -> exit 2
[PASS] keyword-share counts (--top 5 --app 324684580, terms "music streaming" "podcast player" audiobooks):
  raw rows: 324684580 target held 2 prev 2 change -1 avg 2 best 2; 379693831 held 2 prev 0 change +1 avg 3 best 1; 1017492454 held 1 prev 1 change 0; 525463029 1/1/0; ... (gb-only app 2300000009 absent)
  meta.target.gap_terms == [{"term":"podcast player","targetRank":9,"leaders":["525463029","888422857","414834813"]}]
[PASS] keyword-share no terms: raw: {"terms_counted":4,"terms":["audiobooks","candy","music streaming","podcast player"]}
[PASS] keyword-share --country gb: raw: {"storefront":"gb","apps":["324684580","2300000009"]}
[PASS] keyword-share "nonexistent term": raw: {"results":[],"missing":["nonexistent term"]}
[PASS] keyword-share empty store: raw: {"results":[]}; --top 0 -> exit 2; --help lists --country only under Global Flags
[PASS] breakouts (--min-markets 2): raw: {"rows":[{"appId":"1000000001","markets":3,"medianGain":40},{"appId":"1000000003","markets":2,"medianGain":15}],"compared":["de","gb","jp","us"],"skipped":[{"storefront":"ca","reason":"no earlier capture in the window with the same source and row limit as the latest"},{"storefront":"fr","reason":"1 capture(s) in the window; needs 2"},{"storefront":"it","reason":"0 capture(s) in the window; needs 2"}],"below":1}
  (1000000002 climbed +80 in us only: excluded)
[PASS] breakouts --min-markets 3: raw: [{"appId":"1000000001","markets":3}]
[PASS] breakouts --countries us,gb,pk: raw: {"apps":["1000000001"],"skipped":[{"storefront":"pk","reason":"no stored capture of this chart","capturesInWindow":0}]}
[PASS] breakouts empty store: raw: {"results":[]}; --min-markets 0 -> exit 2
[PASS] happy-args on an empty store under PRINTING_PRESS_DOGFOOD=1 (all five): exit 0, results [], stderr "no local mirror at $H/data/data.db ..."
[PASS] research examples traction/keyword-share --agent on an empty store: exit 0, meta.source "local"
[PASS] --dry-run --json (all five): {"dry_run":true,"action":"<cmd>","would":"run <cmd>; no changes made"}
TOTAL: 52 passed, 0 failed
```

### Gate
```
go build ./...   ok
go vet ./...     ok
go test -count=1 ./...   all packages ok (internal/cli 2.3s, internal/store, internal/appstore, internal/mcp/cobratree ...)
gofmt -l internal cmd    (clean)
```
Lock heartbeat `cli-printing-press lock update --cli app-store-pp-cli --phase build-p1-progress`
run after commands 3 and 5 and at slice end.
