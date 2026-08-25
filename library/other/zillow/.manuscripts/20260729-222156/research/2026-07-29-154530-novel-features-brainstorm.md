## Customer model

- Mortgage professional
  - Today: Builds borrower and referral-partner market briefs from current values, rents, inventory, heat, affordability, and YoY movement.
  - Weekly ritual: Refreshes target metros/ZIPs before borrower and realtor calls.
  - Frustration: Raw homeowner-income-needed data does not show client-specific surplus or shortfall.

- Buyer agent
  - Today: Compares pricing, supply, demand, speed, and forecast direction for relocating clients.
  - Weekly ritual: Rechecks candidate regions as new monthly files arrive.
  - Frustration: Single-metric rankings hide markets where signals conflict.

- Homebuyer or residential investor
  - Today: Screens metros for value/rent divergence, demand, inventory normalization, and affordability.
  - Weekly ritual: Updates shortlist before deeper property research.
  - Frustration: Must manually join ZHVI, ZORI, inventory, and sales spreadsheets.

- Housing analyst
  - Today: Maintains repeatable regional time-series datasets and release histories.
  - Weekly ritual: Syncs releases, checks coverage, and investigates revisions.
  - Frustration: Wide CSVs obscure missing months, coverage loss, and questionable discontinuities.

## Candidates (pre-cut)

Store rule for every local/computed hand-code candidate: drain query rows before follow-up queries. Never call pooled Upsert inside an open write transaction.

| # | Name | Command | One-line description | Persona | Source | Long Description | Kill/keep check |
|---|------|---------|----------------------|---------|--------|------------------|-----------------|
| 1 | Client affordability gap | `affordability gap REGION --income USD` | Compares explicit household income with latest homeowner-income-needed value, returning dollar and percentage margin. | Mortgage professional, homebuyer | (a), (b) | none | Keep: mechanical, testable arithmetic over official data; no external service. `// pp:data-source computed` |
| 2 | Rate-shock affordability | `affordability shock REGION --income USD --rate-base PCT --rate-alt PCT` | Shows income/payment sensitivity between two user-supplied mortgage rates. | Mortgage professional | (a) | Use only for rate sensitivity, not current published affordability. | Pre-cut concern: overlaps absorbed `mortgage` math and candidate 1; user pain insufficiently distinct. `// pp:data-source computed` |
| 3 | Rent/value yield proxy | `yield-proxy REGION` | Computes annualized ZORI divided by ZHVI, alongside rent-growth and value-growth spread. | Investor | (a), (b) | none | Keep: two official series, explicit formula, deterministic output; label as gross proxy, never cap rate. `// pp:data-source computed` |
| 4 | Supply absorption ratio | `supply-ratio REGION` | Divides for-sale inventory by sales-count nowcast to estimate inventory relative to monthly sales flow. | Buyer agent, investor | (a), (b) | none | Keep: interpretable cross-dataset leverage; testable with fixture rows; call ratio, not authoritative months-of-supply. `// pp:data-source computed` |
| 5 | Market turning points | `turning-points REGION` | Finds dated temperature-index crossings and slope reversals in inventory, days-to-pending, and ZHVI. | Buyer agent, investor | (a), (c) | Use for dated mechanical crossings and slope reversals, not narrative forecasts. | Keep: mechanical rules, no NLP or external dependency; thresholds exposed in output. `// pp:data-source computed` |
| 6 | Constraint screener | `screen --metric EXPR --metric EXPR` | Filters regions satisfying multiple metric thresholds on a common observation date. | Buyer agent, investor | (a), (c) | Use for hard pass/fail constraints; use weighted shortlist for tradeoffs. | Passes technical checks, but duplicates local SQL/search ergonomics and loses to candidate 7. `// pp:data-source local` |
| 7 | Weighted regional shortlist | `shortlist --regions IDS --weight METRIC=WEIGHT` | Produces transparent normalized composite scores from user-selected Zillow metrics and weights. | Buyer agent, investor | (a), (c) | Use only when multiple explicit metric weights must produce one composite ordering; not for raw observations. | Keep: mechanical and explainable; requires no default investment thesis; emit component contributions. `// pp:data-source computed` |
| 8 | Sync quality audit | `quality audit --metric METRIC` | Reports missing months, duplicates, null runs, coverage loss, and extreme month-over-month jumps. | Housing analyst | (a), (b) | none | Keep: local-data health command, deterministic fixtures, no domain model beyond synced observations. `// pp:data-source local` |
| 9 | Forecast backtest | `forecast backtest --since DATE` | Scores prior ZHVF vintages against later realized ZHVI observations. | Housing analyst, investor | (a), (b) | none | Kill risk: current public files do not guarantee historical forecast vintages; cannot dogfood until many snapshots accumulate. `// pp:data-source local` |
| 10 | Market breadth | `breadth METRIC --group-by FIELD` | Shows percentage of regions rising, falling, or unchanged plus median change by state or geography type. | Housing analyst, mortgage professional | (a), (c) | none | Keep: cross-region aggregation unavailable from one raw CSV row; deterministic local query. `// pp:data-source computed` |
| 11 | Lead-lag explorer | `lead-lag REGION --driver METRIC --target METRIC --max-lag MONTHS` | Tests lagged correlations between supply, demand, rent, and value series. | Housing analyst, investor | (a), (c) | none | Low-confidence: statistically easy to misread, needs domain-heavy QA, and weekly actionability weak. `// pp:data-source computed` |
| 12 | Peer percentile benchmark | `benchmark REGION --metric METRIC` | Places one region in state and geography-type percentiles for selected metrics. | Buyer agent, mortgage professional | (a), (c) | none | Buildable, but absorbed `rank` and `compare` already recover result; weak transcendence. `// pp:data-source computed` |
| 13 | Market regime label | `regime REGION` | Assigns hot, cooling, balanced, or heating labels from several latest indicators. | Buyer agent, homebuyer | (a), (b) | none | Kill risk: composite thresholds become arbitrary product opinion; turning-point evidence is more transparent. `// pp:data-source computed` |
| 14 | Revision volatility | `revision-volatility --metric METRIC` | Scores datasets by frequency and magnitude of revised historical cells across releases. | Housing analyst | (a), (b) | none | Buildable only after stored releases accumulate; overlaps absorbed `watch` and candidate 8. `// pp:data-source computed` |

