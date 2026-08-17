# Parallel.ai Absorb Manifest

## Sources scanned
- Official OpenAPI Product: https://docs.parallel.ai/public-openapi.json (30 paths)
- Official OpenAPI Account: https://api.parallel.ai/account/service/openapi.json
- Official CLI: parallel-cli / parallel-web-tools (https://github.com/parallel-web/parallel-web-tools)
- Community CLI: @rikalabs/parallel (https://github.com/Rika-Labs/parallel)
- Search MCP + Task MCP (docs.parallel.ai)
- Agent skills: parallel-web/parallel-agent-skills
- Official SDKs: parallel-web (Python/TS)

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Web search with objective + queries | parallel-cli search; Search MCP | parallel-pp-cli search | Local store + FTS; session_id continuity; --json/--agent |
| 2 | Extract URL content | parallel-cli extract; @rikalabs/parallel | parallel-pp-cli extract | Offline recall; batch URLs; session stitch |
| 3 | Deep research task run | parallel-cli research; Task MCP createDeepResearch | (generated endpoint) tasks runs create | Poll/stream/result; local run index |
| 4 | Task group batch enrichment | parallel-cli enrich; Task MCP createTaskGroup | (generated endpoint) tasks groups | Local group tracking + FTS |
| 5 | Poll/status/result for runs | parallel-cli research status/poll; Task MCP getStatus/getResultMarkdown | parallel-pp-cli tasks result | Unified run watch + store |
| 6 | FindAll entity discovery | parallel-cli findall | (generated endpoint) findall runs | Local candidate cache |
| 7 | Fast entity search | parallel-cli findall entity-search | (generated endpoint) findall entity-search | Offline re-query of prior hits |
| 8 | Monitor create/list/events | parallel-cli monitor | (generated endpoint) monitors | Local event digests |
| 9 | Chat completions (beta) | OpenAPI / SDKs | (generated endpoint) chat completions | Agent --json |
| 10 | Auth via API key env | parallel-cli / all tools | (behavior in parallel-pp-cli doctor) PARALLEL_API_KEY | Dual-auth clarity in doctor |
| 11 | Device OAuth login | parallel-cli login --device | parallel-pp-cli auth login --device | Account API JWT for balance/apps/keys |
| 12 | Get prepaid balance | Account API; parallel-cli-setup | parallel-pp-cli balance get | Local balance snapshots over time |
| 13 | Add to balance | Account API | parallel-pp-cli balance add | Idempotency key helper |
| 14 | List/create/delete apps | Account API | (generated endpoint) account apps | Local app registry |
| 15 | Create/delete API keys | Account API | (generated endpoint) account keys | Never log full key secrets |
| 16 | Domain include/exclude search | parallel-cli search --include-domains | (behavior in parallel-pp-cli search) advanced_settings | Source policy flags |
| 17 | Non-interactive agent JSON | parallel-cli --json; Hermes skill | (behavior in parallel-pp-cli *) --json/--agent | Typed exit codes + store |
| 18 | Follow-up research interaction_id | Task MCP / parallel-cli | (behavior in parallel-pp-cli tasks) | Local interaction graph |
| 19 | Batch stdin concurrency | @rikalabs/parallel | parallel-pp-cli extract --stdin | Worker pool + store |
| 20 | Sync/search local history | Printing Press standard | parallel-pp-cli sync / search | FTS5 over past researches |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description | Score |
|---|---------|---------|--------------|------------------------|------------------|-------|
| 1 | Research session stitch | session stitch | hand-code | Local multi-entity session graph across search/extract/tasks/findall | Use this command to attach already-completed Product calls into one local session. Do NOT use it to start a new deep research run; use 'tasks runs create' instead. Do NOT use it to walk a single task follow-up chain; use 'tasks lineage' instead. | 9/10 |
| 2 | Balance-aware run cost guard | tasks guard | hand-code | Cross-auth: Account balance JWT + Product task create gate | Use this command to gate expensive Task creates on prepaid balance. Do NOT use it to inspect historical burn; use 'balance burn' instead. | 8/10 |
| 3 | Stale monitor event digest | monitors digest | hand-code | Local aggregation of synced monitor_events | Use this command for a mechanical local digest of synced monitor events. Do NOT use it to create or list monitors; use generated monitors commands instead. | 8/10 |
| 4 | Cross-run research recall | research recall | hand-code | Multi-table FTS across searches/extracts/task summaries | Use this command to find prior local research across searches/extracts/runs. Do NOT use it for live web search; use 'search' instead. | 8/10 |
| 5 | FindAll → Task Group promote | findall promote | hand-code | Compose local FindAll candidates into Product Task Group create | Use this command to turn FindAll candidates into a Task Group. Do NOT use it for one-off deep research on a free-text objective; use 'tasks runs create' instead. | 7/10 |
| 6 | Prepaid burn vs runs | balance burn | hand-code | Diff balance_snapshots joined to local run counts | Use this command for historical credit burn vs local run volume. Do NOT use it to block a new run; use 'tasks guard' instead. | 7/10 |
| 7 | Task interaction lineage | tasks lineage | hand-code | Offline previous_interaction_id graph from local store | Use this command to show the local follow-up chain for a task run. Do NOT use it to poll live status; use 'tasks status' / 'tasks result' instead. | 6/10 |

## Stubs
None. All transcendence rows are shipping-scope hand-code. Account live smoke requires OAuth JWT (not the Product API key).

## Priority check
- Primary parallel-product-api: search, extract, tasks, findall, monitors, chat + most transcendence
- Secondary parallel-account-api: balance get/add, apps, keys + tasks guard / balance burn helpers
- No inversion: Product has the majority command surface
