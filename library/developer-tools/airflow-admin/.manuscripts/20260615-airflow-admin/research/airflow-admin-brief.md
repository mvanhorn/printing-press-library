# Airflow Admin Research Brief

Apache Airflow is a workflow orchestrator for data pipelines. Data engineers use it to schedule and monitor DAGs that run warehouse loads, API extracts, dbt jobs, reports, and machine-learning workflows. A DAG run shows whether a pipeline ran successfully for a schedule or manual trigger. Task instances show the individual steps inside that run, including retries, failures, and timing.

The Airflow UI is still the main operator surface, but agents and engineers often need a read-only command-line path for incident triage. The useful first questions are simple: is Airflow healthy, which DAG run failed, which task failed, and can this metadata be searched locally without repeatedly paging the API?

This print focuses on that operations loop:

- `monitor` checks Airflow component health.
- `dags list` gives a compact DAG inventory.
- `dags dag-runs list` filters run history by DAG, state, and time window.
- `dags dag-runs list-task-instances` identifies failed or retrying tasks inside a run.
- `sync` stores Airflow metadata in SQLite so repeated search and analysis can happen locally.

Local validation used Apache Airflow 3.0.0 in a disposable Docker container with test-only credentials. The validation DAG intentionally failed one task so the CLI could prove the normal data-engineering flow: authenticate, confirm Airflow health, find the failed DAG run, inspect task instances, sync metadata, and search the local store. No customer data, production URL, production token, or generated JWT is committed.
