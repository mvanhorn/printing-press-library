# RunCloud CLI

**Fleet-wide visibility for RunCloud v3 — every SSL cert, PHP version, blocked IP, and SSH key across the whole workspace in one command.**

RunCloud's dashboard is per-server. This CLI mirrors the v3 API into a local SQLite store and gives you the cross-server queries the dashboard refuses to assemble: `fleet ssl-audit` for cert expiry across the fleet, `whois-fleet` for domain → server reverse lookup, `fleet health` powered by v3's new /health/latest endpoint, and `fleet blocked-ips` for fail2ban dedupe. v3's agency-api workspace surface is exposed under `runcloud-pp-cli agency` with a chained `agency onboard` command for new-client provisioning. WordPress one-shot provisioning rides v3's native `POST /webapps/wordpress` endpoint via `webapps create-wordpress`.

Printed by [@JacobPrice](https://github.com/JacobPrice) (jacobprice).

## Install

The recommended path installs both the `runcloud-pp-cli` binary and the `pp-runcloud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install runcloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install runcloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install runcloud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install runcloud --agent claude-code
npx -y @mvanhorn/printing-press install runcloud --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/runcloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-runcloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-runcloud --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-runcloud skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-runcloud. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/runcloud-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RUNCLOUD_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "runcloud": {
      "command": "runcloud-pp-mcp",
      "env": {
        "RUNCLOUD_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

RunCloud v3 uses bearer-token auth. Generate a token from Workspace > Settings > API Management, then export `RUNCLOUD_API_TOKEN` (or pass `--api-key`). v2's key:secret pair is no longer accepted.

## Quick Start

```bash
# Verify the bearer token resolves and /ping responds.
runcloud-pp-cli doctor


# Mirror every server, webapp, domain, database, and SSH key into local SQLite.
runcloud-pp-cli sync --full


# First-week-of-the-month ritual: certs expiring in the next month.
runcloud-pp-cli fleet ssl-audit --expiring-within 30d


# Find which server hosts a domain — the dashboard makes you already know.
runcloud-pp-cli whois-fleet example.com


# Monday glance — uptime/load/disk/memory across the fleet, one read.
runcloud-pp-cli fleet health --agent

```

## Unique Features

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

## Usage

Run `runcloud-pp-cli --help` for the full command reference and flag list.

## Commands

### agency_account

Agency account details (v3 agency-api)

- **`runcloud-pp-cli agency_account`** - Get agency account details

### agency_client_servers

Agency client servers — provisioned servers tied to an agency client (v3 agency-api)

- **`runcloud-pp-cli agency_client_servers add-ip`** - Add an IP address to an agency client server
- **`runcloud-pp-cli agency_client_servers assign-server`** - Assign a server to the client-server slot
- **`runcloud-pp-cli agency_client_servers change-client`** - Reassign an agency client server to a different client
- **`runcloud-pp-cli agency_client_servers create`** - Create a new agency client server
- **`runcloud-pp-cli agency_client_servers delete`** - Delete an agency client server
- **`runcloud-pp-cli agency_client_servers get`** - Get an agency client server's details
- **`runcloud-pp-cli agency_client_servers list`** - List all agency client servers
- **`runcloud-pp-cli agency_client_servers rebuild`** - Rebuild an agency client server
- **`runcloud-pp-cli agency_client_servers resend-webhook`** - Resend the webhook for an agency client server
- **`runcloud-pp-cli agency_client_servers suspend`** - Suspend an agency client server
- **`runcloud-pp-cli agency_client_servers unsuspend`** - Unsuspend an agency client server
- **`runcloud-pp-cli agency_client_servers upgrade-package`** - Upgrade the server package for an agency client server

### agency_clients

Agency clients (v3 agency-api)

- **`runcloud-pp-cli agency_clients change-password`** - Change an agency client's password
- **`runcloud-pp-cli agency_clients create`** - Create a new agency client
- **`runcloud-pp-cli agency_clients delete`** - Delete an agency client
- **`runcloud-pp-cli agency_clients get`** - Get an agency client's details
- **`runcloud-pp-cli agency_clients list`** - List all agency clients
- **`runcloud-pp-cli agency_clients magic-link`** - Create a magic-link for quick client dashboard access
- **`runcloud-pp-cli agency_clients update`** - Update an agency client
- **`runcloud-pp-cli agency_clients update-tags`** - Update agency client tags

### agency_packages

Agency server packages (v3 agency-api)

- **`runcloud-pp-cli agency_packages available-upgrades`** - List current server package's available upgrade plans
- **`runcloud-pp-cli agency_packages client-servers`** - List client servers using this server package
- **`runcloud-pp-cli agency_packages create`** - Create an agency server package
- **`runcloud-pp-cli agency_packages delete`** - Delete an agency server package
- **`runcloud-pp-cli agency_packages duplicate`** - Duplicate an agency server package
- **`runcloud-pp-cli agency_packages get`** - Get an agency server package's details
- **`runcloud-pp-cli agency_packages list`** - List agency server packages
- **`runcloud-pp-cli agency_packages update`** - Update an agency server package
- **`runcloud-pp-cli agency_packages upgrades`** - List available upgrade plans for agency server packages

### agency_teams

Agency teams (v3 agency-api)

- **`runcloud-pp-cli agency_teams add-package`** - Import a server package into the agency team
- **`runcloud-pp-cli agency_teams add-server`** - Add a server to the agency team
- **`runcloud-pp-cli agency_teams cancel-invitation`** - Cancel a pending team member invitation
- **`runcloud-pp-cli agency_teams create`** - Create an agency team
- **`runcloud-pp-cli agency_teams delete`** - Delete an agency team
- **`runcloud-pp-cli agency_teams get`** - Get an agency team's details
- **`runcloud-pp-cli agency_teams get-member`** - Get an agency team member's details
- **`runcloud-pp-cli agency_teams invite-member`** - Send an agency team member invitation
- **`runcloud-pp-cli agency_teams list`** - List all agency teams
- **`runcloud-pp-cli agency_teams remove-member`** - Remove an agency team member
- **`runcloud-pp-cli agency_teams remove-package`** - Remove a server package from the agency team
- **`runcloud-pp-cli agency_teams remove-server`** - Remove a server from the agency team
- **`runcloud-pp-cli agency_teams transfer-member`** - Transfer team member to another agency team
- **`runcloud-pp-cli agency_teams update`** - Update an agency team
- **`runcloud-pp-cli agency_teams update-member`** - Update an agency team member

### backups

Webapp backups: full and database-only, to RunCloud Storage or local server (v3 new)

- **`runcloud-pp-cli backups create-full-cloud`** - Create a full webapp backup to RunCloud Storage
- **`runcloud-pp-cli backups create-full-local`** - Create a full webapp backup to the local server

### cron_jobs

Manage scheduled cron jobs

- **`runcloud-pp-cli cron_jobs create`** - Schedule a new cron job
- **`runcloud-pp-cli cron_jobs delete`** - Remove a cron job
- **`runcloud-pp-cli cron_jobs list`** - List cron jobs on a server

### database_users

Manage MySQL/MariaDB database users

- **`runcloud-pp-cli database_users create`** - Create a database user
- **`runcloud-pp-cli database_users delete`** - Delete a database user
- **`runcloud-pp-cli database_users get`** - Get a database user by ID
- **`runcloud-pp-cli database_users list`** - List database users
- **`runcloud-pp-cli database_users update-password`** - Change a database user's password

### databases

Manage server databases

- **`runcloud-pp-cli databases create`** - Create a database
- **`runcloud-pp-cli databases delete`** - Drop a database
- **`runcloud-pp-cli databases get`** - Get a database by ID
- **`runcloud-pp-cli databases grant`** - Grant a database user access to a database
- **`runcloud-pp-cli databases list`** - List databases on a server
- **`runcloud-pp-cli databases list-grants`** - List database users with access to a database
- **`runcloud-pp-cli databases revoke`** - Revoke a database user's access to a database

### domains

Manage domain attachments for a web application

- **`runcloud-pp-cli domains add`** - Attach a new domain to a web app
- **`runcloud-pp-cli domains delete`** - Remove a domain from a web app
- **`runcloud-pp-cli domains get`** - Get a domain by ID
- **`runcloud-pp-cli domains list`** - List domains for a web app

### external_keys

Manage workspace-level third-party API keys (Cloudflare, Linode, DigitalOcean) — v3 moves these from per-server to workspace scope

- **`runcloud-pp-cli external_keys create`** - Store a 3rd-party API key
- **`runcloud-pp-cli external_keys delete`** - Delete a stored 3rd-party API key
- **`runcloud-pp-cli external_keys get`** - Get a 3rd-party API key by ID
- **`runcloud-pp-cli external_keys list`** - List 3rd-party API keys in the workspace
- **`runcloud-pp-cli external_keys update`** - Update a 3rd-party API key

### fail2ban

Fail2Ban blocked-IP visibility and unblock

- **`runcloud-pp-cli fail2ban list`** - List Fail2Ban-blocked IP addresses
- **`runcloud-pp-cli fail2ban unblock`** - Unblock an IP address from Fail2Ban

### firewall

Manage server firewall rules

- **`runcloud-pp-cli firewall create`** - Create a firewall rule (stage; call deploy to apply)
- **`runcloud-pp-cli firewall delete`** - Remove a firewall rule (call deploy to apply)
- **`runcloud-pp-cli firewall deploy`** - Apply staged firewall rules to the live ruleset
- **`runcloud-pp-cli firewall get`** - Get a firewall rule by ID
- **`runcloud-pp-cli firewall list`** - List firewall rules

### git

Git integration for web applications

- **`runcloud-pp-cli git change-branch`** - Switch the active branch
- **`runcloud-pp-cli git clone`** - Connect a git repository to a web app
- **`runcloud-pp-cli git delete`** - Disconnect the git repository
- **`runcloud-pp-cli git deploy`** - Force a deploy via the configured deploy script
- **`runcloud-pp-cli git get`** - Get the current git connection
- **`runcloud-pp-cli git update-script`** - Update the deploy script

### installers

Install third-party scripts (WordPress, Joomla, Drupal, Magento)

- **`runcloud-pp-cli installers get`** - Get the currently installed script (if any)
- **`runcloud-pp-cli installers install`** - Install a script (e.g. WordPress) on a web app
- **`runcloud-pp-cli installers remove`** - Remove an installed script

### runcloudhub

RunCloudHub WordPress object-cache plugin (v3 new)

- **`runcloud-pp-cli runcloudhub`** - Install the RunCloudHub WordPress cache plugin

### s3_storage

Manage S3-compatible object storage integrations (v3)

- **`runcloud-pp-cli s3_storage`** - Update S3-compatible object storage integration

### server_logs

Retrieve server-level logs (nginx, Apache, MySQL)

- **`runcloud-pp-cli server_logs`** - Retrieve nginx/Apache/MySQL logs from the server

### servers

Manage RunCloud servers

- **`runcloud-pp-cli servers autoupdate`** - Configure automatic security/software updates
- **`runcloud-pp-cli servers change-php-cli`** - Change the server-wide PHP CLI version
- **`runcloud-pp-cli servers create`** - Register a new server with RunCloud (v3 supports nginx/ols web server and native/containerized install)
- **`runcloud-pp-cli servers delete`** - Remove a server from your RunCloud account
- **`runcloud-pp-cli servers disk-cleanup`** - Trigger server disk cleanup (purges logs and cache to reclaim space)
- **`runcloud-pp-cli servers get`** - Get a server by ID
- **`runcloud-pp-cli servers hardware`** - Get server hardware specifications (CPU, RAM, OS, kernel)
- **`runcloud-pp-cli servers health`** - Get latest server health snapshot (v3: uptime, load, disk, memory in a single payload)
- **`runcloud-pp-cli servers install-script`** - Retrieve the bash installation script to run on the target server
- **`runcloud-pp-cli servers list`** - List all servers in your account
- **`runcloud-pp-cli servers meta-update`** - Update server name and provider
- **`runcloud-pp-cli servers php-versions`** - List PHP versions available on this server
- **`runcloud-pp-cli servers shared`** - List servers shared with your account
- **`runcloud-pp-cli servers ssh-settings`** - Get SSH configuration for this server
- **`runcloud-pp-cli servers stats`** - Get server performance metrics (legacy v2 endpoint; prefer 'servers health' on v3)
- **`runcloud-pp-cli servers update-ssh-settings`** - Update SSH configuration

### services

Control system services (nginx, apache, mysql, redis, supervisor)

- **`runcloud-pp-cli services control`** - Start, stop, restart, or reload a service
- **`runcloud-pp-cli services list`** - List services and their status

### ssh_keys

Manage SSH public keys stored on system users

- **`runcloud-pp-cli ssh_keys add`** - Upload an SSH public key for a system user
- **`runcloud-pp-cli ssh_keys delete`** - Delete a stored SSH key
- **`runcloud-pp-cli ssh_keys list`** - List SSH keys for a system user

### ssl

Manage SSL certificates for a web application (basic mode)

- **`runcloud-pp-cli ssl delete`** - Delete the SSL certificate
- **`runcloud-pp-cli ssl get`** - Get the current SSL configuration
- **`runcloud-pp-cli ssl install`** - Install an SSL certificate (Let's Encrypt or custom)
- **`runcloud-pp-cli ssl redeploy`** - Force a Let's Encrypt redeploy of the SSL certificate
- **`runcloud-pp-cli ssl update`** - Update SSL configuration (HSTS, HTTP toggle, protocol)

### ssl_advanced

Advanced per-domain SSL mode (one cert per domain)

- **`runcloud-pp-cli ssl_advanced delete-domain`** - Delete a per-domain SSL certificate
- **`runcloud-pp-cli ssl_advanced get-domain`** - Get SSL details for a specific domain
- **`runcloud-pp-cli ssl_advanced install-domain`** - Install SSL for a specific domain (advanced mode only)
- **`runcloud-pp-cli ssl_advanced redeploy-domain`** - Force a redeploy of a per-domain SSL certificate
- **`runcloud-pp-cli ssl_advanced status`** - Check whether advanced per-domain SSL is enabled
- **`runcloud-pp-cli ssl_advanced switch`** - Switch between basic and advanced SSL modes

### static

Static reference data: timezones, collations, installers, SSL protocols (v3 paths)

- **`runcloud-pp-cli static collations`** - List supported database collations (v3 path)
- **`runcloud-pp-cli static installers`** - List installable scripts (WordPress, Joomla, Drupal, Magento) — v3 path
- **`runcloud-pp-cli static ssl-protocols`** - List supported SSL protocols (v3 path)
- **`runcloud-pp-cli static timezones`** - List supported timezones

### supervisor

Manage long-running supervisor processes

- **`runcloud-pp-cli supervisor control`** - Start, stop, or restart a supervisor job
- **`runcloud-pp-cli supervisor create`** - Create a supervisor-managed process
- **`runcloud-pp-cli supervisor delete`** - Delete a supervisor job
- **`runcloud-pp-cli supervisor list`** - List supervisor jobs

### system_users

Manage Linux system users that own web applications

- **`runcloud-pp-cli system_users create`** - Create a system user
- **`runcloud-pp-cli system_users delete`** - Delete a system user
- **`runcloud-pp-cli system_users get`** - Get a system user by ID
- **`runcloud-pp-cli system_users list`** - List system users on a server
- **`runcloud-pp-cli system_users update-password`** - Change a system user's password

### waf

Web Application Firewall management (v3 new)

- **`runcloud-pp-cli waf`** - Install WAF on a webapp

### webapps

Manage web applications across servers

- **`runcloud-pp-cli webapps alias`** - Create an alias web application sharing this one's document root
- **`runcloud-pp-cli webapps change-php`** - Change the PHP version for this web application
- **`runcloud-pp-cli webapps create`** - Create a custom web application
- **`runcloud-pp-cli webapps create-wordpress`** - Provision a WordPress webapp in one call (v3 native): creates the webapp + database + db user + grant atomically
- **`runcloud-pp-cli webapps default-set`** - Mark this web app as the server default
- **`runcloud-pp-cli webapps default-unset`** - Remove default status from this web app
- **`runcloud-pp-cli webapps delete`** - Delete a web application
- **`runcloud-pp-cli webapps get`** - Get a web application by ID
- **`runcloud-pp-cli webapps list`** - List web applications on a server
- **`runcloud-pp-cli webapps logs`** - View the activity log for this web app
- **`runcloud-pp-cli webapps rebuild`** - Rebuild the web application's nginx/PHP-FPM configuration
- **`runcloud-pp-cli webapps settings`** - Retrieve all settings for a web application
- **`runcloud-pp-cli webapps update-fpmnginx`** - Update PHP-FPM and nginx settings


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
runcloud-pp-cli agency_account

# JSON for scripting and agents
runcloud-pp-cli agency_account --json

# Filter to specific fields
runcloud-pp-cli agency_account --json --select id,name,status

# Dry run — show the request without sending
runcloud-pp-cli agency_account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
runcloud-pp-cli agency_account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
runcloud-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/runcloud-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RUNCLOUD_API_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `runcloud-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RUNCLOUD_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthenticated** — Bearer token missing or invalid. Generate a new one from Workspace > Settings > API Management and `export RUNCLOUD_API_TOKEN=...`. v2 key:secret pairs do not work in v3.
- **429 Too Many Requests** — Check `X-RateLimit-Remaining`. Reduce `--per-page`, batch with `--all` more slowly, or use the local store via `runcloud-pp-cli search`/`sql` instead of live API.
- **fleet command returns empty** — Run `runcloud-pp-cli sync --full` first — fleet queries read from the local store, not the live API.
- **Webapp not found by name** — Names resolve through FTS over the synced store. Either run `sync` or use the numeric webapp ID.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**RunCloudIO/runcloud-sdk-php**](https://github.com/RunCloudIO/runcloud-sdk-php) — PHP
- [**aleksanderem/runcloud-mcp**](https://github.com/aleksanderem/runcloud-mcp) — TypeScript
- [**RunCloud-cdk/shell-api-wrapper**](https://github.com/RunCloud-cdk/shell-api-wrapper) — Shell
- [**develanet/runcloud**](https://github.com/develanet/runcloud) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
