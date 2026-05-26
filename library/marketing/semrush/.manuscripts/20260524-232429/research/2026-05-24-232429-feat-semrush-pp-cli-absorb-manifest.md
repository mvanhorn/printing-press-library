# Semrush CLI Absorb Manifest

## Scope
v3 Analytics + v3 Projects (Standard API). Single API-key query-param auth.
Trends + v4 (Map Rank Tracker, Listing Management, Projects v4) are explicitly
out of scope by user gate decision (see brief).

## Absorbed (match or beat everything that exists)

### Account / utility
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Check API units balance | mrkooblu `semrush units`; osodevops `account balance` | `semrush-pp-cli account balance` | Free probe; integrated into `doctor` for pre-flight + last-N-call rolling estimate from local store |
| 2 | Shell completions (bash/zsh/fish) | osodevops `completions` | `(behavior in semrush-pp-cli completion)` | Printing Press emits across all 4 shells by default |
| 3 | Auto-detect keyword vs domain | mrkooblu `semrush q <input>` | `semrush-pp-cli q <input>` | Same auto-detect + falls through to `semrush sql` when the input looks like a SQL query against the local store |
| 4 | Cost preview before request | osodevops `--dry-run` | `(behavior in --dry-run on every list/get)` | Estimates units, prints expected row count, warns if budget < N% threshold |
| 5 | Cache clear / cache stats | osodevops `cache clear/stats` | `semrush-pp-cli cache clear`, `semrush-pp-cli cache stats` | Same, but backed by SQLite TTL freshness instead of a SHA256 file cache; survives across runs |

### Domain Analytics (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 6 | Domain Overview (one database) | mrkooblu `semrush_domain_rank`; osodevops `domain overview`; official `type=domain_rank` | `(generated endpoint) domain overview` | Persists every row to `domains` table with `(domain, database, snapshot_date)` key for drift |
| 7 | Domain Overview (all databases) | mrkooblu `semrush_domain_overview`; official `type=domain_ranks` | `(generated endpoint) domain overview-all` | Stored in same `domains` table; FTS5-searchable; cross-domain comparison enabled |
| 8 | Domain Overview (history) | mrkooblu `semrush_domain_rank_history`; official `type=domain_rank_history` | `(generated endpoint) domain overview-history` | Time-series rows persisted; `--since YYYY-MM` flag computes Δ vs current |
| 9 | Domain Organic Search Keywords | mrkooblu `semrush_domain_organic_keywords`; osodevops `domain organic`; official `type=domain_organic` | `(generated endpoint) domain organic-keywords` | Rows persist to `keyword_positions`; supports `--where 'kd<30 and nq>100'` filter shorthand |
| 10 | Domain Paid Search Keywords | mrkooblu `semrush_domain_paid_keywords`; osodevops `domain paid`; official `type=domain_adwords` | `(generated endpoint) domain paid-keywords` | Same persistence; cross-domain paid-vs-organic gap query enabled |
| 11 | Competitors in Organic Search | mrkooblu `semrush_competitors`; osodevops `domain competitors`; official `type=domain_organic_organic` | `(generated endpoint) domain competitors-organic` | Persisted to `competitors`; ranks across multiple databases joinable |
| 12 | Competitors in Paid Search | mrkooblu `semrush_paid_competitors`; official `type=domain_adwords_adwords` | `(generated endpoint) domain competitors-paid` | Same; `--kind organic\|paid\|pla` flag selects across the 3 competitor variants |
| 13 | PLA Competitors | mrkooblu `semrush_domain_shopping`; official `type=domain_shopping_competitors` | `(generated endpoint) domain competitors-pla` | Persisted; cross-tier comparison |
| 14 | Domain Ad History | mrkooblu `semrush_domain_ads_history`; osodevops `domain ad-history`; official `type=domain_adwords_historical` | `(generated endpoint) domain ad-history` | Time-series ad copy archive in `ad_copies`; query "what ads did `<dom>` run between dates" |
| 15 | Domain Ads Copies | osodevops `domain ads-copies`; official `type=domain_adwords_unique` | `(generated endpoint) domain ad-copies` | Persisted; FTS5-searchable across ad title + description |
| 16 | Domain PLA Search Keywords | official `type=domain_shopping` | `(generated endpoint) domain pla-keywords` | Persisted |
| 17 | Domain PLA Copies | official `type=domain_shopping_unique` | `(generated endpoint) domain pla-copies` | Persisted |
| 18 | Domain Organic Pages | mrkooblu `semrush_domain_organic_unique`; osodevops `domain pages`; official `type=domain_organic_pages` | `(generated endpoint) domain organic-pages` | Persisted; `--top N` shows highest-traffic pages |
| 19 | Domain Organic Subdomains | mrkooblu `semrush_domain_organic_unique`; osodevops `domain subdomains`; official `type=domain_organic_subdomains` | `(generated endpoint) domain subdomains` | Persisted |
| 20 | Domain vs Domain | mrkooblu `semrush gaps`; osodevops `domain compare`; official `type=domain_domains` | `semrush-pp-cli domain compare <d1> <d2> [<d3>...]` | Variadic — supports 2–5 domains in one shot, intersection/difference modes |

