# Slice 2a (app-page family, live) — build log

Date: 2026-10-24 PKT (2026-10-23 ~19:30 UTC). Worktree `working/wt-2a`, branch `slice-2a`,
base `b1786f3` (slice 1). Build green: `go build ./...`, `go vet ./...`,
`go test -count=1 ./...` all pass. Live budget used: **13 of 15** requests, all HTTP 200,
zero 403/429 (capture log below).

## Manifest rows covered
Absorbed rows (no transcendence rows; built count stays 0/5 from this slice):
1 app details (`app`), 2 bundle-id input (`app com.spotify.client`), 3 batch lookup (`compare`,
`prices`: one Lookup per storefront), 10 ratings (`ratings`), 11 similar (`similar`),
12 developer (`developer`), 14 privacy (`privacy`), 15 version history (`versions`),
16 IAP prices (`iap`), 17 in-app events (`events`), 19 price/availability (`prices`),
20 screenshots per device (`app --full`), 21 accessibility (`app --full`), 22 age rating and
capabilities (`app --full`), 23 Editor's Choice + Featured In (`app`), 24 compare (`compare`).
All ten leaf paths resolve: `./app-store-pp-cli <cmd> --help` exits 0 with
`Usage: app-store-pp-cli <cmd> <args> [flags]` for app, privacy, versions, iap, events, similar,
developer, ratings, prices, compare.

## Files (all slice-2a owned; no foundation file touched)
- Commands: `internal/cli/{app,privacy,versions,iap,events,similar,developer,ratings,prices,compare}.go`
- Helpers: `internal/cli/s2a_helpers.go` (client seam `s2aNewClient`, clock `s2aNow`, arg
  validation, `s2aProductPage`, `s2aLookupIn`, fetch-time helpers, dedup-aware store writes,
  `{meta, items, fetch_failures}` envelope, `s2aFanoutOutcome`, table/print helpers, event status)
- Tests: `internal/cli/{app,privacy,versions,iap,events,similar,developer,ratings,prices,compare}_test.go`,
  `internal/cli/s2a_fixtures_test.go` (fake Apple over httptest serving the slice-1 fixtures plus
  synthetic public catalogue Lookup rows for us/gb/cn; no reviewer data anywhere),
  `internal/cli/s2a_contract_test.go` (checklist: registration, annotations, examples, dry-run,
  help-only, usage exit 2 with zero requests, human output, --agent/--select/--csv).

## Commands built
| Command | Flags | Source (requests) | JSON | Store writes |
|---|---|---|---|---|
| `app <app>` | `--full`, `--platform` | Lookup + canonical product page (2) | object: Lookup fields (Apple names; no description/screenshot lists unless --full) + `subtitle, editorsChoice, histogram, totalRatings, chartPosition, featuredIn, hasInAppPurchases, url, appPlatforms, screenshotCount, ratingCount, averageRating`; `--full` adds `description, versions, privacy, inAppPurchases, events, capabilities, ageRating, accessibility, similar, moreByDeveloper, screenshots` | RecordProductPage |
| `privacy <app>` | — | Lookup + page (2) | `{app, title, storefront, policyUrl, privacyTypes[{identifier,title,detail?,categories[{identifier,title,dataTypes}],purposes?}], note?}` | RecordProductPage (privacy kept only on change) |
| `versions <app>` | `--limit` 1-25 (default 25) | Lookup + page (2) | `[{version, releasedAt, notes}]` newest first | RecordProductPage (version_entries insert-or-ignore) |
| `iap <app>` | `--countries` | Lookup + page per storefront (2 each) | `{meta{app,title,storefronts[{storefront,available,hasInAppPurchases,iapCount}],unavailable,requests,notes}, items[{storefront,position,name,price,currency}], fetch_failures}` | RecordProductPage per storefront (iap_prices only on change) |
| `events <app>` | — | Lookup + page (2) | `[{eventId,title,subtitle?,kind,startDate,endDate,badge,status,detail,storefront}]` | RecordProductPage (app_events upsert) |
| `similar <app>` | `--limit` (0 = all) | Lookup + page (2) | `[{position,appId,title,subtitle,developerName,rating,ratingCount,editorsChoice,priceText,isFree,hasInAppPurchases}]` | none (capture log only) |
| `developer <developer-id-or-app>` | `--platform` iphone/ipad/mac, `--limit` (default 50) | developer Lookup limit=200 + developer page (2; 3 from an app id) | `{developer{id,name,url,storefront}, platform?, appCount, apps[...], shelves[{title,platform,count,appIds}], pageOnlyAppIds, note?, pageError?}` | none (capture log only) |
| `ratings <app>` | `--countries`, `--histogram` | Lookup per storefront (+ page each with --histogram) | `{meta{app,title,storefronts,histogram,unavailable,requests,notes}, items[{storefront,averageRating,ratingCount,currentVersionAverage,currentVersionCount,version,histogram?,histogramTotal?,fetchedAt}], fetch_failures}` | ratings_snapshots source lookup (+ RecordProductPage with --histogram) |
| `prices <app> [app...]` | `--countries` | one batched Lookup per storefront (+1 per bundle id) | `{meta{apps,storefronts,requests,notes}, items[{storefront,app,title,available,price,currency,formattedPrice}], fetch_failures}`; not sold = `available:false` with null price fields | lookup-only app snapshot per available pair |
| `compare <app> <app> [app...]` | `--countries` | one batched Lookup per storefront | `{meta{apps,storefronts,unavailable[{storefront,app}],requests,notes}, items[{storefront,app,title,developer,bundleId,genre,price,formattedPrice,currency,averageRating,ratingCount,currentVersionAverage,currentVersionCount,version,versionReleasedAt,firstReleasedAt,sizeBytes,minimumOsVersion,contentRating,languages}], fetch_failures}`; human view is a transposed table per storefront | ratings_snapshots source lookup |

