---
name: pp-clockify
description: "Every Clockify feature, plus a local time database that reconstructs your weekly timesheet, finds untracked gaps, and audits billable hours. Trigger phrases: `fill my timesheet`, `reconstruct my week in clockify`, `did I log all my hours`, `what can I invoice right now`, `where did my week go`, `use clockify`, `run clockify-pp-cli`. Not for: non-Clockify time trackers (Toggl, Harvest), payroll runs, or calendar scheduling."
author: "melanson633"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clockify-pp-cli
---

# Clockify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clockify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install clockify --cli-only
   ```
2. Verify: `clockify-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Other Clockify CLIs are timers with a thin reporting bolt-on, and none of them remember anything. This one keeps a local SQLite mirror of every time entry, project, client, and tag, so reports, timesheet reconstruction, gap detection, and billable audits run offline and instantly. It also covers the half of Clockify no terminal tool touches — invoices, expenses, time-off, approvals, and scheduling.

## When to Use This CLI

Use this CLI when an agent or user needs to work with Clockify time data programmatically: filling or reviewing a weekly timesheet, checking that all hours are logged before submission, auditing billable time before invoicing, summarizing where time went over a range, or driving any Clockify resource (projects, clients, expenses, invoices, time-off, approvals) from the terminal. Prefer the offline commands (timesheet, recap, audit, search) after a sync — they are instant and need no network.

## Unique Capabilities

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

## Command Reference

**addons** — Manage addons


**approval-requests** — Manage approval requests

- `clockify-pp-cli approval-requests create-approval-for-other` — Submit an approval request for a user
- `clockify-pp-cli approval-requests create-apprroval-request` — Submit approval request
- `clockify-pp-cli approval-requests get` — Get approval requests
- `clockify-pp-cli approval-requests resubmit` — Submit non pending/approved entries/expenses for approval to an existing approval request
- `clockify-pp-cli approval-requests resubmit-for-other` — Re-submit rejected/withdrawn entries/expenses for an approval of a user
- `clockify-pp-cli approval-requests update-approval-status` — Update an approval request

**clients** — Manage clients

- `clockify-pp-cli clients create` — Add a new client
- `clockify-pp-cli clients delete` — Delete a client
- `clockify-pp-cli clients get` — Find clients on a workspace
- `clockify-pp-cli clients get-workspaces` — Get a client by ID
- `clockify-pp-cli clients update` — Update a client

**cost-rate** — Manage cost rate

- `clockify-pp-cli cost-rate <workspaceId>` — Update workspace cost rate

**custom-fields** — Manage custom fields

- `clockify-pp-cli custom-fields create` — Create custom fields on a workspace
- `clockify-pp-cli custom-fields delete` — Delete a custom field
- `clockify-pp-cli custom-fields edit` — Update custom field on workspace
- `clockify-pp-cli custom-fields of-workspace` — Get custom fields on a workspace

**entities** — Manage entities

- `clockify-pp-cli entities get-created-entity-info` — Retrieves records from the database collection that were created within a specified date range. The date range is...
- `clockify-pp-cli entities get-deleted-entity-info` — Retrieves a list of record(s) that were deleted within a specified date range. The date range is determined by the...
- `clockify-pp-cli entities get-updated-entity-info` — Retrieves records that were updated within the specified date range. The date range is determined by the two...

**expenses** — Manage expenses

- `clockify-pp-cli expenses create` — Create an expense
- `clockify-pp-cli expenses create-category` — Add an expense category
- `clockify-pp-cli expenses delete` — Delete an expense
- `clockify-pp-cli expenses delete-category` — Delete an expense category
- `clockify-pp-cli expenses get` — Get all expenses on a workspace
- `clockify-pp-cli expenses get-categories` — Get all expense categories
- `clockify-pp-cli expenses get-workspaces` — Get an expense by ID
- `clockify-pp-cli expenses update` — Update an expense
- `clockify-pp-cli expenses update-category` — Update an expense category
- `clockify-pp-cli expenses update-category-status` — Archive an expense category

**file** — Manage file

- `clockify-pp-cli file` — Add a photo

**holidays** — Manage holidays

