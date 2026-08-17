# Browserbase CLI — Absorb Manifest

Run: 20260813-205859-c0b56b36 · API: Browserbase · First print

## Absorbed (match or beat everything that exists)

### Sessions
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Create session | SDK sessions.create | browserbase-pp-cli sessions create | Flags for projectId, keepAlive, proxies, region, timeout, browserSettings, allowedDomains, userMetadata; --json; --dry-run |
| 2 | List sessions | SDK sessions.list | browserbase-pp-cli sessions list | Pagination, filters, --json/--select, offline cache |
| 3 | Get session | SDK sessions.retrieve | browserbase-pp-cli sessions get | Typed output, live status |
| 4 | Update session (REQUEST_RELEASE) | SDK sessions.update | browserbase-pp-cli sessions update | Explicit release flag; --dry-run |
| 5 | Stop/kill session | browse CLI + pain point #2 | browserbase-pp-cli sessions stop | Explicit stop; solves orphaned keepAlive sessions |
| 6 | Session live debug URLs | SDK sessions.debug | browserbase-pp-cli sessions debug | debuggerUrl/fullscreen/wsUrl/pages |
| 7 | Session logs | SDK sessions.logs.list | browserbase-pp-cli sessions logs | Structured logs, --json |
| 8 | Session recording (rrweb) | SDK sessions.recording.retrieve | browserbase-pp-cli sessions recording | Download/URL |
| 9 | Recording MP4 downloads | SDK recording.downloads.create/list | browserbase-pp-cli sessions recordings | Async MP4 per page |
| 10 | Session replay (HLS) | SDK sessions.replays.retrieve/retrievePage | browserbase-pp-cli sessions replay | .m3u8 playback URLs |
| 11 | Upload file to session | SDK sessions.uploads.create | browserbase-pp-cli sessions upload | Multipart upload |
| 12 | List downloads | SDK sessions.downloads.list | browserbase-pp-cli downloads list | Checksum filters |
| 13 | Get download | SDK | browserbase-pp-cli downloads get | Direct file access |
| 14 | Delete download | SDK | browserbase-pp-cli downloads delete | Cleanup |

### Contexts / Extensions / Certificates
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 15 | Create context | SDK contexts.create | browserbase-pp-cli contexts create | Persisted browser state |
| 16 | Get context | SDK contexts.retrieve | browserbase-pp-cli contexts get | |
| 17 | Delete context | SDK contexts.delete | browserbase-pp-cli contexts delete | |
| 18 | Upload extension | SDK extensions.create | browserbase-pp-cli extensions upload | |
| 19 | Get/list/delete extension | SDK | browserbase-pp-cli extensions get/list/delete | |
| 20 | Upload certificate | SDK certificates.create | browserbase-pp-cli certificates upload | |
| 21 | List/get/delete certificate | SDK | browserbase-pp-cli certificates list/get/delete | |

### Projects / Usage
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 22 | List projects | SDK projects.list | browserbase-pp-cli projects list | Offline cache |
| 23 | Get project | SDK projects.retrieve | browserbase-pp-cli projects get | |
| 24 | Project usage | SDK projects.usage | browserbase-pp-cli projects usage | browserMinutes/proxyBytes |

### Fetch / Search
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 25 | Web search | SDK search.web | browserbase-pp-cli search web | numResults 1-25; rate-limit aware |
| 26 | Fetch page raw/markdown/json | SDK fetchAPI.create | browserbase-pp-cli fetch | format raw/markdown/json+schema |

### Agents / Agent Runs
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 27 | Create agent | SDK agents.create | browserbase-pp-cli agents create | systemPrompt + resultSchema |
| 28 | List/get/update/delete agent | SDK | browserbase-pp-cli agents list/get/update/delete | |
| 29 | Run agent | SDK agents.runs.create | browserbase-pp-cli agents run | |
| 30 | Agent run list/get/messages | SDK agents.runs.list/get/listMessages | browserbase-pp-cli agents runs list/get/messages | |
| 31 | Stop agent run | SDK + pain point | browserbase-pp-cli agents runs stop | Runaway run control |

### Functions
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 32 | List functions | SDK | browserbase-pp-cli functions list | |
| 33 | Function versions/invocations/logs/builds | SDK | browserbase-pp-cli functions versions/invocations/logs/builds | |
| 34 | Invoke function | SDK | browserbase-pp-cli functions invoke | |

