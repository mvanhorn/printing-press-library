# InvestorPaisa CLI Brief

## API Identity
- Domain: investorpaisa.com — Indian equities dashboard for the Nifty 200 (Next.js App Router on Vercel, bom1). "For information and education only." No official public API or docs; surface discovered by browser study of the logged-in site.
- Users: retail/self-directed Indian equity investors who want a fast screener, live movers, sector heatmap, market breadth, 52-week signals and per-stock news without the noise of Screener.in / Moneycontrol / TradingView.
- Data profile: Nifty 200 universe (200 symbols, 18 sectors). Snapshot-style: `/api/live` (15s cache) = intraday LTP rows + index levels (NIFTY50/100/200) + breadth + per-row signals; `/api/screener/rows` = 48 precomputed metrics per stock (rs, rsi14, sma/ema, ret1w..1y, beta1y, cagr3y, maxdd1y, 52w position, volratio, turnover20); `/api/market/universe` = symbol/name/sector/close/chg1d; news is Google-News RSS aggregation tagged by symbol; videos are YouTube (CNBC-TV18, NDTV Profit, Zee Business). Per-stock page embeds ~5y columnar daily OHLCV (d,o,h,l,c,v) + split/bonus adjustment factors (`fac`) + corporate-action `events`.
- Auth: Google sign-in via Auth.js (database session). Every `/api/*` data route and page returns 401 / 307→login without the `__Secure-authjs.session-token` cookie. Public: `/api/health`, `/api/auth/{providers,csrf,session}`.

## Reachability Risk
- None. `probe-reachability https://investorpaisa.com/live` → `mode: standard_http` (stdlib + Surf both 200). No WAF/Cloudflare challenge; Vercel only. 
- Login wall is the only gate (401 on /api/*). Mitigation: `auth login --chrome` cookie import (session cookie), plus `INVESTORPAISA_SESSION_TOKEN` env override.
- Probe-safe endpoint used: GET /api/health

## Top Workflows
1. "What's moving right now?" — live movers (gainers/losers), index level (Nifty 50/100/200), advance/decline, sector heat, unusual volume.
2. Screen the Nifty 200 — filter by expression (`close > sma50 and sma50 > sma200 and rs >= 70`) and/or sector, sort by any metric; save/load screens.
3. One-stock deep dive — price, day/52w range, RSI/SMA/EMA, returns, volume pace, recent news, video, signals, 5y adjusted history.
4. Signals feed — 52-week highs/lows, gap, volume spikes (71 "now" signals on the sampled session), market overview (breadth, % above 50/200 DMA, equal-weight 200 index).
5. Watchlists + bookmarks — up to 20 watchlists (CRUD + PUT items), bookmark news/video cards.

## Table Stakes
- Competitors: Screener.in (fundamental screens, no API), TradingView (charts/screener, paid API-less), NSE India CLI (`nse-india` in the public library: quotes/indices/filings), Groww/Zerodha Kite (broker-bound). None offer a scriptable, agent-native view of this particular momentum/breadth dataset.
- Must have: quote/search by symbol, sector list, screener with expressions, live movers, news by symbol, watchlist CRUD, JSON/CSV output, `--select`, `doctor`, `auth login --chrome`.

## Data Layer
- Primary entities: stocks (universe), snapshots (live rows per fetch), screener metrics (daily), sectors, news cards, videos, watchlists (+items), bookmarks, screens, price history (per-symbol columnar OHLCV).
- Sync cursor: `fetchedAt`/`sessionDate` for live; `asOf` date for daily metrics; news `next` cursor token.
- FTS/search: symbol/name/sector over universe; news titles/summaries.

## Codebase Intelligence
- Source: browser study (Chrome DevTools MCP) + JS bundle grep. Next.js App Router, RSC flight payload embeds page data (`__next_f`). Client `apiFetch` helper; screener expression language compiled client-side (`useScreen` / `filterRows`) over the 200 rows from `/api/screener/rows`; `/api/screener/rows` has no `sector` server filter (the `?sector=` param is ignored — verified identical bytes).
- Auth: Auth.js v5 cookie `__Secure-authjs.session-token` (opaque UUID, DB session), `__Host-authjs.csrf-token`; mutating routes are same-origin.
- Rate limiting: none observed; `/api/live` `cache-control: private, max-age=15`; news `max-age=300`.
- Join note: `/api/screener/rows` rows carry `symbol,name,sector` (join key is `symbol`); `/api/market/universe` uses `s,n,i`.

## User Vision
- "Create, check, promote, dogfood and publish end to end" from the already-open, logged-in Chrome session; study the site thoroughly via Chrome DevTools MCP.

## Product Thesis
- Name: investorpaisa-pp-cli
- Why it should exist: a terminal/agent-native window onto a clean Nifty-200 momentum dataset — movers, breadth, signals, 48-metric screener with a real expression language, and news — that Screener.in/TradingView expose only through a UI, with local SQLite snapshots so you can diff "what changed since yesterday".

## Build Priorities
1. Spec-driven read surface: live, universe, screener rows, news (symbol + feed), market overview, videos, health, me, layout, screens, watchlists (CRUD+items), bookmarks.
2. `auth login --chrome` (Auth.js session cookie) + doctor.
3. Transcendence: local expression screener (`screen "<expr>"` over cached rows), `movers`, `sectors`, `breadth`, `signals`, `quote <SYMBOL>`, `history <SYMBOL>` (parses embedded flight payload of /chart/<SYM>), `snapshot diff`, `watchlist sync/pnl`.
