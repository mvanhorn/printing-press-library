# Ori CLI

**Unified ops CLI for the openclaw A2A bridge: chat, tasks, approvals, service kickstart, and FTS5 search across cached task history.**

Replaces the per-agent MCP tool sprawl (chat_ori, chat_adam, resume_ori, ...) with one cohesive CLI that has a local SQLite mirror, FTS5 search, and a doctor/kickstart pair that bundles the recurring openclaw-stack diagnostic and recovery rituals. Targets a single home-lab operator; loopback only.

## Install

The recommended path installs both the `ori-pp-cli` binary and the `pp-ori` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ori
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ori --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ori-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ori --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ori --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ori skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ori. The skill defines how its required CLI can be installed.
```

## Authentication

Localhost loopback only. No authentication required by default. If the server is configured with A2A_AUTH_TOKEN_FILE, set OPENCLAW_A2A_TOKEN in the environment.

## Quick Start

```bash
# Confirm the launchd bridge is alive, /healthz responds, both agents are reachable, and plugin cache env vars are commented.
ori doctor


# Pull tasks and approvals from both agents into the local SQLite mirror — needed before tasks search and contexts list.
ori sync


# Direct shell dispatch to ori, no Claude Code in the loop.
ori chat ori 'list today's kanban work'


# What is ori currently working on, JSON for jq/scripts.
ori tasks list --agent ori --state running --json


# Full-text search across cached task transcripts; the bridge cannot answer this.
ori tasks search 'kanban hygiene' --since 7d


# Cross-agent view of every approval waiting on you, both ori and adam.
ori approvals pending

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Operational rescue
- **`doctor`** — Run every openclaw stack health check in one verb: launchd a2a-server status, /healthz reachability, both agents responding, agents.toml valid, plugin cache env vars commented in compose, codex OAuth fresh, gateway WS tunnel reachable.

  _When ori chat returns empty failed, this is the one verb that finds the cause across launchd, plugin cache, OAuth, and gateway state._

  ```bash
  ori doctor --json
  ```
- **`kickstart`** — Restart the launchd a2a-server bridge and poll /healthz until ready. Wraps the post-compose-down/up recovery ritual into one command.

  _After every NAS compose down/up the bridge needs this dance or chat returns empty failed; replaces a memorized 2-step incantation._

  ```bash
  ori kickstart --wait
  ```

### Local state that compounds
- **`sync`** — Paginate ListTasks across all configured agents, upsert into the local SQLite mirror, refresh approvals. Foundation for offline search and historical queries.

  _Run this before any tasks search or contexts list to ensure the local store reflects current bridge state._

  ```bash
  ori sync --full
  ```
- **`tasks search`** — Full-text search across cached agent response text. Filters: --agent, --since, --state. Returns task_id, agent, state, first matching line.

  _Answers 'what did I have ori work on related to X' — a question the bridge cannot answer at all._

  ```bash
  ori tasks search 'kanban hygiene' --since 7d --agent ori
  ```
- **`contexts list`** — Group cached tasks by context_id and surface conversations: first task, last task, task count, peek of the first user message.

  _Resume a forgotten thread by context rather than memorizing a task_id from earlier._

  ```bash
  ori contexts list --agent ori --since 24h
  ```

### Operational visibility
- **`watch`** — Poll ListTasks at an interval and print state transitions live: 'ori task abc123: running → input_required at 23:01'.

  _Check whether ori is still working without paying for a Claude Code turn._

  ```bash
  ori watch --agent ori --interval 5s
  ```
- **`logs tail`** — Tail ~/Library/Logs/openclaw-a2a-server/{stdout,stderr}.log via --stream switching. Hides the path.

  _Surface a2a-server stderr instantly during a wedged-bridge incident; no path memorization._

  ```bash
  ori logs tail --stream stderr --lines 100
  ```
- **`approvals pending`** — Aggregate approvals across all agents, with --watch for live polling. Single pane replacing two separate MCP bridge tools.

  _See and decide approvals from both ori and adam in one stream rather than alternating bridge tools._

  ```bash
  ori approvals pending --watch
  ```

## Usage

Run `ori-pp-cli --help` for the full command reference and flag list.

## Commands

### a2a

Manage a2a


### healthz

Manage healthz

- **`ori-pp-cli healthz health`** - Returns 200 with `{ok: true}` when the launchd-managed a2a-server is
responding. The most common failure mode is the bridge being wedged after
a `compose down/up` — in that case, `ori service kickstart` (a hand-built
compound command, not in this spec) re-runs the
`launchctl kickstart -k gui/$(id -u)/dev.error2.openclaw-a2a-server`
recovery.

### well-known

Manage well known

- **`ori-pp-cli well-known list-agents`** - Returns the agent name list configured via OPENCLAW_AGENTS_CONFIG
(~/.openclaw/agents.toml). Typically returns ["ori", "adam"].


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ori-pp-cli healthz

# JSON for scripting and agents
ori-pp-cli healthz --json

# Filter to specific fields
ori-pp-cli healthz --json --select id,name,status

# Dry run — show the request without sending
ori-pp-cli healthz --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ori-pp-cli healthz --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ori -g
```

Then invoke `/pp-ori <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ori ori-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ori-current).
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
    "ori": {
      "command": "ori-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ori-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/ori-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **ori chat returns empty failed** — Run `ori kickstart --wait`. The launchd bridge wedges after every `compose down/up`; this restarts it and polls /healthz until ready.
- **ori doctor flags codex OAuth as stale** — Run `openclaw configure` inside the openclaw-gateway container on the NAS, then re-run `ori doctor` to confirm. Empty Ori bubbles plus 100% isError on openai-codex = dead OAuth refresh token.
- **ori doctor flags plugin cache env vars set** — Comment OPENCLAW_PLUGIN_DISCOVERY_CACHE_MS and OPENCLAW_PLUGIN_MANIFEST_CACHE_MS in the NAS compose file. Setting them to 300000 reproduces the upstream bind-mount dispatch deadlock.
- **ori tasks search returns no results despite recent activity** — Run `ori sync --full` first. Search reads the local SQLite mirror; new tasks only appear after a sync.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
