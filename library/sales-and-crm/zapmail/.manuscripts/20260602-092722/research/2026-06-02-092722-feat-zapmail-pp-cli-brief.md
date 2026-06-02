# Zapmail CLI Brief

## API Identity
- **Domain:** Cold email infrastructure / deliverability. Zapmail provisions pre-warmed Google Workspace and Microsoft 365 mailboxes with automated DNS (SPF/DKIM/DMARC/MX), per-workspace domain isolation, and one-click export into sequencers (Smartlead, Instantly, Reply.io, ReachInbox).
- **Users:** Cold email agencies, SDR/outbound teams, founders running outbound at scale, and AI-SDR platform builders embedding email-infra management.
- **Data profile:** Workspaces -> Domains -> Mailboxes is the spine. Plus subscriptions, wallet/billing, exports + third-party accounts, DNS records, placement tests, DNS Shield, prewarmed/high-reputation domains, domain tags, and Zapbox (read/send email from provisioned inboxes). Responses use a `{status, message, data}` envelope; lists paginate with `page`/`limit` and return `totalPages`/`nextPage`.

## Base / Auth (ground truth from docs)
- **Base URL:** `https://api.zapmail.ai/api`
- **Auth:** API key in header `x-auth-zapmail: {token}` (raw token, not Bearer). Get it at Dashboard -> Settings -> Integrations -> API.
- **Optional scoping headers:** `x-workspace-key` (operate in a non-primary workspace), `x-service-provider` (`GOOGLE` | `MICROSOFT`).
- **Env var:** `ZAPMAIL_API_KEY` (matches the community MCP convention).
- **Versioning:** paths are under `/v2/...` (e.g. `/v2/mailboxes/list`, `/v2/domains`, `/v2/users`).

## Reachability Risk
- **Low.** Standard REST + API-key header auth, official docs, documented rate limits. No reverse-engineering, no bot protection, no login-session discovery needed. Reachability gate (Phase 1.9) will confirm with one live call once the key is provided.

## Rate Limits (documented)
- General: 5/sec, 20/min. Domain Search: 10 per 30 min. Re-export mailbox: 3 per mailbox per week. Over-limit -> `429`.

## Top Workflows
1. **Provision & manage the mailbox fleet** - list mailboxes, get by ID, assign new mailboxes to domains, update, schedule creation, retry failed creations, remove on renewal.
2. **Domain lifecycle** - retrieve/filter domains, connect domain, get/verify name servers, DNS records CRUD, DMARC, forwarding, catch-all, health score, tags, renewal.
3. **Export to sequencers** - export mailboxes to third-party accounts (Smartlead/Instantly/Reply.io), manage third-party accounts, track export status.
4. **Buy capacity** - subscriptions, wallet balance + auto-recharge, purchase add-on mailboxes, available/prewarmed/high-reputation domains, invoices.
5. **Deliverability testing** - placement tests (orders, reports, eligible mailboxes, credits), DNS Shield allocation.
6. **Zapbox inbox ops** - list connected accounts, fetch/search emails, read threads, send email, labels.

## Table Stakes (must match incumbents)
- Everything the community MCP `dsouzaalan/zapmail-mcp` (46+ tools) does: list workspaces/domains/mailboxes, wallet balance, check domain availability (single + batch), purchase domains, create mailboxes for empty domains, add third-party export account, AI domain/username/name generation, bulk mailbox update, mailbox search, export guidance, health check, "call any endpoint".
- The official dashboard surface: ~95 documented endpoints across the groups above.

## Data Layer (local SQLite)
- **Primary entities:** workspaces, domains, mailboxes, subscriptions, third_party_accounts, exports, dns_records, placement_tests, domain_tags.
- **Sync cursor:** page-based pagination per resource; store `updatedAt` where present.
- **FTS/search:** mailbox email + domain name + tag; cross-workspace fleet search.

## Codebase Intelligence (community MCP)
- Source: `github.com/dsouzaalan/zapmail-mcp` (46+ tools). Confirms `ZAPMAIL_API_KEY`, workspace/provider context switching, `call_endpoint` generic passthrough, and AI helpers (domain finder, username/name-pair generators, batch availability).
- Also: `growthenginenowoslawski/coldoutboundskills` has a `zapmail-domain-setup` skill (Dynadot domain buy + Zapmail inbox provisioning) - confirms the buy-domain -> connect -> provision -> export flow as the canonical agency motion.

## Product Thesis
- **Name:** `zapmail` (binary `zapmail-pp-cli`).
- **Why it should exist:** The dashboard and the community MCP let you *do* actions one at a time. Nobody gives you an **offline, queryable mirror of your entire mailbox + domain fleet across all workspaces**. With everything in local SQLite you can answer questions the dashboard can't: which domains are unhealthy, which mailboxes failed creation, which are warmed but unassigned, what's my real cost-per-active-mailbox, which exports stalled, what renews next week and what it'll cost. Plus agent-native output (`--json`, `--select`, typed exit codes, `--dry-run` on every money-spending command).

## Build Priorities
1. **P0 foundation:** SQLite data layer for workspaces, domains, mailboxes, subscriptions, third_party_accounts, exports, dns_records, placement_tests, domain_tags; `sync`, FTS `search`, `sql` passthrough.
2. **P1 absorb:** the full documented endpoint surface (read + management), grouped by resource; `--dry-run` mandatory on every purchase/payment/renew/cancel command (real money).
3. **P2 transcend:** fleet-health rollup, failed-mailbox triage, warmed-but-unassigned finder, renewal cost forecast, export-status watch, cross-workspace fleet query, cost-per-active-mailbox - all local-join commands no other tool offers.

## Safety Notes
- Several endpoints spend real money (purchase subscriptions/domains/add-ons/placement-tests, add wallet balance, renew domains, DNS Shield purchase). These are **write/mutating + costly**: build with mandatory `--dry-run` default-print pattern, and exclude from live smoke testing in Phase 5.
