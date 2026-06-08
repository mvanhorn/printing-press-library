# Substack Notes CLI

Unofficial Substack Notes endpoints observed from the authenticated web app.

Learn more at [Substack Notes](https://substack.com).

Created by [@petergyang](https://github.com/petergyang) (Peter Yang).

## Install

The recommended path installs both the `substack-notes-pp-cli` binary and the `pp-substack-notes` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install substack-notes
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install substack-notes --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install substack-notes --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install substack-notes --agent claude-code
npx -y @mvanhorn/printing-press-library install substack-notes --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-notes-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install substack-notes --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-substack-notes --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-substack-notes --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install substack-notes --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/substack-notes-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUBSTACK_NOTES_COOKIE_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "substack-notes": {
      "command": "substack-notes-pp-mcp",
      "env": {
        "SUBSTACK_NOTES_COOKIE_AUTH": "<cookie-header>"
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

This CLI uses your own signed-in Substack browser session. Substack does not provide an official Notes API token or OAuth flow.

Sign in to Substack in a supported local browser, then run:

```bash
substack-notes-pp-cli auth login --browser chrome
```

Supported browsers: `chrome`, `brave`, and `arc` on macOS Chromium-family profiles. If browser discovery cannot read your profile or keychain, use the advanced fallback:

```bash
substack-notes-pp-cli auth setup
export SUBSTACK_NOTES_COOKIE_AUTH="<cookie-header>"
```

You can also persist the cookie header with `substack-notes-pp-cli auth set-token "<cookie-header>"`. Treat this value like a password. Do not commit it, paste it into shared chats, or include it in screenshots.

### 3. Verify Setup

```bash
substack-notes-pp-cli notes recent --limit 5 --json
```

Start with a read-only check before any write command.

### 4. Try Your First Command

```bash
substack-notes-pp-cli notes draft --text "Draft from the CLI" --json
```

## Usage

Run `substack-notes-pp-cli --help` for the full command reference and flag list.

## Commands

### comment

Raw Substack comment endpoints, mostly for power users and MCP compatibility.

- **`substack-notes-pp-cli comment delete-draft`** - Delete a note draft
- **`substack-notes-pp-cli comment publish-note`** - Publish a note immediately
- **`substack-notes-pp-cli comment save-draft`** - Save or schedule a note draft

### notes

Friendly Note workflows.

- **`substack-notes-pp-cli notes recent --limit 5 --json`** - Read recent published Notes from your authenticated profile
- **`substack-notes-pp-cli notes post --text "Short note"`** - Publish a text Note
- **`substack-notes-pp-cli notes post --text "Image note" --image ./image.png`** - Publish a Note with an image attachment
- **`substack-notes-pp-cli notes draft --file note.txt --image ./image.jpg`** - Save an image-backed draft
- **`substack-notes-pp-cli notes schedule --text "Tomorrow" --at "2026-07-15 09:00"`** - Schedule a Note
- **`substack-notes-pp-cli notes list --limit 20`** - List drafts and scheduled Notes
- **`substack-notes-pp-cli notes delete <draft-id>`** - Delete a draft or scheduled Note

### feed

Manage feed

- **`substack-notes-pp-cli feed`** - List note drafts and scheduled notes


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
substack-notes-pp-cli notes recent --limit 5

# JSON for scripting and agents
substack-notes-pp-cli notes recent --limit 5 --json

# Filter to specific fields
substack-notes-pp-cli notes recent --limit 5 --json --select id,body,canonical_url

# Dry run — show the request without sending
substack-notes-pp-cli notes post --text "Preview" --dry-run

# Agent mode — JSON + compact + no prompts in one flag
substack-notes-pp-cli notes recent --limit 5 --agent
```

`--dry-run` can preview text-only writes. It cannot upload images because Substack must return a real attachment id before the final draft or publish request can be formed.

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
substack-notes-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/substack-notes-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUBSTACK_NOTES_COOKIE_AUTH` | per_call | Yes | Cookie header from your own authenticated Substack session. Sensitive. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `substack-notes-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `substack-notes-pp-cli doctor` to check credentials
- Run `substack-notes-pp-cli auth login --browser chrome` after signing in to Substack
- If you use the manual fallback, verify that `SUBSTACK_NOTES_COOKIE_AUTH` is set without printing its value in shared logs
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
