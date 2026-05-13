# Cf Domain CLI

Agent-native CLI for Cloudflare Registrar domain search, check, and registration.

## Install

The recommended path installs both the `cf-domain-pp-cli` binary and the `pp-cf-domain` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cf-domain
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cf-domain --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cf-domain-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cf-domain --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cf-domain --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cf-domain skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cf-domain. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
cf-domain-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
cf-domain-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource
```

## Usage

Run `cf-domain-pp-cli --help` for the full command reference and flag list.

## Commands

### registrar

Manage registrar

- **`cf-domain-pp-cli registrar domain-check`** - Real-time domain availability and pricing check. Must be run immediately before registration.
- **`cf-domain-pp-cli registrar domain-register`** - Register a domain through Cloudflare Registrar using account default registrant/payment settings. Dangerous: caller must confirm exact domain and price before this request.
- **`cf-domain-pp-cli registrar domain-search`** - Search Cloudflare Registrar for domain suggestions and availability hints. Use domain-check for live final pricing before registering.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource

# JSON for scripting and agents
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource --json

# Filter to specific fields
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource --json --select id,name,status

# Dry run — show the request without sending
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cf-domain-pp-cli registrar domain-check mock-value --domain-name example-resource --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cf-domain -g
```

Then invoke `/pp-cf-domain <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cf-domain cf-domain-pp-mcp -e CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cf-domain-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cf-domain": {
      "command": "cf-domain-pp-mcp",
      "env": {
        "CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cf-domain-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cloudflare-registrar-domains-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cf-domain-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLOUDFLARE_REGISTRAR_DOMAINS_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
