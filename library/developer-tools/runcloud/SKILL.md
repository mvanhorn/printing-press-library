---
name: pp-runcloud
description: "Fleet-wide visibility for RunCloud v3 — every SSL cert, PHP version, blocked IP, and SSH key across the whole... Trigger phrases: `ssl cert expiring across my runcloud servers`, `which server hosts <domain>`, `spin up a wordpress site on runcloud`, `runcloud fleet health`, `fail2ban blocked ips runcloud`, `use runcloud`, `run runcloud`."
author: "jacobprice"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - runcloud-pp-cli
---

# RunCloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `runcloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install runcloud --cli-only
   ```
2. Verify: `runcloud-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

RunCloud's dashboard is per-server. This CLI mirrors the v3 API into a local SQLite store and gives you the cross-server queries the dashboard refuses to assemble: `fleet ssl-audit` for cert expiry across the fleet, `whois-fleet` for domain → server reverse lookup, `fleet health` powered by v3's new /health/latest endpoint, and `fleet blocked-ips` for fail2ban dedupe. v3's agency-api workspace surface is exposed under `runcloud-pp-cli agency` with a chained `agency onboard` command for new-client provisioning. WordPress one-shot provisioning rides v3's native `POST /webapps/wordpress` endpoint via `webapps create-wordpress`.

## When to Use This CLI

Use this CLI when you operate more than one RunCloud server and have to ask fleet-wide questions the dashboard cannot answer in one click — SSL expiry across servers, PHP version compliance, fail2ban dedupe, WordPress site inventory, domain reverse lookup. Also use it for one-shot WordPress provisioning and (on v3 agency plans) chained client onboarding. Pair it with `--json`/`--select` for agentic workflows that need to feed RunCloud state into downstream automation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fleet-wide queries (impossible via API)
- **`fleet ssl-audit`** — Find every SSL certificate across every server that is missing, expiring soon, or already expired — one query, whole fleet.

  _Agents auditing certificate hygiene get a deterministic table instead of having to fan out N parallel /servers/{id}/webapps/{id}/ssl calls._

  ```bash
  runcloud-pp-cli fleet ssl-audit --expiring-within 30d --agent --select webapp,domain,server,expires_at
  ```
- **`whois-fleet`** — Given a domain (or pattern), return the server, webapp, system user, and SSL config that hosts it.

  _Maps a domain to its operational context in one call — essential before any cross-resource action._

  ```bash
  runcloud-pp-cli whois-fleet example.com --agent
  ```
- **`fleet php-audit`** — Find every webapp running below a given PHP version, grouped by server.

  _Pre-upgrade scoping; current alternative is opening every server's PHP tab._

  ```bash
  runcloud-pp-cli fleet php-audit --below 8.2 --agent
  ```
- **`fleet health`** — Show uptime, load, disk, and memory for every server in one comparable table, powered by v3's new /health/latest endpoint.

  _Monday-morning fleet glance — one read instead of N dashboard tabs._

  ```bash
  runcloud-pp-cli fleet health --agent --select server,uptime,load,disk_used_pct,memory_used_pct
  ```
- **`fleet blocked-ips`** — Show every fail2ban-blocked IP across the fleet, deduped, with server count, first seen, and last seen.

  _Identify pattern attackers and shared-blocklist candidates in one query._

  ```bash
  runcloud-pp-cli fleet blocked-ips --since 7d --agent
  ```
- **`fleet installers`** — List every webapp that has a script-installer attached (WordPress, Joomla, Drupal, etc.) with version and site URL.

  _Find every WordPress site for compliance, version sweeps, or plugin auditing._

  ```bash
  runcloud-pp-cli fleet installers --type wordpress --agent
  ```
- **`fleet ssh-keys`** — Inventory every SSH key across every (server, system_user) pair, deduped by fingerprint.

  _Find stale keys, identify shared keys across users, prepare for rotation._

  ```bash
  runcloud-pp-cli fleet ssh-keys --fingerprint AAAA... --agent
  ```
- **`fleet services`** — Filter every server's services list for non-running rows or a named service across the whole fleet.

  _On-call ritual — instant verification that critical services are up across the fleet._

  ```bash
  runcloud-pp-cli fleet services --not-running --agent
  ```

