# Linq CLI

Community-curated OpenAPI 3.1 blueprint for the Linq Partner API, derived from https://docs.linqapp.com/api and cross-checked against github.com/linq-team/linq-go/api.md. The blueprint is generic and contains no RonanRx-specific behavior or PHI examples.

Learn more at [Linq](https://docs.linqapp.com/api/).

## RonanRx private safety layer

This private print adds an agent-safe messaging layer for RonanRx workflows. Agents can sync/search/tail the Linq message stream through an encrypted local mirror, but outbound messaging is routed through guarded commands instead of exposing raw send endpoints as the agent path.

Novel commands:

- `invite-link` — builds a click-to-text inbound front door using routing text plus an opaque token/link, never PHI.
- `welcome-flow` — produces the full inbound-first welcome-flow plan: invite link, secure-link audit, consent audit step, safe draft, guarded send step, and monitoring commands.
- `send` — guarded outbound path; refuses cold sends, opted-out recipients, PHI-shaped bodies/links, and has no cold-send override.
- `send-preflight` — explains whether a send would be allowed without sending.
- `safe-reply-draft` — creates a redacted human-review draft without sending.
- `consent-audit` — summarizes local inbound/opt-out evidence for a chat.
- `needs-human` — finds conversations that should be reviewed by a human.
- `link-audit` — checks HTTPS, allowlisted-host, and no-PHI URL safety.
- `purge` — purges encrypted local mirror rows older than the configured retention window.
- `channel-health`, `pick-number`, `at-risk`, `response-latency`, `delivery-health`, `message-stats`, `health`, `opt-outs`, `trends` — local operational insight commands over the encrypted mirror.

Raw mutating endpoint mirrors are hidden from MCP; `send`/`send-preflight` are the intended agent-facing outbound controls.

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

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

Manage attachments

- **`linq-pp-cli attachments delete-an`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli attachments get-metadata`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli attachments pre-upload-afile`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### capability

Manage capability

- **`linq-pp-cli capability check-imessage`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli capability check-rcscapability`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### chats

Manage chats

- **`linq-pp-cli chats create-anew`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats get-achat-by-id`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats list-all`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli chats update-achat`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

### contact-card

Manage contact card

- **`linq-pp-cli contact-card get`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli contact-card setup`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.
- **`linq-pp-cli contact-card update`** - Source: https://docs.linqapp.com/api. Cross-check: github.com/linq-team/linq-go/api.md.

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
