# Wikipedia CLI

Wikipedia REST API. Get article summaries, search, browse related topics,
and access on-this-day events. No authentication required.
Uses a polite User-Agent header.

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wikipedia-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/wikipedia/cmd/wikipedia-pp-cli@latest
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
wikipedia-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
wikipedia-pp-cli feed mock-value
```

## Usage

Run `wikipedia-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Manage feed

- **`wikipedia-pp-cli feed get-on-this-day`** - Returns events, births, deaths, or holidays that occurred on a given date.

### page

Article content and metadata

- **`wikipedia-pp-cli page get-html`** - Returns the full article body as styled HTML.
- **`wikipedia-pp-cli page get-media`** - Returns images, videos, and other media files associated with an article.
- **`wikipedia-pp-cli page get-random`** - Returns a random Wikipedia article summary.
- **`wikipedia-pp-cli page get-summary`** - Returns a page summary including title, extract text, thumbnail, and coordinates.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wikipedia-pp-cli feed mock-value

# JSON for scripting and agents
wikipedia-pp-cli feed mock-value --json

# Filter to specific fields
wikipedia-pp-cli feed mock-value --json --select id,name,status

# Dry run — show the request without sending
wikipedia-pp-cli feed mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wikipedia-pp-cli feed mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-wikipedia -g
```

Then invoke `/pp-wikipedia <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/wikipedia/cmd/wikipedia-pp-mcp@latest
```

Then register it:

```bash
claude mcp add wikipedia wikipedia-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wikipedia --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wikipedia --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-wikipedia skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-wikipedia. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wikipedia-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/wikipedia/cmd/wikipedia-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wikipedia": {
      "command": "wikipedia-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
wikipedia-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/wikipedia-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
