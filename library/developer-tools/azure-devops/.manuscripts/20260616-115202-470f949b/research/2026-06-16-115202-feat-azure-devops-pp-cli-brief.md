# Azure DevOps CLI Brief

## API Identity
- Domain: DevOps platform — work items, source control, CI/CD pipelines, test management, artifacts
- Users: Software engineers, DevOps engineers, tech leads, QA teams using Azure DevOps for project management and CI/CD
- Data profile: High-cardinality — millions of work items, thousands of builds, hundreds of repos and pipelines per org

## Reachability Risk
- Low — Official Swagger 2.0 specs from Microsoft (MicrosoftDocs/vsts-rest-api-specs), comprehensive coverage
- Host: `dev.azure.com`
- Auth required for all endpoints (PAT Basic auth, Entra OAuth, or browser session)
- Spec files downloaded: wit (80 ops), git (107 ops), build (87 ops), pipelines (10 ops), core (19 ops), work (57 ops), release (31 ops), search (6 ops), wiki (15 ops) — 412 operations across 9 areas

## Auth
- Primary: PAT via Basic auth — `Authorization: Basic base64(":" + PAT)`, env var `AZURE_DEVOPS_TOKEN`
- Secondary: Browser session via Entra ID (user confirmed logged-in session available)
- Configuration: `--org`/`AZURE_DEVOPS_ORG`, `--project`/`AZURE_DEVOPS_PROJECT`
- Note: Legacy ADO OAuth 2.0 being deprecated in 2026; PATs and Entra ID remain

## API Contract
- Base URL: `https://dev.azure.com/{organization}/{project}/_apis/...?api-version=7.1`
- Some org-level APIs: `https://dev.azure.com/{organization}/_apis/...`
- Some alt subdomains: `vssps.dev.azure.com` (identity/graph), `vsaex.dev.azure.com` (entitlements), `analytics.dev.azure.com` (OData)
- Pagination: `x-ms-continuationtoken` response header + `continuationToken` query param; work items use `$skip`/`$top`
- Rate limits: 200 TSTUs per user/pipeline per 5-min window; `Retry-After` header on 429
- Rate limit headers: `X-RateLimit-Resource`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `X-RateLimit-Cost`

## Top Workflows
1. **Sprint planning** — query backlog with WIQL, update work item states/assignments/iterations in batch, view sprint capacity
2. **Pipeline CI/CD automation** — trigger YAML pipeline runs with variables, monitor status, retrieve logs, approve gates
3. **Pull request workflow** — create/review PRs, manage reviewers, check policy status, add comments, auto-complete
4. **Work item bulk management** — batch create/update via WIQL, link work items, manage dependencies
5. **Branch and repo management** — create branches per work item, enforce branch policies, view commit history
6. **Release gate management** — approve/reject release environments, track deployment status across stages
7. **Analytics queries** — OData burndown/velocity data, test result trends, coverage metrics

## Table Stakes (from az devops + MCP servers)
- Work item CRUD + WIQL queries + batch operations
- Pull request create/review/vote/merge + comment threads
- Pipeline trigger/monitor/cancel/retry + log retrieval + artifact download
- Build list/queue/cancel + build definitions
- Repository list/clone/branch management
- Team/project management
- Search (code, work items, wiki)
- Wiki page CRUD
- Agent pool/queue management
- Variable groups + pipeline variables
- Service connections management
- Sprint/iteration management + capacity
- Board management (columns, work items per sprint)
- Release create/deploy/approve
- Test plan management

## Data Layer
- Primary entities: work items (with full revision history), builds/runs, pull requests, commits, wikis, sprints/iterations
- Sync cursor: `continuationToken` for most list endpoints; timestamp-based for work item revisions reporting API
- FTS/search: Work item text search via search API; local FTS5 on synced work item data for offline queries
- High-value local store: work item history/revisions (expensive to paginate live), build history (critical for trend analysis)

## Competing Tools
| Tool | Language | Stars | Key Gaps |
|------|----------|-------|----------|
| `az devops` (Microsoft) | Python | Official | No offline, no JSON-agent output, no novel analytics, no test plans, no search in CLI |
| microsoft/azure-devops-mcp | TypeScript | 1800+ | Browser-only delivery, no offline sync, no SQL queries, no trend analysis |
| Tiberriver256/mcp-server-azure-devops | TypeScript | 374 | MCP-only, no CLI, no offline |
| RyanCardin15/AzureDevOps-MCP | TypeScript | 56 | MCP-only |
| microsoft/azure-devops-go-api | Go | 224 | SDK only, no CLI |
| danielealbano/mcp-for-azure-devops-boards | Rust | 6 | Boards-only scope |

## Codebase Intelligence
- Source: microsoft/azure-devops-go-api
- Auth: `connections.NewPatTokenConnection(url, pat)` → standard Go http client with Basic auth
- Packages: 50+ namespaces covering every API area
- Key package: `azure-devops-go-api/azuredevops/workitemtracking` for WIT operations
- Rate limiting: No built-in rate limiter in the official Go SDK — must add ourselves

## User Vision
- Azure DevOps CLI with browser session support
- User has logged-in browser session (supports `auth login --chrome` pattern)

## Product Thesis
- Name: azure-devops-pp-cli
- Why it should exist: `az devops` is slow, Python-dependent, and produces human-only output. The Microsoft MCP server requires a browser agent. Neither works offline. Engineers who live in Azure DevOps need a fast, scriptable, agent-native CLI that syncs a local SQLite mirror of their critical data — work items, builds, PRs — and lets them query it with SQL, run cross-sprint analytics, and pipe results directly to AI agents via `--agent` output. The first CLI that works completely offline AND speaks both human and JSON natively for AI agents.

## Build Priorities
1. Work item management (WIQL queries, batch CRUD, sprint assignment, links) — the highest-value surface
2. Pull request lifecycle (create, review, vote, merge, comment threads) — daily workflow
3. Pipeline trigger/monitor/logs (YAML pipeline runs, build status, artifact retrieval) — CI/CD automation
4. Local SQLite sync (work items with revisions, builds, PRs) — enables offline analytics
5. Sprint analytics (velocity, burndown, capacity utilization) — novel value
6. PR aging and pipeline health reports — novel value that no existing tool provides
7. Branch management and repo operations — table stakes
8. Release management (deploy/approve gates) — CD workflows
9. Search (code, work items) — unified search across types
10. Wiki management — documentation workflows
