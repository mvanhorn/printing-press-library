# Airflow Admin Local Validation Summary

Validation used a disposable local Apache Airflow 3.0.0 Docker container with explicit test-only credentials (`airflow` / `airflow`). The CLI was pointed at `http://127.0.0.1:18080` with `AIRFLOW_ADMIN_BASE_URL`. The JWT token was stored only in a temporary config file under `/private/tmp` and is not present in this repository.

## Environment

- Airflow image: `apache/airflow:3.0.0`
- Airflow API base URL: `http://127.0.0.1:18080`
- CLI source: generated `airflow-admin-pp-cli`
- Auth: Airflow `/auth/token` JWT flow using local test credentials
- Test DAG: `pp_validation_daily_refresh`, copied into the container under `/opt/airflow/dags`

## Checks

- `apache-airflow-admin-auth --username airflow --password airflow --json --select token_type` reached `/auth/token` through the CLI without printing the JWT.
- `auth set-token <token> --config /private/tmp/airflow-admin-validation-20260615.toml --json --select saved` returned `saved: true`.
- `apache-airflow-admin-version --json --select version` returned Airflow `3.0.0`.
- `monitor --json` returned healthy metadatabase, scheduler, triggerer, and DAG processor statuses.
- `dags list --json --select dag_id,is_active,is_paused --limit 10` returned live DAG metadata from Airflow.
- `pools list --json --select name,slots,occupied_slots --limit 3` returned `default_pool` with `128` slots and `0` occupied slots.
- `dags dag-runs list pp_validation_daily_refresh --json --select dag_id,dag_run_id,state,start_date,end_date` returned live DAG run rows.
- `dags dag-runs get pp_validation_daily_refresh <dag_run_id> --json --select dag_id,dag_run_id,state,start_date,end_date` returned the validation run with state `failed`.
- `dags dag-runs list-task-instances pp_validation_daily_refresh <dag_run_id> --json --select task_id,state,try_number,start_date,end_date` returned `extract` and `load` as `success`, and `intentional_failure` as `failed`.
- `sync --resources dags,pools --json --max-pages 1` stored Airflow envelopes correctly after adding explicit Airflow wrapper-key handling. The summary reported `total_records: 4`, `success: 3`, and `errored: 0`.
- `search validation --data-source local --json --limit 10` returned the synced validation DAG and failed DAG-run rows from the local SQLite store.
- Focused regression tests covered Airflow response envelopes and Airflow identifier fields used by search filtering.

## Notes

The validation DAG intentionally fails one task so the CLI can prove the common data-engineering triage path: find a failed DAG run, list task instances, and identify the failing task. No customer data, production Airflow URL, production token, tenant identifier, or generated JWT is committed.
