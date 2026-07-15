# Calendly CLI

Read-first Calendly API surface for scheduling, availability, event, invitee, and webhook inspection.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `calendly-pp-cli` binary and the `pp-calendly` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install calendly
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install calendly --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install calendly --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install calendly --agent claude-code
npx -y @mvanhorn/printing-press-library install calendly --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/calendly/cmd/calendly-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/calendly-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install calendly --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-calendly --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-calendly --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install calendly --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/calendly-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CALENDLY_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/calendly/cmd/calendly-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "calendly": {
      "command": "calendly-pp-mcp",
      "env": {
        "CALENDLY_BEARER_AUTH": "<your-key>"
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
calendly-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export CALENDLY_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
calendly-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z
```

## Usage

Run `calendly-pp-cli --help` for the full command reference and flag list.

## Commands

### event-type-available-times

Manage event type available times

- **`calendly-pp-cli event-type-available-times`** - List available times for an event type.

### event-type-memberships

Manage event type memberships

- **`calendly-pp-cli event-type-memberships`** - List event type memberships.

### event-types

Manage event types

- **`calendly-pp-cli event-types get`** - Get an event type by UUID.
- **`calendly-pp-cli event-types list`** - List Calendly event types.

### organization-memberships

Manage organization memberships

- **`calendly-pp-cli organization-memberships get`** - Get an organization membership by UUID.
- **`calendly-pp-cli organization-memberships list`** - List organization memberships.

### organizations

Manage organizations

- **`calendly-pp-cli organizations <uuid>`** - Get a Calendly organization by UUID.

### routing-forms

Manage routing forms

- **`calendly-pp-cli routing-forms get`** - Get a routing form by UUID.
- **`calendly-pp-cli routing-forms list`** - List routing forms.

### scheduled-events

Manage scheduled events

- **`calendly-pp-cli scheduled-events get`** - Get a scheduled event by UUID.
- **`calendly-pp-cli scheduled-events list`** - List scheduled events.

### user-busy-times

Manage user busy times

- **`calendly-pp-cli user-busy-times`** - List busy times for a user.

### users

Manage users

- **`calendly-pp-cli users get`** - Get a Calendly user by UUID.
- **`calendly-pp-cli users get-current`** - Get the authenticated Calendly user.

### webhook-subscriptions

Manage webhook subscriptions

- **`calendly-pp-cli webhook-subscriptions get`** - Get a webhook subscription by UUID.
- **`calendly-pp-cli webhook-subscriptions list`** - List webhook subscriptions.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z

# JSON for scripting and agents
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z --json

# Filter to specific fields
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z --json --select id,name,status

# Dry run — show the request without sending
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z --dry-run

# Agent mode — JSON + compact + no prompts in one flag
calendly-pp-cli event-type-available-times --event-type example-value --start-time 2026-01-15T09:00:00Z --end-time 2026-01-15T09:00:00Z --agent
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
calendly-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/calendly-read-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CALENDLY_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `calendly-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `calendly-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CALENDLY_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
