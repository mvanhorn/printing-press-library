# Servosity CLI

**Every Servosity endpoint as a typed command, plus a local fleet mirror, snapshot history, and cross-engine rollups the web UI doesn't have.**

Every Servosity REST endpoint becomes a typed Cobra command (Resellers, Companies, three backup engines, Issues, Reports, Admin) with `--json`, `--select`, `--csv`, `--dry-run`, and typed exit codes. A local SQLite mirror makes the fleet queryable offline. The attention command answers 'what needs my attention?' across all admin rollups in one call. The drift command tells you what got worse since yesterday. The backup-facts view unifies classic, restic, and DR engines into one view nothing else exposes.

Learn more at [Servosity](https://www.servosity.com).

Printed by [@dstevens](https://github.com/dstevens) (Damien Stevens).

## Install

The recommended path installs both the `servosity-pp-cli` binary and the `pp-servosity` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install servosity
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install servosity --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servosity-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-servosity --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-servosity --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-servosity skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-servosity. The skill defines how its required CLI can be installed.
```

## Authentication

Auth is a Servosity API token. Set SERVOSITY_API_TOKEN in your environment (or store it in macOS Keychain as a generic password named SERVOSITY_API_TOKEN and export it from your shell rc) and the CLI sends Authorization: Token <key> on every call — Servosity uses Django REST framework's Token scheme, not Bearer. **The Servosity API is single-tenant production: every mutating command (clear, triage, stale-issues, agent-restart subcommands) defaults to --dry-run; you must drop --dry-run AND pass --confirm to actually call the live API.**

## Quick Start

```bash
# Verify the token reaches /resellers/?page_size=1 successfully before doing anything else.
servosity-pp-cli doctor


# Get the morning fleet rollup: admin attention + dirty repos + DRaaS-in-flight + open issues, ranked per-company.
servosity-pp-cli attention --json


# Pull the /reports/stale-backup-sets/ CSV (use --timeout 180s on big fleets) and slice for >7-day stale sets.
servosity-pp-cli stale-backups --refresh --days 7 --json


# List the first 5 resellers to confirm reads end-to-end. Replace with `sync --resources resellers,companies,issues` once you're ready to populate the local store for offline search and drift.
servosity-pp-cli resellers list --json --select results.id,results.name --page-size 5


# Pull every relevant fact about a single company across all three engines into one screen.
servosity-pp-cli company show 4421 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Loop closure for fleet operators
- **`attention`** — One screen merges admin attention + dirty repos + DRaaS-in-progress + open issues, ranked per-company, and persists each call so tomorrow can drift against today.

  _Use this when an agent or human asks 'what needs my attention right now?' across the whole fleet — it answers in one call instead of four._

  ```bash
  servosity-pp-cli attention --json --select companies.name,companies.score
  ```
- **`triage`** — List open issues with filters, then batch ignore / archive / reactivate / comment in one invocation with --dry-run and typed exit codes.

  _Use this for batch issue actions when the queue is bursty — pipe-friendly, scriptable, idempotent._

  ```bash
  servosity-pp-cli triage --audience support --company 4421 --ignore 18,22,29 --dry-run
  ```
- **`restore-queue list`** — List per-company restore queues across companies the local store knows about; --watch repolls and prints diffs.

  _Use this during an active DR event to keep one terminal pinned on every queue's progress._

  ```bash
  servosity-pp-cli restore-queue list --watch --interval 30s
  ```

### Offline fleet querying
- **`stale-backups`** — Slice the synced /reports/stale-backup-sets/ snapshot by reseller, company, age window, and backup engine — entirely offline once synced. Use --refresh to repull the CSV.

  _Use this on Friday's 'who needs a follow-up?' sweep without burning an API call per slice._

  ```bash
  servosity-pp-cli stale-backups --days 7 --engine restic --json
  ```
- **`drift`** — Diff two snapshots the CLI itself collected (attention, stale, dirty-repos) — show what got worse and what recovered between two timestamps.

  _Use this every morning to surface fleet trend instead of treating every Monday like a fresh slate._

  ```bash
  servosity-pp-cli drift --metric attention --from yesterday --to now --json
  ```
- **`backup-facts`** — Query a unified view (engine + id + company_id + last_successful_at + last_status + size_bytes) over all three backup engines synced into the local store.

  _Use this when you don't care which engine — you just need 'who hasn't backed up successfully since X?' across the whole fleet._

  ```bash
  servosity-pp-cli backup-facts --last-success-before 2026-05-04 --json
  ```
- **`find`** — SQLite FTS5 across companies (name, billing notes), issues (title and comments), and backups (descriptive name, last error) — one query hits the whole fleet.

  _Use this when you remember a phrase but not which entity owned it — one call replaces hunting through three list pages._

  ```bash
  servosity-pp-cli find "image manager" --in issues,backups --json --select hits.resource,hits.snippet
  ```

### Per-company quick view
- **`company show`** — Single command pulls a company's metadata + addresses + contracts + all backups across three engines + open issues + agent sessions into one human or --json view.

  _Use this when a customer asks 'is my backup OK?' — one call, every relevant fact, ready to paste into a ticket._

  ```bash
  servosity-pp-cli company show 4421 --json
  ```

### Tier-One support workflows
- **`clear`** — Resolve one or more names as companies (then resellers) and batch-ignore their active issues until a human-readable time. Defaults to --dry-run for production safety.

  _Use this when a partner is doing planned maintenance and you want their alert noise paused until morning — one command instead of dozens of UI clicks._

  ```bash
  servosity-pp-cli clear "ACME Corp, BDH Technology" --until "6am tomorrow" --dry-run
  ```
- **`stale-issues`** — Pull your FMDB companies, fetch active issues, classify known-safe-to-archive patterns from a shipped rule table, auto-archive the safe ones, ignore non-dashboard noise, and print unknowns for review. Defaults to --dry-run.

  _Use this every weekday before standup to clear the obvious stale noise off your dashboard so triage focuses on what's actually new._

  ```bash
  servosity-pp-cli stale-issues --mine --cutoff "11pm yesterday" --auto-archive-known --dry-run
  ```

## Usage

Run `servosity-pp-cli --help` for the full command reference and flag list.

## Commands

### admin

Manage admin

- **`servosity-pp-cli admin attention-list`** - Attention list
- **`servosity-pp-cli admin dirty-repos-list`** - Dirty repos list
- **`servosity-pp-cli admin draas-in-progress-list`** - Draas in progress list
- **`servosity-pp-cli admin maxio-price-points-create`** - Maxio price points create
- **`servosity-pp-cli admin maxio-price-points-list`** - Maxio price points list
- **`servosity-pp-cli admin maxio-price-points-update`** - Maxio price points update
- **`servosity-pp-cli admin notification-broadcast-create`** - Send a notification to all users.
- **`servosity-pp-cli admin notification-create`** - Send a notification to a user.
- **`servosity-pp-cli admin servosity-one-push-message-create`** - Servosity one push message create
- **`servosity-pp-cli admin servosity-one-push-update-create`** - Servosity one push update create
- **`servosity-pp-cli admin servosity-one-worker-token-list`** - Servosity one worker token list
- **`servosity-pp-cli admin support-staff-list`** - Support staff list
- **`servosity-pp-cli admin users-list`** - Users list
- **`servosity-pp-cli admin worker-agents-list`** - Worker agents list
- **`servosity-pp-cli admin worker-run-list`** - Worker run list
- **`servosity-pp-cli admin worker-run-update`** - Worker run update

### agent-login

Manage agent login

- **`servosity-pp-cli agent-login create`** - Create
- **`servosity-pp-cli agent-login list`** - List

### agent-sessions

Manage agent sessions

- **`servosity-pp-cli agent-sessions read`** - Read

### backup-job-report

Manage backup job report

- **`servosity-pp-cli backup-job-report read`** - View detailed backup report for a backup job and destination.

### backup-job-report-summary

Manage backup job report summary

- **`servosity-pp-cli backup-job-report-summary read`** - View summary backup report for a backup job and destination.

### backup-job-status

Manage backup job status

- **`servosity-pp-cli backup-job-status list`** - List backup job status for a backup account on a specific date.

### backup-jobs

Manage backup jobs

- **`servosity-pp-cli backup-jobs list`** - List backup jobs for a backup account.

### backup-plans

Manage backup plans

- **`servosity-pp-cli backup-plans list`** - List backup plans.
- **`servosity-pp-cli backup-plans read`** - View a backup plan.

### backup-search

Manage backup search

- **`servosity-pp-cli backup-search list`** - List

### backup-sets

Manage backup sets

- **`servosity-pp-cli backup-sets create`** - Create a backup-set for a backup account.
- **`servosity-pp-cli backup-sets delete`** - Delete a backup-set for a backup account.
- **`servosity-pp-cli backup-sets list`** - List backup-sets for a backup account.
- **`servosity-pp-cli backup-sets read`** - View a backup-set for a backup account.
- **`servosity-pp-cli backup-sets update`** - Accepts a json body with the following optional parameters.

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

- **`servosity-pp-cli backups create`** - Create a backup account.
- **`servosity-pp-cli backups delete`** - Delete a backup account, also deleting all backup data.
- **`servosity-pp-cli backups list`** - List backup accounts.
- **`servosity-pp-cli backups mfa-codes`** - Mfa codes
- **`servosity-pp-cli backups partial-update`** - Partial update
- **`servosity-pp-cli backups read`** - View a backup account.
- **`servosity-pp-cli backups update`** - Update a backup account.

### companies

Manage companies

- **`servosity-pp-cli companies create`** - Create a company.
- **`servosity-pp-cli companies delete`** - Delete a company, also deleting all backup accounts and backup data.
- **`servosity-pp-cli companies fully-managed`** - List fully-managed companies.
- **`servosity-pp-cli companies fully-managed-ng`** - List fully-managed companies.
- **`servosity-pp-cli companies list`** - List companies.
- **`servosity-pp-cli companies partial-update`** - Partial update
- **`servosity-pp-cli companies read`** - View a company.
- **`servosity-pp-cli companies summary`** - List companies with account summaries.
- **`servosity-pp-cli companies summary-ng`** - Summary ng
- **`servosity-pp-cli companies update`** - Update a company.

### company-notes

Manage company notes

- **`servosity-pp-cli company-notes create`** - Create
- **`servosity-pp-cli company-notes delete`** - Delete
- **`servosity-pp-cli company-notes list`** - List
- **`servosity-pp-cli company-notes partial-update`** - Partial update
- **`servosity-pp-cli company-notes read`** - Read
- **`servosity-pp-cli company-notes update`** - Update

### components

Manage components

- **`servosity-pp-cli components list`** - List

### contracts

Manage contracts

- **`servosity-pp-cli contracts create`** - Create
- **`servosity-pp-cli contracts get-by-token`** - Get by token
- **`servosity-pp-cli contracts list`** - List
- **`servosity-pp-cli contracts partial-update`** - Partial update
- **`servosity-pp-cli contracts read`** - Read
- **`servosity-pp-cli contracts signatures`** - Signatures
- **`servosity-pp-cli contracts update`** - Update

### credentials

Manage credentials

- **`servosity-pp-cli credentials create`** - Create
- **`servosity-pp-cli credentials delete`** - Delete
- **`servosity-pp-cli credentials list`** - List
- **`servosity-pp-cli credentials partial-update`** - Partial update
- **`servosity-pp-cli credentials read`** - Read
- **`servosity-pp-cli credentials update`** - Update

### current-user

Manage current user

- **`servosity-pp-cli current-user api-token-delete`** - Delete the current user's API token. A new one will be generated when requested.
- **`servosity-pp-cli current-user api-token-list`** - You will receive JSON response with `token`.

To make API calls with the token, add an `Authorization` header to your request in this form:

`Authorization: Token XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`
- **`servosity-pp-cli current-user create`** - Change the password of the current logged in user.
- **`servosity-pp-cli current-user groups-list`** - Groups list
- **`servosity-pp-cli current-user helpjuice-sso-create`** - Helpjuice sso create
- **`servosity-pp-cli current-user hubspot-sso-create`** - Hubspot sso create
- **`servosity-pp-cli current-user list`** - Get information about the current logged in user.
- **`servosity-pp-cli current-user mfa-backup-codes-list`** - Get unused backup codes.
If no unused codes are left, remove all and generate new codes.
- **`servosity-pp-cli current-user mfa-backup-codes-update`** - Remove all backup codes and generate new codes.
- **`servosity-pp-cli current-user notifications-delete`** - Notifications delete
- **`servosity-pp-cli current-user notifications-list`** - Get current user notifications
- **`servosity-pp-cli current-user profile-create`** - Profile create
- **`servosity-pp-cli current-user profile-list`** - Profile list
- **`servosity-pp-cli current-user start-mfa-create`** - Start mfa create
- **`servosity-pp-cli current-user start-mfa-list`** - Start mfa list
- **`servosity-pp-cli current-user start-mfa-verify-create`** - Start mfa verify create
- **`servosity-pp-cli current-user verified-mfa-delete`** - Verified mfa delete
- **`servosity-pp-cli current-user verified-mfa-list`** - Verified mfa list
- **`servosity-pp-cli current-user verified-mfa-send-code-create`** - Verified mfa send code create

### download

Manage download

- **`servosity-pp-cli download servosity-one-windows-list`** - Servosity one windows list

### dr-backups

Manage dr backups

- **`servosity-pp-cli dr-backups create`** - Create a DR backup account.
- **`servosity-pp-cli dr-backups delete`** - Delete a DR backup account.
- **`servosity-pp-cli dr-backups list`** - List
- **`servosity-pp-cli dr-backups partial-update`** - Update a DR backup account.
- **`servosity-pp-cli dr-backups read`** - Read
- **`servosity-pp-cli dr-backups update`** - Update a DR backup account.

### issue-comments

Manage issue comments

- **`servosity-pp-cli issue-comments delete`** - Delete
- **`servosity-pp-cli issue-comments update`** - Update

### issues

Manage issues

- **`servosity-pp-cli issues archived`** - Archived
- **`servosity-pp-cli issues ignored`** - Ignored
- **`servosity-pp-cli issues list`** - List
- **`servosity-pp-cli issues read`** - Read

### postmark-webhook

Manage postmark webhook

- **`servosity-pp-cli postmark-webhook create`** - Create

### report-subscriptions

Manage report subscriptions

- **`servosity-pp-cli report-subscriptions read`** - Read
- **`servosity-pp-cli report-subscriptions unsubscribe`** - Unsubscribe
- **`servosity-pp-cli report-subscriptions verify`** - Verify

### reports

Manage reports

- **`servosity-pp-cli reports account-list`** - Get a report of backup account types for each company and reseller in CSV format.
- **`servosity-pp-cli reports classic-usage-list`** - Get a usage report for all backup accounts in CSV format.
- **`servosity-pp-cli reports clients-list`** - Get a report of backup account client versions.
- **`servosity-pp-cli reports dr-from-email-list`** - Get a report of user profiles.
- **`servosity-pp-cli reports maxio-price-points-list`** - Get CSV with all Maxio price points.
- **`servosity-pp-cli reports product-list`** - Product list
- **`servosity-pp-cli reports stale-backup-sets-list`** - Get a report of all backup set last backup complete times.
- **`servosity-pp-cli reports usage-list`** - Usage list
- **`servosity-pp-cli reports user-profiles-list`** - Get a report of user profiles.

### reseller-notes

Manage reseller notes

- **`servosity-pp-cli reseller-notes create`** - Create
- **`servosity-pp-cli reseller-notes delete`** - Delete
- **`servosity-pp-cli reseller-notes list`** - List
- **`servosity-pp-cli reseller-notes partial-update`** - Partial update
- **`servosity-pp-cli reseller-notes read`** - Read
- **`servosity-pp-cli reseller-notes update`** - Update

### resellers

Manage resellers

- **`servosity-pp-cli resellers create`** - Create a reseller.
- **`servosity-pp-cli resellers delete`** - Delete a reseller, also deleting all companies, backup accounts, and backup data.
- **`servosity-pp-cli resellers list`** - List resellers.
- **`servosity-pp-cli resellers partial-update`** - Partial update
- **`servosity-pp-cli resellers read`** - View a reseller.
- **`servosity-pp-cli resellers summary`** - List resellers with account summaries.
- **`servosity-pp-cli resellers update`** - Update a reseller.

### restic-backups

Manage restic backups

- **`servosity-pp-cli restic-backups create`** - Create a restic backup account.
- **`servosity-pp-cli restic-backups delete`** - Delete a restic backup account.
- **`servosity-pp-cli restic-backups list`** - List
- **`servosity-pp-cli restic-backups partial-update`** - Update a restic backup account.
- **`servosity-pp-cli restic-backups read`** - Read
- **`servosity-pp-cli restic-backups update`** - Update a restic backup account.

### restore-queue-web-login

Manage restore queue web login

- **`servosity-pp-cli restore-queue-web-login list`** - List
- **`servosity-pp-cli restore-queue-web-login update`** - Update

### screenshot

Manage screenshot

- **`servosity-pp-cli screenshot read`** - Read

### sms-fm-mfa-callback

Manage sms fm mfa callback

- **`servosity-pp-cli sms-fm-mfa-callback create`** - Create

### sms-mfa-callback

Manage sms mfa callback

- **`servosity-pp-cli sms-mfa-callback create`** - Create

### stats

Manage stats

- **`servosity-pp-cli stats list`** - List
- **`servosity-pp-cli stats live-list`** - Live list
- **`servosity-pp-cli stats user-list`** - User list

### users

Manage users

- **`servosity-pp-cli users create`** - Create
- **`servosity-pp-cli users delete`** - Remove a user from a reseller or company group.
- **`servosity-pp-cli users list`** - List
- **`servosity-pp-cli users request-password-recovery-create`** - Request password recovery for a user.
- **`servosity-pp-cli users reset-password-create`** - Pass only `token` to confirm the token is valid.

Pass `token` and `password` to set the user's password.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
servosity-pp-cli agent-login list

# JSON for scripting and agents
servosity-pp-cli agent-login list --json

# Filter to specific fields
servosity-pp-cli agent-login list --json --select id,name,status

# Dry run — show the request without sending
servosity-pp-cli agent-login list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
servosity-pp-cli agent-login list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-servosity -g
```

Then invoke `/pp-servosity <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add servosity servosity-pp-mcp -e SERVOSITY_API_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servosity-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SERVOSITY_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "servosity": {
      "command": "servosity-pp-mcp",
      "env": {
        "SERVOSITY_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
servosity-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/servosity-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SERVOSITY_API_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `servosity-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SERVOSITY_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Authentication credentials were not provided** — Confirm SERVOSITY_API_TOKEN is set and the header format is `Token <key>` (not `Bearer`). Run `servosity-pp-cli doctor` for a clean reachability test.
- **403 You do not have permission** — Your token is scoped below the resource you're hitting (e.g. trying admin endpoints with a reseller-only token). Get a token with the right scope from your Servosity admin.
- **stale / drift / backup-facts return empty** — Run `servosity-pp-cli sync` first — these read from the local SQLite mirror, not live API calls.
- **triage / clear / stale-issues refuse to act** — PLAN mode is the default for production safety. Re-run with `--confirm` to actually call the mutation endpoints (and keep the global `--dry-run` flag off; it overrides --confirm).

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
