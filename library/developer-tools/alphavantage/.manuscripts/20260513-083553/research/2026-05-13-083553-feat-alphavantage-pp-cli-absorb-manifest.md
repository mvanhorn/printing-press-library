# alphavantage-pp-cli Absorb Manifest

Built from official AlphaVantage MCP (106 tools), portfoliotree/alphavantage Go wrapper, RomelTorres/alpha_vantage (Python), ashleydavis/alpha-vantage-cli (npm), berlinbra+matteoantoci+calvernaz community MCPs, and Alpha Vantage's HTML docs. Every row below is a feature the printed CLI MUST build.

## Absorbed: 113 features (105 endpoint mirrors + 8 operational essentials)

### Core Stock (11)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 1 | TIME_SERIES_INTRADAY | MCP, Go wrapper | `series intraday SYMBOL --interval 1min\|5min\|15min\|30min\|60min` | SQLite cache, --json/--csv/--select, --dry-run, quota-tracked |
| 2 | TIME_SERIES_DAILY | MCP, Go wrapper | `series daily SYMBOL` | as above |
| 3 | TIME_SERIES_DAILY_ADJUSTED | MCP, Go wrapper | `series daily SYMBOL --adjusted` | as above |
| 4 | TIME_SERIES_WEEKLY | MCP, Go wrapper | `series weekly SYMBOL` | as above |
| 5 | TIME_SERIES_WEEKLY_ADJUSTED | MCP, Go wrapper | `series weekly SYMBOL --adjusted` | as above |
| 6 | TIME_SERIES_MONTHLY | MCP, Go wrapper | `series monthly SYMBOL` | as above |
| 7 | TIME_SERIES_MONTHLY_ADJUSTED | MCP, Go wrapper | `series monthly SYMBOL --adjusted` | as above |
| 8 | GLOBAL_QUOTE | MCP, all wrappers | `quote SYMBOL` | as above |
| 9 | REALTIME_BULK_QUOTES | MCP | `quote bulk SYM1,SYM2,...` (premium) | as above |
| 10 | SYMBOL_SEARCH | MCP, all wrappers | `search KEYWORDS` | as above |
| 11 | MARKET_STATUS | MCP | `market status` | as above |

### Options (3)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 12 | REALTIME_OPTIONS | MCP | `options realtime SYMBOL` (premium) | as above |
| 13 | REALTIME_OPTIONS_FMV | MCP | `options realtime SYMBOL --fmv` (premium) | as above |
| 14 | HISTORICAL_OPTIONS | MCP, Go wrapper | `options historical SYMBOL --date YYYY-MM-DD` | as above |

### Alpha Intelligence (7) — the high-gravity surface
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 15 | NEWS_SENTIMENT | MCP, multiple wrappers | `news sentiment --tickers AAPL,MSFT --topics technology,earnings --time-from 20260101T0000 --time-to 20260601T0000 --sort LATEST --limit 50` | **preserves ticker_sentiment[] array** (every wrapper drops this), persists to SQLite for FTS5 |
| 16 | EARNINGS_CALL_TRANSCRIPT | MCP | `earnings transcript SYMBOL --quarter 2025Q4` | as above + transcript_id cache |
| 17 | TOP_GAINERS_LOSERS | MCP, Go wrapper | `movers top [--side gainers\|losers\|active]` | snapshots to SQLite, daily diff |
| 18 | INSIDER_TRANSACTIONS | MCP | `insider transactions SYMBOL` | as above |
| 19 | INSTITUTIONAL_HOLDINGS | MCP | `institutional holdings SYMBOL` | as above |
| 20 | ANALYTICS_FIXED_WINDOW | MCP | `analytics fixed --tickers AAPL --range full --interval DAILY --calculations MEAN,STDDEV,CORRELATION` | as above |
| 21 | ANALYTICS_SLIDING_WINDOW | MCP | `analytics sliding --tickers AAPL --range full --interval DAILY --calculations MEAN,STDDEV` | as above |