### Subdomain Analytics (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 21 | Subdomain Overview (one db) | mrkooblu `semrush_subdomain_rank`; official `type=subdomain_rank` | `(generated endpoint) subdomain overview` | Persisted |
| 22 | Subdomain Overview (all db) | mrkooblu `semrush_subdomain_ranks`; official `type=subdomain_ranks` | `(generated endpoint) subdomain overview-all` | Persisted |
| 23 | Subdomain Overview (history) | mrkooblu `semrush_subdomain_rank_history`; official `type=subdomain_rank_history` | `(generated endpoint) subdomain overview-history` | Time-series persisted |
| 24 | Subdomain Organic Search Keywords | mrkooblu `semrush_subdomain_organic`; official `type=subdomain_organic` | `(generated endpoint) subdomain organic-keywords` | Persisted |
| 25 | Subdomain Paid Search Keywords | official `type=subdomain_adwords` | `(generated endpoint) subdomain paid-keywords` | Persisted |
| 26 | Subdomain Organic Pages | official `type=subdomain_organic_pages` | `(generated endpoint) subdomain organic-pages` | Persisted |

### Subfolder Analytics (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 27 | Subfolder Overview (one/all/history) | mrkooblu `semrush_subfolder_*`; official `type=subfolder_rank{,s,_history}` | `(generated endpoint) subfolder overview{,-all,-history}` | Persisted (3 endpoints) |
| 28 | Subfolder Organic / Paid Keywords | mrkooblu `semrush_subfolder_organic`/`semrush_subfolder_adwords`; official `type=subfolder_organic`/`subfolder_adwords` | `(generated endpoint) subfolder {organic,paid}-keywords` | Persisted |
| 29 | Subfolder Organic Pages | official `type=subfolder_organic_pages` | `(generated endpoint) subfolder organic-pages` | Persisted |
| 30 | Subfolder Organic Pages (unique) | mrkooblu `semrush_subfolder_organic_unique` | `(generated endpoint) subfolder organic-pages-unique` | Persisted |

### URL Analytics (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 31 | URL Overview (one/all/history) | mrkooblu `semrush_url_rank{,s,_history}`; official `type=url_rank{,s,_history}` | `(generated endpoint) url overview{,-all,-history}` | Persisted (3 endpoints) |
| 32 | URL Organic Keywords | mrkooblu `semrush_url_organic`; official `type=url_organic` | `(generated endpoint) url organic-keywords` | Persisted |
| 33 | URL Paid Keywords | mrkooblu `semrush_url_adwords`; official `type=url_adwords` | `(generated endpoint) url paid-keywords` | Persisted |

