# Workiz CLI

**Every Workiz dispatch and pipeline workflow, plus crew utilization, revenue, and conversion views no other Workiz tool has.**

Workiz has no CLI, MCP server, or agent-native tool today — every existing integration is a thin polling script or a hand-rolled SDK wrapper with zero cross-entity intelligence. This CLI absorbs every job/lead/client/team/time-off operation from the SDK ecosystem, then adds local joins across your synced data to answer questions the live API simply can't: who's overbooked this week, which lead sources convert, and what changed since you last checked.

## Install

The recommended path installs both the `workiz-pp-cli` binary and the `pp-workiz` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install workiz
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install workiz --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install workiz --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install workiz --agent claude-code
npx -y @mvanhorn/printing-press-library install workiz --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/cmd/workiz-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/workiz-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install workiz --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-workiz --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-workiz --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install workiz --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/workiz-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WORKIZ_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/cmd/workiz-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "workiz": {
      "command": "workiz-pp-mcp",
      "env": {
        "WORKIZ_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Workiz uses a two-part credential: an API token embedded in every request URL, and an API secret sent in the body of write calls. Enable the Developer API add-on from Feature Center in the Workiz app, then find both under Settings > Integrations > Developer. Set WORKIZ_API_TOKEN and WORKIZ_API_SECRET (writes only need the secret), or persist the token on disk with 'workiz-pp-cli auth set-token <token>' (the secret still needs WORKIZ_API_SECRET -- there's no on-disk store for it).

## Quick Start

```bash
# Health check that works before you've set any credentials
workiz-pp-cli doctor --dry-run

# Mirror your Workiz data locally so joins and search work offline
workiz-pp-cli sync --resources job,lead,team,timeoff

# See open jobs the way a dispatcher would
workiz-pp-cli job list --status Submitted

# Catch overbooked or double-booked crew before they become no-shows
workiz-pp-cli team bottleneck --week --agent

# See which lead sources are actually converting to paid jobs
workiz-pp-cli lead funnel --since 30d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`team bottleneck --week`** — See per-crew scheduled load and catch double-bookings or time-off conflicts before they become no-shows.

  _Use this for dispatch planning when you need to know who's overbooked or double-booked this week, not just who's on the roster._

  ```bash
  workiz-pp-cli team bottleneck --week --agent
  ```
- **`lead funnel`** — See which lead sources actually turn into paid jobs, with conversion rate and average resulting job value per source.

  _Use this to decide where marketing spend should go, instead of manually eyeballing lead and job dates side by side._

  ```bash
  workiz-pp-cli lead funnel --since 30d --agent
  ```
- **`job revenue`** — Roll up total and outstanding job value by lead source and job status.

  _Use this for a dollar-value rollup by source/status. For lead-to-job conversion counts, use 'lead funnel' instead._

  ```bash
  workiz-pp-cli job revenue --group-by source --agent
  ```
- **`job search`** — Search job notes, lead notes, and comments for free text across your entire synced history.

  _Use this for free-text search inside notes/comments. For structured filtering by status/date/open, use 'job list'/'lead list' flags instead._

  ```bash
  workiz-pp-cli job search "leak" --agent
  ```

### Agent-native plumbing
- **`job audit`** — Find jobs, leads, and clients missing phone, email, amount, or crew fields that would block a downstream billing push.

  _Run this before pushing newly created jobs into a billing or CRM pipeline._

  ```bash
  workiz-pp-cli job audit --agent
  ```
- **`digest`** — See everything new or changed across jobs and leads since your last check, grouped by entity.

  _Use this instead of hand-maintaining your own polling cursor to avoid double-processing records._

  ```bash
  workiz-pp-cli digest --since 24h --agent
  ```

## Recipes


### Morning dispatch check

```bash
workiz-pp-cli team bottleneck --week --agent --select crew,scheduled_hours,conflicts
```

Narrow a deeply-nested crew/job/timeoff join down to just the fields a dispatcher needs before assigning today's calls.

### Marketing spend review

```bash
workiz-pp-cli lead funnel --since 90d --agent
```

Rank lead sources by conversion rate and average resulting job value over the last quarter.

### Pre-billing sweep

```bash
workiz-pp-cli job audit --agent
```

Find jobs missing phone/email/amount before they get pushed into a billing pipeline.

### Catch up after time away

```bash
workiz-pp-cli digest --since 3d --agent
```

See every job/lead/client that changed while you weren't watching, without hand-maintaining a cursor.

## Usage

Run `workiz-pp-cli --help` for the full command reference and flag list.

## Commands

### customer

Customers (clients)

- **`workiz-pp-cli customer create`** - Create a new client
- **`workiz-pp-cli customer get`** - Get a client by id

### job

Scheduled service calls (jobs)

- **`workiz-pp-cli job assign`** - Assign a crew member to a job
- **`workiz-pp-cli job create`** - Create a new job
- **`workiz-pp-cli job get`** - Get a job by UUID
- **`workiz-pp-cli job list`** - List jobs (paginated)
- **`workiz-pp-cli job unassign`** - Unassign a crew member from a job
- **`workiz-pp-cli job update`** - Update a job's schedule

### lead

Pre-job estimates (leads)

- **`workiz-pp-cli lead assign`** - Assign a crew member to a lead
- **`workiz-pp-cli lead create`** - Create a new lead
- **`workiz-pp-cli lead get`** - Get a lead by UUID
- **`workiz-pp-cli lead list`** - List leads (paginated)
- **`workiz-pp-cli lead unassign`** - Unassign a crew member from a lead
- **`workiz-pp-cli lead update`** - Update a lead's schedule

### team

Crew (team) members

- **`workiz-pp-cli team get`** - Get a team member by id
- **`workiz-pp-cli team list`** - List team members

### timeoff

Crew time-off records

- **`workiz-pp-cli timeoff get`** - Get time-off records for a specific team member
- **`workiz-pp-cli timeoff list`** - List time-off records


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
workiz-pp-cli customer get mock-value

# JSON for scripting and agents
workiz-pp-cli customer get mock-value --json

# Filter to specific fields
workiz-pp-cli customer get mock-value --json --select id,name,status

# Dry run — show the request without sending
workiz-pp-cli customer get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
workiz-pp-cli customer get mock-value --agent
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
workiz-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/workiz-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WORKIZ_API_TOKEN` | per_call | Yes | Set to your API credential. |
| `WORKIZ_API_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `workiz-pp-cli doctor` reports `agentcookie: detected` and `workiz-pp-cli auth status` labels the `source` field as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `workiz-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WORKIZ_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Confirm the Developer API add-on is enabled in Feature Center, then re-copy the token from Settings > Integrations > Developer.
- **429 Too Many Requests** — Workiz doesn't publish an exact rate limit; back off and retry, or reduce --records/page size on sync.
- **job/lead create fails with a generic error** — Confirm WORKIZ_API_SECRET is set — every write call requires it in the request body alongside the token.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**BeelineRoutes/workiz**](https://github.com/BeelineRoutes/workiz) — Go
- [**forward-force/workiz**](https://github.com/forward-force/workiz) — PHP
- [**OkoyaUsman/workiz-python-wrapper**](https://github.com/OkoyaUsman/workiz-python-wrapper) — Python
- [**PipedreamHQ/pipedream (workiz component)**](https://github.com/PipedreamHQ/pipedream/tree/master/components/workiz) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
