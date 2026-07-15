---
name: pp-superhuman
description: "Superhuman email from your terminal or Claude Code, backed by a local SQLite store and agent-native JSON I/O. Trigger phrases: `check my email`, `what's in my inbox`, `draft a reply to <thread>`, `send <draft> with undo`, `snooze this thread`, `look up <email>`, `who is <email>`, `use superhuman`, `run superhuman`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - superhuman-pp-cli
---

# Superhuman — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `superhuman-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install superhuman --cli-only
   ```
2. Verify: `superhuman-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/superhuman/cmd/superhuman-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Pick the Superhuman CLI when you want email read/draft/respond access from a terminal or Claude Code, scriptable thread search backed by a local store, capability discovery via `which`, or Ask AI semantic search piped into other tools.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`workflow`** — Compound commands that combine multiple API operations into one verb (see `workflow --help`).

  ```bash
  superhuman-pp-cli workflow --help
  ```
- **`which`** — Resolve a natural-language capability query to the best matching command from this CLI's curated feature index.

  ```bash
  superhuman-pp-cli which 'snooze a thread for tomorrow'
  ```
- **`lookup`** — Live contact enrichment for an email: name, bio, location, timezone, avatar, social links, and Twitter handle from Superhuman's profile service. Optional `--photo` downloads the contact's avatar.

  ```bash
  superhuman-pp-cli lookup alice@example.com --agent --select name,location,twitterHandle
  ```
- **`userdata write`** — Low-level escape hatch over Superhuman's CRDT mutation endpoint for paths the typed commands don't cover. Dry-run by default; `--apply` to fire; path must start with `users/`.

  ```bash
  superhuman-pp-cli userdata write "users/<google-id>/settings/<key>" '{"value":true}' --apply
  ```

## Command Reference

**ai** — Semantic search via Ask AI

- `superhuman-pp-cli ai` — Ask AI semantic search (SSE stream)

**attachments** — Upload, list, download attachments

- `superhuman-pp-cli attachments` — Upload an attachment for a draft

**drafts** — Drafts — create, update, send, delete

- `superhuman-pp-cli drafts list` — List drafts
- `superhuman-pp-cli drafts new` — Create a fresh outbound draft with generated IDs
- `superhuman-pp-cli drafts write` — Create or update a draft (write to draft path)

**messages** — Individual messages within threads

- `superhuman-pp-cli messages get` — Fetch one message by id
- `superhuman-pp-cli messages get-by-rfc822` — Lookup a message by RFC822 Message-ID
- `superhuman-pp-cli messages list` — List messages with Gmail search syntax
- `superhuman-pp-cli messages query` — Semantic email search

**lookup** — Live contact enrichment for an email address

- `superhuman-pp-cli lookup <email>` — Fetch live profile (name, bio, location, timezone, avatar, links, Twitter handle); `--photo <path>` also downloads the photo

**participants** — Local correspondent analytics

- `superhuman-pp-cli participants list` — List email participants from the local store
- `superhuman-pp-cli participants show <email>` — Show participant details

**reminders** — Snooze reminders for threads

- `superhuman-pp-cli reminders cancel` — Cancel a snooze (un-snooze a thread)
- `superhuman-pp-cli reminders create` — Create a snooze reminder for a thread

**teams** — Team and account info

- `superhuman-pp-cli teams` — List teams the user belongs to

**threads** — Email threads — read, list, search, archive, label

- `superhuman-pp-cli threads get` — Get a thread by ID
- `superhuman-pp-cli threads list` — List recent threads

**send** — Send, schedule, and remind from one command

- `superhuman-pp-cli send` — Send a real email through Superhuman's backend

**snippets** — Reusable Superhuman UI snippets

- `superhuman-pp-cli snippets list` — List saved snippets
- `superhuman-pp-cli snippets get <name>` — Show a snippet
- `superhuman-pp-cli snippets create --name <name>` — Create a snippet
- `superhuman-pp-cli snippets update <name>` — Update a snippet
- `superhuman-pp-cli snippets delete <name>` — Delete a snippet

**awaiting-reply** — Threads that need attention

