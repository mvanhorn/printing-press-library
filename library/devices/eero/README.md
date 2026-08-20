# Eero CLI

**Inspect an eero mesh network from the terminal with agent-friendly output for networks, nodes, connected devices, diagnostics, and speed tests.**

eero-pp-cli turns the unofficial eero account API into a small, read-first command surface for networks, nodes, connected devices, diagnostics, and speed tests. It supports browser-cookie login for local use and EERO_SESSION_TOKEN for non-interactive automation.

## Install

The recommended path installs both the `eero-pp-cli` binary and the `pp-eero` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install eero
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install eero --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install eero --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install eero --agent claude-code
npx -y @mvanhorn/printing-press-library install eero --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/eero/cmd/eero-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eero-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install eero --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-eero --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-eero --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install eero --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
eero-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/eero-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/eero/cmd/eero-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "eero": {
      "command": "eero-pp-mcp"
    }
  }
}
```

### Optional HTTP MCP

Run `eero-pp-mcp --transport http` only when a remote MCP transport is needed.
It binds to `127.0.0.1:7777` by default and refuses unauthenticated non-loopback
binds. `--allow-public-http` is an explicit escape hatch for deployments that
provide their own network authentication and isolation; the MCP server itself
does not authenticate HTTP callers.

</details>

## Authentication

The eero service uses a session cookie rather than a conventional API key. Run `eero-pp-cli auth login --chrome` to harvest the eero session from Chrome, or set EERO_SESSION_TOKEN when an existing session token is already available. Never commit cookie files or tokens.

## Quick Start

```bash
# Harvest a logged-in eero browser session for local use.
eero-pp-cli auth login --chrome

# Discover the account and available network IDs.
eero-pp-cli account --agent

# Read one compact health snapshot before drilling into nodes or devices.
eero-pp-cli network 550e8400-e29b-41d4-a716-446655440000 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Read-first network operations
- **`network`** — One agent-friendly read combines network status, speed summary, client count, node count, DNS, and feature settings.

  _Use this first when diagnosing whether an eero network is healthy before drilling into a node or client._

  ```bash
  eero-pp-cli network 550e8400-e29b-41d4-a716-446655440000 --agent
  ```
- **`eero list`** — List all eero nodes in a network with compact agent-friendly output.

  _Use it to identify the node to inspect when coverage or topology is the problem._

  ```bash
  eero-pp-cli eero list 550e8400-e29b-41d4-a716-446655440000 --agent
  ```
- **`device list`** — List connected client devices for a network, with filters and compact JSON for downstream automation.

  _Use it to answer which clients are connected before investigating one device in detail._

  ```bash
  eero-pp-cli device list 550e8400-e29b-41d4-a716-446655440000 --agent
  ```
- **`diagnostics`** — Fetch the latest diagnostic report for a network through the same authenticated CLI surface.

  _Use it when the health snapshot shows a problem and you need the provider's latest diagnostic signal._

  ```bash
  eero-pp-cli diagnostics 550e8400-e29b-41d4-a716-446655440000 --agent
  ```
- **`speed-test`** — Read the latest available speed-test results for a network as structured output.

  _Use it to distinguish topology or client issues from an upstream throughput problem._

  ```bash
  eero-pp-cli speed-test 550e8400-e29b-41d4-a716-446655440000 --agent
  ```

## Usage

Run `eero-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EERO_CONFIG_DIR`, `EERO_DATA_DIR`, `EERO_STATE_DIR`, or `EERO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EERO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EERO_HOME=/srv/eero
eero-pp-cli doctor
```

Under `EERO_HOME=/srv/eero`, the four dirs resolve to `/srv/eero/config`, `/srv/eero/data`, `/srv/eero/state`, and `/srv/eero/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "eero": {
      "command": "eero-pp-mcp",
      "env": {
        "EERO_HOME": "/srv/eero"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EERO_DATA_DIR` overrides an explicit `--home` for that kind. Use `EERO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EERO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `eero-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Authenticated eero account summary

- **`eero-pp-cli account`** - Show the authenticated account and available networks.

### activity

eero Plus network activity summary

- **`eero-pp-cli activity <network_id>`** - Show network activity insights when the account has eero Plus.

### device

Connected client devices

- **`eero-pp-cli device list`** - List connected and recently seen client devices on a network.
- **`eero-pp-cli device show`** - Show one connected device by its resource ID.

### diagnostics

Network diagnostics

- **`eero-pp-cli diagnostics <network_id>`** - Show the latest diagnostic report for a network.

### eero

eero mesh nodes

- **`eero-pp-cli eero list`** - List the eero nodes in a network.
- **`eero-pp-cli eero show`** - Show one eero node by its resource ID.

### network

eero network configuration and health

- **`eero-pp-cli network <network_id>`** - Show network health, speed summary, client count, node count, DNS, and feature settings.

### profiles

Parental-control profiles

- **`eero-pp-cli profiles list`** - List profiles attached to a network.
- **`eero-pp-cli profiles show`** - Show one parental-control profile by its resource ID.

### speed_test

Network speed-test results

- **`eero-pp-cli speed-test <network_id>`** - List the latest available speed-test results for a network.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`eero-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`eero-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`eero-pp-cli learnings list`** - Inspect taught rows
- **`eero-pp-cli learnings forget <query>`** - Undo a teach
- **`eero-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`eero-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`eero-pp-cli teach-pattern`** - Install a query/resource template up front
- **`eero-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EERO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `eero-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
eero-pp-cli account

# JSON for scripting and agents
eero-pp-cli account --json

# Filter to specific fields
eero-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
eero-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
eero-pp-cli account --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
eero-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `eero-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/eero-pp-cli/config.toml`; `--home`, `EERO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EERO_SESSION_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `eero-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `eero-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EERO_SESSION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
