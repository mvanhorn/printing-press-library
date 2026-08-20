# Render CLI Brief

## API Identity
- **Domain**: Render.com — managed hosting platform (web services, static sites, private services, background workers, cron jobs, Postgres, Redis/Key-Value, disks, blueprints/IaC, Workflows).
- **Users**: dev/platform engineers running production deployments; SREs auditing cost, drift, and incident timelines; agents automating deploy/rollback/promotion.
- **Data profile**: ~25 resource families across 122 endpoints — services, deploys, env-groups + env-vars, custom domains, header/route rules, postgres, key-value, redis, disks + snapshots, owners + members, projects + environments, blueprints + syncs, audit logs, events, log streams, metrics, notifications, webhooks, registry credentials, maintenance, workflows + tasks + task-runs.
- **Spec**: `https://api-docs.render.com/openapi/render-public-api-1.json` (OpenAPI 3.0.2 JSON, ~314 KB).
- **Base URL**: `https://api.render.com/v1`.

## Reachability Risk
**None / Low.** Public REST API, Cloudflare-fronted but no JS interstitial on API endpoints. `GET /v1/services` returns 401 without auth — clean failure. Spec URL 302-redirects but is generator-friendly.

## Top Workflows
1. **Promote env vars between environments** — diff staging vs prod env-groups or per-service env-vars; selectively apply changes. Walks env-groups + service env-vars; covered by no surveyed tool.
2. **Preview-environment cleanup** — list services with `type=preview` and stale `updatedAt`; bulk delete. Render auto-cleans on PR-close but stale-by-inactivity preview envs accumulate.
3. **Blueprint drift detection** — compare local `render.yaml` to live workspace state (services + env-groups + databases). Render's blueprint sync is opaque; manual dashboard edits drift undetected.
4. **Rolling deploy with metric-aware rollback** — trigger deploy, poll status + metrics, rollback if SLO breached within window.
5. **Cost rollup** — walk services + postgres + key-value + disks; map plan tiers to monthly $; group by project/env.

## Table Stakes
- All CRUD across the 25 resource families.
- Lifecycle actions: restart, suspend, resume, rollback, scale, cache-purge, preview, failover.
- Logs query + tail (SSE streaming) + label discovery.
- Metrics fetch (cpu, memory, http-latency, http-requests, instance-count, bandwidth, disk-*, replication-lag, task-runs-*).
- `--json` / `--yaml` output, `--dry-run`, `--confirm` for destructive ops.
- Bearer auth via `RENDER_API_KEY` env var (`Authorization: Bearer ...`).
- Workspace switching (`workspaces select`).

## Data Layer
- **Primary entities to cache**: services, deploys, env_groups + env_group_vars, env_vars (per-service), custom_domains, header_rules, route_rules, postgres, key_value, redis, disks + disk_snapshots, owners + owner_members, projects, environments, blueprints + blueprint_syncs, audit_logs, events.
- **Skip from cache**: metrics time-series (live calls; cache only last query result), logs (volume — cache last `--limit N` query for replay).
- **Sync cursor**: list endpoints take `cursor` query param and return `[{cursor, <resource>}, ...]` envelopes — sibling-cursor pagination (not nested `data`/`next_cursor`). Walker pages until short page. No global "since timestamp"; use endpoint-specific `updatedAt` filters or `from`/`to` for audit logs.
- **FTS5 candidates**: `services.name + env-var keys`, `audit_logs.message + actor + target`, `events.text`, `deploys.commit + status`, `blueprints.spec`. Killer cross-resource search: "find every service touched by env var `STRIPE_KEY` across all envs."

## Auth
- **Token format**: opaque API key from Account Settings → API Keys.
- **Header**: `Authorization: Bearer {token}`.
- **Env var**: `RENDER_API_KEY` (universal across official MCP, official CLI, kurtbuilds/render, niyogi/render-mcp).
- **Rate-limit**: 429 documented but limits and `Retry-After` headers undocumented. Implement exponential backoff. Surface `render-request-id` response header in error output for support.

