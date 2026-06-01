# Google Cloud Run CLI — Build Log

## Manifest tracking
Manifest transcendence rows: 8 planned, 8 built. All shipped.

## Priority 0 (data layer)
- Generated: sync path for services, jobs, revisions, executions, tasks (via catalog spec)
- SQLite store at ~/.local/share/google-cloud-run-pp-cli/data.db

## Priority 1 (absorbed features — 24 rows)
- All 24 absorbed features shipped as generated endpoint commands
- Key commands: services list/get/create/patch/delete, cloud-run-admin-jobs list/create/run, services revisions list/get/delete, executions tasks list, services get-iam-policy/set-iam-policy/test-iam-permissions, operations list/wait
- Note: API's "jobs" resource conflicts with framework; renamed to cloud-run-admin-jobs

## Priority 2 (transcendence — 8 hand-code rows)
All 8 transcendence features built and verified:

| # | Command | Status |
|---|---------|--------|
| T1 | services list-all | ✅ Built — loops List Services across projects |
| T2 | revisions prune | ✅ Built — filters non-serving revisions + bulk delete |
| T3 | services traffic | ✅ Built — traffic split table + per-revision image join |
| T4 | executions summary | ✅ Built — execution aggregate (duration, succeeded/failed) |
| T5 | iam audit | ✅ Built — project-wide IAM scan for allUsers/allAuthenticatedUsers |
| T6 | revisions wait-traffic | ✅ Built — CI/CD polling gate on trafficStatuses |
| T7 | revisions diff | ✅ Built — two-revision config diff, env var keys only |
| T8 | iam diff | ✅ Built — current IAM vs. local snapshot diff |

## Intentionally deferred
- Worker Pools: documented in official docs but NOT in the 16-endpoint OpenAPI spec. Deferred pending spec enrichment or a future reprint.
- Cloud Logging integration: executions summary intentionally uses execution metadata (task count, status) rather than log content — Cloud Logging is a separate API outside spec scope.

## Generator limitations found
- None blocking. The resource "jobs" conflict (renamed to cloud-run-admin-jobs) is expected behavior.
