# Apify Absorb Manifest

## Source Tools Surveyed

| Tool | URL | Role | Features contributed |
|------|-----|------|----------------------|
| apify-cli (official, Node) | https://github.com/apify/apify-cli | Actor-developer tooling | init/push/call/run/login (limited platform surface) |
| apify-mcp-server (official) | https://github.com/apify/apify-mcp-server | MCP server | search-actors, call-actor, get-dataset-items, get-actor-log, docs search |
| apify-client-js | https://github.com/apify/apify-client-js | Node SDK | Full API coverage, retries, pagination |
| apify-client-python | https://github.com/apify/apify-client-python | Python SDK | Full API coverage, sync+async, typed responses |
| apify-client-rs (community) | https://github.com/metalwarrior665/apify-client-rs | Rust SDK | Partial coverage, typed |
| Crawlee | https://github.com/apify/crawlee | Scraping framework | Not directly absorbed (framework, not CLI) |

## Absorbed (match or beat everything that exists)

Generated typed commands cover the full REST surface (229 operations). Notable feature rows that map to specific user-visible commands beyond mechanical CRUD:

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List Actors (browse store) | apify-mcp `search-actors` | `apify-pp store search <q>` + cached locally | Offline replay, `--json`, dataset stats included |
| 2 | Get Actor details | apify-client `actor().get()` | `apify-pp actors get <id>` | `--json`, `--select`, agent-native |
| 3 | Call/run Actor | apify-cli `apify call` | `apify-pp actors call <id> --input <json>` | `--wait`, `--timeout`, exit codes typed |
| 4 | Get run status | apify-client `run().get()` | `apify-pp runs get <id>` | Streaming via `--watch` |
| 5 | List runs | apify-client `runs().list()` | `apify-pp runs list --status SUCCEEDED` | FTS over local cache |
| 6 | Stream run log | apify-mcp `get-actor-log` | `apify-pp runs log <id> --follow` | Tail-style follow, JSON line mode |
| 7 | Get dataset items | apify-mcp `get-dataset-items` | `apify-pp datasets items <id>` | Multi-format (json/csv/xlsx/rss/html), `--select` |
| 8 | List datasets | apify-client `datasets().list()` | `apify-pp datasets list` | Sort by `clean_item_count` |
| 9 | Push dataset items | apify-client `dataset().pushItems()` | `apify-pp datasets push <id> --stdin` | Batch from stdin, `--dry-run` |
| 10 | Key-value store get/put/delete | apify-client `keyValueStore().*` | `apify-pp kvs get/put/delete` | Pipe-friendly, binary-safe |
| 11 | Request queue head/add | apify-client `requestQueue().*` | `apify-pp rq head/add` | Batch add from stdin |
| 12 | List schedules | apify-client `schedules().list()` | `apify-pp schedules list` | Cron expression validation |
| 13 | Create schedule | apify-client `schedules().create()` | `apify-pp schedules create` | Idempotent via `--name` |
| 14 | Get schedule log | apify-client `schedule().getLog()` | `apify-pp schedules log <id>` | Last-N runs default |
| 15 | List webhooks | apify-client `webhooks().list()` | `apify-pp webhooks list` | Filter by event type |
| 16 | Test webhook | apify-client `webhook().test()` | `apify-pp webhooks test <id>` | Dry-run by default |
| 17 | List webhook dispatches | apify-client `webhookDispatches().list()` | `apify-pp webhook-dispatches list` | Status filter |
| 18 | Actor tasks (saved configs) | apify-client `tasks().*` | `apify-pp tasks list/get/call` | Run-and-wait |
| 19 | Actor builds | apify-client `builds().*` | `apify-pp builds list/get/log` | `--watch` for live builds |
| 20 | User account info | apify-client `user('me').get()` | `apify-pp users me` | `--json` |
| 21 | Monthly usage | apify-client `user().usage()` | `apify-pp users usage --since 30d` | Date-windowed |
| 22 | Auth setup | apify-cli `apify login` | `apify-pp auth set-token` | Token validation against `/users/me` |
| 23 | Health check | (none direct) | `apify-pp doctor` | Token valid + API reachable + quota status |
| 24 | Default dataset/kvs/rq (last run) | apify-client `lastRun().dataset()` | `apify-pp runs <id> dataset/kvs/rq` | Convenience subroute |

