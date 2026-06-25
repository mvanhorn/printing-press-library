# Postscript CLI

Compact OpenAPI spec assembled from the official Postscript ReadMe endpoint pages listed in https://developers.postscript.io/llms.txt.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `postscript-pp-cli` binary and the `pp-postscript` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install postscript
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install postscript --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install postscript --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install postscript --agent claude-code
npx -y @mvanhorn/printing-press-library install postscript --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/postscript/cmd/postscript-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postscript-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install postscript --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-postscript --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-postscript --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install postscript --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postscript-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `POSTSCRIPT_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/postscript/cmd/postscript-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "postscript": {
      "command": "postscript-pp-mcp",
      "env": {
        "POSTSCRIPT_API_KEY": "<your-key>"
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
export POSTSCRIPT_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/postscript-partner-pp-cli/config.toml`.

### 3. Verify Setup

```bash
postscript-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
postscript-pp-cli subscribers list
```

## Usage

Run `postscript-pp-cli --help` for the full command reference and flag list.

## Agent Discovery

Inspect the agent command-discovery surface:

```bash
postscript-pp-cli which --help
```

Inspect the structured agent metadata command:

```bash
postscript-pp-cli agent-context --help
```

## Commands

### compliance

Unsubscribe and redact compliance actions.

- **`postscript-pp-cli compliance redact`** - Redacts subscriber data. When passed an email, phone number, Shopify customer ID, or Postscript subscriber ID, this endpoint also unsubscribes an active subscriber.
- **`postscript-pp-cli compliance unsubscribe`** - Opts subscribers out of Postscript messaging.

### events

Custom event ingestion for flows.

- **`postscript-pp-cli events`** - Send a custom event to use in Postscript flows.

### subscribers

Subscriber lookup and profile updates.

- **`postscript-pp-cli subscribers get`** - Get an individual subscriber for a shop.
- **`postscript-pp-cli subscribers list`** - Get a list of subscribers for a shop.
- **`postscript-pp-cli subscribers update`** - Updates data for a given subscriber.

## Local Analysis

Inspect the local SQLite sync framework:

```bash
postscript-pp-cli sync --help
```

Inspect the read-only SQL analytics surface:

```bash
postscript-pp-cli analytics --help
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
postscript-pp-cli subscribers list

# JSON for scripting and agents
postscript-pp-cli subscribers list --json

# Filter to specific fields
postscript-pp-cli subscribers list --json --select id,name,status

# Dry run — show the request without sending
postscript-pp-cli subscribers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
postscript-pp-cli subscribers list --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
postscript-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/postscript-partner-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POSTSCRIPT_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `postscript-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `postscript-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POSTSCRIPT_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
