# Servosity Msp CLI

Servosity REST API surface available to authenticated MSP partners. All operations are scoped to the authenticated reseller. Admin-only endpoints (cross-reseller listing, billing back-office, support tooling) are not included.

Printed by [@dstevens](https://github.com/dstevens) (Damien Stevens).

## Install

The recommended path installs both the `servosity-msp-cli` binary and the `pp-servosity-msp` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install servosity-msp
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install servosity-msp --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install servosity-msp --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install servosity-msp --agent claude-code
npx -y @mvanhorn/printing-press install servosity-msp --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servosity-msp-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-servosity-msp --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-servosity-msp --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-servosity-msp skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-servosity-msp. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servosity-msp-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SERVOSITY_MSP_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "servosity-msp": {
      "command": "servosity-msp-mcp",
      "env": {
        "SERVOSITY_MSP_TOKEN": "<your-key>"
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
export SERVOSITY_MSP_TOKEN="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/servosity-partner-msp-pp-cli/config.toml`.

### 3. Verify Setup

```bash
servosity-msp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
servosity-msp-cli agent-login list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Fleet-wide intelligence
- **`attention`** — One screen across your whole book of clients. Merges open issues, stale backups, and in-flight DR events into a per-company ranked view, then persists the result so tomorrow's drift command can compare.

  _Reach for this in the morning to triage what needs follow-up across every client without clicking through a portal._

  ```bash
  servosity-msp-pp-cli attention --top 10 --json
  ```
- **`drift`** — Diff two snapshots the CLI collected — show which companies got worse, which recovered, and which are new since a past anchor. Default compares yesterday-to-now on the attention metric.

  _Use Monday morning to start with situation awareness instead of treating every week as a fresh slate._

  ```bash
  servosity-msp-pp-cli drift --metric attention --from yesterday --to now --json
  ```
- **`stale-backups`** — Slice the stale-backup-sets report by company, age window, and backup engine — entirely offline once cached. Use --refresh to repull from the API.

  _Run this Friday afternoon to compile the list of clients you need to email about a stalled backup._

  ```bash
  servosity-msp-pp-cli stale-backups --days 7 --engine restic --json
  ```
- **`backup-facts`** — Unified view across Servosity's three backup engines (classic, restic, DR) for one company or all. Engine, ID, hostname, last_successful_at, status, size — joined from three local store tables into one table.

  _Reach for this when triaging a client who has multiple engines protecting different devices and you need to know which engine is failing where._

  ```bash
  servosity-msp-pp-cli backup-facts --company 4421 --status fail --json
  ```

### Client-facing reporting
- **`qbr`** — Generate the backup section of a client's Quarterly Business Review as Markdown, HTML, or PDF. Job success rate, restore tests run this quarter, coverage map across all three engines, open issues, storage trend.

  _Use this 1-2 weeks before a client QBR. Saves 30-60 min of manual deck-building per client._

  ```bash
  servosity-msp-pp-cli qbr 4421 --quarter 2026-Q1 --format pdf --out acme-q1.pdf
  ```

### Daily ops efficiency
- **`triage`** — List open issues with filters, then batch-mutate them (ignore / archive / reactivate / comment) in one invocation with --dry-run support and typed exit codes.

  _Use when the issue queue is bursty or during a planned-outage window where many alerts cluster around one client._

  ```bash
  servosity-msp-pp-cli triage --company 4421 --ignore 18,22,29 --comment 'scheduled outage' --dry-run
  ```

### Disaster recovery
- **`restore-queue watch`** — Watch every active company's restore queue across the book during a DR event. Polls each company periodically and prints diffs since the last tick.

  _Use during an active disaster recovery event when multiple clients have restores in flight._

  ```bash
  servosity-msp-pp-cli restore-queue watch --interval 30s --json
  ```

### Business operations
- **`bill --reconcile`** — Pull the MSP's monthly Servosity bill and compare line-by-line against a CSV of what the MSP is invoicing their clients. Surfaces drift — clients under- or over-charged.

  _Run this every month-end before invoicing clients. Catches missed line items and pricing mismatches._

  ```bash
  servosity-msp-pp-cli bill --reconcile invoiced-2026-05.csv --month 2026-05 --json
  ```
- **`unprovisioned`** — List agents installed on client machines but not yet pulling backups, ranked by client. Surfaces lost revenue from incomplete onboardings.

  _Run weekly to catch agents installed during onboarding that never successfully phoned home._

  ```bash
  servosity-msp-pp-cli unprovisioned --age 24h --json
  ```
- **`storage-trend`** — Linear-regression forecast of when a specific client will hit a capacity threshold. Reads the historical storage_bytes time series from local snapshots; with --snapshot, persists a new measurement for future runs.

  _Run quarterly per high-storage client to identify upsell opportunities before they hit a hard limit._

  ```bash
  servosity-msp-pp-cli storage-trend 4421 --weeks 12 --threshold 1TB --json
  ```

## Usage

Run `servosity-msp-cli --help` for the full command reference and flag list.

## Commands

### agent-login

Manage agent login

- **`servosity-msp-cli agent-login create`** - Create
- **`servosity-msp-cli agent-login list`** - List

### agent-sessions

Manage agent sessions

- **`servosity-msp-cli agent-sessions <agent_session_id>`** - Read

### backup-job-report

Manage backup job report

- **`servosity-msp-cli backup-job-report <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>`** - View detailed backup report for a backup job and destination.

### backup-job-report-summary

Manage backup job report summary

- **`servosity-msp-cli backup-job-report-summary <backup_destination_id> <backup_id> <backup_job_id> <backup_set_id>`** - View summary backup report for a backup job and destination.

### backup-job-status

Manage backup job status

- **`servosity-msp-cli backup-job-status <backup_id>`** - List backup job status for a backup account on a specific date.

### backup-jobs

Manage backup jobs

- **`servosity-msp-cli backup-jobs <backup_id>`** - List backup jobs for a backup account.

### backup-plans

Manage backup plans

- **`servosity-msp-cli backup-plans list`** - List backup plans.
- **`servosity-msp-cli backup-plans read`** - View a backup plan.

### backup-search

Manage backup search

- **`servosity-msp-cli backup-search`** - List

### backup-sets

Manage backup sets

- **`servosity-msp-cli backup-sets create`** - Create a backup-set for a backup account.
- **`servosity-msp-cli backup-sets delete`** - Delete a backup-set for a backup account.
- **`servosity-msp-cli backup-sets list`** - List backup-sets for a backup account.
- **`servosity-msp-cli backup-sets read`** - View a backup-set for a backup account.
- **`servosity-msp-cli backup-sets update`** - Accepts a json body with the following optional parameters.

`ReadOnly`: Boolean

`Name`: String
Backup set name

`ShadowCopyEnabled`: Boolean
Enable Windows' Volume Shadow Copy for open file backup

`DeleteTempFile`: Boolean
Remove temporary files after backup

`LogRetentionDays`: Integer
Number of days to keep the backup set log

`FollowLink`: Boolean
Follow link of the backup files

`CompressType`: String
The value can be one of the following: "GzipBestSpeedCompression" (Fast), "GzipDefaultCompression" (Normal)

`LanDomain`: String
Windows User Authentication domain/host name

`LanUsername`: String
Windows User Authentication user name

`LanPassword`: String
Windows User Authentication user password

`WorkingDir`: String
Temporary Driectory for storing backup files

`UploadPermission`: Boolean
Enable to backup permission attribute of files

`ReminderSettings`

`InFileDeltaSettings`

`LocalCopySettings`

`RetentionPolicySettings`

`CdpSettingsV6`

`CdpSettingsV7`

`BandwidthControlSettings`

`FilterSettings`

`ScheduleSettings`

`DestinationSettings`

`SelectedSourceList`

`DeselectedSourceList`

`PreCommandList`

`PostCommandList`

`AllowedIPList`

`ApplicationSettings`

`DestinationList`

`EnableOpenDirect`: Boolean
Note: Cannot be changed once set

### backups

Manage backups

- **`servosity-msp-cli backups create`** - Create a backup account.
- **`servosity-msp-cli backups delete`** - Delete a backup account, also deleting all backup data.
- **`servosity-msp-cli backups list`** - List backup accounts.
- **`servosity-msp-cli backups mfa-codes`** - Mfa codes
- **`servosity-msp-cli backups partial-update`** - Partial update
- **`servosity-msp-cli backups read`** - View a backup account.
- **`servosity-msp-cli backups update`** - Update a backup account.

### companies

Manage companies

- **`servosity-msp-cli companies create`** - Create a company.
- **`servosity-msp-cli companies delete`** - Delete a company, also deleting all backup accounts and backup data.
- **`servosity-msp-cli companies fully-managed`** - List fully-managed companies.
- **`servosity-msp-cli companies fully-managed-ng`** - List fully-managed companies.
- **`servosity-msp-cli companies list`** - List companies.
- **`servosity-msp-cli companies partial-update`** - Partial update
- **`servosity-msp-cli companies read`** - View a company.
- **`servosity-msp-cli companies summary`** - List companies with account summaries.
- **`servosity-msp-cli companies summary-ng`** - Summary ng
- **`servosity-msp-cli companies update`** - Update a company.

### company-notes

Manage company notes

- **`servosity-msp-cli company-notes create`** - Create
- **`servosity-msp-cli company-notes delete`** - Delete
- **`servosity-msp-cli company-notes list`** - List
- **`servosity-msp-cli company-notes partial-update`** - Partial update
- **`servosity-msp-cli company-notes read`** - Read
- **`servosity-msp-cli company-notes update`** - Update

### components

Manage components

- **`servosity-msp-cli components`** - List

### contracts

Manage contracts

- **`servosity-msp-cli contracts create`** - Create
- **`servosity-msp-cli contracts get-by-token`** - Get by token
- **`servosity-msp-cli contracts list`** - List
- **`servosity-msp-cli contracts partial-update`** - Partial update
- **`servosity-msp-cli contracts read`** - Read
- **`servosity-msp-cli contracts signatures`** - Signatures
- **`servosity-msp-cli contracts update`** - Update

### credentials

Manage credentials

- **`servosity-msp-cli credentials create`** - Create
- **`servosity-msp-cli credentials delete`** - Delete
- **`servosity-msp-cli credentials list`** - List
- **`servosity-msp-cli credentials partial-update`** - Partial update
- **`servosity-msp-cli credentials read`** - Read
- **`servosity-msp-cli credentials update`** - Update

### current-user

Manage current user

- **`servosity-msp-cli current-user api-token-delete`** - Delete the current user's API token. A new one will be generated when requested.
- **`servosity-msp-cli current-user api-token-list`** - You will receive JSON response with `token`.

To make API calls with the token, add an `Authorization` header to your request in this form:

`Authorization: Token XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`
- **`servosity-msp-cli current-user create`** - Change the password of the current logged in user.
- **`servosity-msp-cli current-user groups-list`** - Groups list
- **`servosity-msp-cli current-user helpjuice-sso-create`** - Helpjuice sso create
- **`servosity-msp-cli current-user hubspot-sso-create`** - Hubspot sso create
- **`servosity-msp-cli current-user list`** - Get information about the current logged in user.
- **`servosity-msp-cli current-user mfa-backup-codes-list`** - Get unused backup codes.
If no unused codes are left, remove all and generate new codes.
- **`servosity-msp-cli current-user mfa-backup-codes-update`** - Remove all backup codes and generate new codes.
- **`servosity-msp-cli current-user notifications-delete`** - Notifications delete
- **`servosity-msp-cli current-user notifications-list`** - Get current user notifications
- **`servosity-msp-cli current-user profile-create`** - Profile create
- **`servosity-msp-cli current-user profile-list`** - Profile list
- **`servosity-msp-cli current-user start-mfa-create`** - Start mfa create
- **`servosity-msp-cli current-user start-mfa-list`** - Start mfa list
- **`servosity-msp-cli current-user start-mfa-verify-create`** - Start mfa verify create
- **`servosity-msp-cli current-user verified-mfa-delete`** - Verified mfa delete
- **`servosity-msp-cli current-user verified-mfa-list`** - Verified mfa list
- **`servosity-msp-cli current-user verified-mfa-send-code-create`** - Verified mfa send code create

### download

Manage download

- **`servosity-msp-cli download`** - Servosity one windows list

### dr-backups

Manage dr backups

- **`servosity-msp-cli dr-backups create`** - Create a DR backup account.
- **`servosity-msp-cli dr-backups delete`** - Delete a DR backup account.
- **`servosity-msp-cli dr-backups list`** - List
- **`servosity-msp-cli dr-backups partial-update`** - Update a DR backup account.
- **`servosity-msp-cli dr-backups read`** - Read
- **`servosity-msp-cli dr-backups update`** - Update a DR backup account.

### issue-comments

Manage issue comments

- **`servosity-msp-cli issue-comments delete`** - Delete
- **`servosity-msp-cli issue-comments update`** - Update

### issues

Manage issues

- **`servosity-msp-cli issues archived`** - Archived
- **`servosity-msp-cli issues ignored`** - Ignored
- **`servosity-msp-cli issues list`** - List
- **`servosity-msp-cli issues read`** - Read

### report-subscriptions

Manage report subscriptions

- **`servosity-msp-cli report-subscriptions read`** - Read
- **`servosity-msp-cli report-subscriptions unsubscribe`** - Unsubscribe
- **`servosity-msp-cli report-subscriptions verify`** - Verify

### reports

Manage reports

- **`servosity-msp-cli reports account-list`** - Get a report of backup account types for each company and reseller in CSV format.
- **`servosity-msp-cli reports classic-usage-list`** - Get a usage report for all backup accounts in CSV format.
- **`servosity-msp-cli reports clients-list`** - Get a report of backup account client versions.
- **`servosity-msp-cli reports dr-from-email-list`** - Get a report of user profiles.
- **`servosity-msp-cli reports maxio-price-points-list`** - Get CSV with all Maxio price points.
- **`servosity-msp-cli reports product-list`** - Product list
- **`servosity-msp-cli reports stale-backup-sets-list`** - Get a report of all backup set last backup complete times.
- **`servosity-msp-cli reports usage-list`** - Usage list
- **`servosity-msp-cli reports user-profiles-list`** - Get a report of user profiles.

### resellers

Manage resellers

- **`servosity-msp-cli resellers partial-update`** - Partial update
- **`servosity-msp-cli resellers read`** - View a reseller.
- **`servosity-msp-cli resellers update`** - Update a reseller.

### restic-backups

Manage restic backups

- **`servosity-msp-cli restic-backups create`** - Create a restic backup account.
- **`servosity-msp-cli restic-backups delete`** - Delete a restic backup account.
- **`servosity-msp-cli restic-backups list`** - List
- **`servosity-msp-cli restic-backups partial-update`** - Update a restic backup account.
- **`servosity-msp-cli restic-backups read`** - Read
- **`servosity-msp-cli restic-backups update`** - Update a restic backup account.

### screenshot

Manage screenshot

- **`servosity-msp-cli screenshot <key>`** - Read

### stats

Manage stats

- **`servosity-msp-cli stats list`** - List
- **`servosity-msp-cli stats live-list`** - Live list
- **`servosity-msp-cli stats user-list`** - User list

### users

Manage users

- **`servosity-msp-cli users create`** - Create
- **`servosity-msp-cli users delete`** - Remove a user from a reseller or company group.
- **`servosity-msp-cli users list`** - List
- **`servosity-msp-cli users request-password-recovery-create`** - Request password recovery for a user.
- **`servosity-msp-cli users reset-password-create`** - Pass only `token` to confirm the token is valid.

Pass `token` and `password` to set the user's password.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
servosity-msp-cli agent-login list

# JSON for scripting and agents
servosity-msp-cli agent-login list --json

# Filter to specific fields
servosity-msp-cli agent-login list --json --select id,name,status

# Dry run — show the request without sending
servosity-msp-cli agent-login list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
servosity-msp-cli agent-login list --agent
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
servosity-msp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/servosity-partner-msp-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SERVOSITY_MSP_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `servosity-msp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SERVOSITY_MSP_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
