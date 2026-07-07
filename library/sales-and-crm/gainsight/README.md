# Gainsight CLI

Read-first Gainsight NXT API surface for customer success account, relationship, person, timeline, and Cockpit inspection.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `gainsight-pp-cli` binary and the `pp-gainsight` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gainsight
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gainsight --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gainsight --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gainsight --agent claude-code
npx -y @mvanhorn/printing-press-library install gainsight --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gainsight/cmd/gainsight-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gainsight-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gainsight --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gainsight --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gainsight --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gainsight --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gainsight-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GAINSIGHT_ACCESS_KEY_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gainsight/cmd/gainsight-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gainsight": {
      "command": "gainsight-pp-mcp",
      "env": {
        "GAINSIGHT_ACCESS_KEY_AUTH": "<your-key>"
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
export GAINSIGHT_ACCESS_KEY_AUTH="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/gainsight-read-pp-cli/config.toml`.

### 3. Verify Setup

```bash
gainsight-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
gainsight-pp-cli companies list
```

## Usage

Run `gainsight-pp-cli --help` for the full command reference and flag list.

## Commands

### cockpit

Manage cockpit

- **`gainsight-pp-cli cockpit`** - List Cockpit calls to action.

### companies

Manage companies

- **`gainsight-pp-cli companies get-company`** - Get a company by ID.
- **`gainsight-pp-cli companies list`** - List Gainsight companies.

### data

Manage data

- **`gainsight-pp-cli data`** - List Gainsight data objects.

### healthscores

Manage healthscores

- **`gainsight-pp-cli healthscores`** - List customer health score records.

### people

Manage people

- **`gainsight-pp-cli people get-person`** - Get a person by ID.
- **`gainsight-pp-cli people list`** - List people.

### relationships

Manage relationships

- **`gainsight-pp-cli relationships get`** - Get a relationship by ID.
- **`gainsight-pp-cli relationships list`** - List relationships.

### successplans

Manage successplans

- **`gainsight-pp-cli successplans`** - List success plans.

### timeline

Manage timeline

- **`gainsight-pp-cli timeline`** - List Timeline activities.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gainsight-pp-cli companies list

# JSON for scripting and agents
gainsight-pp-cli companies list --json

# Filter to specific fields
gainsight-pp-cli companies list --json --select id,name,status

# Dry run — show the request without sending
gainsight-pp-cli companies list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gainsight-pp-cli companies list --agent
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
gainsight-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gainsight-read-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GAINSIGHT_ACCESS_KEY_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `gainsight-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gainsight-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GAINSIGHT_ACCESS_KEY_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
