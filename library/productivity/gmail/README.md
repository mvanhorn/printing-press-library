# Gmail CLI

**Your entire Gmail mailbox in a local SQLite database — searchable offline, queryable with SQL, and pipeline-ready for agents.**

Every Gmail MCP tool makes live API calls for every operation. The Gmail CLI syncs your mailbox once and lets you query it as a database: FTS search in milliseconds, sender analytics, newsletter detection, attachment pipelines, inbox digests — all offline. Plus the full live API surface for sending, labeling, threading, and managing settings.

Learn more at [Gmail](https://google.com).

## Install

The recommended path installs both the `gmail-pp-cli` binary and the `pp-gmail` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install gmail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install gmail --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-gmail skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-gmail. The skill defines how its required CLI can be installed.
```

## Authentication

Gmail uses OAuth2. Run `gmail-pp-cli auth login` to open a browser window for the Google OAuth2 consent screen. Your refresh token is stored in `~/.config/gmail-pp-cli/token.json` and refreshed automatically. Set `GMAIL_CLIENT_ID` and `GMAIL_CLIENT_SECRET` from a Google Cloud Console project with the Gmail API enabled.

## Quick Start

```bash
# Authenticate via browser — stores refresh token locally
gmail-pp-cli auth login


# Pull all messages, threads, labels, and attachment metadata into local SQLite
gmail-pp-cli sync --full


# Morning inbox summary — no API call needed after sync
gmail-pp-cli digest --since yesterday --label IMPORTANT


# Sender leaderboard with newsletter detection — Friday cleanup starting point
gmail-pp-cli senders top --limit 20 --period 30d --unsubscribe


# Bulk-download all matching attachments to a local directory
gmail-pp-cli attachments export --query "has:attachment from:accounting@" --dir ~/invoices

```

## Unique Features

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

## Usage

Run `gmail-pp-cli --help` for the full command reference and flag list.

## Commands

### drafts

Manage drafts

- **`gmail-pp-cli drafts create`** - Creates a new draft with the `DRAFT` label.
- **`gmail-pp-cli drafts delete`** - Immediately and permanently deletes the specified draft. Does not simply trash it.
- **`gmail-pp-cli drafts get`** - Gets the specified draft.
- **`gmail-pp-cli drafts list`** - Lists the drafts in the user's mailbox.
- **`gmail-pp-cli drafts send`** - Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- **`gmail-pp-cli drafts update`** - Replaces a draft's content.

### gmail-profile

Manage gmail profile

- **`gmail-pp-cli gmail-profile get`** - Gets the current user's Gmail profile.

### history

Manage history

- **`gmail-pp-cli history list`** - Lists the history of all changes to the given mailbox. History results are returned in chronological order (increasing `historyId`).

### labels

Manage labels

- **`gmail-pp-cli labels create`** - Creates a new label.
- **`gmail-pp-cli labels delete`** - Immediately and permanently deletes the specified label and removes it from any messages and threads that it is applied to.
- **`gmail-pp-cli labels get`** - Gets the specified label.
- **`gmail-pp-cli labels list`** - Lists all labels in the user's mailbox.
- **`gmail-pp-cli labels patch`** - Patch the specified label.
- **`gmail-pp-cli labels update`** - Updates the specified label.

### messages

Manage messages

- **`gmail-pp-cli messages batch-delete`** - Deletes many messages by message ID. Provides no guarantees that messages were not already deleted or even existed at all.
- **`gmail-pp-cli messages batch-modify`** - Modifies the labels on the specified messages.
- **`gmail-pp-cli messages delete`** - Immediately and permanently deletes the specified message. This operation cannot be undone. Prefer `messages.trash` instead.
- **`gmail-pp-cli messages get`** - Gets the specified message.
- **`gmail-pp-cli messages import`** - Imports a message into only this user's mailbox, with standard email delivery scanning and classification similar to receiving via SMTP. This method doesn't perform SPF checks, so it might not work for some spam messages, such as those attempting to perform domain spoofing. This method does not send a message.
- **`gmail-pp-cli messages insert`** - Directly inserts a message into only this user's mailbox similar to `IMAP APPEND`, bypassing most scanning and classification. Does not send a message.
- **`gmail-pp-cli messages list`** - Lists the messages in the user's mailbox.
- **`gmail-pp-cli messages send`** - Sends the specified message to the recipients in the `To`, `Cc`, and `Bcc` headers. For example usage, see [Sending email](https://developers.google.com/gmail/api/guides/sending).

### settings

Manage settings

- **`gmail-pp-cli settings create`** - Adds a delegate with its verification status set directly to `accepted`, without sending any verification email. The delegate user must be a member of the same Google Workspace organization as the delegator user. Gmail imposes limitations on the number of delegates and delegators each user in a Google Workspace organization can have. These limits depend on your organization, but in general each user can have up to 25 delegates and up to 10 delegators. Note that a delegate user must be referred to by their primary email address, and not an email alias. Also note that when a new delegate is created, there may be up to a one minute delay before the new delegate is available for use. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings create-gmail`** - Creates a filter. Note: you can only create a maximum of 1,000 filters.
- **`gmail-pp-cli settings create-gmail-2`** - Creates a forwarding address. If ownership verification is required, a message will be sent to the recipient and the resource's verification status will be set to `pending`; otherwise, the resource will be created with verification status set to `accepted`. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings create-gmail-3`** - Creates a custom "from" send-as alias. If an SMTP MSA is specified, Gmail will attempt to connect to the SMTP service to validate the configuration before creating the alias. If ownership verification is required for the alias, a message will be sent to the email address and the resource's verification status will be set to `pending`; otherwise, the resource will be created with verification status set to `accepted`. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings create-gmail-4`** - Creates and configures a client-side encryption identity that's authorized to send mail from the user account. Google publishes the S/MIME certificate to a shared domain-wide directory so that people within a Google Workspace organization can encrypt and send mail to the identity.
- **`gmail-pp-cli settings create-gmail-5`** - Creates and uploads a client-side encryption S/MIME public key certificate chain and private key metadata for the authenticated user.
- **`gmail-pp-cli settings delete`** - Removes the specified delegate (which can be of any verification status), and revokes any verification that may have been required for using it. Note that a delegate user must be referred to by their primary email address, and not an email alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings delete-gmail`** - Immediately and permanently deletes the specified filter.
- **`gmail-pp-cli settings delete-gmail-2`** - Deletes the specified forwarding address and revokes any verification that may have been required. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings delete-gmail-3`** - Deletes the specified send-as alias. Revokes any verification that may have been required for using it. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings delete-gmail-4`** - Deletes a client-side encryption identity. The authenticated user can no longer use the identity to send encrypted messages. You cannot restore the identity after you delete it. Instead, use the CreateCseIdentity method to create another identity with the same configuration.
- **`gmail-pp-cli settings delete-gmail-5`** - Deletes the specified S/MIME config for the specified send-as alias.
- **`gmail-pp-cli settings disable`** - Turns off a client-side encryption key pair. The authenticated user can no longer use the key pair to decrypt incoming CSE message texts or sign outgoing CSE mail. To regain access, use the EnableCseKeyPair to turn on the key pair. After 30 days, you can permanently delete the key pair by using the ObliterateCseKeyPair method.
- **`gmail-pp-cli settings enable`** - Turns on a client-side encryption key pair that was turned off. The key pair becomes active again for any associated client-side encryption identities.
- **`gmail-pp-cli settings get`** - Gets the specified delegate. Note that a delegate user must be referred to by their primary email address, and not an email alias. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings get-auto-forwarding`** - Gets the auto-forwarding setting for the specified account.
- **`gmail-pp-cli settings get-gmail`** - Gets a filter.
- **`gmail-pp-cli settings get-gmail-2`** - Gets the specified forwarding address.
- **`gmail-pp-cli settings get-gmail-3`** - Gets the specified send-as alias. Fails with an HTTP 404 error if the specified address is not a member of the collection.
- **`gmail-pp-cli settings get-gmail-4`** - Retrieves a client-side encryption identity configuration.
- **`gmail-pp-cli settings get-gmail-5`** - Retrieves an existing client-side encryption key pair.
- **`gmail-pp-cli settings get-gmail-6`** - Gets the specified S/MIME config for the specified send-as alias.
- **`gmail-pp-cli settings get-imap`** - Gets IMAP settings.
- **`gmail-pp-cli settings get-language`** - Gets language settings.
- **`gmail-pp-cli settings get-pop`** - Gets POP settings.
- **`gmail-pp-cli settings get-vacation`** - Gets vacation responder settings.
- **`gmail-pp-cli settings insert`** - Insert (upload) the given S/MIME config for the specified send-as alias. Note that pkcs12 format is required for the key.
- **`gmail-pp-cli settings list`** - Lists the delegates for the specified account. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings list-gmail`** - Lists the message filters of a Gmail user.
- **`gmail-pp-cli settings list-gmail-2`** - Lists the forwarding addresses for the specified account.
- **`gmail-pp-cli settings list-gmail-3`** - Lists the send-as aliases for the specified account. The result includes the primary send-as address associated with the account as well as any custom "from" aliases.
- **`gmail-pp-cli settings list-gmail-4`** - Lists the client-side encrypted identities for an authenticated user.
- **`gmail-pp-cli settings list-gmail-5`** - Lists client-side encryption key pairs for an authenticated user.
- **`gmail-pp-cli settings list-gmail-6`** - Lists S/MIME configs for the specified send-as alias.
- **`gmail-pp-cli settings obliterate`** - Deletes a client-side encryption key pair permanently and immediately. You can only permanently delete key pairs that have been turned off for more than 30 days. To turn off a key pair, use the DisableCseKeyPair method. Gmail can't restore or decrypt any messages that were encrypted by an obliterated key. Authenticated users and Google Workspace administrators lose access to reading the encrypted messages.
- **`gmail-pp-cli settings patch`** - Patch the specified send-as alias.
- **`gmail-pp-cli settings patch-gmail`** - Associates a different key pair with an existing client-side encryption identity. The updated key pair must validate against Google's [S/MIME certificate profiles](https://support.google.com/a/answer/7300887).
- **`gmail-pp-cli settings set-default`** - Sets the default S/MIME config for the specified send-as alias.
- **`gmail-pp-cli settings update`** - Updates a send-as alias. If a signature is provided, Gmail will sanitize the HTML before saving it with the alias. Addresses other than the primary address for the account can only be updated by service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings update-auto-forwarding`** - Updates the auto-forwarding setting for the specified account. A verified forwarding address must be specified when auto-forwarding is enabled. This method is only available to service account clients that have been delegated domain-wide authority.
- **`gmail-pp-cli settings update-imap`** - Updates IMAP settings.
- **`gmail-pp-cli settings update-language`** - Updates language settings. If successful, the return object contains the `displayLanguage` that was saved for the user, which may differ from the value passed into the request. This is because the requested `displayLanguage` may not be directly supported by Gmail but have a close variant that is, and so the variant may be chosen and saved instead.
- **`gmail-pp-cli settings update-pop`** - Updates POP settings.
- **`gmail-pp-cli settings update-vacation`** - Updates vacation responder settings.
- **`gmail-pp-cli settings verify`** - Sends a verification email to the specified send-as alias address. The verification status must be `pending`. This method is only available to service account clients that have been delegated domain-wide authority.

### stop

Manage stop

- **`gmail-pp-cli stop gmail_users_stop`** - Stop receiving push notifications for the given user mailbox.

### threads

Manage threads

- **`gmail-pp-cli threads delete`** - Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted. This operation cannot be undone. Prefer `threads.trash` instead.
- **`gmail-pp-cli threads get`** - Gets the specified thread.
- **`gmail-pp-cli threads list`** - Lists the threads in the user's mailbox.

### watch

Manage watch

- **`gmail-pp-cli watch gmail_users_watch`** - Set up or update a push notification watch on the given user mailbox.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gmail-pp-cli drafts list mock-value

# JSON for scripting and agents
gmail-pp-cli drafts list mock-value --json

# Filter to specific fields
gmail-pp-cli drafts list mock-value --json --select id,name,status

# Dry run — show the request without sending
gmail-pp-cli drafts list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gmail-pp-cli drafts list mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-gmail -g
```

Then invoke `/pp-gmail <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add gmail gmail-pp-mcp -e GMAIL_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GMAIL_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gmail": {
      "command": "gmail-pp-mcp",
      "env": {
        "GMAIL_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
gmail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gmail-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GMAIL_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gmail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GMAIL_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 401 on all commands** — Run `gmail-pp-cli auth login` — token is missing or expired. Delete `~/.config/gmail-pp-cli/token.json` and re-authenticate if refresh fails.
- **stale command shows history token expired** — Run `gmail-pp-cli sync --full` to re-sync from scratch. History tokens expire after ~7 days of no incremental sync.
- **digest / senders / attachments return no results** — Run `gmail-pp-cli sync --full` first — these commands query the local store, not the live API.
- **HTTP 429 rate limit during sync** — The sync respects Gmail's rate limits with exponential backoff. Large mailboxes (100k+ messages) may take 60+ minutes on first sync. The sync is resumable — rerun if interrupted.

---

## Cookbook

Common recipes using verified flag names.

```bash
# Morning standup digest — unread threads from the last 24h, grouped by label
gmail-pp-cli digest --since 24h --agent --select label,sender,subject,unread_count

# Find your top 10 newsletter senders with one-click unsubscribe URLs
gmail-pp-cli newsletters list --limit 10 --agent

# Download all PDF invoices from accounting in the last 30 days
gmail-pp-cli attachments export --type application/pdf --query "from:accounting" --dir ~/invoices --skip-existing

# Inbox age report — understand the shape of your unread pile before bulk-archive
gmail-pp-cli inbox age --label INBOX --agent

# Compliance audit — how many emails sent to a partner domain this quarter
gmail-pp-cli sent stats --to-domain acmecorp.com --period 90d --agent

# Top 20 senders this month with unsubscribe detection (Friday cleanup)
gmail-pp-cli senders top --limit 20 --period 30d --unsubscribe --agent

# Full-text search across synced messages (offline, instant)
gmail-pp-cli search "invoice Q4 2025" --json --select id,subject,from,date

# Check if local store is fresh before running offline queries
gmail-pp-cli stale --agent

# Sync incremental changes since last sync (fast, uses history token)
gmail-pp-cli sync

# Full re-sync from scratch (use when history token expires)
gmail-pp-cli sync --full

# List all PDF attachments from a specific sender — no API call needed
gmail-pp-cli attachments list --type application/pdf --query "from:vendor@acme.com" --agent

# Pipe digest to jq for custom processing
gmail-pp-cli digest --since 48h --json | jq '.[] | select(.unread_count > 3) | {label, sender, subject}'
```

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**shinzo-labs/gmail-mcp**](https://github.com/shinzo-labs/gmail-mcp) — TypeScript
- [**GongRzhe/Gmail-MCP-Server**](https://github.com/GongRzhe/Gmail-MCP-Server) — TypeScript
- [**googleworkspace/cli**](https://github.com/googleworkspace/cli) — TypeScript
- [**ThomasHabets/cmdg**](https://github.com/ThomasHabets/cmdg) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
