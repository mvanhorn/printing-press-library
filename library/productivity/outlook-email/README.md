# Outlook Email CLI

**Drive your personal Microsoft 365 inbox from agents — read, send, sync, and run offline triage analytics that Outlook's own UI never exposes.**

Personal MSA support via OAuth 2.0 device-code against /common. A local SQLite store synced through messages/delta unlocks `followup`, `senders`, `since`, `waiting`, `digest`, `stale-unread`, and `bulk-archive` — workflows no other Outlook CLI exposes because they require persisted state. Companion to outlook-calendar with the same auth playbook.

Printed by [@brennaman](https://github.com/brennaman) (Paul Brennaman).

## Install

The recommended path installs both the `outlook-email-pp-cli` binary and the `pp-outlook-email` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install outlook-email
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install outlook-email --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/outlook-email-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-outlook-email --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-outlook-email --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-outlook-email skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-outlook-email. The skill defines how its required CLI can be installed.
```

## Authentication

OAuth 2.0 device-code flow against `https://login.microsoftonline.com/common`. Personal MSAs (Outlook.com, Hotmail, Live) work alongside work/school accounts. Default client id is the Microsoft Graph PowerShell app, so no Azure tenant is needed. BYO Azure AD app via `--client-id`. Refresh tokens persisted at `~/.config/outlook-email-pp-cli/config.toml`; run `auth refresh` or let commands auto-rotate.

## Quick Start

```bash
# One-time: sign in with a personal MSA via device-code flow.
outlook-email-pp-cli auth login --device-code --launch


# Pull the full mailbox into the local SQLite store. Subsequent syncs are delta.
outlook-email-pp-cli sync --full


# Catch up on what arrived since last check, grouped by focused/other and sender.
outlook-email-pp-cli since 2h --agent


# Surface emails you sent that nobody replied to.
outlook-email-pp-cli followup --days 7 --agent --select sender,subject,sent_at,days_quiet


# See who's drowning your inbox — input to bulk-archive.
outlook-email-pp-cli senders --window 30d --min 5 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`followup`** — List emails you sent more than N days ago that the recipient never replied to — close the loops you forgot about.

  _Sales, PM, and recruiting workflows live and die on follow-up timing. Reach for this when an agent needs to surface stalled threads._

  ```bash
  outlook-email-pp-cli followup --days 7 --agent
  ```
- **`senders`** — Group your inbox by sender over a window: count, unread, last-received, dominant folder — see who's drowning your inbox.

  _Use as the input to bulk-archive or to drive inbox-rule decisions. Agents pruning a flooded inbox start here._

  ```bash
  outlook-email-pp-cli senders --window 30d --min 5 --agent --select sender,count,unread,last_received
  ```
- **`since`** — Show what landed since a timestamp, grouped by focused/other, sender, and folder. Perfect for 'what did I miss while I was heads-down'.

  _The first command an agent should run when joining a session to catch up. Cheap, structured, agent-shaped._

  ```bash
  outlook-email-pp-cli since 2h --agent
  ```
- **`flagged`** — List flagged messages with open due dates, days-overdue, age. The inbox-zero work list Outlook never aggregates.

  _An agent's todo list. Pair with reply/forward to actually close the items._

  ```bash
  outlook-email-pp-cli flagged --overdue --agent
  ```
- **`stale-unread`** — Unread messages older than N days, grouped by folder. Surfaces the unread debt hiding in subfolders.

  _Different from `senders` (volume) — answers 'where is unread piling up' so an agent can route folders to bulk-mark-read._

  ```bash
  outlook-email-pp-cli stale-unread --days 14 --agent
  ```
- **`waiting`** — Conversations where the last message is not from you and is unread or unanswered for N days. The 'someone's waiting on you' work list.

  _Symmetric to followup. Pair both at start-of-day to see your half and their half of every open thread._

  ```bash
  outlook-email-pp-cli waiting --days 3 --agent
  ```
- **`conversations`** — Conversations ranked by message count, participants, and unread tail. Find the threads burning your attention budget.

  _Helps an agent decide which thread to summarize or mute next._

  ```bash
  outlook-email-pp-cli conversations --top 20 --window 30d --agent
  ```
- **`quiet`** — Senders you used to hear from but who've gone silent for N days. Useful for relationship lapse detection.

  _Sales/PM use case: surface customers or vendors who used to engage and don't anymore._

  ```bash
  outlook-email-pp-cli quiet --baseline 90d --silent 30d --agent
  ```
- **`digest`** — One-shot daily summary: received/sent/unread/flagged counts, top senders, top conversations, focused/other ratio.

  _Run this from a cron or skill loop to produce a Slack-shaped daily summary without making 10+ Graph calls._

  ```bash
  outlook-email-pp-cli digest --date 2026-05-12 --agent
  ```
- **`attachments-stale`** — Attachments older than N days and over a size threshold, sortable by size. The mailbox-quota rescue plan.

  _When the mailbox is near quota, agents need the largest oldest attachments first; this command is that list._

  ```bash
  outlook-email-pp-cli attachments-stale --days 90 --min-mb 1 --agent --select sender,received_at,size_mb,name
  ```
- **`dedup`** — Find probable duplicate threads or messages by conversation_id, internet_message_id, or normalized (subject, from, to).

  _Surfaces newsletter duplicates and cross-folder copies before bulk-archive runs._

  ```bash
  outlook-email-pp-cli dedup --by subject-sender --agent
  ```

### Agent-native plumbing
- **`bulk-archive`** — Read a sender list or query, print a move plan, optionally execute it. Safe by default, no surprises.

  _Pair with `senders` or `quiet` for end-to-end inbox triage; agents can audit the plan before opting in to --execute._

  ```bash
  outlook-email-pp-cli bulk-archive --from-senders senders.txt --to-folder Archive --execute
  ```

## Usage

Run `outlook-email-pp-cli --help` for the full command reference and flag list.

## Commands

### attachments

Attachments on a specific message

- **`outlook-email-pp-cli attachments get`** - Get a single attachment (metadata + base64 contentBytes for fileAttachment)
- **`outlook-email-pp-cli attachments list`** - List attachments on a message (metadata only by default)

### categories

Master list of color categories applied across mail and calendar

- **`outlook-email-pp-cli categories create`** - Create a new color category
- **`outlook-email-pp-cli categories delete`** - Delete a color category
- **`outlook-email-pp-cli categories get`** - Get a category by id
- **`outlook-email-pp-cli categories list`** - List the master color categories

### folders

Mail folders and their hierarchy

- **`outlook-email-pp-cli folders children`** - List child folders under a parent folder
- **`outlook-email-pp-cli folders create`** - Create a top-level mail folder
- **`outlook-email-pp-cli folders create-child`** - Create a child folder under a parent
- **`outlook-email-pp-cli folders delete`** - Delete a mail folder (moves contents to Deleted Items)
- **`outlook-email-pp-cli folders delta`** - Delta-sync of the mail folder hierarchy
- **`outlook-email-pp-cli folders get`** - Get a mail folder by id or well-known name
- **`outlook-email-pp-cli folders list`** - List top-level mail folders
- **`outlook-email-pp-cli folders messages`** - List messages in a specific folder
- **`outlook-email-pp-cli folders update`** - Rename a mail folder

### inference

Focused/Other inbox classification overrides

- **`outlook-email-pp-cli inference create-override`** - Pin a sender to Focused or Other
- **`outlook-email-pp-cli inference delete-override`** - Remove a sender pin
- **`outlook-email-pp-cli inference list-overrides`** - List sender pins to Focused or Other

### mailbox-settings

Mailbox-level settings: timezone, language, signature, automatic replies

- **`outlook-email-pp-cli mailbox-settings get`** - Get all mailbox settings
- **`outlook-email-pp-cli mailbox-settings update`** - Update mailbox settings (automatic replies, timezone, working hours, language)

### messages

Outlook mail messages on your default mailbox

- **`outlook-email-pp-cli messages copy`** - Copy a message into another mail folder
- **`outlook-email-pp-cli messages create-draft`** - Create a draft message (save without sending)
- **`outlook-email-pp-cli messages delete`** - Delete a message (moves to Deleted Items by default)
- **`outlook-email-pp-cli messages delta`** - Pull incremental message changes since the last delta token
- **`outlook-email-pp-cli messages forward`** - Forward a message to new recipients
- **`outlook-email-pp-cli messages get`** - Get a message by id
- **`outlook-email-pp-cli messages list`** - List messages across all folders
- **`outlook-email-pp-cli messages move`** - Move a message to another mail folder
- **`outlook-email-pp-cli messages reply`** - Send a reply to a message (uses the original sender)
- **`outlook-email-pp-cli messages reply-all`** - Reply to all recipients of a message
- **`outlook-email-pp-cli messages send-draft`** - Send a previously-saved draft message
- **`outlook-email-pp-cli messages update`** - Update message fields (isRead, categories, flag, importance, subject, body)

### rules

Inbox rules that automatically process incoming messages

- **`outlook-email-pp-cli rules create`** - Create an inbox rule
- **`outlook-email-pp-cli rules delete`** - Delete an inbox rule
- **`outlook-email-pp-cli rules get`** - Get a single inbox rule
- **`outlook-email-pp-cli rules list`** - List inbox rules
- **`outlook-email-pp-cli rules update`** - Update an inbox rule

### send_mail

Send a brand-new email in one shot (skips the drafts folder)

- **`outlook-email-pp-cli send-mail`** - Compose and send a message without first saving to drafts


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
outlook-email-pp-cli attachments list mock-value

# JSON for scripting and agents
outlook-email-pp-cli attachments list mock-value --json

# Filter to specific fields
outlook-email-pp-cli attachments list mock-value --json --select id,name,status

# Dry run — show the request without sending
outlook-email-pp-cli attachments list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
outlook-email-pp-cli attachments list mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-outlook-email -g
```

Then invoke `/pp-outlook-email <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add outlook-email outlook-email-pp-mcp -e OUTLOOK_EMAIL_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/outlook-email-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OUTLOOK_EMAIL_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "outlook-email": {
      "command": "outlook-email-pp-mcp",
      "env": {
        "OUTLOOK_EMAIL_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
outlook-email-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/outlook-email-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OUTLOOK_EMAIL_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `outlook-email-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OUTLOOK_EMAIL_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized after auth login** — Run `outlook-email-pp-cli auth refresh`. If still failing, `auth status` shows scope and expiry.
- **`InvalidAuthenticationToken` mid-sync** — Refresh token expired (>90 days). Run `auth login --device-code` again.
- **Mailbox not enabled for personal MSA** — Open Outlook.com once with the account to provision the Exchange mailbox; then retry sync.
- **Sync stalls on a folder** — Run `sync --folder Inbox --reset` to drop the delta token and re-pull the folder.
- **Search returns nothing but inbox has matches** — Local search reads the SQLite FTS5 index — run `sync` first. Use `--server` to force `$search` against Graph.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**msgcli**](https://github.com/skylarbpayne/msgcli) — Python
- [**outpost**](https://github.com/signalclaude/outpost) — Go
- [**outlook-mcp (sajadghawami)**](https://github.com/sajadghawami/outlook-mcp) — TypeScript
- [**outlook-mcp (XenoXilus)**](https://github.com/XenoXilus/outlook-mcp) — TypeScript
- [**outlook-mcp (ryaker)**](https://github.com/ryaker/outlook-mcp) — TypeScript
- [**OutlookMCPServer**](https://github.com/Norcim133/OutlookMCPServer) — Python
- [**cowork-outlook-plugin**](https://github.com/brendanerofeev/cowork-outlook-plugin) — Python
- [**outlook-cli (mhattingpete)**](https://github.com/mhattingpete/outlook-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
