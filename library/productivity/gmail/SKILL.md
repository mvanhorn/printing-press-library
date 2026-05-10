---
name: pp-gmail
description: "Your entire Gmail mailbox in a local SQLite database — searchable offline, queryable with SQL, and pipeline-ready... Trigger phrases: `search my gmail`, `download email attachments`, `find newsletters to unsubscribe`, `who emails me the most`, `use gmail`, `run gmail`, `inbox digest`, `bulk archive gmail`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gmail-pp-cli
---

# Gmail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gmail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install gmail --cli-only
   ```
2. Verify: `gmail-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Gmail MCP tool makes live API calls for every operation. The Gmail CLI syncs your mailbox once and lets you query it as a database: FTS search in milliseconds, sender analytics, newsletter detection, attachment pipelines, inbox digests — all offline. Plus the full live API surface for sending, labeling, threading, and managing settings.

## When to Use This CLI

Use the Gmail CLI when you need to work with email data at scale: bulk attachment extraction, sender analytics, inbox triage automation, or building pipelines that process email content. Ideal for developers and power users who want offline access to their mailbox, SQL-queryable email data, and agent-friendly JSON output for downstream automation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`attachments export`** — Download every attachment matching a Gmail search query to a local directory — invoice extraction in one command.

  _Use when you need to bulk-extract invoice PDFs, report attachments, or any attachment type from a set of emails — without writing a script._

  ```bash
  gmail-pp-cli attachments export --query "from:vendor@acme.com has:attachment" --dir ~/invoices --agent
  ```
- **`senders top`** — Rank your top email senders by volume with unsubscribe link detection — the inbox transparency view Gmail's UI never shows you.

  _Use when you need to find who floods your inbox before a cleanup session, or identify newsletter candidates for bulk unsubscribe._

  ```bash
  gmail-pp-cli senders top --limit 20 --period 30d --unsubscribe --agent
  ```
- **`newsletters list`** — Surface every sender with a List-Unsubscribe header grouped by domain — your actionable unsubscribe queue.

  _Use before a Friday inbox cleanup to get a ranked list of newsletters with direct unsubscribe URLs — no web UI clicking required._

  ```bash
  gmail-pp-cli newsletters list --agent
  ```
- **`attachments list`** — List all attachments in your synced mailbox filtered by MIME type or Gmail query — zero API calls after sync.

  _Use before running attachments export to preview what will be downloaded, or to find a specific file without opening Gmail._

  ```bash
  gmail-pp-cli attachments list --type application/pdf --query "from:accounting@" --agent
  ```
- **`inbox age`** — See how old your unread mail is — bucketed by today / 1-7d / 8-30d / 30-90d / 90d+ per label.

  _Use during inbox cleanup to understand the shape of your unread pile before deciding what to bulk-archive._

  ```bash
  gmail-pp-cli inbox age --label INBOX --agent
  ```
- **`sent stats`** — Count outbound emails to a recipient domain in a time window — the one-command compliance audit.

  _Use before a compliance review to verify email volume to a customer or partner without exporting all mail._

  ```bash
  gmail-pp-cli sent stats --to-domain acmecorp.com --period 7d --agent
  ```

### Agent-native plumbing
- **`digest`** — Your daily inbox summary in one command — threads grouped by label with sender, subject, and unread count.

  _Use every morning to get a structured inbox summary pipeable to an agent or shell script without opening Gmail._

  ```bash
  gmail-pp-cli digest --since yesterday --label IMPORTANT --agent
  ```
- **`stale`** — Check when your local mailbox was last synced and whether the history token is still fresh.

  _Use before running any local-store query to verify your data is current and diagnose why results might be stale._

  ```bash
  gmail-pp-cli stale --agent
  ```

## Command Reference

**drafts** — Manage drafts

- `gmail-pp-cli drafts create` — Creates a new draft with the `DRAFT` label.
- `gmail-pp-cli drafts delete` — Immediately and permanently deletes the specified draft. Does not simply trash it.
- `gmail-pp-cli drafts get` — Gets the specified draft.
- `gmail-pp-cli drafts list` — Lists the drafts in the user's mailbox.
- `gmail-pp-cli drafts send` — Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- `gmail-pp-cli drafts update` — Replaces a draft's content.

**gmail-profile** — Manage gmail profile

- `gmail-pp-cli gmail-profile <userId>` — Gets the current user's Gmail profile.

**history** — Manage history

- `gmail-pp-cli history <userId>` — Lists the history of all changes to the given mailbox. History results are returned in chronological order...

**labels** — Manage labels