### Fundamentals (13) — superset of MCP (8) and Go wrapper
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 22 | COMPANY_OVERVIEW | MCP | `fundamentals overview SYMBOL` | as above |
| 23 | INCOME_STATEMENT | MCP | `fundamentals income SYMBOL` | quarterly normalization in SQLite |
| 24 | BALANCE_SHEET | MCP | `fundamentals balance SYMBOL` | as above |
| 25 | CASH_FLOW | MCP | `fundamentals cashflow SYMBOL` | as above |
| 26 | EARNINGS | MCP | `fundamentals earnings SYMBOL` | as above |
| 27 | EARNINGS_ESTIMATES | docs | `fundamentals earnings-estimates SYMBOL` | as above |
| 28 | DIVIDENDS | Go wrapper, docs | `fundamentals dividends SYMBOL` | as above |
| 29 | SPLITS | Go wrapper, docs | `fundamentals splits SYMBOL` | as above |
| 30 | SHARES_OUTSTANDING | Go wrapper, docs | `fundamentals shares SYMBOL` | as above |
| 31 | LISTING_STATUS | Go wrapper, MCP | `fundamentals listings --date YYYY-MM-DD --state active\|delisted` | CSV-native endpoint with JSON shim |
| 32 | EARNINGS_CALENDAR | MCP, Go wrapper | `calendar earnings --horizon 3month --symbol SYM` | CSV-native endpoint with JSON shim |
| 33 | IPO_CALENDAR | MCP, Go wrapper | `calendar ipo` | CSV-native endpoint with JSON shim |
| 34 | ETF_PROFILE | docs | `fundamentals etf SYMBOL` | as above |

### Forex (4)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 35 | FX_INTRADAY | MCP, Go wrapper | `fx intraday FROM TO --interval 1min\|5min\|15min\|30min\|60min` | as above |
| 36 | FX_DAILY | MCP, Go wrapper | `fx daily FROM TO` | as above |
| 37 | FX_WEEKLY | MCP, Go wrapper | `fx weekly FROM TO` | as above |
| 38 | FX_MONTHLY | MCP, Go wrapper | `fx monthly FROM TO` | as above |

### Crypto (5)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 39 | CURRENCY_EXCHANGE_RATE | MCP, Go wrapper | `crypto rate FROM TO` | as above |
| 40 | DIGITAL_CURRENCY_INTRADAY | MCP, Go wrapper | `crypto intraday SYMBOL MARKET --interval 1min...60min` | as above |
| 41 | DIGITAL_CURRENCY_DAILY | MCP, Go wrapper | `crypto daily SYMBOL MARKET` | as above |
| 42 | DIGITAL_CURRENCY_WEEKLY | MCP, Go wrapper | `crypto weekly SYMBOL MARKET` | as above |
| 43 | DIGITAL_CURRENCY_MONTHLY | MCP, Go wrapper | `crypto monthly SYMBOL MARKET` | as above |

### Commodities (13)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 44 | WTI | MCP, Go wrapper | `commodities wti --interval daily\|weekly\|monthly` | as above |
| 45 | BRENT | MCP, Go wrapper | `commodities brent --interval ...` | as above |
| 46 | NATURAL_GAS | MCP, Go wrapper | `commodities natural-gas --interval ...` | as above |
| 47 | COPPER | MCP, Go wrapper | `commodities copper --interval ...` | as above |
| 48 | ALUMINUM | MCP, Go wrapper | `commodities aluminum --interval ...` | as above |
| 49 | WHEAT | MCP, Go wrapper | `commodities wheat --interval ...` | as above |
| 50 | CORN | MCP, Go wrapper | `commodities corn --interval ...` | as above |
| 51 | COTTON | MCP, Go wrapper | `commodities cotton --interval ...` | as above |
| 52 | SUGAR | MCP, Go wrapper | `commodities sugar --interval ...` | as above |
| 53 | COFFEE | MCP, Go wrapper | `commodities coffee --interval ...` | as above |
| 54 | GOLD_SILVER_SPOT | MCP | `commodities gold-silver-spot` | as above |
| 55 | GOLD_SILVER_HISTORY | MCP | `commodities gold-silver-history --interval daily\|weekly\|monthly` | as above |
| 56 | ALL_COMMODITIES | MCP, Go wrapper | `commodities all --interval ...` | as above |