Every command: `// pp:data-source live` + annotation `pp:data-source: live`, `mcp:read-only: true`,
`pp:happy-args` with real ids (app=324684580, app=553834731, developer=324684583,
prices/compare two apps), help-only branch, `dryRunOK` -> `writeDryRun`, validation with
`usageErr` (exit 2, zero requests), no `cobra.Args`/`MarkFlagRequired`, `boundCtx` before the
client, store opened and `attachCaptureLog` before the first request, JSON via
`printJSONFiltered`, `make(...,0)` lists, human output only inside `wantsHumanTable`, Example via
`strings.Trim(..., "\n")` with 2-space indent, self-registration through `registerNovelCommand` +
`addNovelCommandIfAbsent`. Errors go through `classifyAppStoreErr` (7 throttle, 3 not found,
2 usage, 5 other); fan-out loops return at once on `isRateLimited`.

## Decisions
1. **Snapshot dedup by fetch time.** A response served from the local cache keeps Apple's
   original fetch time, and the store stamps snapshots with it, so re-recording a cached page
   or Lookup would add a second snapshot at the same instant (time-series readers would count
   two captures). `s2aRecordProductPage`, `s2aRecordLookupRatings` and
   `s2aRecordLookupSnapshot` skip a write when that (app, storefront, source, second) capture is
   already stored. Lookup-sourced ratings snapshots are stamped with the Lookup's fetch time
   (`LookupResponse.Meta[batch].FetchedAt`), never "now", so a cached count is never dated
   later than Apple served it. Verified live: 7 cached Spotify page runs -> 1 page+lookup
   snapshot, 1 page ratings row.
2. **Page-family commands persist the whole page** with `store.RecordProductPage` (per the
   slice-1 notes), a superset of the per-command store column; `similar` and `developer` write
   nothing but the capture log (contract "none").
3. **Lookup before page** for every page command: gives exit 3 for unknown ids (`app 1`),
   resolves bundle ids, and the canonical trackViewUrl avoids the slug redirect (2 requests).
4. **Fan-out envelope** `{meta, items, fetch_failures}` for iap/ratings/prices/compare (always
   that shape, `fetch_failures` always present): rule 9 needs a JSON place for fetch_failures;
   `meta` is a recognised envelope-metadata key so `--csv`/`--plain` tabulate `items`, and the
   3-key object stays intact under `--agent` (`results.items`). Storefronts where an app is not
   sold go to `meta.unavailable` (iap, ratings, compare) or `available:false` rows (prices, per
   the contract); a failed fetch is never a zero row.
5. **Fan-out exit rules** (`s2aFanoutOutcome`): throttle -> exit 7 immediately, nothing printed;
   every storefront failed -> that error; nothing sold anywhere and no failures -> exit 3;
   otherwise print, warn `warning: N of M fetches failed ...` on stderr.
6. **`app` degrades on a non-throttle page failure**: Lookup fields still print, with
   `pageError` and a stderr warning (histogram `[]`, never invented). A throttle on the page
   still exits 7.
7. **`developer` disambiguates a bare number** with one Lookup (`id=N&entity=software&limit=200`):
   an artist row means a developer id; an app row (Apple returns the app itself, verified live)
   means an app id, whose artistId is then looked up. App URLs and bundle ids go straight to the
   app path; developer URLs straight to the developer path.
