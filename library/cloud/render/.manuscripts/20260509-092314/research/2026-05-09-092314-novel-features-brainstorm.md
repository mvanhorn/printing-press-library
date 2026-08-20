# Novel Features Brainstorm — render

## Customer model

**Persona A — Priya, Platform Engineer at a 30-service Render workspace (Series-B fintech).**
- Today: chasing a config drift incident — a env-var someone set in the dashboard for `payments-api` doesn't match the `render.yaml` blueprint, and prod started 500'ing after a deploy.
- Weekly ritual: Monday morning blueprint-vs-live diff across all 30 services + 6 env-groups; Friday cost rollup to email finance.
- Frustration: "Render's dashboard hides which env vars differ between staging and prod. I keep a Notion page of grep'd `.env` exports because the API has no diff endpoint." (Brief Top Workflows #1, #3)

**Persona B — Sam, SRE on-call for a Render-hosted SaaS (8 web services + 2 Postgres + 1 KV).**
- Today: a 2am page — `checkout-api` p95 latency tripled after `dep-abc123`. Needs to correlate the deploy timestamp with metric breach and roll back.
- Weekly ritual: weekly incident review — for each Sev-2, pull deploys+events+audit-logs into a single timeline.
- Frustration: "I have three browser tabs open: deploys, metrics, audit log. Nothing joins them by time." (Brief Top Workflows #4)

**Persona C — Devon, Solo founder shipping ~12 PR previews per week.**
- Today: monthly Render bill spiked 40% — needs to find which preview envs are still running and which services are oversized.
- Weekly ritual: prune stale preview envs every Friday; check that no orphan disks/env-groups linger after service deletion.
- Frustration: "Render auto-cleans on PR-close but inactivity-stale previews accumulate. The dashboard makes me click into each one to see when it last deployed." (Brief Top Workflows #2, #5)

**Persona D — Riley, security engineer auditing Render usage for SOC 2.**
- Today: needs to prove who rotated the `STRIPE_KEY` secret across all services in Q1, and which deploys picked up the new value.
- Weekly ritual: scan audit logs for member adds/removes and env-var writes outside change-window.
- Frustration: "Audit logs are JSON dumps. Can't filter by actor + target + time-range without piping through `jq` rituals."

## Candidates (pre-cut)

(see subagent transcript for the full list; survivors and kills below)

## Survivors and kills

### Survivors

| # | Feature | Command | Why Only We Can Do This | Persona Served | Score (Util/Novel/Reach/Build, /40) | Buildability Proof |
|---|---------|---------|-------------------------|----------------|--------------------------------------|--------------------|
| 1 | Env-var diff across services/env-groups | `render env diff <a> <b>` | All surveyed tools (OFC/OMCP/TF) treat env-vars as per-resource lists; none diff. Brief: "covered by no surveyed tool." | Priya | 3/3/2/2 = 10/10 | Reads cached `env_vars` and `env_group_vars` rows for both targets and prints a key-by-key three-way diff (only-in-A / only-in-B / changed). |
| 2 | Env-var promotion with allow/deny lists | `render env promote --from <src> --to <dst>` | OFC has no promote; OMCP only has per-key `update_env_var`. We compute the diff then issue scoped PUTs. | Priya | 3/3/2/2 = 10/10 | Computes the diff above, then calls `PUT /v1/services/{id}/env-vars` (bulk) or per-key `POST /v1/env-groups/{id}/env-vars/{key}` filtered by `--only`/`--exclude`. |
| 3 | Blueprint drift detection | `render drift [--blueprint render.yaml]` | Brief: "Render's blueprint sync is opaque; manual dashboard edits drift undetected." Neither OFC nor OMCP nor TF surfaces drift between a checked-in `render.yaml` and live state. | Priya | 3/3/2/2 = 10/10 | Parses local `render.yaml`, walks `GET /v1/services`, `/v1/env-groups`, `/v1/postgres`, `/v1/disks` from the local store, and reports added/removed/modified entities by name. |
| 4 | Cost rollup across all billable resources | `render cost [--group-by project\|env\|owner]` | No surveyed tool; brief Top Workflow #5. Combines services + postgres + key-value + redis + disks plan tiers — Render's dashboard shows per-resource cost only. | Priya, Devon | 3/3/2/2 = 10/10 | Joins cached `services.plan`, `postgres.plan`, `key_value.plan`, `redis.plan`, `disks.sizeGB` against a curated plan-price table (`// pp:novel-static-reference`) and groups by the requested dimension. |
| 5 | Stale preview environment cleanup | `render preview-cleanup --stale-days <N>` | Brief Top Workflow #2: "Render auto-cleans on PR-close but stale-by-inactivity preview envs accumulate." Dashboard requires per-service click-through. | Devon | 3/3/2/2 = 10/10 | Selects from cached `services` where `type='preview'` and `updatedAt < now - N days`, prints, and on `--confirm` calls `DELETE /v1/services/{id}` for each. |
| 6 | Orphan resource sweep | `render orphans` | No tool surveyed surfaces unattached disks, empty env-groups, dangling domains, or unused registry credentials. | Devon, Riley | 3/2/2/2 = 9/10 | LEFT-JOINs cached `disks` against `services.diskId`, `env_groups` against `service_env_group_links`, `custom_domains` against live `services`, `registry_credentials` against `services.imageRegistryCredentialId`. |
| 7 | Cross-resource env-var locator | `render env where <KEY>` | Brief calls this the "killer cross-resource search": "find every service touched by env var STRIPE_KEY across all envs." | Riley, Priya | 3/3/2/2 = 10/10 | FTS5 query against `env_vars.key + env_group_vars.key` rows from the local store; emits service/env-group id, name, value-hash, and `updatedAt`. |
| 8 | Incident timeline merge | `render incident-timeline <serviceId> --since <ts>` | OFC/OMCP each list deploys/events/audit-logs in isolation; brief Persona B is exactly this gap ("three browser tabs"). | Sam | 3/3/2/2 = 10/10 | UNIONs cached `deploys`, `events`, `audit_logs` rows filtered by service id and time window, sorts by timestamp, prints a single chronological table. |
| 9 | Audit-log forensic search | `render audit search --actor <u> --target <id> --since <ts>` | Brief Data Layer flags `audit_logs.message + actor + target` as an FTS5 candidate. OFC has no audit command; OMCP has `list_audit_logs` only. | Riley | 3/2/2/2 = 9/10 | FTS5 query over the cached `audit_logs` table populated by the cursor walker against `GET /v1/owners/{id}/audit-logs`. |
| 10 | Deploy-to-deploy diff | `render deploys diff <a> <b>` | No surveyed tool diffs deploys; only OFC's `deploys list/get`. Sam needs commit range + env/image/scale deltas in one view for incident reviews. | Sam | 3/2/2/2 = 9/10 | Pulls both deploy records from `GET /v1/services/{id}/deploys/{deployId}` and joins env-var snapshot rows; emits commit range plus changed image/plan/scale fields. |
| 11 | Rightsizing recommendations from metrics | `render rightsize [--since 7d]` | No surveyed tool joins metrics with plans. Brief Top Workflow #5 cost concern depends on this signal. | Priya, Devon | 3/2/2/2 = 9/10 | Calls `GET /v1/metrics/cpu` and `/v1/metrics/memory` per service across the window, computes p95 utilization, and flags services breaching `--high`/`--low` thresholds against their plan capacity. |

### Killed candidates

| Feature | Sibling that survived | Reason for cut |
|---------|----------------------|----------------|
| `render velocity` (deploy-velocity histogram) | `render deploys diff` (#10) | Pretty stats but no clear acted-on outcome; drops to 4/10 on User Pain. |
| `render rolling-deploy` (deploy + auto-rollback on SLO) | `render incident-timeline` (#8), `render rightsize` (#11) | Scope creep — needs background polling, threshold config, rollback orchestration. Kill/keep "Scope creep" rule. |
| `render blueprint apply --plan` | `render drift` (#3) | Overlaps drift; the apply path is owned by Render's existing blueprint-sync endpoint. Reframed-into drift's diff output. |
| `render dns check <domain>` | absorb #17 (`domains verify`) | Spec already exposes `POST /v1/custom-domains/{id}/verify`; querying external resolvers is an external-service dependency the rubric flags. Cut. |
| `render snapshots prune` | `render orphans` (#6) | Orphans already covers "snapshots older than retention"; standalone prune is a one-flag variant of the same query. Merge into orphans, kill the standalone. |
