# Tessie CLI

Live Tesla vehicle data and commands from the terminal via the Tessie API

Created by [@keithah](https://github.com/keithah) (Keith Herrington).

## Install

The recommended path installs both the `tessie-pp-cli` binary and the `pp-tessie` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tessie
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tessie --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tessie --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tessie --agent claude-code
npx -y @mvanhorn/printing-press-library install tessie --agent claude-code --agent codex
```

### Without Node

If `npx` is not available (no Node, offline), install directly with Go 1.26.6 or newer:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/tessie/cmd/tessie-pp-cli@latest
```

This installs into `$GOPATH/bin` (default `$HOME/go/bin`) — add that directory to `$PATH`.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tessie-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tessie --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tessie --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tessie --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tessie --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tessie-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TESSIE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tessie": {
      "command": "tessie-pp-mcp",
      "env": {
        "TESSIE_API_TOKEN": "<your-key>"
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

Get your access token from your API provider's developer portal, then store it:

```bash
tessie-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export TESSIE_API_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
tessie-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
tessie-pp-cli vehicles
```

## Usage

Run `tessie-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TESSIE_CONFIG_DIR`, `TESSIE_DATA_DIR`, `TESSIE_STATE_DIR`, or `TESSIE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TESSIE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TESSIE_HOME=/srv/tessie
tessie-pp-cli doctor
```

Under `TESSIE_HOME=/srv/tessie`, the four dirs resolve to `/srv/tessie/config`, `/srv/tessie/data`, `/srv/tessie/state`, and `/srv/tessie/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "tessie": {
      "command": "tessie-pp-mcp",
      "env": {
        "TESSIE_HOME": "/srv/tessie"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TESSIE_DATA_DIR` overrides an explicit `--home` for that kind. Use `TESSIE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TESSIE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `tessie-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### commands

Send vehicle commands

- **`tessie-pp-cli commands <vin> <command>`** - Send a vehicle command (honk, flash_lights, start_climate, etc.)

### vehicle

Query and command a single vehicle by VIN

- **`tessie-pp-cli vehicle battery`** - Get the battery state of a vehicle
- **`tessie-pp-cli vehicle location`** - Get the location of a vehicle
- **`tessie-pp-cli vehicle state`** - Get the latest state of a vehicle
- **`tessie-pp-cli vehicle wake`** - Wake the vehicle from sleep

### vehicles

List and select Tesla vehicles

- **`tessie-pp-cli vehicles`** - List all vehicles with their latest state


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`tessie-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`tessie-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`tessie-pp-cli learnings list`** - Inspect taught rows
- **`tessie-pp-cli learnings forget <query>`** - Undo a teach
- **`tessie-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`tessie-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`tessie-pp-cli teach-pattern`** - Install a query/resource template up front
- **`tessie-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TESSIE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `tessie-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tessie-pp-cli vehicles

# JSON for scripting and agents
tessie-pp-cli vehicles --json
# Filter to specific fields by name
tessie-pp-cli vehicles --json --select <field>[,<field>...]

# Dry run — show the request without sending
tessie-pp-cli vehicles --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tessie-pp-cli vehicles --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
tessie-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `tessie-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/tessie-pp-cli/config.toml`; `--home`, `TESSIE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TESSIE_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tessie-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tessie-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TESSIE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
