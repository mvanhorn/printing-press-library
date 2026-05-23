# Vibecode CLI Absorb Manifest

## Sources Analyzed
- Official vibecode-cli (github.com/vibecode/vibecode-cli) - 40+ commands
- @vibecodeapp/webapp npm SDK
- @vibecodeapp/v npm package
- vibecode.dev platform documentation

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | User profile | vibecode-cli user | vibecode-pp-cli user | Works offline after first sync, shows local cache age |
| 2 | List projects | vibecode-cli projects list | vibecode-pp-cli projects list | Offline, FTS search, --since filter |
| 3 | Get project | vibecode-cli projects get | vibecode-pp-cli projects get | Cached locally, instant retrieval |
| 4 | Create project | vibecode-cli projects create | vibecode-pp-cli projects create --stdin --dry-run | Agent-native, scriptable, idempotent check |
| 5 | Rename project | vibecode-cli projects rename | vibecode-pp-cli projects rename | --dry-run support |
| 6 | Delete project | vibecode-cli projects delete | vibecode-pp-cli projects delete | Confirmation, --force, dry-run |
| 7 | Project commits | vibecode-cli projects commits | vibecode-pp-cli projects commits | Local cache, FTS on commit messages |
| 8 | List sandboxes | vibecode-cli sandboxes list | vibecode-pp-cli sandboxes list | Offline, status filtering |
| 9 | Get sandbox | vibecode-cli sandboxes get | vibecode-pp-cli sandboxes get | Cached status |
| 10 | Acquire sandbox | vibecode-cli sandboxes acquire | vibecode-pp-cli sandboxes acquire | --dry-run, timeout control |
| 11 | Kill sandbox | vibecode-cli sandboxes kill | vibecode-pp-cli sandboxes kill | Confirmation, --force |
| 12 | SSH sandbox | vibecode-cli sandboxes ssh | vibecode-pp-cli sandboxes ssh | Same capability |
| 13 | Sandbox links list | vibecode-cli sandboxes links list | vibecode-pp-cli sandboxes links list | Cached locally |
| 14 | Sandbox links create | vibecode-cli sandboxes links create | vibecode-pp-cli sandboxes links create | --dry-run |
| 15 | Sandbox links delete | vibecode-cli sandboxes links delete | vibecode-pp-cli sandboxes links delete | Confirmation |
| 16 | List deployments | vibecode-cli deployments list | vibecode-pp-cli deployments list | Offline, status filtering, --since |
| 17 | Get deployment | vibecode-cli deployments get | vibecode-pp-cli deployments get | Cached details |
| 18 | Deploy | vibecode-cli deployments deploy | vibecode-pp-cli deployments deploy | --dry-run, progress streaming |
| 19 | Destroy deployment | vibecode-cli deployments destroy | vibecode-pp-cli deployments destroy | Confirmation, --force |
| 20 | Deployment ready | vibecode-cli deployments ready | vibecode-pp-cli deployments ready | Same capability |
| 21 | SSH deployment | vibecode-cli deployments ssh | vibecode-pp-cli deployments ssh | Same capability |
| 22 | Get auth config | vibecode-cli deployments auth get | vibecode-pp-cli deployments auth get | Cached |
| 23 | Set auth | vibecode-cli deployments auth set | vibecode-pp-cli deployments auth set | --dry-run |
| 24 | Disable auth | vibecode-cli deployments auth disable | vibecode-pp-cli deployments auth disable | --dry-run |
| 25 | Check subdomain | vibecode-cli deployments subdomain check | vibecode-pp-cli deployments subdomain check | Same capability |
| 26 | Set subdomain | vibecode-cli deployments subdomain set | vibecode-pp-cli deployments subdomain set | --dry-run |
| 27 | Get domain | vibecode-cli deployments domain get | vibecode-pp-cli deployments domain get | Cached |
| 28 | Set domain | vibecode-cli deployments domain set | vibecode-pp-cli deployments domain set | --dry-run |
| 29 | Verify domain | vibecode-cli deployments domain verify | vibecode-pp-cli deployments domain verify | Same capability |
| 30 | Remove domain | vibecode-cli deployments domain remove | vibecode-pp-cli deployments domain remove | --dry-run, confirmation |
| 31 | Deployment links list | vibecode-cli deployments links list | vibecode-pp-cli deployments links list | Cached |
| 32 | Deployment links create | vibecode-cli deployments links create | vibecode-pp-cli deployments links create | --dry-run |
| 33 | Deployment links delete | vibecode-cli deployments links delete | vibecode-pp-cli deployments links delete | Confirmation |
| 34 | Agent send | vibecode-cli agent send | vibecode-pp-cli agent send | Streaming, session tracking |
| 35 | Agent stop | vibecode-cli agent stop | vibecode-pp-cli agent stop | Same capability |
| 36 | Yolo (build+deploy) | vibecode-cli yolo | vibecode-pp-cli yolo | Session tracking, local logging |
| 37 | Text output mode | vibecode-cli --output text | (behavior in vibecode-pp-cli --output text) Default logfmt output |
| 38 | JSON output mode | vibecode-cli --output json | (behavior in vibecode-pp-cli --json) Full structured JSON |
| 39 | Quiet mode | vibecode-cli --quiet | (behavior in vibecode-pp-cli --quiet) ID-only output |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Score |
|---|---------|---------|--------------|------------------------|-------|
| 1 | Cross-Project Search | search | spec-emits | Full-text search across locally-indexed project names, commit messages, deployment URLs. Official CLI searches one entity type via API per call. | 9 |
| 2 | Offline Project Cache | sync | spec-emits | Stores full project/deployment/commit snapshots for offline reference. Official CLI requires network for every read operation. | 9 |
| 3 | Since-Style Delta Commands | changes --since | spec-emits | Compares current API state against timestamped local snapshots. Official CLI has no "previous state" concept to diff against. | 9 |
| 4 | Deployment Drift Detection | drift | hand-code | Requires comparing current API state against locally-cached historical snapshots to detect env var changes, build setting modifications. | 9 |
| 5 | Stale Deployment Finder | stale --days N | spec-emits | Queries local deployment history table with timestamps. Finds deployments not updated in N days. | 8 |
| 6 | Build Duration Trends | metrics builds | hand-code | Aggregates historical build times from local store. Shows average build time, p95, trend arrows (improving/degrading). | 8 |
| 7 | Deployment History Graph | history --graph | hand-code | Visualizes deployment ancestry with local commit and deployment linkage. API returns flat lists; graph requires persistent joins. | 8 |
| 8 | Batch Deploy | batch deploy | hand-code | Orchestrates deployments across multiple projects matching a glob pattern with parallelism control and progress tracking. | 8 |
| 9 | Watch Mode | watch | hand-code | Polls API, stores state deltas in SQLite, triggers OS notifications on project/deployment changes. | 7 |
| 10 | Agent Session Tracking | sessions | hand-code | Groups sequential agent commands by session. Shows what Claude Code or Cursor did during a work session. | 7 |

## Feature Count Summary
- Absorbed: 39 features
- Transcendence: 10 features
- Total: 49 features

## Hand-Code Commitment
Of the 10 transcendence features, 6 require hand-written Go:
1. drift (deployment diff logic)
2. metrics builds (duration aggregation)
3. history --graph (visualization)
4. batch deploy (orchestration)
5. watch (polling + notifications)
6. sessions (command grouping logic)

The remaining 4 (search, sync, changes, stale) are spec-emits and will be handled by the generator.

## Stubs
No features are stubbed. All 49 features are shipping scope.

## Data Layer Requirements
- Projects table: id, name, platform, description, subdomain, domain, created_at, updated_at
- Sandboxes table: id, project_id, status, links_json, created_at
- Deployments table: id, project_id, status, url, commit_sha, auth_enabled, created_at, deployed_at
- Commits table: id, project_id, sha, message, created_at
- Links table: id, entity_type, entity_id, port, url, created_at
- Sessions table: id, project_id, agent_type, prompts_json, started_at, ended_at
- Sync metadata: entity_type, last_sync_at
