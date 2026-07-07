# Pangolin CLI Brief

## API Identity
- **Domain:** Self-hosted, WireGuard-based tunneled reverse proxy. The open-source alternative to Cloudflare Tunnels. Expose private services on public domains without opening firewall ports.
- **Users:** Self-hosters, homelab operators, small-team SREs, MSPs running multi-tenant edge proxies. People who'd reach for Cloudflare Tunnel but want self-hosted control.
- **Data profile:** Org-scoped, hierarchical. Orgs contain Sites (WireGuard endpoints), Sites contain Resources (HTTP services), Resources have Targets (upstream backends), Roles/Users/IdPs govern access. Certificates issued per domain. Clients (Newt) connect outbound to Sites.

## Reachability Risk
**None.** Self-hosted, user-controlled. User's instance live at `https://<self-hosted-host>/api/v1` with clean 401 on `/orgs` (auth works). Spec source-of-truth lives in their own deployment (`GET /v1/openapi.json` available).

## Local-deployment note (operationally important)
The OpenAPI spec declares `servers: [{url: /v1}]` (relative). In real Pangolin Enterprise Edition deployments the integration API is mounted at **`/api/v1`** behind the dashboard host. The generated CLI must let users override the base URL via env var (`PANGOLIN_BASE_URL`) and config — do NOT hard-code `/v1` or assume same-host with the dashboard.

## Top Workflows
1. **Site + Resource provisioning** — "Expose service X running on home box at port Y on subdomain Z" — create site (if needed), create resource, attach target, set roles, verify cert.
2. **Bulk health / drift audit** — "Across all orgs/sites, which resources are unreachable, missing certs, or have stale targets?" — no native UI for this; users tab through dashboard.
3. **User & role management** — Onboard/offboard users, audit who has access to what resource across orgs, sync role assignments.
4. **IdP & SSO management** — Configure OIDC providers, see which orgs use which IdP, audit external auth coverage.
5. **Backup / disaster recovery** — Export the full Pangolin config (orgs, sites, resources, targets, roles, idps) as version-controllable JSON; re-apply against a fresh install.

## Table Stakes
- CRUD for all 14 resource groups (org, site, resource, target, site-resource, client, idp, user, role, certificate, domain, access-token, …)
- Auth via Bearer token with env-var fallback (`PANGOLIN_TOKEN`)
- Base-URL override (`PANGOLIN_BASE_URL`) — REQUIRED, see deployment note above
- `--json` / `--select` / `--csv` agent-native output
- `--dry-run` on mutations
- `doctor` health check (token valid? base URL reachable? required endpoints respond?)
- Realistic help examples (`pangolin org get $ORG_ID --json`)

## Data Layer (SQLite)
- **Primary entities:** orgs, sites, resources, site_resources, targets, clients, users, roles, idps, certificates, domains, access_tokens
- **Sync cursor:** per-resource-type last-fetched timestamp (Pangolin has no event stream we can see in the spec; full re-sync per type)
- **FTS/search:** resources_fts on (name, niceId, ssl, http, full domain, target hostnames); orgs_fts on (name, orgId); users_fts on (name, email, username)
- **Join opportunities:** "which resources point at unreachable targets," "which users have access via N different role assignments," "cert expiry across all orgs"

## Codebase Intelligence
- **Source:** spec inspection (fosrl/pangolin EE)
- **Auth:** `Authorization: Bearer <integration-token>` (single security scheme `Bearer Auth`, HTTP scheme `bearer`)
- **Data model:** strict org-scoped hierarchy: `org → site → site-resource | resource`; resource has targets + roles + users + clients; idps are org-level
- **Rate limiting:** not specified in spec; per-deployment (likely Traefik in front)
- **Architecture:** Go server + Next.js dashboard + Gerbil (WireGuard gateway) + Newt (client). Integration API is the same API the dashboard consumes — high-quality, well-shaped

## Ecosystem (existing tooling)
- **Official CLI:** none
- **Official MCP:** none
- **Community CLIs:** none meaningful — fosrl/pangolin is young (~2024) and the integration API was published recently
- **Wrappers / SDKs:** none on npm/PyPI as of cutoff
- **Adjacent tools:** Newt (connect-only client), Gerbil (gateway, server-side), official dashboard

**Implication:** This is greenfield. The absorb manifest's "competing features" set is small. The win comes from (a) being the first decent CLI/MCP for Pangolin, (b) doing the local-store + cross-org analytics nobody else can do, (c) agent-native shapes that let Claude actually drive Pangolin.

## Product Thesis
- **Name:** `pangolin-pp-cli` (binary), Pangolin CLI (prose)
- **Why it should exist:** Pangolin is a thoughtful self-hosted reverse-proxy alternative to Cloudflare Tunnels, but its only interface is the web dashboard. Operators with 10+ resources spread across orgs/sites can't audit, bulk-edit, or back up their configs without clicking through screens. Agents (Claude, scripts) have no way to provision resources programmatically. A first-class CLI with offline SQLite + agent-native output + cross-org analytics turns Pangolin into something an SRE or homelab operator can actually automate against.

## Build Priorities
1. **Foundation:** All 14 entity types in SQLite store, `sync` command, FTS, base-URL override, Bearer auth from env
2. **Absorb:** All 157 spec endpoints exposed as typed commands (`pangolin <resource> <verb>` shape)
3. **Transcend:**
   - `audit` — cross-org health: stale targets, expiring certs, orphaned resources, missing roles
   - `backup` / `restore` — full config export/import as JSON, version-controllable
   - `access-graph` — "who can reach what" — joins users × roles × resources × orgs into one queryable view
   - `cert-watch` — expiring-soon certificates across all orgs, with days-until-expiry
   - `expose` — single-command workflow: create site (if needed) + resource + target + role binding, with `--dry-run` to preview
4. **Polish:** `doctor` with base-URL discovery hints, realistic examples drawn from common homelab patterns
