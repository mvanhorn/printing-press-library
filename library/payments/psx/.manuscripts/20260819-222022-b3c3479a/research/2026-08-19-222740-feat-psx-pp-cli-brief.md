# PSX (Pakistan Stock Exchange) CLI Brief

## API Identity
- Domain: `dps.psx.com.pk` (Data Portal, primary) + `www.psx.com.pk` (corporate/regulatory root, secondary)
- Users: Pakistani retail investors, sell-side/buy-side analysts, quant hobbyists backtesting KSE-100
  names, fintech builders, financial journalists.
- Data profile: ~1,000+ listed instruments (equities, ETFs, TFCs/Sukuks), 18 named indices,
  37 sectors, intraday tick series, multi-year EOD history, margin-eligibility lists, corporate
  announcements and financial reports.
- **There is no official published PSX API.** The portal exposes unauthenticated JSON and
  AJAX HTML-table fragments consumed by its own front end. This is an undocumented-but-live
  surface, not a vendor API program.

## Reachability Risk
- **None.** Every endpoint below returned HTTP 200 unauthenticated from a plain `curl`, with real
  payloads, on 2026-08-19. No Cloudflare/WAF/challenge, no cookies, no login, no API key.
- Corroborating evidence: `mtauha/psxdata` (19*, actively pushed 2026-08-17, runs a schema-drift CI
  workflow) has **zero** open issues matching 403/blocked/broken/deprecated/rate-limit/captcha.
- Tier/permission hints from 4xx body: none observed (no 4xx on any GET probe).
- Probe-safe endpoint used: `GET /symbols` (read-only, no params, no side effects).
- Community-observed politeness ceiling: 2 req/sec, 5 concurrent workers (`psxdata/constants.py`).
  Treat as the rate-limit budget; the portal publishes no formal limit.
- Non-technical risk (recorded, user-weighed): PSX terms-of-service and market-data
  redistribution. Operator has reviewed and elected to proceed; the publish step is separately
  gated at Phase 6.

## Verified Endpoint Surface

### JSON (native, machine-readable)
| Endpoint | Method | Shape |
|---|---|---|
| `/symbols` | GET | `[{symbol,name,sectorName,isETF,isDebt}]` - full instrument master (~130 KB) |
| `/timeseries/int/{symbol}` | GET | `{status,message,data:[[epoch,price,volume]]}` - intraday ticks |
| `/timeseries/eod/{symbol}` | GET | `{status,message,data:[[epoch,close,volume,index]]}` - EOD series |

### AJAX HTML table fragments (structured, replayable)
| Endpoint | Method | Notes |
|---|---|---|
| `/market-watch` | GET | `<table class="tbl">`, ~475 KB, whole-market snapshot |
| `/trading-board/{market}/{board}` | GET | markets `REG,ODL,DFC,SQR,CSF`; boards `main,gem,bnb`; bid/offer depth |
| `/sector-summary/sectorwise` | GET | 37 sector aggregates, ~485 KB |
| `/historical` | **POST** `symbol=<SYM>` | full OHLCV history (OGDC = 2,476 rows) |
| `/screener` | GET | market cap, price, PE (TTM), dividend yield, free float, 30d avg vol, 1y change |
| `/debt-market` | GET | TFCs/Sukuks: coupon, maturity, face value, yields |
| `/eligible-scrips` | GET | margin-trading eligible list |
| `/financial-reports-list` | GET | filter form + report index |
| `/indices`, `/company/{symbol}`, `/announcements` | GET | SSR pages |

**Required headers** (community-proven): `User-Agent` (browser-shaped), `Referer: https://dps.psx.com.pk/`,
`X-Requested-With: XMLHttpRequest` for the AJAX fragments.

### Known quirks (must encode)
1. **`/historical` ignores date parameters server-side.** The POST always returns the *entire*
   history regardless of any start/end sent. Date filtering must happen client-side. Any CLI that
   advertises server-side date ranges here is lying.
2. `/trading-board` 404s without both path segments - it is not a bare endpoint.
3. `/timeseries/int` returns the *last active session*, not necessarily today (thin scrips like SILK
   return stale-dated ticks). Freshness must be surfaced, not assumed.