### Keyword Research (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 34 | Keyword Overview (all databases) | mrkooblu `semrush_keyword_overview`; osodevops `keyword overview`; official `type=phrase_all` | `(generated endpoint) keyword overview-all` | Persisted to `keywords`; one CSV-fetched row covers ~140 databases |
| 35 | Keyword Overview (one database) | mrkooblu `semrush_keyword_overview_single_db`; official `type=phrase_this` | `(generated endpoint) keyword overview` | Persisted |
| 36 | Batch Keyword Overview (up to 100) | mrkooblu `semrush_batch_keyword_overview`; osodevops `keyword batch`; official `type=phrase_these` | `(generated endpoint) keyword batch` | Persisted; `--from-file -` reads stdin keyword list |
| 37 | Related Keywords | mrkooblu `semrush_related_keywords`; osodevops `keyword related`; official `type=phrase_related` | `(generated endpoint) keyword related` | Persisted; `--where 'rr>0.7'` for high-relevance only |
| 38 | Broad Match Keywords | mrkooblu `semrush_broad_match_keywords`; osodevops `keyword broad-match`; official `type=phrase_fullsearch` | `(generated endpoint) keyword broad-match` | Persisted |
| 39 | Phrase Questions | mrkooblu `semrush_phrase_questions`; osodevops `keyword questions`; official `type=phrase_questions` | `(generated endpoint) keyword questions` | Persisted; question-mined for content brief generation |
| 40 | Keyword Difficulty | mrkooblu `semrush_keyword_difficulty`; osodevops `keyword difficulty`; official `type=phrase_kdi` | `(generated endpoint) keyword difficulty` | Persisted; supports comma-separated up to 100 |
| 41 | Keyword Organic SERP Results | mrkooblu `semrush_keyword_organic_results`; osodevops `keyword organic`; official `type=phrase_organic` | `(generated endpoint) keyword organic-serp` | Persisted; per-position SERP-feature flags retained |
| 42 | Keyword Paid SERP Results | mrkooblu `semrush_keyword_paid_results`; official `type=phrase_adwords` | `(generated endpoint) keyword paid-serp` | Persisted |
| 43 | Keyword Ads History | mrkooblu `semrush_keyword_ads_history`; official `type=phrase_adwords_historical` | `(generated endpoint) keyword ads-history` | Persisted to `ad_copies` |

### Backlinks (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 44 | Backlinks Overview | mrkooblu `semrush_backlinks_overview`; osodevops `backlink overview`; official `type=backlinks_overview` | `(generated endpoint) backlink overview` | Persisted |
| 45 | Backlinks list | mrkooblu `semrush_backlinks`; osodevops `backlink list`; official `type=backlinks` | `(generated endpoint) backlink list` | Persisted to `backlinks` table |
| 46 | Referring Domains | mrkooblu `semrush_backlinks_domains`; osodevops `backlink referring-domains`; official `type=backlinks_refdomains` | `(generated endpoint) backlink referring-domains` | Persisted to `referring_domains` |
| 47 | Referring IPs | official `type=backlinks_refips` | `(generated endpoint) backlink referring-ips` | Persisted |
| 48 | TLD Distribution | mrkooblu `semrush_backlinks_tld`; official `type=backlinks_tld` | `(generated endpoint) backlink tld` | Persisted |
| 49 | Referring Domains by Country | osodevops `backlink geo`; official `type=backlinks_geo` | `(generated endpoint) backlink geo` | Persisted |
| 50 | Anchors | mrkooblu `semrush_backlinks_anchors`; osodevops `backlink anchors`; official `type=backlinks_anchors` | `(generated endpoint) backlink anchors` | Persisted; FTS5 across anchor text |
| 51 | Indexed Pages | mrkooblu `semrush_backlinks_pages`; official `type=backlinks_pages` | `(generated endpoint) backlink indexed-pages` | Persisted |
| 52 | Backlink Competitors | osodevops `backlink competitors`; official `type=backlinks_competitors` | `(generated endpoint) backlink competitors` | Persisted |
| 53 | Comparison by Referring Domains | official `type=backlinks_matrix` | `(generated endpoint) backlink compare-refdomains` | Persisted; useful for "who shares backlinks with us" |
| 54 | Batch Comparison | official `type=backlinks_comparison` | `(generated endpoint) backlink compare-batch` | Persisted; up to 200 domains in one call |
| 55 | Authority Score profile | official `type=backlinks_ascore_profile` | `(generated endpoint) backlink authority-score` | Persisted |
| 56 | Categories profile | mrkooblu `semrush_backlinks_categories`; official `type=backlinks_categories_profile` | `(generated endpoint) backlink categories-profile` | Persisted |
| 57 | Categories | official `type=backlinks_categories` | `(generated endpoint) backlink categories` | Persisted |
| 58 | Historical data (backlinks) | official `type=backlinks_historical` | `(generated endpoint) backlink history` | Time-series persisted |

