# EU TED CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Expert search by query string | fbuchner/ted-mcp (source) | `notices search --query "buyer-country=DEU AND classification-cpv IN (72000000)"` | Full pagination (15K+ via iteration), --json, --select, --csv, composable via pipe |
| 2 | Search by country filter | TenderAlerts, ted-mcp | `notices search --country DEU --cpv 72` | Pre-built query constructor; no DSL knowledge required |
| 3 | Search by CPV code | TenderAlerts, ted-mcp | `notices search --cpv 72000000` | CPV validation + human-readable label alongside code |
| 4 | Search by keyword (full-text) | All competing SaaS tools | `notices search --text "SAP transformation"` | Fuzzy match via FT field, offline FTS5 on synced corpus |
| 5 | Deadline filter | TenderAlerts, Tenderbase | `deadline --days 30 --country DEU` | Composite days-remaining + urgency coloring in terminal |
| 6 | Notice scope selection | tap-eu-ted | `--scope LATEST\|ACTIVE\|ALL` | Documented with clear semantics; defaults to ACTIVE for bid managers |
| 7 | Single notice lookup | ted-mcp (source) | `notices get 123456-2024` | All fields available, --json, human-formatted with clickable URL |
| 8 | Contract award search | ted-mcp, Spend Network | `awards --country DEU --cpv 72 --year 2024` | Winner + buyer + value in one table; --json for pipeline |
| 9 | Pagination (full corpus) | tap-eu-ted (source) | Auto-pagination in all list commands; `--page N`, `--limit N`, `--all` | ITERATION mode for >15K results; progress bar |
| 10 | CSV export | All SaaS tools | `--output csv` flag on all list commands | Also: `--output json`, `--output table`; compatible with duckdb/parquet pipelines |
| 11 | Saved search / watch | TenderAlerts, TenderMetric | `alerts add --query "..." --name "my-alert"` | File-based; runs via cron or `alerts check`; outputs new-since-last-run diff |
| 12 | CPV code lookup | Jorpex, SaaS UI features | `cpv search "software"` / `cpv get 72000000` | Full CPV hierarchy (section, division, group, class); reverse lookup by description |
| 13 | Field discovery | TED API docs only | `fields list --search "buyer"` | Searchable list of all 1,830 queryable fields with descriptions |
| 14 | Incremental sync to local store | tap-eu-ted (source) | `sync --query "..."` | Tracks last-publication-date cursor; resumes safely; progress reporting |
| 15 | Offline SQL query | Nothing (novel approach) | `sql "SELECT * FROM notices WHERE cpv LIKE '72%'"` | Full SQLite access to synced corpus; composable with jq |
| 16 | Offline full-text search | Nothing (novel approach) | `search "cloud migration"` | FTS5 index over synced notice titles + descriptions |
| 17 | Doctor / health check | Printing Press standard | `doctor` | Checks API reachability, store integrity, cursor state |
| 18 | JSON output everywhere | ted-mcp (source) | `--json` flag on all commands | Strict JSON, agent-compatible, exit codes typed |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Win Rate analysis | `win-rate --cpv 72000000 --country FRA` | Requires local join of call-for-tender + award notices by buyer+CPV — no API join semantics | 10 |
| 2 | Opportunity Scorer | `score --keywords "cloud" --country DEU` | Composite deadline urgency + value + keyword match density — requires FTS + local aggregation | 9 |
| 3 | Market Concentration | `concentration --cpv 72000000 --country DEU` | HHI computation across full award corpus — API has no group-by/aggregation endpoint | 9 |
| 4 | Procurement Velocity | `velocity --country DEU --cpv 72000000` | Time-series aggregation across hundreds of notices — impossible in single API call | 9 |
| 5 | Winner Graph | `winner-graph --cpv 72000000 --country FRA` | Bipartite buyer-winner edge list requiring cross-buyer join + fuzzy name dedup | 9 |
| 6 | Dark Buyers detector | `dark-buyers --country POL --cpv 45000000` | Call+award join with award-ratio + winner-diversity metrics — compliance/integrity tool | 8 |
| 7 | CPV Drift analysis | `cpv-drift --country DEU --since 2020-01-01` | Year-over-year CPV volume + value pivot — requires full historical corpus locally | 8 |
| 8 | Buyer Profile | `buyer --name "Bundesagentur für Arbeit"` | Full buyer dossier: cadence, CPV mix, typical value, repeat winner rate — multi-join across history | 8 |
| 9 | Deadline Heat map | `deadline-heat --country DEU --cpv 72 --days 14` | Heat score = urgency × value / competition count — three-table local query | 7 |
| 10 | Peer Benchmark | `peer-benchmark --buyer "Stadt München" --peer-country FRA` | Two-population statistical comparison — requires full corpus on both sides | 8 |

## User Vision Feature (added from briefing)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 11 | Construction Leads | `leads --cpv 45 --country DEU` | Combines award notices + winner + location + project value into a B2B outreach list — API has no lead-generation endpoint; requires filtering can-standard notices and extracting winner-contact pairs | 10 |

**klarx use case:** Find recent construction contract award winners (CPV 45xxxxxx) by country/region. These companies just won large construction projects and will need rented machinery (cranes, containers, scaffolding). Output: winner company name, project location (NUTS region), contract value, construction type, and any available contact touchpoints from the notice. Ready-to-use outreach list.
