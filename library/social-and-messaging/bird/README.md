# Bird CLI

**A terminal-native CLI for Bird's Conversations and SMS APIs, with offline search, batch reconcile, and a local SQLite mirror nobody else ships.**

bird-pp-cli wraps every Conversations endpoint plus the SMS-relevant pieces of Bird's Channels API, then layers on what the SDKs miss: send/reconcile pairs with idempotency keys, FTS5 over message bodies, a delivery audit that exits non-zero on failure, and a tenant-readiness checklist that replaces the 12-curl onboarding flow.

Learn more at [Bird](https://bird.com).

Created by [@CleverAI-ZH](https://github.com/CleverAI-ZH) (Stephan Stoeber).

## Install

The recommended path installs both the `bird-pp-cli` binary and the `pp-bird` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install bird
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install bird --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install bird --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install bird --agent claude-code
npx -y @mvanhorn/printing-press-library install bird --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/cmd/bird-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bird-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install bird --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bird --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bird --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install bird --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bird-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BIRD_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/cmd/bird-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bird": {
      "command": "bird-pp-mcp",
      "env": {
        "BIRD_WORKSPACE_ID": "<workspace_id>",
        "BIRD_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication uses Bird's `Authorization: AccessKey <key>` header. Set BIRD_API_KEY plus BIRD_WORKSPACE_ID (every Bird endpoint is workspace-scoped) before calling any command. An optional BIRD_CHANNEL_ID provides a default for SMS commands so you don't have to pass --channel-id every time.

## Quick Start

```bash
# Check that BIRD_API_KEY and BIRD_WORKSPACE_ID resolve and the API is reachable.
bird-pp-cli doctor

# Verify your SMS tenant is ready: channels, anti-spam, compliance keywords, and messageability all probed in one shot.
bird-pp-cli tenant doctor --json

# Inspect the request without sending; drop --dry-run to actually fire.
bird-pp-cli sms send --to +31612345678 --body "Hello from bird-pp-cli" --dry-run

# Find the most recent OTP-shaped SMS sent to that recipient. Works offline once 'bird-pp-cli sync' has populated the local store.
bird-pp-cli sms search "otp" --to +31612345678 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`messages audit`** — Fold a message and its interactions into a chronological delivery timeline; exit non-zero on terminal failure.

  _Use this when an agent needs a one-shot answer to 'did this SMS land?' with a non-zero exit on failure for pipeline gating._

  ```bash
  bird-pp-cli messages audit msg_123 --json
  ```
- **`messages failures`** — Aggregate recent message-interaction failures grouped by reason code.

  _Reach for this before paging humans about an SMS outage — it tells you whether the failures are clustered (one bad number, one carrier) or spread._

  ```bash
  bird-pp-cli messages failures --since 24h --group-by reason --json
  ```
- **`sms search`** — Full-text search over message bodies, optionally filtered by sender or recipient phone number.

  _Use when an agent needs to find a specific outbound SMS without scrolling pages of console output._

  ```bash
  bird-pp-cli sms search "order confirmation" --to +31612345678 --json
  ```

### Send + reconcile
- **`sms send-batch`** — Send a batch of SMS messages from a CSV with per-row idempotency keys, persisting the batch in the local store for later reconcile.

  _Use this for any batch larger than a handful of recipients where you need to know which ones failed afterward._

  ```bash
  bird-pp-cli sms send-batch --csv recipients.csv --body-template "Hi {{name}}, your code is {{code}}" --dry-run
  ```
- **`sms reconcile`** — Re-fetch delivery interactions for every message in a batch, group failures by reason, and optionally retry.

  _Reach for this after every send-batch run — it answers 'how many landed?' and gives you a retry plan in one shot._

  ```bash
  bird-pp-cli sms reconcile batch_2026_05_10_a --retry-failed --json
  ```

### Triage and reach
- **`conversations timeline`** — Render a conversation's messages, participants, and delivery interactions in canonical chronological order.

  _Use when triaging a customer thread — one command shows the full back-and-forth plus per-outbound delivery state._

  ```bash
  bird-pp-cli conversations timeline conv_42 --json
  ```
- **`messages from`** — List every message exchanged with one phone number across all conversations.

  _Use when an agent needs the complete back-and-forth with one customer regardless of which conversation it lived in._

  ```bash
  bird-pp-cli messages from +31612345678 --json
  ```

### Compliance and onboarding
- **`tenant doctor`** — Run an SMS-tenant readiness checklist across channels, channel-config, anti-spam, compliance keywords, and messageability with a single exit code.

  _Use during onboarding for a new workspace or when an existing tenant's outbound SMS suddenly drops to zero._

  ```bash
  bird-pp-cli tenant doctor --test-contact contact_42 --json
  ```
- **`compliance auto-block`** — Scan local inbound messages for STOP-keyword fires within a time window; emit a CSV ready for bulk-add (or apply directly).

  _Use weekly to keep the workspace block list synchronized with customer opt-outs without writing a Python script._

  ```bash
  bird-pp-cli compliance auto-block --since 7d --json
  ```

## Usage

Run `bird-pp-cli --help` for the full command reference and flag list.

## Commands

### channel-config

Per-channel Conversations API configuration

- **`bird-pp-cli channel-config get`** - Get the Conversations configuration for a channel
- **`bird-pp-cli channel-config update`** - Update Conversations configuration for a channel

### channel-media

Channel-specific pre-signed media uploads

- **`bird-pp-cli channel-media presigned-upload`** - Create a channel-scoped pre-signed media upload URL

### channels

SMS channels available in the workspace

- **`bird-pp-cli channels get`** - Get one channel by ID
- **`bird-pp-cli channels list`** - List channels in the workspace (filter --kind sms)
- **`bird-pp-cli channels messageability`** - Check whether a channel can message a given contact (the customer-service window probe)

### compliance

Channel compliance keyword routing (HELP/STOP/START)

### conversations

Manage Bird conversation threads (cross-channel customer interactions)

- **`bird-pp-cli conversations create`** - Start a new conversation
- **`bird-pp-cli conversations delete`** - Delete a conversation
- **`bird-pp-cli conversations get`** - Get one conversation by ID
- **`bird-pp-cli conversations list`** - List conversations across the workspace
- **`bird-pp-cli conversations update`** - Update a conversation (status, name, etc.)

### media

Pre-signed media uploads for messages with attachments

- **`bird-pp-cli media presigned-upload`** - Create a workspace-wide pre-signed media upload URL

### messages

Channel-level messages (the SMS send/receive layer)

- **`bird-pp-cli messages get`** - Get one message by ID
- **`bird-pp-cli messages interactions`** - List delivery-event interactions for a message (sent, delivered, read, failed)
- **`bird-pp-cli messages list`** - List messages on a channel (chronological)
- **`bird-pp-cli messages list-all`** - List messages across the workspace

### participants

Workspace-wide participant lookup

- **`bird-pp-cli participants conversations`** - List conversations a participant belongs to (by participant ID)
- **`bird-pp-cli participants conversations-by-identifier`** - List conversations a participant belongs to (by identifier key and value)

### sms

Programmable SMS send (the headline command)

- **`bird-pp-cli sms send`** - Send an SMS message

### workspace

Workspace-level configuration: anti-spam and allow/block rules

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bird-pp-cli channel-config get mock-value

# JSON for scripting and agents
bird-pp-cli channel-config get mock-value --json

# Filter to specific fields
bird-pp-cli channel-config get mock-value --json --select id,name,status

# Dry run — show the request without sending
bird-pp-cli channel-config get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bird-pp-cli channel-config get mock-value --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `BIRD_WORKSPACE_ID` resolves `{workspace_id}`

Base URL: `https://api.bird.com/workspaces/{workspace_id}`

## Health Check

```bash
bird-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bird-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BIRD_WORKSPACE_ID` | endpoint | Yes |  |
| `BIRD_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bird-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BIRD_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Confirm BIRD_API_KEY is set and the access key is enabled in the Bird dashboard at app.bird.com/settings/security/access-keys.
- **404 'workspace not found'** — Set BIRD_WORKSPACE_ID — every Bird path is workspace-scoped. Find it in the Bird dashboard under Settings → Workspaces.
- **429 Too Many Requests** — Bird allows 50 retrieval calls/sec burst, 2000/min steady. The CLI's adaptive limiter respects 429s; retry after the suggested backoff or wait one minute.
- **sms search returns nothing** — Run bird-pp-cli sync first — search reads from the local store, which is empty until the first sync.
- **messages audit reports 'no interactions yet'** — Bird lags interactions a few seconds behind the message create response; retry after 10 seconds or set --wait.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**messagebird/go-rest-api**](https://github.com/messagebird/go-rest-api) — Go
- [**messagebird/python-rest-api**](https://github.com/messagebird/python-rest-api) — Python
- [**messagebird/messagebird-nodejs**](https://github.com/messagebird/messagebird-nodejs) — JavaScript
- [**messagebird/php-rest-api**](https://github.com/messagebird/php-rest-api) — PHP
- [**messagebird/ruby-rest-api**](https://github.com/messagebird/ruby-rest-api) — Ruby
- [**messagebird/openapi-specs**](https://github.com/messagebird/openapi-specs) — YAML

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