### Workflow chains
- **`agency onboard`** — Onboard a new agency client end-to-end: create client → assign server-package → spin up client-server → optional magic link. Emits a credentials summary.

  _Replaces a multi-modal dashboard flow with one scripted command — essential for agencies running v3's reseller features._

  ```bash
  runcloud-pp-cli agency onboard --client-email new@example.com --package 12 --server-name client-prod --magic-link
  ```

## Command Reference

**agency_account** — Agency account details (v3 agency-api)

- `runcloud-pp-cli agency_account` — Get agency account details

**agency_client_servers** — Agency client servers — provisioned servers tied to an agency client (v3 agency-api)

- `runcloud-pp-cli agency_client_servers add-ip` — Add an IP address to an agency client server
- `runcloud-pp-cli agency_client_servers assign-server` — Assign a server to the client-server slot
- `runcloud-pp-cli agency_client_servers change-client` — Reassign an agency client server to a different client
- `runcloud-pp-cli agency_client_servers create` — Create a new agency client server
- `runcloud-pp-cli agency_client_servers delete` — Delete an agency client server
- `runcloud-pp-cli agency_client_servers get` — Get an agency client server's details
- `runcloud-pp-cli agency_client_servers list` — List all agency client servers
- `runcloud-pp-cli agency_client_servers rebuild` — Rebuild an agency client server
- `runcloud-pp-cli agency_client_servers resend-webhook` — Resend the webhook for an agency client server
- `runcloud-pp-cli agency_client_servers suspend` — Suspend an agency client server
- `runcloud-pp-cli agency_client_servers unsuspend` — Unsuspend an agency client server
- `runcloud-pp-cli agency_client_servers upgrade-package` — Upgrade the server package for an agency client server

**agency_clients** — Agency clients (v3 agency-api)

- `runcloud-pp-cli agency_clients change-password` — Change an agency client's password
- `runcloud-pp-cli agency_clients create` — Create a new agency client
- `runcloud-pp-cli agency_clients delete` — Delete an agency client
- `runcloud-pp-cli agency_clients get` — Get an agency client's details
- `runcloud-pp-cli agency_clients list` — List all agency clients
- `runcloud-pp-cli agency_clients magic-link` — Create a magic-link for quick client dashboard access
- `runcloud-pp-cli agency_clients update` — Update an agency client
- `runcloud-pp-cli agency_clients update-tags` — Update agency client tags

**agency_packages** — Agency server packages (v3 agency-api)

- `runcloud-pp-cli agency_packages available-upgrades` — List current server package's available upgrade plans
- `runcloud-pp-cli agency_packages client-servers` — List client servers using this server package
- `runcloud-pp-cli agency_packages create` — Create an agency server package
- `runcloud-pp-cli agency_packages delete` — Delete an agency server package
- `runcloud-pp-cli agency_packages duplicate` — Duplicate an agency server package
- `runcloud-pp-cli agency_packages get` — Get an agency server package's details
- `runcloud-pp-cli agency_packages list` — List agency server packages
- `runcloud-pp-cli agency_packages update` — Update an agency server package
- `runcloud-pp-cli agency_packages upgrades` — List available upgrade plans for agency server packages

**agency_teams** — Agency teams (v3 agency-api)

- `runcloud-pp-cli agency_teams add-package` — Import a server package into the agency team
- `runcloud-pp-cli agency_teams add-server` — Add a server to the agency team
- `runcloud-pp-cli agency_teams cancel-invitation` — Cancel a pending team member invitation
- `runcloud-pp-cli agency_teams create` — Create an agency team
- `runcloud-pp-cli agency_teams delete` — Delete an agency team
- `runcloud-pp-cli agency_teams get` — Get an agency team's details
- `runcloud-pp-cli agency_teams get-member` — Get an agency team member's details
- `runcloud-pp-cli agency_teams invite-member` — Send an agency team member invitation
- `runcloud-pp-cli agency_teams list` — List all agency teams
- `runcloud-pp-cli agency_teams remove-member` — Remove an agency team member
- `runcloud-pp-cli agency_teams remove-package` — Remove a server package from the agency team
- `runcloud-pp-cli agency_teams remove-server` — Remove a server from the agency team
- `runcloud-pp-cli agency_teams transfer-member` — Transfer team member to another agency team
- `runcloud-pp-cli agency_teams update` — Update an agency team
- `runcloud-pp-cli agency_teams update-member` — Update an agency team member

