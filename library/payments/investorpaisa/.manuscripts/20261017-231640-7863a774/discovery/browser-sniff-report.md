# investorpaisa.com — Browser-Sniff Report

## 1. User Goal Flow
- Goal: "See what is moving in the Nifty 200 right now, screen it, and drill into one stock."
- Steps completed: /live (movers, treemap, sectors, signals, news) → /chart/RELIANCE (candles, SMA50/200, RSI14, day/52w range, news) → /screener → /market → /live/sectors → /live/signals → /news → /watchlist.
- Secondary: JS-bundle endpoint extraction; read-only GET probing of candidate /api paths.
- Coverage: all 5 nav tabs + legal pages visited; no mutating call was made.

## 2. Pages & Interactions
Backend used: Claude DevTools MCP against the user's already-open, signed-in Chrome tab (single tab; reload + same-origin fetch for inspection). Network list, request-header inspection, DOM snapshot, flight-payload inspection.

## 3. Configuration
Backend: chrome-devtools MCP. Pacing: ~1 req/s, ~60 GETs total, no 429 seen. Proxy-envelope: not detected. GraphQL: not detected.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | /api/health | 200 | json | public |
| GET | /api/auth/providers, /api/auth/csrf, /api/auth/session | 200 | json | public (session returns null when logged out) |
| GET | /api/live | 200 | json (cache private,max-age=15) | auth-required (401 anon) |
| GET | /api/market/universe | 200 | json (200 rows) | auth-required |
| GET | /api/screener/rows | 200 | json (200 rows × 48 fields, ~209KB) | auth-required |
| GET | /api/market/overview | 200 | json | auth-required |
| GET | /api/news?symbol=SYM | 200 (400 invalid_symbol if missing) | json (max-age=300) | auth-required |
| GET | /api/news/feed[?cursor=] | 200 | json {items,next,stale} | auth-required |
| GET | /api/videos?kind=video\|short | 200 (400 invalid_kind otherwise) | json | auth-required |
| GET | /api/news/img/{id}, /api/videos/thumb/{id} | image | image | auth-required |
| GET | /api/me, /api/me/layout | 200 | json | auth-required |
| PUT | /api/me/layout, /api/me/theme | (not exercised) | json | auth-required |
| GET | /api/screens | 200 [] | json | auth-required |
| GET/POST | /api/watchlists | 200 | json | auth-required |
| PATCH/DELETE | /api/watchlists/{id} | (from bundle, not exercised) | json | auth-required |
| PUT | /api/watchlists/{id}/items {symbols[]} | (from bundle) | json | auth-required |
| GET/POST/DELETE | /api/bookmarks | GET 200 [] | json | auth-required |
| GET | /chart/{SYM}, /screener, /market, /live/sectors, /live/signals, /news, /watchlist | 307→login anon | html (RSC flight) | auth-required |

## 5. Traffic Analysis
- Protocol: rest_json + Next.js RSC (flight payload `self.__next_f.push`) embedding page data.
- Auth signals: cookie names only — `__Secure-authjs.session-token`, `__Host-authjs.csrf-token`, `__Secure-authjs.callback-url` (Auth.js, Google OIDC). No Authorization header.
- Protection: none (Vercel; probe-reachability → standard_http, confidence 0.95).
- Param evidence: `symbol` (news), `kind` ∈ {video, short} (videos), `cursor` opaque base64 (news feed). `/api/screener/rows?sector=` ignored (identical 209414-byte response).
- Candidate commands: live, movers, sectors, breadth, signals, quote, history, screen, news, watchlist.

## 6. Coverage Analysis
Missed/untested: PATCH rename, bookmark POST/DELETE payload shape (card object), PUT layout, saved-screens POST. Per-symbol price history (columnar d/o/h/l/c/v + fac + events) exists only inside the /chart/{SYM} HTML flight payload — no JSON endpoint found (probed /api/chart, /api/history, /api/bars, /api/candles, /api/prices, /api/symbol: all 404).

## 7. Response Samples (shapes only; no personal data)
- /api/live → {enabled, fetchedAt, payload:{agg{n,up,down,avg,median,hi52,lo52,above200,valueCr,topValue,sectors[18],unusualVolume}, rows[200]{symbol,name,sector,ltp,chg,open,high,low,prevClose,volume,valueCr,relVol,relVal,d50,d200,hi52,lo52,pos52,fromHi,r30,gap,atrMove,rangePos,signals,ts}, indices{NIFTY50,NIFTY100,NIFTY200:{level,chg,chgPct,prevClose,prevDate,coverage}}, intraday[92], lastSession{date,intraday[85]}, lastTrade, sessionDate, crawl}}
- /api/news → {items:[{id,title,link,source,domain,hasLogo,hasImage,hasLargeImage,publishedAt,symbols[],summary}]}
- /api/watchlists → [{id,name,symbols[],updatedAt}]

## 8. Rate Limiting
None observed.

## 9. Authentication Context
Authenticated session used (user's own signed-in Chrome; verified via /api/auth/session). Method: existing tab, no cookie export. Session token/cookie values were NOT written to any file. Printed CLI will use `auth login --chrome` cookie import or INVESTORPAISA_SESSION_TOKEN.

## 10. Bundle Extraction
Main chunks grepped for `/api/` literals: found universe, screener/rows, live, bookmarks, watchlists(+items), me/layout, me/theme, news/img, videos/thumb. Not in bundle but found by probing: /api/news/feed, /api/market/overview, /api/me, /api/screens, /api/health.
