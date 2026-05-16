# Zoho Projects CLI

**Every Zoho Projects API endpoint, plus portal-wide insights (overdue, stale, workload, issue heat) no native view supports.**

Full coverage of Zoho Projects portals, projects, tasks, task-lists, phases, issues, and users — plus FTS5 over locally-synced content, portal-wide overdue/stale detection, workload fairness, project burn projection, and unassigned-items radar. Built on a synced local SQLite store so PMs and team leads can plan without clicking through every project board.

## Install

The recommended path installs both the `zoho-projects-pp-cli` binary and the `pp-zoho-projects` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install zoho-projects
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install zoho-projects --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-projects-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-projects --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zoho-projects --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-zoho-projects skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-zoho-projects. The skill defines how its required CLI can be installed.
```

## Authentication

Zoho Projects uses OAuth 2.0. You need a self-client refresh token, client ID/secret, and your portal ID. Set ZOHOPROJECTS_REFRESH_TOKEN, ZOHOPROJECTS_CLIENT_ID, ZOHOPROJECTS_CLIENT_SECRET, ZOHOPROJECTS_PORTAL_ID — or run `zoho-projects-pp-cli auth set-token` to store them. The CLI auto-refreshes access tokens.

## Quick Start

```bash
# Validates OAuth creds + portal_id + API reachability.
zoho-projects-pp-cli doctor --json


# Populates SQLite with all portals, projects, tasks, issues, users.
zoho-projects-pp-cli sync --full


# Top 10 items assigned to you, ranked by due date.
zoho-projects-pp-cli my-focus --limit 10 --json


# Portal-wide overdue tasks.
zoho-projects-pp-cli overdue --json --select id,name,project,owner,end_date


# Per-user open-task counts.
zoho-projects-pp-cli workload --json --select owner,open_count

```

## Unique Features

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

## Usage

Run `zoho-projects-pp-cli --help` for the full command reference and flag list.

## Commands

### issues

Manage issues

- **`zoho-projects-pp-cli issues list-my`** - List issues assigned to the authenticated user
- **`zoho-projects-pp-cli issues list-portal`** - List issues across the portal

### portal

Manage portal

- **`zoho-projects-pp-cli portal get`** - Get portal details

### portals

Manage portals

- **`zoho-projects-pp-cli portals list`** - Lists all portals the authenticated user has access to.

### projects

Manage projects

- **`zoho-projects-pp-cli projects create`** - Create a project
- **`zoho-projects-pp-cli projects delete`** - Move a project to trash
- **`zoho-projects-pp-cli projects get`** - Get a project
- **`zoho-projects-pp-cli projects list`** - List projects in a portal
- **`zoho-projects-pp-cli projects update`** - Update a project

### tasks

Manage tasks

- **`zoho-projects-pp-cli tasks list-portal`** - List tasks across all portal projects

### users

Manage users

- **`zoho-projects-pp-cli users list`** - List users in the portal


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zoho-projects-pp-cli portal mock-value

# JSON for scripting and agents
zoho-projects-pp-cli portal mock-value --json

# Filter to specific fields
zoho-projects-pp-cli portal mock-value --json --select id,name,status

# Dry run — show the request without sending
zoho-projects-pp-cli portal mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zoho-projects-pp-cli portal mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-zoho-projects -g
```

Then invoke `/pp-zoho-projects <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add zoho-projects zoho-projects-pp-mcp -e ZOHOPROJECTS_REFRESH_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoho-projects-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZOHOPROJECTS_REFRESH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zoho-projects": {
      "command": "zoho-projects-pp-mcp",
      "env": {
        "ZOHOPROJECTS_REFRESH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
zoho-projects-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/zoho-projects-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZOHOPROJECTS_REFRESH_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zoho-projects-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZOHOPROJECTS_REFRESH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **INVALID_OAUTHTOKEN** — Re-run `zoho-projects-pp-cli auth set-token` with a fresh self-client refresh token from accounts.zoho.com/developerconsole. Scopes needed: ZohoProjects.portals.READ, projects.READ, tasks.ALL, bugs.ALL.
- **Portal not found** — Check ZOHOPROJECTS_PORTAL_ID matches a portal the token has access to. `zoho-projects-pp-cli portals list --json` lists every portal your token can see.
- **429 Too Many Requests** — Zoho Projects limits to ~10 req/s per org. The CLI retries with backoff; for large initial syncs use `sync --concurrency 1`.
- **grep returns nothing** — FTS index is populated by `sync`. Run `sync --resources tasks,issues,projects` first.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
