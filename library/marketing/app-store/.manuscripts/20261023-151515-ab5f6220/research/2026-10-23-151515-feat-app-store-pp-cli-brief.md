# App Store CLI Brief

Run 20261023-151515-ab5f6220 · slug `app-store` · binary `app-store-pp-cli` · 2026-10-23
Route: "the website itself" (apps.apple.com) plus Apple's keyless endpoints. No auth.
Evidence base: 6 research sweeps (libs, mcp, market, official, storefront, parity; ~1,200 tool
calls, no direct Apple traffic), an adversarial endpoint verifier (59 confirmed, 11 refuted
details, 7 unverifiable), a completeness critic, a browser capture (agent-browser, 666
requests, 0 throttles), and 34 sequential live probes from the main session (Pakistan IP,
plain curl identity). Revision 2 (18:00 PKT) folds in the verifier, the critic and the
second probe round.

## API Identity
- Domain: Apple's App Store catalogue: about 175 storefronts and six platforms (iPhone, iPad,
  Mac, Apple TV, Watch, Vision Pro). Public data: listings, charts, ratings, reviews, privacy
  labels, version history, in-app purchase prices, in-app events, editorial placements.
- Users:
  - Indie iOS developers checking their category rank, ratings and reviews per market after
    each release (they see own units in App Store Connect, nothing about competitors).
  - ASO marketers at studios and agencies running weekly keyword reports for client apps.
  - Competitive-intelligence analysts at game companies scanning a genre across
    storefronts every week (Sensor Tower seats cost $29.5k to $115k a year).
  - Investors and researchers who want per-country demand over time for apps they do not
    own, and review archives at scale. The owner's own google-play store holds 219,758
    reviews across 483 apps (about 455 per app), so review archiving is a proven habit.
  Paid tools serve these users at $12 to $600 a month (AppTweak, MobileAction, Appfigures,
  AppFollow, Appbot).
- Data profile: read-only and per storefront. Reviews arrive in lagged batches (the
  newest RSS review was ~30 h old on both probes); listings change per release; charts
  regenerate through the day (cadence unmeasured). Value compounds with history, and Apple
  publishes none.
- Not in scope: App Store Connect and real download counts for owned apps (library CLI
  `asc`), Apple Search Ads (`apple-search-ads`), Google Play (`google-play`), download or
  revenue estimates (Apple publishes none; the CLI never fabricates them), developer
  replies (exist only on the robots-disallowed /api reviews resource).

## Reachability Risk
**Medium.** Every documented endpoint and every robots-allowed page answered 200 from a
plain client, including the owner's PK storefront. Risk sits in throttling and in
unofficial surfaces that break.

| Surface | Verified behaviour (2026-10-23) |
|---|---|
| iTunes Lookup `/lookup?id=a,b,…&country=cc` | 200. A 176-id batch returned 176 results in input order in 5.7 s (372 KB). Developer lookup `id={artistId}&entity=software` returns an artist wrapper plus apps (send `limit=200`). Cache max-age varies 39,136-86,400 s: plan for data up to 24 h stale. |
| iTunes Search API `/search?media=software` | 200; documented about 20 calls/min; 403 or 429 above that. Order differs from the web store (Instagram #4 here, TikTok #4 on the web for "spotify"). |
| Legacy RSS charts `/{cc}/rss/{feed}/limit=N[/genre=G]/json` | 200; `limit=200` returns exactly 100; top-grossing exists only here (US and PK verified, PKR prices). Do not send JSONP callbacks (400). Legacy Mac feeds are empty except topfreemacapps. |
| Customer reviews RSS `/{cc}/rss/customerreviews/page=P/id=ID/sortby=S/json` | 200, 50 per page, 10 pages; page 11 is 400. `mostrecent` is a lagged batch; `mosthelpful` page 10 reaches back to 2023, so both sorts yield up to 1,000 distinct reviews per storefront. Empty-feed reports did not reproduce; guard for them anyway. |
| Chart page `apps.apple.com/{cc}/{iphone,ipad,mac}/charts/{genreId}?chart=top-free` | 200 (US, PK, Mac). One page = top-free + top-paid, each 25 lockups + 175 ids (200 ranks); top 25 identical to RSS order. Lockup `ratingCount` is display text ("1.6k"); exact counts come from Lookup. |
| App page `apps.apple.com/{cc}/app/{slug}/id{id}` | 200, cache 900 s. Wrong or missing slug answers 301 to the canonical URL; use Lookup `trackViewUrl` without `?uo=4`. |
| Developer page, see-all pages, Today page, story page | 200. See-all reviews holds 10 most-helpful reviews and no pagination. Today page: 32 stories and 70 featured app ids. Story pages list their apps but carry no publish date and are privately cached. |
| Search hints `search.itunes.apple.com/WebObjects/MZSearchHints.woa/wa/hints` + `X-Apple-Store-Front: {sfid},12` | 200 plist, 10 hints. Without the header it returns nothing (why libraries called it broken). |
| Storefront JSON proxy `apps.apple.com/api/apps/v1/...` | Plain curl: 200, then 429 "API capacity exceeded" 33 s later (no Retry-After). Browser capture: 13 calls, all 200. `probe-reachability` later: stdlib and Surf both 200, mode standard_http (0.95). Throttle cause unknown (identity, edge site, time of day). |
| RSS v2 `rss.marketingtools.apple.com` | Timed out (25 s) then Apple 504 at 31 s from this network; redundant (100 rows, no genre filter). |

### Compliance flags (owner's call at the absorb gate; precedent rule: flag, not kill)
robots.txt is a crawler directive; this CLI is user-invoked. What each surface says:
- **Allowed:** apps.apple.com HTML pages (app, charts, developer, see-all, Today, story,
  grouping, sitemaps) and root `itunes.apple.com/lookup?`.
- **Disallowed but documented by Apple for programmatic use:** `itunes.apple.com/search*`
  (Search API) and `/*/rss/*` (RSS charts and reviews): the first tier of the precedent
  order (official/affiliate API).
- **Disallowed and undocumented:** `/WebObjects/*` (search hints for `suggest`, MZStore
  search for true keyword order), `apps.apple.com/api/*` and `/v1/*` (JSON proxy: deeper
  reviews, developer replies, App Store tags, full IAP list), `*/search?*` (web search page,
  closer to on-device keyword order).
- **User agent:** undecided. The sibling google-play CLI sends a Chrome 131 UA; the
  owner's psx-market CLI chose an honest identity. Every probe here used plain curl and
  got 200.

## Top Workflows
1. **Review archive at scale:** "Pull every reachable review for these 200 apps in my
   markets, keep them, and tell me what changed." (reviews `--countries`, both sorts,
   review-digest, search)
