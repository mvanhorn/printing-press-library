# Algolia Novel Features Brainstorm (Subagent Audit Trail)

## Customer model

**Mara Chen — search relevance engineer, e-commerce (apparel retailer, ~1.2M SKUs).**
- **Today:** Lives in the Algolia dashboard: Rules tab, Synonyms tab, Query Preview side by side, re-running top-20 search terms after every merchandising change. Exports index settings to JSON and eyeballs diffs. Cannot answer "which of my 400 rules is actually doing anything."
- **Weekly ritual:** The relevance sprint: harvest zero-result queries, add synonyms, promote/pin rules, re-verify top-20 queries.
- **Frustration:** Dead-weight rules and silent relevance regressions.

**Diego Alvarez — catalog/platform operator, media streaming service (movies, 25k+ records per index).**
- **Today:** Re-runs browse-and-count shell snippets after every nightly catalog import, spot-checks records, reconciles dev vs prod by exporting config and diffing by hand.
- **Weekly ritual:** Post-import verification: confirm record counts, spot-check records, confirm dev mirrors prod, keep versioned config exports.
- **Frustration:** Discovery lag — records missing required attributes are unreachable by search and nothing flags them; dev-vs-prod parity checking is manual.

**Priya Nair — platform/SRE operator, SaaS vendor running three Algolia applications.**
- **Today:** Rotates integration API keys on a schedule via dashboard/curl, keeps a spreadsheet of key ownership and rotation dates, polls the task endpoint during reindex/import runs, scrolls raw log dumps for error triage.
- **Weekly ritual:** Key rotation + ACL audit, 24h log scan for error spikes, confirmation that long-running tasks completed.
- **Frustration:** Key hygiene (write-ACL keys linger unused for months with no report) and log wall has no shape.

## Candidates (pre-cut)

16 candidates; 4 cut inline (marked CUT). Sources: (a) persona, (b) service content pattern, (c) cross-entity local query, (e) user vision.

1. **Cross-index search** — `find --query "dune" --limit 20` — (a),(b),(c),(e) — KEEP
2. **Settings diff** — `settings diff <index-a> <index-b>` — (a),(b),(c),(e) — KEEP
3. **Stale rules report** — `rules stale --index <idx>` — (a),(b),(c),(e) — KEEP
4. **Duplicate rules detection** — `rules duplicates --index <idx>` — (b),(c) — KEEP then cut (low confidence)
5. **API key ACL report** — `apikeys report` — (a),(c),(e) — KEEP
6. **Unused keys report** — `apikeys unused --since 90d` — (a),(c) — KEEP then cut (subsumed by report)
7. **Search regression check** — `search check --index <idx> --query "dune" --expect tt1160419,tt1524930` — (a),(b) — KEEP
8. **Count matching records** — `count --index <idx> --filters "rating>=4"` — (b) — KEEP then cut (thin wrapper)
9. **Unsearchable-records report** — `objects gaps --index <idx>` — (a),(b),(c) — KEEP
10. **Records diff between indices** — `objects diff <index-a> <index-b>` — (a),(c) — KEEP
11. **Object location** — `objects where <objectID>` — (c) — CUT (subsumed by find)
12. **Logs error digest** — `logs errors --since 24h` — (a),(b),(c) — KEEP
13. **Sync freshness report** — `sync status` — (c) — CUT (framework-owned)
14. **Settings drift check** — `settings drift --index <idx>` — (b),(c) — CUT (overlaps settings diff)
15. **Oversized-records check** — `objects oversize --index <idx>` — (b) — CUT (belongs in objects import)
16. **Index activity ranking** — `logs activity --since 7d` — (b),(c) — KEEP then cut (speculative, overlaps logs errors)

## Survivors and kills

### Survivors
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Cross-index search | `find --query <q> --limit <n>` | 8/10 | hand-code | Scans local SQLite records tables of every synced index via FTS and merges hits with source-index labels; drain-first | Thesis "search across all indices"; absorbed search is per-index only | Searches across every synced index's records in one shot. Use 'search' for a single index or resource; 'find' spans all indices and names the source index per row |
| 2 | Settings diff | `settings diff <index-a> <index-b>` | 8/10 | hand-code | Joins two indices' synced settings snapshots, emits field-level delta | Thesis "diff settings between two indices"; settings get/list/set/import have no comparison | Field-level comparison of two indices' settings (or a settings file vs an index). Use 'settings get' to inspect one index; 'settings diff' only reports what changed |
| 3 | Stale rules report | `rules stale --index <idx>` | 8/10 | hand-code | Cross-joins local rules with index settings to flag rules referencing missing attributes + disabled/never-match | Thesis "find stale rules"; rules browse/search never cross-check settings | Lists rules referencing attributes missing from searchable attributes or that can never match. Use 'rules browse' to see all rules; 'rules stale' only surfaces problematic ones |
| 4 | API key ACL report | `apikeys report` | 7/10 | hand-code | Groups local api_keys by ACL/restriction: write-capable, unrestricted, expired, last-use from logs | Thesis "key ACL report"; apikeys list/get have no audit | Audits all API keys: ACLs, restrictions, expiry. Use 'apikeys list' for a plain listing; 'apikeys report' flags write-capable and unrestricted keys |
| 5 | Search regression check | `search check --index <idx> --query <q> --expect <objectIDs>` | 7/10 | hand-code | Calls live search endpoint, asserts expected objectIDs present, typed exit 0/1 + pass/fail table | Top Workflow #1; User Vision demands live dogfood; verifiable on movie dataset | Asserts that a query returns an expected set of objectIDs and exits 0/1 (CI-friendly). Use 'search' to inspect hit lists; 'search check' only emits pass/fail verdicts |
| 6 | Unsearchable-records report | `objects gaps --index <idx>` | 6/10 | hand-code | Joins local records with searchableAttributes to list records where required attributes are null/empty | Data profile 25k+ records; Diego's discovery-lag frustration | Lists records missing attributes required by the index's settings. Use 'objects browse' to inspect records; 'objects gaps' only surfaces incomplete ones |
| 7 | Records diff between indices | `objects diff <index-a> <index-b>` | 6/10 | hand-code | Joins two indices' records tables on objectID, computes added/removed/changed | Top Workflow #2; prod/staging drift concerns | Compares the records of two indices (added/removed/changed by objectID). Use 'objects browse' to inspect one index's records; 'objects diff' only emits the delta |
| 8 | Logs error digest | `logs errors --since 24h` | 5/10 | hand-code | Aggregates synced log entries by answer code/error type, joins failed taskIDs to tasks | Logs is primary entity; absorbed logs get is raw only | Aggregates synced log entries by error code and links failed tasks. Use 'logs get' for raw entries; 'logs errors' emits a summary digest |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Duplicate rules detection | No evidence of demand, speculative pain, weak dogfood signal | rules stale |
| Unused keys report | Subsumed by apikeys report (same join) | apikeys report |
| Count matching records | Thin wrapper: search with hitsPerPage=0 already yields nbHits | search check |
| Object location | Indistinguishable from find on a one-index account | find |
| Sync freshness report | Framework-owned (sync + hint helpers) | none |
| Settings drift check | Synced snapshot equals live at sync time; marginal value | settings diff |
| Oversized-records check | Belongs in objects import validation; weak ritual | objects gaps |
| Index activity ranking | Speculative without Analytics product; overlaps logs errors | logs errors |
