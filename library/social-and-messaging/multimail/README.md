# MultiMail CLI

**Every MultiMail feature from the terminal, plus inbox health, stale-thread detection, and offline search no other MultiMail tool has.**

A Go CLI that matches all 47 MCP server tools as shell commands, adds a local SQLite cache with FTS5 search, and introduces compound commands that surface compliance insights the API alone cannot answer. Agent-native by default: auto-JSON when piped, --compact for token-efficient output, typed exit codes for scripting.

## Install

The recommended path installs both the `multimail-pp-cli` binary and the `pp-multimail` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install multimail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install multimail --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/multimail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-multimail skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-multimail. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# verify API key and connectivity
multimail-pp-cli doctor


# see available mailboxes
multimail-pp-cli mailbox list


# check recent emails with minimal tokens
multimail-pp-cli inbox --limit 10 --compact


# populate local cache for offline queries
multimail-pp-cli sync --full


# composite inbox health score from cached data
multimail-pp-cli health


# offline full-text search
multimail-pp-cli search 'meeting agenda' --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compliance observatory

- **`health`** — Single-number composite score combining unread ratio, response time, bounce rate, and quota headroom — instantly tells you if an inbox needs attention.

  _When managing multiple agent mailboxes, reach for this to triage which inbox needs attention first without checking each one individually._

  ```bash
  multimail-pp-cli health --mailbox primary --json
  ```
- **`stale`** — Find conversation threads that have gone unanswered past a configurable threshold — surface the emails you're dropping the ball on.

  _Before composing new emails, check stale threads first — replying to an existing conversation is almost always higher-value than starting a new one._

  ```bash
  multimail-pp-cli stale --days 3 --json
  ```
- **`oversight summary`** — See pending approval count, average decision time, approval rate, and most-gated senders across all mailboxes — the operator's command center.

  _When the operator hasn't checked in, run this to know if the oversight queue is backing up and which senders are triggering the most gates._

  ```bash
  multimail-pp-cli oversight summary --json
  ```
- **`trust status`** — Current trust ladder position per mailbox, what's needed for the next level, and progression history — the agent's autonomy roadmap.

  _Before requesting an oversight upgrade, check your current position and what the operator needs to see before granting more autonomy._

  ```bash
  multimail-pp-cli trust status --json
  ```

### Local state that compounds

- **`quota forecast`** — Predict when your email quota will be exhausted based on rolling send rate — days remaining with confidence interval.

  _Before scheduling a large email batch, check quota forecast to know if you'll hit limits and need to suggest a plan upgrade._

  ```bash
  multimail-pp-cli quota forecast --json
  ```
- **`stats`** — Send/receive volume, top correspondents, peak hours, and delivery rate over a configurable period — understand email patterns at a glance.

  _When planning outreach campaigns or reviewing agent communication patterns, stats gives you the baseline numbers to work from._

  ```bash
  multimail-pp-cli stats --period 30d --json
  ```

### Agent-native plumbing

- **`search`** — Full-text search across cached emails — works without network after sync, searches subjects, bodies, senders, and recipients.

  _When searching for a specific email or topic, use search instead of paginating through inbox results — especially useful in CI/CD where network calls add latency._

  ```bash
  multimail-pp-cli search 'invoice overdue' --json --compact
  ```
- **`sync`** — Cursor-tracked incremental sync of all entities to local SQLite — enables every compound command and offline access.

  _Run sync before any local query to ensure fresh data. After the first full sync, incrementals are fast and cheap._

  ```bash
  multimail-pp-cli sync --full
  ```

## Usage

