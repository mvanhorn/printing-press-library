---
name: pp-bird
description: "Terminal-native CLI for Bird's Conversations and SMS APIs with offline search, batch reconcile, and a local SQLite mirror. Trigger phrases: `send an SMS via Bird`, `audit Bird message delivery`, `search Bird conversations`, `block list opt-outs from Bird`, `Bird tenant readiness check`, `use bird`, `run bird`."
author: "Stephan Stoeber"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bird-pp-cli
    install:
      - kind: go
        bins: [bird-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/cmd/bird-pp-cli
---

# Bird — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bird-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install bird --cli-only
   ```
2. Verify: `bird-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/cmd/bird-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for bird-pp-cli when an AI agent or operator needs to interact with a Bird workspace from a terminal: sending one-off transactional SMS, batching reminders with reconcile, triaging open conversations, auditing message delivery, or maintaining the workspace allow/block list. Prefer this over raw curl when you want offline search, agent-shaped JSON, or a single command that folds multiple Bird endpoints into one answer.

## Unique Capabilities

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

## Command Reference

**channel-config** — Per-channel Conversations API configuration

- `bird-pp-cli channel-config get` — Get the Conversations configuration for a channel
- `bird-pp-cli channel-config update` — Update Conversations configuration for a channel

**channel-media** — Channel-specific pre-signed media uploads

- `bird-pp-cli channel-media <channel_id>` — Create a channel-scoped pre-signed media upload URL

**channels** — SMS channels available in the workspace

- `bird-pp-cli channels get` — Get one channel by ID
- `bird-pp-cli channels list` — List channels in the workspace (filter --kind sms)
- `bird-pp-cli channels messageability` — Check whether a channel can message a given contact (the customer-service window probe)

**compliance** — Channel compliance keyword routing (HELP/STOP/START)


**conversations** — Manage Bird conversation threads (cross-channel customer interactions)

- `bird-pp-cli conversations create` — Start a new conversation
- `bird-pp-cli conversations delete` — Delete a conversation
- `bird-pp-cli conversations get` — Get one conversation by ID
- `bird-pp-cli conversations list` — List conversations across the workspace
- `bird-pp-cli conversations update` — Update a conversation (status, name, etc.)

**media** — Pre-signed media uploads for messages with attachments

- `bird-pp-cli media` — Create a workspace-wide pre-signed media upload URL

**messages** — Channel-level messages (the SMS send/receive layer)

- `bird-pp-cli messages get` — Get one message by ID
- `bird-pp-cli messages interactions` — List delivery-event interactions for a message (sent, delivered, read, failed)
- `bird-pp-cli messages list` — List messages on a channel (chronological)
- `bird-pp-cli messages list-all` — List messages across the workspace

**participants** — Workspace-wide participant lookup

- `bird-pp-cli participants conversations` — List conversations a participant belongs to (by participant ID)
- `bird-pp-cli participants conversations-by-identifier` — List conversations a participant belongs to (by identifier key and value)

**sms** — Programmable SMS send (the headline command)

- `bird-pp-cli sms` — Send an SMS message

**workspace** — Workspace-level configuration: anti-spam and allow/block rules



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bird-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Bulk-send an OTP campaign with reconcile

```bash
bird-pp-cli sms send-batch --csv recipients.csv --body-template "Your code is {{code}}" --apply --json
```

Sends one SMS per CSV row with a deterministic idempotency key and persists the batch in the local store. The returned JSON includes the batchId; pass it to 'bird-pp-cli sms reconcile <batchId> --json' afterward to re-fetch delivery interactions and group failures by reason.

### Audit one message

```bash
bird-pp-cli messages audit msg_abc --channel-id ch_sms_1 --json --select status,events
```

Audit folds the message + interactions into a single timeline with a non-zero exit on terminal failure so it works as a CI gate or a Slackbot trigger.

### Find an OTP we sent yesterday to one customer

```bash
bird-pp-cli sms search "code" --to +31612345678 --json --select id,createdAt,body
```

Runs FTS5 over the local store; works offline once sync has run, no Bird API call. The --select narrows the response so an agent doesn't burn context on full message envelopes.

### Onboard a new SMS tenant

```bash
bird-pp-cli tenant doctor --test-contact contact_42 --json
```

Runs the five-endpoint readiness checklist (channel exists, channel-config enabled, anti-spam on, compliance keywords configured, messageability green) and exits non-zero with the failing check.

### Sync compliance opt-outs into the block list

```bash
bird-pp-cli compliance auto-block --since 7d --apply --json
```

auto-block scans local inbound messages for STOP/HELP/UNSUBSCRIBE and emits a structured candidate list. With --apply it calls workspace rules bulk-add internally to write the block rules in one shot. Drop --apply for a print-only dry run.

## Auth Setup

Authentication uses Bird's `Authorization: AccessKey <key>` header. Set BIRD_API_KEY plus BIRD_WORKSPACE_ID (every Bird endpoint is workspace-scoped) before calling any command. An optional BIRD_CHANNEL_ID provides a default for SMS commands so you don't have to pass --channel-id every time.

Run `bird-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bird-pp-cli channel-config get mock-value --agent --select id,name,status
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
bird-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bird-pp-cli feedback --stdin < notes.txt
bird-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bird-pp-cli/feedback.jsonl`. They are never POSTed unless `BIRD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BIRD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bird-pp-cli profile save briefing --json
bird-pp-cli --profile briefing channel-config get mock-value
bird-pp-cli profile list --json
bird-pp-cli profile show briefing
bird-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bird-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/bird/cmd/bird-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add bird-pp-mcp -- bird-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bird-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bird-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bird-pp-cli <command> --help`.