## Survivors and kills

### Survivors

1. Client affordability gap — Weekly use: borrower and referral calls. Wrapper-vs-leverage: combines official homeowner-income-needed data with client input. Transcendence: converts regional statistic into explicit surplus/shortfall. Sibling kill: rate-shock variant duplicates mortgage math. Buildability: one latest-row query plus arithmetic. Long description: `none` valid; command scope is distinct.
2. Rent/value yield proxy — Weekly use: investor metro screening. Wrapper-vs-leverage: joins ZORI and ZHVI rather than exposing either series unchanged. Transcendence: reveals rent/value divergence directly. Sibling kill: lead-lag explorer adds statistical ambiguity without clearer action. Buildability: common-date local join and formula. Long description: `none` valid; proxy name and evidence field prevent cap-rate confusion.
3. Supply absorption ratio — Weekly use: agent inventory review. Wrapper-vs-leverage: joins inventory with sales-count nowcast. Transcendence: expresses stock relative to sales flow. Sibling kill: regime label hides comparable evidence behind arbitrary categories. Buildability: common-date local join and division. Long description: `none` valid; output explicitly says ratio, not months-of-supply.
4. Market turning points — Weekly use: relocating-client and investor updates. Wrapper-vs-leverage: scans multiple histories for dated crossings and slope changes. Transcendence: surfaces change events not visible in latest-value output. Sibling kill: lead-lag explorer is harder to verify and explain. Buildability: ordered local series plus deterministic rules. Long description valid because it excludes forecasts.
5. Weighted regional shortlist — Weekly use: repeatable candidate-market screening. Wrapper-vs-leverage: combines multiple region/metric series with explicit user weights. Transcendence: exposes one composite order and every component contribution. Sibling kill: threshold screen is largely SQL shorthand. Buildability: bounded local joins and normalization. Long description valid because it limits use to explicit weighted tradeoffs.
6. Sync quality audit — Weekly use: post-sync dataset validation. Wrapper-vs-leverage: checks temporal and regional integrity across stored observations. Transcendence: detects defects ordinary freshness and revision checks miss. Sibling kill: forecast backtest and revision-volatility require accumulated vintages. Buildability: deterministic local SQL checks. Long description: `none` valid; scope is dataset integrity.
7. Market breadth — Weekly use: analyst and mortgage-partner market pulse. Wrapper-vs-leverage: aggregates changes across every region in a peer group. Transcendence: measures participation behind headline appreciation. Sibling kill: peer percentile benchmark is mostly another view of absorbed ranking. Buildability: grouped local aggregation. Long description: `none` valid; breadth output is distinct.

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Client affordability gap | `affordability gap REGION --income USD` | 9/10 | hand-code | This uses latest homeowner-income-needed CSV observation plus explicit household income to compute dollar and percentage margin with no external dependencies. | Mortgage-professional borrower-brief workflow; official homeowner-income-needed endpoint family | none |
| 2 | Rent/value yield proxy | `yield-proxy REGION` | 8/10 | hand-code | This joins common-date ZORI and ZHVI observations in local SQLite to compute annualized rent/value ratio and growth spread with no external dependencies. | Investor value/rent-divergence workflow; official ZORI and ZHVI endpoint families | none |
| 3 | Supply absorption ratio | `supply-ratio REGION` | 8/10 | hand-code | This joins common-date for-sale inventory and sales-count-nowcast observations to compute inventory divided by monthly sales flow with no external dependencies. | Agent supply/demand workflow; official inventory and sales-count-nowcast endpoint families | none |
| 4 | Market turning points | `turning-points REGION` | 8/10 | hand-code | This scans local market-temperature, inventory, days-to-pending, and ZHVI series for threshold crossings and slope-sign reversals with no external dependencies. | Buyer-agent forecast-direction workflow; official temperature, inventory, pending-time, and ZHVI datasets | Use for dated mechanical crossings and slope reversals, not narrative forecasts. |
| 5 | Weighted regional shortlist | `shortlist --regions IDS --weight METRIC=WEIGHT` | 9/10 | hand-code | This joins selected regions and metrics on common dates in local SQLite, normalizes them, and applies explicit user weights with no external dependencies. | Investor shortlist workflow; buyer-agent multi-region comparison need; Redfin comparison and ranking evidence | Use only when multiple explicit metric weights must produce one composite ordering; not for raw observations. |
| 6 | Sync quality audit | `quality audit --metric METRIC` | 8/10 | hand-code | This scans local release and observation tables for missing months, duplicates, null runs, coverage changes, and configurable jump thresholds with no external dependencies. | Analyst repeatable-dataset workflow; wide monthly CSV schema; release/revision tracking requirement | none |
| 7 | Market breadth | `breadth METRIC --group-by FIELD` | 7/10 | hand-code | This groups latest local observations by state or geography type and computes rising, falling, unchanged, and median-change statistics with no external dependencies. | Mortgage market-brief workflow; analyst regional dataset workflow; official multi-region CSV structure | none |

### Killed candidates

| feature | kill reason | closest-surviving-sibling |
|---------|-------------|---------------------------|
| Rate-shock affordability | Duplicates absorbed mortgage calculation and adds little beyond explicit current-income gap. | Client affordability gap |
| Constraint screener | Local SQL, search, and ranking already cover threshold filters; weighted scoring better handles real tradeoffs. | Weighted regional shortlist |
| Forecast backtest | Cannot verify from current files without archived forecast vintages accumulated over time. | Sync quality audit |
| Lead-lag explorer | Correlation output is easy to misinterpret, domain-heavy to validate, and weak as weekly decision support. | Market turning points |
| Peer percentile benchmark | Mostly reformats absorbed rank/compare output without new cross-entity leverage. | Market breadth |
| Market regime label | Arbitrary composite thresholds conceal evidence and risk product-authored market advice. | Market turning points |
| Revision volatility | Requires substantial release history and overlaps absorbed revision watch; broader integrity audit ships immediately. | Sync quality audit |
