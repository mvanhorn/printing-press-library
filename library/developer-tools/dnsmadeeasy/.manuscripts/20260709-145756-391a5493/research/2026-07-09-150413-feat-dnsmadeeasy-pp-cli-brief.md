# DNS Made Easy CLI Brief

## API Identity
- Domain: Managed authoritative DNS hosting (DNS Made Easy, a Digicert company). Enterprise DNS with GTD (Global Traffic Director), failover, and secondary DNS.
- Base URL: `https://api.dnsmadeeasy.com/V2.0` (sandbox: `https://api.sandbox.dnsmadeeasy.com/V2.0`)
- Users: SREs, sysadmins, hosting providers, domain admins managing many zones/records programmatically; automation pipelines (Terraform, ACME/Let's Encrypt DNS-01).
- Data profile: Hierarchical — Account → Managed domains → Records; plus Secondary domains, Secondary IP sets, Templates (+ template records), SOA records, Vanity DNS, Transfer ACLs, Folders, Contact lists, Failover/Monitor configs. Records carry rich fields (type, value, ttl, gtdLocation, mxLevel, priority, weight, port, dynamicDns, caaType, redirect fields).

## Auth (CRITICAL — non-standard)
- Scheme: **HMAC-SHA1 request signing**, applied to EVERY request.
- Headers per request:
  - `x-dnsme-apiKey: <API key>`
  - `x-dnsme-requestDate: <RFC1123 timestamp in GMT>` (e.g. `Mon, 09 Jul 2026 14:00:00 GMT`)
  - `x-dnsme-hmac: <hex HMAC-SHA1(secretKey, requestDate)>`
- Credentials: TWO values — API Key + Secret Key. Env var convention (lego/community): `DNSMADEEASY_API_KEY`, `DNSMADEEASY_API_SECRET`. Get from control panel: Config → API Keys (cp.dnsmadeeasy.com/account/info).
- Generator has NO HMAC/signing auth mode (only api_key|bearer_token|oauth2|oauth2_refresh|none). **Foundation task #1: hand-author a signing http.RoundTripper wired into the generated client so ALL endpoint commands sign automatically.** Model spec auth as `api_key` to get two-var credential plumbing; signing is custom.

## Reachability Risk
- None/Low. `api.dnsmadeeasy.com/V2.0/dns/managed` returns HTTP 302 unauthenticated (live host). Official SDK + Terraform provider + ACME (lego) all use this API in production. No community reports of blocking.
- Rate limit: ~150 requests / 5-minute rolling window (default). Response headers `x-dnsme-requestLimit`, `x-dnsme-requestsRemaining`; HTTP 429 on exceed. Client must be rate-limit-aware (retry/backoff).

## Top Workflows
1. List zones and dump/query all records in a zone (read + audit).
2. Create/update/delete records (typed: A, AAAA, CNAME, MX, TXT, SRV, NS, PTR, SPF, CAA, ANAME, HTTPRED).
3. Bulk record operations (createMulti / updateMulti / deleteMulti) — batch changes.
4. Zone provisioning: create domain (optionally from template), configure SOA/vanity/folder.
5. Secondary DNS + IP set management; failover/monitor configuration.
6. Automation: dynamic DNS updates, ACME DNS-01 TXT record churn.

## Table Stakes (from competitors: kigster/dnsmadeeasy `dme` CLI, jswank/dnsme, official dme-go-client, godnsmadeeasy, john-k, soniah, Terraform provider)
- Domains: list, get, get-by-name, create, create-multi, update, delete, delete-multi.
- Records: list (filter by type/name), get, create, update, delete; createMulti/updateMulti/deleteMulti; typed record creators.
- Secondary DNS: list/get/create/update/delete; Secondary IP sets: list/get/create/update/delete.
- Templates: list/get/create/update/delete + template records CRUD.
- SOA records, Vanity DNS, Transfer ACLs, Folders, Contact lists: list/get/create/update/delete.
- Failover/Monitor: get/update per record.
- Usage/reports query.
- Multi-account credentials, config file, env vars.

## Data Layer
- Primary entities to persist: domains, records (the gravity center), secondary_domains, secondary_ip_sets, templates, template_records, folders, monitors/failover.
- Sync cursor: page-based pagination (`?page=N&rows=M` → `data[]`, `totalPages`, `totalRecords`, `page`). Full-account sync = paginate domains, then records per domain (rate-limit aware).
- FTS/search: records by name/value/type across ALL domains — the killer offline capability no incumbent has.

## Why install this CLI over incumbents
- Incumbents are thin API wrappers (one HTTP call per command) or Ruby (heavy runtime). None have: a local SQLite mirror, offline cross-zone record search, drift/change detection, "where is this IP/value used across all zones", zone export to BIND, or agent-native `--json`/`--select`/typed exit codes.
- Single static Go binary, agent-first, works offline once synced.

## Product Thesis
- Name: `dnsmadeeasy-pp-cli` (binary `dnsmadeeasy-pp-cli`), display name "DNS Made Easy".
- Why: Every DNS Made Easy operation matched, plus a local record store that makes cross-zone search, drift detection, and IP/value provenance instant and offline — turning a per-call API into a queryable DNS database.

## Build Priorities
1. HMAC-SHA1 signing RoundTripper (foundation; all commands depend on it) + two-credential config + doctor auth check + rate-limit-aware client.
2. Domains + Records full CRUD (typed record helpers) + multi-record ops + local store sync.
3. Secondary DNS, IP sets, templates, folders, SOA, vanity, ACL, failover — full absorbed surface.
4. Transcendence: offline cross-zone record search, drift detection, where-used IP/value, bulk apply across zones, zone export (BIND/JSON), TTL/health audit.
