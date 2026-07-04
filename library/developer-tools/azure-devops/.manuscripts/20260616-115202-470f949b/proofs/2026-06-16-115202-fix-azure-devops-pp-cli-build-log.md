# Azure DevOps CLI — Phase 3 Build Log

Manifest transcendence rows: 15 planned, 0 built. Phase 3 will not pass until all 15 ship.

## Priority 0: Foundation

### Auth fix (COMPLETED)
- Fixed `internal/config/config.go`: Added `AZURE_DEVOPS_TOKEN` and `ADO_PAT` env var support
- Fixed `internal/config/config.go`: Allow empty username in Basic auth (ADO PAT format is `:PAT`)
- Fixed `internal/cli/doctor.go`: Auth hint now shows `export AZURE_DEVOPS_TOKEN=<your-pat>`
- Added `--org`, `--project`, `--team` flags to rootFlags (reads from `AZURE_DEVOPS_ORG`, `AZURE_DEVOPS_PROJECT`, `AZURE_DEVOPS_TEAM`)

### Sync resources (PENDING)
- `defaultSyncResources()` returns empty — Azure DevOps requires `{org}` and `{project}` path params in all endpoints
- Novel commands use live API calls with `--org` and `--project` flags
- Local SQLite store can be populated via `sync --resources` but requires manual org/project configuration

## Priority 1: Absorbed Features
- Generated 282 endpoint commands from 9 official Swagger 2.0 specs
- All core ADO operations covered: projects, teams, work items, PRs, builds, pipelines, releases, search, wiki
- All endpoint commands use Basic auth with AZURE_DEVOPS_TOKEN
- Cloudflare MCP pattern applied (code orchestration + hidden endpoint tools)

## Priority 2: Transcendence Features (in progress)

| Feature | Command | Status | Notes |
|---------|---------|--------|-------|
| Sprint velocity trend | velocity | stub | Needs iteration + work item data |
| PR aging report | pr aging | stub | Needs PR + timeline data |
| Daily standup digest | standup | stub | Live API: PRs + WIT + builds |
| Pipeline flakiness score | pipeline flaky | stub | Needs build history |
| Work item cycle time | wit cycle-time | stub | Needs revision history |
| Branch stale cleanup plan | branches stale | stub | Needs branches + PRs |
| Build cost trend | builds cost | stub | Needs build history |
| Cross-pipeline pending approvals | release gate-queue | stub | Live API: approvals endpoint |
| Sprint scope creep detector | work sprint-creep | stub | Needs sprint + revision data |
| PR review readiness queue | pr review-queue | stub | Live API: PRs + build status |
| Work item field diff | work diff | stub | Live API: revisions endpoint |
| Sprint rollover counter | work rollover | stub | Needs revision history |
| Area path work item load | work area-load | stub | Needs work items |
| Commit-to-build traceability | git commit-builds | stub | Needs builds + changes |
| Cross-repo branch health | git branch-health | stub | Needs repos + branches + builds |

## Priority 3: Polish
- TODO: Flag descriptions (replace "TODO: describe --xxx" strings)
- TODO: Examples with realistic Azure DevOps values
