# Benzinga CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

Source landscape: the only complete-ish prior tool is the official `benzinga` PyPI client (wraps a SUBSET of the documented surface). `openbb-benzinga` covers news+analyst only. No MCP server exists. The official OpenAPI specs define the FULL surface — so absorbing = generating every documented endpoint as a typed command, which already beats the Python client. Every row ships with `--json`/`--select`/`--compact`/`--csv`/`--dry-run`/typed exits + SQLite persistence + MCP exposure.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | News stories (tickers/channels/updatedSince/displayOutput) | Python client `News.news` | (generated endpoint) news list | Offline FTS5 over bodies, delta-sync, --select |
| 2 | Removed news reconciliation | docs news-removed | (generated endpoint) news removed | Keeps local store honest on retractions |
| 3 | News channels list | docs news/channels | (generated endpoint) news channels | Channel discovery for filtering |
| 4 | Earnings calendar | Python `earnings` | (generated endpoint) calendar earnings | Beat/miss computed offline (see transcendence) |
| 5 | Dividends calendar | Python `dividends` | (generated endpoint) calendar dividends | v2.2 fields, offline yield filters |
| 6 | Splits calendar | Python `splits` | (generated endpoint) calendar splits | |
| 7 | Economics calendar | Python `economics` | (generated endpoint) calendar economics | country + importance + actual/consensus |
| 8 | Guidance calendar | Python `guidance` | (generated endpoint) calendar guidance | |
| 9 | IPOs calendar | Python `ipo` | (generated endpoint) calendar ipos | |
| 10 | Offerings (secondary) | docs offerings | (generated endpoint) calendar offerings | |
| 11 | M&A calendar | docs ma | (generated endpoint) calendar ma | |
| 12 | Conference calls | Python `conference_calls` | (generated endpoint) calendar conference-calls | Joined to transcripts (transcendence) |
| 13 | Analyst ratings | Python `ratings` | (generated endpoint) calendar ratings | Accuracy-ranked (transcendence) |
| 14 | Ratings firms | docs ratings/firms | (generated endpoint) calendar ratings-firms | |
| 15 | Ratings analysts (accuracy) | docs ratings/analysts | (generated endpoint) calendar ratings-analysts | Powers analyst-accuracy |
| 16 | FDA calendar | docs fda | (generated endpoint) calendar fda | PDUFA catalyst tracking |
| 17 | Corporate events | docs events | (generated endpoint) calendar events | |
| 18 | Calendar removed | docs calendar-removed | (generated endpoint) calendar removed | Delete reconciliation |
| 19 | Unusual options activity | Python `options_activity` | (generated endpoint) signals option-activity | Aggregatable via analytics |
| 20 | Block trades | docs block_trade | (generated endpoint) signals block-trade | |
| 21 | Halt / resume | docs halt_resume | (generated endpoint) signals halt-resume | Feeds `why` |
| 22 | Analyst insights | docs analyst/insights | (generated endpoint) analyst insights | |
| 23 | Consensus ratings | docs consensus-ratings | (generated endpoint) analyst consensus-ratings | |
| 24 | Bulls say / bears say | docs bulls_bears_say | (generated endpoint) analyst bulls-bears-say | |
| 25 | ERX gaps | docs erx_gaps | (generated endpoint) analyst erx-gaps | |
| 26 | Congressional trades | docs gov/congress/trades | (generated endpoint) gov congress-trades | Feeds insider-cluster |
| 27 | Congressional trade reports | docs gov/congress/trades/reports | (generated endpoint) gov congress-trade-reports | |
| 28 | SEC insider transactions | docs sec/insider_transactions | (generated endpoint) insider transactions | Feeds insider-cluster |
| 29 | Insider owners | docs insider_transactions/owners | (generated endpoint) insider owners | |
| 30 | Fundamentals (root) | Python `fundamentals` | (generated endpoint) fundamentals get | |
| 31 | Financials | Python `financials` | (generated endpoint) fundamentals financials | |
| 32 | Valuation ratios | Python `valuation_ratios` | (generated endpoint) fundamentals valuation-ratios | |
| 33 | Earning ratios | Python `earning_ratios` | (generated endpoint) fundamentals earning-ratios | |
| 34 | Operation ratios | Python `operation_ratios` | (generated endpoint) fundamentals operation-ratios | |
| 35 | Company / company profile | Python `company`/`company_profile` | (generated endpoint) fundamentals company/company-profile | |
| 36 | Balance sheet / income / cash flow | docs v3 fundamentals | (generated endpoint) fundamentals balance-sheet/income-statement/cash-flow | |
| 37 | Derived figures, share-price-ratios | docs v3 fundamentals | (generated endpoint) fundamentals derived/share-price-ratios | |
| 38 | Alpha/beta, asset classification | Python `alpha_beta`/`asset_classification` | (generated endpoint) fundamentals alpha-beta/asset-classification | |
| 39 | Earning reports, share class | Python `earning_reports`/`share_class` | (generated endpoint) fundamentals earning-reports/share-class/share-class-profile | |
| 40 | Bars / OHLCV | Python `bars` | (generated endpoint) market bars | |
| 41 | Delayed quote | Python `delayed_quote` | (generated endpoint) market delayed-quote | |
| 42 | Movers (gainers/losers) | Python `movers` | (generated endpoint) market movers | session/marketcap filters |
| 43 | Short interest | docs shortinterest | (generated endpoint) market short-interest | |
| 44 | Logos search / sync | Python `logos` | (generated endpoint) logos search/sync | |
| 45 | Trending tickers + list | docs ticker-trends | (generated endpoint) trends tickers/list | |
| 46 | Earnings-call transcripts + audio | docs earnings-call-transcripts | (generated endpoint) transcripts list/audio | |
| 47 | Offline FTS search | (none — novel) | (behavior in benzinga-pp-cli search) | Framework: FTS5 over synced news/calendars |
| 48 | Delta sync w/ removed reconciliation | (none — novel) | (behavior in benzinga-pp-cli sync) | Framework: `updated` cursors + removed endpoints |
| 49 | SQL over local store | (none — novel) | (behavior in benzinga-pp-cli sql) | Framework: arbitrary SELECT over entities |
| 50 | MCP server | (none exists for Benzinga) | (behavior in benzinga-pp-cli mcp) | First-ever Benzinga MCP (Cloudflare search+execute pattern for >50 tools) |