### Economic Indicators (10)
| # | Function | Source | Our Command | Added Value |
|---|----------|--------|-------------|-------------|
| 57 | REAL_GDP | MCP, Go wrapper | `econ real-gdp --interval quarterly\|annual` | as above |
| 58 | REAL_GDP_PER_CAPITA | MCP, Go wrapper | `econ real-gdp-per-capita` | as above |
| 59 | TREASURY_YIELD | MCP, Go wrapper | `econ treasury-yield --maturity 3month\|2year\|5year\|7year\|10year\|30year` | as above |
| 60 | FEDERAL_FUNDS_RATE | MCP, Go wrapper | `econ federal-funds-rate --interval daily\|weekly\|monthly` | as above |
| 61 | CPI | MCP, Go wrapper | `econ cpi --interval monthly\|semiannual` | as above |
| 62 | INFLATION | MCP, Go wrapper | `econ inflation` | as above |
| 63 | RETAIL_SALES | MCP, Go wrapper | `econ retail-sales` | as above |
| 64 | DURABLES | MCP, Go wrapper | `econ durables` | as above |
| 65 | UNEMPLOYMENT | MCP, Go wrapper | `econ unemployment` | as above |
| 66 | NONFARM_PAYROLL | MCP, Go wrapper | `econ nonfarm-payroll` | as above |

### Technical Indicators (45)
All exposed under `indicator <name> SYMBOL --interval ... --time-period ... --series-type ...`:

| # | Functions (group) | Source | Our Command |
|---|-------------------|--------|-------------|
| 67-75 | Moving averages: SMA, EMA, WMA, DEMA, TEMA, TRIMA, KAMA, MAMA, T3 | MCP, Go wrapper | `indicator <name> SYMBOL --interval --time-period --series-type` |
| 76 | VWAP | MCP, Go wrapper | `indicator vwap SYMBOL --interval` |
| 77-83 | Oscillators: MACD, MACDEXT, STOCH, STOCHF, RSI, STOCHRSI, WILLR | MCP, Go wrapper | `indicator <name> SYMBOL --interval --time-period --series-type [--fast-period ...]` |
| 84-100 | Trend/directional: ADX, ADXR, APO, PPO, MOM, BOP, CCI, CMO, ROC, ROCR, AROON, AROONOSC, MFI, TRIX, ULTOSC, DX, MINUS_DI, PLUS_DI, MINUS_DM, PLUS_DM | MCP, Go wrapper | as above |
| 101-107 | Volatility: BBANDS, MIDPOINT, MIDPRICE, SAR, TRANGE, ATR, NATR | MCP, Go wrapper | as above |
| 108-110 | Volume: AD, ADOSC, OBV | MCP, Go wrapper | as above |
| 111 | Hilbert transform group: HT_TRENDLINE, HT_SINE, HT_TRENDMODE, HT_DCPERIOD, HT_DCPHASE, HT_PHASOR (6 functions) | MCP, Go wrapper | as above |

### Operational essentials (8)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 112 | Quota-aware error detection | (nobody handles all 3) | API client middleware checks `Note`/`Information`/`Error Message` in every response | RomelTorres #125, finance-quote #130, TradingAgents #305 all cite this as the #1 wrapper failure |
| 113 | Raw query escape hatch | (none) | `query function=X param1=Y param2=Z` | Lets agents reach premium-only or future endpoints |
| 114 | Env var auth | (Python wrappers force config files) | Reads `ALPHAVANTAGE_API_KEY` (alias `ALPHA_VANTAGE_API_KEY`) | Drop-in for CI / agent env |
| 115 | Doctor health check | (none across wrappers) | `doctor` — validates key, probes free endpoint, reports quota status, surfaces all 3 rate-limit shapes | Pre-deep-dive guard |
| 116 | Local SQLite store | (none) | News with FTS5, time-series cache, fundamentals cache, quota_log, watchlists | Pull-once-query-many |
| 117 | `--json` / `--select` / `--csv` / `--dry-run` | (none consistently) | All commands | Agent-native plumbing |
| 118 | Typed exit codes | (none) | 0/2/3/4/5/7/10 mapped to error classes | Composable in scripts |
| 119 | Sync backbone | (none) | `sync news` / `sync movers` / `sync earnings-calendar` with incremental cursor | Quota-conscious batch hydration |

---

## Transcendence (11 features, only possible with our approach)

Auto-suggested via Phase 1.5c.5 subagent (Pass 1 customer model → Pass 2 candidates → Pass 3 adversarial cut). All scored >=5/10 against the rubric (4 dimensions: differentiation, weekly use, local leverage, agent fit).

