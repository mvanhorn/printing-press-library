# MakerWorld Browser-Sniff Report

## Goal
Capture the keyword-search request that returns empty for server-originated calls
("search needs browser session state"). User pre-approved browser discovery (Phase 0 "website itself").

## Method
Chrome DevTools MCP → navigated a real browser to
`https://makerworld.com/en/search/models?keyword=dragon` (cleared Cloudflare),
listed XHR/fetch, inspected the search request.

## Key finding — the search path is `select/design2` (with a "2")
The site calls:
```
GET https://makerworld.com/api/v1/search-service/select/design2
    ?orderBy=score
    &searchSessionId=<token>      # analytics/ranking continuity — NOT required
    &designType=0
    &isFromSearchList=false
    &keyword=dragon
    &limit=20&offset=20
    &ref_=def_MWSearchResult_ScoreImpression   # analytics — NOT required
```
Custom headers observed: `x-bbl-client-name: MakerWorld`, `x-bbl-app-source: makerworld`,
`x-bbl-client-type: web`, `x-bbl-client-version: 00.00.00.01`. Cookies incl. `__cf_bm` (Cloudflare).

## Verified: works fully anonymously on the clean host
`GET https://api.bambulab.com/v1/search-service/select/design2?keyword=dragon&limit=&offset=&designType=0&orderBy=score&isFromSearchList=false`
returns `total:4768` from **plain curl — no headers, no searchSessionId, no cookie, no Cloudflare**.
My earlier server-side probe failed only because it used `select/design` (no "2").

## Runtime decision
- **base_url:** `https://api.bambulab.com/v1` (clean; no Cloudflare; no auth for reads)
- **transport:** standard HTTP (no Surf, no clearance cookie needed)
- **auth:** none for reads; Bearer Bambu-Cloud JWT only for download/favorites/likes/account
- `orderBy`: `score` (default) and `hotScore` confirmed as distinct sorts (totals 4768 vs 3712).

## Replayability
PASS — every discovered endpoint replays over direct HTTP. No browser sidecar in the shipped CLI.
