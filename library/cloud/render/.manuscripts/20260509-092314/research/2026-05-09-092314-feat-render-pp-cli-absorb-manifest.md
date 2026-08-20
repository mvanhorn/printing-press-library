# Render CLI Absorb Manifest

Best Source legend: **OFC** = render-oss/cli, **OMCP** = render-oss/render-mcp-server, **TF** = render-oss/terraform-provider-render, **NMCP** = niyogi/render-mcp, **KB** = kurtbuilds/render, **API** = direct OpenAPI surface (no tool covered it well).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List services (cursor-paginated) | OFC, OMCP | `render services list` | Local SQLite cache; `sql` + FTS5 across all services |
| 2 | Get service | OFC, OMCP | `render services get <id>` | — |
| 3 | Create web/static/cron/worker/private services | OMCP, OFC, TF | `render services create <type>` (`--repo`, `--branch`, `--runtime`, `--plan`, `--region`) | One command for every service type |
| 4 | Update service | OMCP | `render services update <id>` | — |
| 5 | Delete service | NMCP, API | `render services delete <id>` (`--confirm`) | Print resources at risk first |
| 6 | Restart service | API | `render services restart <id>` | — |
| 7 | Suspend / Resume service | API | `render services suspend/resume <id>` | — |
| 8 | Rollback service to deploy | API | `render services rollback <id> --deploy <deployId>` | — |
| 9 | Scale service | API | `render services scale <id> --num <n>` | — |
| 10 | Purge service cache | API | `render services cache-purge <id>` | — |
| 11 | Trigger preview env | API | `render services preview <id>` | — |
| 12 | List service instances | API | `render services instances <id>` | — |
| 13 | List service events | API | `render services events <id>` | Cached for incident timeline |
| 14 | List deploys | OFC, OMCP | `render deploys list <serviceId>` | Cached for velocity / diff |
| 15 | Get deploy | OFC, OMCP | `render deploys get <serviceId> <deployId>` | — |
| 16 | Create deploy | OFC | `render deploys create <serviceId>` (`--commit`, `--image`, `--clear-cache`, `--wait`) | `--wait` polls + streams status |
| 17 | Cancel deploy | OFC | `render deploys cancel <serviceId> <deployId>` | — |
| 18 | One-off jobs CRUD/cancel | OFC, API | `render jobs list/get/create/cancel <serviceId>` | — |
| 19 | Per-service env-vars (CRUD + bulk) | OMCP, OFC | `render env-vars list/get/set/delete <serviceId>` + `import --file .env` / `export` | Bulk import/export with `.env` format |
| 20 | Per-service secret-files | API | `render secret-files list/get/put/delete <serviceId>` | Path-safe local cache |
| 21 | Env-groups CRUD + per-key vars + attach/detach | API, TF | `render env-groups list/create/get/update/delete` + `env-groups vars get/set/delete` + `env-groups attach/detach <gid> <sid>` | None of the existing CLIs cover this fully |
| 22 | Custom domains list/add/get/delete/verify | NMCP, API | `render domains list/add/get/delete/verify <serviceId>` | DNS verification helper prints required records |
| 23 | Header rules CRUD | API | `render headers list/add/update/delete <serviceId>` | Untouched by every existing tool |
| 24 | Route rules CRUD | API | `render routes list/add/update/delete <serviceId>` | Untouched by every existing tool |
| 25 | Autoscaling get/set/clear | API, TF | `render autoscaling get/set/clear <serviceId>` | Imperative complement to TF |
| 26 | Postgres CRUD + lifecycle | API, OMCP | `render postgres list/create/get/update/delete` + `suspend/resume/restart/failover` | — |
| 27 | Postgres recovery / export / creds / connection-info | API | `render postgres recovery/export/creds list-create-delete/connection-info` | Mask password unless `--reveal` |
| 28 | Postgres psql interactive | OFC | `render postgres psql <id>` | Reuse OFC's SSH-tunnel approach |
| 29 | Key-Value (Redis-compatible) CRUD + lifecycle + connection-info | API, OMCP | `render kv list/create/get/update/delete/suspend/resume/connection-info` | Mirror Postgres lifecycle |
| 30 | Redis (legacy) CRUD + connection-info | API | `render redis list/create/get/update/delete/connection-info` | Marked as legacy in help |
| 31 | Disks CRUD + snapshots | API, TF | `render disks list/create/get/update/delete` + `disks snapshots <id>` + `disks snapshots restore` | — |
| 32 | Projects CRUD | OFC, API | `render projects list/create/get/update/delete` | — |
| 33 | Environments (within project) CRUD + attach/detach | OFC, API | `render environments list/create/get/update/delete` + `attach/detach <eid> <rid>` | — |
| 34 | Workspaces list/get/select | OFC, OMCP | `render workspaces list/get/select` | — |
| 35 | Owners list/get/audit-logs + members manage | API | `render owners list/get/audit-logs` + `render owners members list/update/remove` | Members management absent from every tool |
| 36 | Audit logs (owner/org) | API | `render audit-logs --owner <id>` (also `--org`) | Local store enables forensic queries |
| 37 | Blueprints list/get/update/delete + validate + syncs | OFC, API | `render blueprints list/get/update/delete/validate` + `blueprints syncs <id>` | — |
| 38 | Logs query | OFC, OMCP | `render logs query --service <id> --since 1h --filter ...` | — |
| 39 | Logs subscribe (tail/stream SSE) | OMCP, API | `render logs tail --service <id>` | — |
| 40 | Logs labels + values discovery | OMCP, API | `render logs labels` and `render logs values --label level` | — |
| 41 | Log streams CRUD (per-owner / per-resource) | API, TF | `render log-streams list/get/put/delete` | — |
| 42 | Metrics get (cpu/memory/http-*/instance-count/bandwidth/disk-*/replication-lag/task-runs-*) | OMCP, API | `render metrics get <metric> <serviceId> --since 1h` | — |
| 43 | Metrics filter discovery | API | `render metrics filters {http,application,path}` | — |
| 44 | Metrics streaming put/delete | API | `render metrics stream put/delete --owner <id>` | — |
| 45 | Webhooks CRUD + events | API | `render webhooks list/create/get/update/delete/events <id>` | — |
| 46 | Notification settings get/set | API, TF | `render notifications get/set [--owner <id>\|--service <id>]` | — |
| 47 | Registry credentials CRUD | API, TF | `render registry-credentials list/create/get/update/delete` | — |
| 48 | Maintenance windows | API | `render maintenance list/get/update/trigger` | — |
| 49 | Workflows + tasks + task-runs CRUD/cancel | OFC, API | `render workflows ...` / `tasks ...` / `task-runs ...` | — |
| 50 | SSH into service | OFC | `render ssh <serviceId>` | Reuse OFC's ephemeral-SSH approach |
| 51 | Auth login + whoami | OFC | `render auth login` + `render whoami` | Standard generator pattern |
| 52 | Output formats (--json/--yaml/--text) + --select | OFC | global flags | Default agent-native shape |
| 53 | --confirm flag for destructive ops | OFC | global flag | Required for agent-native safety |
| 54 | --dry-run for mutating ops | API | global flag | Prints request without sending |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Persona Served | Score | Buildability Proof |
|---|---|---|---|---|---|---|
| T1 | Env-var diff across services/env-groups | `render env diff <a> <b>` | All surveyed tools treat env-vars as per-resource lists; none diff. | Priya | 10/10 | Reads cached `env_vars` and `env_group_vars` rows for both targets and prints a key-by-key three-way diff. |
| T2 | Env-var promotion with allow/deny lists | `render env promote --from <src> --to <dst>` | OFC has no promote; OMCP only has per-key update. We diff then issue scoped PUTs. | Priya | 10/10 | Computes the diff above, then `PUT /v1/services/{id}/env-vars` (bulk) or per-key on env-groups, filtered by `--only`/`--exclude`. |
| T3 | Blueprint drift detection | `render drift [--blueprint render.yaml]` | "Render's blueprint sync is opaque; manual dashboard edits drift undetected." | Priya | 10/10 | Parses local `render.yaml`, walks cached `services` + `env-groups` + `postgres` + `disks`, reports added/removed/modified by name. |
| T4 | Cost rollup across all billable resources | `render cost [--group-by project\|env\|owner]` | No surveyed tool covers this; dashboard shows per-resource cost only. | Priya, Devon | 10/10 | Joins cached `services.plan`, `postgres.plan`, `key_value.plan`, `redis.plan`, `disks.sizeGB` against a curated plan-price table (`// pp:novel-static-reference`) and groups by dimension. |
| T5 | Stale preview environment cleanup | `render preview-cleanup --stale-days <N>` | Render auto-cleans on PR-close but inactivity-stale previews accumulate. | Devon | 10/10 | Selects cached `services WHERE type='preview' AND updatedAt < now - N days`; on `--confirm` calls `DELETE /v1/services/{id}`. |
| T6 | Orphan resource sweep | `render orphans` | No tool surfaces unattached disks, empty env-groups, dangling domains, or unused registry credentials. | Devon, Riley | 9/10 | LEFT-JOINs cached `disks/env_groups/custom_domains/registry_credentials` against `services` linkage. |
| T7 | Cross-resource env-var locator | `render env where <KEY>` | "Find every service touched by env var STRIPE_KEY across all envs" — the killer cross-resource search. | Riley, Priya | 10/10 | FTS5 query over `env_vars.key + env_group_vars.key`; emits service/env-group id, name, value-hash, and updatedAt. |
| T8 | Incident timeline merge | `render incident-timeline <serviceId> --since <ts>` | OFC/OMCP list deploys/events/audit-logs in isolation; nothing joins them by time. | Sam | 10/10 | UNIONs cached `deploys`, `events`, `audit_logs` filtered by service id + time window, sorted by timestamp. |
| T9 | Audit-log forensic search | `render audit search --actor <u> --target <id> --since <ts>` | OFC has no audit command; OMCP has `list_audit_logs` only — no FTS. | Riley | 9/10 | FTS5 query over cached `audit_logs` populated by the cursor walker against `GET /v1/owners/{id}/audit-logs`. |
| T10 | Deploy-to-deploy diff | `render deploys diff <serviceId> <a> <b>` | No surveyed tool diffs deploys; only `list/get`. | Sam | 9/10 | Pulls both deploy records, joins env-var snapshots, emits commit range + changed image/plan/scale fields. |
| T11 | Rightsizing recommendations from metrics | `render rightsize [--since 7d]` | No tool joins metrics with plans. Brief Top Workflow #5 cost concern depends on this signal. | Priya, Devon | 9/10 | Calls `GET /v1/metrics/cpu` and `/v1/metrics/memory` per service, computes p95 utilization, flags `--high`/`--low` thresholds against plan capacity. |
