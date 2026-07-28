# Novel Features Brainstorm — wpengine (subagent audit trail)

## Customer model

**Marisol — ops lead at a 12-person agency, ~70 installs across 9 WP Engine accounts.**

- **Today (without this CLI):** She keeps one my.wpengine.com tab per account plus a spreadsheet of client installs with PHP versions and cert dates that goes stale the week she finishes it. The portal is per-account and click-heavy (the brief calls this out explicitly), so cross-account questions — "which client installs are on PHP < 8.2", "which certs expire this month", "which prod installs have no backup in 7 days" — are questions she literally cannot answer without an afternoon of clicking.
- **Weekly ritual:** Monday-morning fleet audit before the team standup: version outliers, cert expiry horizon, backup coverage, anything that will become a client emergency if ignored.
- **Frustration:** Every audit question spans all 9 accounts; the portal answers one install at a time.

**Diego — deploy engineer, ships client updates Tuesdays and Thursdays.**

- **Today (without this CLI):** For each deploy he opens the portal, clicks the backup button, refreshes until the checkpoint looks done, deploys, then clicks the purge buttons. His alternative is a WP-CLI extension built on portal session cookies (ryanshoover-style) that breaks when the portal changes.
- **Weekly ritual:** Backup → verify completed → deploy → purge cache, several times a week, across many installs.
- **Frustration:** There is no scriptable gate for "the checkpoint backup actually reached `completed`" — his CI waits on a human refreshing a page.

**Priya — launch tech, onboards 2-3 client sites a month with several launches in flight at once.**

- **Today (without this CLI):** She pastes Entri links into client emails, adds domains one by one in the portal, and re-checks each in-flight launch daily for TXT verification and Let's Encrypt issuance. Her state lives in a mental checklist across portal tabs.
- **Weekly ritual:** Chase every pending domain: which are still unverified, which verified domains still lack cert coverage, which redirects point at domains that no longer exist.
- **Frustration:** No single list of "everything still blocked across all in-flight launches" — she rediscovers the same pending domains by hand every time.

**Tomás — account director, owns overage and renewal conversations with clients.**

- **Today (without this CLI):** He opens the usage page account by account, exports numbers to a spreadsheet, and still gets month-end surprises because the portal shows actuals, not trajectory, and the API's usage window forgets history.
- **Weekly ritual:** Friday scan of billable visits and storage against plan limits, deciding which clients need a heads-up call before the invoice does it for him.
- **Frustration:** Nothing answers "who is trending over their limit this month" — he finds out when the overage has already happened.

## Candidates (pre-cut)

