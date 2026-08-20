# OpenFDA CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search drug adverse events by drug name, reaction, date range, seriousness | OpenFDA-MCP-Server search_drug_adverse_events | `drug events search --drug <name> --reaction <reaction> --serious` | FTS5 offline search, works without API, SQL composable |
| 2 | Search drug labels by brand/generic name, ingredient, indication | OpenFDA-MCP-Server search_drug_labels | `drug labels search --brand <name> --ingredient <ing>` | Offline, cached locally, full-text across all label sections |
| 3 | Search NDC directory by product code, name, dosage form | OpenFDA-MCP-Server search_drug_ndc | `drug ndc search --name <name> --form <form>` | Offline lookup, cross-reference with adverse events |
| 4 | Search drug recalls by firm, classification, date range | OpenFDA-MCP-Server search_drug_recalls | `drug recalls search --firm <name> --class <1-3>` | Offline, historical trend data |
| 5 | Search Drugs@FDA by sponsor, brand, generic, status | OpenFDA-MCP-Server search_drugs_fda | `drug approvals search --brand <name>` | Offline, cross-reference with adverse events |
| 6 | Search drug shortages by product, status, designation | OpenFDA-MCP-Server search_drug_shortages | `drug shortages search --product <name>` | Offline, historical shortage tracking |
| 7 | Search device 510(k) by device name, applicant, product code | OpenFDA-MCP-Server search_device_510k | `device 510k search --device <name>` | Offline, cross-reference with device events |
| 8 | Search device classifications by name, class, specialty | OpenFDA-MCP-Server search_device_classifications | `device classification search --name <name>` | Offline reference |
| 9 | Search device adverse events by device, manufacturer, event type | OpenFDA-MCP-Server search_device_adverse_events | `device events search --device <name>` | FTS5 offline, historical |
| 10 | Search device recalls by firm, classification, product code | OpenFDA-MCP-Server search_device_recalls | `device recalls search --firm <name>` | Offline, trend tracking |
| 11 | Search food recalls | None — first tool to cover this | `food recalls search --firm <name> --product <desc>` | First CLI to cover food recalls |
| 12 | Search food adverse events (CAERS) | None | `food events search --product <name> --reaction <reaction>` | First CLI to cover CAERS data |
| 13 | Search animal/vet adverse events | None | `animal events search --animal <species> --drug <name>` | First CLI to cover animal safety |
| 14 | Search tobacco problems | None | `tobacco problems search --product <type>` | First CLI to cover tobacco data |
| 15 | Search substance data | None | `substance search --name <name>` | First CLI to cover substance data |
| 16 | Count/aggregate by any field | OpenFDA-MCP-Server (count param) | All endpoints support `count --field <field.exact>` | Cache counts locally for trend comparison |
| 17 | Pagination across all endpoints | OpenFDA-MCP-Server | All list commands support `--limit` and `--skip`, plus `--all` | Auto-pagination beyond single request |
| 18 | Rate-limit-aware requests | OpenFDA-MCP-Server | Built-in adaptive rate limiting with API key support | Auto-backoff, `FDA_API_KEY` env var |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score | Persona | Buildability Proof |
|---|---------|---------|------------------------|-------|---------|-------------------|
| 1 | Drug Signal Tracker | `drug events trend --drug <name> --reaction <reaction> --interval quarter` | Local SQLite time-series aggregation over synced FAERS data; no API endpoint supports temporal bucketing or trend computation | 10/10 | PV analyst, investigative reporter | Uses locally synced adverse_events table, GROUP BY date_bucket(receiptdate) to compute counts per interval |
| 2 | Drug Comparison | `drug events compare --drugs acetaminophen,ibuprofen` | Multi-drug adverse event profile comparison requires parallel local queries and normalized rate computation; no API endpoint compares drugs | 10/10 | PV analyst, investigative reporter | Queries local adverse_events filtered by each drug, computes reaction frequency distributions, normalizes by total reports per drug |
| 3 | Recall-Event Correlation | `drug recalls correlate --drug <name>` | Joins recalls and adverse events by product/firm in local SQLite, computing the event timeline around each recall date | 10/10 | Device compliance manager, investigative reporter | JOINs local recalls and adverse_events on product name/firm, bins events by time relative to recall report_date |
| 4 | Manufacturer Dossier | `manufacturer dossier <firm_name>` | Aggregates across 6+ locally synced tables (drug/device/food recalls, drug/device adverse events, 510k) for a single manufacturer | 8/10 | Device compliance manager, investigative reporter | Queries drug_recalls, device_recalls, food_recalls, drug_events, device_events, device_510k WHERE firm matches |
| 5 | Watchlist Monitor | `watchlist add drug acetaminophen` / `watchlist check` | Maintains local watchlist state and diffs against synced data to surface new events since last check | 9/10 | Investigative reporter, PV analyst | Stores watchlist in SQLite, records last-check timestamp per item, queries synced tables for records newer than last_check |
| 6 | Device Inventory Check | `device check --file inventory.csv` | Batch-checks a CSV of device product codes against locally cached recalls and adverse events | 7/10 | Device compliance manager | Parses CSV, queries local device_recalls and device_events for each code, outputs risk-flagged rows |
| 7 | Reaction Co-occurrence | `drug events cooccur --drug acetaminophen` | Computes which adverse reactions appear together on the same FAERS report; requires report-level grouping impossible via count= API | 8/10 | Public health researcher, PV analyst | Queries adverse_events grouped by safety_report_id WHERE drug matches, computes pairwise reaction co-occurrence |
| 8 | Polypharmacy Check | `drug events polypharmacy --drugs metformin,atorvastatin` | Finds reports where multiple drugs appear as concomitant medications, showing reactions unique to the combination | 7/10 | Public health researcher, PV analyst | Filters reports containing all specified drugs in concomitant fields, computes reaction frequencies unique to combination |

## Killed Candidates

| Feature | Kill Reason | Closest Surviving Sibling |
|---------|-------------|--------------------------|
| Black Box Warning Extractor | Thin field filter; absorbed `drug labels search` covers this with --select | Drug Comparison |
| Shortage Impact Report | Episodic use (shortages are rare); cross-join pattern proven by recall-event correlation | Recall-Event Correlation |
| Recall Velocity | Subsumed by trend pattern; count-over-time on recalls not differentiated from Drug Signal Tracker | Drug Signal Tracker |
| Cross-Category Recall Feed | Simple UNION query; watchlist + dossier cover the use case with more specificity | Manufacturer Dossier |
| FAERS Quarterly Digest | Below-weekly cadence; component pieces (trend, compare, count) compose to same output | Drug Signal Tracker |
| Label Diff | Score 4/10; episodic use, no explicit demand, requires multi-version label sync | No close sibling |
| 510(k) Predicate Chain | Low frequency use; predicate chain field availability uncertain | Device Inventory Check |
| Pipeline Export | Thin formatting wrapper; every CLI has --format csv | Drug Comparison |
