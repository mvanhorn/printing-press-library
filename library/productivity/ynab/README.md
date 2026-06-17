# Ynab CLI

Compact OpenAPI description for the public YNAB v1 API. Covers the read-oriented
endpoints most useful for personal finance snapshots, audits, and purchase decisions.

Created by [@christophersilvestri](https://github.com/christophersilvestri).

## Install

The recommended path installs both the `ynab-pp-cli` binary and the `pp-ynab` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ynab
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ynab --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ynab --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ynab --agent claude-code
npx -y @mvanhorn/printing-press-library install ynab --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ynab/cmd/ynab-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ynab-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ynab --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ynab --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ynab --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ynab --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ynab-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YNAB_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ynab/cmd/ynab-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ynab": {
      "command": "ynab-pp-mcp",
      "env": {
        "YNAB_ACCESS_TOKEN": "<your-key>"
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
ynab-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export YNAB_ACCESS_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
ynab-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
ynab-pp-cli budgets list
```

### 5. Finance snapshot for purchase/runway decisions

```bash
ynab-pp-cli workflow finance-snapshot --months 6 --manual-isa 18044.27 --manual-btc 0.467 --json
```

The snapshot is read-only. It separates operating cash from asset-like accounts, summarizes recent YNAB category burn, computes runway at common monthly burn rates, and keeps tax/business-balance caveats explicit.

For agents, use `--json` for this workflow rather than generic `--agent` compact mode; YNAB responses are nested and finance decisions need the full structured output.

## Usage

Run `ynab-pp-cli --help` for the full command reference and flag list.

## Commands

### budgets

Manage budgets

- **`ynab-pp-cli budgets get`** - Get one budget
- **`ynab-pp-cli budgets list`** - List budgets

### user

Manage user

- **`ynab-pp-cli user`** - Get authenticated user information


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ynab-pp-cli budgets list

# JSON for scripting and agents
ynab-pp-cli budgets list --json

# Filter to specific fields
ynab-pp-cli budgets list --json --select id,name,status

# Dry run — show the request without sending
ynab-pp-cli budgets list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ynab-pp-cli budgets list --agent
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
ynab-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ynab-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YNAB_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ynab-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ynab-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YNAB_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
