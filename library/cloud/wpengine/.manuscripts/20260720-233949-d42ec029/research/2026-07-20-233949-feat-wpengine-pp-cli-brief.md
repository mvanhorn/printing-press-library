# WP Engine CLI Brief

## API Identity
- Domain: Managed WordPress hosting platform management (api.wpengineapi.com/v1, Swagger 2.0 v1.21.2, self-served at /v1/swagger)
- Users: agencies and dev teams managing fleets of WordPress installs on WP Engine — exactly the multi-client agency profile
- Data profile: hierarchical fleet inventory — Accounts → Sites → Installs (prod/staging/dev environments) → Domains/SSL/Backups/Usage. 68 operations, 16 resource families.
- Auth: HTTP Basic (API User ID + Password from my.wpengine.com/api_access). Community env-var convention: `WP_ENGINE_API_USERNAME` / `WP_ENGINE_API_PASSWORD` (jpollock SDKs, generated from this same swagger).

## Reachability Risk
- **None** — `GET /v1/status` returns 200 unauthenticated (verified 2026-07-20: `{"success":true,...}`). Spec itself is served unauthenticated. `/v1/` without auth returns a clean JSON 403 `Missing Authentication Token` (proper API error, not a WAF page).
- Rate limits are undocumented (WP Engine deliberately does not publish request/sec limits); community SDK ships retry/backoff — our client should too.
- Probe-safe endpoint used: `GET /status` (unauthenticated, official Status tag).

## Top Workflows
1. **Fleet inventory & audit** — list all accounts/sites/installs/domains at once; find installs by PHP/WP version, environment, multisite flag. The portal makes this painfully click-heavy across accounts.
2. **Backup ops** — trigger checkpoint backups before deploys, poll status, create downloadable archives from backups, restore.
3. **Cache purge** — purge object/page/cdn/all for an install (the single most-run agency command).
4. **Domain + SSL lifecycle** — add domains (single/bulk), TXT verification, Entri sharing links, Let's Encrypt requests, third-party cert import, cert expiry monitoring.
5. **Usage & billing visibility** — daily usage metrics per install (visits, storage, CDN bytes), account rollups, limits vs. actuals.

## Table Stakes
- Sites/installs/accounts CRUD (thesandybridge/wpengine-cli — Rust, pre-1.0, mostly stubs)
- Backup trigger (ryanshoover/wpe-cli `wp wpe backup`, jpollock SDKs)
- Cache flush (ryanshoover/wpe-cli `wp wpe flush`)
- Site/backup ops with retry & typed errors (jpollock/wp-engine-api-python: rate-limit retry, typed exceptions)
- Domain management (spec-native; no community tool covers it)
- MCP site management (zd87pl/wpengine-mcp-server — repo now 404/private; listing referenced WPENGINE_API_TOKEN)

## Data Layer
- Primary entities: accounts, sites, installs, domains, backups, ssh_keys, account_users, certificates
- Sync cursor: none in API (offset/limit pagination; `count` + `next`/`previous` envelope) — full-refresh sync per resource
- FTS/search: install name, site name, domain name, primary_domain, tags, group_name
- High-gravity joins only possible locally: install↔domains↔certs (expiry audit), install↔backups (staleness audit), install↔usage (growth trends), account↔installs (fleet rollup)

## Codebase Intelligence
- Source: jpollock/wp-engine-api-python + wp-engine-api-php READMEs (both generated from this same swagger via OpenAPI Generator; community, ~2024-12, minimal but confirm auth + retry conventions)
- Auth: HTTP Basic; env vars `WP_ENGINE_API_USERNAME` / `WP_ENGINE_API_PASSWORD`; `.env` supported
- Rate limiting: undocumented server-side; SDK convention is automatic retry with configurable max_retries/retry_delay; 429 defined in spec (`TooManyRequestsOperation`)
- Architecture: clean REST, UUID ids, paginated list envelopes (`previous`/`next`/`count`/`results`), enum-typed cache purge (`object|page|cdn|all`), backup lifecycle `requested→in-progress→completed`
- Portal-internal endpoints (my.wpengine.com session+CSRF, used by ryanshoover/wpe-cli for fetch-db) are a separate private surface — out of scope; official API's backups+archives covers the use case.

## Source Priority
- Single source: official WP Engine Hosting Platform API. Not a combo CLI.

## Product Thesis
- Name: wpengine-pp-cli
- Why it should exist: no maintained, complete CLI exists for this API. The best community attempt (Rust) is pre-1.0 with most resources stubbed "under development"; the rest are WP-CLI extensions built on private portal cookies or thin SDKs. Nothing offers the fleet-wide offline view: every account, site, install, domain, cert, and backup in one local SQLite you can query, join, and audit in milliseconds. For an agency: "which client installs are on PHP < 8.2", "which certs expire this month", "which prod installs have no backup in 7 days" — one command each, offline.

## Build Priorities
1. Full absorb of the 68-op surface with agent-native output (--json/--select/--dry-run, typed exits)
2. SQLite fleet mirror + sync + FTS across accounts/sites/installs/domains/backups/certs
3. Transcendence: fleet audit commands (cert expiry, backup staleness, version drift, usage trends, orphan domains)
4. Safety: destructive ops (delete install/site/domain, restore backup) gated behind --confirm + --dry-run
