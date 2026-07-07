# Clockify CLI

**Every Clockify feature, plus a local time database that reconstructs your weekly timesheet, finds untracked gaps, and audits billable hours — none of which any other Clockify tool can do.**

Other Clockify CLIs are timers with a thin reporting bolt-on, and none of them remember anything. This one keeps a local SQLite mirror of every time entry, project, client, and tag, so reports, timesheet reconstruction, gap detection, and billable audits run offline and instantly. It also covers the half of Clockify no terminal tool touches — invoices, expenses, time-off, approvals, and scheduling.

Created by [@melanson633](https://github.com/melanson633) (melanson633).

## Install

The recommended path installs both the `clockify-pp-cli` binary and the `pp-clockify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clockify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clockify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clockify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install clockify --agent claude-code
npx -y @mvanhorn/printing-press-library install clockify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/clockify/cmd/clockify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clockify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clockify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clockify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clockify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clockify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clockify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLOCKIFY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "clockify": {
      "command": "clockify-pp-mcp",
      "env": {
        "CLOCKIFY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a personal API key from the Clockify web app (Profile Settings -> API). Export it as CLOCKIFY_API_KEY; the CLI sends it as the X-Api-Key header. The key is read-only-safe for listing and reporting and is required for any write.

## Quick Start

```bash
# Confirm the API key works and Clockify is reachable before anything else.
clockify-pp-cli doctor

# Pull workspaces, projects, clients, tags, and time entries into the local store — every offline command reads from here.
clockify-pp-cli sync

# Reconstruct this week's grid from the synced entries — the headline view.
clockify-pp-cli timesheet week

# Find the days you are short before you submit the timesheet.
clockify-pp-cli timesheet gaps

# See where your tracked time actually went.
clockify-pp-cli recap

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Timesheet intelligence
- **`timesheet week`** — Rebuild your whole weekly timesheet grid offline — every project and task across each weekday, with per-day, per-project, and weekly totals.

  _Reach for this when an agent needs the full week at a glance instead of paging raw time-entry JSON._

  ```bash
  clockify-pp-cli timesheet week --agent
  ```
- **`timesheet gaps`** — Find the days you are short — compares each day's tracked hours against your workday target and reports the missing time before you submit.

  _Use this before submitting a timesheet so an agent can flag under-logged days instead of the user discovering them at month-end._

  ```bash
  clockify-pp-cli timesheet gaps --workday 8h --agent
  ```
- **`team timesheets`** — See who has not submitted — diffs the full workspace user list against the week's approval requests so the people who forgot are visible, not invisible.

  _Use this for a manager's Monday approval sweep to chase non-submitters without clicking through every member._

  ```bash
  clockify-pp-cli team timesheets --agent
  ```
- **`backfill`** — Reconstruct the time you forgot to track — turn a CSV export, your shell history, or a CLI session log into draft time entries, review them, then commit them to Clockify.

  _Reach for this when the timer was never started — an agent can draft entries from logs the user already has instead of losing the time._

  ```bash
  clockify-pp-cli backfill --from session-log --file ./session.jsonl --agent
  ```

### Billable revenue protection
- **`audit billable`** — Catch the misfiled entries that silently drop off invoices — billable time with no project, billable tasks marked non-billable, and untagged billable entries.

  _Run this at invoice time so an agent surfaces revenue leakage before the billing period closes._

  ```bash
  clockify-pp-cli audit billable --agent
  ```
- **`billable pending`** — The invoice-ready number — sums billable time not yet covered by any synced invoice, grouped by client.

  _Use this before raising an invoice so an agent knows exactly how much uninvoiced billable time exists per client._

  ```bash
  clockify-pp-cli billable pending --agent
  ```
- **`projects burn`** — Estimate vs actual per project — hours logged against the project's budget or time estimate, with percent consumed.

  _Reach for this when an agent needs to flag projects about to blow their estimate before the overrun happens._

  ```bash
  clockify-pp-cli projects burn --agent
  ```

### Local time database
- **`recap`** — A ranked breakdown of where your tracked time went — by project, client, and tag, with the billable vs non-billable split and percentage of the total.

  _Reach for this for a fast time-allocation summary instead of running and parsing a Clockify web report._

  ```bash
  clockify-pp-cli recap --range last-month --agent
  ```
- **`search`** — Full-text search across every synced time-entry description and project, task, client, and tag name.

  _Use this to locate a past entry by description without paginating the live API._

  ```bash
  clockify-pp-cli search "client onboarding" --agent
  ```

## Usage

Run `clockify-pp-cli --help` for the full command reference and flag list.

## Commands

### addons

Manage addons

### approval-requests

Manage approval requests

- **`clockify-pp-cli approval-requests create-approval-for-other`** - Submit an approval request for a user
- **`clockify-pp-cli approval-requests create-apprroval-request`** - Submit approval request
- **`clockify-pp-cli approval-requests get`** - Get approval requests
- **`clockify-pp-cli approval-requests resubmit`** - Submit non pending/approved entries/expenses for approval to an existing approval request
- **`clockify-pp-cli approval-requests resubmit-for-other`** - Re-submit rejected/withdrawn entries/expenses for an approval of a user
- **`clockify-pp-cli approval-requests update-approval-status`** - Update an approval request

### clients

Manage clients

- **`clockify-pp-cli clients create`** - Add a new client
- **`clockify-pp-cli clients delete`** - Delete a client
- **`clockify-pp-cli clients get`** - Find clients on a workspace
- **`clockify-pp-cli clients get-workspaces`** - Get a client by ID
- **`clockify-pp-cli clients update`** - Update a client

### cost-rate

Manage cost rate

- **`clockify-pp-cli cost-rate <workspaceId>`** - Update workspace cost rate

### custom-fields

Manage custom fields

- **`clockify-pp-cli custom-fields create`** - Create custom fields on a workspace
- **`clockify-pp-cli custom-fields delete`** - Delete a custom field
- **`clockify-pp-cli custom-fields edit`** - Update custom field on workspace
- **`clockify-pp-cli custom-fields of-workspace`** - Get custom fields on a workspace

### entities

Manage entities

- **`clockify-pp-cli entities get-created-entity-info`** - Retrieves records from the database collection that were created within a specified date range.  
The date range is determined by two parameters: start and end.
- **`clockify-pp-cli entities get-deleted-entity-info`** - Retrieves a list of record(s) that were deleted within a specified date range.   
The date range is determined by the two parameters start and end.  

> ### 💡 Note
> Deleted entities will be updated and reflected in this endpoint approximately one minute after the deletion occurs. Also, entities that are created and deleted within the request date range will not appear in the /deleted endpoint.
- **`clockify-pp-cli entities get-updated-entity-info`** - Retrieves records that were updated within the specified date range.   
The date range is determined by the two parameters: start and end.   

> ### 💡 Note
> If an entity is both created and updated within the requested date range, it will be excluded from the /updated endpoint results.

### expenses

Manage expenses

- **`clockify-pp-cli expenses create`** - Create an expense
- **`clockify-pp-cli expenses create-category`** - Add an expense category
- **`clockify-pp-cli expenses delete`** - Delete an expense
- **`clockify-pp-cli expenses delete-category`** - Delete an expense category
- **`clockify-pp-cli expenses get`** - Get all expenses on a workspace
- **`clockify-pp-cli expenses get-categories`** - Get all expense categories
- **`clockify-pp-cli expenses get-workspaces`** - Get an expense by ID
- **`clockify-pp-cli expenses update`** - Update an expense
- **`clockify-pp-cli expenses update-category`** - Update an expense category
- **`clockify-pp-cli expenses update-category-status`** - Archive an expense category

### file

Manage file

- **`clockify-pp-cli file`** - Add a photo

### holidays

Manage holidays

- **`clockify-pp-cli holidays create`** - Create a holiday
- **`clockify-pp-cli holidays delete`** - Delete a holiday
- **`clockify-pp-cli holidays get`** - Get holidays on a workspace
- **`clockify-pp-cli holidays get-in-period`** - Get holidays in a specific period
- **`clockify-pp-cli holidays update`** - Update a holiday

### hourly-rate

Manage hourly rate

- **`clockify-pp-cli hourly-rate <workspaceId>`** - Update workspace billable rate

### invoices

Manage invoices

- **`clockify-pp-cli invoices create`** - Add an invoice
- **`clockify-pp-cli invoices delete`** - Delete an invoice
- **`clockify-pp-cli invoices get`** - Get all invoices on a workspace
- **`clockify-pp-cli invoices get-info`** - Filter out invoices
- **`clockify-pp-cli invoices get-settings`** - Get an invoice in another language
- **`clockify-pp-cli invoices get-workspaces`** - Get an invoice by ID
- **`clockify-pp-cli invoices update`** - Update an invoice
- **`clockify-pp-cli invoices update-settings`** - Change an invoice language

### member-profile

Manage member profile

- **`clockify-pp-cli member-profile get`** - Get a member's profile
- **`clockify-pp-cli member-profile update-with-additional-data`** - Update a member's profile

### projects

Manage projects

- **`clockify-pp-cli projects create-from-template`** - Create project from a template
- **`clockify-pp-cli projects create-new`** - Add a new project
- **`clockify-pp-cli projects delete`** - Delete a project from a workspace
- **`clockify-pp-cli projects get`** - Get all projects on a workspace
- **`clockify-pp-cli projects get-workspaces`** - Find a project by ID
- **`clockify-pp-cli projects update`** - Update a project on a workspace

### scheduling

Manage scheduling

- **`clockify-pp-cli scheduling copy-assignment`** - Copy a scheduled assignment
- **`clockify-pp-cli scheduling create-recurring`** - Create a recurring assignment
- **`clockify-pp-cli scheduling delete-rrecurring-assignment`** - Delete a recurring assignment
- **`clockify-pp-cli scheduling edit-recurring`** - Update a recurring assignment
- **`clockify-pp-cli scheduling edit-recurring-period`** - Change the recurring period
- **`clockify-pp-cli scheduling get-all-assignments`** - Get all assignments
- **`clockify-pp-cli scheduling get-filtered-project-totals`** - Get all scheduled assignments per project
- **`clockify-pp-cli scheduling get-project-totals`** - Get all scheduled assignments per project
- **`clockify-pp-cli scheduling get-project-totals-for-single-project`** - Get all scheduled assignments on project
- **`clockify-pp-cli scheduling get-user-totals`** - Get total of users' capacity on workspace
- **`clockify-pp-cli scheduling get-user-totals-for-single-user`** - Get total capacity of a user
- **`clockify-pp-cli scheduling publish-assignments`** - Publish assignments

### tags

Manage tags

- **`clockify-pp-cli tags create-new`** - Add a new tag
- **`clockify-pp-cli tags delete`** - Delete a tag
- **`clockify-pp-cli tags get`** - Find tags on a workspace
- **`clockify-pp-cli tags get-workspaces`** - Get a tag by ID
- **`clockify-pp-cli tags update`** - Update a tag

### templates

Manage templates

- **`clockify-pp-cli templates create-many`** - Create templates on a workspace
- **`clockify-pp-cli templates delete-1`** - Delete a template
- **`clockify-pp-cli templates get`** - Get all templates on a workspace
- **`clockify-pp-cli templates get-workspaces`** - Get template by ID on a workspace
- **`clockify-pp-cli templates update`** - Update a template

### time-entries

Manage time entries

- **`clockify-pp-cli time-entries create-time-entry`** - Add a new time entry
- **`clockify-pp-cli time-entries delete-time-entry`** - Delete a time entry from a workspace
- **`clockify-pp-cli time-entries get-in-progress`** - Get all in progress time entries on a workspace
- **`clockify-pp-cli time-entries get-time-entry`** - Get a specific time entry on a workspace
- **`clockify-pp-cli time-entries update-invoiced-status`** - Mark time entries as invoiced
- **`clockify-pp-cli time-entries update-time-entry`** - Update time entry on a workspace

### time-off

Manage time off

- **`clockify-pp-cli time-off change-request-status`** - Change a time off request status
- **`clockify-pp-cli time-off create-policy`** - Create a time off policy
- **`clockify-pp-cli time-off create-request`** - Create a time off request
- **`clockify-pp-cli time-off create-request-for-other`** - Create a time off request for a user
- **`clockify-pp-cli time-off delete-policy`** - Delete a policy
- **`clockify-pp-cli time-off delete-request`** - Delete a time off request
- **`clockify-pp-cli time-off find-policies-for-workspace`** - Get policies on a workspace
- **`clockify-pp-cli time-off get-balances-for-policy`** - Get balances for a policy
- **`clockify-pp-cli time-off get-balances-for-user`** - Get balance for a user
- **`clockify-pp-cli time-off get-policy`** - Get a time off policy
- **`clockify-pp-cli time-off get-request`** - Get all time off requests on a workspace
- **`clockify-pp-cli time-off update-balance`** - Update a balance
- **`clockify-pp-cli time-off update-policy`** - Update a policy
- **`clockify-pp-cli time-off update-policy-status`** - Change a policy status

### user

Manage user

- **`clockify-pp-cli user`** - Get currently logged-in user's info

### user-groups

Manage user groups

- **`clockify-pp-cli user-groups create`** - Add a new group
- **`clockify-pp-cli user-groups delete`** - Delete a group
- **`clockify-pp-cli user-groups get`** - Find all groups on a workspace
- **`clockify-pp-cli user-groups update`** - Update a group

### users

Manage users

- **`clockify-pp-cli users add`** - You can add users to a workspace via API only if that workspace has a paid subscription. If the workspace has a paid subscription, you can add as many users as you want but you are limited by the number of paid user seats on that workspace.
- **`clockify-pp-cli users filter-of-workspace`** - Filter workspace users
- **`clockify-pp-cli users get-of-workspace`** - Find all users on a workspace
- **`clockify-pp-cli users remove-member`** - This endpoint is not functional and has been deprecated. A user can be removed/deleted on the CAKE.com Account Members page after deactivating all their existing memberships on all workspaces within an organization.
- **`clockify-pp-cli users update-status`** - Update a user's status

### webhooks

Manage webhooks

- **`clockify-pp-cli webhooks create`** - Creating a webhook generates a new token which can be used to verify that the webhook being sent was sent by Clockify, as it will always be present in the header.
- **`clockify-pp-cli webhooks delete`** - Delete a webhook
- **`clockify-pp-cli webhooks get`** - Get all webhooks on a workspace
- **`clockify-pp-cli webhooks get-workspaces`** - Get a specific webhook by id
- **`clockify-pp-cli webhooks update`** - Update a webhook

### workspaces

Manage workspaces

- **`clockify-pp-cli workspaces create`** - Add a workspace
- **`clockify-pp-cli workspaces get-of-user`** - Get all my workspaces
- **`clockify-pp-cli workspaces get-of-user-workspaceid`** - Get workspace info

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clockify-pp-cli approval-requests get mock-value

# JSON for scripting and agents
clockify-pp-cli approval-requests get mock-value --json

# Filter to specific fields
clockify-pp-cli approval-requests get mock-value --json --select id,name,status

# Dry run — show the request without sending
clockify-pp-cli approval-requests get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clockify-pp-cli approval-requests get mock-value --agent
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
clockify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/clockify-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLOCKIFY_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clockify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLOCKIFY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — CLOCKIFY_API_KEY is missing or stale — regenerate it in the Clockify web app under Profile Settings -> API and re-export it.
- **timesheet or recap output is empty** — Run `clockify-pp-cli sync` first — the offline commands read the local store, not the live API.
- **commands return data from the wrong workspace** — Pass the workspace id explicitly: the `timesheet`, `recap`, `audit`, `team`, and `billable` commands accept `--workspace <id>`, and the generated endpoint commands take it as the first positional argument.
- **429 Too Many Requests during a large sync** — Clockify rate-limits at roughly 50 requests/second; the client backs off and retries automatically — just let the sync finish.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**lucassabreu/clockify-cli**](https://github.com/lucassabreu/clockify-cli) — Go (194 stars)
- [**BlythMeister/ClockifyCli**](https://github.com/BlythMeister/ClockifyCli) — C# (1 stars)
- [**JeremyVyska/clockify-mcp**](https://github.com/JeremyVyska/clockify-mcp) — TypeScript
- [**mentarch/clockify-cli**](https://github.com/mentarch/clockify-cli) — TypeScript
- [**artefactual-labs/clockify-tool**](https://github.com/artefactual-labs/clockify-tool) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
