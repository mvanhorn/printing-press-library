# Trigger.dev CLI Brief (reprint, redo research)

## API Identity
- Domain: durable background jobs / AI agent orchestration / workflow execution platform
- Users: Node.js/TypeScript app developers who run async work outside HTTP request lifetimes — cron-like schedules, long-running AI pipelines, queued task fans, retry-aware mutations. Increasingly shared with AI engineers building agent workflows.
- Data profile: stateful runs (status, timing, cost, errors, metadata, traces, tags), schedules (CRUD + activate/deactivate), deployments (list, latest, promote), batches, queues (concurrency-aware), waitpoints (token-driven async pauses), per-environment env vars, and a query/dashboards analytics surface (`/api/v1/query`).

## Reachability Risk
- **None.** Probe: `curl https://api.trigger.dev/api/v1/timezones` returns 200 in <0.3s. Bearer auth via `TRIGGER_SECRET_KEY` validated against the dev project (key supplied by user for live testing). No anti-bot, no Cloudflare challenge, no rate limit issues reported in recent issues.

## Source Priority
- Single-source. Skip multi-source priority gate.

## Spec Source
- `https://raw.githubusercontent.com/triggerdotdev/trigger.dev/main/docs/v3-openapi.yaml` (last edit 2026-04-14: list deployments, defaultMachine fix, wait token browser completion). 47 typed operations across 10 resource groups: runs, schedules, deployments, batches, queues, waitpoints, tasks, env vars, query, timezones.
- Substantial uplift over the prior internal-YAML spec (8 endpoints, hand-curated). Switching to the official OpenAPI is the single biggest delta of this reprint.

## Top Workflows
1. **Trigger a task and follow it to terminal state.** `tasks trigger` → poll `runs get` (typed exits per status) → `runs result`. Most common interactive use case.
2. **Audit recent failures.** Grep run output across an environment, group by (task, error signature), surface top recurring failure modes. Powers incident response and post-deploy regression detection.
3. **Manage schedules.** Create/list/activate/deactivate cron-like and IMPERATIVE schedules; spot zombie schedules whose recent runs all fail or stop firing.
4. **Promote and audit deployments.** List deployments, fetch latest, promote between environments, see which deployment a run came from.
5. **Watch live failures during a deploy window.** Real-time interrupt when a new failure shows up — non-zero exit, desktop notification, sound, suitable for shell-loop composition.
6. **Cost rollups.** Aggregate LLM-span cost per task/model over a window — answer "is my deploy hurting?" without opening the dashboard.
7. **Env var management & cross-env diffing.** Bulk import, masked-value diff between dev/preview/prod.

## Table Stakes (every competing tool covers some of this)
- Trigger task synchronously or as part of a batch; cancel; replay; reschedule; tag; metadata mutation
- List runs with status/task/tags/date filters; get one; get its result; trace; events
- Schedule CRUD + activate/deactivate; timezone validation
- Deployment list + promote
- Queue management with per-queue concurrency override, pause/reset
- Wait-token primitives (durable async pauses)
- Env var CRUD per project/env, bulk upload
- Bearer-token auth via `TRIGGER_SECRET_KEY` (`tr_dev_…` for dev, `tr_prod_…` for prod)
- API surface explorable as MCP tools

## Data Layer
- Primary entities: `runs`, `schedules`, `deployments`, `batches`, `queues`, `waitpoints`, `envvars`, `tasks` (logical, derived from runs).
- Sync cursor: `createdAt` for runs (paginated lists support cursor); schedules and queues are small enough for full pulls.
- FTS/search: critical. Run errors, tags, metadata payloads, and task identifiers are exactly the substrate where FTS5 unlocks queries no API call covers (regex over error messages, free-text grep across run tags).
- Cache profile: runs are append-mostly (status mutates until terminal); schedules and queues are CRUD-frequent. Sync should treat completed runs as immutable, in-flight runs as TTL-short.

## Codebase Intelligence
- Source: `triggerdotdev/trigger.dev` (Apache 2.0, 14.8K stars, monorepo). API server, official CLI (`trigger.dev` npm package), MCP server (`@trigger.dev/sdk`), Bun-based workers.
- Auth: Bearer `Authorization: Bearer tr_<env>_<random>`. The `tr_dev_` / `tr_prod_` / `tr_pat_` prefixes encode environment scope; the management API rejects mismatched-environment calls with a typed 403.
- Data model: runs reference taskIdentifier + project + environment + deployment; schedules reference taskIdentifier; queues reference task names; wait tokens reference run IDs; env vars are scoped to (project, environment).
- Rate limiting: documented "Reasonable rate limits enforced per project"; 429 returns `Retry-After` (standard seconds-based). No public rate-limit dashboard.
- Architecture: REST API hits the platform, which queues onto Bun workers; MCP server is a thin shim over the SDK that exposes typed tools. The official MCP server adds `readOnlyHint`/`destructiveHint` annotations and a `--readonly` mode that suppresses write tools — directly informs our MCP shape.

## User Vision
None provided beyond "secret key you can use is `tr_dev_…`" (held in env var only, not persisted). The user is using this to live-test the regenerated CLI; treat it as a Phase 5 dependency, not a feature directive.

## Product Thesis
- Name: `trigger-dev-pp-cli`
- Why it should exist: Trigger.dev's official CLI is bundled with the project SDK and dev-time tooling (deploy, dev). The management API surface is rich (47 endpoints) and grew significantly since the last reprint — but **only the SDK / dashboard / MCP server expose it**. There is no first-class agent-ergonomic CLI for run audits, schedule hygiene, cost rollups, or cross-env env-var diffs. Power users hit the dashboard or write throwaway scripts. The CLI fills the same gap that `gh` fills for GitHub: a scriptable, agent-native, offline-capable management tool sitting in front of the same APIs the dashboard uses.

## Build Priorities
1. **Foundation** — auth (`TRIGGER_SECRET_KEY` bearer), client with adaptive 429 handling, doctor, full SQLite store sync for runs / schedules / queues / deployments / envvars, FTS5 over runs.
2. **Absorb** — every endpoint in the v3-openapi.yaml mapped to a typed Cobra+MCP command; SDK parity for the management surface; explicit MCP `readOnlyHint`/`destructiveHint` annotations; a `--readonly` mirror of the official MCP server's flag.
3. **Transcend** — the prior reprint shipped 8 novel features (real-time failure watch, cross-task health, stale schedule, env-var diff, LLM span cost, recurring-failure top, FTS-grep run errors, typed exit codes per terminal status). All eight should be reconciled by the novel-features subagent against current personas; reframe and re-rank, don't carry forward verbatim.
