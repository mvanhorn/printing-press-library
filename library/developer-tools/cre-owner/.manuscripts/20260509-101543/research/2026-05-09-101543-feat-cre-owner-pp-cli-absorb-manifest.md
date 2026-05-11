# CRE Owner CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Property search by market/type/size | PropStream (140+ filters), Crexi search | `search` — filter by market, property type, sqft, price, owner type | Offline SQLite FTS, composable with SQL, works without subscription |
| 2 | Owner of record lookup | Reonomy, PropertyShark | `owner <address/parcel>` — county assessor + Regrid hook | Free via county records, no $thousands/yr subscription |
| 3 | LLC entity search | Reonomy (ML-based), OpenCorporates | `owner --pierce` — OpenCorporates + State SoS | Multi-source entity chain traversal, not single-vendor ML |
| 4 | Officer/registered agent lookup | OpenCorporates, PropertyShark | `owner` subcommand chain | Free via State SoS scraping + OpenCorporates |
| 5 | Active listing details | Crexi, LoopNet, CoStar | `search --source crexi` — Crexi API via cookie auth | Structured JSON output, agent-composable |
| 6 | Sold comps / transaction history | Crexi, CoStar, ATTOM | `comps <address>` — Crexi + county recorder | Free comps via county records, Crexi enrichment |
| 7 | Broker contact info | Crexi, DealMachine (96.5% skip trace) | `owner --contacts` — Crexi broker + enrichment hook | Cross-source confidence scoring |
| 8 | Tax delinquency filtering | PropStream, PropertyShark, BatchLeads | `motivated --signal tax-delinquent` | County assessor scraping, no paid subscription |
| 9 | Parcel boundary/map data | Regrid, PropStream | `parcel <id> --geometry` — county GIS + Regrid hook | Free via county ArcGIS, Regrid hook for nationwide |
| 10 | REIT filing search | SEC EDGAR MCP (stefanoamorelli) | `edgar search <entity>` — SEC EDGAR API | Free, no auth, 10-K/10-Q/8-K filings |
| 11 | Beneficial ownership (Schedule 13D/G) | SEC EDGAR | `edgar ownership <entity>` | Free, institutional holder cross-reference |
| 12 | Business entity status | Indiana INBiz | `entity <name>` — State SoS scraper | Formation date, status, registered agent |
| 13 | Full-text search across all data | N/A (SaaS has limited search) | `search <query>` — FTS5 across all synced entities | Offline, regex-capable, SQL-composable |
| 14 | Local data sync | N/A | `sync` — pull all sources into SQLite | Work offline, avoid re-hitting rate limits |
| 15 | Data export | PropStream CSV export | `--export csv\|json\|xlsx` on all commands | Agent-native JSON by default, human CSV/XLSX |
| 16 | Skip tracing / contact enrichment | DealMachine, BatchLeads | `enrich` — hand off to contact-goat-pp-cli | Cross-CLI composability |
| 17 | Archive/cache dead pages | N/A | `archive <url>` — archive.today + Wayback | Preserve assessor records, dead listing pages |
| 18 | SQL query interface | N/A | `sql <query>` — raw SQL against local mirror | Power-user compound queries |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Hidden portfolio rollup | `portfolio <entity>` | Requires local join across county records + SoS + OpenCorporates to find same beneficial owner across multiple LLCs |
| 2 | Motivated seller scoring | `motivated --market "lake-county-in"` | Requires cross-source signal aggregation: tax delinquency + hold duration + out-of-state LLC + no refinance + code violations |
| 3 | Outreach ranking | `outreach --market "lake-county-in"` | Requires entity chain traversal + contact confidence scoring across Crexi broker + registered agent + enrichment |
| 4 | Portfolio-level distress | `motivated --signal portfolio-distress` | Tax-delinquent parcels owned by entities with 3+ buildings — requires parcel-entity join across county + SoS data |
| 5 | Ownership duration analysis | `motivated --signal long-hold` | Held >15y by out-of-state LLC with no recent refinance — requires deed transfer history + entity jurisdiction |
| 6 | Contact confidence scoring | `owner --contacts --score` | Cross-source confidence: Crexi broker + OpenCorporates agent + SoS registered agent + contact-goat enrichment |
| 7 | Entity chain visualization | `owner --chain` | Visual LLC → officers → other LLCs → beneficial owner chain from multi-source traversal |
| 8 | Market heat map | `market --heat` | Per-submarket aggregation of listings, sales velocity, tax delinquency rate, ownership turnover |
| 9 | Stale listing detection | `search --stale` | Listings on Crexi for 180+ days with price reductions — signals motivated seller |
| 10 | CMBS maturity alerts | `motivated --signal cmbs-maturing` | (Stub — requires Trepp hook) CMBS loans maturing in 12-18mo |
| 11 | Co-Owner Network Discovery | `network --entity "..." --depth 2` | Connects SOS officer/agent graphs across all local entities back to assessor parcel ownership — reveals hidden partnerships |
| 12 | Tax Sale Countdown | `tax-countdown --market "lake-county-in" --within 6mo` | Temporal computation over multi-year delinquency records combined with jurisdiction redemption rules |
| 13 | Dormant Entity Detector | `dormant --market "lake-county-in" --inactive-years 3` | Joins assessor ownership with SOS filing history and entity status — dissolved LLCs still holding property |
| 14 | Comp Gap Analysis | `comp-gap --address "..." --radius 0.5mi` | Joins assessor valuations with Crexi sold comps in geographic radius to find value arbitrage |
| 15 | Portfolio Acquisition Packager | `package --entity "..." --format pdf` | Aggregates assessor, SOS, Crexi, and SEC data into single portfolio dossier for meetings/outreach |
| 16 | Ownership Churn Tracker | `churn --market "lake-county-in" --months 12` | Joins assessor transfer history with entity resolution and listing data across recording periods |
| 17 | Absentee Owner Clusters | `absentee-clusters --market "gary-in"` | Spatial clustering of parcel-vs-mailing address distance joined with entity status |
