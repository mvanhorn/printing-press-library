# Pushover CLI

**Pushover from the terminal: send alerts, watch emergency receipts, inspect quota, and keep a redacted local notification ledger.**

pushover-pp-cli covers the full official API instead of stopping at send-message. It adds agent-safe env defaults, receipt lifecycle tools, quota preflight, and local history so alerts are auditable.

Printed by [@twidtwid](https://github.com/twidtwid) (Todd Dailey).

## Install

The recommended path installs both the `pushover-pp-cli` binary and the `pp-pushover` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pushover
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pushover --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pushover-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pushover --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pushover --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pushover skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pushover. The skill defines how its required CLI can be installed.
```

## Authentication

Use a Pushover application token and user key. Set PUSHOVER_APP_TOKEN and PUSHOVER_USER_KEY, or pass --app-token and --user-key explicitly. Legacy PUSHOVER_TOKEN/PUSHOVER_USER names are accepted for local compatibility.

## Quick Start

```bash
# Check send budget before test notifications
pushover-pp-cli quota --json


# Send a low-priority test notification
pushover-pp-cli notify "Printing Press test" --priority low --json


# Review sends recorded by the CLI
pushover-pp-cli history list --since 24h --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`notify`** — Send a notification from an argument or stdin with env/config defaults, named priorities, and emergency validation.

  _Lets agents send test or operational notifications without leaking token values or remembering the raw endpoint shape._

  ```bash
  pushover-pp-cli notify "Printing Press test" --priority low --json
  ```

### Receipt operations
- **`emergency watch`** — Send or monitor an emergency notification receipt, polling acknowledgement status and supporting cancellation.

  _Turns emergency notifications from fire-and-forget sends into an auditable on-call workflow._

  ```bash
  pushover-pp-cli emergency watch --receipt <receipt> --poll-interval 30s --cancel-on-timeout --json
  ```

### Operational safety
- **`quota`** — Show monthly send limit, remaining sends, and reset time from the app limits endpoint.

  _Agents can check send budget before fanout or repeated test sends._

  ```bash
  pushover-pp-cli quota --json
  ```

### Local state that compounds
- **`history`** — Search and export a local redacted ledger of notifications sent by this CLI and receipt outcomes.

  _Shows what this CLI sent, when, with which priority, without storing raw user keys or tokens._

  ```bash
  pushover-pp-cli history list --since 24h --json
  ```
- **`inbox sync`** — Download Open Client messages into local SQLite before optional server-side delete-through.

  _Makes Pushover usable as both send channel and retrievable inbox for agents that own an Open Client device._

  ```bash
  pushover-pp-cli inbox sync --device-id <device-id> --json
  ```

## Usage

Run `pushover-pp-cli --help` for the full command reference and flag list.

## Commands

### notify

Send an agent-safe notification from an argument, `--message`, or stdin. Uses `PUSHOVER_APP_TOKEN` and `PUSHOVER_USER_KEY` when flags are omitted, accepts named priorities, validates emergency retry fields, and records a redacted local history row.

- **`pushover-pp-cli notify`** - Send a Pushover notification with env defaults and local history

### emergency

Watch emergency-priority receipts until acknowledgement, expiry, callback, or timeout.

- **`pushover-pp-cli emergency watch`** - Poll an emergency receipt at a safe cadence

### quota

Check application send quota before test sends or batches.

- **`pushover-pp-cli quota`** - Show monthly limit, remaining sends, and reset time

### history

Inspect this CLI's local redacted notification ledger.

- **`pushover-pp-cli history`** - List locally recorded notification sends
- **`pushover-pp-cli history show`** - Show one local history row

### inbox

Download Open Client messages into local SQLite before optional delete-through.

- **`pushover-pp-cli inbox sync`** - Store downloaded Open Client messages locally
- **`pushover-pp-cli inbox list`** - List locally synced Open Client messages

### apps

Inspect application-level message quotas

- **`pushover-pp-cli apps limits`** - Check monthly message limit, remaining sends, and reset time

### devices

Manage Open Client devices

- **`pushover-pp-cli devices create`** - Register a new Open Client desktop device
- **`pushover-pp-cli devices delete-through`** - Delete downloaded Open Client messages up to a highest message id

### glances

Update Pushover Glances widgets

- **`pushover-pp-cli glances update`** - Update Glances widget fields

### groups

Create, inspect, and manage Pushover delivery groups

- **`pushover-pp-cli groups add-user`** - Add a user to a delivery group
- **`pushover-pp-cli groups create`** - Create a delivery group
- **`pushover-pp-cli groups disable-user`** - Temporarily disable a group user
- **`pushover-pp-cli groups enable-user`** - Re-enable a disabled group user
- **`pushover-pp-cli groups get`** - Get a delivery group's name and users
- **`pushover-pp-cli groups list`** - List delivery groups owned by the account
- **`pushover-pp-cli groups remove-user`** - Remove a user from a delivery group
- **`pushover-pp-cli groups rename`** - Rename a delivery group

### licenses

Assign and inspect Pushover license credits

- **`pushover-pp-cli licenses assign`** - Assign a pre-paid license credit to a user or email
- **`pushover-pp-cli licenses credits`** - Check remaining license credits

### messages

Send application notifications and download Open Client messages

- **`pushover-pp-cli messages download`** - Download pending messages for an Open Client device
- **`pushover-pp-cli messages send`** - Send a Pushover notification

### receipts

Inspect and cancel emergency-priority message receipts

- **`pushover-pp-cli receipts cancel`** - Cancel retries for one emergency-priority receipt
- **`pushover-pp-cli receipts cancel-by-tag`** - Cancel active emergency-priority retries matching a tag
- **`pushover-pp-cli receipts get`** - Get emergency-priority receipt status

### sounds

Discover notification sounds

- **`pushover-pp-cli sounds list`** - List built-in and account custom notification sounds

### subscriptions

Manage Pushover subscription migrations

- **`pushover-pp-cli subscriptions migrate`** - Migrate an existing user key into a subscription user key

### teams

Manage Pushover for Teams membership

- **`pushover-pp-cli teams add-user`** - Add a user to a team
- **`pushover-pp-cli teams get`** - Show team information and users
- **`pushover-pp-cli teams remove-user`** - Remove a user from a team

### users

Validate users and log in for Open Client sessions

- **`pushover-pp-cli users login`** - Log in a user for Open Client and return a user key plus session secret
- **`pushover-pp-cli users validate`** - Validate a user or group key and optional device


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pushover-pp-cli groups list --app-token your-token-here

# JSON for scripting and agents
pushover-pp-cli groups list --app-token your-token-here --json

# Filter to specific fields
pushover-pp-cli groups list --app-token your-token-here --json --select id,name,status

# Dry run — show the request without sending
pushover-pp-cli groups list --app-token your-token-here --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pushover-pp-cli groups list --app-token your-token-here --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pushover -g
```

Then invoke `/pp-pushover <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add pushover pushover-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pushover-current).
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
    "pushover": {
      "command": "pushover-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
pushover-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/pushover-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **notification send returns invalid user** — Run users validate with the same user key and optional device before saving the destination
- **priority 2 send fails** — Provide retry >= 30 and expire <= 10800; use emergency watch for a safer flow
- **429 quota response** — Run quota and wait until reset or reduce send volume

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**python-pushover**](https://pypi.org/pypi/python-pushover) — Python
- [**po-notify**](https://npm.io/package/po-notify) — JavaScript
- [**mcp-pushover**](https://mcpservers.org/servers/pyang2045/mcp-pushover) — TypeScript
- [**freeformz/pushover-mcp**](https://pkg.go.dev/github.com/freeformz/pushover-mcp) — Go
- [**lifecoach pushover scripts**](local:/Users/todd_1/repo/claude/lifecoach) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