### Projects CRUD (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 59 | List projects | mrkooblu `semrush_list_projects`; osodevops `project list`; official `GET /management/v1/projects` | `(generated endpoint) project list` | Persisted to `projects` |
| 60 | Get project | mrkooblu `semrush_get_project`; osodevops `project get`; official `GET /management/v1/projects/{id}` | `(generated endpoint) project get` | Persisted |
| 61 | Create project | mrkooblu `semrush_create_project`; osodevops `project create`; official `POST /management/v1/projects` | `(generated endpoint) project create` | Persisted; `--dry-run` shows the JSON body it would POST |
| 62 | Update project | mrkooblu `semrush_update_project`; osodevops `project update`; official `PUT /management/v1/projects/{id}` | `(generated endpoint) project update` | Persisted |
| 63 | Delete project | mrkooblu `semrush_delete_project`; osodevops `project delete`; official `DELETE /management/v1/projects/{id}` | `(generated endpoint) project delete` | Requires `--yes` to confirm; otherwise dry-runs |

### Site Audit (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 64 | Enable Site Audit on project | official `POST /reports/v1/projects/{id}/siteaudit/enable` | `(generated endpoint) audit enable` | One-time per project; persisted state |
| 65 | Edit Site Audit campaign | official `POST /reports/v1/projects/{id}/siteaudit/edit` | `(generated endpoint) audit edit` | — |
| 66 | List campaign snapshots | mrkooblu `semrush_site_audit_snapshots`; official `GET /reports/v1/projects/{id}/siteaudit/snapshots` | `(generated endpoint) audit snapshots` | Persisted to `audit_snapshots` |
| 67 | Get text descriptions about issues | official `GET /reports/v1/projects/{id}/siteaudit/issues` | `(generated endpoint) audit issue-catalog` | Persisted |
| 68 | Run audit | mrkooblu `semrush_site_audit_launch`; official `POST /reports/v1/projects/{id}/siteaudit/launch` | `(generated endpoint) audit run` | Triggers; pair with `audit snapshots` to poll |
| 69 | Get campaign info | mrkooblu `semrush_site_audit_info`; official `GET /reports/v1/projects/{id}/siteaudit/info` | `(generated endpoint) audit campaign-info` | — |
| 70 | Get snapshot info | mrkooblu `semrush_site_audit_snapshot_detail`; official `GET /reports/v1/projects/{id}/siteaudit/snapshot/{snap_id}` | `(generated endpoint) audit snapshot-info` | Persisted issue counts by severity |
| 71 | Detailed report for issue | mrkooblu `semrush_site_audit_issues`; official `GET /reports/v1/projects/{id}/siteaudit/snapshot/{snap_id}/issue/{issue_id}` | `(generated endpoint) audit issue` | Persisted to `audit_issues` |
| 72 | Get page ID by URL | official `GET /reports/v1/projects/{id}/siteaudit/snapshot/{snap_id}/page` | `(generated endpoint) audit page-by-url` | — |
| 73 | Get page info | mrkooblu `semrush_site_audit_page_detail`; official `GET /reports/v1/projects/{id}/siteaudit/snapshot/{snap_id}/page/{page_id}` | `(generated endpoint) audit page` | Persisted to `audit_pages` |
| 74 | Get snapshots history | mrkooblu `semrush_site_audit_history`; official `GET /reports/v1/projects/{id}/siteaudit/history` | `(generated endpoint) audit history` | Time-series persisted |