- `clockify-pp-cli holidays create` — Create a holiday
- `clockify-pp-cli holidays delete` — Delete a holiday
- `clockify-pp-cli holidays get` — Get holidays on a workspace
- `clockify-pp-cli holidays get-in-period` — Get holidays in a specific period
- `clockify-pp-cli holidays update` — Update a holiday

**hourly-rate** — Manage hourly rate

- `clockify-pp-cli hourly-rate <workspaceId>` — Update workspace billable rate

**invoices** — Manage invoices

- `clockify-pp-cli invoices create` — Add an invoice
- `clockify-pp-cli invoices delete` — Delete an invoice
- `clockify-pp-cli invoices get` — Get all invoices on a workspace
- `clockify-pp-cli invoices get-info` — Filter out invoices
- `clockify-pp-cli invoices get-settings` — Get an invoice in another language
- `clockify-pp-cli invoices get-workspaces` — Get an invoice by ID
- `clockify-pp-cli invoices update` — Update an invoice
- `clockify-pp-cli invoices update-settings` — Change an invoice language

**member-profile** — Manage member profile

- `clockify-pp-cli member-profile get` — Get a member's profile
- `clockify-pp-cli member-profile update-with-additional-data` — Update a member's profile

**projects** — Manage projects

- `clockify-pp-cli projects create-from-template` — Create project from a template
- `clockify-pp-cli projects create-new` — Add a new project
- `clockify-pp-cli projects delete` — Delete a project from a workspace
- `clockify-pp-cli projects get` — Get all projects on a workspace
- `clockify-pp-cli projects get-workspaces` — Find a project by ID
- `clockify-pp-cli projects update` — Update a project on a workspace

**scheduling** — Manage scheduling

- `clockify-pp-cli scheduling copy-assignment` — Copy a scheduled assignment
- `clockify-pp-cli scheduling create-recurring` — Create a recurring assignment
- `clockify-pp-cli scheduling delete-rrecurring-assignment` — Delete a recurring assignment
- `clockify-pp-cli scheduling edit-recurring` — Update a recurring assignment
- `clockify-pp-cli scheduling edit-recurring-period` — Change the recurring period
- `clockify-pp-cli scheduling get-all-assignments` — Get all assignments
- `clockify-pp-cli scheduling get-filtered-project-totals` — Get all scheduled assignments per project
- `clockify-pp-cli scheduling get-project-totals` — Get all scheduled assignments per project
- `clockify-pp-cli scheduling get-project-totals-for-single-project` — Get all scheduled assignments on project
- `clockify-pp-cli scheduling get-user-totals` — Get total of users' capacity on workspace
- `clockify-pp-cli scheduling get-user-totals-for-single-user` — Get total capacity of a user
- `clockify-pp-cli scheduling publish-assignments` — Publish assignments

**tags** — Manage tags

- `clockify-pp-cli tags create-new` — Add a new tag
- `clockify-pp-cli tags delete` — Delete a tag
- `clockify-pp-cli tags get` — Find tags on a workspace
- `clockify-pp-cli tags get-workspaces` — Get a tag by ID
- `clockify-pp-cli tags update` — Update a tag

**templates** — Manage templates

- `clockify-pp-cli templates create-many` — Create templates on a workspace
- `clockify-pp-cli templates delete-1` — Delete a template
- `clockify-pp-cli templates get` — Get all templates on a workspace
- `clockify-pp-cli templates get-workspaces` — Get template by ID on a workspace
- `clockify-pp-cli templates update` — Update a template

**time-entries** — Manage time entries

- `clockify-pp-cli time-entries create-time-entry` — Add a new time entry
- `clockify-pp-cli time-entries delete-time-entry` — Delete a time entry from a workspace
- `clockify-pp-cli time-entries get-in-progress` — Get all in progress time entries on a workspace
- `clockify-pp-cli time-entries get-time-entry` — Get a specific time entry on a workspace
- `clockify-pp-cli time-entries update-invoiced-status` — Mark time entries as invoiced
- `clockify-pp-cli time-entries update-time-entry` — Update time entry on a workspace

**time-off** — Manage time off

