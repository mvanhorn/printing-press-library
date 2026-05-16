---
name: pp-harvest
description: "Every Harvest CLI feature, plus a local SQLite store and offline FTS no other tool has. Trigger phrases: `log my time`, `check timesheet gaps`, `project burn rate`, `client margin`, `search harvest notes`, `use harvest-pp`, `run harvest-pp`."
author: "Dan Bronson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - harvest-pp-cli
---

# Harvest — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `harvest-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install harvest --cli-only
   ```
2. Verify: `harvest-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every feature in hrvst-cli, hcl, and the MCP servers — for time entries, projects, invoices, and reports — plus offline full-text search over your notes, project burn projection, client margin, utilization trends, and timesheet-gap detection. All built on a synced local SQLite store so cron-driven scripts and AI agents can query without hammering the rate-limited API.

## When to Use This CLI

Reach for harvest-pp-cli when you need to query Harvest data programmatically — cron scripts that fan out across many entries, AI agents asking 'what did I do last week,' invoicing automations that join time entries with rates, or any task where the rate-limited API would be too slow. For interactive time-entry CRUD an agent could call the API directly; use this CLI when the local store, FTS, or cross-entity joins matter.

## Unique Capabilities

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

## Command Reference

**clients** — Manage clients

- `harvest-pp-cli clients create` — Creates a new client object. Returns a client object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli clients delete` — Delete a client. Deleting a client is only possible if it has no projects, invoices, or estimates associated with...
- `harvest-pp-cli clients list` — Returns a list of your clients. The clients are returned sorted by creation date, with the most recently created...
- `harvest-pp-cli clients retrieve` — Retrieves the client with the given ID. Returns a client object and a 200 OK response code if a valid identifier was...
- `harvest-pp-cli clients update` — Updates the specific client by setting the values of the parameters passed. Any parameters not provided will be left...

**company** — Manage company

- `harvest-pp-cli company retrieve` — Retrieves the company for the currently authenticated user. Returns a company object and a 200 OK response code.
- `harvest-pp-cli company update` — Updates the company setting the values of the parameters passed. Any parameters not provided will be left unchanged....

**contacts** — Manage contacts

- `harvest-pp-cli contacts create` — Creates a new contact object. Returns a contact object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli contacts delete` — Delete a contact. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli contacts list` — Returns a list of your contacts. The contacts are returned sorted by creation date, with the most recently created...
- `harvest-pp-cli contacts retrieve` — Retrieves the contact with the given ID. Returns a contact object and a 200 OK response code if a valid identifier...
- `harvest-pp-cli contacts update` — Updates the specific contact by setting the values of the parameters passed. Any parameters not provided will be...

**estimate-item-categories** — Manage estimate item categories

- `harvest-pp-cli estimate-item-categories create-estimate-item-category` — Creates a new estimate item category object. Returns an estimate item category object and a 201 Created response...
- `harvest-pp-cli estimate-item-categories delete-estimate-item-category` — Delete an estimate item category. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli estimate-item-categories list` — Returns a list of your estimate item categories. The estimate item categories are returned sorted by creation date,...
- `harvest-pp-cli estimate-item-categories retrieve-estimate-item-category` — Retrieves the estimate item category with the given ID. Returns an estimate item category object and a 200 OK...
- `harvest-pp-cli estimate-item-categories update-estimate-item-category` — Updates the specific estimate item category by setting the values of the parameters passed. Any parameters not...

**estimates** — Manage estimates

- `harvest-pp-cli estimates create` — Creates a new estimate object. Returns an estimate object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli estimates delete` — Delete an estimate. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli estimates list` — Returns a list of your estimates. The estimates are returned sorted by issue date, with the most recently issued...
- `harvest-pp-cli estimates retrieve` — Retrieves the estimate with the given ID. Returns an estimate object and a 200 OK response code if a valid...
- `harvest-pp-cli estimates update` — Updates the specific estimate by setting the values of the parameters passed. Any parameters not provided will be...

**expense-categories** — Manage expense categories

