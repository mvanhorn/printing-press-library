# Linq CLI

Community-curated OpenAPI 3.1 blueprint for the Linq Partner API, derived from https://docs.linqapp.com/api and cross-checked against github.com/linq-team/linq-go/api.md. The blueprint is generic and contains no private deployment-specific behavior or examples.

Learn more at [Linq](https://docs.linqapp.com/api/).

## Generic Linq messaging experience layer

Use these commands for normal Linq messaging workflows.

- `compose preview` builds and validates the nested `{"message": ...}` body without sending.
- `compose send` sends generic rich messages with text, media, link previews, replies, preferred protocol, effects, decorations, and idempotency.
- `typing start|stop|pulse` wraps bounded iMessage typing indicators.
- `typing watch` reads a captured webhook/debug stream from stdin or `--file` and emits only inbound typing start/stop events. It does not receive webhooks from Linq.
- `webhooks list|show|add-event|remove-event|set-events|doctor` wraps webhook subscription management with catalog validation and safe event-set updates.
- `capability check` checks iMessage first, then RCS, and falls back to SMS for channel-aware feature planning.
- `effects list|preview` exposes screen effects, bubble effects, text styles, and text animations for generic messages.
- `attachments plan|upload|send-url|send-id|audit-url|cleanup` covers direct URL versus pre-upload media workflows.
- `link-preview audit|send|metadata` enforces link-only rich preview rules and checks preview metadata.
- `react add|remove|custom` validates built-in/custom reactions and blocks outbound stickers.
- `contact-share preflight|send` checks native iMessage Name and Photo Sharing readiness before calling the share endpoint.

Examples:

```bash
linq-pp-cli compose preview --text "Congrats!" --effect screen:confetti --decorate 0:8:bold --preferred-service iMessage --agent
linq-pp-cli compose send --chat-id ch_123 --text "Congrats!" --effect screen:confetti --idempotency-key req_123 --agent --dry-run
linq-pp-cli typing start --chat-id ch_123 --agent --dry-run
linq-pp-cli typing stop --chat-id ch_123 --agent --dry-run
linq-pp-cli typing pulse --chat-id ch_123 --dwell 800ms --agent --dry-run
tail -f ./debug/linq-webhooks.ndjson | linq-pp-cli typing watch --agent
linq-pp-cli webhooks add-event sub_123 chat.typing_indicator.started chat.typing_indicator.stopped --agent --dry-run
linq-pp-cli webhooks doctor --subscription-id sub_123 --agent
linq-pp-cli capability check +15551234567 --agent
linq-pp-cli effects list --agent
linq-pp-cli effects preview --text "Hello world" --decorate 0:5:bold --decorate 6:11:shake --agent
linq-pp-cli attachments plan --file ./photo.jpg --agent
linq-pp-cli attachments upload --file ./photo.jpg --agent --dry-run
linq-pp-cli attachments send-url --chat-id ch_123 --url https://cdn.example/photo.jpg --text "Photo attached" --agent --dry-run
linq-pp-cli attachments send-id --chat-id ch_123 --attachment-id att_123 --text "Photo attached" --agent --dry-run
linq-pp-cli attachments audit-url https://cdn.example/photo.jpg --agent
linq-pp-cli attachments cleanup --attachment-id att_123 --agent --dry-run
linq-pp-cli link-preview audit https://example.com --agent
linq-pp-cli link-preview send --chat-id ch_123 --url https://example.com --agent --dry-run
linq-pp-cli link-preview metadata https://example.com --agent --dry-run
linq-pp-cli react add --message-id msg_123 --type like --agent --dry-run
linq-pp-cli react remove --message-id msg_123 --type like --agent --dry-run
linq-pp-cli react custom --message-id msg_123 --emoji "tada" --agent --dry-run
linq-pp-cli contact-share preflight --chat-id ch_123 --from +16282893046 --agent --dry-run
linq-pp-cli contact-share send --chat-id ch_123 --from +16282893046 --yes --agent --dry-run
```

Protocol reminders: typing, effects, and text decorations are iMessage-only; inbound typing indicators are push-only webhook events and this CLI only manages subscriptions or inspects captured streams. Rich previews and reactions work on iMessage/RCS and degrade on SMS. Link preview parts must be the only part in the message, and links are blocked for first outbound `POST /v3/chats` openers.

To enable inbound typing awareness on a webhook subscription, run a dry-run first, then rerun without `--dry-run`:

```bash
linq-pp-cli webhooks add-event \
  sub_123 \
  chat.typing_indicator.started \
  chat.typing_indicator.stopped \
  --agent \
  --dry-run

linq-pp-cli webhooks doctor \
  --subscription-id sub_123 \
  --agent
```

`webhook-subscriptions update-awebhook-subscription` uses `PUT /v3/webhook-subscriptions/{subscriptionId}`. Its `subscribed_events` value is the complete replacement array for that field, not a delta. Prefer `webhooks add-event` and `webhooks remove-event` so the CLI fetches the current set, validates event names against `webhook-events`, computes the union/diff, and sends the replacement array safely.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `linq-pp-cli` binary and the `pp-linq` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install linq
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install linq --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install linq --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install linq --agent claude-code
npx -y @mvanhorn/printing-press-library install linq --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/cmd/linq-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linq-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install linq --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-linq --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-linq --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install linq --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linq-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LINQ_API_V3_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/linq/cmd/linq-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "linq": {
      "command": "linq-pp-mcp",
      "env": {
        "LINQ_API_V3_API_KEY": "<your-key>"
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
linq-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export LINQ_API_V3_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
linq-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
linq-pp-cli contact-card get
```

## Usage

Run `linq-pp-cli --help` for the full command reference and flag list.

## Commands

### attachments

Plan, upload, audit, send, and clean up attachments

- **`linq-pp-cli attachments plan`** - Decide direct URL versus pre-upload based on local file size.
- **`linq-pp-cli attachments upload`** - Request a pre-upload URL and upload file bytes.
- **`linq-pp-cli attachments send-url`** - Send a public HTTPS media URL in a message.
- **`linq-pp-cli attachments send-id`** - Send a pre-uploaded attachment ID in a message.
- **`linq-pp-cli attachments audit-url`** - Check HTTPS and attachment lifecycle guidance for a media URL.
- **`linq-pp-cli attachments cleanup`** - Delete an owned Linq attachment.
- **`linq-pp-cli attachments delete-an`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli attachments get-metadata`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli attachments pre-upload-afile`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### capability

Check whether an address can use iMessage, RCS, or SMS

- **`linq-pp-cli capability check <address>...`** - Resolve each address to `imessage`, `rcs`, or `sms` by checking iMessage then RCS; masks the full address in output.
- **`linq-pp-cli capability check-imessage`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli capability check-rcscapability`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### chats

Manage chats

- **`linq-pp-cli chats create-anew`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats get-achat-by-id`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats list-all`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats update-achat`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### contact-card

Manage iMessage contact cards for sending phone numbers

- **`linq-pp-cli contact-card get`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli contact-card create --phone-number +15551234567 --first-name Acme --last-name Support --image-url https://cdn.example.com/contact-card.jpg`** - Create a contact card for a sending phone number. `setup` remains available as an alias.
- **`linq-pp-cli contact-card update --phone-number +15551234567 --first-name Acme --last-name Support`** - Update an existing active contact card. Use `--stdin` for raw JSON bodies.

### messages

Manage messages

- **`linq-pp-cli messages delete-amessage-from-system`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli messages edit-the-content-of-amessage-part`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli messages get-amessage-by-id`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### phone-numbers

Manage phone numbers

- **`linq-pp-cli phone-numbers`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### phonenumbers

Manage phonenumbers

- **`linq-pp-cli phonenumbers`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### webhook-events

Manage webhook events

- **`linq-pp-cli webhook-events`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### webhook-subscriptions

Manage webhook subscriptions

- **`linq-pp-cli webhook-subscriptions create-anew`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli webhook-subscriptions delete-awebhook-subscription`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli webhook-subscriptions get-awebhook-subscription-by-id`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli webhook-subscriptions list-all`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli webhook-subscriptions update-awebhook-subscription`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### webhooks

Manage webhook subscriptions safely

- **`linq-pp-cli webhooks list`** - List subscriptions with target URLs and subscribed event sets.
- **`linq-pp-cli webhooks show <id>`** - Show one subscription with its subscribed event set.
- **`linq-pp-cli webhooks add-event <id> <event>...`** - Fetch current events, validate requested events, union them, and send the full replacement `subscribed_events` array.
- **`linq-pp-cli webhooks remove-event <id> <event>...`** - Fetch current events, validate requested events, remove them, and send the full replacement `subscribed_events` array.
- **`linq-pp-cli webhooks set-events <id> <event>...`** - Explicitly replace the subscription's event set.
- **`linq-pp-cli webhooks doctor`** - Compare subscriptions against expected events and warn when inbound typing events are missing.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
linq-pp-cli contact-card get

# JSON for scripting and agents
linq-pp-cli contact-card get --json

# Filter to specific fields
linq-pp-cli contact-card get --json --select id,name,status

# Dry run — show the request without sending
linq-pp-cli contact-card get --dry-run

# Create or update a contact card for the sending number
linq-pp-cli contact-card create --phone-number +15551234567 --first-name Acme --last-name Support
linq-pp-cli contact-card update --phone-number +15551234567 --image-url https://cdn.example.com/contact-card.jpg

# Agent mode — JSON + compact + no prompts in one flag
linq-pp-cli contact-card get --agent
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
linq-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/linq-partner-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LINQ_API_V3_API_KEY` | per_call | No | Set to your API credential. |
| `LINQ_API_KEY` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `linq-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `linq-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LINQ_API_V3_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