**backups** — Webapp backups: full and database-only, to RunCloud Storage or local server (v3 new)

- `runcloud-pp-cli backups create-full-cloud` — Create a full webapp backup to RunCloud Storage
- `runcloud-pp-cli backups create-full-local` — Create a full webapp backup to the local server

**cron_jobs** — Manage scheduled cron jobs

- `runcloud-pp-cli cron_jobs create` — Schedule a new cron job
- `runcloud-pp-cli cron_jobs delete` — Remove a cron job
- `runcloud-pp-cli cron_jobs list` — List cron jobs on a server

**database_users** — Manage MySQL/MariaDB database users

- `runcloud-pp-cli database_users create` — Create a database user
- `runcloud-pp-cli database_users delete` — Delete a database user
- `runcloud-pp-cli database_users get` — Get a database user by ID
- `runcloud-pp-cli database_users list` — List database users
- `runcloud-pp-cli database_users update-password` — Change a database user's password

**databases** — Manage server databases

- `runcloud-pp-cli databases create` — Create a database
- `runcloud-pp-cli databases delete` — Drop a database
- `runcloud-pp-cli databases get` — Get a database by ID
- `runcloud-pp-cli databases grant` — Grant a database user access to a database
- `runcloud-pp-cli databases list` — List databases on a server
- `runcloud-pp-cli databases list-grants` — List database users with access to a database
- `runcloud-pp-cli databases revoke` — Revoke a database user's access to a database

**domains** — Manage domain attachments for a web application

- `runcloud-pp-cli domains add` — Attach a new domain to a web app
- `runcloud-pp-cli domains delete` — Remove a domain from a web app
- `runcloud-pp-cli domains get` — Get a domain by ID
- `runcloud-pp-cli domains list` — List domains for a web app

**external_keys** — Manage workspace-level third-party API keys (Cloudflare, Linode, DigitalOcean) — v3 moves these from per-server to workspace scope

- `runcloud-pp-cli external_keys create` — Store a 3rd-party API key
- `runcloud-pp-cli external_keys delete` — Delete a stored 3rd-party API key
- `runcloud-pp-cli external_keys get` — Get a 3rd-party API key by ID
- `runcloud-pp-cli external_keys list` — List 3rd-party API keys in the workspace
- `runcloud-pp-cli external_keys update` — Update a 3rd-party API key

**fail2ban** — Fail2Ban blocked-IP visibility and unblock

- `runcloud-pp-cli fail2ban list` — List Fail2Ban-blocked IP addresses
- `runcloud-pp-cli fail2ban unblock` — Unblock an IP address from Fail2Ban

**firewall** — Manage server firewall rules

- `runcloud-pp-cli firewall create` — Create a firewall rule (stage; call deploy to apply)
- `runcloud-pp-cli firewall delete` — Remove a firewall rule (call deploy to apply)
- `runcloud-pp-cli firewall deploy` — Apply staged firewall rules to the live ruleset
- `runcloud-pp-cli firewall get` — Get a firewall rule by ID
- `runcloud-pp-cli firewall list` — List firewall rules

**git** — Git integration for web applications

- `runcloud-pp-cli git change-branch` — Switch the active branch
- `runcloud-pp-cli git clone` — Connect a git repository to a web app
- `runcloud-pp-cli git delete` — Disconnect the git repository
- `runcloud-pp-cli git deploy` — Force a deploy via the configured deploy script
- `runcloud-pp-cli git get` — Get the current git connection
- `runcloud-pp-cli git update-script` — Update the deploy script

**installers** — Install third-party scripts (WordPress, Joomla, Drupal, Magento)

- `runcloud-pp-cli installers get` — Get the currently installed script (if any)
- `runcloud-pp-cli installers install` — Install a script (e.g. WordPress) on a web app
- `runcloud-pp-cli installers remove` — Remove an installed script

**runcloudhub** — RunCloudHub WordPress object-cache plugin (v3 new)

- `runcloud-pp-cli runcloudhub` — Install the RunCloudHub WordPress cache plugin

