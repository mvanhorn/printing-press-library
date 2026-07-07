# Breezedoc CLI

# Introduction
BreezeDoc's REST API provides a handful of endpoints which can be used to get information about your account and bookings.  It uses conventional OAuth 2.0 protocol for authentication.

# Authentication
### Personal Access Token
Create a personal access token at https://breezedoc.com/integrations/api.  Once created, it can be used to authenticate requests by passing it in the `Authorization` header.
```
Authorization: Bearer {TOKEN}
```

### OAuth 2.0 Client
If you're building a custom integration to BreezeDoc which requires users to authenticate in order to get access tokens to make API requests on their behalf, you'll need to create an OAuth 2.0 client. This is easy to do from the \"OAuth Apps\" settings page found here https://breezedoc.com/integrations/api

Using the `authorization_code` grant type to authenticate users using OAuth 2.0 to retrieve an access token is fairly conventional, more information on that process can be found here: https://www.oauth.com/oauth2-servers/server-side-apps/authorization-code/

* Authorization URL: https://breezedoc.com/oauth/authorize
* Access Token URL: https://breezedoc.com/oauth/token

# Rate Limiting
The API currently has a rate limit of 60 requests per minute. If you exceed this limit, you will receive a 429 error.

Learn more at [Breezedoc](https://breezedoc.com/developer/docs/).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `breezedoc-pp-cli` binary and the `pp-breezedoc` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install breezedoc
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install breezedoc --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install breezedoc --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install breezedoc --agent claude-code
npx -y @mvanhorn/printing-press-library install breezedoc --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/cmd/breezedoc-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/breezedoc-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install breezedoc --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-breezedoc --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-breezedoc --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install breezedoc --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/breezedoc-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BREEZEDOC_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/breezedoc/cmd/breezedoc-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "breezedoc": {
      "command": "breezedoc-pp-mcp",
      "env": {
        "BREEZEDOC_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
breezedoc-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export BREEZEDOC_API_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
breezedoc-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
breezedoc-pp-cli documents list
```

## Usage

Run `breezedoc-pp-cli --help` for the full command reference and flag list.

## Commands

### documents

Manage documents

- **`breezedoc-pp-cli documents get`** - Get a specific document
- **`breezedoc-pp-cli documents list`** - Get list of documents
- **`breezedoc-pp-cli documents store`** - Create a new document

### invoices

Manage invoices

- **`breezedoc-pp-cli invoices create`** - Creates a new invoice with line items. Optionally sends the invoice immediately by setting `send: true`.
- **`breezedoc-pp-cli invoices delete`** - Deletes a draft invoice. Cannot delete invoices that have already been sent.
- **`breezedoc-pp-cli invoices get`** - Get a specific invoice
- **`breezedoc-pp-cli invoices list`** - Retrieves a paginated list of invoices for the authenticated user.
- **`breezedoc-pp-cli invoices patch`** - Same as PUT - updates an existing invoice. Cannot update invoices with status paid, uncollectible, or void.
- **`breezedoc-pp-cli invoices update`** - Updates an existing invoice. Cannot update invoices with status: paid, uncollectible, or void.
When updating items, the entire items array is replaced.

### me

Manage me

- **`breezedoc-pp-cli me`** - Get current user information

### recipients

Manage recipients

- **`breezedoc-pp-cli recipients`** - Get list of recipients

### teams

Manage teams

### templates

Manage templates

- **`breezedoc-pp-cli templates get`** - Get a specific template
- **`breezedoc-pp-cli templates list`** - Get list of templates

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
breezedoc-pp-cli documents list

# JSON for scripting and agents
breezedoc-pp-cli documents list --json

# Filter to specific fields
breezedoc-pp-cli documents list --json --select id,name,status

# Dry run — show the request without sending
breezedoc-pp-cli documents list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
breezedoc-pp-cli documents list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
breezedoc-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/breezedoc-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BREEZEDOC_API_TOKEN` | per_call | No | Set to your API credential. |
| `BREEZEDOC_BEARER_TOKEN` | per_call | No | Set to your API credential. |
| `BREEZEDOC_BEARER_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `breezedoc-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `breezedoc-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BREEZEDOC_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
