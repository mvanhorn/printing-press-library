# Qbo CLI

QuickBooks Online CLI — sync full ledger, query SQLite locally, run duplicate expenses audit, and manage accounting from the terminal.

Created by [@kesslerio](https://github.com/kesslerio) (Martin Kessler).

## Install

The recommended path installs both the `qbo-pp-cli` binary and the `pp-qbo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install qbo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install qbo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install qbo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install qbo --agent claude-code
npx -y @mvanhorn/printing-press-library install qbo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/qbo/cmd/qbo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/qbo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-qbo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-qbo --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install qbo --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/qbo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `QBO_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/qbo/cmd/qbo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "qbo": {
      "command": "qbo-pp-mcp",
      "env": {
        "QBO_CLIENT_ID": "<your-key>"
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
qbo-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export QBO_CLIENT_ID="your-token-here"
```

### 3. Verify Setup

```bash
qbo-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
qbo-pp-cli accounts get mock-value
```

## Usage

Run `qbo-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts

- **`qbo-pp-cli accounts create`** - Create or update an account
- **`qbo-pp-cli accounts get`** - Get an account by ID

### bills

Manage vendor bills

- **`qbo-pp-cli bills create`** - Create or update a bill
- **`qbo-pp-cli bills get`** - Get a bill by ID

### cdc

Change Data Capture (CDC) endpoints for incremental syncs

- **`qbo-pp-cli cdc`** - Query changed entities since a timestamp

### customers

Manage customers

- **`qbo-pp-cli customers create`** - Create or update a customer
- **`qbo-pp-cli customers get`** - Get a customer by ID

### invoices

Manage invoices

- **`qbo-pp-cli invoices create`** - Create or update an invoice
- **`qbo-pp-cli invoices get`** - Get an invoice by ID

### journal_entries

Manage journal entries

- **`qbo-pp-cli journal-entries create`** - Create or update a journal entry
- **`qbo-pp-cli journal-entries get`** - Get a journal entry by ID

### payments

Manage customer payments

- **`qbo-pp-cli payments create`** - Create or update a payment
- **`qbo-pp-cli payments get`** - Get a payment by ID

### purchases

Manage purchases (Expenses)

- **`qbo-pp-cli purchases create`** - Create or update a purchase
- **`qbo-pp-cli purchases get`** - Get a purchase by ID

### query

Run raw SQL queries against QBO (limitations apply; sync first to query SQLite locally)

- **`qbo-pp-cli query`** - Run raw QuickBooks SQL query

### vendors

Manage vendors

- **`qbo-pp-cli vendors create`** - Create or update a vendor
- **`qbo-pp-cli vendors get`** - Get a vendor by ID


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
qbo-pp-cli accounts get mock-value

# JSON for scripting and agents
qbo-pp-cli accounts get mock-value --json

# Filter to specific fields
qbo-pp-cli accounts get mock-value --json --select id,name,status

# Dry run — show the request without sending
qbo-pp-cli accounts get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
qbo-pp-cli accounts get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
qbo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/qbo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `QBO_CLIENT_ID` | per_call | Yes | Set to your API credential. |
| `QBO_CLIENT_SECRET` | per_call | Yes | Set to your API credential. |
| `QBO_REALM_ID` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `qbo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `qbo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $QBO_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
