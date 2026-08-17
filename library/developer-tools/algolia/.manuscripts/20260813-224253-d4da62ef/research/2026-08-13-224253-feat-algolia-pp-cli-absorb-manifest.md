# Algolia Absorb Manifest

## Absorbed (match or beat everything that exists)

Source: official `@algolia/cli` v5.16.0 (Go binary, npm `@algolia/cli`), official REST API spec (78 ops).

### Indices
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List indices | `@algolia/cli indices list` | algolia-pp-cli indices list | Offline cache, `--json`, `--select` |
| 2 | Get index (exists/attrs) | `@algolia/cli indices` | algolia-pp-cli indices get | `--json`, typed exit |
| 3 | Create index | `@algolia/cli indices` | algolia-pp-cli indices create | `--dry-run` |
| 4 | Delete index | `@algolia/cli indices delete` | algolia-pp-cli indices delete | `--dry-run`, `--yes` |
| 5 | Clear index | `@algolia/cli indices clear` | algolia-pp-cli indices clear | `--dry-run` |
| 6 | Copy index | `@algolia/cli indices copy` | algolia-pp-cli indices copy | `--dry-run`, scope flags |
| 7 | Move index | `@algolia/cli indices move` | algolia-pp-cli indices move | `--dry-run` |
| 8 | Index analyze | `@algolia/cli indices analyze` | algolia-pp-cli indices analyze | local store, `--json` |
| 9 | Export index config | `@algolia/cli indices config export` | algolia-pp-cli indices config export | `--json` file |
| 10 | Import index config | `@algolia/cli indices config import` | algolia-pp-cli indices config import | `--dry-run`, `--yes` |

### Objects (Records)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 11 | Browse objects | `@algolia/cli objects browse` | algolia-pp-cli objects browse | offline FTS, pagination, `--select` |
| 12 | Save objects (create/update) | `@algolia/cli objects import` | algolia-pp-cli objects save | `--dry-run`, batch |
| 13 | Delete objects | `@algolia/cli objects delete` | algolia-pp-cli objects delete | `--dry-run`, `--yes` |
| 14 | Update objects (partial) | `@algolia/cli objects update` | algolia-pp-cli objects update | `--dry-run` |
| 15 | Objects operations (batch) | `@algolia/cli objects operations` | algolia-pp-cli objects operations | `--dry-run` |
| 16 | Import objects from file | `@algolia/cli objects import` | algolia-pp-cli objects import | `--dry-run`, validates |
| 17 | Replace all objects | spec `POST /1/indexes/*/objects` | algolia-pp-cli objects replace-all | `--dry-run` |
| 18 | Delete by query | spec `POST /1/indexes/{indexName}/deleteByQuery` | algolia-pp-cli objects delete-by-query | `--dry-run` |

### Search
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 19 | Search index | `@algolia/cli search` | algolia-pp-cli search | offline fallback, `--json`, `--select` |
| 20 | Search with filters/facets | `@algolia/cli search` | algolia-pp-cli search (flags) | `--filters`, `--facets`, `--attributesToRetrieve` |
| 21 | Browse (scroll all records) | spec `POST /1/indexes/{indexName}/browse` | algolia-pp-cli objects browse | cursor pagination |
| 22 | Multi-queries | spec `POST /1/indexes/*/queries` | algolia-pp-cli search --multi | batch |
| 23 | Search facet values | spec `POST /1/indexes/{indexName}/facets/{facetName}/query` | algolia-pp-cli search facets | targeted |

