# Screener.in Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Company search by name/ticker | screenercli (company search) | screener-pp-cli company search | Live JSON autocomplete, --json, offline FTS via sync |
| 2 | Key metrics (Market Cap, CMP, P/E, Book Value, Div Yield, ROCE, ROE) | screenercli key-metrics | screener-pp-cli company profile | Structured parse, --select, --json |
| 3 | Pros/cons analysis | screenercli pros-cons | screener-pp-cli company profile (analysis section) | Machine-generated pros/cons, pipeable |
| 4 | Quarterly results | screenercli quarterly-results | (generated endpoint) company profile + qtrend novel | Raw table + computed YOY flags |
| 5 | Profit & Loss | screenercli profit-loss | (generated endpoint) company profile | Annual P&L + growth tables |
| 6 | Balance sheet | screenercli balance-sheet | (generated endpoint) company profile | Annual BS |
| 7 | Cash flow | screenercli cash-flow | (generated endpoint) company profile | Annual CF + FCF |
| 8 | Ratios | screenercli ratios | (generated endpoint) company profile | Operating ratios |
| 9 | Shareholding | screenercli shareholding | (generated endpoint) company profile | Promoter/FII/DII/Public quarterly |
| 10 | Peer comparison | screenercli peer-comparison | screener-pp-cli company peers | Ranked peers table, --json |
| 11 | Chart data | screener-ai-tool (price history) | screener-pp-cli company chart | Price/DMA50/DMA200/Volume JSON |
| 12 | Screens list/browse | screener-ai-tool (screens) | screener-pp-cli screens list | All screens, explore browse |
| 13 | Run a screen | screener-ai-tool | screener-pp-cli screens run | Ranked result table, pagination |
| 14 | Sector/market browse | MaticAlgos/screener-scraper | screener-pp-cli market sector | Sector tree, company lists |
| 15 | IPO calendar | screener-ai-tool | screener-pp-cli ipo list | Upcoming IPOs + subscription |
| 16 | Latest results (auth) | MaticAlgos (logged-in) | screener-pp-cli results latest | YOY growth cards + filters |
| 17 | Insider trades (auth) | MaticAlgos (logged-in) | screener-pp-cli trades insiders | Bought/sold/ESOP rows + filters |
| 18 | Filings hub (auth) | screener-ai-tool (documents) | screener-pp-cli filings list | Bulk/block/SAST/insider links |
| 19 | Full-text search (auth) | screener-ai-tool | screener-pp-cli full-text-search search | Auth-gated FTS |
| 20 | Consolidated/standalone toggle | screenercli --view | screener-pp-cli company profile / profile-standalone | Both views explicit |
| 21 | Offline local mirror | none (all competitors refetch) | screener-pp-cli sync + search + sql | SQLite mirror, offline FTS, composable SQL |
| 22 | Auth via browser session | none | screener-pp-cli auth login --chrome | Cookie-based login for auth-gated pages |
| 23 | Agent-native output | screenercli JSON | screener-pp-cli --agent --select --compact | Structured, pipeable, jq-composable |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Company compare | compare "TCS" "HDFCBANK" | 8/10 | hand-code | Joins synced company_financials + trades tables in local SQLite into a side-by-side fundamentals table; no single API call returns cross-company comparison | Brief Top Workflow #4 (compare peers); no competitor offers cross-company side-by-side | Use this command to compare two or more companies' current fundamentals side by side. Do NOT use it for a single company's quarter-over-quarter trend; use 'qtrend' instead. |
| 2 | Quarterly trend | qtrend "TCS" --quarters 8 | 8/10 | hand-code | Computes YOY change, margin drift, and consecutive-decline/acceleration flags from the synced quarterly-results table, which the raw HTML table does not expose | Brief API Identity (quarterly financials) + Top Workflow #1; quarterly-with-YOY is Screener.in's signature pattern | Use this command for a single company's multi-quarter profit/sales trend. Do NOT use it to compare different companies side by side; use 'compare' instead. |
| 3 | Screen overlap | overlap "magic-formula" "bull-cartel" | 7/10 | hand-code | Fetches each screen's result table and intersects by symbol, emitting standard columns for the intersection | Brief Top Workflow #2 (screen tables); Priya's manual Excel dedup ritual | Use this command to find companies that appear in multiple screens. Do NOT use it to re-score a single screen; use 'rank' instead. |
| 4 | Screen rank | rank "bull-cartel" --by insider | 7/10 | hand-code | Joins synced screens + trades + cached fundamentals to re-score a screen with a composite of screen columns plus insider net flow | Brief Data Layer (screens + trades); agent-shaped --json serves LLM users | Use this command to score the companies inside a single screen with a composite of screen columns and insider-trade flow. Do NOT use it to intersect two screens; use 'overlap' instead. |
| 5 | Insider flow | insider-flow --since 30d --top 10 | 7/10 | hand-code | Aggregates the synced trades table into net per-company buy/sell flows with totals and distinct-insider counts; the trades endpoint returns chronological rows, not flows | Brief API Identity (insider trades) + Top Workflow #3; "who is net buying most" unanswerable from raw list | Use this command for pure insider-trade aggregation over a period. Do NOT use it to score a screen that happens to include an insider component; use 'rank' instead. |
