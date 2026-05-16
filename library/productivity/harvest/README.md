# Harvest CLI

**Every Harvest CLI feature, plus a local SQLite store and offline FTS no other tool has.**

Every feature in hrvst-cli, hcl, and the MCP servers — for time entries, projects, invoices, and reports — plus offline full-text search over your notes, project burn projection, client margin, utilization trends, and timesheet-gap detection. All built on a synced local SQLite store so cron-driven scripts and AI agents can query without hammering the rate-limited API.

Learn more at [Harvest](https://help.getharvest.com/api-v2/).

## Install

The recommended path installs both the `harvest-pp-cli` binary and the `pp-harvest` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install harvest
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install harvest --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/harvest-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-harvest --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-harvest --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-harvest skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-harvest. The skill defines how its required CLI can be installed.
```

## Authentication

Harvest uses Personal Access Tokens (PATs). Set `HARVEST_ACCESS_TOKEN` and `HARVEST_ACCOUNT_ID` in your environment, or run `harvest-pp-cli auth set-token` to store them locally. `harvest-pp-cli doctor` validates the token and surfaces the authenticated user.

## Quick Start

```bash
# Confirms HARVEST_ACCESS_TOKEN + HARVEST_ACCOUNT_ID are set and reachable.
harvest-pp-cli auth status


# Populates the local SQLite store with all time entries, projects, clients, users, tasks.
harvest-pp-cli sync --full


# Sanity check that the right account is wired up.
harvest-pp-cli users me --json --select first_name,timezone,weekly_capacity


# Demo of the offline FTS feature.
harvest-pp-cli notes search 'auth refactor' --mine --json --select id,spent_date,hours


# List active projects burning >80% of budget — exit 2 if any.
harvest-pp-cli project burn --threshold 80 --json


# Friday chase: who has gaps this week.
harvest-pp-cli timesheet gaps --from 2026-05-11 --to 2026-05-15 --min-hours 6 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`project burn`** — See which projects have burned >80% of budget with a projected exhaust date based on 4-week velocity.

  _Use this to answer 'which projects are about to blow their budget?' in one call — exit code 2 makes it cron-friendly._

  ```bash
  harvest-pp-cli project burn --threshold 80 --projection --json
  ```
- **`notes search`** — FTS5 search across all your time-entry notes, with filters for user/project/date.

  _Use this to recall what you logged without re-paginating the API or scrolling Harvest's web UI._

  ```bash
  harvest-pp-cli notes search 'auth refactor' --mine --from 2026-01-01 --json --select id,spent_date,hours,notes
  ```
- **`client margin`** — Per-client revenue (billable hours × billable_rate) minus cost (hours × cost_rate), with realization %.

  _Use this to answer 'is client X profitable?' without exporting to a spreadsheet._

  ```bash
  harvest-pp-cli client margin --client 'Acme Corp' --from 2026-04-01 --to 2026-04-30 --json
  ```
- **`utilization`** — Billable percentage per user over a range, with rolling 4-week trend.

  _Use this to monitor team utilization trends without rebuilding Google Sheets pivots._

  ```bash
  harvest-pp-cli utilization --weeks 12 --json --select user,week,billable_pct
  ```

### Agent-native plumbing
- **`timesheet gaps`** — List workdays where a user logged less than a threshold (default 6 hours), to chase missing time.

  _Use this on Friday to chase missing entries without manual CSV exports._

  ```bash
  harvest-pp-cli timesheet gaps --from 2026-05-11 --to 2026-05-15 --min-hours 6 --json
  ```
- **`day reconstruct`** — For a given user+date, emit JSON stub entries that fill the day to target hours, proportional to that user's recent project mix.

  _Use this for Friday backfill — generate plausible stubs, edit notes, then commit._

  ```bash
  harvest-pp-cli day reconstruct --user me --date 2026-05-14 --target-hours 8 --json | harvest-pp-cli time-entries create --stdin --dry-run
  ```
- **`time-entries repeat`** — Repost an existing time entry to a new date or N consecutive workdays with the same project/task/hours/notes.

  _Use this for recurring work (standup, planning) — one command per week instead of typing the same flags daily._

  ```bash
  harvest-pp-cli time-entries repeat 12345 --days 5 --dry-run --json
  ```
- **`reconcile`** — Diff local SQLite vs the Harvest API for a date range; surface rows that drifted (edits made after the last sync).

  _Use this nightly to detect retroactive edits and trigger re-aggregation for stale dashboards._

  ```bash
  harvest-pp-cli reconcile --from 2026-05-01 --to 2026-05-15 --json
  ```

## Usage

Run `harvest-pp-cli --help` for the full command reference and flag list.

## Commands

### clients

Manage clients

- **`harvest-pp-cli clients create`** - Creates a new client object. Returns a client object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli clients delete`** - Delete a client. Deleting a client is only possible if it has no projects, invoices, or estimates associated with it. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli clients list`** - Returns a list of your clients. The clients are returned sorted by creation date, with the most recently created clients appearing first.

The response contains an object with a clients property that contains an array of up to per_page clients. Each entry in the array is a separate client object. If no more clients are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your clients.
- **`harvest-pp-cli clients retrieve`** - Retrieves the client with the given ID. Returns a client object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli clients update`** - Updates the specific client by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a client object and a 200 OK response code if the call succeeded.

### company

Manage company

- **`harvest-pp-cli company retrieve`** - Retrieves the company for the currently authenticated user. Returns a
company object and a 200 OK response code.
- **`harvest-pp-cli company update`** - Updates the company setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a company object and a 200 OK response code if the call succeeded.

### contacts

Manage contacts

- **`harvest-pp-cli contacts create`** - Creates a new contact object. Returns a contact object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli contacts delete`** - Delete a contact. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli contacts list`** - Returns a list of your contacts. The contacts are returned sorted by creation date, with the most recently created contacts appearing first.

The response contains an object with a contacts property that contains an array of up to per_page contacts. Each entry in the array is a separate contact object. If no more contacts are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your contacts.
- **`harvest-pp-cli contacts retrieve`** - Retrieves the contact with the given ID. Returns a contact object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli contacts update`** - Updates the specific contact by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a contact object and a 200 OK response code if the call succeeded.

### estimate-item-categories

Manage estimate item categories

- **`harvest-pp-cli estimate-item-categories create-estimate-item-category`** - Creates a new estimate item category object. Returns an estimate item category object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli estimate-item-categories delete-estimate-item-category`** - Delete an estimate item category. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli estimate-item-categories list`** - Returns a list of your estimate item categories. The estimate item categories are returned sorted by creation date, with the most recently created estimate item categories appearing first.

The response contains an object with a estimate_item_categories property that contains an array of up to per_page estimate item categories. Each entry in the array is a separate estimate item category object. If no more estimate item categories are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your estimate item categories.
- **`harvest-pp-cli estimate-item-categories retrieve-estimate-item-category`** - Retrieves the estimate item category with the given ID. Returns an estimate item category object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli estimate-item-categories update-estimate-item-category`** - Updates the specific estimate item category by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an estimate item category object and a 200 OK response code if the call succeeded.

### estimates

Manage estimates

- **`harvest-pp-cli estimates create`** - Creates a new estimate object. Returns an estimate object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli estimates delete`** - Delete an estimate. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli estimates list`** - Returns a list of your estimates. The estimates are returned sorted by issue date, with the most recently issued estimates appearing first.

The response contains an object with a estimates property that contains an array of up to per_page estimates. Each entry in the array is a separate estimate object. If no more estimates are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your estimates.
- **`harvest-pp-cli estimates retrieve`** - Retrieves the estimate with the given ID. Returns an estimate object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli estimates update`** - Updates the specific estimate by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an estimate object and a 200 OK response code if the call succeeded.

### expense-categories

Manage expense categories

- **`harvest-pp-cli expense-categories create-expense-category`** - Creates a new expense category object. Returns an expense category object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli expense-categories delete-expense-category`** - Delete an expense category. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli expense-categories list`** - Returns a list of your expense categories. The expense categories are returned sorted by creation date, with the most recently created expense categories appearing first.

The response contains an object with a expense_categories property that contains an array of up to per_page expense categories. Each entry in the array is a separate expense category object. If no more expense categories are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your expense categories.
- **`harvest-pp-cli expense-categories retrieve-expense-category`** - Retrieves the expense category with the given ID. Returns an expense category object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli expense-categories update-expense-category`** - Updates the specific expense category by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an expense category object and a 200 OK response code if the call succeeded.

### expenses

Manage expenses

- **`harvest-pp-cli expenses create`** - Creates a new expense object. Returns an expense object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli expenses delete`** - Delete an expense. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli expenses list`** - Returns a list of your expenses. If accessing this endpoint as an Administrator, all expenses in the account will be returned. If accessing this endpoint as a Manager, all expenses for assigned teammates and managed projects will be returned. The expenses are returned sorted by the spent_at date, with the most recent expenses appearing first.

The response contains an object with a expenses property that contains an array of up to per_page expenses. Each entry in the array is a separate expense object. If no more expenses are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your expenses.
- **`harvest-pp-cli expenses retrieve`** - Retrieves the expense with the given ID. Returns an expense object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli expenses update`** - Updates the specific expense by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an expense object and a 200 OK response code if the call succeeded.

Note that changes to project_id and expense_category_id will be silently dropped if the expense is locked. Users with sufficient permissions are able to update the rest of a locked expense’s attributes.

### invoice-item-categories

Manage invoice item categories

- **`harvest-pp-cli invoice-item-categories create-invoice-item-category`** - Creates a new invoice item category object. Returns an invoice item category object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli invoice-item-categories delete-invoice-item-category`** - Delete an invoice item category. Deleting an invoice item category is only possible if use_as_service and use_as_expense are both false. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli invoice-item-categories list`** - Returns a list of your invoice item categories. The invoice item categories are returned sorted by creation date, with the most recently created invoice item categories appearing first.

The response contains an object with a invoice_item_categories property that contains an array of up to per_page invoice item categories. Each entry in the array is a separate invoice item category object. If no more invoice item categories are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your invoice item categories.
- **`harvest-pp-cli invoice-item-categories retrieve-invoice-item-category`** - Retrieves the invoice item category with the given ID. Returns an invoice item category object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli invoice-item-categories update-invoice-item-category`** - Updates the specific invoice item category by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an invoice item category object and a 200 OK response code if the call succeeded.

### invoices

Manage invoices

- **`harvest-pp-cli invoices create`** - Creates a new invoice object. Returns an invoice object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli invoices delete`** - Delete an invoice. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli invoices list`** - Returns a list of your invoices. The invoices are returned sorted by issue date, with the most recently issued invoices appearing first.

The response contains an object with a invoices property that contains an array of up to per_page invoices. Each entry in the array is a separate invoice object. If no more invoices are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your invoices.
- **`harvest-pp-cli invoices retrieve`** - Retrieves the invoice with the given ID. Returns an invoice object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli invoices update`** - Updates the specific invoice by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns an invoice object and a 200 OK response code if the call succeeded.

### projects

Manage projects

- **`harvest-pp-cli projects create`** - Creates a new project object. Returns a project object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli projects delete`** - Deletes a project and any time entries or expenses tracked to it.
However, invoices associated with the project will not be deleted.
If you don’t want the project’s time entries and expenses to be deleted, you should archive the project instead.
- **`harvest-pp-cli projects list`** - Returns a list of your projects. The projects are returned sorted by creation date, with the most recently created projects appearing first.

The response contains an object with a projects property that contains an array of up to per_page projects. Each entry in the array is a separate project object. If no more projects are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your projects.
- **`harvest-pp-cli projects retrieve`** - Retrieves the project with the given ID. Returns a project object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli projects update`** - Updates the specific project by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a project object and a 200 OK response code if the call succeeded.

### reports

Manage reports

- **`harvest-pp-cli reports clients-expenses`** - Clients Report
- **`harvest-pp-cli reports clients-time`** - Clients Report
- **`harvest-pp-cli reports expense-categories`** - Expense Categories Report
- **`harvest-pp-cli reports project-budget`** - The response contains an object with a results property that contains an array of up to per_page results. Each entry in the array is a separate result object. If no more results are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your results.
- **`harvest-pp-cli reports projects-expenses`** - Projects Report
- **`harvest-pp-cli reports projects-time`** - Projects Report
- **`harvest-pp-cli reports tasks`** - Tasks Report
- **`harvest-pp-cli reports team-expenses`** - Team Report
- **`harvest-pp-cli reports team-time`** - Team Report
- **`harvest-pp-cli reports uninvoiced`** - The response contains an object with a results property that contains an array of up to per_page results. Each entry in the array is a separate result object. If no more results are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your results.

Note: Each request requires both the from and to parameters to be supplied in the URL’s query string. The timeframe supplied cannot exceed 1 year (365 days).

### roles

Manage roles

- **`harvest-pp-cli roles create`** - Creates a new role object. Returns a role object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli roles delete`** - Delete a role. Deleting a role will unlink it from any users it was assigned to. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli roles list`** - Returns a list of roles in the account. The roles are returned sorted by creation date, with the most recently created roles appearing first.

The response contains an object with a roles property that contains an array of up to per_page roles. Each entry in the array is a separate role object. If no more roles are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your roles.
- **`harvest-pp-cli roles retrieve`** - Retrieves the role with the given ID. Returns a role object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli roles update`** - Updates the specific role by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a role object and a 200 OK response code if the call succeeded.

### task-assignments

Manage task assignments

- **`harvest-pp-cli task-assignments list`** - Returns a list of your task assignments. The task assignments are returned sorted by creation date, with the most recently created task assignments appearing first.

The response contains an object with a task_assignments property that contains an array of up to per_page task assignments. Each entry in the array is a separate task assignment object. If no more task assignments are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your task assignments.

### tasks

Manage tasks

- **`harvest-pp-cli tasks create`** - Creates a new task object. Returns a task object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli tasks delete`** - Delete a task. Deleting a task is only possible if it has no time entries associated with it. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli tasks list`** - Returns a list of your tasks. The tasks are returned sorted by creation date, with the most recently created tasks appearing first.

The response contains an object with a tasks property that contains an array of up to per_page tasks. Each entry in the array is a separate task object. If no more tasks are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your tasks.
- **`harvest-pp-cli tasks retrieve`** - Retrieves the task with the given ID. Returns a task object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli tasks update`** - Updates the specific task by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a task object and a 200 OK response code if the call succeeded.

### time-entries

Manage time entries

- **`harvest-pp-cli time-entries create-time-entry`** - Creates a new time entry object. Returns a time entry object and a 201 Created response code if the call succeeded.

You should only use this method to create time entries when your account is configured to track time via duration. You can verify this by visiting the Settings page in your Harvest account or by checking if wants_timestamp_timers is false in the Company API.
- **`harvest-pp-cli time-entries delete-time-entry`** - Delete a time entry. Deleting a time entry is only possible if it’s not closed and the associated project and task haven’t been archived.  However, Admins can delete closed entries. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli time-entries list`** - Returns a list of time entries. The time entries are returned sorted by spent_date date. At this time, the sort option can’t be customized.

The response contains an object with a time_entries property that contains an array of up to per_page time entries. Each entry in the array is a separate time entry object. If no more time entries are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your time entries.
- **`harvest-pp-cli time-entries retrieve-time-entry`** - Retrieves the time entry with the given ID. Returns a time entry object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli time-entries update-time-entry`** - Updates the specific time entry by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a time entry object and a 200 OK response code if the call succeeded.

### user-assignments

Manage user assignments

- **`harvest-pp-cli user-assignments list`** - Returns a list of your projects user assignments, active and archived. The user assignments are returned sorted by creation date, with the most recently created user assignments appearing first.

The response contains an object with a user_assignments property that contains an array of up to per_page user assignments. Each entry in the array is a separate user assignment object. If no more user assignments are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your user assignments.

### users

Manage users

- **`harvest-pp-cli users create`** - Creates a new user object and sends an invitation email to the address specified in the email parameter. Returns a user object and a 201 Created response code if the call succeeded.
- **`harvest-pp-cli users delete`** - Delete a user. Deleting a user is only possible if they have no time entries or expenses associated with them. Returns a 200 OK response code if the call succeeded.
- **`harvest-pp-cli users list`** - Returns a list of your users. The users are returned sorted by creation date, with the most recently created users appearing first.

The response contains an object with a users property that contains an array of up to per_page users. Each entry in the array is a separate user object. If no more users are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your users.
- **`harvest-pp-cli users list-active-project-assignments-for-the-currently-authenticated`** - Returns a list of your active project assignments for the currently authenticated user. The project assignments are returned sorted by creation date, with the most recently created project assignments appearing first.

The response contains an object with a project_assignments property that contains an array of up to per_page project assignments. Each entry in the array is a separate project assignment object. If no more project assignments are available, the resulting array will be empty. Several additional pagination properties are included in the response to simplify paginating your project assignments.
- **`harvest-pp-cli users retrieve`** - Retrieves the user with the given ID. Returns a user object and a 200 OK response code if a valid identifier was provided.
- **`harvest-pp-cli users retrieve-the-currently-authenticated`** - Retrieves the currently authenticated user. Returns a user object and a 200 OK response code.
- **`harvest-pp-cli users update`** - Updates the specific user by setting the values of the parameters passed. Any parameters not provided will be left unchanged. Returns a user object and a 200 OK response code if the call succeeded.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
harvest-pp-cli clients list

# JSON for scripting and agents
harvest-pp-cli clients list --json

# Filter to specific fields
harvest-pp-cli clients list --json --select id,name,status

# Dry run — show the request without sending
harvest-pp-cli clients list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
harvest-pp-cli clients list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-harvest -g
```

Then invoke `/pp-harvest <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add harvest harvest-pp-mcp -e HARVEST_ACCOUNT_AUTH=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/harvest-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HARVEST_ACCOUNT_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "harvest": {
      "command": "harvest-pp-mcp",
      "env": {
        "HARVEST_ACCOUNT_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
harvest-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/harvest-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HARVEST_ACCOUNT_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `harvest-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HARVEST_ACCOUNT_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Check HARVEST_ACCESS_TOKEN is set (`harvest-pp-cli auth status`). Tokens are personal-access-tokens, NOT OAuth bearer tokens — get one at https://id.getharvest.com/developers.
- **404 Not Found on a known-good account** — Confirm HARVEST_ACCOUNT_ID matches the account that owns the PAT. Account IDs are numeric; tokens are per-account.
- **429 Too Many Requests during sync** — Harvest limits to 100 requests / 15-second window. The CLI retries with backoff; for large backfills add `--rate-limit-window 15s` or run during off-hours.
- **sync --full takes forever** — Use `sync --since YYYY-MM-DD` or `sync --resources time-entries` for targeted refreshes. The full sync hits every endpoint with `updated_since` cursors.
- **Notes search returns nothing** — FTS index is populated by `sync`. Run `sync --resources time-entries` first; the SQLite FTS5 index lives in your local DB.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ianaleck/harvest-mcp-server**](https://github.com/ianaleck/harvest-mcp-server) — TypeScript
- [**taiste/harvest-mcp-server**](https://github.com/taiste/harvest-mcp-server) — TypeScript
- [**standardbeagle/harvest-mcp**](https://github.com/standardbeagle/harvest-mcp) — TypeScript
- [**kgajera/hrvst-cli**](https://github.com/kgajera/hrvst-cli) — TypeScript
- [**lucasconstantino/harvest-cli**](https://github.com/lucasconstantino/harvest-cli) — JavaScript
- [**droath/harvest-toolkit**](https://github.com/droath/harvest-toolkit) — PHP
- [**zenhob/hcl**](https://github.com/zenhob/hcl) — Ruby
- [**simplyspoke/node-harvest**](https://github.com/simplyspoke/node-harvest) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
