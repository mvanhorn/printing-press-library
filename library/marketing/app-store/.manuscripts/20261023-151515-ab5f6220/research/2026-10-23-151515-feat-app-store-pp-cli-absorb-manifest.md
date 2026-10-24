# App Store CLI Absorb Manifest

Run 20261023-151515-ab5f6220 · 2026-10-23 · first print (no prior research.json)
Brief: `2026-10-23-151515-feat-app-store-pp-cli-brief.md` (revision 2).
Novel-features audit trail: `2026-10-23-151515-novel-features-brainstorm.md`.

## Tools surveyed
Libraries: facundoolano/app-store-scraper (npm 0.18.0, 1,436 stars) and forks (fti,
screensstudio, VoldemLive, trazyn, JiantaoFu, netsocks), @perttu/app-store-scraper
(plahteenlahti), app-store-scraper-bun, app-store-scraper-ts, app-store-client,
parse-tunes, PyPI app-store-scraper (archived), app-store-web-scraper,
itunes-app-scraper-dmi, app-reviews, ipatool, Go ports, RSSHub App Store routes,
RSS-Bridge. MCP servers and CLIs: appreply-co/mcp-appstore, drewster99/appstore-mcp-server,
JiantaoFu/AppInsightMCP, MiguelAlvRed, onmyway133, KenanAtmaca/aso-mcp, hristo2612/aso-cli
(aso-kit), semihcihan/App-Store-Optimization-CLI, respectlytics/respectaso, AsoBeast,
about 40 smaller servers and skills. Paid tools: AppTweak, Sensor Tower, Appfigures,
AppFollow, MobileAction, Appbot, AppMagic. Sibling: the owner's google-play CLI.
No official Anthropic plugin exists for the App Store. DeepWiki returned 429 (skipped).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | App details by track id | facundoolano/app-store-scraper `app()`; iTunes Lookup | app-store-pp-cli app | Lookup (44 fields) merged with the app page model: subtitle, Editor's Choice, histogram, chart position, IAP flag, privacy summary |
| 2 | App details by bundle id | app-store-scraper `app({appId})`; Lookup `bundleId=` | (behavior in app-store-pp-cli app) accepts `com.x.y` and resolves through Lookup | One command for both id forms |
| 3 | Batch lookup of many apps | iTunes Lookup comma ids | (behavior in app-store-pp-cli compare) one batched Lookup per storefront (176 ids verified in one call) | One request per country, not one per app |
| 4 | Store search by keyword | app-store-scraper `search()`; iTunes Search API | app-store-pp-cli search-store | `--platform`, `--price free/paid`, source label on every row |
| 5 | Top charts (free, paid) by genre | app-store-scraper `list()` (100 cap, #206); parse-tunes `fetchTopApps` | app-store-pp-cli top | 200 ranks from the robots-allowed chart page (25 lockups + 175 ids hydrated by one Lookup); snapshot recorded |
| 6 | Top-grossing chart | app-store-scraper `list()`; legacy RSS | (behavior in app-store-pp-cli top) `--chart top-grossing` via legacy RSS (100 rows, labelled) | Only free grossing source; cap stated |
| 7 | Charts per platform (iPhone, iPad, Mac) | app-store-scraper collections (Mac RSS empty) | (behavior in app-store-pp-cli top) `--platform iphone/ipad/mac` | Mac charts work on the web page (200 ranks) where Mac RSS is empty |
| 8 | Kids age-band charts | apps.apple.com charts `ageBandId` links | (behavior in app-store-pp-cli top) `--age-band` | Not in any library |
| 9 | Reviews, most recent / most helpful | app-store-scraper `reviews()`; futurice; cowboy-bebug (archived) | app-store-pp-cli reviews | Both sorts merged (up to 1,000 distinct per storefront), `--countries` fan-out, dedupe on id, empty-feed guard, stored with FTS |
| 10 | Ratings histogram | app-store-scraper `ratings()`; @perttu ratings | app-store-pp-cli ratings | Exact counts from Lookup plus the page histogram, per storefront with `--countries`, stored |
| 11 | Similar apps ("You Might Also Like") | app-store-scraper `similar()` (broken since 2024, #203) | app-store-pp-cli similar | Works again from the page (10 apps), rank order kept |
| 12 | Developer's apps | app-store-scraper `developer()` (no limit sent) | app-store-pp-cli developer | Lookup with `limit=200` plus per-platform shelves from the developer page |
| 13 | Search suggestions / autocomplete | app-store-scraper `suggest()` (broken #199); @perttu (fixed 2026-08); wacumov | app-store-pp-cli suggest | Storefront header that makes hints work; robots-flagged surface (gate decision) |
| 14 | Privacy nutrition labels | app-store-scraper `privacy()` (broken #228); fti fork proxy | app-store-pp-cli privacy | From the robots-allowed page, no proxy, no token |
| 15 | Version history | app-store-scraper `versionHistory()` (broken #223); RSSHub update route; RSS-Bridge | app-store-pp-cli versions | 25 newest per fetch (the cap on every surface), stored append-only so history grows |
| 16 | In-app purchase prices | @perttu `inAppPurchases`; RSSHub `/appstore/iap`; xiliourt scripts | app-store-pp-cli iap | Top 10 name + price per storefront; `--countries` for regional pricing |
| 17 | In-app events | trazyn fork `metadata()` | app-store-pp-cli events | Title, kind, start/end, live badge from the page `appEvents` shelf |
| 18 | Genre / category list | app-store-scraper constants (stale) | app-store-pp-cli categories | Live from the apps and games chart pages (incl. 6026, 6027, Kids bands) with a static fallback |
| 19 | Price and availability across storefronts | RSSHub price route; AppleCensorship | app-store-pp-cli prices | One Lookup per storefront; missing = unavailable, recorded |
| 20 | Screenshots per device | app-store-scraper screenshots; guiguito fallback chain | (behavior in app-store-pp-cli app) `--full` per-device media | iPhone, iPad, TV, Watch, Vision sets |
| 21 | Accessibility labels | app page `accessibilityHeader`/`accessibilityFeatures` (no library) | (behavior in app-store-pp-cli app) `--full` accessibility block | Declared features, or "not declared" stated plainly |
| 22 | Age rating and Supports capabilities | app page `information` / `capabilities` | (behavior in app-store-pp-cli app) `--full` | Honest replacement for Play's permissions |
| 23 | Editor's Choice and Featured In | app page lockup / `featuredIn` | (behavior in app-store-pp-cli app) and `top` row flags | No extra request |
| 24 | Compare apps side by side | google-play `compare`; appreply-co competitor tools | app-store-pp-cli compare | One batched Lookup per storefront, aligned table or JSON |
| 25 | Chart movers between snapshots | google-play `movers`; AppTweak Top Charts Trends (paid) | app-store-pp-cli movers | Free, local, per storefront/platform/genre |
| 26 | Rank history for one app | google-play `rank-history`; Appfigures/Sensor Tower (paid) | app-store-pp-cli rank-history | Unlimited local history |
| 27 | Keyword rank capture | google-play `keyword-rank`; drewster99; aso-kit; semihcihan ASO CLI | app-store-pp-cli keyword-rank | Source-labelled rank (official Search API by default), top-N list kept |
| 28 | Keyword rank history | google-play `keyword-history`; MobileAction/AppTweak (paid) | app-store-pp-cli keyword-history | Local time series per term/app/storefront/source |
| 29 | Listing change detection | google-play `watch-listing`; Sensor Tower Update Timeline; ASO.dev changelog | app-store-pp-cli watch-listing | Diffs title, subtitle, version, price, IAPs, privacy, screenshots, events |
| 30 | Review analytics digest | google-play `review-digest`; Appbot (paid) | app-store-pp-cli review-digest | Star/version histograms, complaint-term counts, no LLM |
| 31 | Caching and throttling | app-store-scraper `memoized()`/throttle; rjyo token bucket | (behavior in app-store-pp-cli app) shared client: per-host pacing, cache headers honoured, governor stops at first 403/429 (exit 7), `--plan` estimate | Tri-state results and a capture log |
| 32 | Offline full-text search | framework | app-store-pp-cli search | FTS over stored reviews and listings |
| 33 | Featured / editorial tracking | Appfigures Featured report; Sensor Tower Featured Today (paid) | app-store-pp-cli featured | Today stories and featured apps per storefront from the robots-allowed page, snapshot-recorded (capture date = featuring date) |
| 34 | Keyword difficulty signals | aso-kit difficulty model; drewster99; RespectASO | (behavior in app-store-pp-cli keyword-rank) `--serp` transparent signals: result count, top-N rating counts, title-match share | Inputs shown, no invented popularity |
| 35 | SERP snapshot history | AppTweak Keyword Live Search History; AsoBeast | (behavior in app-store-pp-cli keyword-history) `--serp` replays stored top-N per capture | Local, unlimited |
| 36 | Agent / MCP access | Sensor Tower MCP; AppTweak MCP (paid); appreply-co; drewster99 | (behavior in app-store-pp-cli agent-context) every command mirrored as an MCP tool by app-store-pp-mcp; `--agent` JSON | Honest data, read-only annotations |

No stubs.

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Traction by storefront (labelled public demand proxies) | traction <app>... --countries us,jp --period monthly | 10/10 | hand-code | Uses stored `ratings_snapshot` (exact Lookup counts per storefront), `chart_snapshot` and `review` rows to compute per storefront new ratings (version resets detected, never negative), rating count since last reset, best rank, days charted and captured review volume, rolled up daily/monthly/annual/all-time | Owner request 2026-10-23 17:30 PKT; Apple publishes no download numbers; Sensor Tower $29.5k-$115k/yr; google-play SKILL refuses download estimates | Use this when asked for downloads, installs or demand by country over time: it returns labelled public proxies per storefront (new ratings, chart rank, captured review volume) rolled up daily, monthly, annual or all-time, never download counts. Do NOT use this for the current star histogram in each storefront; use 'ratings' instead, and for one chart's rank trajectory use 'rank-history'. Real per-country downloads exist only in App Store Connect sales reports for apps you own, which the separate asc CLI reads. |
| 2 | Release and change impact | impact <app> --at latest --window 7d | 9/10 | hand-code | Takes an event (version release date, listing content-hash change, featuring, or a date) and compares equal windows before and after it per storefront: best rank, new ratings per day, average stars of reviews dated in the window, tracked keyword positions; reports co-movement only, `insufficient_data` when a window has fewer than two captures | Brief workflows 1-3; Sensor Tower Update Timeline (paid); google-play keyword-history exists to prove whether a listing change moved a keyword | Use this to measure how rank, new ratings, review stars and tracked keyword positions moved in equal windows before and after a release, a listing change, an editorial placement or a given date. Do NOT use this to see what changed; use 'watch-listing' for one app's field diff or 'changes' for a feed across apps, and for complaint terms by version use 'review-digest'. |
| 3 | Cross-app change feed | changes --since 7d --apps <id,...> | 9/10 | hand-code | Unions diffs between consecutive stored snapshots (listing fields, versions, IAP prices, privacy labels, events, Editor's Choice/featuring) across every tracked app into one dated, typed feed | Brief workflow 3; Sensor Tower Update Timeline (paid); incumbent versionHistory/privacy broken (#223, #228); app page cached 900 s vs Lookup 24 h | Use this for a dated feed of every change across your tracked apps inside a window: new versions and their notes, title and subtitle edits, price and in-app purchase price changes, privacy-label changes, new in-app events, and featuring or Editor's Choice gained or lost. Do NOT use this for a field-by-field diff of one app's two latest snapshots; use 'watch-listing' instead, and to measure whether a change moved rank or ratings use 'impact'. |
| 4 | Keyword share of voice | keyword-share [term...] --country us --top 10 --app <id> | 9/10 | hand-code | Aggregates stored keyword-rank top-N lists across tracked terms: per app, terms held in the top N, average and best position, change since the previous capture; `--app` marks the target and lists terms competitors hold that it lacks | Brief workflow 4; AppTweak/MobileAction $12-$600/mo; appreply-co fakes scores; aso-kit keeps SQLite history | Use this to see which apps hold the top search results across a set of tracked terms and how that share moved since the previous capture. Do NOT use this for one app's position on one term over time; use 'keyword-history' instead, and to capture a fresh result list use 'keyword-rank'. |
| 5 | Multi-storefront breakouts | breakouts --genre 6014 --chart top-free --since 7d --min-markets 3 | 7/10 | hand-code | Joins stored chart snapshots for one platform/genre/chart across storefronts: apps that entered or gained at least --min-gain ranks in at least --min-markets storefronts, ranked by market count then median gain; meta lists compared and skipped storefronts | Brief workflow 5; one chart page = 200 ranks per storefront (probe ledger); AppTweak movers (paid) | Use this to find apps climbing or entering one chart in several storefronts at once over a window. Do NOT use this for one storefront's chart diff; use 'movers' instead, and for one app's rank trajectory use 'rank-history'. |

Killed candidates (11) with reasons are in the brainstorm file: gainers, markets,
keyword-gap, events-calendar, rollout, privacy-scan, keyword-ideas, capture,
feature-lift, estimate-downloads (would invent downloads), asc-downloads (needs an App
Store Connect key; owner may opt in at the gate).

## Gate decisions (owner, Phase Gate 1.5, 2026-10-23 ~18:05 PKT)
- Manifest: **approved, generate now** (36 absorbed + 5 transcendence = 41 features).
- User agent: **Chrome browser UA**, matching the sibling google-play CLI.
- Robots-disallowed undocumented surfaces: **all, as explicit opt-ins**: `suggest`
  (search hints), `keyword-rank --source web` (web search page order), and
  `--source api` on the relevant commands (deeper recent reviews, developer replies,
  App Store tags, full IAP list). Never used by default, by `sync`, or by bulk capture.
- App Store Connect downloads: **left to the `asc` CLI**; this CLI stays keyless.
- Crowd-sniff spec: not merged (91 unrelated endpoints); no override requested.
