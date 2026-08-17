# npm CLI

Research npm packages by metadata, downloads, maintainers, freshness, and dependency risk.

## Install

The recommended path installs both the `npm-pp-cli` binary and the `pp-npm` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install npm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install npm --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/npm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-npm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-npm --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-npm skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-npm. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Credentials

No credentials are required for public npm registry reads.

### 3. Verify Setup

```bash
npm-pp-cli doctor
```

This checks your configuration and npm registry connectivity.

### 4. Try Your First Command

```bash
npm-pp-cli package react --json
```

## Usage

Run `npm-pp-cli --help` for the full command reference and flag list.

## Commands

### intelligence

- **`npm-pp-cli package <name>`** - Summarize one package with latest version, maintainers, dependencies, publish freshness, and last-month downloads.
- **`npm-pp-cli compare <package> [package...]`** - Compare packages by adoption, freshness, maintainers, and dependency surface.
- **`npm-pp-cli risk <package>`** - Score maintenance and adoption risk with explicit signals.

### downloads

Manage downloads

- **`npm-pp-cli downloads get`** - Gets the downloads per day for a given period for all packages.
- **`npm-pp-cli downloads get-point`** - Gets the downloads per day for a given period for a specific package.
- **`npm-pp-cli downloads get-range`** - Gets the downloads per day for a given period for all packages.
- **`npm-pp-cli downloads get-range-2`** - Gets the downloads per day for a given period for a specific package.

### versions

Manage versions



## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
npm-pp-cli compare react vue svelte

# JSON for scripting and agents
npm-pp-cli package react --json

# Filter to specific fields
npm-pp-cli compare express fastify koa --json --select name,latest_version,last_month_downloads

# Dry run — show the request without sending
npm-pp-cli package react --dry-run

# Agent mode — JSON + compact + no prompts in one flag
npm-pp-cli downloads get mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-npm -g
```

Then invoke `/pp-npm <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add npm npm-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/npm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. No credential prompt is required for public npm registry reads.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "npm": {
      "command": "npm-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
npm-pp-cli doctor
```

Verifies configuration and connectivity to the npm registry.

## Configuration

Config file: `~/.config/npm-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `NPM_BASE_URL` | override | No | Point commands at a mock or alternate npm-compatible registry. |

## Troubleshooting
**Not found errors (exit code 3)**
- Check the package name is correct.
- Scoped packages can be passed in normal form, for example `@types/node`.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
