# WP Engine Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get/create/rename/delete sites | thesandybridge/wpengine-cli | (generated endpoint) site list/get/create/patch/delete | Offline mirror, --json/--select, typed exits |
| 2 | List/get/create/update/delete installs | thesandybridge/wpengine-cli (stubbed) | (generated endpoint) install CRUD | Actually works; theirs is "under development" |
| 3 | Copy install (files+DB env-to-env) | timstl/WP-Engine-Toolkit | (generated endpoint) install install-copy | --dry-run preview, scriptable |
| 4 | Cache purge (object/page/cdn/all) | ryanshoover/wpe-cli `wp wpe flush` | (generated endpoint) install purge-cache | Official API not portal cookies; enum-typed |
| 5 | Trigger backup checkpoint | ryanshoover/wpe-cli `wp wpe backup`, jpollock SDKs | (generated endpoint) backup create | Notification emails param, status polling |
| 6 | Backup status poll | jpollock/wp-engine-api-python | (generated endpoint) backup get | --json for CI gating |
| 7 | Restore from backup | spec-native (no tool has it) | (generated endpoint) backup restore | --dry-run + confirm gating |
| 8 | Archives (downloadable backup exports) | spec-native; replaces ryanshoover fetch-db portal hack | (generated endpoint) archive list/create | Official surface, no session cookies |
| 9 | Domain CRUD + bulk add + redirects | spec-native | (generated endpoint) domain ops (10 endpoints) | Bulk add, agent-scriptable |
| 10 | Domain TXT verification + Entri sharing links | spec-native | (generated endpoint) domain verification/sharing-link | Agent-scriptable DNS onboarding |
| 11 | SSL: list certs, per-domain info, Let's Encrypt request, 3rd-party import | spec-native | (generated endpoint) certificate ops | Fleet cert visibility |
| 12 | Account + account-user CRUD | timstl/WP-Engine-Toolkit (read-only) | (generated endpoint) account/account-user ops | Full CRUD not just read |
| 13 | Usage metrics: install daily + account summary/insights/limits | spec-native | (generated endpoint) usage ops | Local history retention beyond API window |
| 14 | Site transfers (list/initiate/cancel) | spec-native | (generated endpoint) site-transfer ops | — |
| 15 | Offload settings / LargeFS | spec-native | (generated endpoint) offload-settings ops | — |
| 16 | SSH key list/add/delete | spec-native | (generated endpoint) ssh-key ops | — |
| 17 | Site reports + schedules + templates | spec-native | (generated endpoint) site-reports ops | — |
| 18 | Headless/pipeline mode | thesandybridge `-H` | (behavior in wpengine-pp-cli: global --json/--agent/--select/--csv flags) | Richer than a single flag |
| 19 | Rate-limit retry with backoff | jpollock SDKs | (behavior in wpengine-pp-cli: client automatic 429 retry) | Undocumented server limits handled |
| 20 | Typed errors/exit codes | jpollock SDKs | (behavior in wpengine-pp-cli: typed exit codes 0/2/3/4/5/7/10) | CI-composable |
| 21 | MCP site management tools | zd87pl/wpengine-mcp-server (repo now 404) | (behavior in wpengine-pp-cli mcp: Cobra-tree mirror) | Maintained, complete, typed, read-only hints |
| 22 | Offline fleet mirror + FTS search + SQL | none (nobody has this) | wpengine-pp-cli sync | Core differentiator: accounts/sites/installs/domains/backups/certs in local SQLite |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|---------|--------------|------------------------|------------------|
| 1 | Cert expiry audit | `audit certs --expiring 30d` | 10/10 | Marisol (ops lead) | hand-code | Joins synced installs↔domains↔certificates in local SQLite, computes days-to-expiry, sorts ascending. Portal answers one install at a time. | Use this command for fleet-wide certificate expiry windows. Do NOT use it to find domains with no certificate at all or unverified domains; use 'audit domains' instead. |
| 2 | Backup staleness audit | `audit backups --stale 7d --env production` | 10/10 | Marisol (ops lead) | hand-code | Joins installs↔backups locally, keeps latest completed backup per install, filters by age and environment. No API call answers this fleet-wide. | Use this command for fleet-wide backup staleness across installs. Do NOT use it to list backups for a single install; use 'backup list' instead. |
| 3 | Version drift audit | `audit versions --php-below 8.2 [--drift]` | 10/10 | Marisol (ops lead) | hand-code | Aggregates php_version/wp_version over synced installs with threshold filters; --drift flags sites whose environments disagree. | none |
| 4 | Domain health audit | `audit domains` | 8/10 | Priya (launch tech) | hand-code | Local join domains↔certificates↔redirects: unverified domains, zero cert coverage, dangling redirects — fleet-wide launch checklist. | Use this command for fleet-wide domain health: unverified domains, missing cert coverage, dangling redirects. Do NOT use it to look up which install serves one domain; use 'whois' instead. Do NOT use it for cert expiry windows; use 'audit certs' instead. |
| 5 | Deploy guard | `guard <install> [--purge cdn\|page\|object\|all]` | 9/10 | Diego (deploy engineer) | hand-code | Compound lifecycle: create checkpoint backup → poll requested→in-progress→completed → optional purge, typed exits for CI gating. | Use this command to gate a deploy: it creates a checkpoint backup, waits for completion, and optionally purges cache, with CI-friendly exit codes. Do NOT use it to trigger a backup without waiting; use 'backup create' instead. Do NOT use it to purge cache alone; use 'install purge-cache' instead. |
| 6 | Reverse domain lookup | `whois <domain>` | 8/10 | Marisol (ops lead) | hand-code | Single local join from domain name to install, site, account, environment, cert status, redirect target — the support-ticket triage query. | Use this command to resolve one domain name to the install, site, and account that serve it, plus its cert and redirect status. Do NOT use it for fleet-wide domain health scans; use 'audit domains' instead. |
| 7 | Overage projection | `audit usage --horizon 30d` | 9/10 | Tomás (account director) | hand-code | Linear month-end extrapolation from live month-to-date usage summaries vs live account limits — a cross-account overage rollup no single endpoint provides. | none |

No stubs. All 7 transcendence rows are shipping scope, all hand-code (~80-150 LoC each plus root.go wiring).

Killed candidates and customer model: see 2026-07-20-233949-novel-features-brainstorm.md (audit trail).
