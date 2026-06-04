# Marginal Revolution CLI

Read and filter Marginal Revolution from the command line through its public RSS feed.

This CLI was regenerated with CLI Printing Press 4.2.1. It keeps the generated v4.2 agent/MCP scaffolding and adds RSS-native helper commands for posts, authors, categories, and outbound links.

Created by [@hinuri](https://github.com/hinuri) (Nuri Chang).

## Install

The recommended path installs both the `marginalrevolution-pp-cli` binary and the `pp-marginalrevolution` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install marginalrevolution
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install marginalrevolution --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install marginalrevolution --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install marginalrevolution --agent claude-code
npx -y @mvanhorn/printing-press-library install marginalrevolution --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/cmd/marginalrevolution-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/marginalrevolution-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-marginalrevolution --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-marginalrevolution --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install marginalrevolution --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/marginalrevolution-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "marginalrevolution": {
      "command": "marginalrevolution-pp-mcp"
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
marginalrevolution-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
marginalrevolution-pp-cli latest --limit 5
```

## Usage

Run `marginalrevolution-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Manage feed

- **`marginalrevolution-pp-cli feed get`** - Returns the current public RSS feed. The feed is XML/RSS and does not require authentication.

### RSS-native helpers

- **`marginalrevolution-pp-cli latest`** - list recent posts with dates, authors, categories, comment counts, and canonical URLs
- **`marginalrevolution-pp-cli search <query>`** - search current-feed title, excerpt, body text, author, and category text
- **`marginalrevolution-pp-cli read <url|guid|title>`** - read the body text for a post currently present in the feed
- **`marginalrevolution-pp-cli links`** - extract outbound cited links from recent posts
- **`marginalrevolution-pp-cli categories`** - show current-feed category counts
- **`marginalrevolution-pp-cli authors`** - show current-feed author counts

## Scope

The public RSS feed is available to command-line clients. Marginal Revolution's WordPress JSON API and normal site-search URL returned Cloudflare browser challenges during implementation, so this CLI intentionally keeps search scoped to posts currently present in the feed.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
marginalrevolution-pp-cli latest

# JSON for scripting and agents
marginalrevolution-pp-cli latest --json

# Filter to specific fields
marginalrevolution-pp-cli feed --json --select id,name,status

# Dry run — show the request without sending
marginalrevolution-pp-cli feed --dry-run

# Agent mode — JSON + compact + no prompts in one flag
marginalrevolution-pp-cli search ai --agent
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
marginalrevolution-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/marginal-revolution-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