2. **Rank tracking:** "Where does my app (and each competitor) rank today in its category,
   across my markets, and how did that move this week?" (top, movers, rank-history)
3. **Competitor watch:** "What changed in a competitor's listing: version, What's New,
   price, IAP prices, subtitle, privacy labels, screenshots, events?" (watch-listing,
   versions, iap, privacy, events, compare)
4. **Keyword ASO:** "Where do I rank for these terms, and who owns the top results?"
   (keyword-rank, keyword-history, suggest)
5. **Market scan:** "What is climbing in Games > Puzzle in Japan; who entered the top 200;
   who got featured today?" (top, movers, featured)
6. **Traction by country:** the owner's request, served by labelled proxies (below).

## Table Stakes
From the incumbent library, its forks, about 50 MCP servers and CLIs, and the paid tools:
app details (by id or bundle id), store search, charts (free, paid, grossing; genre;
platform; country), reviews (sort, paging), ratings histogram, similar apps, developer
apps, search suggestions, privacy labels, version history, in-app purchases, in-app
events, genres list, multi-country, throttling and caching, JSON output. The best keyless
tools add true-store-order search (drewster99, aso-kit) and SQLite history (aso-kit). The
most-starred App Store MCP (appreply-co, 78 stars) fakes keyword scores and IAPs: the gap
to beat honestly.

## Data Layer
- Entities: app snapshot (Lookup + page fields, content hash), chart snapshot (storefront,
  platform, genre, chart, rank, app id, source, row limit, captured_at), review (id,
  storefront, sort seen in, rating, version, title, body, author, date, votes), ratings
  snapshot (exact count from Lookup + histogram per storefront), version entry
  (append-only; 25 newest per fetch), privacy snapshot, IAP price point (top 10 per
  storefront), event (kind, start, end), featured snapshot (storefront, story id, app ids,
  capture date), keyword rank (term, storefront, platform, source, app id, rank, top-N
  list), capture log (URL, status, cache headers, entry count, oldest/newest dates, parser
  version) for provenance and tri-state results.
- Identity and size: store extracted fields plus a content hash; keep raw pages only on
  parse failure (product pages run 0.76-0.99 MB). Reviews dedupe on id across sorts.
- Sync cursor: snapshots are append-only time series keyed by capture time.
- FTS: review title+body; app name, subtitle, description.

### Request budget (per command, default scope)
| Command | Requests |
|---|---|
| app (`--full`) | 1 Lookup + 1 app page |
| top (one storefront, platform, genre) | 1 chart page + 1 batched Lookup (200 ranked apps); grossing = 1 RSS |
| reviews (one storefront) | up to 20 RSS pages (10 per sort) |
| keyword-rank (one term) | 1 Search API call |
| ratings/prices/iap `--countries N` | N Lookups or N app pages |
Pacing: per host; Search API at most 20/min; pages 1 request at a time with jitter. A
governor stops the run at the first 403/429 and exits 7; `--plan` prints the request
estimate without sending.

## Codebase Intelligence
- Storefront ids and `X-Apple-Store-Front: {sfid},{platformId}`: 175 live storefronts
  (Apple support 118205 and the live region switcher agree).
- Categories: apps chart page lists All Apps, 24 genres and Kids (4 age bands via
  `ageBandId`); games chart page lists All Games, 17 subgenres (7003 is now Casual) and
  Kids. Developer Tools 6026 and Graphics & Design 6027 are current; the incumbent's
  constants are stale.
