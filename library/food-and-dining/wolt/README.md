# Wolt CLI

Browse-only Wolt consumer CLI. Lists cities, finds venues near a lat/lon,
searches venues and items, and pulls per-venue delivery/ETA snapshots.

Built on Wolt's unauthenticated consumer endpoints discovered via reverse
engineering (OzTamir gist + live browser-sniff). No login, no cart, no orders.

Printed by [@Amit-Tabibi](https://github.com/Amit-Tabibi) (Amit).

## Install

The recommended path installs both the `wolt-pp-cli` binary and the `pp-wolt` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wolt
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wolt --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wolt --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wolt --agent claude-code
npx -y @mvanhorn/printing-press-library install wolt --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wolt-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wolt --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wolt --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-wolt skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-wolt. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wolt-current).
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
    "wolt": {
      "command": "wolt-pp-mcp"
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
wolt-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
wolt-pp-cli cities
```

## Usage

Run `wolt-pp-cli --help` for the full command reference and flag list.

## Commands

### cities

Manage cities

- **`wolt-pp-cli cities`** - Returns the global list of Wolt cities with country code, slug, timezone,
and coordinates. Backbone of the local store.

### order-xp

Manage order xp

- **`wolt-pp-cli order-xp <slug>`** - Per-venue dynamic snapshot: open_status, delivery_open_status, time_slots_schedule,
delivery_configs, banners. Combined with LD+JSON SSR scrape in Phase 3 for full
venue metadata (name, address, phone, rating, cuisine).

### pages

Manage pages

- **`wolt-pp-cli pages list-restaurants-near`** - Returns the discovery page for a given location: venues organized in sections
with name, slug, online status, ETA, delivery fee, rating, tags.
- **`wolt-pp-cli pages search`** - POST with body {q, target, lat, lon}. Set target="venues" to search restaurants,
target="items" to search dishes across venues.

### venues

Manage venues

- **`wolt-pp-cli venues <slug>`** - Documented community endpoint. As of the last sniff, returns HTTP 200 with
empty body via CloudFront cache. The CLI surfaces this honestly and includes
--debug to capture the failing request for upstream investigation.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wolt-pp-cli cities

# JSON for scripting and agents
wolt-pp-cli cities --json

# Filter to specific fields
wolt-pp-cli cities --json --select id,name,status

# Dry run — show the request without sending
wolt-pp-cli cities --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wolt-pp-cli cities --agent
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
wolt-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/wolt-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