### Position Tracking (v3)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 75 | Create tracking campaign | official `POST /reports/v1/projects/{id}/tracking/add` | `(generated endpoint) tracking create` | — |
| 76 | Enable/disable campaign emails | official `POST .../tracking/email/{enable,disable}` | `(generated endpoint) tracking emails {enable,disable}` | Two endpoints, two commands |
| 77 | Add/remove keywords | official `POST .../tracking/keywords/{add,delete}` | `(generated endpoint) tracking keywords {add,remove}` | `--from-file -` reads stdin lists |
| 78 | Add/remove tags | official `POST .../tracking/tags/{add,delete}` | `(generated endpoint) tracking tags {add,remove}` | — |
| 79 | Add/remove competitors | official `POST .../tracking/competitors/{add,delete}` | `(generated endpoint) tracking competitors {add,remove}` | — |
| 80 | List campaigns | official `GET .../tracking/campaigns` | `(generated endpoint) tracking campaigns` | Persisted |
| 81 | Universal location search | official `GET /reports/v1/locations/search` | `(generated endpoint) tracking location-search` | — |
| 82 | Campaign dates | official `GET .../tracking/dates` | `(generated endpoint) tracking dates` | Lets `--snapshot YYYY-MM-DD` flag in other commands target a real snapshot |
| 83 | Organic / AdWords overview report | official `GET .../tracking/{organic,paid}/overview` | `(generated endpoint) tracking {organic,paid}-overview` | Persisted |
| 84 | Organic / AdWords positions report | official `GET .../tracking/{organic,paid}/positions` | `(generated endpoint) tracking {organic,paid}-positions` | Persisted to `tracking_positions` |
| 85 | Organic / AdWords competitors discovery | official `GET .../tracking/{organic,paid}/competitors` | `(generated endpoint) tracking {organic,paid}-competitors` | Persisted |
| 86 | Organic / AdWords visibility index | official `GET .../tracking/{organic,paid}/visibility` | `(generated endpoint) tracking {organic,paid}-visibility` | Persisted |
| 87 | Organic / AdWords landing pages | official `GET .../tracking/{organic,paid}/landings` | `(generated endpoint) tracking {organic,paid}-landings` | Persisted |

### Framework-emitted infra (reused unchanged)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 88 | Local SQLite store + sync | (none of competitors have this) | `(generated endpoint) sync` | Both top competitors lack persistence; this is our base layer |
| 89 | FTS5 search across all synced data | (none) | `semrush-pp-cli search "<term>" [--type keywords\|domains\|backlinks\|...]` | None of the competitors have offline cross-resource search |
| 90 | Raw SQL access for power users | (none) | `semrush-pp-cli sql 'SELECT ...'` | SELECT-only; full schema introspectable via `sql --schema` |
| 91 | Analytics rollups across resources | (none) | `semrush-pp-cli analytics --type keywords --group-by database` | Histogram/aggregation over local store |
| 92 | doctor + auth check + balance + permissions | osodevops `account balance` partial | `semrush-pp-cli doctor` | Runs balance probe, key validity, rate-limit check, store integrity in one call |
| 93 | --json / --select / --csv / --jsonl / --quiet output modes | osodevops parity | `(behavior in every command)` | Auto-detects pipe vs TTY |
| 94 | MCP server (stdio + http) mirroring full Cobra tree | mrkooblu has stdio only | `semrush-pp-mcp` (auto-emitted) | Adds http transport so hosted agents can reach it |
| 95 | Per-source rate limiting (10 RPS) | osodevops token bucket | `(behavior in client.go via cliutil.AdaptiveLimiter)` | Includes `*cliutil.RateLimitError` propagation so agents distinguish throttle from "no data" |

## Transcendence (only possible with our approach)