**s3_storage** — Manage S3-compatible object storage integrations (v3)

- `runcloud-pp-cli s3_storage` — Update S3-compatible object storage integration

**server_logs** — Retrieve server-level logs (nginx, Apache, MySQL)

- `runcloud-pp-cli server_logs` — Retrieve nginx/Apache/MySQL logs from the server

**servers** — Manage RunCloud servers

- `runcloud-pp-cli servers autoupdate` — Configure automatic security/software updates
- `runcloud-pp-cli servers change-php-cli` — Change the server-wide PHP CLI version
- `runcloud-pp-cli servers create` — Register a new server with RunCloud (v3 supports nginx/ols web server and native/containerized install)
- `runcloud-pp-cli servers delete` — Remove a server from your RunCloud account
- `runcloud-pp-cli servers disk-cleanup` — Trigger server disk cleanup (purges logs and cache to reclaim space)
- `runcloud-pp-cli servers get` — Get a server by ID
- `runcloud-pp-cli servers hardware` — Get server hardware specifications (CPU, RAM, OS, kernel)
- `runcloud-pp-cli servers health` — Get latest server health snapshot (v3: uptime, load, disk, memory in a single payload)
- `runcloud-pp-cli servers install-script` — Retrieve the bash installation script to run on the target server
- `runcloud-pp-cli servers list` — List all servers in your account
- `runcloud-pp-cli servers meta-update` — Update server name and provider
- `runcloud-pp-cli servers php-versions` — List PHP versions available on this server
- `runcloud-pp-cli servers shared` — List servers shared with your account
- `runcloud-pp-cli servers ssh-settings` — Get SSH configuration for this server
- `runcloud-pp-cli servers stats` — Get server performance metrics (legacy v2 endpoint; prefer 'servers health' on v3)
- `runcloud-pp-cli servers update-ssh-settings` — Update SSH configuration

**services** — Control system services (nginx, apache, mysql, redis, supervisor)

- `runcloud-pp-cli services control` — Start, stop, restart, or reload a service
- `runcloud-pp-cli services list` — List services and their status

**ssh_keys** — Manage SSH public keys stored on system users

- `runcloud-pp-cli ssh_keys add` — Upload an SSH public key for a system user
- `runcloud-pp-cli ssh_keys delete` — Delete a stored SSH key
- `runcloud-pp-cli ssh_keys list` — List SSH keys for a system user

**ssl** — Manage SSL certificates for a web application (basic mode)

