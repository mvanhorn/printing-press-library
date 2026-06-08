# Continente CLI

Storefront product and cart workflows observed on `continente.pt`.
The CLI currently runs on storefront HTML and fragment surfaces, but it probes for better structured commerce contracts at runtime and reports whether it is operating in preferred or degraded mode via `doctor`.

## Publication Status

This repository is a working source checkout for the `continente` Printing Press CLI.

- Source build and local validation are supported now.
- The repo is not yet published to the public Printing Press library.
- Pre-built release binaries, MCP bundles, and one-command library install flows below are future-state instructions for the published artifact.

## Install

### From source (current path)

```bash
go build -o bin/continente-pp-cli ./cmd/continente-pp-cli
go build -o bin/continente-pp-mcp ./cmd/continente-pp-mcp
```

Or use the repo targets:

```bash
make build-all
make test
make verify-release
```

### From the Printing Press library (after publication)

The recommended path installs both the `continente-pp-cli` binary and the `pp-continente` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install continente
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install continente --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install continente --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install continente --agent claude-code
npx -y @mvanhorn/printing-press-library install continente --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/continente-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-continente --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-continente --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-continente skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-continente. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/continente-current).
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
    "continente": {
      "command": "continente-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

Build from source or, after publication, use one of the [Install](#install) flows above.

### 2. Verify Setup

```bash
continente-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
continente-pp-cli pesquisa --q leite
```

## Usage

Run `continente-pp-cli --help` for the full command reference and flag list.

## Commands

### suggest

Structured storefront suggestions

- **`continente-pp-cli suggest --q leite`** - Get structured search suggestions

### pesquisa

Structured storefront product search

- **`continente-pp-cli pesquisa --q leite`** - Search products and return parsed result items

### produto

Structured product detail lookup

- **`continente-pp-cli produto <slugAndPid>`** - Get parsed product details from a product page


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
continente-pp-cli pesquisa --q leite

# JSON for scripting and agents
continente-pp-cli pesquisa --q leite --json

# Filter to specific fields
continente-pp-cli pesquisa --q leite --json --select items

# Dry run — show the request without sending
continente-pp-cli suggest --q leite --dry-run

# Agent mode — JSON + compact + no prompts in one flag
continente-pp-cli produto leite-uht-meio-gordo-mimosa-mimosa-7745833 --agent

# Agent mode with an even narrower subset
continente-pp-cli checkout slots --agent --select results.slots.slot_ref,results.slots.date_label,results.slots.start,results.slots.end
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Lower-token by default** - `--agent` now implies compact command-aware JSON plus `--meta minimal`
- **Previewable** - `--dry-run` shows the request without sending
- **Guest-first with optional session auth** - product discovery works anonymously; cart auth can be imported from a local browser session
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

For richer debugging context, add `--meta full`. To drop the provenance envelope entirely on structured commands, add `--meta none`.

## Health Check

```bash
continente-pp-cli doctor
```

Verifies configuration and connectivity to the API.
It also reports contract mode and whether structured commerce surfaces were detected or the CLI is falling back to storefront HTML.

## Contributor Validation

Use the repo-native targets before opening changes:

```bash
make verify-release
```

To run the Printing Press publication gate locally:

```bash
go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@v4.20.1
make validate-publish
```

For the distinction between source-ready, public-repo-ready, and Printing Press publish-ready states, see [docs/release-readiness.md](docs/release-readiness.md).

## Configuration

Config file: `~/.config/continente-pp-cli/config.toml`

Cookie jar path defaults to `~/.local/share/continente-pp-cli/cookies.json`.

### Imported Session Auth

Import a local browser HAR into the CLI cookie jar:

```bash
continente-pp-cli auth import-har --file ~/downloads/www.continente.pt.har
continente-pp-cli auth import-cookies --file ~/downloads/continente-cookies.json
continente-pp-cli auth status
```

Log in directly and persist a fresh storefront session:

```bash
printf '%s\n' 'your-password' | continente-pp-cli auth login --email you@example.com --password-stdin
continente-pp-cli auth status
```

Then inspect or mutate the current cart:

```bash
continente-pp-cli cart mini --json
continente-pp-cli cart update --pid 8061027 --uuid acc361ada6f3c403d8876d594e --quantity 2
continente-pp-cli cart remove --pid 8061027 --uuid acc361ada6f3c403d8876d594e
```

Inspect the authenticated pickup checkout and select a delivery slot:

```bash
continente-pp-cli checkout status --json
continente-pp-cli checkout stores
continente-pp-cli checkout slots
continente-pp-cli checkout select-slot --slot-ref 1.2
```

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
