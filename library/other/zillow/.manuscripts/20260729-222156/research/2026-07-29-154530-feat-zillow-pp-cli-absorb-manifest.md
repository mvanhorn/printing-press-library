# Zillow CLI Absorb Manifest

## Product Boundary

Build `zillow-pp-cli` around official Zillow Research downloads and optional, separately authorized Bridge datasets.

Do not automate Zillow.com, replay hidden listing endpoints, solve/bypass CAPTCHA, import clearance cookies, or ship stealth/proxy scraping. Zillow's current Terms of Use prohibit automated queries and access-control bypass.

## Ecosystem Sources

| Source | State | What it contributes |
|---|---|---|
| Zillow Research housing data | Official, live | ZHVI, ZHVF, ZORI, inventory, sales, days-to-pending, market temperature, affordability CSVs |
| Zillow Group Bridge docs | Official, permissioned | Bearer auth, public records, Zestimates, MLS listings, 200-row normal response cap |
| Printing Press Redfin CLI | Live reference | Sync/watch, trends, compare, summary, ranking, comps, export, SQLite/MCP shape |
| Zillapi skills/MCP | Current third party, paid | Property/listing user workflows; not endpoint authority |
| `sap156/zillow-mcp-server` | Stale contract | Feature ideas only; its Bridge URLs do not match current official docs |
| `node-zillow` | Legacy, last published eight years ago | Historical Zillow workflow inventory only |
| Anthropic official external plugins | Checked | No Zillow plugin found |

## Absorbed Features

| # | Feature | Best source | Planned command | Lane | Notes |
|---|---|---|---|---|---|
| 1 | Dataset catalog and freshness | Zillow Research | `datasets` | official/no-key | Show URL, cadence, ETag/Last-Modified, latest observation |
| 2 | Official CSV download | Zillow Research | `research <metric>` | spec-emits | Eight verified HTTP 200 endpoints |
| 3 | Region resolution | Zillow CSV schema; Redfin | `region resolve` | local | Name, state, type, RegionID |
| 4 | Latest value plus MoM/YoY | Zillow Research | `latest` | local | Common normalized output |
| 5 | Historical series | Zillow Research; Redfin | `trends` | local | One region/metric/window |
| 6 | Multi-region comparison | Redfin; Zillow Market Explorer | `compare` | local | Common-date alignment |
| 7 | Regional market summary | Redfin; Zillow MCP | `summary` | local | Values, rents, supply, speed, heat, affordability |
| 8 | SQLite sync | Redfin; Printing Press framework | `sync` | framework | Wide CSV to normalized observations |
| 9 | Release/revision diff | Redfin watch; Zillow cadence | `watch` | local | New release dates and revised historical cells |
| 10 | Growth/appreciation ranking | Redfin; Zillow ZHVI/ZHVF | `rank` | local | Region ranking with explicit metric/window |
| 11 | Normalized bulk export | Redfin; Zillow CSVs | `export` | local | Long-form CSV/JSON |
| 12 | Offline search/analytics/SQL | Printing Press framework | `search`, `analytics`, `sql` | framework | Agent-friendly local queries |
| 13 | Listing search | Bridge MLS; Zillow/Trulia tools | `bridge listings search` | optional-auth | Only approved datasets |
| 14 | Address/ZPID property lookup | Bridge public records; Zillow tools | `bridge property get` | optional-auth | No Zillow.com page scrape |
| 15 | Zestimate/Rent Zestimate | Official Bridge Zestimate | `bridge zestimate get` | optional-auth | No caching unless agreement permits |
| 16 | Assessments and transactions | Official Bridge public records | `bridge records` | optional-auth | Retention follows dataset agreement |
| 17 | Listing price/status history | Bridge MLS snapshots | `bridge history` | optional-auth | Available only when dataset includes history |
| 18 | Property comparison | Redfin; Zillow property tools | `bridge compare` | optional-auth | Field-aligned output |
| 19 | Sold comparables | Redfin; legacy Zillow | `bridge comps` | optional-auth | Requires authorized listing/transaction data |
| 20 | Photos, schools, agent fields | Bridge MLS projection | fields on listing/property output | optional-auth | No separate scraper |
| 21 | Mortgage payment breakdown | Zillow MCP; legacy Zillow | `mortgage` | computed | Explicit inputs; no invented live rate |
| 22 | Saved local watch records | Redfin; Trulia MCP | `watch add/list/run` | local | No Zillow account writes |
| 23 | Safe Zillow page handoff | Website workflow | `open` | computed | Print URL by default; `--launch` opts in |
| 24 | Provenance and attribution | Zillow terms/data page | every command | cross-cutting | Source URL, release date, "Data Provided by Zillow Group" |

