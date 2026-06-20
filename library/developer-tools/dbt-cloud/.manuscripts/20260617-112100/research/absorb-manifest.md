# dbt Cloud CLI — Absorb Manifest

Scope: jobs / runs / artifacts (Administrative API v2). Auth = bearer (`DBT_CLOUD_TOKEN`), verified live.

## Absorbed (match or beat every existing tool: `dbtc`, `dbt-cloud-cli`, raw API)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List jobs | API `GET jobs/` | `(generated endpoint) jobs list` | `--json`/`--select`, defaults account from env |
| 2 | Get job | API `GET jobs/{id}/` | `(generated endpoint) jobs get` | typed output |
| 3 | Trigger job run | API `POST jobs/{id}/run/` | `dbt-cloud-pp-cli jobs run` | `--cause/--git-sha/--git-branch/--schema-override`, returns run id |
| 4 | Rerun job | API `POST jobs/{id}/rerun/` | `(generated endpoint) jobs rerun` | — |
| 5 | List runs (filters) | API `GET runs/` | `dbt-cloud-pp-cli runs list` | status/job/project/env filters, `--json` |
| 6 | Get run (+steps) | API `GET runs/{id}/` | `dbt-cloud-pp-cli runs get` | `--steps` includes run_steps |
| 7 | Cancel run | API `POST runs/{id}/cancel/` | `(generated endpoint) runs cancel` | mutating; `--dry-run` |
| 8 | Retry run | API `POST runs/{id}/retry/` | `(generated endpoint) runs retry` | mutating; `--dry-run` |
| 9 | List run artifacts | API `GET runs/{id}/artifacts/` | `dbt-cloud-pp-cli runs artifacts list` | — |
| 10 | Get run artifact | API `GET runs/{id}/artifacts/{path}` | `dbt-cloud-pp-cli runs artifacts get` | fetch manifest/run_results/logs |
| 11 | Get job artifact | API `GET jobs/{id}/artifacts/{path}` | `(generated endpoint) jobs artifacts` | — |
| 12 | Run step detail | API `GET steps/{id}/` | `(generated endpoint) steps get` | — |

## Transcendence (only possible with our local-store + compound-workflow approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Watch a run to done | `monitor <run_id> [--interval]` | hand-code | Polls `runs/{id}/` until terminal; prints live status; **exit 0 on success / non-zero on failure**; dumps failed-step log tails. No API call gives you this. | Use to block CI/scripts on a run. |
| 2 | Trigger + wait | `trigger <job_id> --cause ... --wait` | hand-code | Compound: trigger job → capture run id → monitor to completion → surface failures, in one command + one exit code. The real day-to-day loop. | The headline workflow; `--wait` makes it CI-grade. |
| 3 | Run-history stats | `runs stats [--job <id>] [--days N]` | hand-code | Local join over synced runs: success rate, avg/p95 duration, failure count per job over a window. API returns one page, not trends. | Use for reliability/velocity review, not a single run. |
| 4 | Recent failures | `failures [--days N]` / `since <window>` | hand-code | Time-windowed scan of runs, grouped by job, with each run's failed-step names. "What broke today" across jobs. | Use for triage; not for one known run id. |
| 5 | Artifact diff | `artifacts diff <run_a> <run_b>` | hand-code | Fetches `run_results.json` for two runs and diffs model pass/fail/timing — which models newly failed or slowed. | Use to compare two runs; regression hunting. |
| 6 | Local run sync | `sync [--days N]` | spec-emits + hand-code | Mirrors run history into SQLite so stats/failures/diff work offline and fast. | Foundation for 3/4/5. |

All transcendence rows are user-shaped (drawn from your `monitor_dbt_run.py` + `dbt_cloud.py`). Minimum 5 met (6 listed).

## Stubs
None planned. All rows ship fully.

## Foundation wiring (not user-facing rows, but mandatory)
- `account_id` defaults from `DBT_CLOUD_ACCOUNT_ID` (flag override allowed).
- base URL from `DBT_CLOUD_HOST`.
- bearer auth from `DBT_CLOUD_TOKEN`.