## Spec Quirks
- **Sibling-cursor pagination**: `[{"cursor":"...","service":{...}}, ...]`. Generator pagination needs a "sibling cursor" mode.
- **Action verbs as path suffixes**: `/cancel`, `/restart`, `/resume`, `/suspend`, `/rollback`, `/scale`, `/cache/purge`, `/failover`, `/verify`, `/trigger`. First-class action commands (e.g., `render services restart <id>`) are the right surface.
- **Nested resources** under `services/{id}/`: `custom-domains`, `env-vars`, `secret-files`, `headers`, `routes`, `jobs`, `instances`, `events`, `autoscaling`. Model as nested commands.
- **camelCase operationIds** — generator should normalize to kebab.
- **Single security scheme** (`BearerAuth`) — clean.

## Codebase Intelligence
- Source: direct GitHub inspection of `render-oss/render-mcp-server`, `render-oss/cli`, `render-oss/terraform-provider-render`.
- **MCP layout**: per-resource Go packages under `pkg/{service,postgres,keyvalue,logs,metrics,deploy,environment,owner}` with curated kebab tool names (`list_services`, `create_web_service`) — not 1:1 with `operationId`. We mirror this curation.
- **OFC structure**: ~50 command files, several workflow-shaped (`run-cancel`, `workflow-init-checklist`) — confirmation that good Render UX isn't a thin endpoint mirror.
- **TF provider**: authoritative list of resources Render considers "managed entities."

## Tools Landscape
| Tool | Role | Coverage |
|---|---|---|
| render-oss/cli (Go, 93★) | Official, interactive-TUI bias | services, deploys, jobs, kv, logs, projects, envs, workspaces, blueprints validate, psql, ssh, workflows |
| render-oss/render-mcp-server (Go, 129★) | Official MCP | ~31 tools across 7 resource families |
| render-oss/terraform-provider-render (Go, 51★) | Official IaC | declarative resources for services + datastores + envs + domains + notifications + log-streams |
| render-oss/skills (Markdown) | 21 SKILL.md docs | prompt seed for novel-feature commands |
| niyogi/render-mcp (TS, 15★) | Unofficial MCP | smaller surface than official |
| kurtbuilds/render (Rust, 20★) | Generated client + thin CLI | sanity check on parameter names |

Multiple tools overlap on the basics (services CRUD, deploys); none cover env-promotion, drift, cost rollup, preview cleanup, orphans, or audit-log search.

## Product Thesis
- **Name**: `render-pp-cli` (binary), CLI alias `render`.
- **Why it should exist**: every existing Render tool is an endpoint mirror. None lets you reason about your full Render footprint offline — diff env vars across services/envs, detect blueprint drift, project monthly cost, clean up stale preview envs, search audit history, find orphans. We absorb every endpoint AND give you the analytical primitives the dashboard refuses to ship.

## Build Priorities
1. **P0 foundation**: data layer for services, deploys, env-groups, env-vars, postgres, key-value, disks, owners, projects, environments, audit-logs, blueprints, custom-domains, header-rules, route-rules. Cursor walker. FTS5 across `services.name + env-var keys + audit-logs.message + events.text`.
2. **P1 absorb**: every command from every surveyed tool — full CRUD + lifecycle actions across all 25 resource families. Logs query + tail. Metrics fetch + filter discovery. Bulk env-var import/export with `.env` file format. Workspace switching.
3. **P2 transcend**: `env diff/promote`, `drift`, `cost`, `preview-cleanup`, `velocity`, `orphans`, `deploys diff`, `audit search` — the analytical primitives no other tool ships.
4. **P3 polish**: SKILL.md with real Render workflows; flag descriptions enriched (especially around plan tiers and region codes); README cookbook for the 5 power workflows.
