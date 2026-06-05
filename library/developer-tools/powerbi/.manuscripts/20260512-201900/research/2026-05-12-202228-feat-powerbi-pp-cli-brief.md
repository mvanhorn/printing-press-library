# Power BI REST API CLI Brief

## API Identity
- Domain: Microsoft Power BI service (`api.powerbi.com/v1.0/myorg`)
- Users: Data analysts, BI engineers, AI agents pulling Power BI data for downstream analysis
- Data profile: Workspaces ("groups") → Datasets / Reports / Dashboards / Dataflows. Datasets are queryable via DAX through `executeQueries`. Reports are exportable to PDF/PPTX/PNG. Dashboards expose tiles.

## Reachability Risk
- Low. `api.powerbi.com` returns HTTP 403 to unauthenticated requests (expected). Officially documented, GA, no bot mitigation. AAD bearer-token required for every endpoint.

## User Vision
- The user wants to **pull data** from Power BI so Claude can analyze it. Read-only. Not building a Power BI developer tool. Specifically interested in querying a report (`groups/804c5edc-6653-4149-8d08-a11279824b7a/reports/37ae3f5d-665b-4c6b-affe-37ebd176d9e5`) but the CLI should generalize to any workspace/report/dataset.

## Top Workflows
1. **Run a DAX query and stream the result as JSON/CSV.** `powerbi dax run "EVALUATE TOPN(100, Sales)" --workspace W --dataset D --csv` — primary data-pull path.
2. **List workspaces / reports / datasets to find the right ID.** `powerbi groups list`, `powerbi reports list --group W`. Needed before the user can query anything.
3. **Export a report to file.** `powerbi report export REPORT_ID --format pdf --output report.pdf` handling the async POST→poll→download flow in one command.
4. **Inspect dataset schema** (tables, columns, measures) so an agent knows what to query without trial and error.
5. **Pull refresh history** to know whether the data is fresh and which datasets are failing.

## Table Stakes (must match competitors)
- List/get workspaces, datasets, reports, dashboards, dataflows, apps
- DAX query execution (`executeQueries`)
- Report export with format selection (PDF/PPTX/PNG; XLSX/CSV/DOCX for paginated)
- Refresh history retrieval
- Both `myorg` (My workspace) and `groups/{groupId}` variants of every endpoint

## Data Layer
- Primary entities: `workspaces` (groups), `datasets`, `reports`, `dashboards`, `dataflows`, `apps`, `refreshes`
- Sync cursor: per-entity-type timestamp; refresh history takes a `$top` page size
- FTS/search: workspace+dataset+report names so an agent can fuzzy-find "the Sales report" without IDs

## Auth
- OAuth2 Bearer token, AAD-issued. Token endpoint `https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token` with scope `https://analysis.windows.net/powerbi/api/.default`.
- Two patterns supported:
  1. **BYO token** (simplest): `POWERBI_TOKEN` env var. Obtain via `az account get-access-token --resource https://analysis.windows.net/powerbi/api`.
  2. **Service principal** (`auth login`): `POWERBI_TENANT_ID` + `POWERBI_CLIENT_ID` + `POWERBI_CLIENT_SECRET`. CLI does the token exchange and caches the access token to `~/.config/powerbi-pp-cli/token.json`.
- Tokens expire after ~1 hour; the auth login flow refreshes automatically when expired.
- Required: AAD app registered, "Allow service principals to use Power BI APIs" tenant setting enabled, service principal added to each workspace.

## Product Thesis
- Name: `powerbi` (binary: `powerbi-pp-cli`)
- Why it should exist: `pbicli` (npm) covers REST but isn't agent-native and bundles 20 command groups including admin/embed/pipeline noise. Microsoft's Remote MCP server is Copilot-only and requires a hosted endpoint. Nothing on the market is a **read-only data-pull CLI with `--json`, `--csv`, `--select`, local SQLite catalog, and an `executeQueries` wrapper that handles the 100K-row limit gracefully**. That's the gap.

## Competitive Landscape
| Tool | Type | Strength | Weakness for our use case |
|---|---|---|---|
| `pbicli` (npm) | Cross-platform CLI | Comprehensive REST coverage | Not agent-native, broad scope incl. admin/write |
| Microsoft Remote MCP | Hosted MCP server | Natural language → DAX, NL insights | Copilot/MCP-client only, no CLI, no scripting |
| `Az.PowerBI` (PowerShell) | Microsoft module | Official, complete | Windows/PowerShell-first, verbose, not Unix-friendly |
| `MinaSaad1/pbi-cli` | Python CLI | Claude Code skill, model authoring | Windows-only, requires PBI Desktop, write-focused (different niche) |
| `sulaiman013/powerbi-mcp` | Community MCP | Free, NL queries | MCP-only, not a CLI |

## Build Priorities
1. **executeQueries wrapper** with `--json`, `--csv`, `--select`, `--file` (read query from file), `--out` (write to file), and graceful handling of the 100K row / 15MB / 120-rpm limits.
2. **Workspace/dataset/report listing** with local SQLite catalog for offline browsing.
3. **Report export + wait** — POST/poll/download in one synchronous command.
4. **Schema introspection** — best-effort DAX-based table/column listing.
5. **Refresh history view** — recent runs, failures, durations across all datasets.

## Spec Strategy
- Hand-craft an internal YAML spec covering ~25 read-only endpoints (datasets, reports, dashboards, dataflows, apps, groups). Excludes admin/write/embed/pipeline.
- Auth declared as `bearer_token` with env var `POWERBI_TOKEN`; `auth login` is a hand-built command in Phase 3.