- `harvest-pp-cli expense-categories create-expense-category` — Creates a new expense category object. Returns an expense category object and a 201 Created response code if the...
- `harvest-pp-cli expense-categories delete-expense-category` — Delete an expense category. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli expense-categories list` — Returns a list of your expense categories. The expense categories are returned sorted by creation date, with the...
- `harvest-pp-cli expense-categories retrieve-expense-category` — Retrieves the expense category with the given ID. Returns an expense category object and a 200 OK response code if a...
- `harvest-pp-cli expense-categories update-expense-category` — Updates the specific expense category by setting the values of the parameters passed. Any parameters not provided...

**expenses** — Manage expenses

- `harvest-pp-cli expenses create` — Creates a new expense object. Returns an expense object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli expenses delete` — Delete an expense. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli expenses list` — Returns a list of your expenses. If accessing this endpoint as an Administrator, all expenses in the account will be...
- `harvest-pp-cli expenses retrieve` — Retrieves the expense with the given ID. Returns an expense object and a 200 OK response code if a valid identifier...
- `harvest-pp-cli expenses update` — Updates the specific expense by setting the values of the parameters passed. Any parameters not provided will be...

**invoice-item-categories** — Manage invoice item categories

- `harvest-pp-cli invoice-item-categories create-invoice-item-category` — Creates a new invoice item category object. Returns an invoice item category object and a 201 Created response code...
- `harvest-pp-cli invoice-item-categories delete-invoice-item-category` — Delete an invoice item category. Deleting an invoice item category is only possible if use_as_service and...
- `harvest-pp-cli invoice-item-categories list` — Returns a list of your invoice item categories. The invoice item categories are returned sorted by creation date,...
- `harvest-pp-cli invoice-item-categories retrieve-invoice-item-category` — Retrieves the invoice item category with the given ID. Returns an invoice item category object and a 200 OK response...
- `harvest-pp-cli invoice-item-categories update-invoice-item-category` — Updates the specific invoice item category by setting the values of the parameters passed. Any parameters not...

**invoices** — Manage invoices

- `harvest-pp-cli invoices create` — Creates a new invoice object. Returns an invoice object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli invoices delete` — Delete an invoice. Returns a 200 OK response code if the call succeeded.
- `harvest-pp-cli invoices list` — Returns a list of your invoices. The invoices are returned sorted by issue date, with the most recently issued...
- `harvest-pp-cli invoices retrieve` — Retrieves the invoice with the given ID. Returns an invoice object and a 200 OK response code if a valid identifier...
- `harvest-pp-cli invoices update` — Updates the specific invoice by setting the values of the parameters passed. Any parameters not provided will be...

**projects** — Manage projects

- `harvest-pp-cli projects create` — Creates a new project object. Returns a project object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli projects delete` — Deletes a project and any time entries or expenses tracked to it. However, invoices associated with the project will...
- `harvest-pp-cli projects list` — Returns a list of your projects. The projects are returned sorted by creation date, with the most recently created...
- `harvest-pp-cli projects retrieve` — Retrieves the project with the given ID. Returns a project object and a 200 OK response code if a valid identifier...
- `harvest-pp-cli projects update` — Updates the specific project by setting the values of the parameters passed. Any parameters not provided will be...

**reports** — Manage reports

- `harvest-pp-cli reports clients-expenses` — Clients Report
- `harvest-pp-cli reports clients-time` — Clients Report
- `harvest-pp-cli reports expense-categories` — Expense Categories Report
- `harvest-pp-cli reports project-budget` — The response contains an object with a results property that contains an array of up to per_page results. Each entry...
- `harvest-pp-cli reports projects-expenses` — Projects Report
- `harvest-pp-cli reports projects-time` — Projects Report
- `harvest-pp-cli reports tasks` — Tasks Report
- `harvest-pp-cli reports team-expenses` — Team Report
- `harvest-pp-cli reports team-time` — Team Report
- `harvest-pp-cli reports uninvoiced` — The response contains an object with a results property that contains an array of up to per_page results. Each entry...

**roles** — Manage roles

- `harvest-pp-cli roles create` — Creates a new role object. Returns a role object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli roles delete` — Delete a role. Deleting a role will unlink it from any users it was assigned to. Returns a 200 OK response code if...
- `harvest-pp-cli roles list` — Returns a list of roles in the account. The roles are returned sorted by creation date, with the most recently...
- `harvest-pp-cli roles retrieve` — Retrieves the role with the given ID. Returns a role object and a 200 OK response code if a valid identifier was...
- `harvest-pp-cli roles update` — Updates the specific role by setting the values of the parameters passed. Any parameters not provided will be left...

**task-assignments** — Manage task assignments

- `harvest-pp-cli task-assignments` — Returns a list of your task assignments. The task assignments are returned sorted by creation date, with the most...

**tasks** — Manage tasks

