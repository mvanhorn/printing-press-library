# Alltrails CLI

Evidence-labeled, live-capable route map for AllTrails browser/mobile surfaces. Not an official API contract.

Learn more at [Alltrails](https://www.alltrails.com).

Created by [@zaydiscold](https://github.com/zaydiscold) (zaydiscold).

## Install

The recommended path installs both the `alltrails-pp-cli` binary and the `pp-alltrails` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install alltrails
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install alltrails --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install alltrails --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install alltrails --agent claude-code
npx -y @mvanhorn/printing-press-library install alltrails --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/alltrails/cmd/alltrails-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alltrails-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install alltrails --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-alltrails --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-alltrails --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install alltrails --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alltrails-current).
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
    "alltrails": {
      "command": "alltrails-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Configure Auth

Public reads may work without credentials, but account routes need caller-owned AllTrails auth:

```bash
export ALLTRAILS_ACCESS_TOKEN="..."
# or
export ALLTRAILS_COOKIE="..."
export ALLTRAILS_CSRF_TOKEN="..." # only for browser-backed writes that require CSRF
```

### 3. Verify Setup

```bash
alltrails-pp-cli doctor
```

This checks your configuration.

### 4. Try Your First Command

```bash
alltrails-pp-cli alltrails list
```

## Usage

Run `alltrails-pp-cli --help` for the full command reference and flag list.

## Commands

### alltrails

Manage alltrails

- **`alltrails-pp-cli alltrails create`** - Upload a new activity recording
- **`alltrails-pp-cli alltrails get`** - Activity detail with GPS/stat surfaces
- **`alltrails-pp-cli alltrails get-v3`** - Trail detail payload
- **`alltrails-pp-cli alltrails get-v3-2`** - Offline map metadata
- **`alltrails-pp-cli alltrails get-v3-3`** - Map static image metadata/render endpoint
- **`alltrails-pp-cli alltrails get-v3-4`** - Trail static map image metadata/render endpoint
- **`alltrails-pp-cli alltrails get-v3-5`** - User activity list
- **`alltrails-pp-cli alltrails list`** - Trail search by text, location, and filters
- **`alltrails-pp-cli alltrails list-v3`** - Current account profile

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
alltrails-pp-cli alltrails list

# JSON for scripting and agents
alltrails-pp-cli alltrails list --json

# Filter to specific fields
alltrails-pp-cli alltrails list --json --select id,name,status

# Dry run — show the request without sending
alltrails-pp-cli alltrails list --dry-run

# Write routes default to dry-run; live writes require both controls
ALLTRAILS_PP_ALLOW_WRITES=1 alltrails-pp-cli alltrails create --live-write --stdin < activity.json

# Agent mode — JSON + compact + no prompts in one flag
alltrails-pp-cli alltrails list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
alltrails-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/alltrails-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Auth Environment

- `ALLTRAILS_ACCESS_TOKEN` sets an `Authorization: Bearer ...` header.
- `ALLTRAILS_COOKIE` sets the browser Cookie header for authenticated AllTrails routes.
- `ALLTRAILS_CSRF_TOKEN` sets `x-csrftoken` for browser-backed write routes when needed.

### Write Safety

Reads run without a write gate. Any non-read route is labeled with `pp:risk` and `mcp:risk`, defaults to `--dry-run` in the CLI unless `--live-write` is passed, and is blocked at the HTTP transport unless `ALLTRAILS_PP_ALLOW_WRITES=1` is present. MCP write tools also default to dry-run unless the same env gate is set.

See [docs/write-operations.md](docs/write-operations.md) for the exact live-write contract.

### Sync

AllTrails browser APIs are DataDome-protected outside the logged-in browser context, so `sync` defaults to a safe no-op. To try a resource explicitly, pass `--resources routing_info` with caller-owned auth/cookies configured.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses standard Go HTTP transport with HTTP/2 enabled when AllTrails negotiates it. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
