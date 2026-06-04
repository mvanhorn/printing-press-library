# Tidycal CLI

# Introduction
TidyCal's REST API provides a handful of endpoints which can be used to get information about your account and bookings.  It uses conventional OAuth 2.0 protocol for authentication.

# Authentication
### Personal Access Token
Create a personal access token at https://tidycal.com/integrations/oauth.  Once created, it can be used to authenticate requests by passing it in the `Authorization` header.
```
Authorization: Bearer {TOKEN}
```

### OAuth 2.0 Client
If you're building a custom integration to TidyCal which requires users to authenticate in order to get access tokens to make API requests on their behalf, you'll need to create an OAuth 2.0 client. This is easy to do from the \"OAuth Apps\" settings page found here https://tidycal.com/integrations/oauth

Using the `authorization_code` grant type to authenticate users using OAuth 2.0 to retrieve an access token is fairly conventional, more information on that process can be found here: https://www.oauth.com/oauth2-servers/server-side-apps/authorization-code/

* Authorization URL: https://tidycal.com/oauth/authorize
* Access Token URL: https://tidycal.com/oauth/token

Learn more at [Tidycal](https://tidycal.com/developer/docs/).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `tidycal-pp-cli` binary and the `pp-tidycal` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tidycal
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tidycal --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tidycal --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tidycal --agent claude-code
npx -y @mvanhorn/printing-press-library install tidycal --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tidycal/cmd/tidycal-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tidycal-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tidycal --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-tidycal --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install tidycal --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tidycal-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TIDYCAL_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tidycal/cmd/tidycal-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tidycal": {
      "command": "tidycal-pp-mcp",
      "env": {
        "TIDYCAL_API_TOKEN": "<your-key>"
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
tidycal-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export TIDYCAL_API_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
tidycal-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
tidycal-pp-cli booking-types list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agentic scheduling
- **`brief`** — Produce a contact-aware schedule brief for a day or date range.

  _Use this before drafting prep notes or deciding whether a booking needs attention._

  ```bash
  tidycal-pp-cli brief --date today --format json
  ```
- **`triage`** — Identify booking problems such as missing meeting URLs, duplicate contacts, off-hours bookings, cancelled bookings, and incomplete contact data.

  _Use this to decide which bookings require operator review._

  ```bash
  tidycal-pp-cli triage --from today --to +7d --format json
  ```
- **`propose-times`** — Fetch available TidyCal timeslots for a booking type and rank a paste-ready shortlist by date window, timezone, preference, and weekend policy.

  _Use this when proposing meeting options in chat or email._

  ```bash
  tidycal-pp-cli propose-times 123 --from today --to +14d --count 3 --format json
  ```
- **`followups`** — Create an AI-ready follow-up queue from recent bookings without sending messages or notifications.

  _Use this to prepare follow-up tasks after recent meetings._

  ```bash
  tidycal-pp-cli followups --from -7d --to today --format json
  ```
- **`assisted-book`** — Book on behalf of a contact through an inspectable dry-run and explicit confirmation gate.

  _Use this only when the operator has approved the booking details._

  ```bash
  tidycal-pp-cli assisted-book 123 --name Ada --email ada@example.com --slot 2026-06-02T15:00:00Z --dry-run --format json
  ```

## Usage

Run `tidycal-pp-cli --help` for the full command reference and flag list.

## Commands

### booking-types

Manage booking types

- **`tidycal-pp-cli booking-types create`** - Create a new booking type.
- **`tidycal-pp-cli booking-types list`** - Get a list of booking types.

### bookings

Manage bookings

- **`tidycal-pp-cli bookings get`** - Get a booking by ID.
- **`tidycal-pp-cli bookings list`** - Get a list of bookings.

### contacts

Manage contacts

- **`tidycal-pp-cli contacts create`** - Create a new contact.
- **`tidycal-pp-cli contacts list`** - Get a list of contacts.

### me

Manage me

- **`tidycal-pp-cli me`** - Get account details.

### teams

Manage teams

- **`tidycal-pp-cli teams get`** - Get details of a specific team.
- **`tidycal-pp-cli teams list`** - Get a list of teams the authenticated user has access to.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tidycal-pp-cli booking-types list

# JSON for scripting and agents
tidycal-pp-cli booking-types list --json

# Filter to specific fields
tidycal-pp-cli booking-types list --json --select id,name,status

# Dry run — show the request without sending
tidycal-pp-cli booking-types list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tidycal-pp-cli booking-types list --agent
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
tidycal-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tidycal-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TIDYCAL_API_TOKEN` | per_call | No | Set to your API credential. |
| `TIDYCAL_BEARER_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tidycal-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tidycal-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TIDYCAL_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
