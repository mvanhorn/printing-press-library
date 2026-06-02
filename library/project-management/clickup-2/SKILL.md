---
name: pp-clickup-2
description: "Every ClickUp endpoint plus a local SQLite mirror with offline full-text search and cross-task analytics no other ClickUp CLI ships. Trigger phrases: `what's assigned to me in clickup`, `what changed on my clickup tasks`, `how long are tasks stuck in review`, `search my clickup workspace`, `who's overloaded on the team`, `use clickup`, `run clickup`."
author: "riccardovandra"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clickup-2-pp-cli
---

# ClickUp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clickup-2-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install clickup-2 --cli-only
   ```
2. Verify: `clickup-2-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/cmd/clickup-2-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

clickup-2-pp-cli matches the full ClickUp v2 API surface (tasks, lists, time tracking, goals, custom fields, webhooks and more) and adds what every other ClickUp tool lacks: a local store you can search offline and query with SQL. That local layer powers commands the web app charges for or cannot do at all, like time-in-status cycle analytics, assignee workload, and a 'what changed since yesterday' activity delta.

## When to Use This CLI

Use clickup-2-pp-cli when you manage ClickUp work from the terminal or from an agent and want more than a thin API mirror. It is the right choice when you need offline access to your tasks, full-text search across a synced workspace, or analytics like cycle time and workload that the ClickUp web app gates behind paid Dashboards. It is also the agent-friendly option: deterministic --json/--select output, --dry-run on mutations, and an offline resolver that turns fuzzy input into IDs without extra API calls.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`changed-since`** — See exactly what moved on your tasks since the last sync: status changes, new or removed assignees, and due-date shifts, grouped by task.

  _Reach for this at standup or after time off instead of scrolling each task's activity feed._

  ```bash
  clickup-2-pp-cli changed-since last --agent
  ```
- **`my-day`** — Your open tasks across every list, sorted by due date with overdue and stuck flags, served entirely from the local store with no network.

  _The first command of the morning; works on a plane._

  ```bash
  clickup-2-pp-cli my-day --due 7d --agent
  ```
- **`stale`** — Tasks with no update in N days, optionally filtered to a status like 'review', surfaced from the local store.

  _Find forgotten work before it rots, in one command._

  ```bash
  clickup-2-pp-cli stale --days 14 --status review --agent
  ```

### Analytics ClickUp charges for
- **`workload`** — Open-task count, summed time estimates, and active-timer status per team member across a space, so you can see who is overloaded before sprint planning.

  _Answers 'who is overloaded' without a paid Dashboard or a manual CSV pivot._

  ```bash
  clickup-2-pp-cli workload --space 90010 --agent
  ```
- **`time-in-status`** — How long tasks dwell in each status, per task or rolled up per list, computed from status-change history captured during sync.

  _Quantifies 'how long do tasks sit in review' that ClickUp only shows in paid Dashboards._

  ```bash
  clickup-2-pp-cli time-in-status 90010 --rank --agent
  ```

### Agent-native plumbing
- **`unblocked`** — Tasks whose blockers are all closed (or, with 'blocked', the open blockers holding a task up), computed by joining tasks with their dependencies.

  _Tells an agent or a person exactly what is actually ready to work on right now._

  ```bash
  clickup-2-pp-cli unblocked --list 90010 --agent
  ```
- **`resolve`** — Turn fuzzy human input (status name, assignee 'me' or a name, natural-language due date) into hard ClickUp IDs as JSON, with zero API calls.

  _Lets agents resolve names to IDs without burning rate-limit budget on every lookup._

  ```bash
  clickup-2-pp-cli resolve --status review --assignee me --due 'tomorrow 5pm' --agent
  ```

## Command Reference

**checklist** — Manage checklist

- `clickup-2-pp-cli checklist delete` — Delete a checklist from a task.
- `clickup-2-pp-cli checklist edit` — Rename a task checklist, or reorder a checklist so it appears above or below other checklists on a task.

**comment** — Manage comment

- `clickup-2-pp-cli comment delete` — Delete a task comment.
- `clickup-2-pp-cli comment update` — Replace the content of a task commment, assign a comment, and mark a comment as resolved.

**folder** — Manage folder

- `clickup-2-pp-cli folder delete` — Delete a Folder from your Workspace.
- `clickup-2-pp-cli folder get` — View the Lists within a Folder.
- `clickup-2-pp-cli folder update` — Rename a Folder.

**goal** — Manage goal

