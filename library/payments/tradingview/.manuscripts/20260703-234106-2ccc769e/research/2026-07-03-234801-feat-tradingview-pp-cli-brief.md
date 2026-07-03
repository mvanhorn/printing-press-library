# TradingView CLI Brief

## API Identity
- Domain: Financial market data (stocks, crypto, forex) via TradingView's undocumented public endpoints.
- Users: Traders, quants, and agents who want fast terminal/scriptable access to symbol prices without a paid data-API key.
- Data profile: Symbol metadata (exchange, type, currency, country) + last price + FX rates.

## Reachability Risk
- None. All endpoints probed live 2026-07-03, HTTP 200, no bot-protection:
  - `GET https://symbol-search.tradingview.com/symbol_search/v3/?text=<q>` → symbol candidates
  - `GET https://scanner.tradingview.com/symbol?symbol=<EXCHANGE:TICKER>&fields=...&no_404=true` → flat price object (any market)
  - `POST https://scanner.tradingview.com/<market>/scan` → batch quotes (market-scoped)
- No API key required for these endpoints. User has a paid TV subscription + logged-in Chrome, but public endpoints already deliver real-time-ish prices for the requested features.

## User Vision
- Two features for now: (1) research a ticker/symbol and query its price in USD; (2) convert that price to EUR.
- Markets: stocks and crypto (forex used internally for conversion).
- Deliberately lean scope — not an absorb-everything build.

## Top Workflows
1. `search AAPL` → find the exact `NASDAQ:AAPL` ticker (+ exchange, type, currency).
2. `quote NASDAQ:AAPL` → last price shown in native currency, USD, and EUR in one call.
3. `convert 100 USD EUR` → currency conversion using TradingView's own forex rates.

## Table Stakes (from community wrappers)
- Symbol search/resolve (shner-elmo/TradingView-Screener, tvscreener).
- Last price / quote for a fully-qualified ticker (all wrappers).
- Multi-market support: stocks, crypto, forex (scanner market paths + universal /symbol).
- `--json` structured output for scripting/agents.

## Data Layer
- Primary entities: symbols (exchange, ticker, type, currency, price), fx_rates (pair, rate).
- Optional local store: a watchlist of symbols for batch quoting (offline-cache).
- No mandatory sync — prices are fetched live; store is opt-in convenience.

## Endpoint Contract (confirmed live)
- Symbol search response: `{"symbols":[{"symbol":"AAPL"(may wrap matches in <em>),"description","type","exchange","currency_code","country","typespecs":[...]}]}`. Strip `<em>`/`</em>`.
- Universal quote (`/symbol`): `{"close":308.63,"currency":"USD","change":4.84,"pricescale":100,"description":"Apple Inc.","type":"stock"}`.
- Batch scan: `{"totalCount":N,"data":[{"s":"NASDAQ:AAPL","d":[<columns in requested order>]}]}`.
- FX: `FX_IDC:EURUSD` close = USD per 1 EUR → EUR = USD / EURUSD; generalized native→USD via `FX_IDC:<CUR>USD`. USDT treated as ≈USD (stablecoin peg), surfaced explicitly.

## Competitors
- Python: shner-elmo/TradingView-Screener, deepentropy/tvscreener, AnalyzerREST/python-tradingview-ta (TA recommendations).
- TS/npm: jmargieh/tradingview-screener, tradingview-screener-ts; Mathieu2301/TradingView-API (WebSocket real-time).
- No dedicated CLI exists. No terminal-first, agent-native tool. No community tool converts a symbol's price into another fiat currency.

## Product Thesis
- Name: TradingView CLI (`tradingview-pp-cli`).
- Why it should exist: the fastest terminal way to resolve a ticker and get its price in USD *and* EUR, no API key, agent-native `--json`. Community wrappers give you a screener library, not a one-command "what's AAPL worth in euros" answer.

## Build Priorities
1. Data/client scaffold with required TradingView browser headers (User-Agent, Origin, Referer).
2. `search` — resolve query → EXCHANGE:TICKER candidates (strip <em>, expose type/exchange/currency).
3. `quote` — price in native + USD + EUR via universal /symbol + FX conversion.
4. `convert` — generic fiat conversion via TradingView forex rates.
5. (Optional) local watchlist store for batch quotes.
