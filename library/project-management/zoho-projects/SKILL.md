---
name: pp-zoho-projects
description: "Every Zoho Projects API endpoint, plus portal-wide insights (overdue, stale, workload, issue heat) no native view... Trigger phrases: `what's overdue in zoho projects`, `show me stale projects`, `team workload zoho projects`, `my zoho projects tasks today`, `search zoho projects`, `use zoho-projects-pp`, `run zoho-projects-pp`."
author: "Dan Bronson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zoho-projects-pp-cli
---

# Zoho Projects — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zoho-projects-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zoho-projects --cli-only
   ```
2. Verify: `zoho-projects-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Full coverage of Zoho Projects portals, projects, tasks, task-lists, phases, issues, and users — plus FTS5 over locally-synced content, portal-wide overdue/stale detection, workload fairness, project burn projection, and unassigned-items radar. Built on a synced local SQLite store so PMs and team leads can plan without clicking through every project board.

## When to Use This CLI

Reach for zoho-projects-pp-cli when you need to plan or report across projects — portal-wide overdue/stale/workload views, cross-project search, project burn projection, and unassigned-items cleanup. For one-off CRUD operations on a single ticket or task, the API or MCP is fine; use this CLI when local cache, FTS, or cross-project joins matter.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`overdue`** — List tasks whose end_date is in the past but status isn't 'closed'/'completed'.

  _Use this for daily standup — who's behind on which projects, in one shot._

  ```bash
  zoho-projects-pp-cli overdue --json --select id,name,project,owner,end_date,days_overdue
  ```
- **`stale-projects`** — Active projects with no task activity in N days.

  _Use this in weekly portfolio review to find projects that have lost momentum._

  ```bash
  zoho-projects-pp-cli stale-projects --days 14 --json
  ```
- **`workload`** — Open tasks + issues per user with mean/stddev/Gini across the portal.

  _Use this for sprint planning — surfaces who's stretched across multiple projects._

  ```bash
  zoho-projects-pp-cli workload --sort spread --json
  ```
- **`grep`** — Search task names/descriptions, issue titles/descriptions, project names/descriptions locally.

  _Use this to recall the right ticket/task without paginating per-module search endpoints._

  ```bash
  zoho-projects-pp-cli grep 'payment gateway' --in tasks,issues --json
  ```
- **`project-burn`** — Per-project completed-task ratio with projected completion date based on 4-week velocity.

  _Use this to call out projects likely to slip end-dates before the customer notices._

  ```bash
  zoho-projects-pp-cli project-burn --json --select project,pct_complete,velocity,projected_end_date
  ```

### Agent-native plumbing
- **`issue-heat`** — Per-project open-issue counts grouped by severity, surfacing projects with concentrated bugs.

  _Use this in monthly QA review to find projects accumulating critical issues._

  ```bash
  zoho-projects-pp-cli issue-heat --json --select project,critical,high,medium,low
  ```
- **`unassigned`** — Open tasks and issues with no owner across active projects.

  _Use this Friday afternoon to clean up orphaned work before the weekend._

  ```bash
  zoho-projects-pp-cli unassigned --json
  ```
- **`my-focus`** — Tasks + issues assigned to the authenticated user across all portals, ranked by due date.

  _Use this every morning to plan the day without clicking through every project board._

  ```bash
  zoho-projects-pp-cli my-focus --limit 10 --json
  ```

## Command Reference

**issues** — Manage issues

- `zoho-projects-pp-cli issues list-my` — List issues assigned to the authenticated user
- `zoho-projects-pp-cli issues list-portal` — List issues across the portal

**portal** — Manage portal

- `zoho-projects-pp-cli portal <portal_id>` — Get portal details

**portals** — Manage portals

- `zoho-projects-pp-cli portals` — Lists all portals the authenticated user has access to.

**projects** — Manage projects

- `zoho-projects-pp-cli projects create` — Create a project
- `zoho-projects-pp-cli projects delete` — Move a project to trash
- `zoho-projects-pp-cli projects get` — Get a project
- `zoho-projects-pp-cli projects list` — List projects in a portal
- `zoho-projects-pp-cli projects update` — Update a project

**tasks** — Manage tasks

- `zoho-projects-pp-cli tasks <portal_id>` — List tasks across all portal projects

**users** — Manage users

- `zoho-projects-pp-cli users <portal_id>` — List users in the portal


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zoho-projects-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning focus

```bash
zoho-projects-pp-cli my-focus --limit 10 --json --select id,name,kind,project,end_date,priority
```

Top 10 items assigned to you, ranked by due date — across every portal you have access to.

### Portal overdue report

```bash
zoho-projects-pp-cli overdue --json --select project,owner,name,end_date,days_overdue
```

Every task/issue past its end_date that isn't closed. Pipe into Slack.

### Workload check (deep --select)

```bash
zoho-projects-pp-cli workload --sort spread --json --select owner,open_tasks,open_issues,total_open,share_pct
```

Per-user open-work distribution — surfaces who's overcommitted.

### Stale project audit

```bash
zoho-projects-pp-cli stale-projects --days 14 --json --select id,name,owner,days_inactive,task_count
```

Active projects with no task activity in 2 weeks.

### Cross-module FTS

```bash
zoho-projects-pp-cli grep 'payment gateway' --in tasks,issues --json --select id,kind,name,project
```

FTS5 across task + issue content — finds related work no native search returns.

## Auth Setup

Zoho Projects uses OAuth 2.0. You need a self-client refresh token, client ID/secret, and your portal ID. Set ZOHOPROJECTS_REFRESH_TOKEN, ZOHOPROJECTS_CLIENT_ID, ZOHOPROJECTS_CLIENT_SECRET, ZOHOPROJECTS_PORTAL_ID — or run `zoho-projects-pp-cli auth set-token` to store them. The CLI auto-refreshes access tokens.

Run `zoho-projects-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zoho-projects-pp-cli portal mock-value --agent --select id,name,status
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
zoho-projects-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zoho-projects-pp-cli feedback --stdin < notes.txt
zoho-projects-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zoho-projects-pp-cli/feedback.jsonl`. They are never POSTed unless `ZOHO_PROJECTS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZOHO_PROJECTS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zoho-projects-pp-cli profile save briefing --json
zoho-projects-pp-cli --profile briefing portal mock-value
zoho-projects-pp-cli profile list --json
zoho-projects-pp-cli profile show briefing
zoho-projects-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zoho-projects-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zoho-projects-pp-mcp -- zoho-projects-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zoho-projects-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zoho-projects-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zoho-projects-pp-cli <command> --help`.
