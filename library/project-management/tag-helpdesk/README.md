# Tag Helpdesk CLI

Odoo 18 CE helpdesk ticket management for tag.msg.it.

Connects to the Odoo XML-RPC external API at /xmlrpc/2/common (auth)
and /xmlrpc/2/object (ORM). Syncs helpdesk.ticket records into a local
SQLite cache for offline analysis and Claude-native pipelines.

Auth: set ODOO_URL, ODOO_DB, ODOO_USER, ODOO_API_KEY environment variables.
Generate an API key in Odoo via Settings → Users → your user → Account Security.

Printed by [@andreampiovesana](https://github.com/andreampiovesana) (Andrea M. Piovesana).

## Install

The recommended path installs both the `tag-helpdesk-pp-cli` binary and the `pp-tag-helpdesk` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install tag-helpdesk
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tag-helpdesk --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tag-helpdesk-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tag-helpdesk --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tag-helpdesk --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tag-helpdesk skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tag-helpdesk. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export TAG_HELPDESK_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/tag-helpdesk-pp-cli/config.toml`.

### 3. Verify Setup

```bash
tag-helpdesk-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
tag-helpdesk-pp-cli xmlrpc authenticate
```

## Usage

Run `tag-helpdesk-pp-cli --help` for the full command reference and flag list.

## Commands

### xmlrpc

Manage xmlrpc

- **`tag-helpdesk-pp-cli xmlrpc authenticate`** - Calls common.authenticate(db, username, api_key, {}) via XML-RPC.
Returns integer UID used in all subsequent object calls.
- **`tag-helpdesk-pp-cli xmlrpc count-tickets`** - Count tickets matching a domain
- **`tag-helpdesk-pp-cli xmlrpc create-ticket`** - Create a new helpdesk ticket
- **`tag-helpdesk-pp-cli xmlrpc get-ticket`** - Calls execute_kw(db, uid, api_key, 'helpdesk.ticket', 'read', [[id]], {fields}).
- **`tag-helpdesk-pp-cli xmlrpc get-ticket-messages`** - Calls execute_kw on mail.message with domain [('res_model','=','helpdesk.ticket'),('res_id','=',id)].
Returns message log including internal notes and customer replies.
- **`tag-helpdesk-pp-cli xmlrpc list-categories`** - List ticket categories
- **`tag-helpdesk-pp-cli xmlrpc list-stages`** - List ticket stages
- **`tag-helpdesk-pp-cli xmlrpc list-tags`** - List ticket tags
- **`tag-helpdesk-pp-cli xmlrpc list-teams`** - List helpdesk teams
- **`tag-helpdesk-pp-cli xmlrpc list-tickets`** - Calls execute_kw(db, uid, api_key, 'helpdesk.ticket', 'search_read', [domain], {fields, limit, offset, order}).
Returns list of ticket records.
- **`tag-helpdesk-pp-cli xmlrpc post-note`** - Post an internal note on a ticket
- **`tag-helpdesk-pp-cli xmlrpc update-ticket`** - Update ticket fields


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tag-helpdesk-pp-cli xmlrpc authenticate

# JSON for scripting and agents
tag-helpdesk-pp-cli xmlrpc authenticate --json

# Filter to specific fields
tag-helpdesk-pp-cli xmlrpc authenticate --json --select id,name,status

# Dry run — show the request without sending
tag-helpdesk-pp-cli xmlrpc authenticate --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tag-helpdesk-pp-cli xmlrpc authenticate --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-tag-helpdesk -g
```

Then invoke `/pp-tag-helpdesk <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add tag-helpdesk tag-helpdesk-pp-mcp -e TAG_HELPDESK_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tag-helpdesk-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TAG_HELPDESK_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tag-helpdesk": {
      "command": "tag-helpdesk-pp-mcp",
      "env": {
        "TAG_HELPDESK_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
tag-helpdesk-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tag-helpdesk-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TAG_HELPDESK_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tag-helpdesk-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TAG_HELPDESK_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