| # | Feature | Command | Score | Persona served | Why only we can do this |
|---|---------|---------|-------|----------------|-------------------------|
| T1 | Quota status + dry-run plan | `quota status` / `quota plan <subcmd args...>` | 9/10 | Wei (daily), Ji (every agent invocation) | Local `quota_log` table written by API client; `plan` uses static cost-per-subcommand map — no API call. AV-specific because 200-with-Note + 25/day is unique surface. |
| T2 | News sweep + ticker_sentiment preserved | `news sweep --watchlist NAME` | 9/10 | Wei (weekly), Mei (weekly) | Persists per-article `ticker_sentiment[]` rows that every other wrapper drops (RomelTorres, Go, npm). FTS5 + SQL joins. Burns ≤ N AV calls for N tickers. |
| T3 | Sentiment timeline | `news timeline SYMBOL --days 30 [--by-topic]` | 7/10 | Mei (weekly), Ji (on-demand) | Pure local SQL aggregation: `SELECT date, AVG(sentiment_score), COUNT(*) GROUP BY date FROM ticker_sentiment WHERE ticker=?`. Zero API cost after sync. |
| T4 | FTS5 news search | `news search "QUERY" [--from DATE] [--tickers ...]` | 8/10 | Mei (weekly) | SQLite FTS5 virtual table over articles.title + summary + topics. Boolean AND/OR, prefix match, ranked results. No re-fetch. |
| T5 | Pre-earnings briefing | `briefing earnings SYMBOL` | 8/10 | Wei (weekly), Ji (on-demand) | One command joins EARNINGS_CALENDAR + EARNINGS + NEWS_SENTIMENT(topic=earnings) + GLOBAL_QUOTE + last EARNINGS_CALL_TRANSCRIPT extract. Local cache where fresh. |
| T6 | Movers + sentiment overlay | `movers brief [--side gainers\|losers] [--enrich sentiment]` | 8/10 | Wei (daily), Mei (weekly), Ji (pulse) | TOP_GAINERS_LOSERS API call LEFT JOIN local ticker_sentiment grouped by ticker; emits 7d sentiment z-score per mover. |
| T7 | Macro snapshot | `macro snapshot` | 7/10 | Ji (daily /market-pulse) | Joins CPI + FEDERAL_FUNDS_RATE + TREASURY_YIELD + UNEMPLOYMENT + NONFARM_PAYROLL with indicator-natural TTL caching so daily reruns cost 0 API calls. |
| T8 | Watchlist sentiment + delta | `watchlist sentiment --name NAME` | 8/10 | Wei (weekly ritual) | Named persisted watchlists × `ticker_sentiment` history; reports mean sentiment + delta-since-last-scan. |
| T9 | Compound screen | `screen [--watchlist NAME] [--sentiment-min X] [--has-earnings-in Nd] [--insider-net-buy]` | 7/10 | Mei (weekly), Ji (on-demand) | Multi-predicate SQL across local `ticker_sentiment` (aggregated) + `earnings_calendar` + `insider_transactions`. No single API call can do this. |
| T10 | Daily pulse bundle | `pulse us` | 8/10 | Ji (daily /market-pulse 08:00 Asia/Shanghai) | Designed for ≤2 AV calls/day: top movers (1 call cached daily) + macro snapshot (0 calls after first) + watchlist sentiment delta (incremental). Quota strategy IS the feature. |
| T11 | Sync backbone | `sync news --watchlist NAME [--since last]` / `sync movers daily` / `sync earnings-calendar` | 9/10 | All three personas | Pull-once-query-many. Incremental cursor (time_published / date / quarter) stored in `sync_log`. Every other transcendence row depends on this existing. |

---

## Summary
- **Absorbed:** 113 features (106 endpoint mirrors + 8 operational)
- **Transcendence:** 11 (all >= 5/10)
- **vs official MCP (boss):** matches all 106 tools, adds 5 fundamental functions MCP doesn't expose, adds 11 transcendence features no MCP can offer (MCPs can't write to local SQLite)
- **vs Go wrapper (closest competitor):** matches the ~70 functions it covers, adds Alpha Intelligence's 5 newest features (insider/institutional/analytics/transcripts), adds SQLite layer + 11 transcendence
