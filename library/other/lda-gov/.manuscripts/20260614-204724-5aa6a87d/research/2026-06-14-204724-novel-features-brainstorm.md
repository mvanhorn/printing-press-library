# Novel Features Brainstorm

## Customer model

**Maya Chen, investigative reporter**

Today: Maya searches lobbying filings around companies, issues, bills, and government entities when a story breaks.

Weekly ritual: She checks new quarterly activity, amendments, terminations, and source links before drafting claims.

Frustration: The official API returns useful records, but the evidence trail is split across nested filings, entity IDs, print pages, and repeated CSV cleanup.

**Luis Ortega, civic-tech researcher**

Today: Luis syncs LDA data into local stores, normalizes names, and runs SQL or CSV analysis across years.

Weekly ritual: He refreshes filings, contribution reports, registrants, clients, lobbyists, and constants without tripping anonymous rate limits.

Frustration: Every notebook reimplements pagination limits, name normalization, relationship joins, and stale-data checks.

**Priya Raman, watchdog analyst**

Today: Priya monitors new registrations, LD-203 contribution reports, duplicate-risk filings, amendments, terminations, and client/registrant conflicts.

Weekly ritual: After filing deadlines, she compares the newest quarter against prior quarters and flags suspicious changes.

Frustration: The highest-value checks are mechanical, but they live in ad hoc scripts instead of one reproducible command.

**Sam Brooks, compliance policy staffer**

Today: Sam resolves official registrant, client, and lobbyist IDs before citing LDA data in internal or public reports.

Weekly ritual: He verifies exact source records, handles name variants, and confirms that cited entities are the official ones.

Frustration: Names collide, suffixes differ, entities appear across multiple resource types, and manual source verification is slow.

## Candidates (pre-cut)

The candidate list was generated from persona frustrations, LDA-specific content patterns, and cross-entity local joins. The cut retained seven hand-code features and killed weak wrappers or unverifiable inferences.

## Survivors and kills

### Survivors

| Feature | Command | Score | Persona served | Buildability | Why Only We Can Do This | Long Description |
|---|---|---:|---|---|---|---|
| Filing Anomaly Audit | `lda-gov-pp-cli audit filings --agent` | 10/10 | Priya Raman | hand-code | The official API exposes records separately; the local PP store can join them and run repeatable quality checks across resources. | Use this for local risk flags across synced filings; use `reports quarter` for period-level activity summaries. |
| Entity Resolve Dossier | `lda-gov-pp-cli entities resolve "Boeing" --agent` | 10/10 | Sam Brooks | hand-code | PP has the synced multi-resource index; the raw API requires separate calls and manual reconciliation. | Use this for one-name official ID disambiguation; use `graph export` for bulk relationship edges. |
| Lobbying Spend Timeline | `lda-gov-pp-cli audit spend --csv` | 9/10 | Maya Chen | hand-code | The API returns filings, not ready-to-compare longitudinal spend tables. | Use this for LD-1/LD-2 spend over time; use `contributions totals` for LD-203 contribution items. |
| Relationship Graph Export | `lda-gov-pp-cli graph export --csv` | 9/10 | Luis Ortega | hand-code | PP owns the normalized local relationship store created from all official endpoints. | Use this for bulk network edges; use `entities resolve` for a single ambiguous name. |
| Contribution Counterparty Totals | `lda-gov-pp-cli contributions totals --csv` | 9/10 | Priya Raman | hand-code | The local store can flatten and aggregate nested item arrays across reports while preserving official source IDs. | Use this for LD-203 item totals; use `audit spend` for lobbying income/expense filings. |
| Quarterly Activity Report | `lda-gov-pp-cli reports quarter --year 2024 --period year_end --agent` | 9/10 | Maya Chen | hand-code | PP can combine locally synced resources safely after rate-limited collection. | Use this for quarter snapshots; use `audit filings` for row-level flags and `audit spend` for longitudinal totals. |
| Covered Positions Map | `lda-gov-pp-cli lobbyists covered-positions --csv` | 8/10 | Maya Chen | hand-code | The local CLI can join covered positions to active filing relationships without external revolving-door data. | Use this for former-office exposure; use `entities resolve` for name disambiguation. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Filing Change Poll | Too close to `sync --latest-only` and filings list; the useful part is better expressed as quarter stats or anomaly flags. | Quarterly Activity Report |
| Contribution Item Ledger | Raw row-per-item flattening is already close to absorbed contribution list/get behavior; aggregation is the differentiated layer. | Contribution Counterparty Totals |
| Amendment Chain | Amendment lineage may require inference from grouped fields rather than explicit API relationships, making correctness hard to verify. | Filing Anomaly Audit |
| Evidence Packet | Mostly wraps absorbed get/source URL behavior and does not add enough cross-entity leverage. | Entity Resolve Dossier |
| Deadline Completeness Check | Expected quarterly filing obligations are not fully derivable from the API, so missing findings would be speculative. | Quarterly Activity Report |
| Name Normalize Helper | Too small and mostly covered by constants plus entity resolution. | Entity Resolve Dossier |
| Issue/Government Footprint | Narrow slice of the same local data covered more flexibly by quarter reports, spend timelines, and graph export. | Quarterly Activity Report |
