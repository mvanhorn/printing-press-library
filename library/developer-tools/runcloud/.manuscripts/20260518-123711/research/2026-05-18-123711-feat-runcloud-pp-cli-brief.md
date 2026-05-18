# RunCloud CLI Brief (v3 regen)

## API Identity
- **Domain:** server management / WordPress hosting orchestration. RunCloud manages a fleet of cloud servers (DigitalOcean, AWS, Vultr, Linode, GCP, etc.) for WordPress and other PHP apps — installs nginx + PHP-FPM + MariaDB, provisions webapps, manages domains/SSL, system users, cron, supervisor, firewall, fail2ban.
- **Users:** WordPress agency operators, freelance devs running 5–50 client sites, devops engineers maintaining production PHP fleets.
- **Data profile:** A few servers per workspace × many webapps × many domains per webapp × per-server databases, system users, ssh keys, cron jobs, supervisor jobs. Fleet-wide queries ("show me every SSL cert expiring in 30 days across all 12 servers") cannot be answered without local aggregation.

## Version Change: v2 → v3

**This is a regen of the prior `runcloud-pp-cli` that targeted API v2.** v2 build archived at `~/printing-press/library/runcloud-v2-archive/`.

**v3 deltas:**
- **Auth model:** v2 used HTTP Basic with `key:secret` pair (`Authorization: Basic <base64>`). v3 uses a **single bearer token** (`Authorization: Bearer <token>`) issued from Workspace > Settings > API Management.
- **Base URL:** `/api/v2` → `/api/v3`.
- **Two API surfaces:** v3 splits into `agency-api` (under `/agency`) and `core-api`. v2 was a single flat surface.
- **New health endpoint:** `/servers/{id}/health/latest` (v3 timeseries-friendly health snapshot) and `/servers/{id}/health/diskcleaner` (server disk cleanup action).
- **New resources documented but not yet in PHP SDK:** WAF (`/servers/{id}/webapps/{id}/waf`), RunCloudHub (`/servers/{id}/webapps/{id}/runcloudhub` — WordPress object-cache plugin).
- **Pagination unchanged:** `?page=N&perPage=N` (max 40 per page).
- **Rate limits:** `X-RateLimit-Limit`, `X-RateLimit-Remaining` headers, `429` when exceeded.
- **Required headers:** `Content-Type: application/json` and `Accept: application/json` on every request.

## Reachability Risk
- **None.** Direct probe to `https://manage.runcloud.io/api/v3/ping` returned `HTTP 401` unauthenticated (expected) — API is live and answering. No bot-protection signals (no Cloudflare/Vercel challenge, no WAF/DataDome HTML). Bearer auth in header will unlock real responses.

## Top Workflows (carried from v2, validated against v3 surface)
1. **Provision a new webapp end-to-end** — server + webapp + domain + SSL + git + (optional) WordPress installer. A 15-click dashboard sequence today.
2. **Fleet-wide SSL audit** — every webapp/domain across every server with expiring or missing certificates.
3. **Server health snapshot** — uptime, load, disk, memory, services for the whole fleet in one command (v3's `/health/latest` makes this cleaner than v2's `/stats`).
4. **Reverse-lookup a domain** — "which server + webapp + system user + SSL cert hosts `foo.com`?" — impossible via the API (per-server endpoints only).
5. **Friday security sweep** — every fail2ban-blocked IP across the fleet, dedupe, identify pattern attackers.

## Table Stakes (the absorb set)
The PHP SDK's `src/config.php` (which we mined into `runcloud-sdk-config.php` in this run's research dir) is the authoritative v3 endpoint map. Coverage:

**core-api** (~85 endpoints):
- Static data: collations, timezones, installers, SSL protocols
- 3rd-party API keys: CRUD
- Servers: CRUD + shared servers + stats + hardware info + install script + SSH config + PHP CLI version + metadata + autoupdate
- **Health** (new in v3): `/health/latest` snapshot + `/health/diskcleaner` action
- Webapps: CRUD + alias + rebuild + default + settings (php / fpmnginx) + activity log
- Git per-webapp: clone + details + branch + script + force-deploy + delete
- Installer per-webapp: install + get + remove
- Domains per-webapp: CRUD
- SSL basic per-webapp: install + get + update + redeploy + delete
- SSL advanced per-webapp: status + switch + per-domain CRUD
- Databases: CRUD
- Database users: CRUD + password
- Grants (db × dbuser): attach + list + revoke
- System users: CRUD + password + deployment key
- SSH keys per server: CRUD
- Cron jobs: CRUD + rebuild
- Supervisor: CRUD + binaries + status + reload + rebuild
- Services: list + trigger
- Firewall rules: CRUD + deploy
- Fail2Ban blocked IPs: list + unblock
- Server logs

**agency-api** (~46 endpoints, new top-level command group for v3):
- Agency account (1)
- Clients: CRUD + password reset + magic link + tags (8)
- Server packages: CRUD + upgrades + duplicate + client-servers (9)
- Teams: CRUD + member invitations + members CRUD + transfer + server-package assignment + server assignment (15)
- Client servers: CRUD + suspend/unsuspend + rebuild + change-client + add-ip + assign-server + upgrade-package + resend-webhook (13)

**Documented but not in SDK** (v3-newer, scope decision):
- WAF (`/webapps/{id}/waf`)
- RunCloudHub object-cache plugin (`/webapps/{id}/runcloudhub`)