- `clockify-pp-cli time-off change-request-status` — Change a time off request status
- `clockify-pp-cli time-off create-policy` — Create a time off policy
- `clockify-pp-cli time-off create-request` — Create a time off request
- `clockify-pp-cli time-off create-request-for-other` — Create a time off request for a user
- `clockify-pp-cli time-off delete-policy` — Delete a policy
- `clockify-pp-cli time-off delete-request` — Delete a time off request
- `clockify-pp-cli time-off find-policies-for-workspace` — Get policies on a workspace
- `clockify-pp-cli time-off get-balances-for-policy` — Get balances for a policy
- `clockify-pp-cli time-off get-balances-for-user` — Get balance for a user
- `clockify-pp-cli time-off get-policy` — Get a time off policy
- `clockify-pp-cli time-off get-request` — Get all time off requests on a workspace
- `clockify-pp-cli time-off update-balance` — Update a balance
- `clockify-pp-cli time-off update-policy` — Update a policy
- `clockify-pp-cli time-off update-policy-status` — Change a policy status

**user** — Manage user

- `clockify-pp-cli user` — Get currently logged-in user's info

**user-groups** — Manage user groups

- `clockify-pp-cli user-groups create` — Add a new group
- `clockify-pp-cli user-groups delete` — Delete a group
- `clockify-pp-cli user-groups get` — Find all groups on a workspace
- `clockify-pp-cli user-groups update` — Update a group

**users** — Manage users

- `clockify-pp-cli users add` — You can add users to a workspace via API only if that workspace has a paid subscription. If the workspace has a paid...
- `clockify-pp-cli users filter-of-workspace` — Filter workspace users
- `clockify-pp-cli users get-of-workspace` — Find all users on a workspace
- `clockify-pp-cli users remove-member` — This endpoint is not functional and has been deprecated. A user can be removed/deleted on the CAKE.com Account...
- `clockify-pp-cli users update-status` — Update a user's status

**webhooks** — Manage webhooks

- `clockify-pp-cli webhooks create` — Creating a webhook generates a new token which can be used to verify that the webhook being sent was sent by...
- `clockify-pp-cli webhooks delete` — Delete a webhook
- `clockify-pp-cli webhooks get` — Get all webhooks on a workspace
- `clockify-pp-cli webhooks get-workspaces` — Get a specific webhook by id
- `clockify-pp-cli webhooks update` — Update a webhook

**workspaces** — Manage workspaces

- `clockify-pp-cli workspaces create` — Add a workspace
- `clockify-pp-cli workspaces get-of-user` — Get all my workspaces
- `clockify-pp-cli workspaces get-of-user-workspaceid` — Get workspace info


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clockify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Reconstruct this week's timesheet

```bash
clockify-pp-cli timesheet week
```

Pivots every synced entry into the project/task by weekday grid with per-day and weekly totals.

### Catch under-logged days before Friday

```bash
clockify-pp-cli timesheet gaps --workday 8h
```

Reports each day below the 8-hour target and how much time is missing.

### Narrow the week grid for an agent

```bash
clockify-pp-cli timesheet week --agent --select projects.project,projects.total_hours,total_hours
```

Narrows the weekly grid JSON to just project names and totals with dotted-path selection, so an agent does not burn context on every daily cell.

### Find what is invoice-ready

```bash
clockify-pp-cli billable pending
```

Sums billable time not yet covered by a synced invoice, grouped by client.

### Summarize last month

```bash
clockify-pp-cli recap --range last-month
```

Ranked project/client/tag breakdown with the billable split for the prior month.

## Auth Setup

Authenticate with a personal API key from the Clockify web app (Profile Settings -> API). Export it as CLOCKIFY_API_KEY; the CLI sends it as the X-Api-Key header. The key is read-only-safe for listing and reporting and is required for any write.

Run `clockify-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clockify-pp-cli approval-requests get mock-value --agent --select id,name,status
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
clockify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clockify-pp-cli feedback --stdin < notes.txt
clockify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.clockify-pp-cli/feedback.jsonl`. They are never POSTed unless `CLOCKIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLOCKIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clockify-pp-cli profile save briefing --json
clockify-pp-cli --profile briefing approval-requests get mock-value
clockify-pp-cli profile list --json
clockify-pp-cli profile show briefing
clockify-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `clockify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add clockify-pp-mcp -- clockify-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which clockify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clockify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clockify-pp-cli <command> --help`.
