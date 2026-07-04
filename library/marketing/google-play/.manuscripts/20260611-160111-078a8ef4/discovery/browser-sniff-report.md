# Google Play Store Browser-Sniff Discovery Report

## User Goal Flow
- Goal: "Browse the Play games store as a market analyst" — load top games, open a game's detail page, read its reviews, then search.
- Steps completed:
  1. Opened `https://play.google.com/store/games` (loaded games landing; fired `qnKhOb` + `w3QCWb` cluster RPCs)
  2. Scrolled to trigger lazy cluster loads (more `w3QCWb`)
  3. Clicked into a game detail page `/store/apps/details?id=com.yalla.yallagames` (fired the `CLXjtf,A6yuRe,Ws7gDc,ZittHe,yowZ5,ag2B9c,e7uDs,oCPfdb` batch — the details ds: bundle)
  4. Scrolled detail page, opened the reviews dialog (fired `oCPfdb` reviews RPC)
  5. Scrolled the reviews dialog to trigger review pagination (`oCPfdb` again with continuation token)
- Steps skipped: in-page search box was not reachable with the reviews dialog open; search + suggest contracts were instead locked with direct curl (see below).
- Secondary flows: charts (`vyAe2`) and suggest (`IJ4APc`) verified directly with curl using the reference scraper's exact payloads.
- Coverage: 5 of 5 planned interactive steps; all 10 target surfaces contract-verified.

## Pages & Interactions
- `/store/games` — landing, scrolled twice
- `/store/apps/details?id=com.yalla.yallagames` — detail page, scrolled, opened "See all reviews" dialog, scrolled dialog
- Direct curl verification: charts `vyAe2`, reviews `oCPfdb`, suggest `IJ4APc`, app-details GET HTML, search GET HTML

## Browser-Sniff Configuration
- Backend: browser-use v0.13.0 (CLI mode, headless, no LLM key), XHR + fetch body interceptors installed after first load
- Pacing: ~1 req/s, no 429s observed during capture
- Proxy pattern: not a proxy-envelope. Two transports: GET+HTML (SSR `AF_initDataCallback`) and a single batchexecute RPC endpoint dispatched by `rpcid`.
- Runtime (probe-reachability): `standard_http`, confidence 0.95. stdlib HTTP and surf-chrome both returned 200. No clearance cookie or resident browser needed in the printed CLI.

## Endpoints Discovered
| Method | Path | rpcid / ds | Status | Content-Type | Auth |
|--------|------|-----------|--------|--------------|------|
| GET | /store/apps/details?id=&hl=&gl= | ds:5 (Ws7gDc) | 200 | text/html | public |
| GET | /store/search?q=&c=apps&hl=&gl=&price= | ds:4 | 200 | text/html | public |
| GET | /store/apps/datasafety?id=&hl= | ds:3 | 200 | text/html | public |
| GET | /store/apps/developer?id=NAME / /store/apps/dev?id=NUM | ds:3 | 200 | text/html | public |
| GET | /store/apps | category anchors | 200 | text/html | public |
| POST | /_/PlayStoreUi/data/batchexecute (charts) | vyAe2 | 200 | application/json+protobuf | public |
| POST | /_/PlayStoreUi/data/batchexecute (reviews) | oCPfdb | 200 | application/json+protobuf | public |
| POST | /_/PlayStoreUi/data/batchexecute (suggest) | IJ4APc | 200 | application/json+protobuf | public |
| POST | /_/PlayStoreUi/data/batchexecute (permissions) | xdSrCf | 200 | application/json+protobuf | public |
| POST | /_/PlayStoreUi/data/batchexecute (list/search pagination) | qnKhOb | 200 | application/json+protobuf | public |
| GET (2-step) | details page -> ag2B9c similar cluster URL -> GET cluster | ag2B9c | 200 | text/html | public |

## Traffic Analysis
- Protocols: `ssr_embedded_data` (0.95) on GET HTML pages, `google_batchexecute` (0.95) for RPCs.
- Auth signals: none. All surfaces answer anonymously, no token/cookie/CSRF (`at=` token optional, verified removed).
- Protection signals: none at capture time. Documented volume risk: 429, 503+captcha, and a 200-body sentinel `com.google.play.gateway.proto.PlayGatewayError`.
- Generation hints: standard_http transport; positional-protojson responses require hand-parsing; rate-limit mitigation contract (throttle + backoff + cache).
- Warning: batchexecute uses the `)]}'` length-prefixed double-encoded JSON envelope; index paths shift ~yearly on redesigns.

## Coverage Analysis
- Exercised: app details, charts/clusters, reviews (+ pagination), search, suggest, similar, developer, datasafety, permissions, categories.
- Likely missed: nothing in the public-store surface that the reference scrapers (facundoolano JS, JoMingyu Python) cover; both were read as ground truth.

## Response Samples
- Charts `vyAe2`: 243KB, `[["wrb.fr","vyAe2",...]]` with real appIds (com.dreamgames.royalkingdom, com.epicgames.fortnite, ...).
- Reviews `oCPfdb`: `[["wrb.fr","oCPfdb","<double-encoded review array>",...]]`, reviews at payload[0], next token at payload[1][1].
- Suggest `IJ4APc`: `[["wrb.fr","IJ4APc","[[[["puzzle games",...,"/store/search?q=puzzle+games&c=apps"],...]]]",...]]`.

## Rate Limiting Events
- None during capture (~1 req/s). The printed CLI ships a ~1 req/s default throttle, exponential backoff on 429/503, and PlayGatewayError-in-200 detection as a typed RateLimitError.

## Authentication Context
- No authenticated session used; the entire public store is anonymous. Session state file: none created. No credentials in any artifact.

## Reference Source
- `reference-list.js` (facundoolano/google-play-scraper lib/list.js) archived in this directory: exact working `vyAe2` charts f.req payload template (`${num}`, `${collection}`, `${category}` placeholders), used to lock the charts contract.
