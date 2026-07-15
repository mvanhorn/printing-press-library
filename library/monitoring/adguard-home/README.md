# AdGuard Home CLI

A command-line interface for [AdGuard Home](https://github.com/AdguardTeam/AdGuardHome) — the network-wide ad and tracker blocker that runs as a self-hosted DNS sinkhole. This CLI wraps the full AdGuard Home REST API so you can manage your DNS filtering, parental controls, safe browsing, DHCP server, TLS certificates, filter lists, DNS rewrites, client rules, and query logs from a terminal or AI agent.

**Key capabilities:**

- **DNS filtering** — manage blocklists, custom filter rules, per-client access controls, and blocked services (YouTube, Facebook, TikTok, etc.)
- **Local sync & offline search** — `sync` pulls all data into a local SQLite database for instant full-text search, analytics, and health reporting without hitting the API
- **DNS health dashboard** — `health` summarizes protection status, block rate, top blocked domains, stale data detection, and query volume trends
- **Parental & safe browsing controls** — enable/disable safe search, safe browsing (malware/phishing), and parental filtering from the command line
- **DHCP management** — configure the built-in DHCP server, manage static leases, check for conflicts
- **TLS & encryption** — configure DoH/DoT/DoQ, validate TLS certificates, generate Apple mobile config profiles
- **Real-time monitoring** — `tail` streams live DNS query changes; `analytics` runs aggregate queries on synced data
- **Agent-ready** — `--agent` flag sets JSON output, compact mode, no-color, and no-prompts in one flag for AI agent workflows

Created by [@e-jung](https://github.com/e-jung) (Eric Jung).

## Install

The recommended path installs both the `adguard-home-pp-cli` binary and the `pp-adguard-home` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install adguard-home
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install adguard-home --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install adguard-home --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install adguard-home --agent claude-code
npx -y @mvanhorn/printing-press-library install adguard-home --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/cmd/adguard-home-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/adguard-home-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install adguard-home --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-adguard-home --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-adguard-home --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install adguard-home --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/adguard-home-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ADGUARD_HOME_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/cmd/adguard-home-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "adguard-home": {
      "command": "adguard-home-pp-mcp",
      "env": {
        "ADGUARD_HOME_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export ADGUARD_HOME_USERNAME="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/adguard-home-pp-cli/config.toml`.

### 3. Verify Setup

```bash
adguard-home-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
adguard-home-pp-cli access list
```

## Usage

Run `adguard-home-pp-cli --help` for the full command reference and flag list.

## Commands

### access

Manage access

- **`adguard-home-pp-cli access list`** - List (dis)allowed clients, blocked hosts, etc.
- **`adguard-home-pp-cli access set`** - Set (dis)allowed clients, blocked hosts, etc.

### adguard-home-profile

Manage adguard home profile

- **`adguard-home-pp-cli adguard-home-profile get`** - Get
- **`adguard-home-pp-cli adguard-home-profile update`** - Updates current user info

### apple

Manage apple

- **`adguard-home-pp-cli apple mobile-config-do-h`** - Get DNS over HTTPS .mobileconfig.
- **`adguard-home-pp-cli apple mobile-config-do-t`** - Get DNS over TLS .mobileconfig.

### blocked-services

Blocked services controls

- **`adguard-home-pp-cli blocked-services all`** - Get available services to use for blocking
- **`adguard-home-pp-cli blocked-services available-services`** - Deprecated: Use `GET /blocked_services/all` instead.
- **`adguard-home-pp-cli blocked-services list`** - Deprecated: Use `GET /blocked_services/get` instead.
- **`adguard-home-pp-cli blocked-services schedule`** - Get blocked services
- **`adguard-home-pp-cli blocked-services schedule-update`** - Update blocked services
- **`adguard-home-pp-cli blocked-services set`** - Deprecated: Use `PUT /blocked_services/update` instead.

### cache-clear

Manage cache clear

- **`adguard-home-pp-cli cache-clear`** - Clear DNS cache

### clients

Clients list operations

- **`adguard-home-pp-cli clients add`** - Add a new client
- **`adguard-home-pp-cli clients delete`** - Remove a client
- **`adguard-home-pp-cli clients find`** - Get information about clients by their IP addresses or ClientIDs.
- **`adguard-home-pp-cli clients search`** - Retrieve information about clients by performing an exact match search using IP addresses, CIDRs, MAC addresses, or ClientIDs.
- **`adguard-home-pp-cli clients status`** - Get information about configured clients
- **`adguard-home-pp-cli clients update`** - Update client information

### dhcp

Built-in DHCP server controls

- **`adguard-home-pp-cli dhcp add-static-lease`** - Adds a static lease
- **`adguard-home-pp-cli dhcp check-active`** - Searches for an active DHCP server on the network
- **`adguard-home-pp-cli dhcp interfaces`** - Gets the available interfaces
- **`adguard-home-pp-cli dhcp remove-static-lease`** - Removes a static lease
- **`adguard-home-pp-cli dhcp reset`** - Reset DHCP configuration
- **`adguard-home-pp-cli dhcp reset-leases`** - Reset DHCP leases
- **`adguard-home-pp-cli dhcp set-config`** - Updates the current DHCP server configuration
- **`adguard-home-pp-cli dhcp status`** - Gets the current DHCP settings and status
- **`adguard-home-pp-cli dhcp update-static-lease`** - Updates IP address, hostname of the static lease.  IP version must be the same as previous.

### dns-config

Manage dns config

- **`adguard-home-pp-cli dns-config`** - Set general DNS parameters

### dns-info

Manage dns info

- **`adguard-home-pp-cli dns-info`** - Get general DNS parameters

### filtering

Rule-based filtering

- **`adguard-home-pp-cli filtering add-url`** - Add filter URL or an absolute file path
- **`adguard-home-pp-cli filtering check-host`** - Check if host name is filtered
- **`adguard-home-pp-cli filtering config`** - Set filtering parameters
- **`adguard-home-pp-cli filtering refresh`** - Reload filtering rules from URLs.  This might be needed if new URL was just added and you don't want to wait for automatic refresh to kick in. This API request is ratelimited, so you can call it freely as often as you like, it wont create unnecessary burden on servers that host the URL.  This should work as intended, a `force` parameter is offered as last-resort attempt to make filter lists fresh.  If you ever find yourself using `force` to make something work that otherwise wont, this is a bug and report it accordingly.
- **`adguard-home-pp-cli filtering remove-url`** - Remove filter URL
- **`adguard-home-pp-cli filtering set-rules`** - Set user-defined filter rules
- **`adguard-home-pp-cli filtering set-url`** - Set URL parameters
- **`adguard-home-pp-cli filtering status`** - Get filtering parameters

### i18n

Application localization

- **`adguard-home-pp-cli i18n change-language`** - Change current language.  Argument must be an ISO 639-1 two-letter code.
- **`adguard-home-pp-cli i18n current-language`** - Get currently set language.  Result is ISO 639-1 two-letter code.  Empty result means default language.

### install

First-time install configuration handlers

- **`adguard-home-pp-cli install check-config`** - Checks configuration
- **`adguard-home-pp-cli install configure`** - Applies the initial configuration.
- **`adguard-home-pp-cli install get-addresses`** - Gets the network interfaces information.

### login

Manage login

- **`adguard-home-pp-cli login`** - Perform administrator log-in

### logout

Manage logout

- **`adguard-home-pp-cli logout`** - Perform administrator log-out

### parental

Blocking adult and explicit materials

- **`adguard-home-pp-cli parental disable`** - Disable parental filtering
- **`adguard-home-pp-cli parental enable`** - Enable parental filtering
- **`adguard-home-pp-cli parental status`** - Get parental filtering status

### protection

Manage protection

- **`adguard-home-pp-cli protection`** - Set protection state and duration

### querylog

Manage querylog

- **`adguard-home-pp-cli querylog get-query-log-config`** - Get query log parameters
- **`adguard-home-pp-cli querylog put-query-log-config`** - Set query log parameters
- **`adguard-home-pp-cli querylog query-log`** - Get DNS server query log.

### querylog-clear

Manage querylog clear

- **`adguard-home-pp-cli querylog-clear`** - Clear query log

### querylog-config

Manage querylog config

- **`adguard-home-pp-cli querylog-config`** - Deprecated: Use `PUT /querylog/config/update` instead.

### querylog-info

Manage querylog info

- **`adguard-home-pp-cli querylog-info`** - Deprecated: Use `GET /querylog/config` instead.

NOTE: If `interval` was configured by editing configuration file or new
HTTP API call `PUT /querylog/config/update` and it's not equal to
previous allowed enum values then it will be equal to `90` days for
compatibility reasons.

### rewrite

DNS rewrites

- **`adguard-home-pp-cli rewrite add`** - Add a new Rewrite rule
- **`adguard-home-pp-cli rewrite delete`** - Remove a Rewrite rule
- **`adguard-home-pp-cli rewrite list`** - Get list of Rewrite rules
- **`adguard-home-pp-cli rewrite settings-get`** - Get rewrite settings
- **`adguard-home-pp-cli rewrite settings-update`** - Update rewrite settings
- **`adguard-home-pp-cli rewrite update`** - Update a Rewrite rule

### safebrowsing

Blocking malware/phishing sites

- **`adguard-home-pp-cli safebrowsing disable`** - Disable safebrowsing
- **`adguard-home-pp-cli safebrowsing enable`** - Enable safebrowsing
- **`adguard-home-pp-cli safebrowsing status`** - Get safebrowsing status

### safesearch

Enforce family-friendly results in search engines

- **`adguard-home-pp-cli safesearch disable`** - Disable safesearch
- **`adguard-home-pp-cli safesearch enable`** - Enable safesearch
- **`adguard-home-pp-cli safesearch settings`** - Update safesearch settings
- **`adguard-home-pp-cli safesearch status`** - Get safesearch status

### stats

AdGuard Home statistics

- **`adguard-home-pp-cli stats get-config`** - Get statistics parameters
- **`adguard-home-pp-cli stats put-config`** - Set statistics parameters
- **`adguard-home-pp-cli stats stats`** - Get DNS server statistics

### stats-config

Manage stats config

- **`adguard-home-pp-cli stats-config`** - Deprecated: Use `PUT /stats/config/update` instead.

### stats-info

Manage stats info

- **`adguard-home-pp-cli stats-info`** - Deprecated: Use `GET /stats/config` instead.

NOTE: If `interval` was configured by editing configuration file or new
HTTP API call `PUT /stats/config/update` and it's not equal to
previous allowed enum values then it will be equal to `90` days for
compatibility reasons.

### stats-reset

Manage stats reset

- **`adguard-home-pp-cli stats-reset`** - Reset all statistics to zeroes

### status

Manage status

- **`adguard-home-pp-cli status`** - Get DNS server current status and general settings

### test-upstream-dns

Manage test upstream dns

- **`adguard-home-pp-cli test-upstream-dns`** - Test upstream configuration

### tls

AdGuard Home HTTPS/DoH/DoQ/DoT settings

- **`adguard-home-pp-cli tls configure`** - Updates current TLS configuration
- **`adguard-home-pp-cli tls status`** - Returns TLS configuration and its status
- **`adguard-home-pp-cli tls validate`** - Checks if the current TLS configuration is valid

### update

Manage update

- **`adguard-home-pp-cli update`** - Begin auto-upgrade procedure

### version-json

Manage version json

- **`adguard-home-pp-cli version-json`** - Gets information about the latest available version of AdGuard

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
adguard-home-pp-cli access list

# JSON for scripting and agents
adguard-home-pp-cli access list --json

# Filter to specific fields
adguard-home-pp-cli access list --json --select id,name,status

# Dry run — show the request without sending
adguard-home-pp-cli access list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
adguard-home-pp-cli access list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
adguard-home-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/adguard-home-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ADGUARD_HOME_USERNAME` | per_call | Yes |  |
| `ADGUARD_HOME_PASSWORD` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `adguard-home-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ADGUARD_HOME_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
