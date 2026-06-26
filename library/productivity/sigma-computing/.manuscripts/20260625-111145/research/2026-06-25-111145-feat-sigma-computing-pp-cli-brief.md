# Sigma Computing CLI Brief

## API Identity
- **Domain:** Cloud BI / analytics platform. REST API v2.0.0, OpenAPI 3.1.0, 175 paths (115 GET, 73 POST, 37 DELETE, 22 PATCH, 5 PUT). Base `https://api.sigmacomputing.com` (12 per-cloud/region servers).
- **Users:** Sigma admins (member/team provisioning, permission/grant management, governance) and developers/data engineers (workbook export, embed signing, materialization scheduling, lineage auditing, connection/data-model management).
- **Data profile:** Resource-oriented. Top resource groups by op count: **workbooks (54)**, connections (22), dataModels (16), user-attributes (16), deploymentPolicies (12), members (11), reports (11), teams (10), datasets (9, deprecated), tenants (9), workspaces (9), translations (8), saml (7). Returns JSON everywhere. Cursor pagination (`nextPage` token) being made mandatory on list endpoints.

## Reachability Risk
- **Low.** Documented, stable, official OpenAPI spec provided by the user. Auth is standard OAuth2 client-credentials. No bot-protection. Only quirk: `/v2/auth/token` is rate-limited to 1 req/sec; other endpoints' limits undocumented (implement adaptive backoff). No live credentials this run → Phase 5 live smoke skipped.

## Top Workflows
1. **Workbook lifecycle** — list/get/copy/export (CSV/PDF/XLSX)/transfer-ownership/version-history/materialization. The headline. (workbooks = 54 ops, the de-facto "complete" sample set per Sigma's own quickstarts.)
2. **Member & team provisioning** — bulk create/update/deactivate members, assign teams, set user attributes. The *only* programmatic provisioning path for non-Enterprise (SCIM is Enterprise+SAML gated).
3. **Permission & grant auditing** — who-can-see-what across workbooks, datasets, connections, workspaces (grants endpoints on every resource).
4. **Connection & data-model management** — list connections, test, sync schema, inspect data-model lineage/elements/sources.
5. **Materialization scheduling** — schedule/run workbook + data-model materializations (gotcha: a schedule must have run once before a manual run works).

## Table Stakes (absorb everything that exists)
- Sigma's JS quickstart sample set: auth, connections list+schema-sync, members CRUD+team-assign+deactivate, workbooks list/copy/export/paginate/transfer-ownership/materialize, files/inodes permissions+ownership.
- Official `sigma-agent-skills`: data-model spec CRUD (sources, columns, metrics, relationships, filters, controls, column-level security).
- Truto's 136 mapped operations = the full surface to aspire to (member/team CRUD, workspace/folder/workbook mgmt, embeds, exports, grants, source swaps, materialization, user attributes, version tags).
- `gh`-style ergonomics nobody in this ecosystem has: `auth login/status`, composable resource model, `--json | jq`, `--web` to open the UI.

## Data Layer
- **Primary entities:** workbooks, members, teams, connections, dataModels, datasets, workspaces, grants, user-attributes, tags. (The generator builds the store + sync/search/SQL from the spec's GET-list endpoints.)
- **Sync cursor:** cursor-based (`nextPage`). updatedAt timestamps on most resources for freshness.
- **FTS/search:** workbook names/paths, member emails/names, connection names — high-value offline search targets for an admin who manages thousands of workbooks.

## Codebase Intelligence
- **Auth:** OAuth2 client-credentials. `POST /v2/auth/token`, `application/x-www-form-urlencoded`, body `grant_type=client_credentials&client_id=...&client_secret=...`. Response: `access_token`, `refresh_token`, `token_type=bearer`, `expires_in=3599`. Token ~1hr; refresh before expiry. Env-var convention (community-established): `SIGMA_CLIENT_ID`, `SIGMA_CLIENT_SECRET`, `SIGMA_BASE_URL`. Credentials from Admin → Developer Access. Base URL is per-org (Admin → Developer Access → API base URL) → CLI needs `--base-url` / `SIGMA_BASE_URL`.
- **Rate limiting:** 1 rps on token endpoint; others undocumented. Implement adaptive backoff; surface typed rate-limit errors.
- **Architecture quirk:** workbook copy assigns ownership to the calling admin, not the recipient — correct with a follow-up `PATCH /v2/inodes/{id}`. A CLI `workbook copy` should bundle this.

## Product Thesis
- **Name:** `sigma-computing-pp-cli` (display: **Sigma**). Headline: the first full-featured Sigma CLI — every REST resource, plus a local SQLite mirror, offline search, and admin-governance views no existing tool has.
- **Why it should exist:** Today the only Sigma tooling is two narrow agent skills, a sample script, a React embed SDK, and a prototype MCP. There is **no Go SDK, no CLI, nothing on PyPI/npm**. Admins managing members/grants and developers managing workbooks/materializations have to hand-roll OAuth + pagination every time. A `gh`-style CLI with `--json`, a local store, and governance aggregations (who-owns-what, stale workbooks, grant audits) fills a completely empty field.

## Build Priorities
1. **Auth + client + store + sync/search/SQL** (Priority 0 — generator). OAuth2 client-credentials with token refresh; cursor pagination.
2. **Absorb all spec endpoints** (Priority 1 — generator). Workbooks, members, teams, connections, dataModels, grants, workspaces, user-attributes, etc. — full CRUD where the spec exposes it.
3. **Transcendence** (Priority 2 — hand-built). Governance/audit aggregations that require the local join: grant audits, stale-workbook detection, ownership maps, member-access reports, workbook lineage rollups.
