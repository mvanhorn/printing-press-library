# ClickUp CLI

**Every ClickUp endpoint plus a local SQLite mirror with offline full-text search and cross-task analytics no other ClickUp CLI ships.**

clickup-2-pp-cli matches the full ClickUp v2 API surface (tasks, lists, time tracking, goals, custom fields, webhooks and more) and adds what every other ClickUp tool lacks: a local store you can search offline and query with SQL. That local layer powers commands the web app charges for or cannot do at all, like time-in-status cycle analytics, assignee workload, and a 'what changed since yesterday' activity delta.

## Install

The recommended path installs both the `clickup-2-pp-cli` binary and the `pp-clickup-2` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clickup
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clickup --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clickup --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install clickup --agent claude-code
npx -y @mvanhorn/printing-press-library install clickup --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickup-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clickup-2 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clickup-2 --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-clickup-2 skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-clickup-2. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickup-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLICKUP_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "clickup": {
      "command": "clickup-2-pp-mcp",
      "env": {
        "CLICKUP_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a ClickUp personal API token (Settings > Apps > API Token, format pk_...). Set it once with `clickup-2-pp-cli auth set-token` or export CLICKUP_API_TOKEN. The token goes in the Authorization header with no Bearer prefix, matching ClickUp's scheme.

## Quick Start

```bash
# store your personal token so every command is authenticated
clickup-2-pp-cli auth set-token pk_xxx

# list your workspaces (teams) and note the id you want, the root of the hierarchy
clickup-2-pp-cli team

# pull your workspaces and their tasks into the local SQLite store so offline and analytics commands work
clickup-2-pp-cli sync

# your open tasks across all lists, due in the next week, served offline
clickup-2-pp-cli my-day --due 7d

# full-text search across synced tasks and comments
clickup-2-pp-cli search "migration rollback"

```

## Unique Features

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

## Usage

Run `clickup-2-pp-cli --help` for the full command reference and flag list.

## Commands

### checklist

Manage checklist

- **`clickup-2-pp-cli checklist delete`** - Delete a checklist from a task.
- **`clickup-2-pp-cli checklist edit`** - Rename a task checklist, or reorder a checklist so it appears above or below other checklists on a task.

### comment

Manage comment

- **`clickup-2-pp-cli comment delete`** - Delete a task comment.
- **`clickup-2-pp-cli comment update`** - Replace the content of a task commment, assign a comment, and mark a comment as resolved.

### folder

Manage folder

- **`clickup-2-pp-cli folder delete`** - Delete a Folder from your Workspace.
- **`clickup-2-pp-cli folder get`** - View the Lists within a Folder.
- **`clickup-2-pp-cli folder update`** - Rename a Folder.

### goal

Manage goal

- **`clickup-2-pp-cli goal delete`** - Remove a Goal from your Workspace.
- **`clickup-2-pp-cli goal get`** - View the details of a Goal including its Targets.
- **`clickup-2-pp-cli goal update`** - Rename a Goal, set the due date, replace the description, add or remove owners, and set the Goal color.

### group

Manage group

- **`clickup-2-pp-cli group delete-team`** - This endpoint is used to remove a [User Group](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups) from your Workspace.\
 \
In our API documentation, `team_id` refers to the id of a Workspace, and `group_id` refers to the id of a user group.
- **`clickup-2-pp-cli group get-teams1`** - This endpoint is used to view [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups) in your Workspace.\
 \
In our API documentation, `team_id` refers to the ID of a Workspace, and `group_id` refers to the ID of a User Group.
- **`clickup-2-pp-cli group update-team`** - This endpoint is used to manage [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups), which are groups of users within your Workspace.\
 \
In our API, `team_id` in the path refers to the Workspace ID, and `group_id` refers to the ID of a User Group.\
 \
**Note:** Adding a guest with view-only permissions to a User Group automatically converts them to a paid guest.\
 \
If you don't have any paid guest seats available, a new member seat is automatically added to increase the number of paid guest seats.\
 \
This incurs a prorated charge based on your billing cycle.

### key-result

Manage key result

- **`clickup-2-pp-cli key-result delete`** - Delete a target from a Goal.
- **`clickup-2-pp-cli key-result edit`** - Update a Target.

### list

Manage list

- **`clickup-2-pp-cli list delete`** - Delete a List from your Workspace.
- **`clickup-2-pp-cli list get`** - View information about a List.
- **`clickup-2-pp-cli list update`** - Rename a List, update the List Info description, set a due date/time, set the List's priority, set an assignee, set or remove the List color.

### oauth

Manage oauth

- **`clickup-2-pp-cli oauth`** - These are the routes for authing the API and going through the [OAuth flow](doc:authentication).\
 \
Applications utilizing a personal API token don't use this endpoint.\
 \
***Note:** OAuth tokens are not supported when using the [**Try It** feature](doc:trytheapi) of our Reference docs. You can't try this endpoint from your web browser.*

### space

Manage space

- **`clickup-2-pp-cli space delete`** - Delete a Space from your Workspace.
- **`clickup-2-pp-cli space get`** - View the Spaces available in a Workspace.
- **`clickup-2-pp-cli space update`** - Rename, set the Space color, and enable ClickApps for a Space.

### task

Manage task

- **`clickup-2-pp-cli task delete`** - Delete a task from your Workspace.
- **`clickup-2-pp-cli task get`** - View information about a task. You can only view task information of tasks you can access. \
 \
Tasks with attachments will return an "attachments" response. \
 \
Docs attached to a task are not returned.
- **`clickup-2-pp-cli task get-bulk-timein-status`** - View how long two or more tasks have been in each status. The Total time in Status ClickApp must first be enabled by the Workspace owner or an admin.
- **`clickup-2-pp-cli task update`** - Update a task by including one or more fields in the request body.

### team

Manage team

- **`clickup-2-pp-cli team`** - View the Workspaces available to the authenticated user.

### user

Manage user

- **`clickup-2-pp-cli user`** - View the details of the authenticated user's ClickUp account.

### view

Manage view

- **`clickup-2-pp-cli view delete`** - Delete View
- **`clickup-2-pp-cli view get`** - View information about a specific task or page view. The information returned about a view varies by the type of view.
- **`clickup-2-pp-cli view update`** - Rename a view, update the grouping, sorting, filters, columns, and settings of a view.

### webhook

Manage webhook

- **`clickup-2-pp-cli webhook delete`** - Delete a webhook to stop monitoring the events and locations of the webhook.
- **`clickup-2-pp-cli webhook update`** - Update a webhook to change the events to be monitored.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clickup-2-pp-cli folder get mock-value

# JSON for scripting and agents
clickup-2-pp-cli folder get mock-value --json

# Filter to specific fields
clickup-2-pp-cli folder get mock-value --json --select id,name,status

# Dry run — show the request without sending
clickup-2-pp-cli folder get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clickup-2-pp-cli folder get mock-value --agent
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
clickup-2-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/clickup-reference-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLICKUP_API_TOKEN` | per_call | Yes | Set to your API credential. |
| `CLICKUP_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `clickup-2-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clickup-2-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLICKUP_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / OAUTH_017 on every call** — Run `clickup-2-pp-cli auth set-token pk_...` with a valid personal token, or check CLICKUP_API_TOKEN is exported. ClickUp personal tokens go in Authorization with no Bearer prefix.
- **Analytics commands (time-in-status, changed-since, workload) return empty** — Run `clickup-2-pp-cli sync` first; these read the local store, not the live API.
- **429 rate limited** — ClickUp allows 100 requests/minute per token. Sync backs off automatically; for ad-hoc loops add --limit or pause between calls.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**taazkareem/clickup-mcp-server**](https://github.com/taazkareem/clickup-mcp-server) — JavaScript (460 stars)
- [**blockful/clickup-cli**](https://github.com/blockful/clickup-cli) — Go
- [**triptechtravel/clickup-cli**](https://github.com/triptechtravel/clickup-cli) — Go
- [**dang3r/clickupy**](https://github.com/dang3r/clickupy) — Python
- [**hauptsacheNet/clickup-mcp**](https://github.com/hauptsacheNet/clickup-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
