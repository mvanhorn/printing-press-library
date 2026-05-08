# PP Clarity CLI

**Generate and audit Microsoft Clarity browser instrumentation from the terminal.**

Microsoft Clarity's client API is made of JavaScript calls and HTML attributes, so this CLI treats it as an instrumentation assistant instead of a fake REST wrapper. It can fetch the public tag script, render copy-safe snippets, and audit local HTML for the calls Microsoft documents.

Learn more at [PP Clarity](https://clarity.microsoft.com).

## Install

The recommended path installs both the `clarity-pp-cli` binary and the `pp-clarity` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install clarity
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install clarity --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/clarity/cmd/clarity-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clarity-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clarity --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clarity --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-clarity skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-clarity. The skill defines how its required CLI can be installed.
```

## Authentication

No server API token is required for the documented Clarity client API. Your Clarity project ID is embedded in the browser tracking snippet and acts as the setup key for client-side instrumentation.

The Data Export API does use a bearer token. Generate it in Microsoft Clarity under Settings -> Data Export -> Generate new API token, then keep it out of chat and source control:

```bash
export PP_CLARITY_API_TOKEN="..."
```

For local testing with an agent, a token file is usually easier because the agent can read the same filesystem but not your shell environment:

```bash
mkdir -p ~/.config/clarity-pp-cli
printf '%s' 'YOUR_TOKEN_HERE' > ~/.config/clarity-pp-cli/api-token
chmod 600 ~/.config/clarity-pp-cli/api-token
```

`clarity-pp-cli insights live` also accepts `MICROSOFT_CLARITY_API_TOKEN`, `CLARITY_API_TOKEN`, or `PP_CLARITY_API_TOKEN_FILE`.

## Quick Start

```bash
# Render the install snippet for your Clarity project ID.
clarity-pp-cli snippet install abc123 --format html


# Generate a custom identifiers call.
clarity-pp-cli snippet identify user-42 --session-id sess-9 --page-id checkout --friendly-name "Paid customer"


# Check local HTML before deployment.
clarity-pp-cli audit html ./index.html --json --select found_project_id,calls


# Fetch live dashboard export data using PP_CLARITY_API_TOKEN.
clarity-pp-cli insights live --days 1 --dimension OS --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Instrumentation authoring
- **`snippet install`** — Render a complete Clarity tracking snippet for a project ID, plus focused snippets for every documented client API call.

  _Use this when adding Clarity to a site or handing implementation-ready code to another agent._

  ```bash
  clarity-pp-cli snippet install abc123 --format html
  ```

### Instrumentation review
- **`audit html`** — Inspect an HTML file for a Clarity tag script, masking attributes, and common window.clarity client API calls.

  _Use this before shipping page changes that are supposed to include Clarity instrumentation._

  ```bash
  clarity-pp-cli audit html ./index.html --json --select found_project_id,calls
  ```

## Usage

Run `clarity-pp-cli --help` for the full command reference and flag list.

## Commands

### tag

Inspect the Microsoft Clarity tracking tag script

- **`clarity-pp-cli tag get`** - Fetch the Clarity tracking tag script for a project ID

### insights

Read Microsoft Clarity Data Export API insights.

- **`clarity-pp-cli insights live`** - Fetch project live insights from the Microsoft Clarity Data Export API.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clarity-pp-cli tag mock-value

# JSON for scripting and agents
clarity-pp-cli tag mock-value --json

# Filter to specific fields
clarity-pp-cli tag mock-value --json --select id,name,status

# Dry run — show the request without sending
clarity-pp-cli tag mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clarity-pp-cli tag mock-value --agent
```

## Live Data Export

The Data Export API is read-only but quota-limited by Microsoft to 10 requests per project per day. Use `--dry-run` first to verify the exact request without spending quota:

```bash
clarity-pp-cli insights live --days 1 --dimension OS --dry-run
clarity-pp-cli insights live --days 1 --dimension OS --json
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-clarity -g
```

Then invoke `/pp-clarity <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/clarity/cmd/clarity-pp-mcp@latest
```

Then register it:

```bash
claude mcp add pp-clarity clarity-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clarity-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/clarity/cmd/clarity-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pp-clarity": {
      "command": "clarity-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
clarity-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/clarity-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **No sessions appear in Clarity after install** — Confirm the project ID in the generated snippet matches the project under Settings -> Setup and check for POST requests to https://www.clarity.ms/collect in the browser network panel.
- **Sensitive content appears masked or unmasked unexpectedly** — Use snippet mask to generate the exact data-clarity-mask or data-clarity-unmask attribute and audit the HTML to confirm it is present on the intended element.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Microsoft Learn Clarity client API reference**](https://learn.microsoft.com/en-us/clarity/setup-and-installation/clarity-api)
- [**Microsoft Learn Clarity setup guide**](https://learn.microsoft.com/en-us/clarity/setup-and-installation/clarity-setup)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
