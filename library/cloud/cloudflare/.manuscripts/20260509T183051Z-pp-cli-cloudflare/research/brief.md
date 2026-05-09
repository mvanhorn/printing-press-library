# Cloudflare CLI Brief

## API Identity
- **Domain:** Cloudflare global edge platform — DNS, CDN, WAF, Workers (compute), R2 (object storage), KV (key-value), D1 (SQLite), Pages (static hosting), Stream (video), Images, Spectrum, Access/Zero Trust, Magic Transit, Tunnel, Logs, Analytics, Notifications.
- **Users:** Web operators, platform engineers, SREs managing 1–N zones; Worker developers; Zero Trust admins; agents acting on infrastructure.
- **Data profile:** Account-scoped + zone-scoped resources, ~3000 API endpoints, deeply hierarchical (account → zone → resource → sub-resource).

## Reachability Risk
- **None / Low.** The API itself (`api.cloudflare.com/client/v4/*`) is rock-stable; verified live with bad-token probe → returns proper 400 JSON. No 403/Cloudflare-WAF issues against their own API. Some wrangler deploy flakiness is reported but it's wrangler-specific (preview-token handling), not Cloudflare API blockage.

## Top Workflows
1. **DNS record CRUD across N zones** — bulk add/update/delete A/CNAME/TXT/MX records, including for incident response (subdomain takedown, MX reconfig, SPF/DKIM/DMARC adjustments).
2. **Cache purge** — by URL, by tag, by hostname, full purge.
3. **Worker deploy + log tail** — `wrangler deploy && wrangler tail` is the most-used dev workflow on the platform.
4. **R2 / KV bulk operations** — current CLIs don't support bulk; users write Python scripts.
5. **Zone settings audit + diff** — "what's different between staging and prod zones?" — currently dashboard-only, painful.
6. **WAF / Ruleset / Page Rule management** — bulk rule create/disable/audit. Terraform-only today.
7. **Access policy export + audit** — Zero Trust posture review. Dashboard-only.

## Table Stakes (must absorb from competitors)
- **From wrangler:** Workers deploy, tail (real-time logs), KV CRUD, D1 query/migrations, R2 bucket CRUD, Pages deploy, secrets management, DO lookup.
- **From flarectl:** Zone CRUD, DNS record CRUD, page rules, firewall, user management, Origin CA, Railgun (deprecated, skip).
- **From cloudflare-cli (npm):** DNS list/add/edit/delete, cache purge, zone activation, SSL settings.
- **From Terraform provider:** ~300 resource types — full coverage of Access, WAF, Spectrum, Notifications, Logpush, Health Checks, Load Balancing, Workers Routes, Custom Hostnames.

## Data Layer
- **Primary entities:** accounts, account_members, zones, dns_records, workers (scripts), worker_routes, kv_namespaces, kv_keys (metadata only), r2_buckets, d1_databases, pages_projects, pages_deployments, page_rules, rulesets, waf_rules, access_apps, access_policies, access_groups, tunnels, certificates, custom_hostnames, load_balancers, notifications.
- **Sync cursor:** `modified_on` (ISO timestamp present on most resources); per-resource pagination cursors.
- **FTS/search:** zones (name + status), dns_records (name + content + type), workers (name + script content metadata), page_rules (target + actions), waf_rules (description), access_apps (name + domain).

## Codebase Intelligence
- **Cloudflare's official Code Mode MCP** at `mcp.cloudflare.com` exposes the entire ~2,500-endpoint surface through 2 tools (`search()` + `execute()`) consuming ~1k tokens vs ~1M for endpoint-mirror MCPs. This is **literally the pattern the printing-press calls "the Cloudflare pattern"** — we're building a CLI for the API that named the pattern.
- **Auth:** API tokens (preferred, scoped) via `Authorization: Bearer <token>` header. Legacy: `X-Auth-Email` + `X-Auth-Key`. Env vars: `CLOUDFLARE_API_TOKEN` (canonical), `CLOUDFLARE_API_KEY` + `CLOUDFLARE_EMAIL` (legacy).
- **Rate limiting:** 1200 req / 5 min default per token, with 429 + `Retry-After` header. Some endpoints (Logs, Analytics) have separate quotas.
- **Architecture:** Account-scoped + zone-scoped REST. Most routes start `/accounts/{account_id}/...` or `/zones/{zone_id}/...`. Standard envelope: `{success: bool, errors: [], messages: [], result: {...}|[...], result_info: {...}}`.

## User Vision
A representative user runs a small fleet of self-hosted apps and is mid-sprint on a new zone's DNS configuration. Near-term need: Cloudflare DNS A-record setup for `example.com` → 203.0.113.10, page rule `legacy.example.com/*` → 301 redirect, custom domain attach + Let's Encrypt SSL. This CLI should make that workflow trivial: one command to add DNS, one to add the page rule, one to verify propagation.

## Source Priority
- Single source — no priority gate needed. Cloudflare's official OpenAPI is the spec.

## Product Thesis
- **Name:** `cloudflare-pp-cli` (binary), advertised as "the unified Cloudflare CLI agents and operators both want."
- **Why it should exist:** Today, managing Cloudflare imperatively requires juggling `wrangler` (Workers), `flarectl` (DNS/zones, partial), Terraform (everything-as-code, declarative), and `curl` (the gap). No single CLI covers the platform end-to-end with `--json`, `--select`, `--dry-run`, and structured filtering — the table-stakes for both human shell pipes and LLM agents. Cloudflare's own Code Mode MCP solved this for agents; nobody has solved it for operators on the command line. We'll absorb every feature from every existing tool, run it offline-cached for cross-product queries (drift detection, audit, diff), and emit MCP tools through the Cloudflare pattern so a single `cloudflare_search` + `cloudflare_execute` pair covers the whole surface for agents.

## Build Priorities
1. **Foundation:** Auth (token + legacy), config (account_id, zone_id resolution), client envelope handling, paginated list iterator, SQLite store with FTS5 over zones / dns_records / workers / page_rules / waf_rules / access_apps.
2. **Absorbed (from all incumbents):** Zones (CRUD + settings), DNS (CRUD + bulk + import/export), cache (purge by URL/tag/hostname/all), workers (deploy/list/tail/secrets/routes), KV (namespace CRUD + bulk get/put/delete), R2 (buckets + objects), D1 (databases + query + migrations), Pages (projects + deployments + env), page_rules (CRUD + bulk), rulesets / WAF (CRUD), Access apps + policies + groups, tunnels, custom hostnames, certificates, load balancers, notifications, account members.
3. **Transcend (only this CLI can do):** drift between zones, multi-zone audit, cross-product search (find a domain wherever it appears — DNS / Worker route / Page Rule / Access app), bulk operations across accounts, offline analytics (sync logs to local store), zone-clone, IaC export.

## Strategic Choice — adopt the Cloudflare MCP pattern
Given the >2500-endpoint surface, default to the printing-press's Cloudflare pattern at generation time:
- `mcp.transport: [stdio, http]` (remote-capable, since this CLI will run as an installed MCP server for agents)
- `mcp.orchestration: code` (thin `cloudflare_search` + `cloudflare_execute` pair)
- `mcp.endpoint_tools: hidden` (suppress raw per-endpoint tools — agents use the orchestration pair)
- `mcp.intents:` (a small set of named multi-step intents for the headline workflows: `setup_zone`, `bulk_dns_apply`, `cache_purge_release`, `deploy_worker`, `audit_zone_drift`)

This is symmetric with what Cloudflare themselves ship. It is also the design the printing-press was built around — so this CLI doubles as the canonical example of the pattern in the public library.
