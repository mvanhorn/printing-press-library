---
name: pp-clickup
description: "Every ClickUp v2 + v3 endpoint as a typed CLI plus offline sync, search, and ergonomic chat/docs aliases. Trigger phrases: `list my clickup workspaces`, `sync clickup tasks to local`, `search clickup for <term>`, `send a clickup chat message`, `use clickup`, `run clickup-pp-cli`."
author: "Kevin Magnan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clickup-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/project-management/clickup/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# ClickUp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clickup-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install clickup --cli-only
   ```
2. Verify: `clickup-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/clickup/cmd/clickup-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use clickup-pp-cli when you need to script against the full ClickUp v2 + v3 surface, populate a local SQLite store for offline analytics across a workspace's full hierarchy, or expose ClickUp as an MCP server to an agent. Particularly useful for time-tracking aggregation, sprint reporting, and bulk task operations that the web UI doesn't surface.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Walks the full ClickUp hierarchy in dependency order (teams → spaces → folders → lists → tasks; teams → docs; teams → channels) and lands every record in the local SQLite store. Every parent-child relationship in the API is traversed automatically.

  _Reach for this when you want offline search, analytics, or any compound query across a ClickUp workspace's full structure. One command populates the entire tree._

  ```bash
  clickup-pp-cli sync --json --profile default
  ```
- **`sync`** — Distinguishes "recognized envelope with empty array" from "unrecognized response shape" so tenants with zero records of a resource type sync cleanly instead of crashing. ClickUp returns {"spaces":[]} for workspaces with no spaces; the original framework treated this as a singular response and tried to upsert the wrapper as a single record, failing with "missing id for space".

  _Without this, sync silently fails for any workspace with empty resource collections, leaving the local store partially populated._

  ```bash
  clickup-pp-cli sync --resources space --profile default --json
  ```

### Agent-native plumbing
- **`docs`** — Top-level `docs` and `chat` commands with idiomatic verbs (search, get, pages, page, listing, create, edit, send, react, reply, members, followers, messages) replacing the spec-derived workspaces/docs/<verb>-public and workspaces/chat/get-channels paths. ~22 aliases total. Original verbs kept as Cobra aliases for back-compat.

  _Reach for `docs search`, `chat list`, `chat send` directly instead of remembering the deeply-nested workspaces tree. Especially useful when scripting against v3 endpoints._

  ```bash
  clickup-pp-cli docs search 9017321407 --json --profile default
  ```

## Command Reference

**checklist** — Manage checklist

- `clickup-pp-cli checklist delete` — Delete a checklist from a task.
- `clickup-pp-cli checklist edit` — Rename a task checklist, or reorder a checklist so it appears above or below other checklists on a task.

**comment** — Manage comment

- `clickup-pp-cli comment delete` — Delete a task comment.
- `clickup-pp-cli comment update` — Replace the content of a task commment, assign a comment, and mark a comment as resolved.

**folder** — Manage folder

- `clickup-pp-cli folder delete` — Delete a Folder from your Workspace.
- `clickup-pp-cli folder get` — View the Lists within a Folder.
- `clickup-pp-cli folder update` — Rename a Folder.

**goal** — Manage goal

- `clickup-pp-cli goal delete` — Remove a Goal from your Workspace.
- `clickup-pp-cli goal get` — View the details of a Goal including its Targets.
- `clickup-pp-cli goal update` — Rename a Goal, set the due date, replace the description, add or remove owners, and set the Goal color.

**group** — Manage group

