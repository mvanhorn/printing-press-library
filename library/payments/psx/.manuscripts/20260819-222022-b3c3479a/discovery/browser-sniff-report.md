# PSX Browser-Sniff Discovery Report

**Backend:** agent-browser (Playwright HAR capture). browser-use v0.13.6 was preferred but its
daemon requires an interactive Chrome "Allow remote debugging" grant, so the documented
fallback was used. 235 requests captured across a 9-page user flow.

**Primary goal walked:** "Check today's market, then drill into one company" — homepage →
search → company/OGDC → announcements → payouts → calendar → circuit-breakers → listings →
graphical-view.

**Runtime verdict:** `reachability.mode = standard_http`. No WAF, no challenge, no cookies,
no auth. The printed CLI ships plain HTTP transport — **no Surf, no browser-clearance, no
resident browser**. Cardinal Rule 5 (replayability) satisfied: every discovered surface was
re-executed successfully with bare `curl`.

## What browser-sniff found that direct probing and the community libraries did NOT

| Endpoint | Method | Params / notes |
|---|---|---|
| `/announcements` | POST | `type,symbol,query,count,offset,date_from,date_to,page` — **222,464 entries**, server-side search + date filter + pagination |
| `/payouts` | POST | `symbol,count,offset` — paginated dividend/bonus history |
| `/calendar` | POST | `from,to` — **JSON**; AGM/EOGM events |
| `/company/payouts` | POST | `symbol` — per-company payout history |
| `/company/reports/{symbol}` | GET | per-company financial report index |
| `/data/top-10-sectors` | GET | **JSON** `[{name,code,volume}]` |
| `/performers` | GET | TOP ACTIVE / gainers / losers tables |
| `/debt-performers` | GET | debt-market movers |
| `/watchlist` | GET | homepage watchlist fragment |
| `/listings-table/{board}/{status}` | GET | e.g. `/listings-table/main/nc` |
| `/circuit-breakers`, `/calendar`, `/listings`, `/graphical-view`, `/corporate-briefing` | GET | additional SSR feeds |
| `/announcements/{psx,companies,cdc,secp,nccpl}` | GET | five distinct announcement streams |
| `financials.psx.com.pk` | — | separate host surfaced in nav; not captured this run |

None of `announcements`, `payouts`, `calendar`, `performers`, or `top-10-sectors` appear in
`psxdata`, the PSX MCP server, or any other surveyed tool.

## Negative findings (equally important)

- **Search is client-side.** Typing into `#mainSearchbar` fired **zero** network requests; the
  page filters the already-loaded `/symbols` payload in JS. There is no autocomplete endpoint to
  wrap — and the printed CLI can reproduce the same search fully offline.
- **HAR carried no response bodies.** `traffic-analysis.json` raised `empty_response_shapes`
  (confidence 0.9). Per the Phase 0 rule, every endpoint was re-fetched with `curl` and real
  response shapes captured to `discovery/samples/` before any spec was trusted.
- **Auto-generated spec is unusable as-is.** The sniffer emitted 5 endpoints and modelled
  `/timeseries/int/KSE100` and `/timeseries/int/OGDC` as two *separate* endpoints
  (`list_KSE100`, `list_OGDC`) rather than one path-parameterised endpoint. The spec is
  hand-authored from the captured evidence instead.

## Verified response shapes

- `/symbols` -> array[1004] `{symbol,name,sectorName,isETF,isDebt}`
- `/timeseries/int/{sym}` -> `{status,message,data:[[epoch,price,volume]]}` (OGDC: 3,041 ticks)
- `/timeseries/eod/{sym}` -> `{status,message,data:[[epoch,close,volume,?]]}` (OGDC: 1,240 bars;
  4th element consistently exceeds close — likely VWAP/average, **not** confirmed; preserved
  unnamed rather than guessed)
- `/data/top-10-sectors` -> array[10] `{name,code,volume}`
- `POST /calendar` -> `{status,message,data:[{id,symbol,name,type,date,time,city,period_end}]}`

## Required request headers

`User-Agent` (browser-shaped), `Referer: https://dps.psx.com.pk/`, and
`X-Requested-With: XMLHttpRequest` for the AJAX fragment endpoints.
