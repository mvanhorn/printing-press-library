# n8n CLI Brief

## API Identity
- **Domain:** Workflow automation platform (self-hosted + cloud). REST API for managing workflows, executions, credentials, variables, data tables, projects, users, community packages, source control, insights, and folders.
- **Users:** DevOps/automation engineers, RevOps teams, developers who self-host n8n. Power users managing multiple instances, CI/CD pipelines, and automation governance.
- **Data profile:** Workflows (JSON DAGs), Executions (logs + status), Credentials (secrets), Variables, Tags, Data Tables (dynamic datasets), Projects (multi-user workspaces), Insights (analytics).
- **Spec:** `https://raw.githubusercontent.com/n8n-io/n8n-docs/main/docs/api/v1/openapi.yml` — v1.1.1, 47 paths, ~85+ operations.

## Reachability Risk
- **Low.** 401/403 issues in the community are about OAuth credentials *within* n8n, not about the public REST API being blocked. API key auth via `X-N8N-API-KEY` header is stable and well-documented. Live instance at `n8n.srv1165730.hstgr.cloud` confirms `/healthz` is reachable.

## Top Workflows
1. **Workflow lifecycle management** — list, activate/deactivate, archive, monitor failures, bulk-update tags
2. **Execution triage** — filter running/failed executions, retry failed, stop runaway executions, export execution logs
3. **GitOps / source control** — sync workflows to/from git, import/export for CI/CD, diff environments
4. **Instance health + monitoring** — check active workflow count, failure rates, insights summary, doctor health check
5. **Multi-instance management** — manage credentials/variables/packages across staging/prod, transfer workflows between projects

## Table Stakes (competitors have these)
From absorbing ubie-oss/n8n-cli, digital-boss/n8n-manager, tcoretech/n8n-git, edenreich/n8n-cli, leonardsellem/n8n-mcp-server:
- List/get/create/update/delete workflows
- Activate and deactivate workflows
- List/get/delete/retry/stop executions
- List/get/create/delete/test credentials
- Manage tags, variables, projects, users
- List and install community packages
- Source control pull
- YAML import/export with format conversion
- Lint workflow definitions
- Git sync (push/pull with folder hierarchy)
- Webhook-triggered workflow execution

## Data Layer
- **Primary entities:** Workflow, Execution, Credential, Tag, Variable, DataTable, Project, User, CommunityPackage, Folder
- **Sync cursor:** Execution list is paginated via `cursor`, supports status filtering; workflow list supports `active` filter
- **FTS/search:** Local SQLite FTS5 over workflow names, tags, node types used, execution status/error messages
- **High-gravity fields:** workflow.active, execution.status, execution.startedAt, workflow.updatedAt, execution.workflowId

## Codebase Intelligence
- Auth: `X-N8N-API-KEY` header (primary), `Authorization: Bearer <jwt>` (secondary)
- Env vars: `N8N_API_KEY`, `N8N_BASE_URL` (e.g., `https://n8n.example.com`)
- Data model: Workflow → has many Executions; Execution has status ∈ {success,error,running,waiting,new,canceled,crashed,unknown}; Credential has type (githubApi, stripeApi, etc.); Project is a workspace container
- Rate limiting: Not documented explicitly; standard self-hosted limits apply
- Architecture: REST, JSON, cursor pagination, filter-by-status on executions

## User Vision
- (none provided — will be derived from ecosystem research)

## Product Thesis
- **Name:** n8n-pp-cli
- **Why it should exist:** Every existing tool either focuses on one dimension (GitOps, lint, basic CRUD) or requires a Node.js runtime. This CLI gives power users a fast, offline-capable, agent-native Go binary that combines workflow management, execution triage, health monitoring, GitOps, and novel analytics features — working whether the n8n instance is reachable or not via a local SQLite store. It replaces 5 separate tools with one that supports --json, --select, typed exit codes, and FTS across all entities.

## Build Priorities
1. **Foundation:** SQLite store for workflows, executions, credentials (sync + FTS)
2. **Core CRUD:** workflows, executions, credentials, tags, variables — full lifecycle
3. **Instance health:** doctor command, insights summary, running execution count, failure rate
4. **GitOps:** export-to-yaml, import-from-yaml, diff between local and remote
5. **Transcendence:** bottleneck detection, failure analysis, drift detection, execution timeline, workflow dependency graph
