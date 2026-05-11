# Artyfacts CLI

Artyfacts — persistent workspace for AI agent work products

Learn more at [Artyfacts](https://artyfacts.ai).

## Install

The recommended path installs both the `artyfacts-pp-cli` binary and the `pp-artyfacts` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install artyfacts
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install artyfacts --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/artyfacts-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-artyfacts --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-artyfacts --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-artyfacts skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-artyfacts. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
artyfacts-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export ARTYFACTS_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
artyfacts-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
artyfacts-pp-cli artifacts list
```

## Usage

Run `artyfacts-pp-cli --help` for the full command reference and flag list.

## Commands

### artifacts

Manage AI-generated work product artifacts

- **`artyfacts-pp-cli artifacts create`** - Create a new artifact envelope. Add content using section tools after creation.
- **`artyfacts-pp-cli artifacts get`** - Get a specific artifact with all sections
- **`artyfacts-pp-cli artifacts list`** - List artifacts, optionally filtered by type, folder, or root-level
- **`artyfacts-pp-cli artifacts update`** - Update artifact metadata (title, summary, status, tags, visibility, retention)

### org

Organization context and settings

- **`artyfacts-pp-cli org context`** - Get organization details, agent conventions, and preferred workflows

### sections

Manage sections within an artifact

- **`artyfacts-pp-cli sections create`** - Create a new section with content in one step
- **`artyfacts-pp-cli sections delete`** - Delete a section from an artifact
- **`artyfacts-pp-cli sections get`** - Get a specific section by artifact and section ID
- **`artyfacts-pp-cli sections list`** - List all sections of an artifact, ordered by position
- **`artyfacts-pp-cli sections update`** - Update section content or streaming state


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
artyfacts-pp-cli artifacts list

# JSON for scripting and agents
artyfacts-pp-cli artifacts list --json

# Filter to specific fields
artyfacts-pp-cli artifacts list --json --select id,name,status

# Dry run — show the request without sending
artyfacts-pp-cli artifacts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
artyfacts-pp-cli artifacts list --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-artyfacts -g
```

Then invoke `/pp-artyfacts <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add artyfacts artyfacts-pp-mcp -e ARTYFACTS_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/artyfacts-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ARTYFACTS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "artyfacts": {
      "command": "artyfacts-pp-mcp",
      "env": {
        "ARTYFACTS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
artyfacts-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/artyfacts-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ARTYFACTS_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `artyfacts-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ARTYFACTS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
