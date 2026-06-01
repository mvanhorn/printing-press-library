# Google Cloud Run CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List services in a project/region | gcloud run services list | google-cloud-run-pp-cli services list --project --region --json | Offline (SQLite), FTS search, --select fields |
| 2 | Get service details | gcloud run services describe | google-cloud-run-pp-cli services get --name --region | --json, --select, deep revision info |
| 3 | Create service | gcloud run services create | google-cloud-run-pp-cli services create --image --name | --dry-run (validateOnly), --json output |
| 4 | Update service (deploy) | gcloud run services update / deploy | google-cloud-run-pp-cli services update --name --image --region | --dry-run, typed exit codes |
| 5 | Delete service | gcloud run services delete | google-cloud-run-pp-cli services delete --name --region | --dry-run, confirmation guard |
| 6 | List revisions | gcloud run revisions list | google-cloud-run-pp-cli revisions list --service --region | Offline FTS, traffic % shown |
| 7 | Get revision | gcloud run revisions describe | google-cloud-run-pp-cli revisions get --name | Full JSON spec output |
| 8 | Delete revision | gcloud run revisions delete | google-cloud-run-pp-cli revisions delete --name | --dry-run guard |
| 9 | Create job | gcloud run jobs create | google-cloud-run-pp-cli jobs create --name --image | --dry-run |
| 10 | List jobs | gcloud run jobs list | google-cloud-run-pp-cli jobs list --project --region | Offline, FTS, --json |
| 11 | Run/execute job | gcloud run jobs run | google-cloud-run-pp-cli jobs run --name --region | Returns operation ID, --wait flag |
| 12 | List executions | gcloud run jobs executions list | google-cloud-run-pp-cli executions list --job --region | Success/fail counts per execution |
| 13 | List tasks | gcloud run jobs executions tasks list | google-cloud-run-pp-cli tasks list --execution | Per-task exit codes |
| 14 | Get IAM policy | gcloud run services get-iam-policy | google-cloud-run-pp-cli iam get --resource | --json, --select member/role |
| 15 | Set IAM policy | gcloud run services set-iam-policy | google-cloud-run-pp-cli iam set --resource --member --role | --dry-run |
| 16 | Test IAM permissions | gcloud run services check-iam-policy | google-cloud-run-pp-cli iam test --resource --permissions | Batch permission testing |
| 17 | List operations | gcloud run operations list | google-cloud-run-pp-cli operations list --project --region | Shows completion status |
| 18 | Wait for operation | gcloud run operations wait | google-cloud-run-pp-cli operations wait --name | --timeout flag, exit 0 on success |
| 19 | List services (MCP) | cloud-run-mcp list-services | (generated endpoint) services list | Identical but with offline cache |
| 20 | Get service (MCP) | cloud-run-mcp get-service | (generated endpoint) services get | Identical |
| 21 | Get service logs (MCP) | cloud-run-mcp get-service-log | google-cloud-run-pp-cli logs --service --region --lines | --json, --select timestamp/severity/text |
| 22 | deploy-file-contents (MCP) | cloud-run-mcp deploy-file-contents | google-cloud-run-pp-cli services deploy --file --name | Source-based deployment |
| 23 | Interactive region/project switch | run-cli (JulienBreux) | google-cloud-run-pp-cli config set-project / config set-region | Persistent config, doctor checks |
| 24 | validateOnly dry-run | cloud-run-mcp (internal) | google-cloud-run-pp-cli services create/update --dry-run | Exposes validateOnly to CLI users |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Deployment drift detection | drift | hand-code | Requires comparing live service spec vs stored baseline in SQLite — no single API call reveals if a service changed unexpectedly | Use this command to detect unexpected service changes since last sync. Compare current spec to baseline snapshot. Do NOT use for IAM changes; use 'iam get' instead. |
| 2 | Multi-region health matrix | health-matrix | hand-code | Requires parallel GET across all regions × all services; no API aggregates this | Shows which services are healthy across all regions simultaneously. Do NOT use for single-region status; use 'services get' instead. |
| 3 | Cold start risk analysis | cold-start | hand-code | Requires joining minInstances, concurrency, and container memory config from local SQLite | Identifies services likely to suffer cold starts based on min-instances=0, high memory, or low concurrency. none |
| 4 | Job failure pattern aggregator | job-patterns | hand-code | Requires windowed aggregation over execution history not possible in a single API call | Find jobs with recurring failures grouped by time window. none |
| 5 | Service dependency graph | dep-graph | hand-code | Requires parsing env vars across all services to find service-to-service URLs encoded in environment | Maps which services call each other via environment variable URL references. none |
| 6 | Revision traffic vs error correlation | revision-health | hand-code | Requires joining revision traffic % with execution failure data from SQLite — not available in any single endpoint | Surfaces which revision is serving the most traffic relative to its observed error rate. none |

## Transcendence — Updated (after subagent adversarial cut)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|-----------------|
| T1 | Cross-project service inventory | services list-all | 9/10 | hand-code | Loop List Services per project; join client-side; display unified table | Maya's multi-project need; any multi-project team | none |
| T2 | Zombie revision sweeper | revisions prune | 9/10 | hand-code | List Revisions → filter traffic-serving → delete non-keepers; --dry-run shows plan | Maya spends 40 min/week manually; revision quota exhaustion documented | none |
| T3 | Traffic split inspector | services traffic | 8/10 | hand-code | Get Service (traffic fields) + Get Revision per entry for image tag; aligned table | Every deploy; canary workflows; Carlos's CI/CD dependency | Shows traffic split table with image tag; requires Get Revision per entry to surface image info absent on Service object. |
| T4 | Execution outcome summary | executions summary | 9/10 | hand-code | List Executions + List Tasks per execution; aggregate succeeded/failed/duration | Dmitri's "git log for jobs"; any team running scheduled Jobs | none |
| T5 | Public exposure audit | iam audit | 9/10 | hand-code | List Services + Get IAM Policy per service; flag allUsers/allAuthenticatedUsers bindings | Priya's weekly ritual; security teams | none |
| T6 | Wait-for-traffic | revisions wait-traffic | 9/10 | hand-code | Poll Get Service; inspect trafficStatuses for revision at target %; exit 0 on success | Carlos's 3 on-call incidents; CI/CD gate need; distinct from operation wait | Polls trafficStatuses until revision reaches target %; distinct from operation wait, which completes before traffic shifts. |
| T7 | Revision config diff | revisions diff | 8/10 | hand-code | Get Revision × 2; local diff on image, limits, scaling, service account, VPC, env keys | Incident review; canary investigation | Shows field-by-field diff; env var values omitted to avoid secret exposure. |
| T8 | IAM policy diff | iam diff | 7/10 | hand-code | Get IAM Policy + read local snapshot; diff bindings | Priya's compliance drift detection | Diffs current IAM policy against JSON snapshot from `iam get --output-file`. |
