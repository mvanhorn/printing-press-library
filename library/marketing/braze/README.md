# Braze CLI

Read-first Braze REST API surface for marketing analytics, campaign inspection, content assets, and catalog lookup.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `braze-pp-cli` binary and the `pp-braze` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install braze
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install braze --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install braze --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install braze --agent claude-code
npx -y @mvanhorn/printing-press-library install braze --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/braze/cmd/braze-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/braze-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install braze --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-braze --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-braze --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install braze --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/braze-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BRAZE_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/braze/cmd/braze-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "braze": {
      "command": "braze-pp-mcp",
      "env": {
        "BRAZE_BEARER_AUTH": "<your-key>"
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
braze-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export BRAZE_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
braze-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
braze-pp-cli campaigns list
```

## Usage

Run `braze-pp-cli --help` for the full command reference and flag list.

## Commands

### campaigns

Manage campaigns

- **`braze-pp-cli campaigns get-data-series`** - Export campaign analytics over time.
- **`braze-pp-cli campaigns get-details`** - Export metadata and configuration for a campaign.
- **`braze-pp-cli campaigns list`** - Export a list of Braze campaigns.

### canvas

Manage canvas

- **`braze-pp-cli canvas get-data-series`** - Export Canvas analytics over time.
- **`braze-pp-cli canvas get-data-summary`** - Export summarized Canvas analytics.
- **`braze-pp-cli canvas get-details`** - Export metadata and configuration for a Canvas.
- **`braze-pp-cli canvas list-canvases`** - Export a list of Braze Canvases.

### catalogs

Manage catalogs

- **`braze-pp-cli catalogs`** - List catalogs.

### content-blocks

Manage content blocks

- **`braze-pp-cli content-blocks get-info`** - Get content block details.
- **`braze-pp-cli content-blocks list`** - List content blocks.

### custom-attributes

Manage custom attributes

- **`braze-pp-cli custom-attributes`** - Export custom attributes and metadata.

### events

Manage events

- **`braze-pp-cli events get-data-series`** - Export custom event analytics over time.
- **`braze-pp-cli events list-custom`** - Export custom event definitions and metadata.
- **`braze-pp-cli events list-custom-names`** - Export names of custom events recorded for an app group.

### kpi

Manage kpi

- **`braze-pp-cli kpi get-daily-active-users`** - Export daily active user counts.
- **`braze-pp-cli kpi get-monthly-active-users`** - Export monthly active user counts.
- **`braze-pp-cli kpi get-new-users`** - Export daily new user counts.

### preference-center

Manage preference center

- **`braze-pp-cli preference-center get`** - Get preference center details.
- **`braze-pp-cli preference-center list`** - List preference centers.

### segments

Manage segments

- **`braze-pp-cli segments get-data-series`** - Export estimated segment size over time.
- **`braze-pp-cli segments get-details`** - Export metadata for a segment.
- **`braze-pp-cli segments list`** - Export a list of Braze segments.

### sessions

Manage sessions

- **`braze-pp-cli sessions`** - Export app session counts over time.

### templates

Manage templates

- **`braze-pp-cli templates get-email-info`** - Get email template details.
- **`braze-pp-cli templates list-email`** - List email templates.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
braze-pp-cli campaigns list

# JSON for scripting and agents
braze-pp-cli campaigns list --json

# Filter to specific fields
braze-pp-cli campaigns list --json --select id,name,status

# Dry run — show the request without sending
braze-pp-cli campaigns list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
braze-pp-cli campaigns list --agent
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
braze-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/braze-read-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BRAZE_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `braze-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `braze-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BRAZE_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
