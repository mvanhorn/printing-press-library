## Absorb Manifest (draft for novel-features subagent)

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Web search with objective + queries | parallel-cli search; Search MCP | parallel-pp-cli search | Local store + FTS; session_id continuity; --json/--agent |
| 2 | Extract URL content | parallel-cli extract; @rikalabs/parallel | parallel-pp-cli extract | Offline recall; batch URLs; session stitch |
| 3 | Deep research task run | parallel-cli research; Task MCP createDeepResearch | (generated endpoint) tasks runs create | Poll/stream/result; local run index |
| 4 | Task group batch enrichment | parallel-cli enrich; Task MCP createTaskGroup | (generated endpoint) tasks groups | Local group tracking + FTS |
| 5 | Poll/status/result for runs | parallel-cli research status/poll; Task MCP getStatus/getResultMarkdown | parallel-pp-cli tasks result / tasks status | Unified run watch + store |
| 6 | FindAll entity discovery | parallel-cli findall | (generated endpoint) findall runs | Local candidate cache |
| 7 | Fast entity search | parallel-cli findall entity-search | (generated endpoint) findall entity-search | Offline re-query of prior hits |
| 8 | Monitor create/list/events | parallel-cli monitor | (generated endpoint) monitors | Local event digests |
| 9 | Chat completions (beta) | OpenAPI / SDKs | (generated endpoint) chat completions | Agent --json |
| 10 | Auth via API key env | parallel-cli / all tools | (behavior in parallel-pp-cli doctor) PARALLEL_API_KEY | Dual-auth clarity in doctor |
| 11 | Device OAuth login | parallel-cli login --device | parallel-pp-cli auth login --device | Account API JWT for balance/apps/keys |
| 12 | Get prepaid balance | Account API docs; parallel-cli-setup skill | parallel-pp-cli balance get | Local balance snapshots over time |
| 13 | Add to balance | Account API | parallel-pp-cli balance add | Idempotency key helper |
| 14 | List/create/delete apps | Account API | (generated endpoint) account apps | Local app registry |
| 15 | Create/delete API keys | Account API | (generated endpoint) account keys | Never print full key after create to logs |
| 16 | Domain include/exclude search | parallel-cli search --include-domains | (behavior in parallel-pp-cli search) advanced_settings | Source policy flags |
| 17 | Non-interactive agent JSON | parallel-cli --json; Hermes skill | (behavior in parallel-pp-cli *) --json/--agent | Typed exit codes + store |
| 18 | Follow-up research interaction_id | Task MCP / parallel-cli --previous-interaction-id | (behavior in parallel-pp-cli tasks) | Local interaction graph |
| 19 | Batch stdin concurrency | @rikalabs/parallel | parallel-pp-cli extract --stdin | Worker pool + store |
| 20 | Sync/search local history | Printing Press standard | parallel-pp-cli sync / search --local | FTS5 over past researches |