### API Keys
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 24 | List API keys | `@algolia/cli apikeys list` | algolia-pp-cli apikeys list | offline cache |
| 25 | Get API key | `@algolia/cli apikeys get` | algolia-pp-cli apikeys get | `--json` |
| 26 | Create API key | `@algolia/cli apikeys create` | algolia-pp-cli apikeys create | `--dry-run` |
| 27 | Delete API key | `@algolia/cli apikeys delete` | algolia-pp-cli apikeys delete | `--dry-run`, `--yes` |
| 28 | Rotate API key | `@algolia/cli apikeys rotate` | algolia-pp-cli apikeys rotate | `--dry-run` |
| 29 | Restore API key | spec `POST /1/keys/{key}/restore` | algolia-pp-cli apikeys restore | `--dry-run` |
| 30 | Generate secured API key | spec `POST /1/generateSecuredApiKey` | algolia-pp-cli apikeys secured generate | `--dry-run` |

### Rules
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 31 | Browse rules | `@algolia/cli rules browse` | algolia-pp-cli rules browse | offline FTS |
| 32 | Save rule | spec `PUT /1/indexes/{indexName}/rules/{objectID}` | algolia-pp-cli rules save | `--dry-run` |
| 33 | Delete rule | `@algolia/cli rules delete` | algolia-pp-cli rules delete | `--dry-run`, `--yes` |
| 34 | Import rules from file | `@algolia/cli rules import` | algolia-pp-cli rules import | `--dry-run` |
| 35 | Clear rules | spec `POST /1/indexes/{indexName}/rules/clear` | algolia-pp-cli rules clear | `--dry-run` |
| 36 | Search rules | spec `POST /1/indexes/{indexName}/rules/search` | algolia-pp-cli rules search | offline FTS |

### Synonyms
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 37 | Browse synonyms | `@algolia/cli synonyms browse` | algolia-pp-cli synonyms browse | offline FTS |
| 38 | Save synonym | `@algolia/cli synonyms save` | algolia-pp-cli synonyms save | `--dry-run` |
| 39 | Delete synonym | `@algolia/cli synonyms delete` | algolia-pp-cli synonyms delete | `--dry-run`, `--yes` |
| 40 | Import synonyms | `@algolia/cli synonyms import` | algolia-pp-cli synonyms import | `--dry-run` |
| 41 | Clear synonyms | spec `POST /1/indexes/{indexName}/synonyms/clear` | algolia-pp-cli synonyms clear | `--dry-run` |
| 42 | Search synonyms | spec `POST /1/indexes/{indexName}/synonyms/search` | algolia-pp-cli synonyms search | offline FTS |

### Settings
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 43 | Get settings | `@algolia/cli settings get` | algolia-pp-cli settings get | `--json`, offline cache |
| 44 | Set settings | `@algolia/cli settings set` | algolia-pp-cli settings set | `--dry-run`, file input |
| 45 | List settings | `@algolia/cli settings list` | algolia-pp-cli settings list | offline |
| 46 | Import settings | `@algolia/cli settings import` | algolia-pp-cli settings import | `--dry-run` |

### Dictionaries
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 47 | Browse dictionary entries | `@algolia/cli dictionary entries browse` | algolia-pp-cli dictionary entries browse | offline |
| 48 | Clear dictionary entries | `@algolia/cli dictionary entries clear` | algolia-pp-cli dictionary entries clear | `--dry-run` |
| 49 | Delete dictionary entry | `@algolia/cli dictionary entries delete` | algolia-pp-cli dictionary entries delete | `--dry-run` |
| 50 | Import dictionary entries | `@algolia/cli dictionary entries import` | algolia-pp-cli dictionary entries import | `--dry-run` |
| 51 | Get dictionary settings | `@algolia/cli dictionary settings get` | algolia-pp-cli dictionary settings get | `--json` |
| 52 | Set dictionary languages | `@algolia/cli dictionary settings set languages` | algolia-pp-cli dictionary settings set | `--dry-run` |

### Logs & Tasks
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 53 | Get logs | spec `GET /1/logs` | algolia-pp-cli logs get | `--json`, filters |
| 54 | Wait for task | spec `GET /1/task/{taskID}` | algolia-pp-cli tasks wait | typed exit |
| 55 | Wait for index task | spec `GET /1/indexes/{indexName}/task/{taskID}` | algolia-pp-cli tasks wait-index | typed exit |
| 56 | Index operations (move/copy/clear) | spec `POST /1/indexes/{indexName}/operation` | algolia-pp-cli indices operation | `--dry-run` |