Every absorbed row maps to a generator-emitted typed endpoint or a framework behavior. Stubs: none.

## Transcendence (only possible with our approach)

Minimum-5 met (6 features). All `hand-code` cross-entity SQLite joins. From the brainstorm subagent (scores >=8/10).

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Overnight watchlist change scan | watch | hand-code | Joins local ratings+news+signals filtered to a ticker set and `updated >` cursor — a multi-entity diff the REST API can't express in one call | Use this for a multi-ticker "what changed on my names since I last looked" diff across ratings+news+signals. Do NOT use it to deep-dive one ticker's intraday move — use 'why'; for upcoming events use 'catalysts'. |
| 2 | Single-ticker move explainer | why | hand-code | Merges local option_activity+block_trade+halt_resume+ratings+news for one symbol into one time-ordered timeline | Use this to assemble one ticker's catalyst timeline. Do NOT use it for a watchlist sweep (use 'watch') or forward events (use 'catalysts'). |
| 3 | Unified forward catalyst agenda | catalysts | hand-code | Unions local earnings+dividends+splits+ipos+fda+conference-calls+guidance+ma into one forward-dated agenda keyed by (date,ticker) | Use this for upcoming dated events across calendar families. Do NOT use it for past changes (use 'watch') or computed beat/miss (use 'earnings-season'). |
| 4 | Analyst/firm accuracy scorecard | analyst-accuracy | hand-code | Ranks ratings/analysts rows by Benzinga's `ratings_accuracy` field and left-joins today's calendar/ratings to tag each issuer's hit rate | none |
| 5 | Earnings surprise tracker | earnings-season | hand-code | Computes EPS/revenue beat-miss + surprise % from local earnings rows, joins conference-calls + transcript availability, ranked by surprise | Use this for retrospective beat/miss + surprise ranking with linked calls/transcripts. Do NOT use it for the forward schedule — use 'catalysts'. |
| 6 | Clustered insider/congress buying | insider-cluster | hand-code | Groups local insider-transactions + congress-trades by ticker, flags symbols with >=N distinct buyers in the window | none |