- `superhuman-pp-cli awaiting-reply` — List threads where the latest message awaits your reply

**watch** — Gmail history stream

- `superhuman-pp-cli watch` — Emit Gmail history deltas as NDJSON

**users** — User account state

- `superhuman-pp-cli users` — User achievements / gamification state

**userdata** — Low-level Superhuman CRDT surface (advanced)

- `superhuman-pp-cli userdata write <path> <json>` — Write a raw value to a CRDT path; dry-run by default, `--apply` to fire, path must start with `users/`


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
superhuman-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Fast triage view across recent threads

```bash
superhuman-pp-cli threads list --limit 100 --json --select id,subject,snippet,participants
```

Pull a wide swath of recent threads with only the fields you need; pipe to jq or an LLM for further triage.

### Folder views and thread filters

```bash
superhuman-pp-cli threads list --type sent --limit 20 --json
superhuman-pp-cli threads list --type starred --label IMPORTANT --json
superhuman-pp-cli threads list --participants-file participants.txt --intersect-with-stdin < thread-ids.txt --json
```

Use `--type` for common mailbox folders: `inbox`, `sent`, `done`, `starred`, `archived`, `spam`, `trash`, and `important`. Add `--label`, `--participants-file`, or `--intersect-with-stdin` when composing local-store filters.

### Draft from stdin and send with an undo window

```bash
echo 'Body text here' | superhuman-pp-cli send --to teammate@example.com --subject 'Update' --body-stdin --undo 30s
```

Write a draft from a piped body, then send it with a 30s undo window: agents can abort before the send fires.

### Create a fresh draft without hand-building IDs

```bash
superhuman-pp-cli drafts new --to teammate@example.com --subject 'Update' --body-stdin < body.txt --json
```

`drafts new` creates a fresh outbound draft and generates the Superhuman-style draft, thread, and RFC822 identifiers.

### Snooze a thread for later

```bash
superhuman-pp-cli reminders create --thread-id <thread-id> --trigger-at 2026-05-13T09:00:00Z
```

Schedule a thread to resurface at a specific time; pair with `reminders cancel` to un-snooze.

### Send with follow-up reminders and scheduled delivery

```bash
superhuman-pp-cli send --to teammate@example.com --subject 'Proposal' --body-file proposal.txt --remind-in 3d --if-no-reply
superhuman-pp-cli send --to teammate@example.com --subject 'Tomorrow' --body 'See you then' --schedule-at '+1d'
superhuman-pp-cli send --cancel-schedule <draft-id>
```

Use `--remind-in` or `--remind-on` to attach a reminder. `--if-no-reply` marks the reminder as conditional on no recipient response. `--schedule-at` accepts RFC3339, relative specs such as `+2d`, and weekday specs such as `Mon 8am`.

### Reuse snippets

```bash
superhuman-pp-cli send --to teammate@example.com --snippet intro --var name=Alice
```

Create or update snippets with `superhuman-pp-cli snippets create` / `superhuman-pp-cli snippets update`. Snippet variables are plain `{{key}}` text replacements. They are not Go templates and do not execute logic.

Snippets sync with Superhuman's Snippets folder. If an older CLI left local snippets in `~/.superhuman-pp-cli/snippets.json`, the first `snippets list` prints a one-time migration hint. The CLI never auto-uploads, modifies, or deletes that local file.

### Search messages and correlate participants

```bash
superhuman-pp-cli messages list --query 'from:teammate@example.com newer_than:7d' --json
superhuman-pp-cli messages get-by-rfc822 '<message-id@example.com>' --json
superhuman-pp-cli participants list --since 30d --sort count --json
superhuman-pp-cli participants show teammate@example.com --json
```

Use Gmail search syntax in `messages list --query`. Use `get-by-rfc822` when another system gives you an RFC822 `Message-ID` header instead of a Gmail message id.

### Watch for Gmail changes

```bash
superhuman-pp-cli watch --once --json
superhuman-pp-cli watch --interval 30s --filter messageAdded
```

`watch` polls Gmail history and emits newline-delimited JSON events. Use `--once` for cron-style checks and `--interval` for a long-running stream.

