# Zylos Console CLI

**Every Zylos conversation, from the terminal — with offline search, analytics, and streaming no other tool offers.**

Zylos Console CLI connects to your local Zylos instance to send messages, monitor status, and sync conversations to a local SQLite database. Search past messages offline, analyze response patterns, and stream new messages in real-time.

## Install

The recommended path installs both the `zylos-pp-cli` binary and the `pp-zylos` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install zylos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install zylos --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zylos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zylos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zylos --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-zylos skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-zylos. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Authenticate with your Zylos instance (password or ZYLOS_PASSWORD env var)
zylos-pp-cli auth login


# Check if the AI agent is ready
zylos-pp-cli status


# Send a message to the AI
zylos-pp-cli send "What can you help me with?"


# View recent conversation history
zylos-pp-cli conversations list --limit 5


# Sync all conversations to local SQLite for offline search
zylos-pp-cli sync

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local analytics
- **`stats`** — See message counts, response patterns, and activity trends across all your Zylos conversations.

  _Use this to understand your interaction patterns and Zylos usage trends without manual counting._

  ```bash
  zylos-pp-cli stats --days 7 --json
  ```
- **`timeline`** — View your conversation history as a chronological timeline with gap detection.

  _See the full arc of a conversation day at a glance, including gaps where the AI was idle._

  ```bash
  zylos-pp-cli timeline --today --json
  ```
- **`search`** — Search conversation history with surrounding messages for context.

  _Find what was discussed around a keyword, not just the matching message alone._

  ```bash
  zylos-pp-cli search "deployment" --context 3 --json
  ```
- **`latency`** — Analyze AI response times across conversations.

  _Track whether the AI agent is getting slower or faster over time._

  ```bash
  zylos-pp-cli latency --last 10 --json
  ```

### Data portability
- **`export`** — Export conversations to JSON or Markdown for archival and sharing.

  _Archive important conversations before clearing history or migrating setups._

  ```bash
  zylos-pp-cli export --format markdown --output ./conversations/
  ```

### Monitoring
- **`status watch`** — Monitor AI agent status with state-change detection and auto-exit.

  _Wait for the AI to become available before sending a message, scriptable in CI._

  ```bash
  zylos-pp-cli status watch --watch --until idle
  ```
- **`conversations follow`** — Stream new messages in real-time, pipeable to other tools.

  _Tail the AI conversation in real-time from scripts or other CLIs._

  ```bash
  zylos-pp-cli conversations follow --follow --json | jq '.content'
  ```

## Usage

Run `zylos-pp-cli --help` for the full command reference and flag list.

## Commands

### Conversations

- **`conversations recent`** - Get recent conversation messages
- **`conversations poll`** - Poll for new messages since a given message ID
- **`conversations send`** - Send a message to the AI
- **`conversations follow`** - Stream new messages in real-time (NDJSON, pipeable)

### Analytics

- **`stats`** - Message counts, direction breakdown, and activity trends
- **`timeline`** - Chronological timeline of conversations with gap detection
- **`latency`** - AI response latency analysis
- **`analytics`** - Count, group-by, and summary queries on synced data

### Search & Export

- **`search`** - Full-text search conversation history with surrounding context
- **`export`** - Export conversations to JSON or Markdown
- **`import`** - Import data from JSONL file via API create/upsert calls

### Sync & Data

- **`sync`** - Sync API data to local SQLite for offline search and analysis
- **`tail`** - Stream live API changes at configurable intervals (NDJSON)
- **`workflow archive`** - Sync all resources to local store in one step
- **`workflow status`** - Show local archive status and sync state

### Status & Monitoring

- **`status`** - Get the current status of the AI agent
- **`status watch`** - Monitor AI agent status with periodic polling

### Authentication

- **`session login`** - Authenticate with a password to establish a session
- **`session check`** - Check if authentication is required and current state
- **`session logout`** - End the current authenticated session
- **`auth login`** - Authenticate with the API
- **`auth status`** - Show authentication status

### Utilities