### Other
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 57 | Cluster mapping (users) | spec `GET /1/clusters/mapping` | algolia-pp-cli clusters mapping | `--json` |
| 58 | List security sources | spec `GET /1/security/sources` | algolia-pp-cli security-sources list | `--json` |
| 59 | Append security source | spec `POST /1/security/sources/append` | algolia-pp-cli security-sources append | `--dry-run` |
| 60 | Delete security source | spec `DELETE /1/security/sources/{source}` | algolia-pp-cli security-sources delete | `--dry-run` |
| 61 | Auth login/logout/status | `@algolia/cli auth` | algolia-pp-cli auth login/set-token/status | env + config |
| 62 | Open dashboard | `@algolia/cli open` | algolia-pp-cli open | `--launch` guard |
| 63 | Describe API | `@algolia/cli describe` | algolia-pp-cli describe | `--json` |

## Transcendence (only possible with our approach)

Source: novel-features subagent brainstorm (2026-08-13). All 8 survivors scored >= 5/10, all `hand-code`.

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Cross-index search | find --query <q> --limit <n> | 8/10 | hand-code | Scans local SQLite records tables of every synced index via FTS and merges hits with source-index labels; no single API endpoint spans indices | Searches across every synced index's records in one shot. Use 'search' for a single index or resource; 'find' spans all indices and names the source index per row |
| 2 | Settings diff | settings diff <index-a> <index-b> | 8/10 | hand-code | Joins two indices' synced settings snapshots (or a settings file vs index) and emits field-level delta | Field-level comparison of two indices' settings (or a settings file vs an index). Use 'settings get' to inspect one index; 'settings diff' only reports what changed |
| 3 | Stale rules report | rules stale --index <idx> | 8/10 | hand-code | Cross-joins local rules with index settings to flag rules referencing missing attributes + disabled/never-match | Lists rules referencing attributes missing from searchable attributes or that can never match. Use 'rules browse' to see all rules; 'rules stale' only surfaces problematic ones |
| 4 | API key ACL report | apikeys report | 7/10 | hand-code | Groups local api_keys by ACL/restriction: write-capable, unrestricted, expired, plus last-use from synced logs | Audits all API keys: ACLs, restrictions, expiry. Use 'apikeys list' for a plain listing; 'apikeys report' flags write-capable and unrestricted keys |
| 5 | Search regression check | search check --index <idx> --query <q> --expect <objectIDs> | 7/10 | hand-code | Calls live search endpoint, asserts expected objectIDs present, typed exit 0/1 + pass/fail table | Asserts that a query returns an expected set of objectIDs and exits 0/1 (CI-friendly). Use 'search' to inspect hit lists; 'search check' only emits pass/fail verdicts |
| 6 | Unsearchable-records report | objects gaps --index <idx> | 6/10 | hand-code | Joins local records with searchableAttributes to list records where required attributes are null/empty (records search cannot return) | Lists records missing attributes required by the index's settings. Use 'objects browse' to inspect records; 'objects gaps' only surfaces incomplete ones |
| 7 | Records diff between indices | objects diff <index-a> <index-b> | 6/10 | hand-code | Joins two indices' records tables on objectID, computes added/removed/changed counts plus changed-field samples | Compares the records of two indices (added/removed/changed by objectID). Use 'objects browse' to inspect one index's records; 'objects diff' only emits the delta |
| 8 | Logs error digest | logs errors --since 24h | 5/10 | hand-code | Aggregates synced log entries by answer code/error type, joins failed taskIDs to tasks table | Aggregates synced log entries by error code and links failed tasks. Use 'logs get' for raw entries; 'logs errors' emits a summary digest |

### Killed candidates (from subagent Pass 3)
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

## Stubs
None.