- `runcloud-pp-cli ssl delete` — Delete the SSL certificate
- `runcloud-pp-cli ssl get` — Get the current SSL configuration
- `runcloud-pp-cli ssl install` — Install an SSL certificate (Let's Encrypt or custom)
- `runcloud-pp-cli ssl redeploy` — Force a Let's Encrypt redeploy of the SSL certificate
- `runcloud-pp-cli ssl update` — Update SSL configuration (HSTS, HTTP toggle, protocol)

**ssl_advanced** — Advanced per-domain SSL mode (one cert per domain)

- `runcloud-pp-cli ssl_advanced delete-domain` — Delete a per-domain SSL certificate
- `runcloud-pp-cli ssl_advanced get-domain` — Get SSL details for a specific domain
- `runcloud-pp-cli ssl_advanced install-domain` — Install SSL for a specific domain (advanced mode only)
- `runcloud-pp-cli ssl_advanced redeploy-domain` — Force a redeploy of a per-domain SSL certificate
- `runcloud-pp-cli ssl_advanced status` — Check whether advanced per-domain SSL is enabled
- `runcloud-pp-cli ssl_advanced switch` — Switch between basic and advanced SSL modes

**static** — Static reference data: timezones, collations, installers, SSL protocols (v3 paths)

- `runcloud-pp-cli static collations` — List supported database collations (v3 path)
- `runcloud-pp-cli static installers` — List installable scripts (WordPress, Joomla, Drupal, Magento) — v3 path
- `runcloud-pp-cli static ssl-protocols` — List supported SSL protocols (v3 path)
- `runcloud-pp-cli static timezones` — List supported timezones

**supervisor** — Manage long-running supervisor processes

- `runcloud-pp-cli supervisor control` — Start, stop, or restart a supervisor job
- `runcloud-pp-cli supervisor create` — Create a supervisor-managed process
- `runcloud-pp-cli supervisor delete` — Delete a supervisor job
- `runcloud-pp-cli supervisor list` — List supervisor jobs

**system_users** — Manage Linux system users that own web applications

- `runcloud-pp-cli system_users create` — Create a system user
- `runcloud-pp-cli system_users delete` — Delete a system user
- `runcloud-pp-cli system_users get` — Get a system user by ID
- `runcloud-pp-cli system_users list` — List system users on a server
- `runcloud-pp-cli system_users update-password` — Change a system user's password

**waf** — Web Application Firewall management (v3 new)

- `runcloud-pp-cli waf` — Install WAF on a webapp

**webapps** — Manage web applications across servers

- `runcloud-pp-cli webapps alias` — Create an alias web application sharing this one's document root
- `runcloud-pp-cli webapps change-php` — Change the PHP version for this web application
- `runcloud-pp-cli webapps create` — Create a custom web application
- `runcloud-pp-cli webapps create-wordpress` — Provision a WordPress webapp in one call (v3 native): creates the webapp + database + db user + grant atomically
- `runcloud-pp-cli webapps default-set` — Mark this web app as the server default
- `runcloud-pp-cli webapps default-unset` — Remove default status from this web app
- `runcloud-pp-cli webapps delete` — Delete a web application
- `runcloud-pp-cli webapps get` — Get a web application by ID
- `runcloud-pp-cli webapps list` — List web applications on a server
- `runcloud-pp-cli webapps logs` — View the activity log for this web app
- `runcloud-pp-cli webapps rebuild` — Rebuild the web application's nginx/PHP-FPM configuration
- `runcloud-pp-cli webapps settings` — Retrieve all settings for a web application
- `runcloud-pp-cli webapps update-fpmnginx` — Update PHP-FPM and nginx settings


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
runcloud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday SSL sweep

```bash
runcloud-pp-cli fleet ssl-audit --expiring-within 30d --agent --select webapp,domain,server,expires_at
```

Every webapp/domain whose cert expires in the next month, agent-shaped output for piping into a renewal script.

### Domain-to-server reverse lookup

```bash
runcloud-pp-cli whois-fleet example.com --agent
```

Find which server + webapp + system user + SSL config hosts `example.com` — answer the question the dashboard makes you already know.

### Spin up a WordPress site (v3 native)

```bash
runcloud-pp-cli webapps create-wordpress --server-id 42 --name client-site --domain-name example.com --site-title "Client Site" --admin-username wpadmin --admin-email admin@example.com --password ChangeMeStrongPwd123
```

Single call to v3's POST /servers/{id}/webapps/wordpress — creates the webapp, database, db user, and grant atomically. Add `webapps ssl install` as a follow-up if you want HTTPS.

### Onboard a new agency client

```bash
runcloud-pp-cli agency onboard --client-email new@example.com --package 12 --server-name client-prod --magic-link
```

v3 agency-api chain: create-client → create-client-server → optional magic-link; emits credentials summary.

### Friday security sweep

```bash
runcloud-pp-cli fleet blocked-ips --since 7d --agent --select ip,servers_count,first_seen,last_seen
```

Every fail2ban-blocked IP across the fleet from the last week, deduped with `servers_count` to find pattern attackers.

## Auth Setup

RunCloud v3 uses bearer-token auth. Generate a token from Workspace > Settings > API Management, then export `RUNCLOUD_API_TOKEN` (or pass `--api-key`). v2's key:secret pair is no longer accepted.

Run `runcloud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  runcloud-pp-cli agency_account --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
runcloud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
runcloud-pp-cli feedback --stdin < notes.txt
runcloud-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.runcloud-pp-cli/feedback.jsonl`. They are never POSTed unless `RUNCLOUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RUNCLOUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
runcloud-pp-cli profile save briefing --json
runcloud-pp-cli --profile briefing agency_account
runcloud-pp-cli profile list --json
runcloud-pp-cli profile show briefing
runcloud-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `runcloud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add runcloud-pp-mcp -- runcloud-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which runcloud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   runcloud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `runcloud-pp-cli <command> --help`.
