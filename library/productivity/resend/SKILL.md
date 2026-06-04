---
name: pp-resend
description: "The full Resend API (100 endpoints across emails, broadcasts, audiences, contacts, domains, templates, webhooks)... Trigger phrases: `send an email with resend`, `check resend deliverability`, `list resend audiences`, `what did we send to <email>`, `use resend`, `run resend`."
author: "Giuliano Giacaglia"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - resend-pp-cli
---

# Resend — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `resend-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install resend --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `resend-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/resend/cmd/resend-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs to answer cross-resource questions about a Resend account that the dashboard or SDK cannot answer in one call — what did we send to a specific recipient, which audiences is a contact in, how does broadcast performance trend over time, what's the 7-day bounce rate. Stick with `npx resend send` for one-shot transactional sends or with React Email for templates; this CLI is the agent-facing companion, not a replacement.

## Unique Capabilities

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

## Command Reference

**api-keys** — Create and manage API Keys through the Resend API.

- `resend-pp-cli api-keys create` — Create a new API key
- `resend-pp-cli api-keys delete` — Remove an existing API key
- `resend-pp-cli api-keys list` — Retrieve a list of API keys

**audiences** — Deprecated: Use Segments instead. Create and manage Audiences through the Resend API.

- `resend-pp-cli audiences create` — Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- `resend-pp-cli audiences delete` — Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- `resend-pp-cli audiences get` — Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.
- `resend-pp-cli audiences list` — Deprecated: Use Segments instead. These endpoints still work, but will be removed in the future.

**automations** — Create and manage Automations through the Resend API.

- `resend-pp-cli automations create` — Create an automation
- `resend-pp-cli automations delete` — Delete an automation
- `resend-pp-cli automations get` — Retrieve a single automation
- `resend-pp-cli automations list` — Retrieve a list of automations
- `resend-pp-cli automations update` — Update an automation

**broadcasts** — Create and manage Broadcasts through the Resend API.

- `resend-pp-cli broadcasts create` — Create a broadcast
- `resend-pp-cli broadcasts delete` — Remove an existing broadcast that is in the draft status
- `resend-pp-cli broadcasts get` — Retrieve a single broadcast
- `resend-pp-cli broadcasts list` — Retrieve a list of broadcasts
- `resend-pp-cli broadcasts update` — Update an existing broadcast

**contact-properties** — Create and manage Contact Properties through the Resend API.

- `resend-pp-cli contact-properties create` — Create a new contact property
- `resend-pp-cli contact-properties delete` — Remove an existing contact property
- `resend-pp-cli contact-properties get` — Retrieve a single contact property
- `resend-pp-cli contact-properties list` — Retrieve a list of contact properties
- `resend-pp-cli contact-properties update` — Update an existing contact property

**contacts** — Create and manage Contacts through the Resend API.

- `resend-pp-cli contacts create` — Create a new contact
- `resend-pp-cli contacts delete` — Remove an existing contact by ID or email
- `resend-pp-cli contacts get` — Retrieve a single contact by ID or email
- `resend-pp-cli contacts list` — Retrieve a list of contacts
- `resend-pp-cli contacts update` — Update a single contact by ID or email

**domains** — Create and manage domains through the Resend API.

- `resend-pp-cli domains create` — Create a new domain
- `resend-pp-cli domains delete` — Remove an existing domain
- `resend-pp-cli domains get` — Retrieve a single domain
- `resend-pp-cli domains list` — Retrieve a list of domains
- `resend-pp-cli domains update` — Update an existing domain

**emails** — Start sending emails through the Resend API.

- `resend-pp-cli emails create` — Send an email
- `resend-pp-cli emails create-batch` — Trigger up to 100 batch emails at once.
- `resend-pp-cli emails get` — Retrieve a single email
- `resend-pp-cli emails get-receiving` — Retrieve a single received email
- `resend-pp-cli emails get-receiving-2` — Retrieve a list of attachments for a received email
- `resend-pp-cli emails get-receiving-3` — Retrieve a single attachment for a received email
- `resend-pp-cli emails list` — Retrieve a list of emails
- `resend-pp-cli emails list-receiving` — Retrieve a list of received emails
- `resend-pp-cli emails update` — Update a single email

