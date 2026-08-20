## Customer model

**Persona 1 — Priya, retail dividend investor (age 42, software PM in Austin)**
- Today (without this CLI): Logs into Schwab + Yahoo Finance web every Sunday, manually copies dividend payouts into a Google Sheet, eyeballs yield-on-cost. Uses yfinance in a Jupyter notebook for ad-hoc charts.
- Weekly ritual: Sunday morning — reviews dividends received that week, checks ex-div dates for next 30 days, scans for any dividend cuts or special distributions across her 28 holdings.
- Frustration: No tool tracks her cost basis. Every "yield" she sees is yield-on-market, not yield-on-cost. Yahoo's site shows aggregate dividend amounts but never per-lot.

**Persona 2 — Marcus, options-selling swing trader (age 31, day-job at a logistics co.)**
- Today: Has yfinance + a hand-rolled Python script that downloads option chains nightly into CSVs. Filters in pandas. Reads Reddit r/options + Yahoo before market open.
- Weekly ritual: Sunday night — scans 40 tickers for next-Friday and 30-45 DTE options chains, looks for OTM puts with high IV at strikes 5-15% below spot, cross-checks earnings dates to avoid earnings-week assignments.
- Frustration: yfinance returns the full chain every time; he has to manually compute moneyness and DTE in pandas. No tool joins "earnings this week" against his option-watch tickers.

**Persona 3 — Lena, value-screening hobbyist quant (age 27, junior analyst at a boutique RIA)**
- Today: Pays for Stock Rover for screening; uses yfinance at home for tinkering. Maintains a spreadsheet of insider buying signals she pulls manually from openinsider.com.
- Weekly ritual: Saturday afternoon — runs custom P/E < 15, ROE > 15%, debt/equity < 1 screens across the S&P 500. Cross-checks any hits against insider net-buying in the last 30 days.
- Frustration: Yahoo's remote screener only has 12 predefined IDs. yfinance has no screener. She wants SQL against synced fundamentals.

**Persona 4 — Sam, autonomous agent author (age 35, indie dev building a "daily market briefing" Claude agent)**
- Today: Has a Claude agent that needs to call Yahoo data through Python subprocess on top of yfinance. Constantly hits 429s on his Hetzner VPS. Has resorted to scraping Yahoo HTML with a headless browser.
- Weekly ritual: Re-runs his briefing agent every weekday morning at 7am; agent needs structured JSON output, deterministic exit codes, and a working session even when the VPS IP is rate-limited.
- Frustration: yfinance returns Python objects, not JSON. No tool offers a `--chrome` cookie-import escape hatch when Yahoo blocks the cloud IP.

## Candidates (pre-cut)

