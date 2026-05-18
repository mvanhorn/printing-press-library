# Trustpilot Browser-Sniff Report

## Tooling and consent
- Backend: agent-browser 0.23.4 (Chromium via /Applications/Google Chrome.app)
- Consent: user approved authenticated browser-sniff via Phase 1.7 AskUserQuestion. No login — anonymous browsing only.
- Capture target: `https://www.trustpilot.com/review/thriftbooks.com` (Trustpilot canonicalized to `www.thriftbooks.com`)

## Reachability classification
- `probe-reachability` mode: `browser_clearance_http` (stdlib HTTP and Surf-Chrome both 403 with AWS WAF marker).
- After browser-sniff verification: **clearance-cookie replay sufficient.** Runtime is `browser_clearance_http` per the recommendation but with confirmed cookie-only HTTP replay working — no live page-context execution required.
- Generator can ship Chrome-cookie capture + Surf/standard `net/http` transport for replay.

## Confirmed endpoints (replayable via cookie + `x-nextjs-data: 1` header)

### 1. Review page HTML (cookie-harvest + page-1 reviews)
```
GET https://www.trustpilot.com/review/<domain>
```
Returns server-rendered HTML containing `<script id="__NEXT_DATA__">` with the full first-page payload and `buildId`. Used once per session to harvest the `aws-waf-token` cookie and the review-pages `buildId`.

### 2. Paginated reviews via JSON API
```
GET https://www.trustpilot.com/_next/data/<reviews-buildId>/review/<domain>.json
    ?businessUnit=<domain>
    &page=<N>
    &stars=<1..5>
    &languages=<en|de|fr|...>
    &sort=<recency|relevance>
    &date=<last30days|last3months|last6months|last12months>
    &verified=<true|false>
Headers:
  Cookie: aws-waf-token=...; (full jar best)
  x-nextjs-data: 1     ← REQUIRED, else 308-redirects to HTML
  User-Agent: <Chrome-like>
  Referer: https://www.trustpilot.com/review/<domain>
```
Returns the same `pageProps` payload as the HTML page's `__NEXT_DATA__` — but as pure JSON. Tested and confirmed:
- page=2 returned 20 fresh reviews (no overlap with page 1 except 1 timing race)
- stars=1 filter returned all 1-star reviews; pagination claims 71,060 1-star reviews for ThriftBooks
- stars=5 + languages=en returned 5-star English only (2,444,519 results)
- sort=recency returned reviews from minutes ago

`buildId` for review pages observed in capture: `businessunitprofile-consumersite-2.6787.0`.

### 3. Company search via JSON API
```
GET https://www.trustpilot.com/_next/data/<search-buildId>/search.json?query=<name>
Headers: same as above
```
Returns `pageProps.businessUnits[]` — list of matched companies with `identifyingName` (the canonical domain key for review lookups), `displayName`, `trustScore`, `numberOfReviews`, `stars`, `logoUrl`, `categories`.

`buildId` for search/categories observed in capture: `categoriespages-consumersite-2.1231.0`.

NOTE: review pages and search pages live in different Next.js apps with different `buildId`s. The CLI needs to harvest BOTH at session start (one HTML page load + one HTML search-page fetch) or fall back to HTML scraping when the buildId is unknown.

## __NEXT_DATA__ shape (verified)

