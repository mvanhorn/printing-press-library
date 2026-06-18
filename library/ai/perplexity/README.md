# Perplexity CLI

**Browse and export Perplexity research traces from the terminal.**

Perplexity already holds the user's live research trail. This CLI turns that trail into a terminal-first workflow: browse recent threads, inspect full transcripts, and export individual conversations as Markdown, PDF, or DOCX for durable storage in the monorepo.

Learn more at [Perplexity](https://www.perplexity.ai).

Created by [@erikrogne](https://github.com/erikrogne) (Erik Rogne).

## Install

The recommended path installs both the `perplexity-pp-cli` binary and the `pp-perplexity` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install perplexity
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install perplexity --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install perplexity --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install perplexity --agent claude-code
npx -y @mvanhorn/printing-press-library install perplexity --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/perplexity/cmd/perplexity-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/perplexity-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install perplexity --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-perplexity --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-perplexity --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install perplexity --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
perplexity-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/perplexity-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/perplexity/cmd/perplexity-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "perplexity": {
      "command": "perplexity-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Perplexity uses a logged-in browser session for history and export flows. The CLI is designed to work from an authenticated Chrome session so agents can reuse the user's account without relying on the paid API.

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .perplexity.ai in Chrome, then:

```bash
perplexity-pp-cli auth login --chrome
```

Requires a cookie extraction tool. Install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

### 3. Verify Setup

```bash
perplexity-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`history export`** — Export a Perplexity thread as Markdown, PDF, or DOCX from the logged-in browser session.

  ```bash
  perplexity-pp-cli history export --thread-uuid <uuid> --format md
  ```
- **`history read`** — Fetch a full Perplexity thread transcript by UUID or slug.

  ```bash
  perplexity-pp-cli history read <entry_uuid_or_slug> --agent
  ```
- **`history recent`** — List recent Perplexity threads from the signed-in account.

  ```bash
  perplexity-pp-cli history recent --agent
  ```
- **`auth login --chrome`** — Capture the browser session from Chrome instead of asking for a paid API key.

  ```bash
  perplexity-pp-cli auth login --chrome
  ```

## Usage

Run `perplexity-pp-cli --help` for the full command reference and flag list.

## Commands

### history

Perplexity thread history, transcripts, and export helpers.

- **`perplexity-pp-cli history export`** - Export a Perplexity thread as Markdown, PDF, or DOCX.
- **`perplexity-pp-cli history read`** - Fetch a full Perplexity thread transcript by UUID or slug.
- **`perplexity-pp-cli history recent`** - List recent Perplexity threads from the signed-in account.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000 --agent
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
perplexity-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/perplexity-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `perplexity-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
