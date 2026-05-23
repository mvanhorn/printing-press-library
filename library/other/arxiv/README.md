# arXiv CLI

Public Atom API for searching and fetching arXiv e-print metadata.

Printed by [@hiten-shah](https://github.com/hiten-shah) (Hiten Shah).

## Install

The recommended path installs both the `arxiv-pp-cli` binary and the `pp-arxiv` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install arxiv
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install arxiv --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install arxiv --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install arxiv --agent claude-code
npx -y @mvanhorn/printing-press-library install arxiv --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/arxiv/cmd/arxiv-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arxiv-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-arxiv --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-arxiv --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-arxiv skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-arxiv. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/arxiv-current).
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
    "arxiv": {
      "command": "arxiv-pp-mcp"
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
arxiv-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
arxiv-pp-cli query --search-query 'cat:cs.AI' --max-results 5 --sort-by submittedDate --sort-order descending
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Research discovery

- **`query`** — Search arXiv with documented query expressions and agent-friendly output controls.
- **`query`** — Fetch latest AI/research papers by category using submitted-date sorting and bounded result counts.
- **`query`** — Fetch exact papers by arXiv ID or versioned arXiv ID.

## Usage

Run `arxiv-pp-cli --help` for the full command reference and flag list.

## Commands

### query

Manage query

- **`arxiv-pp-cli query --search-query 'cat:cs.AI' --max-results 5`** - Search arXiv papers or fetch recent papers by category.
- **`arxiv-pp-cli query --id-list 1706.03762 --max-results 1`** - Fetch exact papers by arXiv ID or versioned arXiv ID.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
arxiv-pp-cli query --search-query 'cat:cs.AI' --max-results 5 --sort-by submittedDate --sort-order descending

# JSON for scripting and agents
arxiv-pp-cli query --id-list 1706.03762 --json

# Filter to specific fields
arxiv-pp-cli query --id-list 1706.03762 --json --select entries.id,entries.title

# Dry run — show the request without sending
arxiv-pp-cli query --search-query 'cat:cs.CL' --max-results 1 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
arxiv-pp-cli query --search-query 'all:electron' --max-results 3 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select entries.id,entries.title` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Live-first** - arXiv search is most useful against the live API; generic sync/local-store commands are present from the scaffold but `/api/query` requires caller-supplied search or ID parameters
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
arxiv-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/arxiv-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
