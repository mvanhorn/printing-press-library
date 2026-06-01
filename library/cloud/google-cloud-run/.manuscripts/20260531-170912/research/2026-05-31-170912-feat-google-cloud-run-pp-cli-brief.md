# Google Cloud Run CLI Brief

## API Identity
- Domain: Serverless container platform on Google Cloud (deploy, manage, scale containers)
- Users: DevOps engineers, SREs, platform teams, CI/CD pipelines, solo developers
- Data profile: Services (long-running HTTP containers), Jobs (batch containers), Revisions, Executions, Tasks, Operations, IAM policies

## Reachability Risk
- None — Cloud Run Admin API v2 is a stable, publicly reachable API at `https://run.googleapis.com`
- 404 on unauthenticated probe = expected (not 403/blocked)
- Probe-safe endpoint: `GET /v2/projects` (returns 401 without auth)

## Top Workflows
1. **Deploy and manage services** — create/update Cloud Run services, control traffic splits between revisions
2. **Run batch jobs** — create Cloud Run Jobs, trigger executions, monitor job status and logs
3. **Revision management** — list revisions, delete old ones, check which revision is serving traffic
4. **IAM / access control** — get/set IAM policies on services, test permissions, make services public
5. **Operations monitoring** — list/wait on long-running operations (deployments, deletions)

## Table Stakes
- `gcloud run services list/describe/update/delete` — the incumbent baseline
- `gcloud run jobs create/execute/list/describe` — job management baseline
- `gcloud run revisions list/describe/delete` — revision lifecycle
- `gcloud run operations list/describe` — async operation tracking
- `@google-cloud/cloud-run-mcp` tools: list-services, get-service, get-service-log, deploy
- `run-cli` (JulienBreux): interactive TUI dashboard, project/region switching, revision history, log streaming

## Data Layer
- Primary entities: services, jobs, revisions, executions, tasks, operations, locations
- Sync cursor: service/job `updateTime` timestamp (ISO 8601)
- FTS/search: service names, job names, revision names, container images, labels/annotations

## Codebase Intelligence
- Source: GoogleCloudPlatform/cloud-run-mcp analysis
- Auth: Bearer token, `Authorization: Bearer {token}`, ADC cascade (explicit token → GOOGLE_APPLICATION_CREDENTIALS → ADC)
- Data model: Services (with Revisions for versioning), Jobs (with Executions and Tasks), Operations (async wrappers)
- Rate limiting: Standard Cloud Run Admin API — 300 reads/min, 60 writes/min per project
- Architecture: Google Discovery API → operationId-encoded resource hierarchy; paths use `/v2/{name}` pattern with resource type in operationId

## Product Thesis
- Name: `google-cloud-run-pp-cli`
- Why it should exist: `gcloud run` requires the full Cloud SDK installed, uses positional arguments and verbose flags, and is unscriptable for agents. This CLI provides: (1) offline service/job/revision state in SQLite so agents can query without API calls, (2) agent-native JSON output with `--select`, (3) deployment drift detection, (4) multi-region health matrix, and (5) job failure pattern analysis that gcloud never attempted.

## Build Priorities
1. Services and Jobs CRUD with all 16 spec endpoints (+ sync to local SQLite)
2. IAM management (getIamPolicy, setIamPolicy, testIamPermissions)
3. Revision and execution lifecycle management
4. Novel: deployment drift detector, multi-region health check, cold start risk analysis
5. Job failure pattern aggregator, dependency graph via env var analysis
