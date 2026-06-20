# dbt Cloud CLI Brief

## API Identity
- Domain: dbt Cloud Administrative API **v2** — orchestration of dbt jobs/runs in dbt Cloud.
- Scope (user-confirmed): **jobs, runs, artifacts** only (the "script surface"). No v3 admin objects, no Discovery/Semantic Layer APIs.
- Users: analytics engineers / data platform teams operating dbt Cloud jobs and CI.
- Data profile: jobs (definitions), runs (executions with steps, status, duration, git context), run artifacts (manifest.json, run_results.json, catalog.json, logs).

## Reachability Risk
- None. Live probe `GET /api/v2/accounts/{id}/runs/?limit=1` returned HTTP 200 with real data.
- Auth verified: BOTH `Authorization: Bearer <token>` and `Authorization: Token <token>` succeed. Modeling as `http bearer` (official spec). Env var `DBT_CLOUD_TOKEN` confirmed working.

## Auth / Config (verified)
- Token: `DBT_CLOUD_TOKEN` (PAT or service token), sent as `Authorization: Bearer <token>`.
- Account: `DBT_CLOUD_ACCOUNT_ID` — must default the required `account_id` path param so commands don't force `--account-id` everywhere. **Build-time wiring item.**
- Host: `DBT_CLOUD_HOST` (default `https://cloud.getdbt.com`) — dbt Cloud is multi-region (`{prefix}.us1.dbt.com`, `emea.dbt.com`). Client must honor the override. **Build-time wiring item.**

## Top Workflows (from user's proven Python scripts)
1. Trigger a job and watch the run to completion, exit non-zero on failure, surface failed-step logs (`monitor_dbt_run.py` + `dbt_cloud.py jobs trigger`). **THE headline.**
2. List recent runs filtered by status/job/project, inspect a run with its steps.
3. List/fetch run artifacts (run_results.json, manifest.json, logs) after a run.
4. Cancel / retry a run.

## Table Stakes (from API + community `dbtc`, `dbt-cloud-cli`)
- jobs list/get/trigger(run)/rerun
- runs list (filters: status, job_definition_id, project_id, environment_id, order_by)/get(include run_steps)/cancel/retry
- artifacts list (run)/get (run + job)
- step detail (steps/{id})
- `--json` machine output for piping

## Data Layer
- Primary entities: `runs` (high-gravity: status, duration, job, git_sha, finished_at), `jobs`.
- Sync cursor: `runs?order_by=-created_at` + `finished_at__range`.
- Enables run-history analytics the API can't return in one call (success rate, duration trends, failure clustering).

## Product Thesis
- Name: **dbt Cloud CLI** (`dbt-cloud-pp-cli`).
- Why it should exist: a single static Go binary that does the operate-and-watch loop (`trigger --wait`, `monitor`) with honest exit codes for CI, plus local run-history analytics and artifact diffing that no thin API wrapper offers. Agent-native (`--json`, `--select`), offline-queryable.

## Build Priorities
1. Foundation: bearer auth from `DBT_CLOUD_TOKEN`, `account_id` from `DBT_CLOUD_ACCOUNT_ID`, base URL from `DBT_CLOUD_HOST`; local SQLite run store + sync.
2. Absorb: all jobs/runs/artifacts/steps endpoint commands with `--json`.
3. Transcend: `monitor`, `trigger --wait`, `runs stats`, `failures/since`, `artifacts diff`.