Survivors from the Phase 1.5c.5 novel-features subagent. Full audit trail
(personas, candidates, killed list) at
`2026-05-24-232429-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Weekly drift report | `semrush-pp-cli drift <domain> --since 7d` | 9/10 | hand-code | Local SQL over two `keyword_positions` + `domains` snapshots; emits gainers/losers/new/lost with Δposition and ΔKD | Brief Top Workflow #3 ("set-difference logic the API can't return natively"); Maya's Monday CSV-diff ritual; both competitors lack persistence |
| 2 | Snapshot tag + diff | `semrush-pp-cli snapshot tag <label>` / `snapshot diff <a> <b>` | 8/10 | hand-code | Writes a `snapshot_labels(label, resource_type, query_signature, taken_at)` row pointing at current store state; `diff` joins two labels' rows for the named resource | Maya's weekly-baseline workflow; brief Build Priority 3 explicitly names `snapshot` as a transcendence command |
| 3 | Backlink delta since last sync | `semrush-pp-cli backlink new <domain> --since 7d` | 8/10 | hand-code | Selects from `referring_domains` and `backlinks` where `first_seen > now() - 7d` | Diego's "did anything new link to us" weekly question; brief Top Workflow #4 |
| 4 | Cost ledger / budget | `semrush-pp-cli budget [--since 30d]` | 9/10 | hand-code | Logs every API call's documented unit cost to `credit_log(ts, command, resource, units, balance_after)`; `budget` rolls up by day/command and projects month-end burn | Brief Reachability Risk; osodevops `--dry-run`; Diego's credit anxiety and Priya's invisible-spend frustration |
| 5 | Keyword gap | `semrush-pp-cli keyword gap <me> <them> [<them2>...]` | 9/10 | hand-code | Set-difference query over `keyword_positions` rows from two-or-more domains in the same `database` column | Brief Top Workflow #3 explicitly; Maya's #1 weekly question |
| 6 | Backlink gap | `semrush-pp-cli backlink gap <me> <them> --min-ascore 70` | 8/10 | hand-code | Left-anti-join over `referring_domains` between two `target_domain` values, filtered by `domain_ascore` | Brief Top Workflow #4 explicitly; Diego's client workflow |
| 7 | Audit triage | `semrush-pp-cli audit triage <project-id>` | 7/10 | hand-code | Joins `audit_issues` × `audit_pages` × generated issue catalog, weights `errors*3 + warnings*1 + notices*0.1`, orders pages by weighted score | Brief Top Workflow #5; absorb rows 67/70/71/73 persist data but nobody aggregates it |
| 8 | Position drift | `semrush-pp-cli tracking drift <project-id> --since 30d` | 7/10 | hand-code | Window function over `tracking_positions` grouped by `(phrase, region, device)`, latest snapshot vs prior, emits movers crossing page-1 / top-3 thresholds | Brief Top Workflow #5; Maya's tracker-email frustration |
| 9 | Country fan-out | `semrush-pp-cli domain regions <d> --databases us,uk,de,fr,it` | 6/10 | hand-code | Sequential fan-out across N `database` values into a new `domain regions` Cobra command (sibling to `domain overview`); all rows persisted with `(domain, database, snapshot_date)` key for later cross-region drift. Renamed from `domain overview --databases` to keep hand-edits out of generator-emitted files. | Brief Data Layer; user's existing `dataforseo`/`serpapi-google-local` use `--source` fan-out pattern |
| 10 | SERP feature monitor | `semrush-pp-cli serp-features <keyword> --since 30d` | 6/10 | hand-code | Local time-series query over the SERP-feature flag columns persisted by `keyword organic-serp`; reports first-seen/last-seen for each feature per keyword | Absorb row 41 persists per-position SERP-feature flags but no command surfaces them as a time series |
| 11 | Cannibalization detector | `semrush-pp-cli cannibalization <domain>` | 7/10 | hand-code | `SELECT phrase, COUNT(DISTINCT ur) FROM keyword_positions WHERE domain=? GROUP BY phrase HAVING COUNT(DISTINCT ur) > 1` | Service-specific SEO content pattern; brief Top Workflow #1 |
| 12 | Audit regression watch | `semrush-pp-cli audit regression <project-id>` | 7/10 | hand-code | Diff latest two `audit_snapshots` rows for a project: new issue_ids, resolved issue_ids, count delta per severity | Brief Top Workflow #5; Maya's tech-SEO mode |

**Hand-code commitment for Phase Gate 1.5:** 12 features × `hand-code` = 12 hand-authored Cobra command files (~80-150 LoC each) + their `root.go` AddCommand wiring + `internal/store/` migrations for the new tables (`credit_log`, `snapshot_labels`). 0 features tagged `spec-emits` (all transcendence is hand-code by definition; spec-emits coverage is in the absorbed table above).

## Stubs
None. Every feature in this manifest is fully implemented.