**events** — Create and manage Events through the Resend API.

- `resend-pp-cli events create` — Create an event
- `resend-pp-cli events create-send` — Send an event
- `resend-pp-cli events delete` — Delete an event
- `resend-pp-cli events get` — Retrieve a single event
- `resend-pp-cli events list` — Retrieve a list of events
- `resend-pp-cli events update` — Update an event

**logs** — Retrieve API request logs through the Resend API.

- `resend-pp-cli logs get` — Retrieve a single log
- `resend-pp-cli logs list` — Retrieve a list of logs

**segments** — Create and manage Segments through the Resend API.

- `resend-pp-cli segments create` — Create a new segment
- `resend-pp-cli segments delete` — Remove an existing segment
- `resend-pp-cli segments get` — Retrieve a single segment
- `resend-pp-cli segments list` — Retrieve a list of segments

**templates** — Create and manage Templates through the Resend API.

- `resend-pp-cli templates create` — Create a template
- `resend-pp-cli templates delete` — Remove an existing template
- `resend-pp-cli templates get` — Retrieve a single template
- `resend-pp-cli templates list` — Retrieve a list of templates
- `resend-pp-cli templates update` — Update an existing template

**topics** — Create and manage Topics through the Resend API.

- `resend-pp-cli topics create` — Create a new topic
- `resend-pp-cli topics delete` — Remove an existing topic
- `resend-pp-cli topics get` — Retrieve a single topic
- `resend-pp-cli topics list` — Retrieve a list of topics
- `resend-pp-cli topics update` — Update an existing topic

**webhooks** — Create and manage Webhooks through the Resend API.

- `resend-pp-cli webhooks create` — Create a new webhook
- `resend-pp-cli webhooks delete` — Remove an existing webhook
- `resend-pp-cli webhooks get` — Retrieve a single webhook
- `resend-pp-cli webhooks list` — Retrieve a list of webhooks
- `resend-pp-cli webhooks update` — Update an existing webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
resend-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Reconstruct what a user has received

```bash
resend-pp-cli emails to user@example.com --json --select id,subject,status,sent_at,opened_at --limit 50
```

Use this for support tickets and GDPR exports — every email sent to one recipient, with delivery state.

### Pre-campaign audience check

```bash
resend-pp-cli audiences inventory --json --select name,contact_count,unsubscribed_count,last_broadcast_at
```

Run before scheduling a broadcast — surfaces audiences with high unsubscribe rates or stale engagement.

### Weekly deliverability review

```bash
resend-pp-cli deliverability summary --window 7d --agent --select bounce_rate,complaint_rate,suppression_count
```

Trend snapshot for IP/domain reputation; pipe through `jq` to alert on threshold breaches.

### Audit stale API keys

```bash
resend-pp-cli api-keys rotation --older-than 90d --json --select name,created_at,last_used_at
```

Find unused keys during quarterly security reviews.

### Search every sent email for a phrase

```bash
resend-pp-cli search 'password reset' --json --select id,subject,to,sent_at
```

FTS5-backed; needs `sync --full` first.

## Auth Setup

Run `resend-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
resend-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `RESEND_API_KEY` as an environment variable.

Run `resend-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  resend-pp-cli api-keys list --agent --select id,name,status
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
resend-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
resend-pp-cli feedback --stdin < notes.txt
resend-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.resend-pp-cli/feedback.jsonl`. They are never POSTed unless `RESEND_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RESEND_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
resend-pp-cli profile save briefing --json
resend-pp-cli --profile briefing api-keys list
resend-pp-cli profile list --json
resend-pp-cli profile show briefing
resend-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `resend-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add resend-pp-mcp -- resend-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which resend-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   resend-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `resend-pp-cli <command> --help`.
