# Salesloft CLI

Read-first Salesloft API surface for agent workflows, based on the public Salesloft v2 developer documentation.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `salesloft-pp-cli` binary and the `pp-salesloft` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install salesloft
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install salesloft --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install salesloft --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install salesloft --agent claude-code
npx -y @mvanhorn/printing-press-library install salesloft --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/cmd/salesloft-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/salesloft-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install salesloft --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-salesloft --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-salesloft --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install salesloft --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/salesloft-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SALESLOFT_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/cmd/salesloft-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "salesloft": {
      "command": "salesloft-pp-mcp",
      "env": {
        "SALESLOFT_BEARER_AUTH": "<your-key>"
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
salesloft-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export SALESLOFT_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
salesloft-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
salesloft-pp-cli accounts list
```

## Usage

Run `salesloft-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts

- **`salesloft-pp-cli accounts get`** - Get a Salesloft account by ID.
- **`salesloft-pp-cli accounts list`** - List Salesloft accounts.

### activities

Manage activities

- **`salesloft-pp-cli activities`** - List recent Salesloft activities.

### cadences

Manage cadences

- **`salesloft-pp-cli cadences get`** - Get a Salesloft cadence by ID.
- **`salesloft-pp-cli cadences list`** - List Salesloft cadences.

### calls

Manage calls

- **`salesloft-pp-cli calls get`** - Get a Salesloft call by ID.
- **`salesloft-pp-cli calls list`** - List Salesloft calls.

### me

Manage me

- **`salesloft-pp-cli me`** - Get the authenticated Salesloft user.

### meetings

Manage meetings

- **`salesloft-pp-cli meetings`** - List Salesloft meetings.

### notes

Manage notes

- **`salesloft-pp-cli notes get`** - Get a Salesloft note by ID.
- **`salesloft-pp-cli notes list`** - List Salesloft notes.

### opportunities

Manage opportunities

- **`salesloft-pp-cli opportunities get-opportunity`** - Get a Salesloft opportunity by ID.
- **`salesloft-pp-cli opportunities list`** - List Salesloft opportunities.

### people

Manage people

- **`salesloft-pp-cli people get-person`** - Get a Salesloft person by ID.
- **`salesloft-pp-cli people list`** - List Salesloft people.

### tasks

Manage tasks

- **`salesloft-pp-cli tasks get`** - Get a Salesloft task by ID.
- **`salesloft-pp-cli tasks list`** - List Salesloft tasks.

### team

Manage team

- **`salesloft-pp-cli team`** - Get the authenticated user's Salesloft team.

### users

Manage users

- **`salesloft-pp-cli users get`** - Get a Salesloft user by ID.
- **`salesloft-pp-cli users list`** - List Salesloft users.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
salesloft-pp-cli accounts list

# JSON for scripting and agents
salesloft-pp-cli accounts list --json

# Filter to specific fields
salesloft-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
salesloft-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
salesloft-pp-cli accounts list --agent
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
salesloft-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/salesloft-read-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SALESLOFT_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `salesloft-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `salesloft-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SALESLOFT_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