- Page model: `{data:[{intent:{$kind,...}, data:{...}}], userTokenHash}` (older captures a
  bare array); check `intent.$kind` (ProductPageIntent, ChartsPageIntent,
  DeveloperPageIntent, SeeAllPageIntent, LandingPage_*_Intent).
- App page fields: versions = `mostRecentVersion.seeAllAction.pageData.shelves[].items`
  (25; dates are JS Date strings); privacy = `privacyHeader.seeAllAction.pageData`; IAPs =
  `information` item "In-App Purchases" (10 textPairs + a Learn More button); events =
  `appEvents` (title, kind, startDate, endDate, appEventBadgeKind); editorial =
  `featuredIn`; ratings = `productRatings.ratingCounts`; accessibility =
  `accessibilityHeader` + `accessibilityFeatures` ("Supported Features"; most third-party
  apps have not declared any); tags `description.items[0].tags` is empty on SSR (tags
  appear only on /api).
- The SPA itself calls /api with an empty `Authorization: Bearer` header; Apple's edge
  supplies the token.

## Boundaries (route to siblings)
| Request | Use |
|---|---|
| Downloads, sales, proceeds for apps you own | `asc` (App Store Connect) |
| Search Ads campaigns, bids, keyword popularity | `apple-search-ads` |
| Android / Google Play | `google-play` |

## User Vision
- From the invocation: "Apple App Store, sibling to google-play". Mirror the owner's
  published google-play CLI: command names (app, top, search-store, reviews, similar,
  suggest, developer, categories, compare, movers, rank-history, keyword-rank,
  keyword-history, watch-listing, review-digest), global flags (--country, --lang,
  --agent, --select, --data-source), bare-JSON output for hand-built commands, typed exit
  codes (7 = rate limited), SQLite snapshot schema, dogfood trimming. Play's `permissions`
  becomes honest `privacy`, `supports` and `age-rating`; no fabricated permission list.
- Owner request (2026-10-23 17:30 PKT): "downloads by country on a daily, monthly, annual
  and all time basis". Constraint: Apple publishes no download or install numbers for any
  app at any granularity (google-play's only figure is Play's global all-time
  installs/realInstalls; its SKILL refuses download estimates). Serve the intent with
  clearly labelled public proxies per storefront: all-time rating count (Lookup
  `userRatingCount` per country; resettable per version), new ratings per day from
  snapshots, daily chart rank (200 free/paid, 100 grossing), review volume; rolled up
  daily / monthly / annual / all-time from local history (no backfill before first
  capture). Real per-country downloads exist only in App Store Connect sales reports for
  apps the user owns (needs an ASC API key; the library's `asc` CLI covers ASC) — an
  explicit option at the absorb gate, not a default.

## Product Thesis
- Name: App Store (display), `app-store-pp-cli`.
- Why it should exist: the free tools are broken or stale (the incumbent npm package's
  last release is from 2023-11 and its privacy, versionHistory, similar and reviews calls
  fail upstream), the MCPs fake scores, and the paid tools charge monthly for history
  that public pages can supply. This CLI reads Apple's documented APIs and robots-allowed
  pages, keeps every capture in a local SQLite history with provenance, and answers review,
  rank, listing-change and traction questions across storefronts with no key, no daemon
  and no account. Charts go to 200 ranks per chart from the robots-allowed web page,
  against 100 in every RSS-based tool (parse-tunes reaches about 200 only through an
  internal endpoint), and it adds App Store-only data (privacy labels, version history,
  IAP prices, in-app events, Editor's Choice, accessibility).

## Build Priorities
1. One shared multi-host client: per-host pacing, honour cache headers, governor (first
   403/429 stops the run, exit 7), tri-state results (ok / verified-empty / failed),
   capture log. Page-model parser with `$kind` checks and fixture tests.
2. Live surfaces: reviews (both sorts, `--countries`), app, top (200 ranks + grossing),
   search-store, similar, developer, categories, privacy, versions, iap, events, ratings,
   prices, featured, compare, suggest (pending the compliance decision).
3. Snapshot store and novel commands: movers, rank-history, watch-listing, keyword-rank,
   keyword-history, review-digest, traction, impact, changes, keyword-share, breakouts.
4. Agent-native polish: google-play output parity, `--select` recipes, doctor pointed at
   Lookup, dogfood budget trims, no proxy calls by default.

## Verification
- verify-endpoints: 59 confirmed, 11 refuted (all detail-level; corrections applied above:
  Lookup cache TTL, Mac feed content, JSONP, popIds, see-all paging, chart Media API
  params, sitemap shard counts, reviews feed alive, 200-rank charts via the web page, 429
  timing), 7 unverifiable (none build-critical).
- Critic: gaps applied above (review lag, budget model, UA decision, version cap 25, IAP
  count, categories, boundaries, priorities). Probe round 2 settled hydration, PK, Mac,
  tags, events, accessibility, Today and story pages.
- verify-features: pending at revision 2.
