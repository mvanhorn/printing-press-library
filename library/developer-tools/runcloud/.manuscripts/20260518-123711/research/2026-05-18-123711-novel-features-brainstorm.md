# Novel Features Brainstorm — RunCloud v3 (reprint of v2)

Subagent run: 2026-05-18 12:37:11. Reprint reconciliation against `~/printing-press/manuscripts/runcloud-v2-archive/20260515-205429/research.json`.

## Customer model

**Persona 1: Maya — WordPress agency operator (8-25 client sites across 4-12 servers).**
- Today: lives in the RunCloud dashboard one server at a time; keeps a Notion page mapping client → server → domain.
- Weekly ritual: Monday SSL/PHP eyeball check across servers; Friday "did anything break" sweep.
- Frustration: every cross-server question (which sites expire this month? which are on PHP 7.4? where does foo.com live?) is a manual fan-out.

**Persona 2: Devon — agency owner using v3's agency-api workspace features.**
- Today: invites clients via dashboard, manually maps `server_package` tiers to client billing, exports client lists by copy-paste.
- Weekly ritual: onboarding 1-3 new clients (create client → assign server-package → spin up client-server → magic-link handoff); reviewing team-member access.
- Frustration: the agency surface is per-modal — there's no roll-up of "which clients are on which packages with which usage" and no scripted onboarding chain.

**Persona 3: Sam — freelance dev / solo SRE on 5-15 PHP/Laravel servers.**
- Today: SSHes when something feels wrong; uses dashboard for provisioning; relies on email alerts for outages.
- Weekly ritual: Monday morning health glance; ad-hoc "is nginx running everywhere" check; new-site provisioning ~once a fortnight.
- Frustration: dashboard health page is one-server-at-a-time; fail2ban triage means clicking through each server's security tab.

**Persona 4: Riley — security-conscious devops on a small prod fleet.**
- Today: rotates SSH keys quarterly, audits fail2ban manually, no fleet-level inventory of who-has-access-where.
- Weekly ritual: Friday security sweep — blocked IPs, recent firewall changes, SSH key inventory.
- Frustration: SSH keys are scoped per system-user per server; there's no "show me every key on every server, deduped by fingerprint" view, and the same attacker IP shows up in 5 separate dashboard pages.

## Candidates (pre-cut)

(See subagent transcript for full list — 21 candidates generated across sources (a) persona-driven, (b) service-specific, (c) cross-entity, (d) reprint reconciliation MANDATORY [12 prior features scored], (e) user-vision, (f) codebase intelligence. 10 survived; 11 killed.)

## Survivors

| # | Feature | Command | Score | Buildability | How It Works | Source |
|---|---------|---------|-------|--------------|--------------|--------|
| 1 | WordPress end-to-end provision | `webapps wordpress --server X --name Y --domain Z --user U --admin-email E --admin-user A [--ssl]` | 10/10 | hand-code | Chains `POST /webapps/custom` → `POST /webapps/{id}/domains` → `POST /databases` → `POST /databaseusers` → `POST /databases/{id}/grant` → `POST /webapps/{id}/installer` → `POST /webapps/{id}/ssl` with partial-failure reporting + `--resume`. | prior (kept) |
| 2 | Fleet SSL audit | `fleet ssl-audit [--expiring-within Nd] [--missing]` | 10/10 | hand-code | SQL over local `webapps ⋈ domains ⋈ ssl_certs`; API is server-keyed and structurally cannot answer this. | prior (kept) |
| 3 | Reverse domain lookup | `whois-fleet <domain-or-pattern>` | 9/10 | hand-code | FTS5 over `domains.name`/`webapps.name`/`servers.serverName`; returns server + webapp + system-user + SSL row. | prior (kept) |
| 4 | Fleet PHP version audit | `fleet php-audit [--below X.Y]` | 8/10 | hand-code | Joins `servers.phpCliVersion` + `webapps.phpVersion` from synced store; SemVer comparison. | prior (kept) |
| 5 | Fleet health roll-up | `fleet health` | 8/10 | hand-code | Fans out to v3 `/health/latest` (new in v3), caches in store, prints comparable table. Bounded under `IsDogfoodEnv`. | prior (reframed v2 `/stats` → v3 `/health/latest`) |
| 6 | Fleet blocked-IPs dedupe | `fleet blocked-ips [--since Nd] [--ip X]` | 8/10 | hand-code | Aggregates per-server fail2ban rows into local table keyed by IP with `servers_count`, `first_seen`, `last_seen`. | prior (kept) |
| 7 | Fleet WordPress/CMS inventory | `fleet installers [--type wordpress]` | 8/10 | hand-code | Joins per-webapp `/installer` results across fleet; surfaces site URL + CMS type + version. | prior (kept) |
| 8 | Agency client onboarding chain | `agency onboard --client-email X --package <id> --server-name Y [--magic-link]` | 7/10 | hand-code | Chains v3 agency-api: `POST /agency/clients` → `POST /agency/client-servers` (with `server_package_id`) → optional magic-link; emits credentials summary. | new (v3-only agency-api) |
| 9 | Fleet SSH-key audit | `fleet ssh-keys [--fingerprint X]` | 6/10 | hand-code | Dedupes per-(server, system_user) `/sshcredentials` rows by fingerprint; emits `fingerprint → [(server, user)]` inventory. | prior (kept) |
| 10 | Fleet stale services | `fleet services [--not-running] [--name X]` | 6/10 | hand-code | Filters synced per-server `services` table for non-running rows or named service across fleet. | prior (kept) |