## Transcendence

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Client affordability gap | `affordability gap REGION --income USD` | 9/10 | hand-code | This uses latest homeowner-income-needed CSV observation plus explicit household income to compute dollar and percentage margin with no external dependencies. | Mortgage-professional borrower-brief workflow; official homeowner-income-needed endpoint family | none |
| 2 | Rent/value yield proxy | `yield-proxy REGION` | 8/10 | hand-code | This joins common-date ZORI and ZHVI observations in local SQLite to compute annualized rent/value ratio and growth spread with no external dependencies. | Investor value/rent-divergence workflow; official ZORI and ZHVI endpoint families | none |
| 3 | Supply absorption ratio | `supply-ratio REGION` | 8/10 | hand-code | This joins common-date for-sale inventory and sales-count-nowcast observations to compute inventory divided by monthly sales flow with no external dependencies. | Agent supply/demand workflow; official inventory and sales-count-nowcast endpoint families | none |
| 4 | Market turning points | `turning-points REGION` | 8/10 | hand-code | This scans local market-temperature, inventory, days-to-pending, and ZHVI series for threshold crossings and slope-sign reversals with no external dependencies. | Buyer-agent forecast-direction workflow; official temperature, inventory, pending-time, and ZHVI datasets | Use for dated mechanical crossings and slope reversals, not narrative forecasts. |
| 5 | Weighted regional shortlist | `shortlist --regions IDS --weight METRIC=WEIGHT` | 9/10 | hand-code | This joins selected regions and metrics on common dates in local SQLite, normalizes them, and applies explicit user weights with no external dependencies. | Investor shortlist workflow; buyer-agent multi-region comparison need; Redfin comparison and ranking evidence | Use only when multiple explicit metric weights must produce one composite ordering; not for raw observations. |
| 6 | Sync quality audit | `quality audit --metric METRIC` | 8/10 | hand-code | This scans local release and observation tables for missing months, duplicates, null runs, coverage changes, and configurable jump thresholds with no external dependencies. | Analyst repeatable-dataset workflow; wide monthly CSV schema; release/revision tracking requirement | none |
| 7 | Market breadth | `breadth METRIC --group-by FIELD` | 7/10 | hand-code | This groups latest local observations by state or geography type and computes rising, falling, unchanged, and median-change statistics with no external dependencies. | Mortgage market-brief workflow; analyst regional dataset workflow; official multi-region CSV structure | none |
| 8 | Buy versus rent break-even | `buy-vs-rent REGION --down-payment PCT --years N` | 9/10 | hand-code | This joins common-date ZHVI, ZORI, and total-monthly-payment observations, then applies explicit holding-period, transaction-cost, maintenance, and appreciation assumptions to find the first ownership break-even year. | Buyer decision workflow; official ZHVI, ZORI, and total-monthly-payment datasets | Scenario model, not individualized financial advice. Every assumption remains visible in output. |
| 9 | Negotiation leverage | `negotiation REGION` | 9/10 | hand-code | This aligns price-cut share, mean sale-to-list ratio, days-to-pending, and inventory, then emits each component and an explainable buyer-leverage score. | Buyer-agent offer workflow; official price-cut, sale-to-list, days-to-pending, and inventory datasets | Score is descriptive and formula-backed, not a prediction. |
| 10 | Price-tier spread | `tier-spread REGION` | 8/10 | hand-code | This compares bottom-, middle-, and top-tier ZHVI growth on common dates to expose entry-level compression or luxury divergence. | First-time-buyer and move-up-buyer workflow; official tiered ZHVI datasets | none |
| 11 | Demand pressure | `demand-pressure REGION` | 9/10 | hand-code | This combines ZORDI, inventory, sales flow, days-to-pending, and market temperature into a component-visible demand/supply pressure readout. | Rental and purchase market workflow; official ZORDI and for-sale market datasets | Score is descriptive and formula-backed, not a forecast. |
| 12 | New-construction gap | `new-build-gap REGION` | 8/10 | hand-code | This compares new-construction price, price per square foot, and sales count with the broader market series available locally. | Builder, buyer, and mortgage market workflow; official Zillow new-construction datasets | Coverage starts later than core ZHVI and varies by geography. |
| 13 | Client-ready market brief | `client-brief REGION` | 10/10 | hand-code | This composes affordability, demand, negotiation, supply, value/rent, freshness, and source evidence into deterministic Markdown or JSON without an LLM dependency. | Mortgage and relocation client workflow; all official local datasets | No unsupported narrative claims; missing sections remain explicit. |
| 14 | Formula and provenance explainer | `explain COMMAND` | 9/10 | hand-code | This prints formula, required datasets, assumptions, freshness rules, caveats, and source URLs for every compound command. | Compliance, analyst reproducibility, and recruiter-review workflow | none |

## Killed Candidates

| Feature | Reason |
|---|---|
| Rate-shock affordability | Duplicates mortgage calculation |
| Constraint screener | Local SQL/rank already covers it |
| Forecast backtest | Historical forecast vintages unavailable until snapshots accumulate |
| Lead-lag explorer | Easy to misinterpret; weak weekly actionability |
| Peer percentile benchmark | Mostly another rank view |
| Market regime label | Arbitrary product-authored thresholds |
| Revision volatility | Overlaps release watch and quality audit |

## Build Commitment

- 24 absorbed features across official Zillow Research, Printing Press framework, local workflows, and optional Bridge.
- 14 novel features survived scoring.
- All 14 novel features require hand-written Go after generation.
- 17 official Zillow Research CSV endpoints were live-validated for the approved core.
- Optional Bridge group also requires API-specific hand-written code and remains disabled unless `BRIDGE_ACCESS_TOKEN` is configured.
- No scraping dependency, resident browser, proxy service, CAPTCHA handling, or unapproved paid key.

## Gate Recommendation

Approved by Hunter on 2026-07-29. Proceed with official/no-key core. Keep Bridge group optional and visibly gated. Do not upload, push, publish, or open a PR without a separate review and explicit approval.
