# World Bank CLI — Absorb Manifest

## Sources absorbed
- **wbgapi** (tgherzog) — canonical modern Python wrapper; full surface
- **wbdata** (OliverSherouse) — pandas search + fetch
- **world_bank_data** (mwouts) — WDI explorer
- **anshumax/world_bank_mcp_server** + **worldbank/data360-mcp** — MCP shims (list countries, list indicators, analyze for country+range)

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Fetch indicator data (country+date) | wbgapi `data.fetch` | (behavior in world-bank-pp-cli data) envelope-parsed observations | Offline cache, --json/--select/--csv, typed exit codes |
| 2 | Multi-country query (;-joined) | wbgapi `economy` list | (behavior in world-bank-pp-cli data) accepts USA;CHN;IND or all | Single call, scriptable |
| 3 | Most-recent-value (mrv) | wbgapi `mrv=` | (generated endpoint) data --mrv | Same, plus gapfill |
| 4 | List indicators | wbgapi `series.info` | (generated endpoint) indicators list | Mirrors to SQLite for offline search |
| 5 | Indicator metadata | wbgapi `series.metadata` | (generated endpoint) indicators get | --select fields, agent-native |
| 6 | List/get economies | wbgapi `economy` | (generated endpoint) countries list / countries get | Region/income/lending filters |
| 7 | List sources | wbgapi `source` | (generated endpoint) sources list | offline |
| 8 | List topics + topic indicators | wbgapi `topic` | (generated endpoint) topics list / topics indicators | offline |
| 9 | List regions / income / lending | wbgapi `region`/`income`/`lending` | (generated endpoint) regions/income-levels/lending-types list | offline |
| 10 | Local sync + FTS search | (none — wrappers are live-only) | (behavior in world-bank-pp-cli sync + search) | Nobody else persists the catalog |

## Transcendence (only possible with our offline-store + SQL approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Offline indicator search | `indicators find <query>` | hand-code | FTS over the ~16k-indicator catalog in local SQLite; wrappers require live calls per search | Use to discover indicator codes by keyword offline. Do NOT use for fetching values; use 'data'. |
| 2 | Cross-country compare | `compare <indicator> <c1;c2;..>` | hand-code | Local join of observations across countries into one aligned table + delta vs baseline | Use to line up one indicator across countries. Do NOT use for one country's history; use 'trend'. |
| 3 | Trend / growth stats | `trend <country> <indicator>` | hand-code | CAGR, YoY %, min/max/latest computed locally over the full series | Use for one country's trajectory on one indicator. |
| 4 | Rank countries | `rank <indicator> --year Y --top N` | hand-code | Rank all economies for an indicator+year, filter by region/income — a cross-entity local join no wrapper exposes as a command | Use to find leaders/laggards on an indicator. Filter with --region/--income. |
| 5 | Pipeline export | `export <pairs> --wide` | hand-code | Bulk country×indicator pull emitted as pipeline-ready wide or long CSV in one command | Use for data-pipeline extracts feeding notebooks/sheets. |
| 6 | Revision diff | `data diff <country> <indicator>` | hand-code | Compares a fresh pull against the last synced snapshot to surface revised observations — needs local history | Use to detect World Bank data revisions between vintages. |

Hand-code count: 6 transcendence rows (all `hand-code`). Plus the envelope-aware `data` and `sync`/`search` store behavior.
No stubs.
