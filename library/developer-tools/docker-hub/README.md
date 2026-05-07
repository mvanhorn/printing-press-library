# Docker Hub CLI

Docker Hub public API. Search container images, browse tags, check sizes,
inspect Dockerfiles, and explore the world's largest container registry.
No authentication required for public repositories (rate limited to ~18 req/min).

## Install

The recommended path installs both the `docker-hub-pp-cli` binary and the `pp-docker-hub` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install docker-hub
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install docker-hub --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/docker-hub/cmd/docker-hub-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/docker-hub-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
docker-hub-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
docker-hub-pp-cli docker-hub-search
```

## Usage

Run `docker-hub-pp-cli --help` for the full command reference and flag list.

## Commands

### docker-hub-search

Manage docker hub search

- **`docker-hub-pp-cli docker-hub-search search-repositories`** - Full-text search across all Docker Hub repositories. Returns name, description, stars, and pull counts.

### repositories

Repository metadata and details

- **`docker-hub-pp-cli repositories get-repository`** - Full metadata for a Docker Hub repository including pull count, stars, description, and last update time.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
docker-hub-pp-cli docker-hub-search

# JSON for scripting and agents
docker-hub-pp-cli docker-hub-search --json

# Filter to specific fields
docker-hub-pp-cli docker-hub-search --json --select id,name,status

# Dry run — show the request without sending
docker-hub-pp-cli docker-hub-search --dry-run

# Agent mode — JSON + compact + no prompts in one flag
docker-hub-pp-cli docker-hub-search --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-docker-hub -g
```

Then invoke `/pp-docker-hub <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/docker-hub/cmd/docker-hub-pp-mcp@latest
```

Then register it:

```bash
claude mcp add docker-hub docker-hub-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-docker-hub --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-docker-hub --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-docker-hub skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-docker-hub. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/docker-hub-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/docker-hub/cmd/docker-hub-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "docker-hub": {
      "command": "docker-hub-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
docker-hub-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/docker-hub-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
