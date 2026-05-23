# cmux CLI

**Every cmux feature, plus persisted state, cross-surface search, and a notify-driven event stream no other cmux tool offers.**

This CLI wraps the cmux Unix-socket CLI with an AI-agent-shaped surface: a local SQLite store of workspaces, surfaces, status entries, and notifications; FTS5 search across titles and sampled pane content with surface-level matches; and a long-running `watch` command that emits cmux notification events as JSONL to stdout, files, Slack, or any webhook so ecosystem-manager-style agents can wait on events instead of polling.

Printed by [@dstevens](https://github.com/dstevens) (Damien Stevens).

## Install

The recommended path installs both the `cmux-pp-cli` binary and the `pp-cmux` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cmux
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cmux --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cmux-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cmux --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cmux --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cmux skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cmux. The skill defines how its required CLI can be installed.
```

## Authentication

cmux uses a local Unix socket at /tmp/cmux.sock with password auth resolved in this order: --password flag, CMUX_SOCKET_PASSWORD env, then the password saved in cmux's keychain entry. cmux-pp-cli shells out to the cmux binary; if cmux is not running or the socket password is wrong, `cmux-pp-cli doctor` says so up front.

## Quick Start

```bash
# verify the cmux binary is on PATH, cmux is running, the socket answers ping, and credentials resolve
cmux-pp-cli doctor


# snapshot every workspace, surface, status entry, and unread notification into the local SQLite store
cmux-pp-cli sync


# list every workspace whose canonical state is awaiting input
cmux-pp-cli status awaiting --all --json


# find which surface mentioned the phrase, not just which workspace
cmux-pp-cli search 'rate limit' --json --select results.workspace_ref,results.surface_ref,results.snippet


# stream notification events as JSONL — pipe to jq, exec hook, or Slack webhook
cmux-pp-cli watch --source notifications --sink stdout --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-pane intelligence
- **`search`** — Search workspace titles, surface titles, notification bodies, and sampled pane content with FTS5 — get the surface and snippet, not just the workspace; --switch jumps cmux to the matching surface.

  _When an agent or user knows a phrase but not which workspace, this returns the exact surface to focus instead of forcing a workspace-only walk._

  ```bash
  cmux-pp-cli search 'WAF cookie' --json --select results.workspace_ref,results.surface_ref,results.snippet
  ```
- **`workspaces card`** — One-shot summary for a workspace: metadata (cwd, git_branch, pr) + current status entries + last 3 notifications + last sampled pane snippets per surface.

  _Gives a manager subagent the full picture of a workspace in one tool call._

  ```bash
  cmux-pp-cli workspaces card Tuck --json
  ```

### Notify-driven loop closure
- **`watch`** — Long-running stream of cmux notification events as JSONL, with pluggable sinks (stdout, file, exec hook, Slack webhook, generic webhook). Replaces capture-pane polling with id-cursored notification events or fsnotify on the session JSON.

  _Lets a manager subagent wait on events instead of burning context on full pane scans._

  ```bash
  cmux-pp-cli watch --source notifications --sink stdout --json
  ```
- **`alert add`** — Declarative rules that fire when a workspace transitions into a state (e.g. Tuck claude_code Running -> Needs input). Sinks: stdout, file, exec, macOS osascript, Slack webhook, generic webhook.

  _Closes the loop on the user instead of on cmux's sidebar._

  ```bash
  cmux-pp-cli alert add --workspace Tuck --on awaiting --sink slack:https://hooks.slack.com/services/X --json
  ```

### Local state that compounds
- **`status timeline`** — Time-series of agent state per workspace (and key) over a window, drawn from local status snapshots persisted on every sync.

  _Answers 'when did Tuck go awaiting?' which is needed to triage stuck panes and to bound sync cadence._

  ```bash
  cmux-pp-cli status timeline --workspace Tuck --since 4h --json
  ```
- **`status stuck`** — List every (workspace, key) whose latest persisted state is awaiting input and whose transition timestamp is older than a threshold.

  _Surfaces the panes a manager subagent should triage first, without re-capturing screens._

  ```bash
  cmux-pp-cli status stuck --over 30m --json
  ```
- **`status awaiting`** — Single canonical state per workspace (idle / working / awaiting / stranded) computed by joining status entries with title-icon decode and surface health.

  _Gives an agent a single boolean to drive 'should I look here?' instead of decoding folklore icons._

  ```bash
  cmux-pp-cli status awaiting --all --json
  ```
- **`status changes`** — List workspaces (and keys) whose persisted state changed within a time window, with prev_value and new_value.

  _Direct replacement for the manager's 'did anything change since my last tick?' question, without rewalking every pane._

  ```bash
  cmux-pp-cli status changes --since 1h --json
  ```

## Usage

Run `cmux-pp-cli --help` for the full command reference and flag list.

## Commands

### buffers

cmux paste buffers

- **`cmux-pp-cli buffers list`** - List paste buffers.

### capabilities

Methods the running cmux exposes

- **`cmux-pp-cli capabilities list`** - List every RPC method the running cmux exposes.

### hooks

cmux event hooks

- **`cmux-pp-cli hooks list`** - List configured hooks.

### logs

Sidebar log entries per workspace

- **`cmux-pp-cli logs list`** - List sidebar log entries for a workspace.

### notifications

cmux notifications (the event stream for loop closure)

- **`cmux-pp-cli notifications list`** - List notifications across all workspaces. Use --json for the full payload, or filter on workspace_id.

### panes

cmux panes (split containers inside a workspace)

- **`cmux-pp-cli panes list`** - List panes in a workspace.

### status

Per-workspace agent status entries (e.g., claude_code state)

- **`cmux-pp-cli status list`** - List status entries for a workspace.

### surfaces

cmux surfaces (terminal or browser tabs inside a pane)

- **`cmux-pp-cli surfaces health`** - Surface health: which surfaces are stranded (not in any window).
- **`cmux-pp-cli surfaces list`** - List surfaces, optionally scoped to a workspace.

### windows

cmux windows (top-level OS windows)

- **`cmux-pp-cli windows current`** - Show the current window.
- **`cmux-pp-cli windows list`** - List all cmux windows.

### workspaces

cmux workspaces (logical pane groups)

- **`cmux-pp-cli workspaces current`** - Show the currently selected workspace.
- **`cmux-pp-cli workspaces get`** - Get a single workspace with sidebar metadata (cwd, branch, pr).
- **`cmux-pp-cli workspaces list`** - List all workspaces.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cmux-pp-cli buffers

# JSON for scripting and agents
cmux-pp-cli buffers --json

# Filter to specific fields
cmux-pp-cli buffers --json --select id,name,status

# Dry run — show the request without sending
cmux-pp-cli buffers --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cmux-pp-cli buffers --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cmux -g
```

Then invoke `/pp-cmux <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cmux cmux-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cmux-current).
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
    "cmux": {
      "command": "cmux-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
cmux-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/cmux-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports `cmux binary not found`** — install cmux.app and add `/Applications/cmux.app/Contents/Resources/bin` to PATH, or set CMUX_BIN to the binary path.
- **doctor reports `socket not answering`** — start cmux.app and wait for the menu-bar icon, then re-run `cmux-pp-cli doctor`.
- **search returns nothing for a phrase you can see on screen** — run `cmux-pp-cli panes sample --workspace W --surface S --scrollback` to seed the FTS table; sync only stores titles + notifications by default.
- **`watch --source fsnotify` exits immediately** — ensure `~/Library/Application Support/cmux/session-com.cmuxterm.app.json` exists by interacting with cmux at least once after install; fsnotify cannot watch a non-existent path.
- **alerts to Slack do not fire** — test the webhook with `cmux-pp-cli alert send-test --sink slack:<url>`; webhook URLs that 4xx will be quoted in `--json` output.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
