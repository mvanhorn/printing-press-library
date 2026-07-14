# TikTok Creative Center CLI Brief

## API Identity
- **Domain:** TikTok Creative Center — `ads.tiktok.com/business/creativecenter`. A free,
  public-facing trend discovery + ad-inspiration product for marketers and creators.
- **Users:** TikTok marketers, content strategists, niche researchers, competitor analysts,
  faceless/short-form creators sourcing trends. Heavily used by AI agents doing niche + competitor research.
- **Data profile:** Trend aggregations (hashtags, sounds/songs, creators, products, topics,
  keywords) ranked by popularity with growth metrics; plus a Top Ads (ad library) surface with
  creative metadata and performance bands (impressions, CTR, industry, region, objective, duration).
  Read-only, list/detail oriented, region+time+industry parameterized, paginated.

## Reachability Risk
- **Low.** `probe-reachability` on the Creative Center HTML page returned `mode: standard_http`,
  HTTP 200 via both stdlib and surf-chrome (confidence 0.95). No Cloudflare/WAF wall on the page.
- The legacy XHR host `tccapi.tiktokv.com` no longer resolves (DNS no-such-host) — the BFF has
  moved. The live XHR host + paths will be confirmed by authenticated browser-sniff capture
  (historically an API like `/creative_radar_api/...` under `ads.tiktok.com`). Expect region/locale
  cookies and a device `User-Agent`; most endpoints are browsable without login, Top Ads filters
  and saved lists benefit from a logged-in session (user confirmed they will log in).

## Top Workflows
1. **Niche discovery:** "What's trending in <niche> in <region> this week?" → trending hashtags
   with popularity + growth, plus the rising sounds, creators, and products in that niche.
2. **Hashtag/keyword trend analytics:** popularity curve + growth % for a specific hashtag or
   keyword over a timeframe, with related/associated tags.
3. **Competitor / ad spying:** search the Top Ads library by brand, keyword, or industry → see
   competitors' live and recent ads with creative, format, objective, region, and performance bands.
4. **Creator sourcing:** trending creators in an industry/region for partnership or content modeling.
5. **Product trend tracking:** trending products for commerce/affiliate niche selection.

## Table Stakes (every competing tool has these)
- Trending hashtags list (region/period/industry filters)
- Trending songs/sounds list
- Trending creators list
- Trending products list
- Top Ads / ad library search (keyword, industry, region, format, objective, date)
- Ad detail (creative, metrics, run dates)
- Keyword/hashtag popularity-over-time trend
- JSON + paginated output

## Data Layer
- **Primary entities:** `hashtags`, `songs`, `creators`, `products`, `top_ads`, `trends`
  (keyword/hashtag time series), plus `regions`/`industries` reference data.
- **Sync cursor:** region + period + entity type; pages are integer-offset paginated.
- **FTS/search:** full-text over hashtag names, song titles, creator handles, ad keywords;
  enables offline "find trending <x> mentioning <y>".

## Competitors / Tools To Absorb
- Apify "TikTok Trending Hashtags Analytics" actor — hashtag analytics HTTP API
- davidteather/TikTok-Api (Python) — hashtag/trend/user endpoints
- ensembledata, socialcrawl.dev, crawlora, Sociavault — paid TikTok scrapers (trending + top-ads)
- TikTok official Marketing API (ads only; separate auth, paid/approved)
- Generic ad-spy tools (BigSpy, PiPiADS, Minea) — TikTok Top Ads search

## Product Thesis
- **Name:** `tiktok-creative-center-pp-cli` (binary `ttcc-pp-cli`)
- **Why it should exist:** The Creative Center is a goldmine for niche + competitor research but is
  only exposed as a click-heavy web app with no official API. An agent-native CLI that replays its
  real XHR endpoints, syncs trends into a local SQLite store, and adds cross-entity "niche report",
  "trend velocity", and "competitor ad sweep" commands — none of which exist in any current tool —
  lets an AI agent do in one command what takes a human 20 minutes of clicking.

## Build Priorities
1. Data layer + sync for all six primary entities (foundation for everything transcendent)
2. Absorbed endpoints: trending hashtags/songs/creators/products, keyword trends, Top Ads search + detail
3. Transcendence: `niche` (cross-entity report), `velocity` (growth across snapshots), `competitor`
   (brand ad sweep), `watch`/`since` (snapshot diff + new-since), `similar` (related tags/songs)
4. Polish: region/industry/period flag defaults, --json/--select/--compact everywhere, doctor check
