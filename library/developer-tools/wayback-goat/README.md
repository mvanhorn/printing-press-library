# Wayback Goat CLI

Query the Internet Archive's Wayback Machine capture index (CDX Server API). Given a URL, return the list of archived captures (timestamp, original URL, HTTP status, MIME type, and content digest). No authentication required. The content digest (SHA1) enables change detection: consecutive identical digests mean the page did not change; a digest flip marks a real content change.

## Install

The recommended path installs both the `wayback-goat-pp-cli` binary and the `pp-wayback-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install wayback-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/wayback-goat/cmd/wayback-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wayback-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wayback-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wayback-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wayback-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wayback-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/wayback-goat/cmd/wayback-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wayback-goat": {
      "command": "wayback-goat-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
wayback-goat-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
wayback-goat-pp-cli cdx --url https://example.com/resource
```

## Usage

Run `wayback-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### cdx

Manage cdx

- **`wayback-goat-pp-cli cdx`** - Return archived captures the Wayback Machine knows for a URL. Supports field selection, time bounds, match scope, and collapsing.

### changes

Report when a URL's archived content actually changed.

- **`wayback-goat-pp-cli changes <url>`** - Collapse a URL's capture history by content digest and report each point where the page content changed. The first capture is the baseline (first-seen); every later digest flip is a change event. By default only HTTP 200 captures are considered, so a transient redirect or 404 snapshot is not mistaken for a content change. This is the analysis the Wayback web UI does not surface.

```bash
# When did this page's content actually change?
wayback-goat-pp-cli changes example.com

# A specific page, as JSON
wayback-goat-pp-cli changes https://example.com/pricing --json

# Include non-200 captures, or bound the window
wayback-goat-pp-cli changes example.com --all-status --from 20200101 --to 20231231
```


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wayback-goat-pp-cli cdx --url https://example.com/resource

# JSON for scripting and agents
wayback-goat-pp-cli cdx --url https://example.com/resource --json

# Filter to specific fields
wayback-goat-pp-cli cdx --url https://example.com/resource --json --select id,name,status

# Dry run — show the request without sending
wayback-goat-pp-cli cdx --url https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wayback-goat-pp-cli cdx --url https://example.com/resource --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wayback-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/internet-archive-wayback-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
