# Paul Graham CLI

Static public essay index for paulgraham.com.

Created by [@debgotwired](https://github.com/debgotwired) (Deb Mukherjee).

## Install

The recommended path installs both the `paul-graham-pp-cli` binary and the `pp-paul-graham` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install paul-graham
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install paul-graham --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install paul-graham --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install paul-graham --agent claude-code
npx -y @mvanhorn/printing-press-library install paul-graham --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/cmd/paul-graham-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/paul-graham-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install paul-graham --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-paul-graham --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-paul-graham --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install paul-graham --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/paul-graham-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/cmd/paul-graham-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "paul-graham": {
      "command": "paul-graham-pp-mcp"
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
paul-graham-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
paul-graham-pp-cli latest --limit 5
```

## Usage

Run `paul-graham-pp-cli --help` for the full command reference and flag list.

## Commands

### articles-html

Manage articles html

- **`paul-graham-pp-cli articles-html`** - Returns the static HTML essay index from paulgraham.com. The site does not expose an official JSON API; native CLI helpers parse this page and linked essay pages.

### latest

- **`paul-graham-pp-cli latest --limit 10`** - List the newest essays from the canonical essay index.

### list

- **`paul-graham-pp-cli list --query startup --limit 20`** - List essays, optionally filtering by title or slug.

### search

- **`paul-graham-pp-cli search startup --limit 10`** - Search essay titles and slugs.
- **`paul-graham-pp-cli search "default alive" --full-text --json`** - Fetch matching essay pages and search full text.

### read

- **`paul-graham-pp-cli read greatwork`** - Read an essay by slug, URL, title, or title substring.
- **`paul-graham-pp-cli read "Founder Mode" --max-chars 2000 --json`** - Return extracted essay text with word count and excerpt metadata.

### links

- **`paul-graham-pp-cli links greatwork --json`** - Extract links from an essay page.

### random

- **`paul-graham-pp-cli random --seed 42 --json`** - Pick a random essay from the index.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
paul-graham-pp-cli latest --limit 5

# JSON for scripting and agents
paul-graham-pp-cli search startup --json --limit 5

# Filter to specific fields
paul-graham-pp-cli list --json --select title,url

# Dry run — show the request without sending
paul-graham-pp-cli articles-html --dry-run

# Agent mode — JSON + compact + no prompts in one flag
paul-graham-pp-cli read greatwork --agent --max-chars 2000
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
paul-graham-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/paul-graham-essays-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
