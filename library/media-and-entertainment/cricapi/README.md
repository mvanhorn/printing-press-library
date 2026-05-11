# Cricapi CLI

Live cricket scores, fixtures, players, series, and fantasy data — with first-class MCP and an offline SQLite store.

## Install

The recommended path installs both the `cricapi-pp-cli` binary and the `pp-cricapi` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cricapi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cricapi --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cricapi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cricapi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cricapi --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cricapi skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cricapi. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export CRICAPI_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/cricapi-pp-cli/config.toml`.

### 3. Verify Setup

```bash
cricapi-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
cricapi-pp-cli countries
```

## Usage

Run `cricapi-pp-cli --help` for the full command reference and flag list.

## Commands

### countries

Country / team roster

- **`cricapi-pp-cli countries list`** - All supported countries / international teams

### matches

Cricket matches — live, upcoming, and historical

- **`cricapi-pp-cli matches current`** - Live and about-to-start matches across all cricket
- **`cricapi-pp-cli matches info`** - Detailed information for a single match
- **`cricapi-pp-cli matches list`** - Match list with optional team/series name search
- **`cricapi-pp-cli matches points`** - Fantasy points for a match
- **`cricapi-pp-cli matches score`** - Quick live-score snapshot across all live matches
- **`cricapi-pp-cli matches scorecard`** - Full scorecard with batting/bowling per innings
- **`cricapi-pp-cli matches squad`** - Both team squads for a match

### players

Player roster and career data

- **`cricapi-pp-cli players info`** - Career stats for a single player
- **`cricapi-pp-cli players search`** - Player search by name

### series

Cricket series and tournaments

- **`cricapi-pp-cli series info`** - Series details including matches, squads, and points table
- **`cricapi-pp-cli series list`** - Recent and current series


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cricapi-pp-cli countries

# JSON for scripting and agents
cricapi-pp-cli countries --json

# Filter to specific fields
cricapi-pp-cli countries --json --select id,name,status

# Dry run — show the request without sending
cricapi-pp-cli countries --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cricapi-pp-cli countries --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cricapi -g
```

Then invoke `/pp-cricapi <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cricapi cricapi-pp-mcp -e CRICAPI_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cricapi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CRICAPI_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cricapi": {
      "command": "cricapi-pp-mcp",
      "env": {
        "CRICAPI_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cricapi-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cricapi-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CRICAPI_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cricapi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CRICAPI_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