8. **Empty results are honest**: `[]` (or `privacyTypes: []` / `items: []`) plus a note on stderr
   in machine mode and on stdout in human mode (no versions, no events, no similar shelf, no IAP
   list, no privacy labels, no developer apps).
9. **events `status`** (upcoming/live/ended/unknown) is derived from the dates against the clock.
10. Human/stderr writes go through `s2aPrintf`/`s2aPrintln` (drop write errors in one place) so
    golangci-lint errcheck is clean on all slice files.

## Deviations from the contract
1. `iap`, `ratings`, `prices`, `compare` emit the `{meta, items, fetch_failures}` envelope
   instead of a bare row array (rows are `items`), to carry `fetch_failures` per rule 9. The
   `iap 553834731` acceptance "10 rows" is `items` (10).
2. `app` JSON adds `ratingCount`/`averageRating` (same values as `userRatingCount`/
   `averageUserRating`) and `totalRatings`; default output omits Lookup `description` and the
   three screenshot URL lists (they come with `--full`, as `description` and per-device
   `screenshots`).
3. Page commands write the full page via `RecordProductPage` (superset of the per-command
   store column).
4. Extra fields beyond the contract shapes: events `status`/`storefront`/`subtitle`, iap
   `currency`, ratings `currentVersion*`/`histogramTotal`/`fetchedAt`, developer
   `appCount`/`pageOnlyAppIds`/`appIds`, privacy `title`/`note`.
5. `versions --limit` outside 1-25 is a usage error (exit 2) rather than a silent clamp.
6. `developer --platform` accepts iphone, ipad, mac only (the Lookup entities that exist);
   other platforms exit 2.

## Foundation requests / notes for other slices
1. **Apple's Lookup CDN serves per-URL copies up to 24 h old.** Live: the single-id Spotify
   Lookup came back with `max-age=39985` (about 12.9 h old at the edge) showing 9.1.92 /
   42,719,156 ratings, while the batched Lookup 43 s later (`max-age=86400`, fresh) showed
   9.1.94 / 42,722,184 and the page already listed 9.1.94 (released 2026-10-23T17:59:53Z).
   Snapshots stamped with our fetch time can therefore be older than they look, and two
   commands hitting different Lookup URLs can record out-of-order values. Request: expose
   `Age`/`max-age` on `FetchMeta` so snapshots can carry the data's origin time; slice 3b
   (traction) should not attribute a jump between two lookup snapshots minutes apart to that
   window.
2. `RecordProductPage`, `RecordRatings`, `RecordLookupSnapshot` are not idempotent on
   (app, storefront, source, captured_at); slice 2a dedups in the CLI layer. Making the store
   idempotent would protect every slice.
3. golangci-lint errcheck flags `internal/cli/appstore_common.go:270,339` (unchecked
   `fmt.Fprint*`) and `appstore_common_test.go:167` (`defer st.Close()`); slice-1 files, not
   edited here.

## Tests (go test, no network): 42 slice tests, all PASS
TestApp{SpotifyMergesLookupAndPage, FullAccessibilityDeclaredVsNot, UnknownIDExits3,
UsageErrorsSendNothing, DryRunAndHelp, CachedRepeatRecordsOnce, ThrottleExits7,
PageFailureKeepsLookupFields, BundleIDAndStoreURL, HumanOutput}; TestPrivacy{SpotifyLabels,
NoLabelsIsHonestEmpty, Negative}; TestVersions{Spotify25NewestFirst, LimitAndBounds,
NoHistoryIsEmptyWithNote}; TestEvents{CandyCrush, NoneIsEmptyWithNote},
TestEventStatusFollowsTheClock; TestSimilar{SpotifyInPageOrder, NoShelfAndNegative};
TestIAP{CandyCrushTenDollarRows, NoListAndUnavailableStorefront, PartialFailureAndThrottle,
NotFoundAndDogfoodCap}; TestRatings{ExactLookupCounts, HistogramAndStorefronts,
FailuresNeverBecomeRows, NotFound}; TestPrices{BatchedPerStorefront, UnavailableIsAnHonestRow,
BundleIDsAndNegative}; TestCompare{AlignedRows, StorefrontsAndUnavailable, Negative};
TestDeveloper{ByDeveloperID, FromAppReferences, PlatformFilterAndNegative};
TestS2A{CommandsFollowTheBuildChecklist, DryRunAndHelpSendNothing, HumanOutputEveryCommand,
AgentEnvelopeAndSelect}.
Covered: behaviour on content (counts, order, RFC3339 descending, histogram sum, prices "$",
aligned keys), absence (no versions/events/similar/IAP/privacy -> empty + note; unavailable
storefront -> no row / `available:false`; failed fetch -> `fetch_failures`, never a row; cached
repeat -> no duplicate snapshot), negatives (`<cmd> 1` exit 3, bad refs/flags exit 2 with zero
requests, throttle exit 7 with nothing printed and no further requests, all-failed exit 5),
dogfood cap (countries <= 2).

