# Trigger.dev CLI — Absorb Manifest (reprint, redo research)

## Absorbed (match or beat everything that exists)

The 47-operation OpenAPI is auto-absorbed by the generator. Each row in the table below names a specific table-stakes capability the official ecosystem (CLI/SDK/MCP) exposes, the source we beat it against, and how our CLI's coverage adds agent-native value beyond the generic typed wrapper.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Trigger a task by identifier (single + batch) | @trigger.dev/sdk `tasks.trigger()`, official MCP `triggerTask` | Spec endpoints `POST /api/v1/tasks/{taskIdentifier}/trigger`, `POST /api/v1/tasks/{taskIdentifier}/batch` | --dry-run, --stdin batch input, agent-native JSON, idempotency-key flag |
| 2 | Retrieve, cancel, replay, reschedule a run | SDK `runs.*`, official MCP `getRunDetails`/`cancelRun` | Spec endpoints under `/api/v1/runs/{runId}` and `/api/v2/runs/{runId}/cancel`, plus `/api/v3/runs/{runId}` | Typed exit codes per terminal status (Pri 2), offline-cached metadata, `--watch` polling |
| 3 | List runs with filters (status, taskIdentifier, tags, cursor) | SDK `runs.list`, MCP `listRuns` | Spec endpoint `GET /api/v1/runs` (also `/api/v1/projects/{projectRef}/runs`) | --json, --select, --csv, FTS5 grep over cached runs (Pri 2) |
| 4 | Run trace, events, metadata mutation, tags, result | SDK + MCP individual tools | Spec endpoints `/api/v1/runs/{runId}/(trace\|events\|metadata\|tags\|result)` | --watch on tail, indexed local store, JSON-path projections |
| 5 | Schedules CRUD + activate/deactivate + timezones | SDK `schedules.*`, MCP `*ScheduleTool` | Spec endpoints under `/api/v1/schedules` plus `/api/v1/timezones` | Offline timezone validation, `schedules stale` (Pri 2) |
| 6 | Deployments list/get/latest/promote | SDK `deployments.*`, official CLI `promote` | Spec endpoints under `/api/v1/deployments` | Offline cache, deploy-window correlation in watch (Pri 2) |
| 7 | Batches retrieve + results | SDK `batches.*` | Spec `/api/v1/batches/{batchId}(/results)?` | --watch + cache; result selection via --select |
| 8 | Queues list/get/pause + concurrency override/reset | SDK `queues.*` | Spec endpoints under `/api/v1/queues` | Local snapshot of concurrency caps |
| 9 | Waitpoint tokens (create, list, complete, callback) | SDK `waitpoints.*` | Spec endpoints under `/api/v1/waitpoints/tokens` | Idempotent create with --idempotency-key; callback-hash mode supported |
| 10 | Env vars CRUD + bulk upload (per project/env) | SDK `envvars.*` | Spec endpoints under `/api/v1/projects/{projectRef}/envvars/{env}` | Masked values default; `envvars diff` (Pri 2) |
| 11 | TRQL query + schema + dashboards | SDK `query.*`, MCP `queryTool`/`getQuerySchemaTool`/`runDashboardQueryTool` | Spec endpoints `/api/v1/query`, `/api/v1/query/schema`, `/api/v1/query/dashboards` | Curated TRQL recipe catalog (Pri 2), agent-native results |
| 12 | `whoami` / auth status | Official CLI `whoami` | Generated `auth status` + `doctor` | Validates token env, prints project ref, exits with diagnostic |
| 13 | MCP `--readonly` mode | Official MCP server flag | MCP server respects `MCP_READONLY=1` env var, mirrors official behavior | Same agent-safe-by-default contract; pairs with our endpoint annotations |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence | Source |
|---|---------|---------|-------|-------------|----------|--------|
| 1 | LLM span cost rollup | `runs span-cost --since 7d --by model,task --top 20` | 9/10 | Local SQL group-by over synced run-span enrichment, fills gaps via real `/api/v1/runs/{id}/spans/{spanId}` | Brief workflow #6; Maya persona's CTO/finance ritual; spec exposes per-span model+tokens+cost | prior (kept) |
| 2 | Real-time failure watch | `runs watch --task <id> --notify --sound` | 9/10 | Polls `/api/v1/runs?status=FAILED` + local cursor; non-zero exit + OS notify + sound on new failures; short-circuits in `cliutil.IsVerifyEnv()` | Brief workflow #5; Diego persona's oncall + post-deploy frustration | prior (kept) |
| 3 | Stale schedule detection | `schedules stale --days 14 --min-success-rate 0.5` | 8/10 | Joins synced `schedules` table with recent runs in local SQLite | Brief workflow #3; Priya persona's hygiene sweep | prior (kept) |
| 4 | Env var diff | `envvars diff --from prod --to staging` | 8/10 | Two real `/api/v1/projects/{ref}/envvars/{env}` GETs + Go set-diff with masked values | Brief workflow #7; Priya persona's drift audit | prior (kept) |
| 5 | Recurring failure patterns | `failures top --since 7d --top 20` | 8/10 | Local group-by on (task, regex-normalized error signature) over cached runs; ranks by count + last_seen | Brief workflow #2; Maya's incident triage ritual | prior (kept) |
| 6 | FTS over run errors | `runs find "<query>" --status FAILED --since 30d` | 8/10 | SQLite FTS5 over cached error/tag/metadata; composes with status/since/json filters offline | Brief data layer ("FTS/search: critical… substrate where FTS5 unlocks queries no API call covers"); dashboard search loses filters per Maya | prior (kept) |
| 7 | Typed exit codes per terminal status | `runs get <run-id>` | 7/10 | Cobra `pp:typed-exit-codes` annotation maps run terminal states to exit codes 0/20/21/22/23/3/4 | Brief workflow #1; coding-agent persona's primary loop affordance | prior (kept) |
| 8 | TRQL recipe library | `query recipes` / `query run <recipe>` | 6/10 | Static-curated recipe catalog (`// pp:novel-static-reference`) + real `/api/v1/query` POST with parameter substitution | TRQL is brief's distinctive analytics surface; agents can't author TRQL from scratch reliably | new |

### Dropped prior features (reprint reconciliation)

| Prior feature | Drop reason |
|---------------|-------------|
| Cross-task health rollup (`tasks health`) | Re-scoring against current personas: duplicates the survivor trio (`failures top` + `runs span-cost` + `runs find`) without a sharper persona moment. The user can override this drop at the Phase 1.5 gate review. |

### Stubs

None. Every survivor ships fully implemented or it doesn't ship.

## Source tools surveyed

- `triggerdotdev/trigger.dev` (Apache 2.0, 14.8K stars) — repo for official CLI, SDK, MCP server, dashboard, runtime
- `@trigger.dev/sdk` (npm, 1.78M downloads/month) — Node SDK; 41 management methods
- Official `trigger.dev` CLI (npm, 960K downloads/month) — 17 dev-time commands (`dev`, `deploy`, `init`, `login`, `mcp`, etc.)
- Official MCP server (built into `@trigger.dev/sdk`, accessed via `npx trigger.dev@latest dev --mcp`) — ~30 tools across orgs/projects/runs/tasks/deploys/query/profiles/devServer/prompts/preview-branches
- Inngest CLI (competitor reference) — narrower scope, ~6 commands
- Temporal CLI (competitor reference) — broader scope, ~25 commands
- triggerdotdev/claude-code-sdk-trigger-example (community) — example task triggering Claude Code SDK

No third-party Trigger.dev MCP servers or Claude plugins/skills found beyond official.
