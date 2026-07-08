# Lobsters CLI

Read-only JSON surfaces for Lobsters stories, tags, and comments.

## Install

The recommended path installs both the `lobsters-pp-cli` binary and the `pp-lobsters` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install lobsters
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install lobsters --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lobsters-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lobsters --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lobsters --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-lobsters skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-lobsters. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
lobsters-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
lobsters-pp-cli hottest-json
```

## Usage

Run `lobsters-pp-cli --help` for the full command reference and flag list.

## Commands

### hottest-json

Manage hottest json

- **`lobsters-pp-cli hottest-json get-hottest-stories`** - Get hottest stories

### newest

Manage newest

- **`lobsters-pp-cli newest get-stories`** - Get newest stories

### page

Manage page

- **`lobsters-pp-cli page get-front`** - Get front page stories

### s

Manage s

- **`lobsters-pp-cli s get-story`** - Get story details and comments

### t

Manage t

- **`lobsters-pp-cli t get-tag-stories`** - Get stories for a tag


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lobsters-pp-cli hottest-json

# JSON for scripting and agents
lobsters-pp-cli hottest-json --json

# Filter to specific fields
lobsters-pp-cli hottest-json --json --select id,name,status

# Dry run — show the request without sending
lobsters-pp-cli hottest-json --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lobsters-pp-cli hottest-json --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-lobsters -g
```

Then invoke `/pp-lobsters <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add lobsters lobsters-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lobsters-current).
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
    "lobsters": {
      "command": "lobsters-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
lobsters-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/lobsters-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
