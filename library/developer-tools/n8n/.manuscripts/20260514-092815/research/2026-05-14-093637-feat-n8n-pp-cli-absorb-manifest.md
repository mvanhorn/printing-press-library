# n8n CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Apply/deploy local workflows with dry-run | ubie-oss/n8n-cli | --dry-run, --confirm, conflict detection | Go binary (no Node), --json output |
| 2 | Convert workflow files JSON ↔ YAML | ubie-oss/n8n-cli | Piped transform, stdin/stdout | Composable, --select fields |
| 3 | Import workflows from n8n to local | ubie-oss/n8n-cli | --output dir, --format yaml/json | Offline after sync |
| 4 | Lint workflow definitions | ubie-oss/n8n-cli | Configurable rules, --json violations | FTS on errors, --strict mode |
| 5 | Format (auto-organize node positions) | ubie-oss/n8n-cli | --dry-run preview | Fast, no GUI needed |
| 6 | Execute workflow via webhook | ubie-oss/n8n-cli, leonardsellem | --payload, --wait for result | Typed exit codes 0/2 |
| 7 | Workflow CRUD (list/get/create/update/delete) | all tools | Full lifecycle, --json, FTS offline | Works offline via sync |
| 8 | Workflow activate/deactivate | all tools | Bulk --ids list, idempotent | Dry-run, --confirm |
| 9 | Execution list/get/delete/retry/stop | all tools | Status filter, time range, --limit | Offline from store |
| 10 | Tag CRUD | all tools | --json, FTS, paginated | Offline from store |
| 11 | Credential CRUD + test + transfer | all tools | --json, type filter | Offline from store |
| 12 | Data table management (tables + rows + columns) | ubie-oss/n8n-cli, digital-boss | Full CRUD, upsert, delete rows | Agent-native row query |
| 13 | Node schema inspect (built-in types) | ubie-oss/n8n-cli | --json, searchable | Offline static reference |
| 14 | Trace data flow through workflow nodes | ubie-oss/n8n-cli | --execution-id, node-level | Agent-readable output |
| 15 | Workflow batch publish/save | digital-boss/n8n-manager | --tag filter, --dry-run | Idempotent, exit codes |
| 16 | Credential import/export between environments | digital-boss/n8n-manager | --from-instance, --to-instance profiles | Secret-masking in output |
| 17 | Data table migrate between environments | digital-boss/n8n-manager | --from/--to profiles | Dry-run, diff output |
| 18 | Community package install/uninstall/update | digital-boss/n8n-manager | --json, version pin | Doctor health check |
| 19 | API key create/delete | digital-boss/n8n-manager | --json output | CI-friendly |
| 20 | Git push (export with folder hierarchy) | tcoretech/n8n-git | --output dir, --format | Folder structure preserved |
| 21 | Git pull (import from git) | tcoretech/n8n-git | --input dir, conflict detect | Dry-run support |
| 22 | Time-travel git reset to any commit/tag/time | tcoretech/n8n-git | --commit, --tag, --at timestamp | Soft/hard reset modes |
| 23 | GitOps sync (apply only changed workflows) | edenreich/n8n-cli | --since last-sync tracking | Fast incremental |
| 24 | Execution run via API (manual trigger) | leonardsellem/n8n-mcp | --data payload, --wait | Typed exit codes |
| 25 | Execution retry failed | salacoste/mcp | --execution-id, idempotent | Exit 0/2 |
| 26 | Workflow/credential ownership transfer | kingler/n8n-mcp | --to-user, --project | Dry-run |
| 27 | Health/connectivity check (doctor) | kingler/n8n-mcp | Multi-instance aware | JSON output |
| 28 | Node-level operations (modify without full JSON) | Gladium-AI/n8n-cli | Stable node refs (n0,n1), surgical edits | Agent-native |
| 29 | Connection management (list/create/delete edges) | Gladium-AI/n8n-cli | --from-node, --to-node | Dry-run |
| 30 | Workflow graph analysis (roots/leaves/cycles) | Gladium-AI/n8n-cli | --format dot/json | Composes with graphviz |
| 31 | Workflow archive/unarchive | n8n API | --ids list, --dry-run | Missing from all tools |
| 32 | Bulk stop running executions | n8n API | --workflow filter | Safety guard |
| 33 | Variables CRUD | n8n API | --json, FTS | Missing from most tools |
| 34 | Projects CRUD + user management + folders | n8n API | --json, paginated | Missing from most tools |
| 35 | Source control pull | n8n API | --json | Missing from most tools |
| 36 | Insights summary (analytics) | n8n API | --json, --period filter | Missing from most tools |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Cross-instance health compare | `health compare` | 9/10 | Calls GET /workflows?active=true and GET /executions?status=error on each named profile config; renders comparison table | n8n community top request; no tool has multi-instance view |
| 2 | Stale workflow detector | `workflows stale` | 8/10 | Queries local execution store: active workflows with no executions in N days (default 30). JOIN workflows + executions tables in SQLite | Operational debt pattern; community requests for "cleanup old workflows" |
| 3 | Credential reference audit | `credentials audit` | 8/10 | Scans synced workflow node parameters for credential ID references; flags orphaned creds + active workflows with failing/deleted credentials | Credential misconfiguration = top failure cause; no absorbed tool cross-references workflow↔credential |
| 4 | Execution log export | `executions export` | 8/10 | Store query with --workflow, --status, --since, --until filters; emits CSV/JSON/NDJSON | Universal DevOps need; no absorbed tool has structured filterable export |
| 5 | Deployment diff | `diff` | 8/10 | Compares workflow sets across two instance profiles or local dir vs live; exits 1 on any diff (CI-safe). Uses workflow list + get on both sides | GitOps requirement; CI "did deploy apply?" use case unaddressed |
| 6 | Tag-based bulk operations | `workflows bulk` | 7/10 | --tag or --name-pattern selects workflow set; subcommands: activate/deactivate/archive/tag-add/tag-remove; --dry-run required before --confirm | Fleet management primitive; digital-boss has batch but no tag targeting |
| 7 | Node type inventory | `workflows node-inventory` | 7/10 | Aggregates node.type across all synced workflows; flags community-package-sourced types by cross-referencing package list | Pre-flight check before package removal; no absorbed tool has this |
| 8 | Workflow dependency map | `workflows deps` | 7/10 | Scans for executeWorkflow node type in local store; emits DOT/JSON adjacency list; flags cycles | Blast-radius audit before deactivating; composes with graphviz |
| 9 | CI execution wait gate | `executions wait` | 7/10 | Polls GET /executions/{id} until terminal; exits 0=success, 2=failure, 1=timeout; --timeout, --interval flags | CI/CD primitive; typed exit codes; not in any absorbed tool |
| 10 | Variable drift between instances | `variables diff` | 6/10 | GET /variables on two profiles; reports missing/extra/differing (secret values masked) | Multi-instance promotion safety; community discussion evidence |
| 11 | Execution rate watchdog | `executions rate-check` | 6/10 | Count executions in --window from local store for --workflow; exit 2 if > --threshold | Runaway trigger detection; cron/CI composable; fully offline |
