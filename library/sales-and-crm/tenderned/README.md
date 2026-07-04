# Tenderned CLI

Every Dutch public tender, with the sub-threshold long tail that EU TED never sees, in a local-first CLI you can pipe.

TenderNed is the Dutch national public procurement platform run by PIANOo / Ministerie van EZK. Every Dutch contracting authority must publish above- and below-threshold tender notices here. Above-threshold notices are forwarded to EU TED; the sub-threshold long tail (€40k–€220k) is TenderNed-only.

This CLI covers the unauthenticated TNS publication webservice (search/list/filter publications, document download, contracting-authority directory, RSS feed) and the Basic-auth eForms XML endpoint. Data is CC-0 public domain.

Created by [@markvandeven](https://github.com/markvandeven) (markvandeven).

## Install

The recommended path installs both the `tenderned-pp-cli` binary and the `pp-tenderned` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tenderned
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tenderned --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tenderned --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tenderned --agent claude-code
npx -y @mvanhorn/printing-press-library install tenderned --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/tenderned/cmd/tenderned-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tenderned-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tenderned --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tenderned --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tenderned --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tenderned --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tenderned-current).
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
    "tenderned": {
      "command": "tenderned-pp-mcp"
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
tenderned-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
tenderned-pp-cli buyers list
```

## Usage

Run `tenderned-pp-cli --help` for the full command reference and flag list.

## Commands

### buyers

Browse contracting authorities (aanbestedende diensten) — Dutch public buyers

- **`tenderned-pp-cli buyers get`** - Fetch one contracting authority by ID
- **`tenderned-pp-cli buyers list`** - List Dutch contracting authorities (paginated)

### docs

List and download tender documents (bestek, PvE, evaluation criteria, Q&A)

- **`tenderned-pp-cli docs download`** - Download all documents for one publication as a zip archive
- **`tenderned-pp-cli docs get`** - Download a single document's binary content (PDF/Word/etc.)
- **`tenderned-pp-cli docs list`** - List attached documents for one publication

### notices

Search, list and fetch tender notices (aankondigingen) from TenderNed — mirrors 'eu-tenders notices' for the Dutch market

- **`tenderned-pp-cli notices get`** - Fetch full structured metadata for one publication
- **`tenderned-pp-cli notices list`** - Search and list tender publications with rich filters (CPV, dates, buyer, procedure, scope)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tenderned-pp-cli buyers list

# JSON for scripting and agents
tenderned-pp-cli buyers list --json

# Filter to specific fields
tenderned-pp-cli buyers list --json --select id,name,status

# Dry run — show the request without sending
tenderned-pp-cli buyers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tenderned-pp-cli buyers list --agent
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
tenderned-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/tenderned-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
