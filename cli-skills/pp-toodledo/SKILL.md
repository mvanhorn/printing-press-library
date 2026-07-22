---
name: pp-toodledo
description: "Every Toodledo feature from the terminal, plus an offline SQLite mirror, GTD reviews Trigger phrases: `what are my next actions`, `run my weekly review`, `add a task to toodledo`, `what's overdue in toodledo`, `capture these tasks`, `use toodledo`, `run toodledo`."
author: "wwilson1017"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - toodledo-pp-cli
    install:
      - kind: go
        bins: [toodledo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/toodledo/cmd/toodledo-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/toodledo/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Toodledo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `toodledo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install toodledo --cli-only
   ```
2. Verify: `toodledo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/toodledo/cmd/toodledo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Toodledo caps you at 100 API calls per access token, so a naive wrapper is unusable. toodledo-pp-cli mirrors your whole task universe into local SQLite, then runs GTD next-actions, weekly review, stalled-project detection, goal rollups, and full-text search entirely offline — with JSON on every command and a complete agent-native MCP surface. sync-cost tells you what a refresh will spend before you spend it.

## When to Use This CLI

Reach for this CLI when an agent or script needs to read or mutate a Toodledo GTD system offline and cheaply: listing next actions, running a weekly review, capturing tasks in bulk, rolling up goal progress, or querying task data with SQL. The local SQLite mirror means most reads cost zero API calls, which matters because Toodledo caps you at 100 calls per access token. It is also the right tool when you want JSON-shaped task data an agent can act on directly.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for other task managers (Todoist, TickTick, Asana, Things) — it speaks only the Toodledo v3 API.
- Do not use it to mass-rewrite or mass-delete a user's task history without explicit confirmation; writes hit the live Toodledo account.
- Do not use it for real-time collaboration, sharing, or calendar features the Toodledo API does not expose.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### GTD rituals, offline
- **`next-actions`** — Your GTD 'what should I do now?' list — incomplete Next-Action tasks, sorted by priority then due date, optionally scoped to a context or goal.

  _When an agent needs the single best next action, this answers offline in one call instead of refetching and re-filtering every task._

  ```bash
  toodledo-pp-cli next-actions --context @work --agent
  ```
- **`review`** — The full GTD weekly review in one pass: inbox (untriaged), overdue, stalled projects, waiting-for, and someday/maybe.

  _Hands an agent the entire weekly-review state in one offline payload so it can drive the Sunday-night ritual without rate-limit risk._

  ```bash
  toodledo-pp-cli review --agent
  ```
- **`stalled-projects`** — Folders (projects) that have open tasks but zero Next Actions — the GTD failure mode that silently stalls progress.

  _Surfaces the highest-value weekly-review bucket on its own so an agent can prompt the user to define a next action._

  ```bash
  toodledo-pp-cli stalled-projects --json
  ```
- **`goal-progress`** — Per-goal counts of completed vs incomplete contributing tasks, rolled up the lifetime/long-term/short-term goal hierarchy.

  _Lets an agent report whether a user's goals are actually being advanced by their day-to-day tasks._

  ```bash
  toodledo-pp-cli goal-progress --level short --json
  ```
- **`dashboard`** — A one-screen status board: incomplete task counts by status, priority, folder, and context, plus overdue, due-today, and starred totals.

  _Gives an agent the whole task-system state at a glance without N separate grouped queries._

  ```bash
  toodledo-pp-cli dashboard --json
  ```

### Rate-budget-aware
- **`sync-cost`** — Forecasts how many of your 100 per-token API calls an incremental sync would spend, before fetching any rows.

  _Toodledo locks you out after 100 calls per token; this tells an agent whether a sync is safe to run before spending the budget._

  ```bash
  toodledo-pp-cli sync-cost --since 7d
  ```
- **`capture`** — Add many tasks from a file or stdin (one title per line), resolving folder/context names to ids, in budget-aware batches of 50.

  _Turns a pile of captured ideas into Toodledo tasks in a handful of calls rather than dozens, without exhausting the token budget._

  ```bash
  toodledo-pp-cli capture --file ~/inbox.txt --folder Inbox
  ```

## Command Reference

**account** — Account info (subscription, sync cursors)

- `toodledo-pp-cli account` — Get account info, including per-resource lastedit/lastdelete sync cursors and Pro status

**contexts** — Contexts (GTD contexts like @home, @work)

- `toodledo-pp-cli contexts add` — Create a context
- `toodledo-pp-cli contexts delete` — Delete a context (its tasks become unassigned)
- `toodledo-pp-cli contexts edit` — Rename a context
- `toodledo-pp-cli contexts list` — List all contexts

**folders** — Folders (GTD projects)

- `toodledo-pp-cli folders add` — Create a folder
- `toodledo-pp-cli folders delete` — Delete a folder (its tasks become unassigned)
- `toodledo-pp-cli folders edit` — Edit/rename/archive a folder
- `toodledo-pp-cli folders list` — List all folders

**goals** — Goals (lifetime / long-term / short-term)

- `toodledo-pp-cli goals add` — Create a goal
- `toodledo-pp-cli goals delete` — Delete a goal
- `toodledo-pp-cli goals edit` — Edit a goal
- `toodledo-pp-cli goals list` — List all goals

**lists** — Custom lists (user-defined tabular lists)

- `toodledo-pp-cli lists add` — Create custom list(s). Pass a JSON array of list objects.
- `toodledo-pp-cli lists delete` — Delete custom list(s). Pass a JSON array of list ids.
- `toodledo-pp-cli lists deleted` — List custom-list ids deleted after a timestamp
- `toodledo-pp-cli lists edit` — Edit custom list(s). Pass a JSON array of list objects including id.
- `toodledo-pp-cli lists list` — List custom lists. Feeds the local mirror via sync.

**locations** — Locations (named places with coordinates)

- `toodledo-pp-cli locations add` — Create a location
- `toodledo-pp-cli locations delete` — Delete a location
- `toodledo-pp-cli locations edit` — Edit a location
- `toodledo-pp-cli locations list` — List all locations

**notes** — Notes (standalone notes, optionally filed in folders)

- `toodledo-pp-cli notes add` — Create note(s). Pass a JSON array of note objects.
- `toodledo-pp-cli notes delete` — Delete note(s). Pass a JSON array of note ids.
- `toodledo-pp-cli notes deleted` — List note ids deleted after a timestamp
- `toodledo-pp-cli notes edit` — Edit note(s). Pass a JSON array of note objects including id.
- `toodledo-pp-cli notes list` — List notes. Feeds the local mirror via sync.

**outlines** — Outlines (hierarchical outline documents)

- `toodledo-pp-cli outlines add` — Create outline(s). Pass a JSON array of outline objects.
- `toodledo-pp-cli outlines delete` — Delete outline(s). Pass a JSON array of outline ids.
- `toodledo-pp-cli outlines deleted` — List outline ids deleted after a timestamp
- `toodledo-pp-cli outlines edit` — Edit outline(s). Pass a JSON array of outline objects including id.
- `toodledo-pp-cli outlines list` — List outlines. Feeds the local mirror via sync.

**tasks** — Tasks (the GTD hub). Writes resolve folder/context/goal names to ids and parse YYYY-MM-DD dates.

- `toodledo-pp-cli tasks add [title]` — Create a task (flags: --folder, --context, --goal, --priority, --status, --star, --due, --tag, --note, --repeat, --parent)
- `toodledo-pp-cli tasks edit <id>` — Edit a task; only the fields you pass change (empty --due/--start clears them)
- `toodledo-pp-cli tasks complete <id> [id...]` — Mark task(s) complete (batched to 50)
- `toodledo-pp-cli tasks reopen <id> [id...]` — Reopen completed task(s)
- `toodledo-pp-cli tasks delete <id> [id...]` — Permanently delete task(s) (batched to 50)
- `toodledo-pp-cli tasks deleted` — List task ids deleted after a timestamp (for mirror reconciliation)
- `toodledo-pp-cli tasks list` — List tasks (incomplete by default). Feeds the local mirror via sync.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
toodledo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning next actions

```bash
toodledo-pp-cli next-actions --context @work --agent
```

The @work next-action list as agent-shaped JSON, answered offline from the local mirror.

### Weekly review, narrowed for an agent

```bash
toodledo-pp-cli review --agent --select overdue.title,overdue.duedate,stalled_projects.folder
```

Pull only the overdue titles/dates and stalled-project names from the five-bucket review so the agent does not ingest the full payload.

### Budget-safe sync

```bash
toodledo-pp-cli sync-cost --since 7d
```

Preview how many of your 100 token-calls a 7-day incremental sync will spend before running it.

### Bulk capture from a file

```bash
toodledo-pp-cli capture --file ~/inbox.txt --folder Inbox
```

Add one task per line, resolving the Inbox folder name, in batches of 50.

### Find stalled projects

```bash
toodledo-pp-cli stalled-projects --json
```

Projects with open tasks but no Next Action — the weekly review's highest-value bucket.

## Auth Setup

Toodledo uses OAuth 2.0. Register an app at toodledo.com to get a client id and secret, set TOODLEDO_CLIENT_ID and TOODLEDO_CLIENT_SECRET, then run 'toodledo-pp-cli auth login' to authorize in your browser. Access tokens last two hours and are refreshed automatically; the refresh token expires after 30 idle days, after which you re-run auth login. The token endpoint sits behind Cloudflare and occasionally returns 403 to valid requests — the CLI treats that distinctly from a real 401 auth failure.

Run `toodledo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  toodledo-pp-cli account --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
toodledo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
toodledo-pp-cli feedback --stdin < notes.txt
toodledo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/toodledo-pp-cli/feedback.jsonl`. They are never POSTed unless `TOODLEDO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TOODLEDO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
toodledo-pp-cli profile save briefing --json
toodledo-pp-cli --profile briefing account
toodledo-pp-cli profile list --json
toodledo-pp-cli profile show briefing
toodledo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `toodledo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/toodledo/cmd/toodledo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add toodledo-pp-mcp -- toodledo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which toodledo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   toodledo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `toodledo-pp-cli <command> --help`.