| # | Candidate | Command | Description | Persona | Source | Long Description (planned) | Inline verdict |
|---|-----------|---------|-------------|---------|--------|----------------------------|----------------|
| 1 | Cert expiry audit | `audit certs --expiring 30d` | Fleet-wide cert expiry horizon: local join installs↔domains↔certificates, sorted by days-to-expiry | Marisol | (a)(c) | Scope redirect vs `audit domains` (see Pass 3) | Keep — thesis names it verbatim |
| 2 | Backup staleness audit | `audit backups --stale 7d --env production` | Prod installs whose latest completed backup is older than N days, from local backups join | Marisol | (a)(c) | Redirect vs generated `backup list` | Keep — thesis names it verbatim |
| 3 | Version drift audit | `audit versions --php-below 8.2` | PHP/WP version distribution across the fleet with outlier filters | Marisol | (b)(c) | none | Keep — thesis names it verbatim |
| 4 | Domain health audit | `audit domains` | Unverified domains, domains with zero cert coverage, dangling redirects, fleet-wide | Priya | (a)(b) | Scope redirects vs `whois` and `audit certs` | Keep — workflow 4 + build priority "orphan domains" |
| 5 | Deploy guard | `guard <install> [--purge cdn]` | Create checkpoint backup, poll `requested→in-progress→completed`, optional cache purge, typed exits for CI | Diego | (a)(b)(f) | Redirects vs `backup create` and `install purge-cache` | Keep — workflow 2 verbatim; lifecycle enum from Codebase Intelligence |
| 6 | Backup wait | `backup wait <id>` | Poll one backup ID until terminal state | Diego | (f) | would redirect to `guard` | Soft kill in Pass 3 — degenerate case of #5 |
| 7 | Reverse domain lookup | `whois <domain>` | Resolve any domain to its install, site, account, environment, cert status, redirect status in one local join | Marisol | (c) | Scope redirect vs `audit domains` | Keep — the support-ticket triage query the portal can't do cross-account |
| 8 | Overage projection | `audit usage --horizon 30d` | Linear month-end projection of billable visits/storage vs account limits, from locally retained usage history | Tomás | (a)(c) | none | Keep — workflow 5 "limits vs. actuals"; mechanical math, no LLM |
| 9 | Environment triad drift | `fleet triads` | Group installs by site; flag sites missing staging or where staging/prod PHP versions disagree | Diego | (b) | none | Soft kill in Pass 3 — foldable into #3 as `--drift` |
| 10 | SSL coverage gap | `audit ssl-coverage` | Domains lacking any cert SAN match | Priya | (c) | none | Soft kill — merges into #1/#4 split |
| 11 | Orphan domains | `fleet orphan-domains` | Domains on inactive installs / duplicate domains across installs | Priya | (b) | none | Soft kill — merges into #4 |
| 12 | Install resolver | `find <name>` | Fuzzy-resolve client name to install UUID | Marisol | (a) | none | **Kill now** — framework `search "<term>" --type installs` already covers it (absorb #21); pure reimplementation of existing surface |
| 13 | Account rollup report | `account report <id>` | Per-client multi-metric rollup (installs, storage, visits, cert health, backup recency) | Tomás | (c) | none | Soft kill in Pass 3 — report, not decision |
| 14 | Fleet diff | `fleet diff --since 7d` | What changed across the fleet between syncs: new installs, removed domains, version bumps | Marisol | (c) | none | Soft kill in Pass 3 — needs snapshot-history tables the full-refresh sync doesn't retain |
| 15 | Bulk purge by filter | `fleet purge --tag clientX --type cdn` | Iterate cache purge over installs matched by a local filter | Diego | (b) | none | Soft kill in Pass 3 — mass-mutation footgun |
| 16 | Autoload/storage growth scan | `audit autoload` | Installs whose `autoloaded_bytes` or DB storage grew abnormally week-over-week from usage history | Diego | (c)(f) | none | Soft kill in Pass 3 — occasional, not weekly |

Kill/keep checks applied inline: no candidate needs an LLM, an external service, or auth beyond the same Basic credentials. #8 is mechanical extrapolation, not "insights". #12 killed immediately as reimplementation of the framework `search` command. All local-store candidates are local-data commands, not fake API calls.

## Survivors and kills

Pass 3 answers, condensed per survivor: all seven pass weekly-use (Marisol's Monday audit runs 1-3; Diego runs 5 on every deploy, twice weekly; Priya checks 4 weekly across in-flight launches; Tomás runs 8 every Friday; 7 fires on every support ticket naming a domain). None is a single-endpoint wrapper: 1-4, 7, 8 are multi-table SQLite joins; 5 is a compound lifecycle workflow exploiting the `requested→in-progress→completed` enum. Transcendence source is local SQLite for 1-4/7/8 and a service-specific content pattern for 5. Sibling kills are named in the kills table. All seven are `hand-code` (~80-150 LoC each plus `root.go` wiring); local-join commands use the drain-first pattern (scan parent rows into structs, close, then resolve names), call `hintIfUnsynced`/`hintIfStale` before returning local results, and carry `// pp:data-source local`; `guard` carries `// pp:data-source live`. Long-description validity: every redirect below points only at surviving hand-coded commands or generated manifest commands (`backup create`, `backup list`, `install purge-cache`).

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Cert expiry audit | `audit certs --expiring 30d` | 10/10 | Marisol | hand-code | Joins synced installs↔domains↔certificates in local SQLite, computes days-to-expiry, sorts ascending; `pp:data-source local` | Product Thesis names "which certs expire this month" verbatim; Workflow 4 lists cert expiry monitoring; no community tool covers certs at all | Use this command for fleet-wide certificate expiry windows. Do NOT use it to find domains with no certificate at all or unverified domains; use 'audit domains' instead. |
| 2 | Backup staleness audit | `audit backups --stale 7d --env production` | 10/10 | Marisol | hand-code | Joins synced installs↔backups locally, keeps latest completed backup per install, filters by age and environment; `pp:data-source local` | Product Thesis names "which prod installs have no backup in 7 days" verbatim; Workflow 2 | Use this command for fleet-wide backup staleness across installs. Do NOT use it to list backups for a single install; use 'backup list' instead. |
| 3 | Version drift audit | `audit versions --php-below 8.2 [--drift]` | 10/10 | Marisol | hand-code | Aggregates php_version/wp_version over synced installs with threshold filters; `--drift` groups by site and flags environments whose versions disagree; `pp:data-source local` | Product Thesis names "which client installs are on PHP < 8.2" verbatim; Workflow 1 "find installs by PHP/WP version"; env triad is the Data Profile's core hierarchy | none |
| 4 | Domain health audit | `audit domains` | 8/10 | Priya | hand-code | Local join of domains↔certificates↔redirects: emits unverified domains, verified domains with zero cert coverage, redirects targeting nonexistent domains; `pp:data-source local` | Workflow 4 (TXT verification, Entri, Let's Encrypt lifecycle); Build Priority 3 names "orphan domains"; spec exposes verification status per domain | Use this command for fleet-wide domain health: unverified domains, missing cert coverage, dangling redirects. Do NOT use it to look up which install serves one domain; use 'whois' instead. Do NOT use it for cert expiry windows; use 'audit certs' instead. |
| 5 | Deploy guard | `guard <install> [--purge cdn\|page\|object\|all]` | 9/10 | Diego | hand-code | Calls live `backup create`, polls `backup get` until `completed`/failed with typed exits, then optionally calls `install purge-cache`; `pp:data-source live` | Workflow 2 verbatim ("trigger checkpoint backups before deploys, poll status"); Codebase Intelligence documents the `requested→in-progress→completed` lifecycle; ryanshoover wpe-cli's backup+flush pairing shows demand | Use this command to gate a deploy: it creates a checkpoint backup, waits for completion, and optionally purges cache, with CI-friendly exit codes. Do NOT use it to trigger a backup without waiting; use 'backup create' instead. Do NOT use it to purge cache alone; use 'install purge-cache' instead. |
| 6 | Reverse domain lookup | `whois <domain>` | 8/10 | Marisol | hand-code | Single local join from domain name to install, site, account, environment, primary_domain flag, cert status, redirect target; `pp:data-source local` | Workflow 1's stated pain ("painfully click-heavy across accounts"); Data Layer lists domain/install/site FTS fields and the install↔domains↔certs join as local-only | Use this command to resolve one domain name to the install, site, and account that serve it, plus its cert and redirect status. Do NOT use it for fleet-wide domain health scans; use 'audit domains' instead. |
| 7 | Overage projection | `audit usage --horizon 30d` | 9/10 | Tomás | hand-code | Linear month-end extrapolation of billable_visits and storage from locally retained DailyUsageMetrics history joined against synced account limits; flags accounts projected past limit; `pp:data-source local` | Workflow 5 "limits vs. actuals"; absorb #13 already commits to local history retention beyond the API window; spec ships summary/insights/limits endpoints | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Backup wait (`backup wait`) | Degenerate case of the deploy guard's poll stage; two commands for one intent would split agent tool choice | Deploy guard |
| Environment triad drift (`fleet triads`) | Not independently weekly; folded into `audit versions --drift`, which is where the Monday audit already looks | Version drift audit |
| SSL coverage gap (`audit ssl-coverage`) | Overlapping input surface with two audits guaranteed agent tool confusion; coverage gaps folded into `audit domains`, expiry stays in `audit certs` | Domain health audit |
| Orphan domains (`fleet orphan-domains`) | Same table joins and same persona ritual as domain health; merged into `audit domains` | Domain health audit |
| Install resolver (`find`) | Reimplementation of the framework `search "<term>" --type installs` surface (absorb #21) | Reverse domain lookup |
| Account rollup report (`account report`) | A report, not a decision command; every actionable slice already ships as an audit, and counts come from framework `analytics` | Overage projection |
| Fleet diff (`fleet diff --since 7d`) | Requires per-resource snapshot-history tables the full-refresh sync layer does not retain — new infrastructure, feasibility 0 | Backup staleness audit |
| Bulk purge by filter (`fleet purge`) | Mass mutation across locally filtered installs is a footgun; the weekly ritual is per-deploy, which the guard covers | Deploy guard |
| Autoload/storage growth scan (`audit autoload`) | Perf-smell scan is occasional, not weekly; retained usage history stays queryable via the framework `sql` command | Overage projection |
