# ClickUp CLI

**Every ClickUp v2 + v3 endpoint as a typed CLI plus offline sync, search, and ergonomic chat/docs aliases.**

172 endpoints across the v2 reference (Tasks, Spaces, Folders, Lists, Goals, Time Tracking, Webhooks, Comments, Custom Fields, Members, Templates, Views) and the v3 public API (Chat, Docs, Audit Logs, ACLs). Hierarchical sync walks the full workspace tree into local SQLite for offline search and analytics. Top-level `docs` and `chat` commands give you idiomatic verbs without remembering the workspaces/<verb>-public spec path.

Created by [@kjmagnan1s](https://github.com/kjmagnan1s) (Kevin Magnan).

## Install

The recommended path installs both the `clickup-pp-cli` binary and the `pp-clickup` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

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

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/project-management/clickup/cmd/clickup-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickup-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clickup --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clickup --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clickup --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clickup --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickup-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLICKUP_AUTHORIZATION_TOKEN` when Claude Desktop prompts you.

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
      "command": "clickup-pp-mcp",
      "env": {
        "CLICKUP_AUTHORIZATION_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

ClickUp accepts both personal API tokens (pk_...) and OAuth bearer tokens via the Authorization header. Set CLICKUP_AUTHORIZATION_TOKEN in your shell or in ~/.config/clickup-pp-cli/config.toml. Run `clickup-pp-cli doctor` to verify auth and API reachability.

## Quick Start

```bash
# Confirm auth, API reachability, and config path before any command.
clickup-pp-cli doctor

# List your workspaces. v2 calls these "teams" for legacy reasons.
clickup-pp-cli team --json

# Hydrate the local SQLite store with the full hierarchy. Parent-ID traversal walks teams → spaces → folders → lists → tasks plus teams → docs and teams → channels in one command.
clickup-pp-cli sync --json --profile default

# Full-text search across every synced record, no API call required.
clickup-pp-cli search "sprint planning" --profile default --data-source local

# Use the v3 docs alias instead of the spec-derived `workspaces docs search-public`.
clickup-pp-cli docs search <team_id> --json --profile default

```

## Unique Features

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

## Usage

Run `clickup-pp-cli --help` for the full command reference and flag list.

## Known Gaps

These limitations exist in the current build. Items marked **(framework)** affect every printing-press CLI; fixing them at `mvanhorn/cli-printing-press` lifts the whole library on the next regen.

- **(framework) `analytics --group-by` doesn't follow nested paths.** `--group-by status.status` returns one bucket of `<nil>` for every row instead of grouping by the nested status field. Top-level fields work.
- **(framework, regen-only) 172 endpoint-mirror MCP tools.** The default endpoint-mirror surface for ClickUp's full v2+v3 API burns agent context. The fix is an `mcp:` block in the spec (`orchestration: code`, `endpoint_tools: hidden`) for a thin `clickup_search` + `clickup_execute` pair instead. Pending a regen with the enrichment.
- **`with_message_since` rejects ISO timestamps on incremental v3 chat sync.** The framework sends an ISO timestamp; ClickUp v3 chat wants a Unix epoch number. Workaround: `sync --full` to clear the cursor.
- **Reaction emoji name format is undocumented.** `chat react --reaction thumbsup` returns a 400. The supported emoji-name format isn't named in the v3 spec; awaiting clarification from ClickUp docs.

### Tips for cross-resource queries

For workspace-scoped task queries, prefer `team task get <team_id>` over per-list fetching. It supports `--order-by created --reverse`, `--list-ids`, `--space-ids`, `--statuses`, `--assignees`, and `--date-created-gt/lt` filters in a single call. Example: `clickup-pp-cli team task get <team_id> --order-by created --reverse --json | jq '.results.tasks[0:5]'` returns the 5 oldest tasks in a workspace. For joins or aggregations beyond what `team task get` and `analytics` cover, the local store at `~/.local/share/clickup-pp-cli/data.db` is plain SQLite — query it directly with `sqlite3` until a generic `sql` command lands upstream.

## Commands

### checklist

Manage checklist

- **`clickup-pp-cli checklist delete`** - Delete a checklist from a task.
- **`clickup-pp-cli checklist edit`** - Rename a task checklist, or reorder a checklist so it appears above or below other checklists on a task.

### comment

Manage comment

- **`clickup-pp-cli comment delete`** - Delete a task comment.
- **`clickup-pp-cli comment update`** - Replace the content of a task commment, assign a comment, and mark a comment as resolved.

### folder

Manage folder

- **`clickup-pp-cli folder delete`** - Delete a Folder from your Workspace.
- **`clickup-pp-cli folder get`** - View the Lists within a Folder.
- **`clickup-pp-cli folder update`** - Rename a Folder.

### goal

Manage goal

- **`clickup-pp-cli goal delete`** - Remove a Goal from your Workspace.
- **`clickup-pp-cli goal get`** - View the details of a Goal including its Targets.
- **`clickup-pp-cli goal update`** - Rename a Goal, set the due date, replace the description, add or remove owners, and set the Goal color.

### group

Manage group

- **`clickup-pp-cli group delete-team`** - This endpoint is used to remove a [User Group](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups) from your Workspace.\
 \
In our API documentation, `team_id` refers to the id of a Workspace, and `group_id` refers to the id of a user group.
- **`clickup-pp-cli group get-teams1`** - This endpoint is used to view [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups) in your Workspace.\
 \
In our API documentation, `team_id` refers to the ID of a Workspace, and `group_id` refers to the ID of a User Group.
- **`clickup-pp-cli group update-team`** - This endpoint is used to manage [User Groups](https://docs.clickup.com/en/articles/4010016-teams-how-to-create-user-groups), which are groups of users within your Workspace.\
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

- **`clickup-pp-cli key-result delete`** - Delete a target from a Goal.
- **`clickup-pp-cli key-result edit`** - Update a Target.

### list

Manage list

- **`clickup-pp-cli list delete`** - Delete a List from your Workspace.
- **`clickup-pp-cli list get`** - View information about a List.
- **`clickup-pp-cli list update`** - Rename a List, update the List Info description, set a due date/time, set the List's priority, set an assignee, set or remove the List color.

### oauth

Manage oauth

- **`clickup-pp-cli oauth get-access-token`** - These are the routes for authing the API and going through the [OAuth flow](doc:authentication).\
 \
Applications utilizing a personal API token don't use this endpoint.\
 \
***Note:** OAuth tokens are not supported when using the [**Try It** feature](doc:trytheapi) of our Reference docs. You can't try this endpoint from your web browser.*

### space

Manage space

- **`clickup-pp-cli space delete`** - Delete a Space from your Workspace.
- **`clickup-pp-cli space get`** - View the Spaces available in a Workspace.
- **`clickup-pp-cli space update`** - Rename, set the Space color, and enable ClickApps for a Space.

### task

Manage task

- **`clickup-pp-cli task delete`** - Delete a task from your Workspace.
- **`clickup-pp-cli task get`** - View information about a task. You can only view task information of tasks you can access. \
 \
Tasks with attachments will return an "attachments" response. \
 \
Docs attached to a task are not returned.
- **`clickup-pp-cli task get-bulk-timein-status`** - View how long two or more tasks have been in each status. The Total time in Status ClickApp must first be enabled by the Workspace owner or an admin.
- **`clickup-pp-cli task update`** - Update a task by including one or more fields in the request body.

### team

Manage team

- **`clickup-pp-cli team get-authorized`** - View the Workspaces available to the authenticated user.

### user

Manage user

- **`clickup-pp-cli user get-authorized`** - View the details of the authenticated user's ClickUp account.

### view

Manage view

- **`clickup-pp-cli view delete`** - Delete View
- **`clickup-pp-cli view get`** - View information about a specific task or page view. The information returned about a view varies by the type of view.
- **`clickup-pp-cli view update`** - Rename a view, update the grouping, sorting, filters, columns, and settings of a view.

### webhook

Manage webhook

- **`clickup-pp-cli webhook delete`** - Delete a webhook to stop monitoring the events and locations of the webhook.
- **`clickup-pp-cli webhook update`** - Update a webhook to change the events to be monitored.

### workspaces

Manage workspaces

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clickup-pp-cli folder get mock-value

# JSON for scripting and agents
clickup-pp-cli folder get mock-value --json

# Filter to specific fields
clickup-pp-cli folder get mock-value --json --select id,name,status

# Dry run — show the request without sending
clickup-pp-cli folder get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clickup-pp-cli folder get mock-value --agent
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
clickup-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/clickup-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLICKUP_AUTHORIZATION_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clickup-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLICKUP_AUTHORIZATION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports API reachable (HTTP 404 at /)** — Expected. ClickUp returns 404 on the root path; doctor confirms reachability via the response, not the status code.
- **sync errors with "with_message_since must be a number string" on v3 chat** — ClickUp's v3 chat expects a Unix epoch number, not the ISO timestamp the framework sends on incremental sync. Run `sync --full` to clear the cursor and avoid the since parameter.
- **sync writes records under the wrong resource_type after a previous run with different naming** — The resources table primary key is `id` alone, not (id, resource_type). Delete stale rows: sqlite3 ~/.local/share/clickup-pp-cli/data.db "DELETE FROM resources WHERE resource_type='<old-name>';" then re-sync.
- **chat react returns 400 "The reaction <name> is not supported"** — ClickUp's reaction emoji name format is undocumented. "thumbsup" and similar plain names fail; the supported set may require Slack-style :name: or raw unicode.
- **Sync fan-out hits ClickUp's 100 req/min rate limit** — Use the default profile's --rate-limit 1.5 (clickup-pp-cli profile save default --rate-limit 1.5 --idempotent) so the limiter stays under the cap.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**clickup-mcp-server**](https://github.com/TaazKareem/clickup-mcp-server) — TypeScript
- [**clickrup**](https://github.com/psolymos/clickrup) — R

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
