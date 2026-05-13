# Novel Features Brainstorm — alphavantage-pp-cli

## Customer model

**Persona 1: Wei, the A-share retail trader maintaining a US watchlist**

*Today (without this CLI):* Wei holds A-shares as core but keeps a 12-name US watchlist (NVDA, AAPL, TSM, AMD, MSFT, GOOG, META, AVGO, ARM, TSLA, COIN, MSTR) for "what's happening in the West tonight." Before bed (Asia evening = US pre-market), he opens Sina Finance, Xueqiu, manually clicks through 12 tickers checking pre-market news and after-hours moves. He uses 财联社 for headlines but it skips US-only stories. He has a free Alpha Vantage key buried in a `.env` from a Python script he wrote in 2024 — it broke when AV cut free tier from 500/day to 25/day. He cannot answer "across my 12 names, which one has the strongest negative sentiment swing in the last 24h" without scrolling 12 Yahoo Finance tabs.

*Weekly ritual:* Sunday night Asia time — pull a "what should I be watching this week" snapshot for his US names: pre-earnings tickers, biggest sentiment shifts since last Sunday, any insider transactions, top movers that intersect his watchlist. He pastes findings into his notes app.

*Frustration:* The 25/day quota. If he calls `NEWS_SENTIMENT` once per ticker (12 calls), he's burned half his daily budget before doing anything else. He can't re-query without re-burning. He once accidentally burned all 25 calls debugging a script and got silently fed empty results for the rest of the day because the wrapper didn't surface AV's "Information" field.

---

**Persona 2: Ji (the user), running `/stock-deep-dive` and `/market-pulse` from claude-desktop**

*Today (without this CLI):* Ji has a stock-toolkit with `/stock-deep-dive`, `/market-pulse`, `/portfolio-check` that orchestrate across SEC EDGAR (built), Finnhub (pending), FRED (pending), AKShare (built), Semantic Scholar, Unpaywall, etc. For US tickers, the deep-dive needs news sentiment + analyst-grade context + fundamentals. EDGAR gives filings, Finnhub gives quote/profile/insider, FRED gives macro — but **no source gives quantified sentiment scores per ticker per article**. He currently has to fall back to free-text Finnhub news + manual reading, which doesn't aggregate. He has a free AV key but no CLI to use it from the agent.

*Weekly ritual:* Daily morning `/market-pulse` (08:00 Asia/Shanghai scheduled task) + on-demand `/stock-deep-dive` whenever a ticker comes up in conversation. Pulse needs "top US movers + a sentiment read on each." Deep-dive needs "sentiment timeline for this ticker over last 30d + which articles drove the spikes." Both currently leave a sentiment-shaped hole.

*Frustration:* The 25/day quota inside an agent loop is brutal. The agent might burn 10 calls in one deep-dive doing `NEWS_SENTIMENT` per ticker + `EARNINGS_CALL_TRANSCRIPT` + a few `OVERVIEW`. He needs the CLI to pull-once-cache-forever so the next 5 deep-dives that touch NVDA reuse the same article corpus. Also he needs the agent to know "you have 8 calls left today" before it kicks off a 12-ticker sweep.

---

**Persona 3: Mei, the quant-curious researcher backtesting sentiment-vs-returns**

*Today (without this CLI):* Mei is testing a hypothesis: "do tickers with negative aggregate news sentiment in the prior 7 days underperform the market in the following 7 days?" She has been pulling AV `NEWS_SENTIMENT` for 30 names over 6 months and stuffing JSON into a local folder. She greps with `jq` across 180 files. She wants FTS5 ("which articles mentioned 'tariff' AND 'TSMC' in Feb 2026?") but is hand-rolling sqlite3 imports. She also wants to join `TOP_GAINERS_LOSERS` daily snapshots against her news sentiment table — "of the top 20 gainers each day, what was their pre-spike sentiment trajectory?"

*Weekly ritual:* Saturday batch — pull a week's worth of news_sentiment + top_gainers_losers, append to local store, then run a few analyses ("ticker-level sentiment z-score vs 1-week forward return"). Wants a permanent local archive she can grow forever.

*Frustration:* Every wrapper she's tried throws away the structured `ticker_sentiment` array from NEWS_SENTIMENT — they keep the article title and lose the per-ticker relevance/sentiment_score pairs which are the highest-information field in the whole API. She'd kill for a local table that preserves these arrays for SQL joins.

---

## Candidates (pre-cut)

[See subagent output — full table preserved here for audit trail; 16 candidates generated, 4 dropped pre-cut as subsets or already-absorbed]

## Survivors and kills

### Survivors (11, all >=5/10)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Quota status + dry-run plan | `quota status` / `quota plan <subcmd>` | 9/10 | Wei, Ji |
| 2 | News sweep + ticker_sentiment preserved | `news sweep --watchlist NAME` / `--tickers A,B,C` | 9/10 | Wei, Mei |
| 3 | Sentiment timeline | `news timeline SYMBOL --days 30` | 7/10 | Mei, Ji |
| 4 | FTS5 news search | `news search "QUERY" [--from DATE]` | 8/10 | Mei |
| 5 | Pre-earnings briefing | `briefing earnings SYMBOL` | 8/10 | Wei, Ji |
| 6 | Movers + sentiment overlay | `movers brief [--enrich sentiment]` | 8/10 | Wei, Mei, Ji |
| 7 | Macro snapshot | `macro snapshot` | 7/10 | Ji |
| 8 | Watchlist sentiment + delta | `watchlist sentiment --name NAME` | 8/10 | Wei |
| 9 | Compound screen | `screen [--watchlist] [--sentiment-min] [--has-earnings-in] [--insider-net-buy]` | 7/10 | Mei, Ji |
| 10 | Daily pulse bundle | `pulse us` | 8/10 | Ji |
| 11 | Sync backbone | `sync news` / `sync movers` / `sync earnings-calendar` | 9/10 | All |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Earnings transcripts grep | Single-endpoint wrapper + local regex; not weekly across personas | #5 pre-earnings briefing |
| Sentiment-vs-return analyze | Single persona (Mei), narrow claim, soft "depends" on weekly use | #9 compound screen |
| Raw query escape hatch | Already in absorbed manifest as table stake | n/a |
| News topics weekly | Subsumed — `--by-topic` flag on #3 covers it | #3 sentiment timeline |
| Movers vs watchlist | Subset of #6 with watchlist filter | #6 movers brief |

## Reprint verdicts

N/A — first print.