Hand-code commitment: 10 commands, all `hand-code`, each ~50-150 LoC + `root.go` wiring.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| `fleet orphan-dbs` | Low weekly-use; quarterly-cleanup only. | `fleet php-audit` |
| `fleet no-git` | Niche archaeology; reachable via `sql`. | `fleet installers` |
| `servers disk-cleanup` | Spec-emit wrapper for one v3 endpoint; not transcendence. | (spec-emitted) |
| `fleet cert-renewals` | Sibling — same query as `--expiring-within`. | `fleet ssl-audit --expiring-within` |
| `fleet wordpress-versions` | Sibling — same join. | `fleet installers --type wordpress` |
| `fleet domains-without-ssl` | Sibling — same query as `--missing`. | `fleet ssl-audit --missing` |
| `fleet firewall-diff` | Speculative; no research backing. | (none) |
| `fleet cron-conflicts` | Speculative; no evidence of real ritual. | (none) |
| `webapps waf` / `webapps runcloudhub` | Spec-emit candidates (v3-documented), not novel. | (spec-emit) |
| `agency client-usage` | Answerable via `sql` once tables synced. | `agency onboard` + `sql` |
| `webapps provision` (non-WP) | Subsumed by WP chain superset; rare for non-WP. | `webapps wordpress` |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Fleet SSL audit (`fleet ssl-audit`) | **Keep** | Persona fit (Maya weekly); score 10/10. |
| Fleet PHP audit (`fleet php-audit`) | **Keep** | Persona fit (Maya pre-upgrade); score 8/10. |
| Reverse domain lookup (`whois-fleet`) | **Keep** | Persona fit; score 9/10; structurally impossible via API. |
| Fleet blocked-IPs (`fleet blocked-ips`) | **Keep** | Riley Friday sweep; score 8/10. |
| Fleet stale services (`fleet services`) | **Keep** | Sam on-call; score 6/10. |
| Server health roll-up (`fleet health`) | **Reframe** | Same intent; switch data source from v2 `/stats` to v3 `/health/latest`. |
| Installed scripts roll-up (`fleet installers`) | **Keep** | Maya WordPress inventory; score 8/10. |
| Provision a new site end-to-end (`webapps provision`) | **Drop** | Subsumed by `webapps wordpress` superset; non-WP use is rare. |
| Orphaned databases (`fleet orphan-dbs`) | **Drop** | No weekly persona; reachable via `sql`. |
| Webapps without git (`fleet no-git`) | **Drop** | Niche; reachable via `sql`. |
| SSH key fleet audit (`fleet ssh-keys`) | **Keep** | Riley quarterly; score 6/10. |
| WordPress end-to-end provision (`webapps wordpress`) | **Keep** | Flagship; score 10/10; all 7 chained endpoints confirmed in v3. |

**New for v3:** `agency onboard` — v3 introduces the agency-api surface; Devon persona requires a chained onboarding command.
