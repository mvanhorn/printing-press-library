# Podscan CLI

Podscan REST API — search 51M+ podcast episodes and 4.4M+ podcasts.
Full transcripts, AI-extracted entities, mentions, brand-safety analysis.

Learn more at [Podscan](https://podscan.fm).

Created by [@gregvanhorn](https://github.com/gregvanhorn) (Greg Van Horn).

## Install

The recommended path installs both the `podscan-pp-cli` binary and the `pp-podscan` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install podscan
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install podscan --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install podscan --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install podscan --agent claude-code
npx -y @mvanhorn/printing-press-library install podscan --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podscan/cmd/podscan-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podscan-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-podscan --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-podscan --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install podscan --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/podscan-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PODSCAN_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "podscan": {
      "command": "podscan-pp-mcp",
      "env": {
        "PODSCAN_BEARER_AUTH": "<your-key>"
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
podscan-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export PODSCAN_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
podscan-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
podscan-pp-cli alerts list
```

## Usage

Run `podscan-pp-cli --help` for the full command reference and flag list.

## Commands

### alerts

Manage alerts

- **`podscan-pp-cli alerts create`** - Create a new alert
- **`podscan-pp-cli alerts delete`** - Delete an alert
- **`podscan-pp-cli alerts get`** - Get an alert
- **`podscan-pp-cli alerts list`** - List configured alerts

### categories

Manage categories

- **`podscan-pp-cli categories list`** - List all podcast categories

### episodes

Manage episodes

- **`podscan-pp-cli episodes get`** - Get an episode by ID
- **`podscan-pp-cli episodes search`** - Search episodes by transcript content

### exports

Manage exports

- **`podscan-pp-cli exports download`** - Download an export file
- **`podscan-pp-cli exports list-episode`** - List daily episode export files
- **`podscan-pp-cli exports list-podcast`** - List podcast catalog export files

### podcasts

Manage podcasts

- **`podscan-pp-cli podcasts get`** - Get a podcast by ID
- **`podscan-pp-cli podcasts search`** - Search podcasts by name or description

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
podscan-pp-cli alerts list

# JSON for scripting and agents
podscan-pp-cli alerts list --json

# Filter to specific fields
podscan-pp-cli alerts list --json --select id,name,status

# Dry run — show the request without sending
podscan-pp-cli alerts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
podscan-pp-cli alerts list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
podscan-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/podscan-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PODSCAN_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `podscan-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PODSCAN_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
