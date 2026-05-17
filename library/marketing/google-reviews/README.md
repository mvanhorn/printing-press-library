# Google Reviews CLI

Discovered API spec for google

Learn more at [Google Reviews](https://www.google.com).

Printed by [@dahliasan](https://github.com/dahliasan) (dahlia).

## Install

The recommended path installs both the `google-reviews-pp-cli` binary and the `pp-google-reviews` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install google-reviews
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-reviews --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install google-reviews --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install google-reviews --agent claude-code
npx -y @mvanhorn/printing-press install google-reviews --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-reviews-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-reviews --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-reviews --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-reviews skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-reviews. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-reviews-current).
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
    "google-reviews": {
      "command": "google-reviews-pp-mcp"
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
google-reviews-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
google-reviews-pp-cli maps list-listentitiesreviews
```

## Usage

Run `google-reviews-pp-cli --help` for the full command reference and flag list.

## Commands

### maps

Operations on listentitiesreviews

- **`google-reviews-pp-cli maps list-listentitiesreviews`** - GET /maps/preview/review/listentitiesreviews
- **`google-reviews-pp-cli maps list-place`** - GET /maps/preview/place


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-reviews-pp-cli maps list-listentitiesreviews

# JSON for scripting and agents
google-reviews-pp-cli maps list-listentitiesreviews --json

# Filter to specific fields
google-reviews-pp-cli maps list-listentitiesreviews --json --select id,name,status

# Dry run — show the request without sending
google-reviews-pp-cli maps list-listentitiesreviews --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-reviews-pp-cli maps list-listentitiesreviews --agent
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
google-reviews-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/google-reviews-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.google.com/maps/preview/review/listentitiesreviews
- Capture coverage: 2 API entries from 2 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: api_key — query: authuser
- Generation hints: weak_schema_confidence
- Candidate command ideas: list_listentitiesreviews — Derived from observed GET /maps/preview/review/listentitiesreviews traffic.; list_place — Derived from observed GET /maps/preview/place traffic.

Warnings from discovery:
- raw_protocol_envelope: Response contains raw RPC transport markers that should be decoded before user-facing output.
- raw_protocol_envelope: Response contains raw RPC transport markers that should be decoded before user-facing output.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
