# Toodledo CLI

**Every Toodledo feature from the terminal, plus an offline SQLite mirror, GTD reviews, and rate-budget-aware sync that no other Toodledo tool has.**

Toodledo caps you at 100 API calls per access token, so a naive wrapper is unusable. toodledo-pp-cli mirrors your whole task universe into local SQLite, then runs GTD next-actions, weekly review, stalled-project detection, goal rollups, and full-text search entirely offline — with JSON on every command and a complete agent-native MCP surface. sync-cost tells you what a refresh will spend before you spend it.

## Install

The recommended path installs both the `toodledo-pp-cli` binary and the `pp-toodledo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install toodledo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install toodledo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install toodledo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install toodledo --agent claude-code
npx -y @mvanhorn/printing-press-library install toodledo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/toodledo/cmd/toodledo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toodledo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install toodledo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-toodledo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-toodledo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install toodledo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
toodledo-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/toodledo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TOODLEDO_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/toodledo/cmd/toodledo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "toodledo": {
      "command": "toodledo-pp-mcp",
      "env": {
        "TOODLEDO_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Toodledo uses OAuth 2.0. Register an app at toodledo.com to get a client id and secret, set TOODLEDO_CLIENT_ID and TOODLEDO_CLIENT_SECRET, then run 'toodledo-pp-cli auth login' to authorize in your browser. Access tokens last two hours and are refreshed automatically; the refresh token expires after 30 idle days, after which you re-run auth login. The token endpoint sits behind Cloudflare and occasionally returns 403 to valid requests — the CLI treats that distinctly from a real 401 auth failure.

## Quick Start

```bash
# Check the binary, config, and Toodledo API reachability before anything else
toodledo-pp-cli doctor

# Authorize via OAuth in the browser; tokens are stored and auto-refreshed
toodledo-pp-cli auth login

# Mirror your whole Toodledo account into local SQLite once
toodledo-pp-cli sync --full

# Your GTD next actions for @work, answered offline from the mirror
toodledo-pp-cli next-actions --context @work

# The full weekly review (inbox, overdue, stalled, waiting, someday) with zero API calls
toodledo-pp-cli review

```

## Unique Features

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

## Usage

Run `toodledo-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Account info (subscription, sync cursors)

- **`toodledo-pp-cli account`** - Get account info, including per-resource lastedit/lastdelete sync cursors and Pro status

### contexts

Contexts (GTD contexts like @home, @work)

- **`toodledo-pp-cli contexts add`** - Create a context
- **`toodledo-pp-cli contexts delete`** - Delete a context (its tasks become unassigned)
- **`toodledo-pp-cli contexts edit`** - Rename a context
- **`toodledo-pp-cli contexts list`** - List all contexts

### folders

Folders (GTD projects)

- **`toodledo-pp-cli folders add`** - Create a folder
- **`toodledo-pp-cli folders delete`** - Delete a folder (its tasks become unassigned)
- **`toodledo-pp-cli folders edit`** - Edit/rename/archive a folder
- **`toodledo-pp-cli folders list`** - List all folders

### goals

Goals (lifetime / long-term / short-term)

- **`toodledo-pp-cli goals add`** - Create a goal
- **`toodledo-pp-cli goals delete`** - Delete a goal
- **`toodledo-pp-cli goals edit`** - Edit a goal
- **`toodledo-pp-cli goals list`** - List all goals

### lists

Custom lists (user-defined tabular lists)

- **`toodledo-pp-cli lists add`** - Create custom list(s). Pass a JSON array of list objects.
- **`toodledo-pp-cli lists delete`** - Delete custom list(s). Pass a JSON array of list ids.
- **`toodledo-pp-cli lists deleted`** - List custom-list ids deleted after a timestamp
- **`toodledo-pp-cli lists edit`** - Edit custom list(s). Pass a JSON array of list objects including id.
- **`toodledo-pp-cli lists list`** - List custom lists. Feeds the local mirror via sync.

### locations

Locations (named places with coordinates)

- **`toodledo-pp-cli locations add`** - Create a location
- **`toodledo-pp-cli locations delete`** - Delete a location
- **`toodledo-pp-cli locations edit`** - Edit a location
- **`toodledo-pp-cli locations list`** - List all locations

### notes

Notes (standalone notes, optionally filed in folders)

- **`toodledo-pp-cli notes add`** - Create note(s). Pass a JSON array of note objects.
- **`toodledo-pp-cli notes delete`** - Delete note(s). Pass a JSON array of note ids.
- **`toodledo-pp-cli notes deleted`** - List note ids deleted after a timestamp
- **`toodledo-pp-cli notes edit`** - Edit note(s). Pass a JSON array of note objects including id.
- **`toodledo-pp-cli notes list`** - List notes. Feeds the local mirror via sync.

### outlines

Outlines (hierarchical outline documents)

- **`toodledo-pp-cli outlines add`** - Create outline(s). Pass a JSON array of outline objects.
- **`toodledo-pp-cli outlines delete`** - Delete outline(s). Pass a JSON array of outline ids.
- **`toodledo-pp-cli outlines deleted`** - List outline ids deleted after a timestamp
- **`toodledo-pp-cli outlines edit`** - Edit outline(s). Pass a JSON array of outline objects including id.
- **`toodledo-pp-cli outlines list`** - List outlines. Feeds the local mirror via sync.

### tasks

Tasks (the GTD hub). Writes (add/edit/complete/delete) are hand-built ergonomic commands; list/deleted feed the local mirror.

- **`toodledo-pp-cli tasks deleted`** - List task ids deleted after a timestamp (for mirror reconciliation)
- **`toodledo-pp-cli tasks list`** - List tasks (incomplete by default). Feeds the local mirror via sync.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
toodledo-pp-cli account

# JSON for scripting and agents
toodledo-pp-cli account --json

# Filter to specific fields
toodledo-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
toodledo-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
toodledo-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
toodledo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/toodledo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TOODLEDO_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `TOODLEDO_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `toodledo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `toodledo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TOODLEDO_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 Forbidden during auth login or sync** — Toodledo sits behind Cloudflare, which intermittently WAF-blocks valid API traffic. Wait a minute and retry; the CLI sends a real User-Agent and reports 403 distinctly from a 401 auth failure.
- **401 / token_rejected after a couple of hours** — Access tokens expire after 2 hours and refresh automatically. If the refresh token expired (30 days idle), run 'toodledo-pp-cli auth login' again.
- **Subtasks set with --parent are silently dropped** — parent/child links require a Toodledo Pro subscription; non-Pro accounts sync the task without the link. Confirm your plan with 'toodledo-pp-cli account get'.
- **Locked out after a burst of commands** — Toodledo allows 100 calls per access token. Run 'toodledo-pp-cli sync-cost' to preview a sync's cost and prefer 'sync --since 7d' over '--full' for routine refreshes.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**poodledo**](https://github.com/handyman5/poodledo) — Python (29 stars)
- [**toodledo-python**](https://github.com/rkhwaja/toodledo-python) — Python (8 stars)
- [**tdcli**](https://github.com/insanum/tdcli) — Go (2 stars)
- [**toodledo-mcp**](https://github.com/wwilson1017/toodledo-mcp) — TypeScript (1 stars)
- [**org-toodledo**](https://github.com/myuhe/org-toodledo) — Emacs Lisp

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
