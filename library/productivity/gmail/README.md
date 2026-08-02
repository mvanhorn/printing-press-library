# Gmail CLI

**Every Gmail API surface plus the things Gmail itself will not give you: scheduled sends, sender analytics, follow-up tracking, and an offline full-text mailbox.**

gmail-pp-cli mirrors your mailbox into local SQLite with gmail-pp-cli pull (incremental history sync), so search, sender leaderboards, storage analytics, and follow-up tracking run instantly and cost zero quota. It also fills the API's biggest gap with a local scheduled-send queue, and ships agent-native output on every command.

Learn more at [the Gmail API documentation](https://developers.google.com/gmail/api).

Created by [@rahulbansal16](https://github.com/rahulbansal16) (Rahul Bansal).

## Install

The recommended path installs both the `gmail-pp-cli` binary and the `pp-gmail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gmail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gmail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gmail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gmail --agent claude-code
npx -y @mvanhorn/printing-press-library install gmail --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gmail --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gmail --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gmail --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gmail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GMAIL_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/gmail/cmd/gmail-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gmail": {
      "command": "gmail-pp-mcp",
      "env": {
        "GMAIL_USER_ID": "<userId>",
        "GMAIL_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Gmail uses OAuth2. Run `gmail-pp-cli auth login` once: it opens a browser consent flow using OAuth client credentials supplied via --client-id/--client-secret, the GMAIL_CLIENT_ID / GMAIL_CLIENT_SECRET environment variables, or an interactive prompt. Tokens are stored locally and refresh automatically. For headless use, set GMAIL_ACCESS_TOKEN to a pre-minted token.

## Quick Start

```bash
# Verify the binary, config, and auth wiring before touching the API
gmail-pp-cli doctor --dry-run

# Live Gmail search with the same operators as the Gmail search box
gmail-pp-cli find "is:unread newer_than:2d"

# Mirror recent mail (headers plus decoded bodies) into local SQLite so analytics and offline search work
gmail-pp-cli pull --since 30d

# Unread summary grouped by sender and category
gmail-pp-cli triage

# Who fills your mailbox: counts, unread, and size per sender from the local store
gmail-pp-cli senders --limit 10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### What the API itself lacks
- **`schedule send`** — Write an email now and have it sent at any future time, even though the Gmail API itself has no schedule-send. Delivery requires `schedule run` from cron/launchd (or `schedule run --watch`).

  _Reach for this whenever a user asks to send an email later, at a specific time, or in another timezone; no other Gmail API tool can do it. Tell the user to wire `gmail-pp-cli schedule run` into cron, or the queue will sit undelivered._

  ```bash
  gmail-pp-cli schedule send --to prospect@example.com --subject "Following up" --body-file pitch.txt --at "2026-08-04T09:00:00"
  ```
- **`filters diff`** — Keep Gmail filters in a version-controlled YAML file; diff against live settings and apply the reconciliation plan.

  _Use when the user manages filters across machines or wants filter changes reviewed before they touch the account._

  ```bash
  gmail-pp-cli filters diff --file filters.yaml
  ```

### Local mailbox intelligence
- **`senders`** — See who actually fills your mailbox: per-sender message count, unread count, total size, and first/last seen.

  _Use this before any cleanup or unsubscribe pass to rank which senders matter; the live API cannot aggregate by sender at all. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli senders --limit 20 --agent
  ```
- **`storage`** — Rank senders and years by mailbox bytes consumed, with the exact cleanup command printed next to each group.

  _Use when the user is near their storage quota or asks what is taking up space in Gmail. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli storage --group-by sender --agent
  ```
- **`followups`** — List threads where the last word was theirs (you owe a reply) or yours (they never replied), aged past N days.

  _Use for outreach chasing (who never replied) and inbox accountability (what do I owe); no Gmail search operator can express either. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli followups --direction out --days 3 --agent
  ```
- **`unsub`** — Rank mailing lists by volume and unread ratio, and emit each list's unsubscribe target ready to act on.

  _Use when the user wants to cut inbox noise; the unread ratio identifies lists they never actually read. Requires a populated local mirror: run `gmail-pp-cli pull` first._

  ```bash
  gmail-pp-cli unsub --min-count 10 --agent
  ```

## Recipes

### Morning triage

```bash
gmail-pp-cli triage --agent
```

Unread mail grouped by sender and category in compact agent-shaped JSON, ready for a summarize-and-archive loop.

### Narrow a live list for an agent

```bash
gmail-pp-cli messages list me --q "has:attachment newer_than:7d" --agent --select messages.id,messages.threadId
```

Typed endpoint call with dotted-path field selection so the agent gets ids without the verbose envelope.

### Schedule tomorrow's follow-up

```bash
gmail-pp-cli schedule send --to alex@example.com --subject "Re: proposal" --body-file followup.txt --in 18h
```

Queues the send locally; a cron-invoked schedule run fires it at the due time, exactly once.

### Chase silent prospects

```bash
gmail-pp-cli followups --direction out --days 3 --agent
```

Sent threads where nobody replied in 3 days, from the local store at zero quota cost.

### Cut inbox noise

```bash
gmail-pp-cli unsub --min-count 10 --agent
```

Mailing lists ranked by volume and never-read ratio with their unsubscribe targets.

## Usage

Run `gmail-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GMAIL_CONFIG_DIR`, `GMAIL_DATA_DIR`, `GMAIL_STATE_DIR`, or `GMAIL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GMAIL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GMAIL_HOME=/srv/gmail
gmail-pp-cli doctor
```

Under `GMAIL_HOME=/srv/gmail`, the four dirs resolve to `/srv/gmail/config`, `/srv/gmail/data`, `/srv/gmail/state`, and `/srv/gmail/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "gmail": {
      "command": "gmail-pp-mcp",
      "env": {
        "GMAIL_HOME": "/srv/gmail"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GMAIL_DATA_DIR` overrides an explicit `--home` for that kind. Use `GMAIL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GMAIL_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `gmail-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### drafts

Manage drafts

- **`gmail-pp-cli drafts create`** - Creates a new draft with the `DRAFT` label.
- **`gmail-pp-cli drafts delete`** - Immediately and permanently deletes the specified draft. Does not simply trash it.
- **`gmail-pp-cli drafts get`** - Gets the specified draft.
- **`gmail-pp-cli drafts list`** - Lists the drafts in the user's mailbox.
- **`gmail-pp-cli drafts send`** - Sends the specified, existing draft to the recipients in the `To`, `Cc`, and `Bcc` headers.
- **`gmail-pp-cli drafts update`** - Replaces a draft's content.

### history

Manage history

- **`gmail-pp-cli history <userId>`** - Lists the history of all changes to the given mailbox. History results are returned in chronological order (increasing `historyId`).

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
- **`gmail-pp-cli messages modify`** - Modifies the labels on the specified message.
- **`gmail-pp-cli messages trash`** - Moves the specified message to the trash.
- **`gmail-pp-cli messages untrash`** - Removes the specified message from the trash.
- **`gmail-pp-cli messages attachments get`** - Gets the specified message attachment.
- **`gmail-pp-cli messages import`** - Imports a message into only this user's mailbox, with standard email delivery scanning and classification similar to receiving via SMTP. This method doesn't perform SPF checks, so it might not work for some spam messages, such as those attempting to perform domain spoofing. This method does not send a message. Note: This function doesn't trigger forwarding rules or filters set up by the user.
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

- **`gmail-pp-cli stop <userId>`** - Stop receiving push notifications for the given user mailbox.

### threads

Manage threads

- **`gmail-pp-cli threads delete`** - Immediately and permanently deletes the specified thread. Any messages that belong to the thread are also deleted. This operation cannot be undone. Prefer `threads.trash` instead.
- **`gmail-pp-cli threads get`** - Gets the specified thread.
- **`gmail-pp-cli threads modify`** - Modifies the labels applied to the thread.
- **`gmail-pp-cli threads trash`** - Moves the specified thread to the trash.
- **`gmail-pp-cli threads untrash`** - Removes the specified thread from the trash.
- **`gmail-pp-cli threads list`** - Lists the threads in the user's mailbox.

### users_profile

Manage users profile

- **`gmail-pp-cli users-profile <userId>`** - Gets the current user's Gmail profile.

### watch

Manage watch

- **`gmail-pp-cli watch <userId>`** - Set up or update a push notification watch on the given user mailbox.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`gmail-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`gmail-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`gmail-pp-cli learnings list`** - Inspect taught rows
- **`gmail-pp-cli learnings forget <query>`** - Undo a teach
- **`gmail-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`gmail-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`gmail-pp-cli teach-pattern`** - Install a query/resource template up front
- **`gmail-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GMAIL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `gmail-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gmail-pp-cli drafts list me

# JSON for scripting and agents
gmail-pp-cli drafts list me --json

# Filter to specific fields
gmail-pp-cli drafts list me --json --select drafts.id,drafts.message.threadId

# Dry run — show the request without sending
gmail-pp-cli drafts list me --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gmail-pp-cli drafts list me --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - search and analytics commands read the local SQLite store populated by `gmail-pp-cli pull`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `GMAIL_USER_ID` resolves `{userId}`

Base URL: `https://gmail.googleapis.com`

## Health Check

```bash
gmail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `gmail-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/gmail-pp-cli/config.toml`; `--home`, `GMAIL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GMAIL_USER_ID` | endpoint | No | Mailbox that `{userId}` resolves to. Defaults to `me`, the authenticated user. |
| `GMAIL_ACCESS_TOKEN` | per_call | No | Pre-minted OAuth access token for headless use. Not needed after `auth login`, which stores and refreshes tokens automatically. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `gmail-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gmail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GMAIL_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 accessNotConfigured or API disabled errors** — Enable the Gmail API for your GCP project: https://console.cloud.google.com/apis/library/gmail.googleapis.com then retry
- **invalid_grant or token expired after 7 days** — Your OAuth consent screen is in Testing mode; publish the app or re-run gmail-pp-cli auth login to re-consent
- **429 rateLimitExceeded during pull** — Re-run gmail-pp-cli pull with a narrower window such as --since 7d; the client backs off automatically and the saved history cursor resumes incrementally
- **search or senders returns nothing but find works** — Local analytics need a populated mirror; run gmail-pp-cli pull --since 30d first
- **scheduled email did not send** — The queue only fires while gmail-pp-cli schedule run is invoked (cron) or running with --watch; check gmail-pp-cli schedule list for pending items

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**googleworkspace/cli (gws)**](https://github.com/googleworkspace/cli) — Rust (30100 stars)
- [**Got Your Back (GYB)**](https://github.com/GAM-team/got-your-back) — Python (3100 stars)
- [**gmailctl**](https://github.com/mbrt/gmailctl) — Go (2200 stars)
- [**Gmail-MCP-Server**](https://github.com/GongRzhe/Gmail-MCP-Server) — TypeScript (1167 stars)
- [**lieer**](https://github.com/gauteh/lieer) — Python (648 stars)
- [**simplegmail**](https://github.com/jeremyephron/simplegmail) — Python (407 stars)
- [**ezgmail**](https://github.com/asweigart/ezgmail) — Python (272 stars)
- [**shinzo-labs/gmail-mcp**](https://github.com/shinzo-labs/gmail-mcp) — TypeScript (56 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
