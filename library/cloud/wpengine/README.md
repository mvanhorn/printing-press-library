# WP Engine CLI

**Every WP Engine Hosting Platform API operation, plus a local fleet mirror that answers cross-account audit questions the portal cannot.**

Manage sites, installs, domains, SSL, backups, and cache across every WP Engine account from one terminal. Sync the whole fleet into local SQLite, then run audits no API call can answer: cert expiry ('audit certs'), backup staleness ('audit backups'), PHP version drift ('audit versions'), and domain health ('audit domains'). 'audit usage' projects month-end overages from live plan limits, and 'guard' turns backup-then-deploy-then-purge into one CI-safe command.

## Install

The recommended path installs both the `wpengine-pp-cli` binary and the `pp-wpengine` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wpengine
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wpengine --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wpengine --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wpengine --agent claude-code
npx -y @mvanhorn/printing-press-library install wpengine --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/wpengine/cmd/wpengine-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wpengine-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wpengine --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wpengine --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wpengine --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wpengine --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wpengine-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WP_ENGINE_API_USERNAME` and `WP_ENGINE_API_PASSWORD` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/wpengine/cmd/wpengine-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wpengine": {
      "command": "wpengine-pp-mcp",
      "env": {
        "WP_ENGINE_API_USERNAME": "<your-api-user-id>",
        "WP_ENGINE_API_PASSWORD": "<your-api-password>"
      }
    }
  }
}
```

</details>

## Authentication

WP Engine uses HTTP Basic auth with an API User ID and Password generated at my.wpengine.com/api_access (enable API access on your account first). Set WP_ENGINE_API_USERNAME and WP_ENGINE_API_PASSWORD in your environment — the same variables the community SDKs use. Rate limits are undocumented server-side; the client retries 429s with backoff automatically.

## Quick Start