Plus every other spec endpoint exposed as `apify-pp <resource> <endpoint>` per generator convention. Full surface = 229 typed commands.

## Transcendence (only possible with our approach)

From novel-features subagent — Pass 1 produced 13 candidates, Pass 2 killed 3 (webhook listener: feasibility too heavy; actor recommender: weak fit signal; schema unification: kept as plumbing, not standalone command).

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Novelty diffing | `apify-pp run <actor> --only-new --format markdown` | Local SQLite of prior dataset items lets us URL+content-hash diff every run; apify-cli has no memory of prior runs | 10/10 |
| 2 | Cross-actor dataset FTS | `apify-pp search "<q>" --since 7d --actors twitter,reddit,hn` | SQLite FTS5 over normalized cross-Actor schema; Apify has no cross-dataset search | 9/10 |
| 3 | True cost ledger | `apify-pp cost report --since 30d --group-by actor,schedule` | Joins cached runs with usage endpoint for per-run USD (CU + RAM + storage + PPE); dashboard only shows aggregates | 9/10 |
| 4 | Newsletter digest renderer | `apify-pp digest --topic "<t>" --since 24h --template <name>` | Requires local normalized FTS + novelty + dedupe; nothing in Apify stack renders markdown digests | 9/10 |
| 5 | Curated weekly workflow | `apify-pp workflow run weekly-newsletter.yaml` | One declarative YAML chains run → normalize → novelty → digest → publish; callable as single MCP tool | 9/10 |
| 6 | Schedule-as-code | `apify-pp schedule apply schedules.yaml` + `schedule diff` | Terraform-style plan/apply/diff against schedule API with YAML source of truth | 8/10 |
| 7 | Cost budget enforcement | `apify-pp run <actor> --max-cost 0.50 --max-cu 100` | Pre-flight projection from local p50/p90 of past runs for THIS Actor; Apify has no per-invocation guard | 8/10 |
| 8 | Input presets | `apify-pp preset save twitter weekly-ai --from-run <id>` + `--preset` on call | Captures known-good input JSON with override-on-replay; solves "what flags did I use last week" | 7/10 |
| 9 | Actor A/B cost+quality test | `apify-pp ab run <actor1> <actor2> --input shared.json --judge novelty` | Runs both Actors, normalizes via unified schema, reports cost-per-novel-item + overlap %; needs local layers | 7/10 |
| 10 | Local replay mode | `apify-pp digest --offline` / `apify-pp run --replay-from <run_id>` | Iterate on templates against local SQLite copy of past datasets; zero API spend | 7/10 |

## Architectural Foundation (per subagent)

All 10 survivors share one critical dependency: an **on-ingest schema unification layer** plus the local SQLite store. Build order:

1. **Foundation** (~600-900 LoC): SQLite store + `runs`, `dataset_items`, `kv_store_records`, `webhook_dispatches`, `schedules`, FTS5 over `dataset_items`. Per-Actor normalize profiles shipped as YAML data files (top 15-20 newsletter-relevant Actors: apify/twitter-scraper, trudax/reddit-scraper, apify/google-news-scraper, etc.). User overrides at `~/.apify-pp/profiles/`.
2. **Each transcendence feature** (~80-150 LoC): thin Go command on top of foundation.

## Stubs (none planned)

All 10 transcendence features ship fully. The unification profile set ships with the top 15-20 Actors covered; users can drop overrides for more.

## Build Estimate

- Generated (Priority 0/1): ~229 endpoint commands + base store + sync + search + doctor + auth + agent-context. Generator handles.
- Hand-built foundation (Priority 0 extension): normalize layer + per-Actor profiles + extended store schema. ~600-900 LoC.
- Hand-built transcendence (Priority 2): 10 features. Total ~1000-1500 LoC across `internal/normalize/`, `internal/cli/`, `internal/digest/`, `internal/cost/`, `internal/schedule/`, `internal/preset/`, `internal/workflow/`, `internal/ab/`, `internal/budget/`.

## Risk / Open Questions

- `/v2/users/me/usage/monthly` granularity unconfirmed — may need to fall back to `run.stats.computeUnits * unitPrice` from user's plan if usage rollup is too coarse.
- Per-Actor normalize profiles need to be authored. Top 15-20 covered for v1; long-tail uses raw JSON passthrough.
- Schedule-as-code drift detection: schedule API supports the full set; no blockers.