```
{
  "buildId": "<string>",
  "props": {
    "pageProps": {
      "businessUnit": {
        "id": "491795f0000064000503e008",
        "displayName": "ThriftBooks",
        "identifyingName": "www.thriftbooks.com",
        "numberOfReviews": 2780622,
        "trustScore": 4.7,
        "stars": 4.5,
        "websiteUrl": "https://www.thriftbooks.com",
        "categories": [...],
        "isClaimed": true,
        "isCollectingReviews": true,
        "verification": {...},
        "activity": {...},     ← claimed/replied/etc stats
        "contactInfo": {...}
      },
      "reviews": [               ← 20 reviews per page
        {
          "id": "6a0719e97a6499ab7cb81370",
          "filtered": false,
          "isPending": false,
          "rating": 5,
          "title": "...",
          "text": "...",
          "language": "en",
          "likes": 0,
          "labels": {"merged": ..., "verification": ...},
          "dates": {
            "experiencedDate": "2026-05-15T...",
            "publishedDate":   "2026-05-15T...",
            "updatedDate":     null,
            "submittedDate":   "2026-05-15T..."
          },
          "consumer": {
            "id":               "...",
            "displayName":      "...",
            "imageUrl":         "...",
            "numberOfReviews":  N,
            "countryCode":      "US",
            "hasImage":         true|false
          },
          "reply": null | {                      ← business reply
            "publishedDate": "...",
            "updatedDate":   "...",
            "message":       "..."
          },
          "consumersReviewCountOnSameDomain": N,
          "productReviews": [],
          "source": "AFSv2",                     ← review source enum
          "report":  null
        },
        ...
      ],
      "relevantReviews":         [...],          ← 4 curated featured reviews
      "aiSummaryReviews":        [...],          ← 4 reviews used in Trustpilot's AI summary
      "topicAiSummaryReviews":   [...],          ← 5 grouped by AI-detected topic
      "aiSummary": {
        "summary":      "<paragraph>",           ← Trustpilot's own AI summary
        "modelVersion": "2.1.0"
      },
      "topicAiSummaries":        [...],          ← 5 topic-specific AI summaries
      "filters": {
        "hasActiveFilters":              bool,
        "totalNumberOfReviews":          N,
        "totalNumberOfFilteredReviews":  N,
        "pagination": {
          "currentPage": 2,
          "perPage":     20,
          "totalCount":  1509276,
          "totalPages":  75464
        },
        "selected": {                            ← echoes back applied filters
          "languages": [...],
          "date":      "...",
          "stars":     [...],
          "topics":    [...],
          "search":    "...",
          "sort":      "...",
          "verified":  bool,
          "replies":   "..."
        },
        "reviewStatistics": {
          "hasMultipleLanguages": bool,
          "reviewLanguages":      [...],
          "ratings":              {1: N, 2: N, ...}
        }
      },
      "similarBusinessUnits":    [...]           ← 8 similar companies (great for compare)
    }
  }
}
```

## Cookies (only `aws-waf-token` is functionally required)
Observed cookie jar after page load:
```
OptanonConsent=...; TP.uuid=...; ajs_anonymous_id=...;
amplitude_id*=...; _gcl_au=...; _ga*=...;
aws-waf-token=<REDACTED>
```
`aws-waf-token` value redacted at write time per Phase 5.6 secret-protection rules. The token is the only cookie used by WAF to authorize subsequent JSON-API calls. Lifespan per AWS WAF docs: 5-15 minutes.

## Pagination limits and the segmenting workaround
- `filters.pagination.totalPages` for an unfiltered company can be enormous (ThriftBooks: 75,464 pages = 1.5M reviews).
- Multiple existing scrapers report a soft cutoff around page 200 (4,000 reviews).
- Workaround proven by Apify and others: segment by `stars=1..5` × `languages=...` to keep each filtered set under the cutoff. The CLI's `sync` command should expose `--bust-cutoff` (default true) that loops through star slices.

## Bot protection
- AWS WAF (confirmed via probe-reachability "AWS WAF marker" evidence).
- JS challenge issues `aws-waf-token` cookie.
- TLS fingerprinting (JA3/JA4) — observed: Surf with Chrome TLS still 403 (the WAF requires the JS challenge to issue the cookie, not just TLS).
- Working bypass: one-shot Chrome via agent-browser/chromedp to harvest cookie → reuse cookie in plain HTTP for 5-15 minutes.
- No proxy rotation required at low rates (single-digit pages/minute).

## Generated CLI runtime contract
- **Auth type**: `cookie`
- **Auth login flow**: `trustpilot-pp-cli auth login --chrome` launches Chrome via agent-browser/chromedp, navigates to a Trustpilot home/review page, harvests `aws-waf-token` + both `buildId`s, persists to local config.
- **Refresh policy**: if any request returns 403 OR cookie age > 4 minutes, refresh.
- **Transport**: plain net/http with the cookie attached.
- **HTML transport**: not needed for the pagination path. Used only at first-page harvest (which always goes through the browser anyway).

## Artifacts written
- `discovery/thriftbooks-page1-next-data.json` (370 KB) — full page-1 `__NEXT_DATA__` for offline schema work.
- `discovery/replay-page2-success.json` (310 KB) — proof that JSON-API replay works on page 2.
- `discovery/search-thriftbooks.html` (160 KB) — search-page HTML for offline schema work.
- `discovery/cookies-sanitized.txt` — cookie keys/structure for audit; `aws-waf-token` value redacted.