### Find threads awaiting your reply

```bash
superhuman-pp-cli awaiting-reply --min-age 2h --external-only --json
```

This reads the local SQLite store and returns threads where the latest message is inbound and old enough to need attention.

### Discover a capability by description

```bash
superhuman-pp-cli which 'send an email with an undo window'
```

Resolve a natural-language capability query to the best matching command from this CLI's curated feature index.

### Ask AI semantic search across mail

```bash
superhuman-pp-cli ai --query 'what did Alice say about pricing last week' --agent
```

Run a semantic query through Superhuman's Ask AI surface; `--agent` gives you JSON streams pipeable to other tools.

### Enrich a contact before drafting

```bash
superhuman-pp-cli lookup alice@example.com --agent --select name,location,timeZone,twitterHandle
```

Pull Superhuman's live profile for an email and narrow to the fields you need with `--select`. Add `--photo ./alice.jpg` to also save the contact's avatar. Contacts with no photo on file report that and exit 0.

### Write a raw CRDT path the typed commands don't cover

```bash
# Preview first (dry-run by default):
superhuman-pp-cli userdata write "users/<google-id>/settings/<key>" '{"value":true}'
# Then fire it:
superhuman-pp-cli userdata write "users/<google-id>/settings/<key>" '{"value":true}' --apply
```

Advanced escape hatch. Prefer the typed commands (send, reminders, drafts, snippets) when one exists. The path must start with `users/`, and nothing is sent until `--apply` is passed.

## Anti-Triggers

This CLI delivers mail via the Gmail API, not Superhuman's `/messages/send` MTA. Do not promise Superhuman-server-side send behaviors that the Gmail path doesn't provide. There is no `share` (team draft sharing) or `mail-merge` command yet, and the CLI cannot mint the iOS-audience token some Superhuman-only endpoints require — don't claim those capabilities.

## Auth Setup

Superhuman has no public API key. The durable path reads Chrome's on-disk session and stores refreshable account tokens:

```bash
superhuman-pp-cli auth login --disk
superhuman-pp-cli auth status
superhuman-pp-cli doctor
```

`doctor --json` reports `tokens`, `gmail_oauth`, `auto_refresh_active`, and `binary_age_days`. If a backend call returns 401, the CLI adds a hint to run `superhuman-pp-cli auth login --chrome` when a fresh browser capture is needed.

### Migrating From Pre-Overhaul

Existing commands still work. The newer path adds automatic bootstrap and refresh rather than requiring a manual `sync` before every read. Use `--no-refresh`, a saved profile value, or `SUPERHUMAN_NO_AUTO_REFRESH=1` to opt out. Response envelopes remain opt-in via `--envelope`; use `--envelope off` for raw legacy JSON.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  superhuman-pp-cli drafts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands can wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live", "synced_at": "...", "auto_refresh_skipped": false},
  "results": <data>
}
```

`--envelope minimal` is the default agent-friendly shape, `--envelope full` includes extended freshness fields, and `--envelope off` returns raw command output.

### Auto-Refresh

Read commands run a lightweight Gmail history refresh before command execution. Suppression precedence is explicit flag, then profile, then environment:

```bash
superhuman-pp-cli threads list --no-refresh
SUPERHUMAN_NO_AUTO_REFRESH=1 superhuman-pp-cli messages list --query 'newer_than:1d'
```

Auto-refresh is skipped for setup and diagnostic commands such as `sync`, `auth`, `doctor`, `agent-context`, `profile`, `feedback`, `which`, and `bootstrap`.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
superhuman-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
superhuman-pp-cli feedback --stdin < notes.txt
superhuman-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.superhuman-pp-cli/feedback.jsonl`. They are never POSTed unless `SUPERHUMAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUPERHUMAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
superhuman-pp-cli profile save briefing --json
superhuman-pp-cli --profile briefing drafts list
superhuman-pp-cli profile list --json
superhuman-pp-cli profile show briefing
superhuman-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `superhuman-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add superhuman-pp-mcp -- superhuman-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which superhuman-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   superhuman-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `superhuman-pp-cli <command> --help`.