## Data Layer
- **Primary entities:** `servers`, `webapps`, `domains`, `databases`, `database_users`, `db_grants`, `system_users`, `ssh_keys`, `cron_jobs`, `supervisor_jobs`, `services`, `firewall_rules`, `blocked_ips`, `ssl_certs`, `installed_scripts`, plus agency-side `clients`, `teams`, `team_members`, `server_packages`, `client_servers`.
- **Sync cursor:** per-resource `updated_at` from response payload where available; otherwise `last_synced_at` per (server_id, resource_type).
- **FTS/search:** cross-resource FTS5 over `webapps.name`, `domains.name`, `servers.ipAddress`, `servers.serverName`, `databases.databaseName`, `system_users.username`, `clients.name`, `clients.email`. Powers reverse-lookup workflows.
- **Cardinality estimate:** typical agency workspace = 5–50 servers × 5–20 webapps each × 1–5 domains each = a few thousand domains plus a few hundred databases, system users, cron jobs. SQLite handles this trivially.

## Codebase Intelligence
- **Source: PHP SDK v3** (RunCloudIO/runcloud-sdk-php @ master, `src/config.php` parsed inline).
- **Auth:** Bearer token in `Authorization` header. Single token, no key/secret pair. Token type literal: `RUNCLOUD_API_TOKEN` (proposed canonical env var; v2 used `RUNCLOUD_API_KEY` + `RUNCLOUD_API_SECRET`).
- **Data model:** Server is the root entity; nearly every core-api resource is server-scoped (`/servers/{id}/...`). The agency-api inverts: clients/teams/packages are workspace-level.
- **Rate limiting:** advertised in `X-RateLimit-Limit`/`X-RateLimit-Remaining` headers, 429 on exceed. No per-request retry guidance in docs — implement standard exponential backoff with header-driven hint.
- **Architecture:** REST + JSON. Two surfaces share the same bearer token and base host; differ only in URL prefix (`/api/v3/agency/...` for agency-api, `/api/v3/...` for core-api). Confirmed by SDK's `'base' => '/agency'` config.

## User Vision
*User invocation:* "Yes lets archive the old one in case you want to reference it, potentially for its novel commands, but the new one will be the same name 'runcloud'" — and "probably should do full re-research against v3."

Implication: preserve the spirit of the v2 build (fleet-wide audits, reverse-lookup, WordPress provision) and rebuild on v3 auth/endpoints. Mine the v2 archive's transcendence features when they remain useful.

## Product Thesis
- **Name:** `runcloud-pp-cli` (binary), `runcloud` (slug)
- **Why it should exist:** RunCloud's official dashboard is per-server. There is no `runcloud servers list | jq` equivalent. The MCP servers found in the wild expose endpoint mirrors but no SQLite layer, no offline fleet queries, no compound WordPress provision command. This CLI is the only way to ask "across my whole fleet, where does foo.com live and is its SSL about to expire?" and get an answer in one command.

## Build Priorities
1. **Foundation** (generator-emitted): every core-api resource as a generated command, with SQLite store + FTS + sync + search + sql + agent-native flags (`--json --select --csv --compact --quiet --dry-run`).
2. **Agency-api surface**: separate top-level command group (`runcloud agency clients ...`, `runcloud agency teams ...`, etc.) so workspace operators get the full v3 capability.
3. **Transcendence** (hand-built in Phase 3):
   - `fleet ssl-audit [--expiring-within 30d] [--missing]`
   - `fleet php-audit [--below 8.2]`
   - `whois-fleet <domain-or-pattern>` (reverse lookup)
   - `fleet blocked-ips [--since 7d] [--ip <ip>]`
   - `fleet services [--not-running] [--name nginx]`
   - `fleet health`
   - `fleet installers [--type wordpress]`
   - `fleet orphan-dbs`
   - `fleet no-git`
   - `fleet ssh-keys [--fingerprint <fp>]`
   - `webapps provision --server <id> --name <n> --domain <d> [--git <url>] [--ssl]` (chained API)
   - `webapps wordpress --server <id> --name <n> --domain <d> ...` (full WordPress install chain)
4. **v3-new surface** (decide at Phase 1.5 gate whether to include):
   - `webapps health` using v3's `/health/latest`
   - `servers disk-cleanup` using v3's `/health/diskcleaner`
   - WAF + RunCloudHub commands (if scope permits)

## Notable competitors / alternatives
| Tool | Lang | API ver | Stars | Coverage |
|------|------|---------|-------|----------|
| [RunCloudIO/runcloud-sdk-php](https://github.com/RunCloudIO/runcloud-sdk-php) | PHP | **v3 (official)** | low | ~130 endpoints via action-map, no CLI surface |
| [aleksanderem/runcloud-mcp](https://github.com/aleksanderem/runcloud-mcp) | TypeScript | v2 | 0 | 85 MCP tools, endpoint mirrors |
| [RunCloud-cdk/shell-api-wrapper (rcdk)](https://github.com/RunCloud-cdk/shell-api-wrapper) | Shell | v2 | 0 | 11 commands |
| [develanet/runcloud (runcloudjs)](https://github.com/develanet/runcloud) | JS | unclear | low | Only `servers.list()` shipped |
| [onhovercode/runcloud-sdk](https://github.com/onhovercode/runcloud-sdk) | PHP | v2 only | 0 | superseded by official |
| **runcloud-pp-cli v2 archive** (this Press, prior run) | Go | v2 | n/a | Full v2 coverage + 12 transcendence features + offline FTS |

No competing v3 **CLI** exists today. The official PHP SDK is the only known v3 client.
