# Sncf Connect CLI

navitia.io is the open API for building cool stuff with mobility data. It provides the following services

    * journeys computation
    * line schedules
    * next departures
    * exploration of public transport data / search places
    * and sexy things such as isochrones

    navitia is a HATEOAS API that returns JSON formated results

Learn more at [Sncf Connect](https://www.navitia.io/).

Created by [@jmbernabotto](https://github.com/jmbernabotto) (jmbernabotto).

## Install

The recommended path installs both the `sncf-connect-pp-cli` binary and the `pp-sncf-connect` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect --agent claude-code
npx -y @mvanhorn/printing-press-library install sncf-connect --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sncf-connect/cmd/sncf-connect-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sncf-connect-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sncf-connect --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sncf-connect --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sncf-connect --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sncf-connect-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SNCF_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sncf-connect": {
      "command": "sncf-connect-pp-mcp",
      "env": {
        "SNCF_API_KEY": "<your-key>"
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
export SNCF_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/navitia-pp-cli/config.toml`.

### 3. Verify Setup

```bash
sncf-connect-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
sncf-connect-pp-cli coverage get
```

## Usage

Run `sncf-connect-pp-cli --help` for the full command reference and flag list.

## Commands

### coord

Manage coord

- **`sncf-connect-pp-cli coord <lon> <lat>`** - Get lon lat

### coords

Manage coords

- **`sncf-connect-pp-cli coords <lon> <lat>`** - Get lon lat

### coverage

Manage coverage

- **`sncf-connect-pp-cli coverage get`** - Get
- **`sncf-connect-pp-cli coverage get-lon-lat`** - Get lon lat
- **`sncf-connect-pp-cli coverage get-region`** - Get region

### elevations

Manage elevations

- **`sncf-connect-pp-cli elevations`** - Get

### journeys

Manage journeys

- **`sncf-connect-pp-cli journeys`** - Get

### line-groups

Manage line groups

- **`sncf-connect-pp-cli line-groups`** - Get

### lines

Manage lines

- **`sncf-connect-pp-cli lines`** - Get

### networks

Manage networks

- **`sncf-connect-pp-cli networks`** - Get

### places

Manage places

- **`sncf-connect-pp-cli places get`** - Get
- **`sncf-connect-pp-cli places get-id`** - Get id

### route-schedules

Manage route schedules

- **`sncf-connect-pp-cli route-schedules`** - Get

### routes

Manage routes

- **`sncf-connect-pp-cli routes`** - Get

### stop-areas

Manage stop areas

- **`sncf-connect-pp-cli stop-areas`** - Get

### stop-points

Manage stop points

- **`sncf-connect-pp-cli stop-points`** - Get

### stop-schedules

Manage stop schedules

- **`sncf-connect-pp-cli stop-schedules`** - Get

### terminus-schedules

Manage terminus schedules

- **`sncf-connect-pp-cli terminus-schedules`** - Get

### vehicle-journeys

Manage vehicle journeys

- **`sncf-connect-pp-cli vehicle-journeys`** - Get

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sncf-connect-pp-cli coverage get

# JSON for scripting and agents
sncf-connect-pp-cli coverage get --json

# Filter to specific fields
sncf-connect-pp-cli coverage get --json --select id,name,status

# Dry run — show the request without sending
sncf-connect-pp-cli coverage get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sncf-connect-pp-cli coverage get --agent
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
sncf-connect-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/navitia-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SNCF_API_KEY` | per_call | Yes |  |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sncf-connect-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SNCF_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
