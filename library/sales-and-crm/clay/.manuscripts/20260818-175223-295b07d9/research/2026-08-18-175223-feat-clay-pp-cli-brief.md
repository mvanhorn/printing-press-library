# Clay CLI Brief

## API Identity
- Domain: GTM / lead generation data platform (tables + enrichment waterfalls)
- Users: GTM engineers, agency operators, RevOps, outbound teams
- Data profile: workbooks > tables > columns (fields) > rows (records); columns are
  text, formula, or `action` (enrichment / HTTP API) with input bindings

## Source Priority
- **Primary: `clay-app-internal`** — `https://api.clay.com/v3`, auth = `claysession`
  cookie. No official spec. Discovered by authenticated browser capture on
  2026-08-18. This is the ONLY surface that can author tables/columns/formulas/
  enrichments.
- **Secondary: `clay-documented-api`** — `https://api.clay.com/public/v0`, auth =
  `clay-api-key` header. Official OpenAPI 3.1, 13 endpoints. Read/search/routines only.
- **Economics:** primary needs no paid key (session cookie). Secondary needs a
  personal Public API key. Auth is scoped per surface via `composed` auth; each API
  ignores the other's credential (verified: cookie -> /public/v0 = 403).
- **Inversion risk:** the secondary has the clean official spec and the primary has
  none. Do NOT let spec availability promote the secondary to headline. The user's
  stated vision maps 100% to the primary.

## Reachability Risk
- None. `GET /v3/workspaces/{ws}` = 200 with session cookie.
  `GET /public/v0/me` = 200 with `clay-api-key`. Both verified live.
- Durability risk: MEDIUM-HIGH. `/v3` is an undocumented internal API behind a
  versioned frontend (`X-Clay-Frontend-Version`). Clay can change it without notice.

## Top Workflows
1. Author a lead-gen table from scratch: create table, add columns, set formulas,
   attach enrichment providers, wire an HTTP API column to an external data source.
2. Reproduce a proven table design in a new market/vertical (same columns, new query).
3. Inspect an existing table: schema, row count, per-column enrichment run status.
4. Discover which enrichment provider to use for a job, and which credential it needs.
5. Pull rows back out for downstream processing.

## Table Stakes
- List/get workbooks and tables; read column schema; read rows
- Search the enrichment catalog
- List connected provider accounts (API keys)
- `--json` everywhere, typed exit codes, dry-run on writes

## Data Layer
- Primary entities: workspace, workbook, table, column(field), account, enrichment
- Sync cursor: `updatedAt` on tables; `runstatus` per field for enrichment progress
- FTS/search: enrichment catalog (name + description), column names, table names

## Codebase Intelligence
- Official CLI `clay-run/agent-plugins` (Shell, 101 stars) ships `clay` + skills.
  Its `tables-cli` SKILL has an explicit "Not supported" section: creating tables,
  adding fields/columns, and writing data are all excluded. Public API is
  "query/list-only as well."
- Official MCP (mcp.clay.com) surface: search/enrich/routines/objects/credits.
  No table or column authoring tool.
- Column model learned from capture:
  - text:     `{type:text, typeSettings:{dataTypeSettings:{type:text}}}`
  - formula:  `{type:formula, typeSettings:{formulaText:"{{f_id}}", mappedResultPath}}`
  - action:   `{type:action, typeSettings:{actionKey, actionPackageId, actionVersion,
               inputsBinding[], authAccountId}}`
  - http-api: action with `actionKey:"http-api-v2"`, inputsBinding url/body/headers
- Formulas reference other columns by field id (`{{f_...}}`), so a table is a DAG.

## User Vision
Ade wants to author lead-gen tables entirely from the CLI: create tables, create
columns, put formulas in columns, configure enrichments, make HTTP API calls, and
manage the API keys those calls need. The motivating use case is a local-services lead-gen table built from an
external SERP data provider wired in through an HTTP API column.

## Product Thesis
- Name: `clay-pp-cli`
- Why it should exist: Clay's own CLI and MCP can only read tables. Everything that
  makes a Clay table valuable (the column graph: formulas, enrichment bindings, HTTP
  API wiring) is UI-only and therefore unversionable, unreviewable, and
  unreproducible. This CLI makes the column graph a file you can commit.

## Build Priorities
1. Data layer + sync for workspace/workbook/table/column/account/enrichment
2. Authoring commands: tables create/update, columns create/update (text, formula,
   action), formulas generate, enrichments search
3. Transcendence: table blueprints (export/apply), column dependency graph,
   enrichment cost preflight, run-status watch
