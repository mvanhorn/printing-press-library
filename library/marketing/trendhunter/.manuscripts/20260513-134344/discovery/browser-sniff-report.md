# TrendHunter Discovery Report

## Method

Agent-driven HTTP discovery via stdlib Go with a realistic Chrome User-Agent and
Accept header. No live browser capture was needed - `probe-reachability` returned
`mode: standard_http` with 0.95 confidence, and direct HTTP returned 200 for every
endpoint we needed.

The original `BROWSER_SNIFF_TARGET_URL=https://www.trendhunter.com/` was set in
Phase 0 ("website itself" choice), pre-approving any browser-sniff that proved
necessary. None was.

## Reachability

| Probe                                          | Result                                  |
| ---------------------------------------------- | --------------------------------------- |
| `curl` with default UA                         | 403 (UA filtering)                      |
| `curl` with Chrome UA + `Accept: */*`          | 403 (Accept header filtering)           |
| stdlib Go with Chrome UA + full Accept header  | 200 across all probed endpoints         |
| `printing-press probe-reachability`            | `standard_http`, confidence 0.95        |

No Cloudflare/Vercel/AWS-WAF/DataDome challenge artifacts. The site's filter is a
straightforward UA + Accept-header check. Any HTTP client that imitates Chrome's
default request header set passes.

## Endpoints

All return HTTP 200 with a Chrome-imitating header set:

- `GET /rss` - RSS 2.0, 30 latest trends across the whole site (~6.5KB).
- `GET /sitemap.xml` - 430+ URLs - categories, futurists, contributors, top-list pages (~59KB).
- `GET /results?search=<term>` - HTML, ~20-30 trend cards per query.
- `GET /trends/<slug>` - Full trend detail HTML (~150KB). Rich extractable fields.
- `GET /<category>` - Category index, ~30 trend cards (~70KB).
- `GET /popular` - Popular-right-now landing (~44KB, ~30 trend cards).
- `GET /scoreboard` - Top contributors + trends (~83KB).
- `GET /megatrends` - List of pattern/megatrend nav (~34KB).
- `GET /trendreports` - Index of premium reports (~72KB).
- `GET /futurist/<name>` - Futurist profile.
- `GET /<username>` - Contributor profile.

## What an /rss item looks like

```xml
<item>
  <title><![CDATA[Hydration-Based Grant Initiatives - American Water Charitable Foundation Launches a Public Charity (TrendHunter.com)]]></title>
  <link>http://www.trendhunter.com/innovation/american-water-charitable-foundation</link>
  <description><![CDATA[<a href='...'><img src='...' /></a> (<a href='...'>TrendHunter.com</a>) The American Water...]]></description>
  <pubDate>Wed, 13 May 2026 16:43 GMT</pubDate>
</item>
```

30 items per fetch.

## What a /trends/<slug> page contains

- `<h1 class='tha__title1'>` - human title (e.g. "Executive AI Venting Systems").
- `<meta name='description'>` and `<meta property='og:description'>` - long summary.
- `<meta name='keywords'>` - tag list.
- `<meta property='og:image'>` - high-res image URL.
- `<a href='/<category>'>...</a>` breadcrumb - primary category.
- "by <Name>" text node - author display name.
- Trend ID embedded in HTML attributes (e.g. `613400`).
- `<a href='/trends/<related-slug>'>` - 6-10 related trend slugs.
- `<script type='application/ld+json'>` with `@type:FAQPage` and 4-8 Q&A pairs - excellent agent-summary content.

## Important notes

- Per-category RSS does NOT work - `/rss/tech` returns a valid but empty
  `<channel>`. Only the global `/rss` has items.
- Some trend pages redirect to vertical microsites (cleanthesky.com,
  treehugger.com, etc.). The canonical trendhunter.com URL still resolves, so
  emit the canonical link.
- Search is GET-only: `/results?search=<term>`. There is no JSON search API.
- No login or paid surface needed for the public CLI. Pro/megatrend listing
  is open; only the full PDF download is gated.
- No client-rendered JavaScript for any of the data we extract - everything
  is server-rendered HTML or RSS.

## Replayability

The printed CLI will use stdlib HTTP (Go `net/http`) with a Chrome User-Agent
and the standard Accept header. No browser sidecar. No clearance cookie. No
WebDriver. No browser-compatible Surf transport needed.

## Rate limiting observations

No throttling observed in probing (~30 sequential requests over ~2 minutes). The
CLI will still ship `cliutil.AdaptiveLimiter` with a conservative default of
2 req/s and `--rate-limit` flag, with typed `*cliutil.RateLimitError` on 429
so cross-source fan-out commands don't silently drop sources.
