# RunCloud CLI — Absorb Manifest (v3)

Target: `runcloud-pp-cli` v3. Authoritative source: official PHP SDK `src/config.php` (persisted at `runcloud-sdk-config.php` in this run's research dir).

## Absorbed (match or beat everything that exists)

### core-api (~85 endpoints)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | Servers list/get/create/delete | PHP SDK `get-servers`/`create-server`/`get-server-details`/`delete-server` | `servers list/get/create/delete` | SQLite cache + FTS, `--json`, typed exit codes, `--dry-run` | spec-emit |
| 2 | Server shared list | PHP SDK `get-shared-servers` | `servers shared` | Cached | spec-emit |
| 3 | Server stats + hardware + install script | PHP SDK `get-server-stats`/`get-server-hardware-info`/`get-server-installation-script` | `servers stats/hardware/install-script` | Agent-native | spec-emit |
| 4 | Server SSH config get/set + autoupdate + metadata | PHP SDK `get-server-ssh-configuration`/`update-server-ssh-configuration`/`update-server-software-update-settings`/`update-server-metadata` | `servers ssh-config get/set`, `servers autoupdate set`, `servers meta-update` | Validation, `--dry-run` | spec-emit |
| 5 | Server PHP CLI version + available versions | PHP SDK `get-available-server-php-version`/`update-server-php-cli-version` | `servers php list/change-cli` | `--json` matrix | spec-emit |
| 6 | Server health latest snapshot (v3) | PHP SDK `get-server-lastest-health-data` → `/health/latest` | `servers health` | Cached snapshot | spec-emit |
| 7 | Server disk cleanup (v3) | PHP SDK `cleanup-server-disk` → `/health/diskcleaner` | `servers disk-cleanup` | `--dry-run` | spec-emit |
| 8 | Server logs | PHP SDK `get-server-logs` | `servers logs` | Terminal output | spec-emit |
| 9 | Webapps CRUD + default + alias + rebuild | PHP SDK `create-web-app`/`get-web-apps`/`get-web-app-details`/`delete-web-app`/`set-default-web-app`/`remove-default-web-app`/`create-web-app-alias`/`rebuild-web-app` | `webapps list/get/create/delete/default/alias/rebuild` | FTS-resolved `--webapp <name>`, `--dry-run` | spec-emit |
| 9a | Webapps WordPress one-shot create (v3-new, single endpoint) | docs `POST /servers/{id}/webapps/wordpress` (api-8617109) | `webapps create-wordpress` | Single-call provisioning (webapp + db + dbuser + grant atomically) | spec-emit |
| 10 | Webapp settings (php / fpm-nginx) + activity log | PHP SDK `get-web-app-settings`/`update-web-app-php-version`/`update-web-app-fpm-nginx-settings`/`get-web-app-activity-log` | `webapps settings get`, `webapps settings php/fpm-nginx`, `webapps log` | Pipe-safe | spec-emit |
| 11 | Webapp git clone/info/branch/script/force-deploy/delete | PHP SDK git_* (6) | `webapps git clone/info/branch/script/deploy/delete` | `--dry-run` on destructive | spec-emit |
| 12 | Webapp installer install/get/remove | PHP SDK installer_* (3) | `webapps installer install/get/remove` | Idempotent | spec-emit |
| 13 | Webapp domains CRUD | PHP SDK domain_* (4) | `webapps domains list/get/add/delete` | FTS over domain names | spec-emit |
| 14 | Webapp SSL basic install/get/update/redeploy/delete | PHP SDK basic ssl (5) | `webapps ssl install/get/update/redeploy/delete` | Idempotent | spec-emit |
| 15 | Webapp SSL advanced status/switch + per-domain CRUD | PHP SDK advanced ssl (7) | `webapps ssl advanced status/switch`, `webapps domains ssl install/get/update/redeploy/delete` | | spec-emit |
| 16 | Databases CRUD | PHP SDK db (4) | `databases list/get/create/delete` | Cached | spec-emit |
| 17 | Database users CRUD + password | PHP SDK dbuser (5) | `db-users list/get/create/delete/password` | Pipe-safe | spec-emit |
| 18 | Database grants attach/list/revoke | PHP SDK grant (3) | `databases grant`, `databases list-grants`, `databases revoke` | Idempotent | spec-emit |
| 19 | System users CRUD + password + deployment key | PHP SDK user (6) | `system-users list/get/create/delete/password/deployment-key` | Pipe-safe | spec-emit |
| 20 | SSH keys per server CRUD | PHP SDK sshcredentials (4) | `ssh-keys list/get/add/delete` | FTS resolution | spec-emit |
| 21 | Cron jobs CRUD + rebuild | PHP SDK cronjob (5) | `cron-jobs list/get/create/delete/rebuild` | `--dry-run` | spec-emit |
| 22 | Supervisor CRUD + binaries + status + reload + rebuild | PHP SDK supervisor (8) | `supervisor list/get/create/delete/status/reload/rebuild/binaries` | `--dry-run` | spec-emit |
| 23 | Services list + trigger | PHP SDK services (2) | `services list/control` | Typed action validation | spec-emit |
| 24 | Firewall rules CRUD + deploy | PHP SDK firewall (5) | `firewall list/get/create/delete/deploy` | Plan-before-apply | spec-emit |
| 25 | Fail2Ban blocked IPs list + unblock | PHP SDK fail2ban (2) | `security blocked-ips list/unblock` | Pipe-friendly | spec-emit |
| 26 | 3rd-party API keys CRUD | PHP SDK externalapi (5) | `external-keys list/get/create/update/delete` | Redacted output | spec-emit |
| 27 | Static data (collations, timezones, installers, ssl protocols) | PHP SDK static (4) | `static collations/timezones/installers/ssl-protocols` | Local SQLite-backed lookups | spec-emit |

### agency-api (~46 endpoints, v3-new top-level group)

| # | Feature | Best Source | Our Implementation | Status |
|---|---------|-------------|--------------------|--------|
| 28 | Agency account details | PHP SDK `get-agency-account` | `agency account` | spec-emit |
| 29 | Agency clients CRUD + password + magic link + tags | PHP SDK agency clients (8) | `agency clients list/get/create/delete/update/password/magic-link/tags` | spec-emit |
| 30 | Agency server packages CRUD + upgrades + available-upgrades + client-servers + duplicate | PHP SDK agency packages (9) | `agency packages list/get/create/update/delete/upgrades/available-upgrades/client-servers/duplicate` | spec-emit |
| 31 | Agency teams CRUD + member invitations + members + transfers + server-package assign + server assign | PHP SDK agency teams (15) | `agency teams list/get/create/update/delete`, `agency teams invitations create/cancel`, `agency teams members get/update/delete/transfer`, `agency teams packages add/remove`, `agency teams servers add/remove` | spec-emit |
| 32 | Agency client-servers CRUD + suspend/unsuspend + rebuild + change-client + assign + upgrade-package + add-ip + resend-webhook | PHP SDK agency client-servers (13) | `agency client-servers list/get/create/delete/suspend/unsuspend/rebuild/change-client/assign-server/upgrade-package/add-ip/resend-webhook` | spec-emit |

### Cross-cutting absorbed capabilities

| # | Feature | Source | Our Implementation | Status |
|---|---------|--------|--------------------|--------|
| 33 | Pagination | docs `page`/`perPage` (max 40) | Auto-paginate with `--all`, default `--per-page 40` | spec-emit |
| 34 | Agent-native output flags | n/a | `--json/--select/--csv/--compact/--quiet/--dry-run` on every command | spec-emit |
| 35 | Offline cross-resource FTS | n/a | `runcloud-pp-cli search "<term>"` over synced SQLite | spec-emit (generator) |
| 36 | SQL access | n/a | `runcloud-pp-cli sql 'SELECT …'` | spec-emit (generator) |
| 37 | Doctor health check | generic | `runcloud-pp-cli doctor` — bearer token valid, `/ping` reachable, rate-limit headroom | spec-emit (generator) |
| 38 | Sync / stale / reconcile / orphans | n/a | `runcloud-pp-cli sync/stale/reconcile/orphans` | spec-emit (generator) |

## Transcendence (only possible with our approach)

9 hand-coded novel features. Each ~50-150 LoC + `root.go` wiring. Hand-code commitment count: **9**.

**Dropped during gate review:** Original flagship `webapps wordpress` (v2-style 7-call chain) was demoted — v3 ships `POST /servers/{id}/webapps/wordpress` as a single-call provisioner that creates the webapp + database + db-user + grant atomically (see absorbed #9a). The v2 chain was a v3 wrapper, not transcendence.

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|-------------------------|
| 1 | Fleet SSL audit (flagship) | `fleet ssl-audit [--expiring-within Nd] [--missing]` | 10/10 | hand-code | Joins synced `webapps × domains × ssl_certs` across every server in one SQL query. The v3 API is server-keyed (`/servers/{id}/...`) and structurally cannot answer "every cert across the fleet" in one request. |
| 2 | Reverse domain lookup | `whois-fleet <domain-or-pattern>` | 9/10 | hand-code | FTS5 over local mirror of `domains`/`webapps`/`servers`; returns server + webapp + system-user + SSL row for any matching domain. The dashboard requires you to already know which server hosts a domain. |
| 3 | Fleet PHP version audit | `fleet php-audit [--below X.Y]` | 8/10 | hand-code | Joins `servers.phpCliVersion` + `webapps.phpVersion` from synced store; SemVer comparison. Pre-upgrade scoping that requires opening every server's PHP tab today. |
| 4 | Fleet health roll-up | `fleet health` | 8/10 | hand-code | Fan-out to v3's `/servers/{id}/health/latest` (new in v3 — replaces v2's `/stats`); caches in store and prints comparable table. Bounded under `IsDogfoodEnv`. |
| 5 | Fleet blocked-IPs dedupe | `fleet blocked-ips [--since Nd] [--ip X]` | 8/10 | hand-code | Aggregates per-server fail2ban rows into local table keyed by IP with `servers_count`, `first_seen`, `last_seen`. The same attacker IP appearing on 5 servers is currently 5 separate clicks. |
| 6 | Fleet WordPress/CMS inventory | `fleet installers [--type wordpress]` | 8/10 | hand-code | Joins per-webapp `/installer` results across the fleet; surfaces site URL + CMS type + version. RunCloud's core identity is WordPress hosting; this is the inventory the dashboard refuses to assemble. |
| 7 | Agency client onboarding chain (v3-new) | `agency onboard --client-email X --package <id> --server-name Y [--magic-link]` | 7/10 | hand-code | Chains v3 agency-api: `POST /agency/clients` → `POST /agency/client-servers` (with `server_package_id`) → optional `POST /agency/clients/{id}/magic-link`. No single-call equivalent exists in v3. Emits a credentials summary the agency can hand directly to the new client. |
| 8 | Fleet SSH-key audit | `fleet ssh-keys [--fingerprint X]` | 6/10 | hand-code | Dedupes per-(server, system_user) `/sshcredentials` rows by fingerprint; emits `fingerprint → [(server, user)]` inventory. Quarterly key-rotation requires this and the API gives only one-server-one-user lists today. |
| 9 | Fleet stale services | `fleet services [--not-running] [--name X]` | 6/10 | hand-code | Filters synced per-server services across the fleet for non-running rows or a named service. "Is nginx running everywhere?" should be one command, not 12. |

## Dropped from v2 (reprint verdicts)

| Prior feature | Verdict | Reason |
|---------------|---------|--------|
| `webapps wordpress` (v2-style 7-call chain) | Drop | v3 ships `POST /servers/{id}/webapps/wordpress` as a single-call atomic provisioner — the v2 chain became a v3 wrapper. Absorbed as `webapps create-wordpress` (spec-emit). |
| `webapps provision` (non-WordPress) | Drop | Niche; spec-emit `webapps create` covers it. |
| `fleet orphan-dbs` | Drop | Low weekly-use; reachable via `sql` over synced tables. |
| `fleet no-git` | Drop | Niche archaeology; reachable via `sql`. |

## v3-additional resources (added during gate review, beyond PHP SDK)

| # | Feature | Endpoint(s) confirmed | Implementation | Status |
|---|---------|----------------------|----------------|--------|
| 39 | WAF install | `POST /servers/{id}/webapps/{id}/waf` (paranoia 1-4, anomaly 1-100, rulesExclusion per-CMS, rulesOrdering) | `webapps waf install` | spec-emit (partial; CRUD verbs TBD) |
| 40 | RunCloudHub install (WP object cache) | `POST /servers/{id}/webapps/{id}/runcloudhub` (cacheType: native/redis, redisObjectCache, cacheFolderSize, cacheValidMinute) | `webapps runcloudhub install` | spec-emit (partial; status/update/uninstall TBD) |
| 41 | Backup create | `POST .../backups` (Full to RunCloud Storage; Full to Local Storage) with retention, frequency, notifications, file/table exclusions | `backups create-full --target runcloud-storage\|local` | spec-emit (create-only initial; list/restore/delete TBD) |
| 42 | S3 storage integration | `PATCH .../s3-storage` update | `external-keys s3 update` | spec-emit |
| 43 | v3 Create Server (richer body) | `POST /servers` with provider, webServerType (nginx/ols), installationType (native/containerized — v3-new) | superset of PHP SDK `create-server` | spec-emit (replaces SDK shape) |

## Residual gap

PHP SDK action map: ~130 endpoints. Apidog project (user-confirmed, not exportable): **222 endpoints**. Confirmed during gate review: ~13 additional endpoints (WAF/RunCloudHub/Backups/S3/v3-server-creation/WordPress one-shot). Estimated residual gap: **~80 endpoints**, expected to cover:

- WAF: get/update/delete/rules CRUD (~7-10 endpoints)
- RunCloudHub: status/update/uninstall (~3-5 endpoints)
- Backups: list/restore/delete/history/database-only/file-only variants (~15-20 endpoints)
- Agency-api edge cases (workspace settings, billing, additional team member ops)
- Possibly: notification routing, webhook subscriptions, container-stack operations, tags, snapshots

Documented in `research.json` gaps[]. v0.2 reprint via `/printing-press-amend runcloud` or `/printing-press --spec <apidog-export>` will fill this when Apidog data becomes exportable.