## Live acceptance (binary built in the worktree, `--home <wt>/.acceptance-home`)
Order: app 324684580; versions/app --full/privacy/similar/events/ratings --histogram/human app on
the cached Spotify page (0 requests); iap 553834731; events 553834731 (cached); app 361309726
--full; app 1; developer 324684583; ratings --countries us,gb; prices 324684580 553834731
--countries us,gb; compare 324684580 553834731 (cached); developer 324684580 (app id).
Assertions by `scratchpad/scripts/s2a_acceptance.py` on the saved stdout/stderr, raw:
```
PASS | app 324684580 --json: trackName starts Spotify | trackName='Spotify: Music and Podcasts'
PASS | app 324684580 --json: ratingCount > 1,000,000 | ratingCount=42719156 userRatingCount=42719156
PASS | app 324684580 --json: histogram 5 ints summing within 1%% of totalRatings | histogram=[37458149, 2996730, 950717, 373640, 942947] sum=42722183 totalRatings=42722184 diff=0.00000%
PASS | app 324684580: subtitle/chartPosition/url from page | subtitle='Songs, Playlists & Audiobooks' chartPosition={"chart": "top-free", "genreId": "6011", "genreName": "Music", "position": 1, "platform": "iphone"} editorsChoice=False url=https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580
PASS | versions 324684580 --limit 25: 25 rows, RFC3339, descending | rows=25 first=9.1.94 2026-10-23T17:59:53Z last=9.1.40 2026-04-21T13:36:42Z descending=True
PASS | app 324684580 --full: accessibility.declared = false | {"declared": false, "features": [], "summary": "The developer has not yet indicated which accessibility features this app supports."}
PASS | app 324684580 --full: versions/privacy/similar/screenshots present | versions=25 privacyTypes=2 similar=10 screenshotSets=4 events=0 iap=0
PASS | app 361309726 --full: accessibility.declared = true with >= 1 feature | declared=True features=['VoiceOver', 'Voice Control', 'Larger Text', 'Dark Interface', 'Differentiate Without Color Alone', 'Sufficient Contrast', 'Reduced Motion']
PASS | iap 553834731: 10 rows with prices starting $ | rows=10 prices=['$1.99', '$0.99', '$0.99', '$0.99', '$0.99', '$2.99', '$7.99', '$5.99', '$0.99', '$1.99'] fetch_failures=[]
PASS | privacy 324684580: label types with categories | types=[('Data Used to Track You', 3), ('Data Linked to You', 12)] policyUrl=http://www.spotify.com/legal/privacy-policy/
PASS | similar 324684580: ranked lockups, positions 1..n, not the app itself | rows=10 first=912390687 'TREBEL Music - Download Songs'
PASS | events 324684580: [] with a note, or real rows | rows=0 stderr='note: no in-app events are listed on the us product page for 324684580'
PASS | events 553834731: rows with RFC3339 dates and a status | rows=[('6805077554', 'MAJOR UPDATE', '2026-10-12T07:00:00Z', '2026-11-07T23:00:00Z', 'available', 'live')]
PASS | ratings 324684580 --histogram: exact count + 5-star-first histogram | ratingCount=42719156 histogram=[37458149, 2996730, 950717, 373640, 942947] histogramTotal=42722184 fetchedAt=2026-10-23T19:28:54Z
PASS | developer 324684583: developer + apps + shelves | developer={"id": "324684583", "name": "Spotify", "url": "https://apps.apple.com/us/developer/spotify/id324684583", "storefront": "us"} appCount=4 apps=['Spotify: Music and Podcasts', 'Spotify for Creators', 'Spotify for Artists', 'Spotify Kids'] shelves=[('Latest Release', '', 1), ('iPad & iPhone', 'iphone', 4), ('Apple TV', 'appletv', 1), ('Apple Watch', 'watch', 1)] pageOnly=[]
PASS | developer 324684580 (an app id) resolves to developer 324684583 | {"id": "324684583", "name": "Spotify", "url": "https://apps.apple.com/us/developer/spotify/id324684583", "storefront": "us"}
PASS | ratings --countries us,gb: one row per storefront | rows=[('us', 42719156, 4.77084), ('gb', 7033822, 4.73691)] requests=1
PASS | prices 324684580 553834731 --countries us,gb: 4 rows, USD/GBP | rows=[('us', '324684580', True, 'Free', 'USD'), ('us', '553834731', True, 'Free', 'USD'), ('gb', '324684580', True, 'Free', 'GBP'), ('gb', '553834731', True, 'Free', 'GBP')] requests=2
PASS | compare 324684580 553834731: aligned rows in argument order | rows=[('324684580', 'Spotify: Music and Podcasts', 42722184, 4.77081, '9.1.94'), ('553834731', 'Candy Crush Saga', 4030874, 4.70418, '1.339.0.2')] requests=0
SUMMARY 19/19 passed
```
Negative, raw: `./app-store-pp-cli app 1 --json` -> exit 3, stdout empty, stderr
`Error: no App Store app 1 in storefront us (check the id, or try another --country)`.