- `clickup-pp-cli group delete-team` — This endpoint is used to remove a [User Group](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-...
- `clickup-pp-cli group get-teams1` — This endpoint is used to view [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-gro...
- `clickup-pp-cli group update-team` — This endpoint is used to manage [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-g...

**key-result** — Manage key result

- `clickup-pp-cli key-result delete` — Delete a target from a Goal.
- `clickup-pp-cli key-result edit` — Update a Target.

**list** — Manage list

- `clickup-pp-cli list delete` — Delete a List from your Workspace.
- `clickup-pp-cli list get` — View information about a List.
- `clickup-pp-cli list update` — Rename a List, update the List Info description, set a due date/time, set the List's priority, set an assignee, set...

**oauth** — Manage oauth

- `clickup-pp-cli oauth` — These are the routes for authing the API and going through the [OAuth flow](doc:authentication). Applications...

**space** — Manage space

- `clickup-pp-cli space delete` — Delete a Space from your Workspace.
- `clickup-pp-cli space get` — View the Spaces available in a Workspace.
- `clickup-pp-cli space update` — Rename, set the Space color, and enable ClickApps for a Space.

**task** — Manage task

- `clickup-pp-cli task delete` — Delete a task from your Workspace.
- `clickup-pp-cli task get` — View information about a task. You can only view task information of tasks you can access. Tasks with attachments...
- `clickup-pp-cli task get-bulk-timein-status` — View how long two or more tasks have been in each status. The Total time in Status ClickApp must first be enabled by...
- `clickup-pp-cli task update` — Update a task by including one or more fields in the request body.

**team** — Manage team

- `clickup-pp-cli team` — View the Workspaces available to the authenticated user.

**user** — Manage user

- `clickup-pp-cli user` — View the details of the authenticated user's ClickUp account.

**view** — Manage view

- `clickup-pp-cli view delete` — Delete View
- `clickup-pp-cli view get` — View information about a specific task or page view. The information returned about a view varies by the type of view.
- `clickup-pp-cli view update` — Rename a view, update the grouping, sorting, filters, columns, and settings of a view.

**webhook** — Manage webhook

- `clickup-pp-cli webhook delete` — Delete a webhook to stop monitoring the events and locations of the webhook.
- `clickup-pp-cli webhook update` — Update a webhook to change the events to be monitored.

**workspaces** — Manage workspaces



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clickup-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Drill from workspace to tasks

```bash
clickup-pp-cli team space get <team_id> --json --profile default
```

Get every space ID under a workspace. Pipe into space folder get / space list get-folderless to descend further.

### Compact JSON for agent context

```bash
clickup-pp-cli task get <task_id> --select id,name,status.status,assignees --profile default --agent
```

--agent enables JSON+compact+no-input+no-color+yes for one flag. --select narrows to high-gravity fields, keeping the response under 1KB even on rich tasks.

### Send a chat message via the v3 alias

```bash
clickup-pp-cli workspaces chat create-message <team_id> <channel_id> --type message --content "shipped: rate-limit fix" --json --profile default
```

The unambiguous full path. The shorter `chat send` alias maps to this same endpoint. ClickUp returns the new message id under .data.id for use in chat react / chat reply.

### Local analytics on synced tasks

```bash
clickup-pp-cli analytics --type task --json --profile default
```

Aggregates against the local SQLite store. After sync, no API call is needed. Counts and basic group-by work; nested-path group-by (status.status) is a known framework limitation.

## Auth Setup

ClickUp accepts both personal API tokens (pk_...) and OAuth bearer tokens via the Authorization header. Set CLICKUP_AUTHORIZATION_TOKEN in your shell or in ~/.config/clickup-pp-cli/config.toml. Run `clickup-pp-cli doctor` to verify auth and API reachability.

Run `clickup-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clickup-pp-cli folder get mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
clickup-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clickup-pp-cli feedback --stdin < notes.txt
clickup-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.clickup-pp-cli/feedback.jsonl`. They are never POSTed unless `CLICKUP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLICKUP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clickup-pp-cli profile save briefing --json
clickup-pp-cli --profile briefing folder get mock-value
clickup-pp-cli profile list --json
clickup-pp-cli profile show briefing
clickup-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `clickup-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add clickup-pp-mcp -- clickup-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which clickup-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clickup-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clickup-pp-cli <command> --help`.
