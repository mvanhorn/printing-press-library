---
name: pp-etherpad
description: "Printing Press CLI for Etherpad. Etherpad is a real-time collaborative editor scalable to thousands of simultaneous real time users. It provides full..."
author: "John McLear"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - etherpad-pp-cli
---

# Etherpad — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `etherpad-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install etherpad --cli-only
   ```
2. Verify: `etherpad-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/etherpad/cmd/etherpad-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**anonymize-author** — Manage anonymize author

- `etherpad-pp-cli anonymize-author` — anonymizes an author across all their edits

**append-chat-message** — Manage append chat message

- `etherpad-pp-cli append-chat-message` — appends a chat message

**append-text** — Manage append text

- `etherpad-pp-cli append-text` — appends text to a pad

**check-token** — Manage check token

- `etherpad-pp-cli check-token` — returns ok when the current API token is valid

**compact-pad** — Manage compact pad

- `etherpad-pp-cli compact-pad` — compacts a pad's revision history, keeping recent revisions only

**copy-pad** — Manage copy pad

- `etherpad-pp-cli copy-pad` — copies a pad with full history and chat

**copy-pad-without-history** — Manage copy pad without history

- `etherpad-pp-cli copy-pad-without-history` — copies a pad without history or chat

**create-author** — Manage create author

- `etherpad-pp-cli create-author` — creates a new author

**create-author-if-not-exists-for** — Manage create author if not exists for

- `etherpad-pp-cli create-author-if-not-exists-for` — this functions helps you to map your application author ids to Etherpad author ids

**create-diff-html** — Manage create diff html

- `etherpad-pp-cli create-diff-html` — returns an HTML diff between two revisions of a pad

**create-group** — Manage create group

- `etherpad-pp-cli create-group` — creates a new group

**create-group-if-not-exists-for** — Manage create group if not exists for

- `etherpad-pp-cli create-group-if-not-exists-for` — this functions helps you to map your application group ids to Etherpad group ids

**create-group-pad** — Manage create group pad

- `etherpad-pp-cli create-group-pad` — creates a new pad in this group

**create-pad** — Manage create pad

- `etherpad-pp-cli create-pad` — creates a new (non-group) pad. Note that if you need to create a group Pad, you should call createGroupPad

**create-session** — Manage create session

- `etherpad-pp-cli create-session` — creates a new session. validUntil is an unix timestamp in seconds

**delete-group** — Manage delete group

- `etherpad-pp-cli delete-group` — deletes a group

**delete-pad** — Manage delete pad

- `etherpad-pp-cli delete-pad` — deletes a pad

**delete-session** — Manage delete session

- `etherpad-pp-cli delete-session` — deletes a session

**get-attribute-pool** — Manage get attribute pool

- `etherpad-pp-cli get-attribute-pool` — returns the attribute pool of a pad

**get-author-name** — Manage get author name

- `etherpad-pp-cli get-author-name` — Returns the Author Name of the author

**get-chat-head** — Manage get chat head

- `etherpad-pp-cli get-chat-head` — returns the chatHead (chat-message) of the pad

**get-chat-history** — Manage get chat history

- `etherpad-pp-cli get-chat-history` — returns the chat history

**get-html** — Manage get html

- `etherpad-pp-cli get-html` — returns the text of a pad formatted as HTML

**get-last-edited** — Manage get last edited

- `etherpad-pp-cli get-last-edited` — returns the timestamp of the last revision of the pad

**get-pad-id** — Manage get pad id

- `etherpad-pp-cli get-pad-id` — returns the read-write pad ID for a given read-only pad ID

**get-public-status** — Manage get public status

- `etherpad-pp-cli get-public-status` — return true of false

**get-read-only-id** — Manage get read only id

- `etherpad-pp-cli get-read-only-id` — returns the read only link of a pad

**get-revision-changeset** — Manage get revision changeset

- `etherpad-pp-cli get-revision-changeset` — returns the changeset at a given revision of a pad

**get-revisions-count** — Manage get revisions count

- `etherpad-pp-cli get-revisions-count` — returns the number of revisions of this pad

**get-saved-revisions-count** — Manage get saved revisions count

- `etherpad-pp-cli get-saved-revisions-count` — returns the number of saved revisions of a pad

**get-session-info** — Manage get session info

- `etherpad-pp-cli get-session-info` — returns information about a session

**get-stats** — Manage get stats

- `etherpad-pp-cli get-stats` — returns server-wide statistics

**get-text** — Manage get text

- `etherpad-pp-cli get-text` — returns the text of a pad

**list-all-groups** — Manage list all groups

- `etherpad-pp-cli list-all-groups` — returns the IDs of all groups on this server

**list-all-pads** — Manage list all pads

- `etherpad-pp-cli list-all-pads` — list all the pads

**list-authors-of-pad** — Manage list authors of pad

- `etherpad-pp-cli list-authors-of-pad` — returns an array of authors who contributed to this pad

**list-pads** — Manage list pads

- `etherpad-pp-cli list-pads` — returns all pads of this group

**list-pads-of-author** — Manage list pads of author

- `etherpad-pp-cli list-pads-of-author` — returns an array of all pads this author contributed to

**list-saved-revisions** — Manage list saved revisions

- `etherpad-pp-cli list-saved-revisions` — returns the list of saved revisions of a pad

**list-sessions-of-author** — Manage list sessions of author

- `etherpad-pp-cli list-sessions-of-author` — returns all sessions of an author

**list-sessions-of-group** — Manage list sessions of group

- `etherpad-pp-cli list-sessions-of-group` — returns all sessions of a group

**move-pad** — Manage move pad

- `etherpad-pp-cli move-pad` — moves a pad — copy then delete the original

**pad-users** — Manage pad users

- `etherpad-pp-cli pad-users` — returns the list of users that are currently editing this pad

**pad-users-count** — Manage pad users count

- `etherpad-pp-cli pad-users-count` — returns the number of user that are currently editing this pad

**restore-revision** — Manage restore revision

- `etherpad-pp-cli restore-revision` — restores a pad to a specific revision

**save-revision** — Manage save revision

- `etherpad-pp-cli save-revision` — saves a revision of a pad

**send-clients-message** — Manage send clients message

- `etherpad-pp-cli send-clients-message` — sends a custom message of type msg to the pad

**set-html** — Manage set html

- `etherpad-pp-cli set-html` — sets the text of a pad with HTML

**set-public-status** — Manage set public status

- `etherpad-pp-cli set-public-status` — sets a boolean for the public status of a pad

**set-text** — Manage set text

- `etherpad-pp-cli set-text` — sets the text of a pad


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
etherpad-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
etherpad-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `ETHERPAD_OPENID` as an environment variable.

Run `etherpad-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  etherpad-pp-cli anonymize-author --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
etherpad-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
etherpad-pp-cli feedback --stdin < notes.txt
etherpad-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.etherpad-pp-cli/feedback.jsonl`. They are never POSTed unless `ETHERPAD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ETHERPAD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
etherpad-pp-cli profile save briefing --json
etherpad-pp-cli --profile briefing anonymize-author
etherpad-pp-cli profile list --json
etherpad-pp-cli profile show briefing
etherpad-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `etherpad-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add etherpad-pp-mcp -- etherpad-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which etherpad-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   etherpad-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `etherpad-pp-cli <command> --help`.