- `clickup-2-pp-cli goal delete` — Remove a Goal from your Workspace.
- `clickup-2-pp-cli goal get` — View the details of a Goal including its Targets.
- `clickup-2-pp-cli goal update` — Rename a Goal, set the due date, replace the description, add or remove owners, and set the Goal color.

**group** — Manage group

- `clickup-2-pp-cli group delete-team` — This endpoint is used to remove a [User Group](https://docs.clickup.
- `clickup-2-pp-cli group get-teams1` — This endpoint is used to view [User Groups](https://docs.clickup.
- `clickup-2-pp-cli group update-team` — This endpoint is used to manage [User Groups](https://docs.clickup.

**key-result** — Manage key result

- `clickup-2-pp-cli key-result delete` — Delete a target from a Goal.
- `clickup-2-pp-cli key-result edit` — Update a Target.

**list** — Manage list

- `clickup-2-pp-cli list delete` — Delete a List from your Workspace.
- `clickup-2-pp-cli list get` — View information about a List.
- `clickup-2-pp-cli list update` — Rename a List, update the List Info description, set a due date/time, set the List's priority, set an assignee

**oauth** — Manage oauth

- `clickup-2-pp-cli oauth` — These are the routes for authing the API and going through the [OAuth flow](doc:authentication).

**space** — Manage space

- `clickup-2-pp-cli space delete` — Delete a Space from your Workspace.
- `clickup-2-pp-cli space get` — View the Spaces available in a Workspace.
- `clickup-2-pp-cli space update` — Rename, set the Space color, and enable ClickApps for a Space.

**task** — Manage task

- `clickup-2-pp-cli task delete` — Delete a task from your Workspace.
- `clickup-2-pp-cli task get` — View information about a task. You can only view task information of tasks you can access.
- `clickup-2-pp-cli task get-bulk-timein-status` — View how long two or more tasks have been in each status.
- `clickup-2-pp-cli task update` — Update a task by including one or more fields in the request body.

**team** — Manage team

- `clickup-2-pp-cli team` — View the Workspaces available to the authenticated user.

**user** — Manage user

- `clickup-2-pp-cli user` — View the details of the authenticated user's ClickUp account.

**view** — Manage view

- `clickup-2-pp-cli view delete` — Delete View
- `clickup-2-pp-cli view get` — View information about a specific task or page view. The information returned about a view varies by the type of view.
- `clickup-2-pp-cli view update` — Rename a view, update the grouping, sorting, filters, columns, and settings of a view.

**webhook** — Manage webhook

- `clickup-2-pp-cli webhook delete` — Delete a webhook to stop monitoring the events and locations of the webhook.
- `clickup-2-pp-cli webhook update` — Update a webhook to change the events to be monitored.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clickup-2-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning triage

```bash
clickup-2-pp-cli my-day --due 3d --agent
```

Everything assigned to you due in three days, JSON for piping, served from the local store.

### Standup delta

```bash
clickup-2-pp-cli changed-since last --agent
```

What moved on your tasks since the last sync, grouped by changed field.

### Find the stuck work

```bash
clickup-2-pp-cli time-in-status 90010 --rank --agent
```

Per-status dwell time for a list, ranked worst-first, to spot bottlenecks.

### Narrow a verbose task payload

```bash
clickup-2-pp-cli task get 86abc --agent --select name,status.status,assignees.username,due_date
```

ClickUp task objects are tens of KB; dotted --select returns only the fields you need so agents do not burn context.

### Resolve fuzzy input for an agent

```bash
clickup-2-pp-cli resolve --status review --assignee me --due 'friday' --agent
```

Turns names and natural-language dates into ClickUp IDs offline, zero API calls.

## Auth Setup

Authenticate with a ClickUp personal API token (Settings > Apps > API Token, format pk_...). Set it once with `clickup-2-pp-cli auth set-token` or export CLICKUP_API_TOKEN. The token goes in the Authorization header with no Bearer prefix, matching ClickUp's scheme.

Run `clickup-2-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clickup-2-pp-cli folder get mock-value --agent --select id,name,status
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
clickup-2-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clickup-2-pp-cli feedback --stdin < notes.txt
clickup-2-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/clickup-2-pp-cli/feedback.jsonl`. They are never POSTed unless `CLICKUP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLICKUP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clickup-2-pp-cli profile save briefing --json
clickup-2-pp-cli --profile briefing folder get mock-value
clickup-2-pp-cli profile list --json
clickup-2-pp-cli profile show briefing
clickup-2-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `clickup-2-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add clickup-2-pp-mcp -- clickup-2-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which clickup-2-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clickup-2-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clickup-2-pp-cli <command> --help`.