Run `multimail-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`multimail-pp-cli account create`** - Create a new account
- **`multimail-pp-cli account create-challenge`** - Request a verification challenge for account creation
- **`multimail-pp-cli account create-resendconfirmation`** - Resend the account activation email
- **`multimail-pp-cli account delete`** - Permanently delete account and all associated data
- **`multimail-pp-cli account list`** - Get current account info and usage
- **`multimail-pp-cli account update`** - Update account settings

### admin

Create and email a new API key to the account owner

- **`multimail-pp-cli admin create`** - Admin-only. Creates a new API key. Required: reason, tenant_id.

### api-keys

Manage api keys

- **`multimail-pp-cli api-keys create`** - Create a new API key. Requires admin scope.
- **`multimail-pp-cli api-keys delete`** - Delete an API key (requires admin scope, two-step approval)
- **`multimail-pp-cli api-keys list`** - Requires admin scope. Returns key prefix, scopes, and metadata.
- **`multimail-pp-cli api-keys update`** - Update API key name or scopes

### audit-log

Manage audit log

- **`multimail-pp-cli audit-log list`** - Returns audit log entries with cursor pagination. Requires admin scope.

### billing

Manage billing

- **`multimail-pp-cli billing create`** - Cancel subscription (retains access until end of billing period)
- **`multimail-pp-cli billing create-checkout`** - Create a checkout session for plan upgrade
- **`multimail-pp-cli billing create-cryptocheckout`** - Create a crypto payment checkout
- **`multimail-pp-cli billing create-portal`** - Open the billing management portal
- **`multimail-pp-cli billing create-pricingcheckout`** - Start the signup checkout flow
- **`multimail-pp-cli billing list`** - Retrieve your API key after checkout

### confirm

Manage confirm

- **`multimail-pp-cli confirm create`** - Activate account with confirmation code

### contacts

Manage contacts

- **`multimail-pp-cli contacts create`** - Add a contact to the address book. Requires send scope.
- **`multimail-pp-cli contacts delete`** - Requires admin scope.
- **`multimail-pp-cli contacts list`** - Search address book by name or email. Omit query to list all. Requires read scope.

### domains

Manage domains

- **`multimail-pp-cli domains create`** - Add a custom domain (Pro/Scale only)
- **`multimail-pp-cli domains delete`** - Delete a custom domain
- **`multimail-pp-cli domains get`** - Get custom domain detail
- **`multimail-pp-cli domains list`** - Requires admin scope.

### emails

List spam and quarantined emails across all mailboxes

- **`multimail-pp-cli emails list`** - Requires read scope. List emails with optional status filter.

### funnel

Track funnel analytics events

- **`multimail-pp-cli funnel create`** - Record a funnel analytics event.

### mailboxes

Manage mailboxes

- **`multimail-pp-cli mailboxes create`** - Create a new mailbox. Requires admin scope.
- **`multimail-pp-cli mailboxes delete`** - Requires admin scope.
- **`multimail-pp-cli mailboxes list`** - Requires read scope.
- **`multimail-pp-cli mailboxes update`** - Update mailbox settings. Requires admin scope.

### multimail-export

Manage multimail export

- **`multimail-pp-cli multimail-export list`** - Requires admin scope. Rate limited to 1 request per hour.

### multimail-health

Check API health status

- **`multimail-pp-cli multimail-health list`** - Health check. No auth required.

### operator

Manage operator

- **`multimail-pp-cli operator create`** - End operator session. Requires admin scope.
- **`multimail-pp-cli operator create-startsession`** - Start operator session. Sends a verification code. Requires admin scope.
- **`multimail-pp-cli operator create-verifysession`** - Verify operator session with one-time code. Requires admin scope.
- **`multimail-pp-cli operator list`** - Check operator session status. Requires admin scope.

### oversight

Manage oversight

- **`multimail-pp-cli oversight create`** - Requires oversight scope. Approved outbound emails are sent immediately.
- **`multimail-pp-cli oversight list`** - List emails pending oversight approval

### slug-check

Check if an account name is available

- **`multimail-pp-cli slug-check get`** - Check if an account name is available. Returns suggestions if taken or reserved. No auth required.

### support

Submit a support request

- **`multimail-pp-cli support create`** - Send a support message. Requires a verification challenge.

### suppression

Manage suppression

- **`multimail-pp-cli suppression delete`** - Allows future emails to be sent to this address again. Requires admin scope.
- **`multimail-pp-cli suppression list`** - Returns addresses suppressed due to bounces, spam complaints, or manual unsubscribes. Requires admin scope.

### unsubscribe

Manage unsubscribe

- **`multimail-pp-cli unsubscribe create`** - Process unsubscribe request
- **`multimail-pp-cli unsubscribe get`** - Process unsubscribe (CAN-SPAM)

### usage

Manage usage

- **`multimail-pp-cli usage list`** - Requires read scope. Returns usage counts for the current billing period.

### webhook-deliveries

Manage webhook deliveries

- **`multimail-pp-cli webhook-deliveries list`** - Returns recent webhook delivery attempts. Requires admin scope.

### webhooks

Manage webhooks

- **`multimail-pp-cli webhooks create`** - Subscribe to email events. Requires admin scope.
- **`multimail-pp-cli webhooks delete`** - Delete a webhook subscription
- **`multimail-pp-cli webhooks get`** - Get webhook details. Requires admin scope.
- **`multimail-pp-cli webhooks list`** - List webhook subscriptions. Requires admin scope.

### well-known

Manage well known

- **`multimail-pp-cli well-known get`** - Look up sender identity by hash
- **`multimail-pp-cli well-known list`** - Get the public signing key


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
multimail-pp-cli account list

# JSON for scripting and agents
multimail-pp-cli account list --json

# Filter to specific fields
multimail-pp-cli account list --json --select id,name,status

# Dry run — show the request without sending
multimail-pp-cli account list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
multimail-pp-cli account list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-multimail -g
```

Then invoke `/pp-multimail <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-mcp@latest
```

Then register it:

```bash
claude mcp add multimail multimail-pp-mcp -e MULTIMAIL_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/multimail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MULTIMAIL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/multimail/cmd/multimail-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "multimail": {
      "command": "multimail-pp-mcp",
      "env": {
        "MULTIMAIL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
multimail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/multimail-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MULTIMAIL_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `multimail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MULTIMAIL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every command** — Set MULTIMAIL_API_KEY=mm_live_... (get from MultiMail dashboard or mm auth setup)
- **403 with 'error code: 1020'** — The API gateway is blocking the request. Contact support if this persists.
- **Empty inbox results after sync** — Verify mailbox_id with 'mm mailbox list'. If using multiple mailboxes, specify --mailbox <id>.
- **Oversight decide returns 403** — The API key needs 'oversight' scope. Create a new key with 'mm key create --scopes oversight'.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
