# Superhuman CLI

**Superhuman email from your terminal or Claude Code, backed by a local SQLite store and agent-native JSON I/O.**

Read, draft, send, schedule, and snooze Superhuman email from your terminal or Claude Code. Pair durable Chrome-session auth with a local SQLite store for offline thread search, participant analysis, watch streams, draft management, and Ask AI semantic queries.

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `superhuman-pp-cli` binary and the `pp-superhuman` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install superhuman
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install superhuman --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install superhuman --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install superhuman --agent claude-code
npx -y @mvanhorn/printing-press-library install superhuman --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/superhuman/cmd/superhuman-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/superhuman-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-superhuman --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-superhuman --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install superhuman --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/superhuman-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SUPERHUMAN_JWT` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "superhuman": {
      "command": "superhuman-pp-mcp",
      "env": {
        "SUPERHUMAN_JWT": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Superhuman has no public API key. The durable path reads Chrome's on-disk session and stores refreshable account tokens locally. Run `superhuman-pp-cli auth setup` for step-by-step instructions.

```bash
superhuman-pp-cli auth login --disk
superhuman-pp-cli auth status
superhuman-pp-cli doctor
```

`doctor --json` reports token refresh state, Gmail OAuth state, local cache state, binary age, and whether auto-refresh is active. If a backend call returns 401, the CLI hints to run `superhuman-pp-cli auth login --chrome` when a fresh browser capture is needed.

## Quick Start

```bash
# One-time: capture durable tokens from Chrome's on-disk session
superhuman-pp-cli auth login --disk

# Confirm auth and connectivity are green
superhuman-pp-cli doctor

# Populate the local SQLite store; read commands refresh recent Gmail history automatically
superhuman-pp-cli bootstrap --per-folder 100

# List recent threads as structured JSON
superhuman-pp-cli threads list --type inbox --limit 20 --json

# Send with an undo window
superhuman-pp-cli send --to teammate@example.com --subject "Update" --body-file body.txt --undo 30s

# Look up Superhuman's live enrichment for a contact
superhuman-pp-cli lookup teammate@example.com --json --select name,location,twitterHandle

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`workflow`** — Compound commands that combine multiple API operations into one verb (see `workflow --help`).

  ```bash
  superhuman-pp-cli workflow --help
  ```
- **`which`** — Resolve a natural-language capability query to the best matching command from this CLI's curated feature index.

  ```bash
  superhuman-pp-cli which 'snooze a thread for tomorrow'
  ```
- **`lookup`** — Live contact enrichment for any email: name, bio, location, timezone, avatar, social links, and Twitter handle, straight from Superhuman's profile service.

  ```bash
  superhuman-pp-cli lookup alice@example.com --json --select name,location,twitterHandle
  ```
- **`userdata write`** — Low-level escape hatch over Superhuman's CRDT mutation endpoint for paths the typed commands don't cover. Dry-run by default; `--apply` to fire.

  ```bash
  superhuman-pp-cli userdata write "users/<google-id>/settings/<key>" '{"value":true}'
  ```

## Usage

Run `superhuman-pp-cli --help` for the full command reference and flag list.

## Commands

### ai

Semantic search via Ask AI

- **`superhuman-pp-cli ai ask`** - Ask AI semantic search (SSE stream)

### attachments

Upload, list, download attachments

- **`superhuman-pp-cli attachments upload`** - Upload an attachment for a draft

### drafts

Drafts — create, update, send, delete

- **`superhuman-pp-cli drafts list`** - List drafts
- **`superhuman-pp-cli drafts new`** - Create a fresh outbound draft
- **`superhuman-pp-cli drafts write`** - Create or update a draft (write to draft path)

### lookup

Live contact enrichment for an email address

- **`superhuman-pp-cli lookup <email>`** - Fetch profile (name, bio, location, timezone, avatar, links, Twitter handle); `--photo <path>` also downloads the contact's photo

### messages

Individual messages within threads

- **`superhuman-pp-cli messages get`** - Fetch one message by id
- **`superhuman-pp-cli messages get-by-rfc822`** - Lookup a message by RFC822 Message-ID
- **`superhuman-pp-cli messages list`** - List messages with Gmail search syntax
- **`superhuman-pp-cli messages query`** - Semantic email search

### participants

Aggregate correspondents from the local message store

- **`superhuman-pp-cli participants list`** - List email participants
- **`superhuman-pp-cli participants show <email>`** - Show participant details

### reminders

Snooze reminders for threads

- **`superhuman-pp-cli reminders cancel`** - Cancel a snooze (un-snooze a thread)
- **`superhuman-pp-cli reminders create`** - Create a snooze reminder for a thread

### teams

Team and account info

- **`superhuman-pp-cli teams suggest`** - List teams the user belongs to

### threads

Email threads — read, list, search, archive, label

- **`superhuman-pp-cli threads get`** - Get a thread by ID
- **`superhuman-pp-cli threads list`** - List recent threads

### send

Send, schedule, and attach follow-up reminders

- **`superhuman-pp-cli send`** - Send a real email through Superhuman's backend

### snippets

Reusable Superhuman UI snippets

- **`superhuman-pp-cli snippets list`** - List saved snippets
- **`superhuman-pp-cli snippets get <name>`** - Show a snippet
- **`superhuman-pp-cli snippets create --name <name>`** - Create a snippet
- **`superhuman-pp-cli snippets update <name>`** - Update a snippet
- **`superhuman-pp-cli snippets delete <name>`** - Delete a snippet

