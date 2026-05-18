# BrainOS CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List/filter any table | PostgREST clients | Every table gets `<resource> list --filter --limit --json` | Offline via SQLite, typed filters |
| 2 | CRUD operations (get/post/patch/delete) | PostgREST clients | Full CRUD per resource with --dry-run | Idempotent, --dry-run, typed exit codes |
| 3 | Full-text search on thoughts | brain-mcp search_thoughts | `thoughts search <query>` with FTS5 SQLite | Works offline, regex, SQL composable |
| 4 | Capture new thought | brain-mcp capture_thought | `thoughts add --content "..."` | --stdin, --dry-run, batch via JSONL |
| 5 | List thoughts | brain-mcp list_thoughts | `thoughts list --type --topics --limit` | Offline, timestamped, --select fields |
| 6 | Thought stats | brain-mcp thought_stats | `thoughts stats --group-by type` | Type breakdown, topic frequency, time series |
| 7 | Health/doctor check | supabase/cli status | `doctor` — API reachable, auth valid, sync status | Tests actual table access, reports sync lag |
| 8 | SQL passthrough | supabase MCP execute_sql | `sql "<query>"` | Read-only guard, offline vs live modes |
| 9 | List MCP servers | supabase MCP (generic) | `mcp servers list --status --json` | Shows status, tool count, last-seen |
| 10 | Agent message inbox | (none) | `agents messages --unread --sender <name>` | Priority-sorted, --since, read-receipt tracking |
| 11 | Sync data locally | (none) | `sync --table <name> --full` | SQLite mirror for offline queries |
| 12 | Active memory by agent | (none) | `memory list --agent <name> --expiring-soon` | Importance-ranked, expiry alerts |
| 13 | NatureOS task queue | (none) | `agents queue --status pending --agent-type` | Priority-sorted, blocked-task detection |
| 14 | Trading sessions list | (none) | `trading sessions --date --account-mode --json` | Today's session quick-view |
| 15 | Trading premortems | (none) | `trading premortems --ticker --date --json` | Filter by ticker/date/setup |
| 16 | Trading postmortems | (none) | `trading postmortems --ticker --won --json` | Win/loss filter, RR calc |
| 17 | Executor briefs list | (none) | `executor briefs --active --expired` | Shows goal, folder_ids, expiry |
| 18 | Shared state get/set | (none) | `state get <namespace> <key>` / `state set` | Locking awareness, version tracking |
| 19 | Skills list | (none) | `skills list --domain --proficiency-min` | Sorted by proficiency, domain filter |
| 20 | Tool usage stats | (none) | `tools stats --sort latency --top 10` | avg_ms, error rate, last called |
| 21 | Cron jobs list | (none) | `cron list --active` | Shows schedule, command, active status |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Live trading pulse | `trading pulse` | Requires join across today's session + open postmortems + PnL; no single API call |
| 2 | MCP reliability report | `mcp reliability` | Requires local join across activity_logs + auth_errors + servers; p50/p95 latency per server |
| 3 | Memory load by agent | `memory load [agent]` | Requires aggregation across active_memory by agent + expiry buckets; not a single endpoint |
| 4 | Brain activity window | `brain since 2h` | Time-windowed join across thoughts + agent_messages + shared_state changes |
| 5 | Agent throughput | `agents throughput` | Requires aggregation of natureos_task_queue completions by agent_type + time window |
| 6 | Trading calibration summary | `trading calibrate` | Current win rate by setup_type + regime from calibration table + last N sessions |
| 7 | Skill gap analysis | `skills gap` | Proficiency distribution by domain, compares skills vs tool_usage_stats to find gaps |
| 8 | Cross-domain anomaly | `brain anomalies --hours 24` | Detects unusual patterns: spike in mcp_auth_errors, memory expiry flood, task queue backlog |

## Novel Survivors (from adversarial brainstorm)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 9 | Trading Setup Drift Alert | `trading drift [--weeks 2]` | Rolling window premortem distribution joined to calibration EV — detects unconscious drift toward worst setups before P&L shows it |
| 10 | MCP Blast Radius | `mcp blast-radius [--since 1h]` | Temporal anti-join across mcp_auth_errors + mcp_activity_logs + natureos_task_queue (no FK) — reconstructs causal failure propagation |
| 11 | Stale Lock / Deadlock Detection | `agents deadlock` | Cross-references shared_state.locked_by against agent_messages.sender recency — finds agents holding locks while silent |
| 12 | Thought-to-Task Latency | `brain lag [--topic ops]` | Fuzzy-joins thoughts.topics to natureos_dual_model_tasks.project_slug on creation timestamps — measures insight→action lag |