- **`doctor`** - Check CLI health, auth, and connectivity
- **`which`** - Find the command that implements a capability
- **`api`** - Browse all API endpoints by interface name
- **`profile`** - Named sets of flags saved for reuse
- **`feedback`** - Record feedback about this CLI (local by default)
- **`agent-context`** - Emit structured JSON describing this CLI for agents


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zylos-pp-cli status

# JSON for scripting and agents
zylos-pp-cli status --json

# Filter to specific fields
zylos-pp-cli status --json --select id,name,status

# Dry run — show the request without sending
zylos-pp-cli status --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zylos-pp-cli status --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-zylos -g
```

Then invoke `/pp-zylos <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
zylos-pp-cli auth login --chrome

claude mcp add zylos zylos-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
zylos-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zylos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zylos": {
      "command": "zylos-pp-mcp"
    }
  }
}
```

</details>

## Cookbook

```bash
# Full first-run: authenticate, sync, and check status
zylos-pp-cli session login
zylos-pp-cli sync
zylos-pp-cli stats --json

# Search for a keyword with context
zylos-pp-cli search "error" --context 3 --json

# See today's conversation timeline
zylos-pp-cli timeline --today --json

# Export today's conversations as Markdown
zylos-pp-cli export --today --format markdown --output today.md

# Wait for AI to be idle, then send a message
zylos-pp-cli status watch --watch --until idle && \
  zylos-pp-cli conversations send "Ready to work"

# Stream new messages to a file
zylos-pp-cli conversations follow --follow --json >> conversation-log.jsonl

# Analyze response latency over the last 50 exchanges
zylos-pp-cli latency --last 50 --json

# Weekly analytics summary
zylos-pp-cli stats --days 7 --json

# Tail all live API changes every 10 seconds
zylos-pp-cli tail --interval 10s --json

# Find which command to use for a task
zylos-pp-cli which "export conversations"

# Self-hosted Zylos: override the API URL
ZYLOS_BASE_URL=https://zylos.example.com zylos-pp-cli doctor

# Save a profile for daily sync and reuse it
zylos-pp-cli --json profile save daily
zylos-pp-cli --profile daily sync
```

## Health Check

```bash
zylos-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API. Sample output:

```
  OK Config: ok
  OK Auth: configured
  OK Env Vars: ok
  OK API: reachable
  config_path: ~/.config/zylos-pp-cli/config.json
  base_url: http://127.0.0.1:3456
  version: 1.0.0
  OK Cache: fresh
    db_path: ~/.local/share/zylos-pp-cli/data.db
```

## Configuration

Config file: `~/.config/zylos-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZYLOS_PASSWORD` | per_call | Yes | API credential (password). Overrides config file value. |
| `ZYLOS_BASE_URL` | per_call | No | Override the default API base URL (`http://127.0.0.1:3456`). Use for self-hosted instances. |
| `ZYLOS_CONFIG` | per_call | No | Override config file path. |

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `zylos-pp-cli doctor` to check credentials and connectivity
- Verify the environment variable is set: `echo $ZYLOS_PASSWORD`
- Try re-authenticating: `zylos-pp-cli session login`

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `zylos-pp-cli which "<what you want>"` to find the right command

**Config errors (exit code 10)**
- Verify config file exists and is valid JSON: `cat ~/.config/zylos-pp-cli/config.json`
- Check for typos in `ZYLOS_BASE_URL` if using a self-hosted instance

**Rate limited (exit code 7)**
- Wait and retry. Use `--rate-limit` to throttle requests automatically.

**API-specific**

- **Connection refused to 127.0.0.1:3456** — Ensure Zylos is running (check Docker container or process). Override with `ZYLOS_BASE_URL` if using a different host/port.
- **Empty conversation history after sync** — Verify auth session is valid with `zylos-pp-cli doctor`. Check that Zylos has conversations to sync.
- **Sync reports access warnings** — Some resources may require elevated permissions. Check `zylos-pp-cli auth status` and review the sync warnings in `--json` output.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**aichat**](https://github.com/sigoden/aichat) — Rust (12000 stars)
- [**llm**](https://github.com/simonw/llm) — Python (5000 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
