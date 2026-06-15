# LDA.gov Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List and filter LD-1/LD-2 filings | Official OpenAPI `listFilings`; us-gov-open-data MCP `lobbying_search`; R `lobby` | (generated endpoint) filings listFilings | Official endpoint plus JSON/table/CSV/select, local sync, rate-aware defaults |
| 2 | Retrieve a filing by UUID | Official OpenAPI `retrieveFiling`; MCP `lobbying_detail`; Ruby client | (generated endpoint) filings retrieveFiling | Full nested detail with source URL and agent-friendly field selection |
| 3 | List and filter LD-203 contribution reports | Official OpenAPI `listContributionReports`; MCP `lobbying_contributions`; Ruby client | (generated endpoint) contributions listContributionReports | Supports anonymous reads, JSON/CSV, flattened analysis downstream |
| 4 | Retrieve a contribution report by UUID | Official OpenAPI `retrieveContributionReport`; Ruby client | (generated endpoint) contributions retrieveContributionReport | Full contribution item detail with source citation |
| 5 | Search registrants | Official OpenAPI `listRegistrants`; MCP `lobbying_registrants`; R packages | (generated endpoint) registrants listRegistrants | ID lookup, table/JSON output, local FTS |
| 6 | Retrieve registrant detail | Official OpenAPI `retrieveRegistrant`; Ruby client | (generated endpoint) registrants retrieveRegistrant | Source-backed detail and local joins to clients/filings |
| 7 | Search clients | Official OpenAPI `listClients`; R `lobby`; Ruby client | (generated endpoint) clients listClients | Client lookup with local joins and CSV export |
| 8 | Retrieve client detail | Official OpenAPI `retrieveClient`; Ruby client | (generated endpoint) clients retrieveClient | Detail output plus source URL and related filings |
| 9 | Search lobbyists | Official OpenAPI `listLobbyists`; MCP `lobbying_lobbyists`; R `lobby` | (generated endpoint) lobbyists listLobbyists | Name/firm filtering with local relationship joins |
| 10 | Retrieve lobbyist detail | Official OpenAPI `retrieveLobbyist`; Ruby client | (generated endpoint) lobbyists retrieveLobbyist | Detail output and agent-friendly selection |
| 11 | List filing types | Official constants endpoint | (generated endpoint) constants listFilingTypes | Local validation and code display mapping |
| 12 | List lobbying activity issue codes | Official constants endpoint; MCP issue-code enums | (generated endpoint) constants listLobbyingActivityGeneralIssues | Code/name lookup for filters and reports |
| 13 | List government entities | Official constants endpoint | (generated endpoint) constants listGovernmentEntities | Normalized entity names for search and graphing |
| 14 | List countries and states | Official constants endpoints | (generated endpoint) constants listCountries; (generated endpoint) constants listStates | Validation helpers for filters |
| 15 | List lobbyist prefixes/suffixes | Official constants endpoints | (generated endpoint) constants listLobbyistPrefixes; (generated endpoint) constants listLobbyistSuffixes | Normalizes person names |
| 16 | List contribution item types | Official constants endpoint | (generated endpoint) constants listContributionItemTypes | Contribution type display mapping |
| 17 | Optional API-key auth | Official docs and ToS | (behavior in lda-gov-pp-cli auth set-token) Optional `Authorization: Token <key>` support | Anonymous works by default; token boosts rate limit |
| 18 | Rate-limit-safe pagination | Official docs and ToS | (behavior in lda-gov-pp-cli sync) Honors `page_size<=25` and rate limits | Avoids 400 pagination errors and ToS blocks |
| 19 | Local sync/search/SQL | Printing Press framework | lda-gov-pp-cli sync; lda-gov-pp-cli search; lda-gov-pp-cli sql | Offline analysis, FTS, jq-friendly output |
| 20 | Source/citation URLs | Official API fields; MCP source summaries | (generated endpoint) filings retrieveFiling | Evidence links for journalists and analysts |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona served | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|------:|----------------|--------------|------------------------|------------------|
| 1 | Filing Anomaly Audit | `audit filings --agent` | 10/10 | Priya Raman | hand-code | The official API exposes records separately; the local PP store can join them and run repeatable quality checks across resources. | Use this for local risk flags across synced filings; use `reports quarter` for period-level activity summaries. |
| 2 | Entity Resolve Dossier | `entities resolve "Boeing" --agent` | 10/10 | Sam Brooks | hand-code | PP has the synced multi-resource index; the raw API requires separate calls and manual reconciliation. | Use this for one-name official ID disambiguation; use `graph export` for bulk relationship edges. |
| 3 | Lobbying Spend Timeline | `audit spend --csv` | 9/10 | Maya Chen | hand-code | The API returns filings, not ready-to-compare longitudinal spend tables. | Use this for LD-1/LD-2 spend over time; use `contributions totals` for LD-203 contribution items. |
| 4 | Relationship Graph Export | `graph export --csv` | 9/10 | Luis Ortega | hand-code | PP owns the normalized local relationship store created from all official endpoints. | Use this for bulk network edges; use `entities resolve` for a single ambiguous name. |
| 5 | Contribution Counterparty Totals | `contributions totals --csv` | 9/10 | Priya Raman | hand-code | The local store can flatten and aggregate nested item arrays across reports while preserving official source IDs. | Use this for LD-203 item totals; use `audit spend` for lobbying income/expense filings. |
| 6 | Quarterly Activity Report | `reports quarter --year 2024 --period year_end --agent` | 9/10 | Maya Chen | hand-code | PP can combine locally synced resources safely after rate-limited collection. | Use this for quarter snapshots; use `audit filings` for row-level flags and `audit spend` for longitudinal totals. |
| 7 | Covered Positions Map | `lobbyists covered-positions --csv` | 8/10 | Maya Chen | hand-code | The local CLI can join covered positions to active filing relationships without external revolving-door data. | Use this for former-office exposure; use `entities resolve` for name disambiguation. |

## Notes
- Stubs: none.
- Risky dependencies: none beyond the official public API and local SQLite.
- Expensive endpoints: bulk filings and contribution pagination must be sharded by filters and rate-limited, especially anonymous traffic.