iap items (first 3, raw): `[{"storefront": "us", "position": 1, "name": "XS Gold Bars Pack", "price": "$1.99", "currency": "USD"}, {"storefront": "us", "position": 2, "name": "Extra Moves", "price": "$0.99", "currency": "USD"}, {"storefront": "us", "position": 3, "name": "Extra Lives", "price": "$0.99", "currency": "USD"}]`
versions (first 3, notes omitted): `[{"version": "9.1.94", "releasedAt": "2026-10-23T17:59:53Z"}, {"version": "9.1.92", "releasedAt": "2026-10-19T13:57:56Z"}, {"version": "9.1.90", "releasedAt": "2026-10-08T07:37:17Z"}]`

Capture log of the acceptance store (every network request; cache hits are not requests):
```
1 | 2026-10-23T19:28:54Z | 200 | ok | 1 | https://itunes.apple.com/lookup?country=us&id=324684580 | max-age=39985
2 | 2026-10-23T19:28:55Z | 200 | ok | 1 | https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580 | public, max-age=900
3 | 2026-10-23T19:29:21Z | 200 | ok | 1 | https://itunes.apple.com/lookup?country=us&id=553834731 | max-age=81068
4 | 2026-10-23T19:29:22Z | 200 | ok | 1 | https://apps.apple.com/us/app/candy-crush-saga/id553834731 | public, max-age=900
5 | 2026-10-23T19:29:23Z | 200 | ok | 1 | https://itunes.apple.com/lookup?country=us&id=361309726 | max-age=24380
6 | 2026-10-23T19:29:24Z | 200 | ok | 1 | https://apps.apple.com/us/app/pages-create-documents/id361309726 | public, max-age=900
7 | 2026-10-23T19:29:25Z | 200 | verified-empty | 0 | https://itunes.apple.com/lookup?country=us&id=1 | max-age=75610
8 | 2026-10-23T19:29:35Z | 200 | ok | 5 | https://itunes.apple.com/lookup?country=us&entity=software&id=324684583&limit=200 | max-age=41772
9 | 2026-10-23T19:29:36Z | 200 | ok | 1 | https://apps.apple.com/us/developer/spotify/id324684583 | public, max-age=900
10 | 2026-10-23T19:29:36Z | 200 | ok | 1 | https://itunes.apple.com/lookup?country=gb&id=324684580 | max-age=86400
11 | 2026-10-23T19:29:37Z | 200 | ok | 2 | https://itunes.apple.com/lookup?country=us&id=324684580%2C553834731 | max-age=86400
12 | 2026-10-23T19:29:38Z | 200 | ok | 2 | https://itunes.apple.com/lookup?country=gb&id=324684580%2C553834731 | max-age=86400
13 | 2026-10-23T19:29:38Z | 200 | ok | 1 | https://itunes.apple.com/lookup?country=us&entity=software&id=324684580&limit=200 | max-age=86400
```
Store after the runs: app_snapshots 7 (3 page+lookup, 4 lookup-only from prices), version_entries
25 each for 324684580/361309726/553834731, iap_prices 10 (553834731 us) + 2 (361309726 us),
app_events 1 (6805077554), privacy_snapshots 1 per app; seven cached Spotify reruns added no
duplicate snapshot. Final-binary smoke after the lint pass (ratings/prices/compare from cache):
0 new requests (captures stayed at 13).

## Gate output
`go build ./...` OK; `go vet ./...` OK; `go test -count=1 ./...`: every package ok
(internal/cli 2.9 s, internal/mcp/cobratree 10.6 s, ...). gofmt clean; staticcheck and
golangci-lint report nothing on slice-2a files. Lock heartbeat
`cli-printing-press lock update --cli app-store-pp-cli --phase build-p1-progress` run three times.
