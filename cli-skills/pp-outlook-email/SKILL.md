---
name: pp-outlook-email
description: "Drive your personal Microsoft 365 inbox from agents — read, send, sync, and run offline triage analytics that... Trigger phrases: `what's in my inbox`, `find emails from X`, `send an outlook email`, `follow up on email I sent`, `stale unread in outlook`, `who's emailing me most`, `use outlook-email`, `run outlook-email`."
author: "Paul Brennaman"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - outlook-email-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/outlook-email/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Outlook Email — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `outlook-email-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install outlook-email --cli-only
   ```
2. Verify: `outlook-email-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/cmd/outlook-email-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Personal MSA support via OAuth 2.0 device-code against /common. A local SQLite store synced through messages/delta unlocks `followup`, `senders`, `since`, `waiting`, `digest`, `stale-unread`, and `bulk-archive` — workflows no other Outlook CLI exposes because they require persisted state. Companion to outlook-calendar with the same auth playbook.

## When to Use This CLI

Reach for this CLI when an agent needs to triage a personal Microsoft 365 inbox at scale, surface follow-ups, or compose mail without a Copilot license. It is the right choice over generic HTTP because the offline store unlocks cross-folder/cross-time queries (followup, senders, waiting, digest) that require persisted state.

## Unique Capabilities

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

## Command Reference

**attachments** — Attachments on a specific message

- `outlook-email-pp-cli attachments get` — Get a single attachment (metadata + base64 contentBytes for fileAttachment)
- `outlook-email-pp-cli attachments list` — List attachments on a message (metadata only by default)

**categories** — Master list of color categories applied across mail and calendar

- `outlook-email-pp-cli categories create` — Create a new color category
- `outlook-email-pp-cli categories delete` — Delete a color category
- `outlook-email-pp-cli categories get` — Get a category by id
- `outlook-email-pp-cli categories list` — List the master color categories

**folders** — Mail folders and their hierarchy

- `outlook-email-pp-cli folders children` — List child folders under a parent folder
- `outlook-email-pp-cli folders create` — Create a top-level mail folder
- `outlook-email-pp-cli folders create-child` — Create a child folder under a parent
- `outlook-email-pp-cli folders delete` — Delete a mail folder (moves contents to Deleted Items)
- `outlook-email-pp-cli folders delta` — Delta-sync of the mail folder hierarchy
- `outlook-email-pp-cli folders get` — Get a mail folder by id or well-known name
- `outlook-email-pp-cli folders list` — List top-level mail folders
- `outlook-email-pp-cli folders messages` — List messages in a specific folder
- `outlook-email-pp-cli folders update` — Rename a mail folder

**inference** — Focused/Other inbox classification overrides

- `outlook-email-pp-cli inference create-override` — Pin a sender to Focused or Other
- `outlook-email-pp-cli inference delete-override` — Remove a sender pin
- `outlook-email-pp-cli inference list-overrides` — List sender pins to Focused or Other

**mailbox-settings** — Mailbox-level settings: timezone, language, signature, automatic replies

- `outlook-email-pp-cli mailbox-settings get` — Get all mailbox settings
- `outlook-email-pp-cli mailbox-settings update` — Update mailbox settings (automatic replies, timezone, working hours, language)

**messages** — Outlook mail messages on your default mailbox

- `outlook-email-pp-cli messages copy` — Copy a message into another mail folder
- `outlook-email-pp-cli messages create-draft` — Create a draft message (save without sending)
- `outlook-email-pp-cli messages delete` — Delete a message (moves to Deleted Items by default)
- `outlook-email-pp-cli messages delta` — Pull incremental message changes since the last delta token
- `outlook-email-pp-cli messages forward` — Forward a message to new recipients
- `outlook-email-pp-cli messages get` — Get a message by id
- `outlook-email-pp-cli messages list` — List messages across all folders
- `outlook-email-pp-cli messages move` — Move a message to another mail folder
- `outlook-email-pp-cli messages reply` — Send a reply to a message (uses the original sender)
- `outlook-email-pp-cli messages reply-all` — Reply to all recipients of a message
- `outlook-email-pp-cli messages send-draft` — Send a previously-saved draft message
- `outlook-email-pp-cli messages update` — Update message fields (isRead, categories, flag, importance, subject, body)

**rules** — Inbox rules that automatically process incoming messages

- `outlook-email-pp-cli rules create` — Create an inbox rule
- `outlook-email-pp-cli rules delete` — Delete an inbox rule
- `outlook-email-pp-cli rules get` — Get a single inbox rule
- `outlook-email-pp-cli rules list` — List inbox rules
- `outlook-email-pp-cli rules update` — Update an inbox rule

**send_mail** — Send a brand-new email in one shot (skips the drafts folder)

- `outlook-email-pp-cli send_mail` — Compose and send a message without first saving to drafts


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
outlook-email-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning catchup

```bash
outlook-email-pp-cli sync && outlook-email-pp-cli since 12h --agent --select sender,subject,inference_classification,received_at
```

Delta-sync the mailbox, then surface only what landed overnight, grouped for an agent to summarize.

### Follow-up sweep

```bash
outlook-email-pp-cli followup --days 7 --agent --select recipient,subject,sent_at,days_quiet
```

Emails you sent more than a week ago with no reply — the loops you owe yourself to close.

### Top noisy senders

```bash
outlook-email-pp-cli senders --window 30d --min 10 --agent
```

Highest-volume senders over the last 30 days. Feed the output to `bulk-archive` to plan the prune.

### Reply with selected fields only

```bash
outlook-email-pp-cli messages get AAMkAGI2... --agent --select subject,from,body_preview,received_at
```

Narrow a deeply-nested message payload to the four fields an agent actually needs before composing a reply.

### Plan an inbox cleanup

```bash
outlook-email-pp-cli bulk-archive --from-senders senders.txt --to-folder Archive --agent
```

Reads a sender list from senders.txt (one address per line), resolves matching messages in the local store, and prints the move plan. Pass --execute to actually call POST /me/messages/{id}/move per id.

## Auth Setup

OAuth 2.0 device-code flow against `https://login.microsoftonline.com/common`. Personal MSAs (Outlook.com, Hotmail, Live) work alongside work/school accounts. Default client id is the Microsoft Graph PowerShell app, so no Azure tenant is needed. BYO Azure AD app via `--client-id`. Refresh tokens persisted at `~/.config/outlook-email-pp-cli/config.toml`; run `auth refresh` or let commands auto-rotate.

Run `outlook-email-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  outlook-email-pp-cli attachments list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
outlook-email-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
outlook-email-pp-cli feedback --stdin < notes.txt
outlook-email-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.outlook-email-pp-cli/feedback.jsonl`. They are never POSTed unless `OUTLOOK_EMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OUTLOOK_EMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
outlook-email-pp-cli profile save briefing --json
outlook-email-pp-cli --profile briefing attachments list mock-value
outlook-email-pp-cli profile list --json
outlook-email-pp-cli profile show briefing
outlook-email-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `outlook-email-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add outlook-email-pp-mcp -- outlook-email-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which outlook-email-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   outlook-email-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `outlook-email-pp-cli <command> --help`.