| # | Name | Command | Source | Persona | Kill/keep notes |
|---|------|---------|--------|---------|-----------------|
| C1 | Portfolio performance tracker | `portfolio perf` | (d) prior-built, (c) cross-entity | Priya | KEEP. |
| C2 | Dividend income report with yield-on-cost | `portfolio dividends --year 2026` | (d) prior-keep, (a) Priya | Priya | KEEP. |
| C3 | Morning digest across watchlist | `digest` | (d) prior-built | Sam, Priya | KEEP. |
| C4 | Earnings calendar filtered to holdings | `earnings-calendar --holdings` | (d) prior-reframe | Marcus, Priya | Reframe; folds into digest. |
| C5 | Options moneyness + DTE filter | `options AAPL --moneyness otm --max-dte 45` | (d) prior-keep, (a) Marcus | Marcus | KEEP. |
| C6 | Local SQL screener over synced fundamentals | `screen-local --pe-max 15 --roe-min 0.15` | (d) prior-keep, (a) Lena | Lena | KEEP. |
| C7 | Chrome cookie import fallback | `auth login --chrome` | (d) prior-keep, (b) crumb | Sam | KEEP. |
| C8 | Insider net-buying signal screener | `insiders --recent 30d --net-buying` | (d) prior-keep, (a) Lena | Lena | KEEP. |
| C9 | Multi-symbol total-return comparator | `compare AAPL MSFT NVDA --range 1y --include-divs` | (a) Priya, (c) | Priya, Lena | KEEP. |
| C10 | Covered-call screener over holdings | `options --covered-calls --min-yield-annualized 0.10 --max-dte 45` | (a) Marcus, (c) | Marcus | KEEP. |
| C11 | Watchlist correlation matrix | `watchlist correlate tech --range 6m` | (c) | Lena | KEEP. |
| C12 | Drawdown / max-drawdown report | `perf drawdown AAPL --range 5y` | (a) Lena | Lena | KILL — thin, fold into compare. |
| C13 | Ex-dividend calendar across watchlist | `ex-div --watchlist tech --next 30d` | (a) Priya, (c) | Priya | KILL — sibling of digest. |
| C14 | Stale-cache report | `stale --resource history --older-than 24h` | (c) local-data | Sam | KILL — duplicated by framework. |
| C15 | Earnings beat/miss surprise history | `earnings AAPL --surprises` | (b) | Marcus | KILL — already absorbed #18. |
| C16 | News sentiment aggregator | `news --watchlist tech --sentiment` | (a) Sam | Sam | KILL rule #1 — LLM dependency. |
| C17 | Quote alert | `quote AAPL --alert price>200` | (a) | — | KILL rule #4 — scope creep. |
| C18 | Sustainability/ESG report | `sustainability AAPL` | (b) | Lena | KILL — wrapper. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| T1 | Portfolio performance tracker | `portfolio perf` | 9/10 | hand-code | Joins local `portfolio_lots` × cached `history` × `dividends`. No public tool maintains cost basis. |
| T2 | Dividend income + yield-on-cost | `portfolio dividends --year 2026` | 8/10 | hand-code | Per-lot dividend roll-up + YoC; Priya's killer feature. |
| T3 | Morning digest across watchlist | `digest --watchlist tech` | 9/10 | hand-code | Fans out quote/news/calendarEvents/dailyGainers, filtered by local watchlist. |
| T4 | Options moneyness + DTE filter | `options AAPL --moneyness otm --max-dte 45` | 8/10 | hand-code | Local processing of full chain against spot + expiration. |
| T5 | Local SQL screener | `screen-local --pe-max 15 --roe-min 0.15` | 8/10 | hand-code | Parametrized SQL over synced fundamentals; Yahoo's remote screener is 12 IDs only. |
| T6 | Chrome cookie import | `auth login --chrome` | 9/10 | hand-code | Reads Chrome's Cookies SQLite; persists A1/A3/B1 for `.yahoo.com`. No competitor has this. |
| T7 | Insider net-buying screener | `insiders --recent 30d --net-buying --watchlist tech` | 7/10 | hand-code | Joins `insiderTransactions` × watchlist + buy/sell ratio aggregation. |
| T8 | Total-return comparator | `compare AAPL MSFT NVDA --range 1y --include-divs` | 7/10 | hand-code | Joins cached `history` × `dividends` for total return (price + reinvested div). |
| T9 | Covered-call screener over holdings | `options --covered-calls --min-yield-annualized 0.10 --max-dte 45` | 7/10 | hand-code | Joins `portfolio_lots` (shares >=100) × options chains × spot for annualized yield. |
| T10 | Watchlist correlation matrix | `watchlist correlate tech --range 6m` | 6/10 | hand-code | Pairwise Pearson over cached daily returns. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Earnings calendar `--holdings` | Folds into digest; absorbed #19 covers per-symbol | digest (T3) |
| Drawdown report (C12) | Thin compute, inline as `--metrics` on compare | compare (T8) |
| Ex-div calendar (C13) | Subset of digest output | digest (T3) |
| Stale-cache report (C14) | Duplicated by framework `sync --since` | (framework) |
| Earnings beat/miss history (C15) | Absorbed manifest #18 covers it | absorbed #18 |
| News sentiment aggregator (C16) | LLM dependency kill | absorbed #40 |
| Quote alert (C17) | Scope creep — needs background process | (none) |
| Sustainability/ESG (C18) | Thin wrapper of quoteSummary | absorbed #16 |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Portfolio performance tracker (`portfolio perf`) | **Keep** | Already shipped; Priya weekly ritual; 9/10. |
| Dividend income report (`portfolio dividends`) | **Keep** | Planned but never built last run; Priya explicit frustration. |
| Morning digest (`digest`) | **Keep** | Already shipped; Sam + Priya both touch daily. |
| Earnings calendar filtered to holdings (`earnings-calendar --watchlist`) | **Reframe** | Right idea, wrong shape — folds into digest. Listed as killed so user can override. |
| Options moneyness filter (`options --moneyness otm --max-dte 45`) | **Keep** | Marcus weekly ritual. |
| Local SQL screener (`screen-local`) | **Keep** | Lena weekly ritual; Yahoo screener gap is real. |
| Chrome cookie import fallback (`auth login --chrome`) | **Keep** | Brief P0; live probe confirms 429s on this IP. |
| Insider-buying signal screener (`insiders --recent 30d --net-buying`) | **Keep** | Lena weekly ritual; openinsider proves demand. |
