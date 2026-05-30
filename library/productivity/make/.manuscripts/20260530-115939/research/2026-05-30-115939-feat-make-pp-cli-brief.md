# Make.com CLI Brief

## API Identity
- **Domain:** Make.com Management API (https://{zone}.make.com/api/v2/) — formerly Integromat
- **Auth:** `Authorization: Token <api_token>` header. Token created at profile → API. No OAuth on the management API; an MCP-only token type also exists.
- **Zones:** `us1`, `us2`, `eu1`, `eu2`. The token is bound to one zone — wrong zone returns 401.
- **Users:** ops/automation engineers running production scenarios on Make; consultants moving scenarios dev → prod; SmartSuite/Notion/HubSpot integrators; AI agent builders who want to trigger Make workflows synchronously.
- **Data profile:** Scenarios (high gravity), Blueprints (versionable JSON), Executions/Logs (per-scenario history), Connections (auth grants), Hooks (webhooks), DLQs (incomplete executions), Data Stores + Records, Data Structures, Folders, Templates, Devices, Keys, Teams, Organizations.

## Reachability Risk
- **None.** Live probe against `us1.make.com/api/v2/` returned 200 on `/users/me`, `/organizations`, `/teams`, `/scenarios?teamId=`, `/scenarios/{id}/blueprint`, `/connections`, `/hooks`, `/data-stores`, `/data-structures`, `/templates`, `/dlqs?scenarioId=`, `/keys`, `/devices`, `/scenarios-folders`. Token has 60+ scopes including `scenarios:run`, `scenarios:write`, `dlqs:write`, `hooks:write`.
- API is JSON-clean, well-paginated (`pg.limit`, `pg.offset`, `pg.sortBy`), no Cloudflare/anti-bot signal.

## Top Workflows
1. **Trigger a scenario from an agent and wait for the result.** `run --wait` is the headline. Today this requires polling logs manually. With `--wait`, return the execution outcome inline so an agent can act on it.
2. **Promote a scenario from dev workspace → prod workspace.** Today: manual blueprint export, manual import, manually re-pin connections/webhooks/data-store IDs by clicking. Pain: ID remapping is lossy and undocumented.
3. **Daily blueprint backup to git.** Community has built ad-hoc daily-export Make scenarios that push to GitHub. A CLI `blueprint export --all` + git workflow replaces that.
4. **Inbox triage of incomplete executions (DLQs).** `dlq list --age 24h` then `dlq retry`/`resolve` across many scenarios. Today: per-scenario UI clicks.
5. **Connection health audit.** Find expired/expiring tokens, unused connections, scope drift. Today: visually scroll the Connections tab.
6. **WIP scenario detection.** Find scenarios that haven't run in N days, or that are active but failing > X% — invisible without scripting against the API.

## Table Stakes (must match the make-cli & @makehq/sdk surface)

From official `make-cli` (TypeScript) categories:
- Scenarios: list/get/create/update/delete/clone, activate/deactivate, run, executions, incomplete executions, folders, functions, webhooks, devices, data structures
- Credentials: connections list/get/create/delete/verify, keys, credential requests
- Data Stores: stores CRUD + records CRUD
- Account: teams, organizations, users
- Custom App Dev: sdk apps/modules/connections/functions/rpcs/webhooks
- Utilities: enumerations, auth

From `@makehq/sdk` typed surface (additionally exposes):
- Blueprints, Public Templates, Folders, Functions

We absorb ALL of the above plus the user's explicit ask: `scenarios run --wait`, `blueprint export/import` for version control.

## Data Layer (SQLite mirror)
- **Primary entities:** scenarios, blueprints (snapshots), executions, dlqs, connections, hooks, data_stores, data_store_records, data_structures, folders, templates, devices, keys, teams, organizations, users.
- **Sync cursor:** `lastEdit` for scenarios; execution `id` (monotonic) for logs; `created` for executions and dlqs.
- **FTS/search:** scenarios (name + description + used packages + module list), blueprints (extracted module names, mapper string values, connection names), executions (status + scenario name), dlqs (error reason).
- **Special:** blueprint snapshots are stored versioned in SQLite to enable local diff and time-travel restore.

## Codebase Intelligence
- **Source:** WebSearch + live API probe + `make-cli` and `@makehq/sdk` READMEs.
- **Auth:** `Authorization: Token <token>`, zone-scoped base URL. Token scopes are fine-grained (e.g., `scenarios:run` vs `scenarios:write`).
- **Data model:** Org → Team → (Scenarios | Connections | Hooks | DataStores | DataStructures | Folders | Keys | Devices | Templates). Most resources require `teamId` query param (some accept `organizationId` instead).
- **Pagination:** `pg[limit]`, `pg[offset]`, `pg[sortBy]`, `pg[sortDir]`.
- **Rate limiting:** Make returns 429 with `Retry-After`; per-zone, account-tier dependent.
- **Architecture insight:** Make's blueprint JSON is a `{flow: [{id, module, mapper, parameters, metadata}]}` tree. Module-level metadata.expect/restore contains UI defaults that ARE round-tripped on import — this is why naïve blueprint exports re-import with wrong defaults.

## User Vision
The user (<the user> / Wade) provided this brief:
- **`scenarios run --wait` is the killer use case** for agent-driven Make.com workflows. Blocking execution that returns a real result lets an agent decide what to do next.
- **Blueprint export/import enables version control** from the terminal.
- Already a heavy Make user (<a client workspace>, <the content workspace>, <client> pipelines visible in their team) — needs CLI that handles real multi-folder, multi-connection ops, not toy demos.

## Source Priority
Single primary source (Make.com Management API). No combo CLI; no inversion risk.

## Product Thesis
- **Name:** `make-pp-cli`
- **Why it should exist:**
  1. The official `make-cli` is TypeScript-only (npm install), mirrors REST 1:1, no offline store, no agent-native `--wait`/`--json`/`--select`, no bulk ops, no blueprint diff.
  2. The Cloud MCP at mcp.make.com is paid-tier locked beyond scenario-run, and ties you to Anthropic-hosted MCP. A local Go binary CLI + local MCP server beats both axes (offline + free).
  3. No Python or Go SDK exists — the install-friction wedge is wide open.
  4. The version-control + dev→prod promote workflow is the most-requested community pain point and has no first-party answer. A CLI that does git-backed blueprint sync with ID remapping is genuinely novel.

## Build Priorities
1. Data layer for all primary entities + blueprint snapshot table.
2. `scenarios run --wait` (poll executions with backoff until terminal status, return JSON result).
3. `scenarios blueprint export/import` with `--git` integration and ID remap manifest.
4. Match every command in official `make-cli` categories.
5. Inbox: `dlq list/get/retry/resolve`.
6. Audits: `connections audit` (stale/expiring), `scenarios stale --days 30`.
7. Local search + FTS over scenarios and blueprints.