```bash
# Verify the CLI is wired up and see which credentials it will look for
wpengine-pp-cli doctor --dry-run

# Build the local fleet mirror that powers every audit
wpengine-pp-cli sync --resources accounts,sites,installs

# Resolve a client name to its install without knowing UUIDs
wpengine-pp-cli search "client" --type installs

# Fleet-wide question the portal cannot answer: who is on old PHP
wpengine-pp-cli audit versions --php-below 8.2 --agent

# Preview the deploy gate: backup, wait for completion, purge CDN
wpengine-pp-cli guard my-install --purge cdn --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Fleet audits from local state
- **`audit certs`** — See every SSL certificate across your whole fleet sorted by days to expiry, before a client site goes insecure.

  _Reach for this when asked about certificate expiry anywhere in the fleet instead of walking installs one API call at a time._

  ```bash
  wpengine-pp-cli audit certs --expiring 30d --agent
  ```
- **`audit backups`** — Find production installs whose latest completed backup is older than a threshold you set.

  _Answers 'which prod installs are unprotected right now' in one call instead of N backup-list calls._

  ```bash
  wpengine-pp-cli audit backups --stale 7d --env production --agent
  ```
- **`audit versions`** — Fleet-wide PHP and WordPress version distribution with outlier filters, plus per-site environment drift detection.

  _Use it when asked which installs run outdated PHP/WordPress or whether staging matches production._

  ```bash
  wpengine-pp-cli audit versions --php-below 8.2 --agent
  ```
- **`audit domains`** — One list of everything blocked across launches: unverified domains, domains without cert coverage, dangling redirects.

  _Use it to find every domain still blocking a launch instead of re-checking installs one by one._

  ```bash
  wpengine-pp-cli audit domains --agent
  ```
- **`whois`** — Resolve any domain name to the install, site, and account that serve it, plus cert and redirect status.

  _Use it first when a support ticket names a domain and you need to know which install and account own it._

  ```bash
  wpengine-pp-cli whois clientsite.com --agent
  ```

### Deploy plumbing
- **`guard`** — Gate a deploy: create a checkpoint backup, wait until it completes, then optionally purge cache, with CI-friendly exit codes.

  _Use it before any deploy that needs a verified restore point; it blocks until the backup actually completes._

  ```bash
  wpengine-pp-cli guard my-install --purge cdn
  ```

### Live API projections
- **`audit usage`** — Project month-end billable visits and storage against account limits and flag accounts trending over.

  _Use it to warn about overages before they land on an invoice, not after._

  ```bash
  wpengine-pp-cli audit usage --horizon 30d --agent
  ```

## Recipes

### Monday fleet audit

```bash
wpengine-pp-cli sync --full && wpengine-pp-cli audit certs --expiring 30d && wpengine-pp-cli audit backups --stale 7d --env production
```

Refresh the mirror, then surface expiring certs and unprotected production installs in one pass.

### Gate a deploy on a verified backup

```bash
wpengine-pp-cli guard my-install --purge cdn
```

Creates a checkpoint backup, blocks until it reaches completed, then purges the CDN cache — exit code tells CI whether to proceed.

### Support-ticket triage by domain

```bash
wpengine-pp-cli whois clientsite.com --agent --select install,account,environment,cert_status
```

One local join resolves a domain to the install, account, environment, and cert status that serve it.

### Who is trending over their plan

```bash
wpengine-pp-cli audit usage --horizon 30d --agent
```

Projects month-end billable visits and bandwidth against plan limits from live month-to-date usage — no sync needed.

### Fleet PHP distribution in one query

```bash
wpengine-pp-cli analytics --type installs --group-by php_version
```

The whole fleet is local SQLite — group any synced resource by any field without an API call.

## Usage

Run `wpengine-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WPENGINE_CONFIG_DIR`, `WPENGINE_DATA_DIR`, `WPENGINE_STATE_DIR`, or `WPENGINE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WPENGINE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WPENGINE_HOME=/srv/wpengine
wpengine-pp-cli doctor
```

Under `WPENGINE_HOME=/srv/wpengine`, the four dirs resolve to `/srv/wpengine/config`, `/srv/wpengine/data`, `/srv/wpengine/state`, and `/srv/wpengine/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "wpengine": {
      "command": "wpengine-pp-mcp",
      "env": {
        "WPENGINE_HOME": "/srv/wpengine"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WPENGINE_DATA_DIR` overrides an explicit `--home` for that kind. Use `WPENGINE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WPENGINE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `wpengine-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Manage accounts

- **`wpengine-pp-cli accounts get`** - Returns a single Account
- **`wpengine-pp-cli accounts list`** - # Description
Use this to list your WP Engine accounts.

### install-copy

Manage install copy

- **`wpengine-pp-cli install-copy`** - Copy the full file system and database from one WordPress installation to another

### installs

Manage installs

- **`wpengine-pp-cli installs create`** - Create a new WordPress installation
- **`wpengine-pp-cli installs delete`** - This will delete the install, The delete is permanent and there is no confirmation prompt.
- **`wpengine-pp-cli installs get`** - Returns a single Install
- **`wpengine-pp-cli installs list`** - List your WordPress installations
- **`wpengine-pp-cli installs update`** - Update a WordPress installation

### site-reports

Manage site reports

- **`wpengine-pp-cli site-reports create-schedule`** - # Description
Creates a scheduled report for a site.

Reports can be scheduled to run monthly, weekly or every two weeks.
To run a report on a monthly basis, set the Unit to MONTHLY and set the Frequency Value to the day of the month you want the report to be run every month.  For example, Unit = MONTHLY and Frequency Value = 15 will run the report on the 15th of every month.  Note:  if you set a report to run on the 29th, 30th or 31st of the month and that date doesn't occur in the given month, the report will run on the last day of the month.  For example, if you designate the 31st of the month, the report will run on the last day of February.
You can run reports on a weekly or every two week basis by setting the Unit to DAYS and then setting the Frequency Value to 7 for weekly reports or 14 for reports to run every two weeks.  The day of the week is determined by the next_scheduled_date value.  For example, if the next_scheduled_date value is a Tuesday and the Frequency Value is 7, then the report will run each week on Tuesday.
- **`wpengine-pp-cli site-reports delete-schedule`** - # Description
Deletes a report schedule by its ID.
- **`wpengine-pp-cli site-reports get`** - Returns a list of generated site reports for a particular site.
- **`wpengine-pp-cli site-reports get-schedules`** - # Description
Reports can be scheduled to run monthly, weekly, or every two weeks. This endpoint returns all active report schedules for a given site, allowing you to manage and monitor when reports are generated automatically.

For monthly scheduling, set the Unit to MONTHLY and set the Frequency Value to the day of the month (1-31) when you want the report to run. For example, Unit = MONTHLY and Frequency Value = 15 will run the report on the 15th of every month. If you set a report to run on the 29th, 30th, or 31st and that date doesn't exist in a given month, the report will run on the last day of that month. For instance, designating the 31st will result in the report running on February 28th (or 29th in leap years).

For weekly or bi-weekly scheduling, set the Unit to DAYS and then set the Frequency Value to 7 for weekly reports or 14 for reports to run every two weeks. The day of the week is determined by the next_scheduled_date value. For example, if the next_scheduled_date is a Tuesday and the Frequency Value is 7, then the report will run each week on Tuesday.
- **`wpengine-pp-cli site-reports get-sections`** - Returns the available sections for a site report
- **`wpengine-pp-cli site-reports list-templates`** - # Description
Returns a list of report templates available for Site Reports.

Templates define the structure, branding, and sections included in generated reports.
- **`wpengine-pp-cli site-reports update-schedule`** - # Description
Updates an existing report schedule.

Reports can be scheduled to run monthly, weekly or every two weeks.

To run a report on a monthly basis, set the Unit to MONTHLY and set the Frequency Value to the day of the month you want the report to be run every month.  For example, Unit = MONTHLY and Frequency Value = 15 will run the report on the 15th of every month.  Note:  if you set a report to run on the 29th, 30th or 31st of the month and that date doesn't occur in the given month, the report will run on the last day of the month.  For example, if you designate the 31st of the month, the report will run on the last day of February.

You can run reports on a weekly or every two week basis by setting the Unit to DAYS and then setting the Frequency Value to 7 for weekly reports or 14 for reports to run every two weeks.  The day of the week is determined by the next_scheduled_date value.  For example, if the next_scheduled_date value is a Tuesday and the Frequency Value is 7, then the report will run each week on Tuesday.

### sites

Manage sites

- **`wpengine-pp-cli sites create`** - Create a new site
- **`wpengine-pp-cli sites delete`** - This will delete the site and any installs associated with this site. This delete is permanent and there is no confirmation prompt.
- **`wpengine-pp-cli sites get`** - Returns a single site
- **`wpengine-pp-cli sites list`** - List your sites
- **`wpengine-pp-cli sites update`** - Change a site name

### ssh-keys

Manage ssh keys

- **`wpengine-pp-cli ssh-keys create`** - # Description
Use this to add a new SSH key to WP Engine.
- **`wpengine-pp-cli ssh-keys delete`** - # Description
This will delete the SSH key.
- **`wpengine-pp-cli ssh-keys list`** - # Description
Use this to list the SSH keys that you've added to WP Engine.

### status

Manage status

- **`wpengine-pp-cli status`** - # Description
This endpoint will report the system status and any outages that might be occurring.

### swagger

Manage swagger

- **`wpengine-pp-cli swagger`** - # Description
This will output the current swagger specification

### user

Manage user

- **`wpengine-pp-cli user`** - Returns the currently authenticated user


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`wpengine-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`wpengine-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`wpengine-pp-cli learnings list`** - Inspect taught rows
- **`wpengine-pp-cli learnings forget <query>`** - Undo a teach
- **`wpengine-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`wpengine-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`wpengine-pp-cli teach-pattern`** - Install a query/resource template up front
- **`wpengine-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WPENGINE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `wpengine-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wpengine-pp-cli accounts list

# JSON for scripting and agents
wpengine-pp-cli accounts list --json

# Filter to specific fields
wpengine-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
wpengine-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wpengine-pp-cli accounts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wpengine-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `wpengine-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/wpengine-pp-cli/config.toml`; `--home`, `WPENGINE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WP_ENGINE_API_USERNAME` | per_call | Yes |  |
| `WP_ENGINE_API_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wpengine-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wpengine-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WP_ENGINE_API_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Generate API credentials at my.wpengine.com/api_access and export WP_ENGINE_API_USERNAME and WP_ENGINE_API_PASSWORD; run 'wpengine-pp-cli doctor' to confirm
- **Local-mirror audits (certs, backups, versions, domains) return empty results** — Those audits read the local mirror; run 'wpengine-pp-cli sync --full' first and re-run ('audit usage' is live and needs no sync)
- **429 Too Many Requests during bulk operations** — WP Engine does not publish rate limits; the client already retries with backoff — space out cron-driven bulk syncs rather than tightening the loop
- **404 Not Found when passing an install name** — Endpoint commands take UUIDs; resolve names first with 'wpengine-pp-cli search "name" --type installs'

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wpe-cli**](https://github.com/ryanshoover/wpe-cli) — PHP (27 stars)
- [**wp-engine-api-python**](https://github.com/jpollock/wp-engine-api-python) — Python (1 stars)
- [**wpengine-cli**](https://github.com/thesandybridge/wpengine-cli) — Rust
- [**WP-Engine-Toolkit**](https://github.com/timstl/WP-Engine-Toolkit) — PHP
- [**wp-engine-api-php**](https://github.com/jpollock/wp-engine-api-php) — PHP

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