### awaiting-reply

Threads where the latest message awaits your reply

- **`superhuman-pp-cli awaiting-reply`** - List threads requiring attention

### watch

Gmail history stream

- **`superhuman-pp-cli watch`** - Emit Gmail history deltas as NDJSON

### users

User account state

- **`superhuman-pp-cli users achievements`** - User achievements / gamification state

### userdata

Low-level Superhuman CRDT surface (advanced)

- **`superhuman-pp-cli userdata write <path> <json>`** - Write a raw value to a userdata CRDT path. Dry-run by default; pass `--apply` to fire. Path must start with `users/`.

## Freshness and Bootstrap

The CLI keeps a local SQLite store for fast agent workflows. `bootstrap` seeds recent Gmail folders, and read commands run a lightweight Gmail history refresh before execution.

```bash
superhuman-pp-cli bootstrap --folders inbox,sent,archived --per-folder 100
superhuman-pp-cli threads list --type starred --json
superhuman-pp-cli messages list --query 'newer_than:7d has:attachment' --json
```

Auto-refresh is on by default for read workflows and skipped for setup/diagnostic commands. Suppression precedence is explicit flag, then profile, then environment:

```bash
superhuman-pp-cli threads list --no-refresh
SUPERHUMAN_NO_AUTO_REFRESH=1 superhuman-pp-cli participants list
```

Use `--envelope minimal`, `--envelope full`, or `--envelope off` to control the response envelope. The envelope includes freshness metadata under `meta` and command data under `results`.

## Workflow Examples

```bash
# Common folders: inbox, sent, done, starred, archived, spam, trash, important
superhuman-pp-cli threads list --type sent --limit 20 --json

# Compose local filters
superhuman-pp-cli threads list --label IMPORTANT --participants-file people.txt --json

# Find stale inbound threads
superhuman-pp-cli awaiting-reply --min-age 4h --external-only --json

# Reuse snippets with plain {{key}} replacement
superhuman-pp-cli snippets create --name intro --subject "Intro" --body "Hi {{name}},"
superhuman-pp-cli send --to teammate@example.com --snippet intro --var name=Alice

# Schedule delivery or add a conditional follow-up reminder
superhuman-pp-cli send --to teammate@example.com --subject "Tomorrow" --body "See you then" --schedule-at '+1d'
superhuman-pp-cli send --to teammate@example.com --subject "Proposal" --body-file proposal.txt --remind-in 3d --if-no-reply

# Correlate external systems by RFC822 Message-ID
superhuman-pp-cli messages get-by-rfc822 '<message-id@example.com>' --json

# Stream Gmail changes as NDJSON
superhuman-pp-cli watch --once --json
```

## Migrating From Pre-Overhaul

Existing commands and flags continue to work. The overhaul adds automatic bootstrap/refresh behavior, durable auth diagnostics, response-envelope controls, direct `send`, Superhuman-synced snippets, participant analytics, Gmail search passthrough, RFC822 lookup, and watch streams. Use `--no-refresh` or `--envelope off` when a script needs the older no-refresh/raw-output behavior.

Older pre-backend builds stored snippets at `~/.superhuman-pp-cli/snippets.json`. The first `snippets list` after upgrading prints a one-time migration hint if that local file exists and is non-empty. The CLI never auto-uploads, modifies, or deletes that file; recreate any snippets you still need with `snippets create --name <n> --body <b>`.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
superhuman-pp-cli drafts list

# JSON for scripting and agents
superhuman-pp-cli drafts list --json

# Filter to specific fields
superhuman-pp-cli drafts list --json --select id,name,status

# Dry run — show the request without sending
superhuman-pp-cli drafts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
superhuman-pp-cli drafts list --agent
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

## Health Check

```bash
superhuman-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/superhuman-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SUPERHUMAN_JWT` | per_call | Yes | Legacy bearer-token fallback when durable tokens are not configured. |
| `SUPERHUMAN_NO_AUTO_REFRESH` | runtime | No | Set to `1`, `true`, `yes`, or `on` to disable automatic Gmail history refresh. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `superhuman-pp-cli doctor` to check credentials
- Re-capture durable tokens with `superhuman-pp-cli auth login --disk`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 on every backend call** - Run `superhuman-pp-cli doctor --json` and inspect `tokens` / `gmail_oauth`; then run `superhuman-pp-cli auth login --disk` or `superhuman-pp-cli auth login --chrome`.
- **`auth setup` says no token configured** - Run `superhuman-pp-cli auth login --disk`; use `SUPERHUMAN_JWT` only as a legacy fallback.
- **`threads list` is empty after bootstrap** - Confirm `doctor --json` shows cache resources; if not, re-run `bootstrap --full`.
- **`ai` returns 400 invalid-question-event-id** - The active account may not match the stored token; run `auth status` and `auth use <email>`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**edwinhu/superhuman-cli**](https://github.com/edwinhu/superhuman-cli) — TypeScript (3 stars)
- [**superhuman/mcp-mail**](https://github.com/superhuman/mcp-mail) — JavaScript
- [**himalaya**](https://github.com/pimalaya/himalaya) — Rust
- [**aerc**](https://git.sr.ht/~rjarry/aerc) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