- `gmail-pp-cli labels create` — Creates a new label.
- `gmail-pp-cli labels delete` — Immediately and permanently deletes the specified label and removes it from any messages and threads that it is...
- `gmail-pp-cli labels get` — Gets the specified label.
- `gmail-pp-cli labels list` — Lists all labels in the user's mailbox.
- `gmail-pp-cli labels patch` — Patch the specified label.
- `gmail-pp-cli labels update` — Updates the specified label.

**messages** — Manage messages

- `gmail-pp-cli messages batch-delete` — Deletes many messages by message ID. Provides no guarantees that messages were not already deleted or even existed...
- `gmail-pp-cli messages batch-modify` — Modifies the labels on the specified messages.
- `gmail-pp-cli messages delete` — Immediately and permanently deletes the specified message. This operation cannot be undone. Prefer `messages.trash`...
- `gmail-pp-cli messages get` — Gets the specified message.
- `gmail-pp-cli messages import` — Imports a message into only this user's mailbox, with standard email delivery scanning and classification similar to...
- `gmail-pp-cli messages insert` — Directly inserts a message into only this user's mailbox similar to `IMAP APPEND`, bypassing most scanning and...
- `gmail-pp-cli messages list` — Lists the messages in the user's mailbox.
- `gmail-pp-cli messages send` — Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers. For example usage, see [Sending...

**settings** — Manage settings

- `gmail-pp-cli settings create` — Adds a delegate with its verification status set directly to `accepted`, without sending any verification email. The...
- `gmail-pp-cli settings create-gmail` — Creates a filter. Note: you can only create a maximum of 1,000 filters.
- `gmail-pp-cli settings create-gmail-2` — Creates a forwarding address. If ownership verification is required, a message will be sent to the recipient and the...
- `gmail-pp-cli settings create-gmail-3` — Creates a custom 'from' send-as alias. If an SMTP MSA is specified, Gmail will attempt to connect to the SMTP...
- `gmail-pp-cli settings create-gmail-4` — Creates and configures a client-side encryption identity that's authorized to send mail from the user account....
- `gmail-pp-cli settings create-gmail-5` — Creates and uploads a client-side encryption S/MIME public key certificate chain and private key metadata for the...
- `gmail-pp-cli settings delete` — Removes the specified delegate (which can be of any verification status), and revokes any verification that may have...
- `gmail-pp-cli settings delete-gmail` — Immediately and permanently deletes the specified filter.
- `gmail-pp-cli settings delete-gmail-2` — Deletes the specified forwarding address and revokes any verification that may have been required. This method is...
- `gmail-pp-cli settings delete-gmail-3` — Deletes the specified send-as alias. Revokes any verification that may have been required for using it. This method...
- `gmail-pp-cli settings delete-gmail-4` — Deletes a client-side encryption identity. The authenticated user can no longer use the identity to send encrypted...
- `gmail-pp-cli settings delete-gmail-5` — Deletes the specified S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings disable` — Turns off a client-side encryption key pair. The authenticated user can no longer use the key pair to decrypt...
- `gmail-pp-cli settings enable` — Turns on a client-side encryption key pair that was turned off. The key pair becomes active again for any associated...
- `gmail-pp-cli settings get` — Gets the specified delegate. Note that a delegate user must be referred to by their primary email address, and not...
- `gmail-pp-cli settings get-auto-forwarding` — Gets the auto-forwarding setting for the specified account.
- `gmail-pp-cli settings get-gmail` — Gets a filter.
- `gmail-pp-cli settings get-gmail-2` — Gets the specified forwarding address.
- `gmail-pp-cli settings get-gmail-3` — Gets the specified send-as alias. Fails with an HTTP 404 error if the specified address is not a member of the...
- `gmail-pp-cli settings get-gmail-4` — Retrieves a client-side encryption identity configuration.
- `gmail-pp-cli settings get-gmail-5` — Retrieves an existing client-side encryption key pair.
- `gmail-pp-cli settings get-gmail-6` — Gets the specified S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings get-imap` — Gets IMAP settings.
- `gmail-pp-cli settings get-language` — Gets language settings.
- `gmail-pp-cli settings get-pop` — Gets POP settings.
- `gmail-pp-cli settings get-vacation` — Gets vacation responder settings.
- `gmail-pp-cli settings insert` — Insert (upload) the given S/MIME config for the specified send-as alias. Note that pkcs12 format is required for the...
- `gmail-pp-cli settings list` — Lists the delegates for the specified account. This method is only available to service account clients that have...
- `gmail-pp-cli settings list-gmail` — Lists the message filters of a Gmail user.
- `gmail-pp-cli settings list-gmail-2` — Lists the forwarding addresses for the specified account.
- `gmail-pp-cli settings list-gmail-3` — Lists the send-as aliases for the specified account. The result includes the primary send-as address associated with...
- `gmail-pp-cli settings list-gmail-4` — Lists the client-side encrypted identities for an authenticated user.
- `gmail-pp-cli settings list-gmail-5` — Lists client-side encryption key pairs for an authenticated user.
- `gmail-pp-cli settings list-gmail-6` — Lists S/MIME configs for the specified send-as alias.
- `gmail-pp-cli settings obliterate` — Deletes a client-side encryption key pair permanently and immediately. You can only permanently delete key pairs...
- `gmail-pp-cli settings patch` — Patch the specified send-as alias.
- `gmail-pp-cli settings patch-gmail` — Associates a different key pair with an existing client-side encryption identity. The updated key pair must validate...
- `gmail-pp-cli settings set-default` — Sets the default S/MIME config for the specified send-as alias.
- `gmail-pp-cli settings update` — Updates a send-as alias. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias....
- `gmail-pp-cli settings update-auto-forwarding` — Updates the auto-forwarding setting for the specified account. A verified forwarding address must be specified when...
- `gmail-pp-cli settings update-imap` — Updates IMAP settings.
- `gmail-pp-cli settings update-language` — Updates language settings. If successful, the return object contains the `displayLanguage` that was saved for the...
- `gmail-pp-cli settings update-pop` — Updates POP settings.
- `gmail-pp-cli settings update-vacation` — Updates vacation responder settings.
- `gmail-pp-cli settings verify` — Sends a verification email to the specified send-as alias address. The verification status must be `pending`. This...

**stop** — Manage stop

- `gmail-pp-cli stop <userId>` — Stop receiving push notifications for the given user mailbox.

**threads** — Manage threads

- `gmail-pp-cli threads delete` — Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted....
- `gmail-pp-cli threads get` — Gets the specified thread.
- `gmail-pp-cli threads list` — Lists the threads in the user's mailbox.

**watch** — Manage watch

- `gmail-pp-cli watch <userId>` — Set up or update a push notification watch on the given user mailbox.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gmail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Extract all PDF invoices from the last 30 days

```bash
gmail-pp-cli attachments export --query "has:attachment after:2026/04/01" --dir ~/invoices --agent --select filename,sender,date,size
```

Finds all attachments in the local store matching the query, then batch-fetches and saves each one — the --select flag keeps the JSON summary tight.

### Friday inbox cleanup — find newsletter candidates

```bash
gmail-pp-cli newsletters list --agent | jq '.[] | select(.message_count > 5) | .unsubscribe_url'
```

Lists senders with List-Unsubscribe headers, filters to high-volume ones, extracts the unsubscribe URLs for one-click action.

### Morning digest piped to an agent

```bash
gmail-pp-cli digest --since yesterday --agent --select label,from,subject,snippet,unread_count
```

Returns a compact JSON array of thread summaries grouped by label — pass to an LLM to prioritize your morning.

### Compliance audit — emails sent to a customer domain

```bash
gmail-pp-cli sent stats --to-domain acmecorp.com --period 30d --agent
```

Counts outbound messages to a domain from the local SENT archive — no export required for the weekly compliance report.

### Inbox age check before bulk archive

```bash
gmail-pp-cli inbox age --label INBOX --agent --select bucket,count,oldest_date
```

Shows how old your unread pile is before you commit to bulk-archiving — the --select keeps the payload small for agent consumption.

## Auth Setup

Gmail uses OAuth2. Run `gmail-pp-cli auth login` to open a browser window for the Google OAuth2 consent screen. Your refresh token is stored in `~/.config/gmail-pp-cli/token.json` and refreshed automatically. Set `GMAIL_CLIENT_ID` and `GMAIL_CLIENT_SECRET` from a Google Cloud Console project with the Gmail API enabled.

Run `gmail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gmail-pp-cli drafts list mock-value --agent --select id,name,status
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
gmail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gmail-pp-cli feedback --stdin < notes.txt
gmail-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gmail-pp-cli/feedback.jsonl`. They are never POSTed unless `GMAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GMAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gmail-pp-cli profile save briefing --json
gmail-pp-cli --profile briefing drafts list mock-value
gmail-pp-cli profile list --json
gmail-pp-cli profile show briefing
gmail-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gmail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gmail-pp-mcp -- gmail-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gmail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gmail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gmail-pp-cli <command> --help`.