- `harvest-pp-cli tasks create` — Creates a new task object. Returns a task object and a 201 Created response code if the call succeeded.
- `harvest-pp-cli tasks delete` — Delete a task. Deleting a task is only possible if it has no time entries associated with it. Returns a 200 OK...
- `harvest-pp-cli tasks list` — Returns a list of your tasks. The tasks are returned sorted by creation date, with the most recently created tasks...
- `harvest-pp-cli tasks retrieve` — Retrieves the task with the given ID. Returns a task object and a 200 OK response code if a valid identifier was...
- `harvest-pp-cli tasks update` — Updates the specific task by setting the values of the parameters passed. Any parameters not provided will be left...

**time-entries** — Manage time entries

- `harvest-pp-cli time-entries create-time-entry` — Creates a new time entry object. Returns a time entry object and a 201 Created response code if the call succeeded....
- `harvest-pp-cli time-entries delete-time-entry` — Delete a time entry. Deleting a time entry is only possible if it’s not closed and the associated project and task...
- `harvest-pp-cli time-entries list` — Returns a list of time entries. The time entries are returned sorted by spent_date date. At this time, the sort...
- `harvest-pp-cli time-entries retrieve-time-entry` — Retrieves the time entry with the given ID. Returns a time entry object and a 200 OK response code if a valid...
- `harvest-pp-cli time-entries update-time-entry` — Updates the specific time entry by setting the values of the parameters passed. Any parameters not provided will be...

**user-assignments** — Manage user assignments

- `harvest-pp-cli user-assignments` — Returns a list of your projects user assignments, active and archived. The user assignments are returned sorted by...

**users** — Manage users

- `harvest-pp-cli users create` — Creates a new user object and sends an invitation email to the address specified in the email parameter. Returns a...
- `harvest-pp-cli users delete` — Delete a user. Deleting a user is only possible if they have no time entries or expenses associated with them....
- `harvest-pp-cli users list` — Returns a list of your users. The users are returned sorted by creation date, with the most recently created users...
- `harvest-pp-cli users list-active-project-assignments-for-the-currently-authenticated` — Returns a list of your active project assignments for the currently authenticated user. The project assignments are...
- `harvest-pp-cli users retrieve` — Retrieves the user with the given ID. Returns a user object and a 200 OK response code if a valid identifier was...
- `harvest-pp-cli users retrieve-the-currently-authenticated` — Retrieves the currently authenticated user. Returns a user object and a 200 OK response code.
- `harvest-pp-cli users update` — Updates the specific user by setting the values of the parameters passed. Any parameters not provided will be left...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
harvest-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Friday timesheet chase

```bash
harvest-pp-cli timesheet gaps --from "$(date -v-Mon +%Y-%m-%d)" --to "$(date +%Y-%m-%d)" --min-hours 6 --json --select user_name,date,total_hours
```

Run on Friday to surface every team member with gaps for the current week, in agent-friendly JSON.

### Project burn alert (cron)

```bash
harvest-pp-cli project burn --threshold 80 --json --select project,client,pct_used,projected_exhaust_date || echo 'project budget breach'
```

Cron-friendly — exit 2 when any active project crosses 80% used. Pipe to Slack or PagerDuty.

### Find that auth-refactor entry

```bash
harvest-pp-cli notes search 'auth refactor' --mine --from 2026-01-01 --json --select id,spent_date,hours,project,notes
```

FTS5 over your local notes — finds entries without paginating the API.

### Recurring weekly work

```bash
harvest-pp-cli time-entries repeat 12345 --days 5 --dry-run --json
```

Dry-run first to see what would be posted, then commit. Idempotent — re-running skips dates already filled.

### Month-end realization check (deep --select)

```bash
harvest-pp-cli client margin --client 'Acme Corp' --from 2026-04-01 --to 2026-04-30 --json --select client.name,revenue,cost,realization_pct,hours_billable,hours_non_billable
```

Joins time entries × rates × client locally. Use the dotted `--select` paths to pull only the fields your dashboard needs.

## Auth Setup

Harvest uses Personal Access Tokens (PATs). Set `HARVEST_ACCESS_TOKEN` and `HARVEST_ACCOUNT_ID` in your environment, or run `harvest-pp-cli auth set-token` to store them locally. `harvest-pp-cli doctor` validates the token and surfaces the authenticated user.

Run `harvest-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  harvest-pp-cli clients list --agent --select id,name,status
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
harvest-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
harvest-pp-cli feedback --stdin < notes.txt
harvest-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.harvest-pp-cli/feedback.jsonl`. They are never POSTed unless `HARVEST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HARVEST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
harvest-pp-cli profile save briefing --json
harvest-pp-cli --profile briefing clients list
harvest-pp-cli profile list --json
harvest-pp-cli profile show briefing
harvest-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `harvest-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add harvest-pp-mcp -- harvest-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which harvest-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   harvest-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `harvest-pp-cli <command> --help`.