4. 18 index codes are fixed vocabulary: KSE100, KSE100PR, ALLSHR, KSE30, KMI30, BKTI, OGTI,
   KMIALLSHR, PSXDIV20, UPP9, NITPGI, NBPPGI, MZNPI, JSMFI, ACI, JSGBKTI, HBLTTI, MII30.

## Top Workflows
1. **Morning market check** - KSE-100 level, advances/declines, top movers by volume and % change.
2. **Historical pull for analysis/backtest** - multi-year OHLCV for one or many symbols, to CSV/JSON.
3. **Fundamental screening** - filter the universe by PE, dividend yield, market cap, free float.
4. **Watchlist / portfolio tracking** - a fixed set of holdings, priced and diffed over time.
5. **Sector rotation** - which of the 37 sectors is leading/lagging, and which names drive it.
6. **Corporate-action monitoring** - announcements and financial reports for held names.

## Table Stakes
- Historical OHLCV by symbol with date range (client-side filtered)
- Full ticker/symbol listing with sector metadata
- Live quote for a symbol
- Index constituents for a named index
- Sector aggregates
- Debt-market instruments
- Margin-eligible scrips
- Financial reports / fundamentals per symbol

## Data Layer
- Primary entities: `symbols`, `quotes` (market-watch snapshots), `eod_bars`, `intraday_ticks`,
  `sectors`, `screener_rows`, `debt_instruments`, `eligible_scrips`, `indices`, `index_constituents`,
  `announcements`, `financial_reports`.
- Sync cursor: trading date for `eod_bars`; snapshot timestamp for `quotes`/`screener_rows`.
- FTS/search: company `name`, `symbol`, `sectorName`, announcement text.
- Rationale: PSX's portal renders a *current* view and retains nothing for the user. Every
  longitudinal question - "what changed", "how did this sector trend", "when did PE compress" -
  is unanswerable upstream and trivial once mirrored locally.

## Codebase Intelligence
- Source: `mtauha/psxdata` (19*, Python, active), `ahad-raza24/PSX-MCP-Server` (5*, FastMCP, 12 tools),
  `AbdulSami455/PSX-Data-Api` (11*, FastAPI, Selenium-era), `psx-data-reader` (PyPI).
- Auth: none. No token, no header secret, no cookie.
- Data model: instrument master keyed by `symbol`; series keyed by `(symbol, epoch)`; boards keyed by
  `(market, board)`.
- Rate limiting: no documented server limit; community self-imposes 2 rps / 5 workers.
- Architecture insight: `psxdata` deliberately extracts columns *dynamically from `<th>` text* rather
  than by fixed position, because PSX reorders columns without notice. Position-indexed parsing (as in
  PSX-MCP-Server's `cells[7]`) is the known silent-breakage bug class here. Our HTML extraction must be
  header-name-driven, not index-driven.

## User Vision
- Operator flagged `dps.psx.com.pk` as the likely structured-data host over the marketing root.
  Confirmed correct: the portal carries all machine-readable surfaces. Scope approved as
  **dps portal (primary) + www root (secondary)**.

## Source Priority
- Primary: `dps-psx-data-portal` - no official spec; browser-sniff/direct-HTTP derived - auth: free
- Secondary: `www-psx-root` - no spec; SSR HTML - auth: free
- **Economics:** both free, no key anywhere. No paid tier to scope, no auth split.
- **Inversion risk:** low. The secondary has *less* structure than the primary, so the usual
  "clean spec inverts the ordering" failure mode cannot fire here.

## Product Thesis
- Name: `psx-pp-cli`
- Why it should exist: PSX publishes no API, and every existing tool is a *fetch-and-return* wrapper -
  a Python DataFrame library, an MCP scraper, a FastAPI proxy. None of them **keep** anything. The
  portal shows you today; nobody can tell you what changed since Tuesday. A single Go binary with a
  local SQLite mirror turns a stateless scrape target into a queryable, longitudinal market database
  with FTS, cross-entity SQL, agent-native JSON, and an MCP server - none of which exists for PSX today.

## Build Priorities
1. Data layer + sync for `symbols`, `eod_bars`, `quotes`, `sectors`, `screener_rows` (foundation).
2. Absorb every psxdata / PSX-MCP-Server capability as a first-class command with `--json`/`--agent`.
3. Header-name-driven HTML table extraction (never positional) as a shared internal helper.
4. Transcendence commands that require the local mirror - the longitudinal questions upstream cannot answer.
