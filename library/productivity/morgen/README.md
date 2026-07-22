# Morgen CLI

Morgen calendar & tasks API CLI — unified access to calendars, events, tasks, and tags across connected providers

Printed by [@nickscarabosio](https://github.com/nickscarabosio) (Nick Scarabosio).

## Install

The recommended path installs both the `morgen-pp-cli` binary and the `pp-morgen` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install morgen
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install morgen --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install morgen --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install morgen --agent claude-code
npx -y @mvanhorn/printing-press-library install morgen --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/morgen/cmd/morgen-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/morgen-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-morgen --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-morgen --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-morgen skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-morgen. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/morgen-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MORGEN_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/morgen/cmd/morgen-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "morgen": {
      "command": "morgen-pp-mcp",
      "env": {
        "MORGEN_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export MORGEN_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/morgen/config.toml`.

### 3. Verify Setup

```bash
morgen-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
morgen-pp-cli calendars list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-source synthesis
- **`agenda`** — Unified chronological day view merging calendar events and due tasks across all connected accounts.

  _Reach for this to see everything on a day in one call instead of querying events per calendar and tasks separately._

  ```bash
  morgen-pp-cli agenda --date 2026-06-16
  ```

## Usage

Run `morgen-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

List calendars and update Morgen-specific calendar metadata

- **`morgen-pp-cli calendars list`** - List calendars across all connected accounts
- **`morgen-pp-cli calendars update`** - Update Morgen-specific metadata for a calendar (busy, color, name overrides)

### events

List, create, update, and delete calendar events

- **`morgen-pp-cli events create`** - Create an event in a calendar
- **`morgen-pp-cli events delete`** - Delete an event. Use --series-update-mode for recurring events.
- **`morgen-pp-cli events list`** - List events from one or more calendars in a time window
- **`morgen-pp-cli events update`** - Update an event (patch). Use --series-update-mode for recurring events.

### integrations

View connected accounts and available providers (read-only; connect/disconnect happens in the Morgen app)

- **`morgen-pp-cli integrations accounts`** - List connected calendar/task accounts
- **`morgen-pp-cli integrations providers`** - List available integration providers (Google, Microsoft 365, iCloud, Todoist, etc.)

### tags

Manage tags

- **`morgen-pp-cli tags create`** - Create a tag
- **`morgen-pp-cli tags delete`** - Delete a tag
- **`morgen-pp-cli tags get`** - Get a single tag by ID
- **`morgen-pp-cli tags list`** - List tags
- **`morgen-pp-cli tags update`** - Update a tag

### tasks

Manage tasks across connected task providers

- **`morgen-pp-cli tasks close`** - Mark a task complete
- **`morgen-pp-cli tasks create`** - Create a task
- **`morgen-pp-cli tasks delete`** - Delete a task
- **`morgen-pp-cli tasks get`** - Get a single task by ID
- **`morgen-pp-cli tasks list`** - List tasks
- **`morgen-pp-cli tasks move`** - Reorder or re-parent a task
- **`morgen-pp-cli tasks reopen`** - Reopen a completed task
- **`morgen-pp-cli tasks update`** - Update a task (patch)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
morgen-pp-cli calendars list

# JSON for scripting and agents
morgen-pp-cli calendars list --json

# Filter to specific fields
morgen-pp-cli calendars list --json --select id,name,status

# Dry run — show the request without sending
morgen-pp-cli calendars list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
morgen-pp-cli calendars list --agent
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
morgen-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/morgen/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MORGEN_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `morgen-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MORGEN_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