### Competitive surfaces
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 35 | MCP navigate/act/observe/extract | MCP server | (generated endpoint) sessions | Agent-native equivalents via fetch/search |
| 36 | DOM snapshot/click/fill (browse CLI) | browse CLI | (stub - requires live CDP session; sessions run prints connectUrl) | Deferred: CDP interaction needs a resident browser transport |
| 37 | Named session daemon (browse CLI) | browse CLI | (behavior in browserbase-pp-cli sessions run) --session alias | Explicit named sessions |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Orphaned-session sweeper | `sessions orphans --older-than 15m` | 10/10 | hand-code | `// pp:data-source local`; scans synced sessions for running+old, sums runtime, optional `--stop` batch release via sessions stop | GitHub pain #1 (keepAlive orphans), no kill in Python SDK | Use this command to find running sessions that were never released (keepAlive orphans) and the runtime they're burning. Do NOT use it for an overall status breakdown of all sessions; use 'projects digest' instead. |
| 2 | Safe session runner | `sessions run --project <id> --timeout 15m` | 10/10 | hand-code | `// pp:data-source live`; POST /v1/sessions create, print connectUrl, guaranteed REQUEST_RELEASE on SIGINT/timeout/completion | GitHub pain #1/#2, Top Workflow #1 | Use this command when you want a session created, used, and guaranteed-stopped in a single invocation. Do NOT use it to find sessions that were already abandoned; use 'sessions orphans' instead. |
| 3 | Batch fetcher | `fetch batch --file urls.txt --format markdown --resume` | 9/10 | hand-code | `// pp:data-source auto`; loops /v1/fetch with 5 req/sec pacing, resumable checkpoint in SQLite | Rate limit doc, Top Workflow #3, browse CLI lacks paced batch | Use this command when you have a list of URLs to fetch right now with rate-limit pacing and resumable progress. Do NOT use it to look back at what was already fetched; use 'web history' instead. |
| 4 | Project digest | `projects digest --project <name> --since 7d` | 8/10 | hand-code | `// pp:data-source local`; joins synced sessions + agent runs + downloads by projectId/day | Build Priorities #8, Top Workflow #5 | Use this command for a per-project weekly activity report (sessions, agent runs, downloads). Do NOT use it for usage-quota or cost metrics; use 'usage trend' instead. |
| 5 | Agent-run diff | `agents runs diff <runA> <runB> --agent <name>` | 7/10 | hand-code | `// pp:data-source auto`; syncs run messages, unified diff of message sequences + final results locally | Build Priorities #8, Dev's prompt-iteration ritual | Use this command to compare two specific runs of an agent. Do NOT use it for weekly aggregation of agent-run activity; use 'projects digest' instead. |
| 6 | Usage trend | `usage trend --project <name> --since 30d` | 6/10 | hand-code | `// pp:data-source local`; accumulates /v1/projects/{id}/usage snapshots on sync, renders deltas/trend | Build Priorities #8, Data Layer, Marcus's Friday ritual | Use this command to see how project usage quotas have changed over sync history. Do NOT use it for activity summaries; use 'projects digest' instead. |
| 7 | Web activity history | `web history --since 7d --type fetch` | 6/10 | hand-code | `// pp:data-source local`; reads local fetch/search cache table, can re-emit cached results | Build Priorities #8, Rhea's provenance gap | Use this command to review past fetch and search activity and re-run cached results. Do NOT use it to fetch a fresh list of URLs; use 'fetch batch' instead. |

**Stubs (explicit):** Row 36 (DOM snapshot/click/fill) ships as `(stub)` — requires a live CDP session transport, out of scope for a replayable HTTP CLI; `sessions run` prints the connectUrl so users can attach their own tooling.

## Killed candidates (audit trail)
| Feature | Kill reason | Sibling |
|---------|------------|---------|
| sessions health | overlaps digest + orphans; dashboard = scope creep | projects digest |
| agents runs stale | same scan as sessions orphans on another resource | sessions orphans |
| sessions watch | background monitor = scope creep; tail --follow exists | sessions run |
| agents runs latest | thin wrapper over runs get + messages | agents runs diff |
| usage cost | needs pricing table not in spec (external service) | usage trend |
| search history | redundant with web history | web history |
| search burst | redundant with fetch batch | fetch batch |
| downloads pull | loop over downloads get, no leverage | projects digest |
| recordings index | speculative, niche | projects digest |
