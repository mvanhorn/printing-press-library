# Novel Features Brainstorm — Browserbase CLI

Subagent output (Step 1.5c.5), first print, no prior research. 7 survivors, 9 killed.

## Customer model
1. **Rhea Alvarez — Data-collector engineer**: scheduled scraping pipelines via fetch/search; no provenance/history, no dedup, rate-limit pacing is manual.
2. **Dev Okonkwo — AI browsing-agent builder**: create→connect→stop loop; orphaned keepAlive sessions burn minutes; diffing run outputs is manual.
3. **Priya Raman — QA engineer**: audits failed sessions + recordings/MP4s for regression reports; sessions are islands.
4. **Marcus Lee — Platform owner**: pays the bill; weekly per-project usage review; no trend/breakdown, manual screen-scraping.

## Survivors (>=5/10)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Orphaned-session sweeper | `sessions orphans --older-than 15m` | 10/10 | hand-code | local scan of synced sessions for running+old, sum runtime, optional batch release via sessions stop | GitHub pain #1 keepAlive orphans; no kill in Python SDK | Use this command to find running sessions that were never released (keepAlive orphans). Do NOT use it for overall status; use 'projects digest' instead. |
| 2 | Safe session runner | `sessions run --project <id> --timeout 15m` | 10/10 | hand-code | POST /v1/sessions create, print connectUrl, guaranteed REQUEST_RELEASE on SIGINT/timeout/completion | GitHub pain #1/#2; Top Workflow #1 | Use when you want a session created, used, and guaranteed-stopped in one invocation. Do NOT use it to find abandoned sessions; use 'sessions orphans' instead. |
| 3 | Batch fetcher | `fetch batch --file urls.txt --format markdown --resume` | 9/10 | hand-code | loops /v1/fetch with 5 req/sec pacing, resumable checkpoint in SQLite | Rate limit doc; Top Workflow #3; browse CLI lacks paced batch | Use when you have a list of URLs to fetch now with pacing + resume. Do NOT use it to look back; use 'web history' instead. |
| 4 | Project digest | `projects digest --project <name> --since 7d` | 8/10 | hand-code | joins sessions + agent runs + downloads by projectId/day locally | Brief Build Priorities #8; Top Workflow #5 | Use for per-project weekly activity report. Do NOT use for usage-quota metrics; use 'usage trend' instead. |
| 5 | Agent-run diff | `agents runs diff <runA> <runB> --agent <name>` | 7/10 | hand-code | syncs run messages, unified diff of message sequences + final results locally | Build Priorities #8; Dev's prompt-iteration ritual | Use to compare two specific runs of an agent. Do NOT use for weekly aggregation; use 'projects digest' instead. |
| 6 | Usage trend | `usage trend --project <name> --since 30d` | 6/10 | hand-code | accumulates /v1/projects/{id}/usage snapshots on sync, renders deltas/trend | Build Priorities #8; Data Layer; Marcus's Friday ritual | Use to see how project usage changed over sync history. Do NOT use for activity summaries; use 'projects digest' instead. |
| 7 | Web activity history | `web history --since 7d --type fetch` | 6/10 | hand-code | reads local fetch/search cache table, can re-emit cached results | Build Priorities #8; Rhea's provenance gap | Use to review past fetch/search activity. Do NOT use to fetch fresh URLs; use 'fetch batch' instead. |

## Killed candidates
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
