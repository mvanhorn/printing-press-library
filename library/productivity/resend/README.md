# Resend CLI

**The full Resend API (100 endpoints across emails, broadcasts, audiences, contacts, domains, templates, webhooks) plus a local SQLite cache that powers cross-resource queries the dashboard and SDK cannot answer.**

The official Resend CLI is fast and one-shot per command; this CLI is the agent-native companion with --json --select --dry-run consistency, FTS5 search over sent emails, and rollups (audiences inventory, broadcasts performance, deliverability summary, contacts where, emails to <recipient>) that exist only because every Resend resource is mirrored locally.

Printed by [@giacaglia](https://github.com/giacaglia) (Giuliano Giacaglia).

## Install

The recommended path installs both the `resend-pp-cli` binary and the `pp-resend` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install resend
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install resend --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install resend --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install resend --agent claude-code
npx -y @mvanhorn/printing-press-library install resend --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/resend/cmd/resend-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/resend-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-resend --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-resend --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-resend skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-resend. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/resend-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RESEND_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "resend": {
      "command": "resend-pp-mcp",
      "env": {
        "RESEND_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify RESEND_API_KEY and reach api.resend.com.
resend-pp-cli doctor

# Populate the local SQLite store with audiences, contacts, domains, broadcasts, and templates.
resend-pp-cli sync --full

# First read — confirm sync landed.
resend-pp-cli domains list --json --select id,name,status

# Cross-audience rollup the official CLI cannot produce.
resend-pp-cli audiences inventory --json

# FTS across emails, contacts, broadcasts, and templates.
resend-pp-cli search invoice --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-resource queries over local state
- **`emails to`** — Every email sent to a single recipient address, with delivery status and timestamps, ordered newest-first.

  _Reach for this when an agent needs to reconstruct what a single user has received — support tickets, GDPR exports, dispute investigations._

  ```bash
  resend-pp-cli emails to <your-email> --json --select id,subject,status,sent_at
  ```
- **`emails timeline`** — Collapsed event chain for a single email: sent → delivered → opened → clicked → bounced, in one ordered table.

  _Use when debugging 'why didn't this arrive?' — one command shows whether it was sent, delivered, opened, or bounced and when._

  ```bash
  resend-pp-cli emails timeline 4ef9a417-d4ff-4ec5-9af2-c80a4d5d2c1f --json
  ```
- **`audiences inventory`** — Per-audience rollup: contact count, unsubscribed count, last-broadcast timestamp, recent open-rate.

  _Use before planning a campaign — quickly see which audiences are healthy and recently engaged._

  ```bash
  resend-pp-cli audiences inventory --json
  ```
- **`contacts where`** — Find every audience, segment, and topic a contact (by email or name) belongs to in one query.

  _Use when triaging unsubscribe requests, GDPR deletions, or 'why is bob getting this email?'._

  ```bash
  resend-pp-cli contacts where <contact-email> --json --select audience_name,subscribed
  ```

### Aggregate analytics from local store
- **`broadcasts performance`** — Open / click / bounce rate across all broadcasts, sortable; not limited to a single broadcast or 30-day window.

  _Use to compare campaign performance across the lifetime of the account._

  ```bash
  resend-pp-cli broadcasts performance --json --select broadcasts,count,status
  ```
- **`domains health`** — Verification + DKIM/SPF/DMARC status across every domain in one table, flags missing or unverified records.

  _Use on call-out / deliverability incidents to confirm all sending domains are fully verified._

  ```bash
  resend-pp-cli domains health --json
  ```
- **`deliverability summary`** — Bounce rate, complaint rate, and suppression count over a rolling window (default 7d) computed from local event data.

  _Use weekly to monitor IP/domain reputation trends before they cause deliverability incidents._

  ```bash
  resend-pp-cli deliverability summary --window 7d --json
  ```

### Operational hygiene
- **`api-keys rotation`** — API keys sorted by age + last-used timestamp (joined from logs); flags stale keys older than N days.

  _Use during quarterly security reviews — find unused keys that should be rotated or revoked._

  ```bash
  resend-pp-cli api-keys rotation --older-than 90d --json
  ```

## Usage

Run `resend-pp-cli --help` for the full command reference and flag list.

## Commands

### api-keys

Create and manage API Keys through the Resend API.

- **`resend-pp-cli api-keys create`** - Create a new API key
- **`resend-pp-cli api-keys delete`** - Remove an existing API key
- **`resend-pp-cli api-keys list`** - Retrieve a list of API keys

### audiences

Deprecated: Use Segments instead. Create and manage Audiences through the Resend API.

- **`resend-pp-cli audiences create`** - Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- **`resend-pp-cli audiences delete`** - Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- **`resend-pp-cli audiences get`** - Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- **`resend-pp-cli audiences list`** - Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.

### automations

Create and manage Automations through the Resend API.

- **`resend-pp-cli automations create`** - Create an automation
- **`resend-pp-cli automations delete`** - Delete an automation
- **`resend-pp-cli automations get`** - Retrieve a single automation
- **`resend-pp-cli automations list`** - Retrieve a list of automations
- **`resend-pp-cli automations update`** - Update an automation

### broadcasts

Create and manage Broadcasts through the Resend API.

- **`resend-pp-cli broadcasts create`** - Create a broadcast
- **`resend-pp-cli broadcasts delete`** - Remove an existing broadcast that is in the draft status
- **`resend-pp-cli broadcasts get`** - Retrieve a single broadcast
- **`resend-pp-cli broadcasts list`** - Retrieve a list of broadcasts
- **`resend-pp-cli broadcasts update`** - Update an existing broadcast

### contact-properties

Create and manage Contact Properties through the Resend API.

- **`resend-pp-cli contact-properties create`** - Create a new contact property
- **`resend-pp-cli contact-properties delete`** - Remove an existing contact property
- **`resend-pp-cli contact-properties get`** - Retrieve a single contact property
- **`resend-pp-cli contact-properties list`** - Retrieve a list of contact properties
- **`resend-pp-cli contact-properties update`** - Update an existing contact property

### contacts

Create and manage Contacts through the Resend API.

- **`resend-pp-cli contacts create`** - Create a new contact
- **`resend-pp-cli contacts delete`** - Remove an existing contact by ID or email
- **`resend-pp-cli contacts get`** - Retrieve a single contact by ID or email
- **`resend-pp-cli contacts list`** - Retrieve a list of contacts
- **`resend-pp-cli contacts update`** - Update a single contact by ID or email

### domains

Create and manage domains through the Resend API.

- **`resend-pp-cli domains create`** - Create a new domain
- **`resend-pp-cli domains delete`** - Remove an existing domain
- **`resend-pp-cli domains get`** - Retrieve a single domain
- **`resend-pp-cli domains list`** - Retrieve a list of domains
- **`resend-pp-cli domains update`** - Update an existing domain

### emails

Start sending emails through the Resend API.

- **`resend-pp-cli emails create`** - Send an email
- **`resend-pp-cli emails create-batch`** - Trigger up to 100 batch emails at once.
- **`resend-pp-cli emails get`** - Retrieve a single email
- **`resend-pp-cli emails get-receiving`** - Retrieve a single received email
- **`resend-pp-cli emails get-receiving-2`** - Retrieve a list of attachments for a received email
- **`resend-pp-cli emails get-receiving-3`** - Retrieve a single attachment for a received email
- **`resend-pp-cli emails list`** - Retrieve a list of emails
- **`resend-pp-cli emails list-receiving`** - Retrieve a list of received emails
- **`resend-pp-cli emails update`** - Update a single email

### events

Create and manage Events through the Resend API.

- **`resend-pp-cli events create`** - Create an event
- **`resend-pp-cli events create-send`** - Send an event
- **`resend-pp-cli events delete`** - Delete an event
- **`resend-pp-cli events get`** - Retrieve a single event
- **`resend-pp-cli events list`** - Retrieve a list of events
- **`resend-pp-cli events update`** - Update an event

### logs

Retrieve API request logs through the Resend API.

- **`resend-pp-cli logs get`** - Retrieve a single log
- **`resend-pp-cli logs list`** - Retrieve a list of logs

### segments

Create and manage Segments through the Resend API.

- **`resend-pp-cli segments create`** - Create a new segment
- **`resend-pp-cli segments delete`** - Remove an existing segment
- **`resend-pp-cli segments get`** - Retrieve a single segment
- **`resend-pp-cli segments list`** - Retrieve a list of segments

### templates

Create and manage Templates through the Resend API.

- **`resend-pp-cli templates create`** - Create a template
- **`resend-pp-cli templates delete`** - Remove an existing template
- **`resend-pp-cli templates get`** - Retrieve a single template
- **`resend-pp-cli templates list`** - Retrieve a list of templates
- **`resend-pp-cli templates update`** - Update an existing template

### topics

Create and manage Topics through the Resend API.

- **`resend-pp-cli topics create`** - Create a new topic
- **`resend-pp-cli topics delete`** - Remove an existing topic
- **`resend-pp-cli topics get`** - Retrieve a single topic
- **`resend-pp-cli topics list`** - Retrieve a list of topics
- **`resend-pp-cli topics update`** - Update an existing topic

### webhooks

Create and manage Webhooks through the Resend API.

- **`resend-pp-cli webhooks create`** - Create a new webhook
- **`resend-pp-cli webhooks delete`** - Remove an existing webhook
- **`resend-pp-cli webhooks get`** - Retrieve a single webhook
- **`resend-pp-cli webhooks list`** - Retrieve a list of webhooks
- **`resend-pp-cli webhooks update`** - Update an existing webhook

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
resend-pp-cli api-keys list

# JSON for scripting and agents
resend-pp-cli api-keys list --json

# Filter to specific fields
resend-pp-cli api-keys list --json --select id,name,status

# Dry run — show the request without sending
resend-pp-cli api-keys list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
resend-pp-cli api-keys list --agent
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
resend-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/resend-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RESEND_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `resend-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RESEND_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 unauthorized on every command** — Confirm RESEND_API_KEY is exported. Keys start with re_… and are visible at resend.com/api-keys.
- **Local commands return zero rows** — Run `resend-pp-cli sync --full` — local-only commands read from the SQLite cache.
- **429 rate limit** — Resend documents Retry-After. Lower --rate-limit (default 0 = unbounded) or run sync in off-hours.
- **Domain reports unverified after creation** — Resend caches DNS for ~24h; `domains verify <id>` triggers re-check, and `domains get <id>` shows current DKIM/SPF/DMARC state.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**resend/resend-node**](https://github.com/resend/resend-node) — TypeScript (907 stars)
- [**resend/mcp-send-email**](https://github.com/resend/mcp-send-email) — TypeScript (510 stars)
- [**resend/resend-cli**](https://github.com/resend/resend-cli) — TypeScript (368 stars)
- [**resend/resend-go**](https://github.com/resend/resend-go) — Go
- [**resend/resend-python**](https://github.com/resend/resend-python) — Python
- [**resend/resend-openapi**](https://github.com/resend/resend-openapi) — YAML
- [**resend/n8n-nodes-resend**](https://github.com/resend/n8n-nodes-resend) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
