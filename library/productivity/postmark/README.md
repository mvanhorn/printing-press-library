# Postmark CLI

**One account token for every Postmark server, and a local archive that outlasts Postmark's 45-day retention.**

Postmark gives every server its own token and keeps message history for 45 days by default. This CLI takes one account token and works across all your servers. Pick a server by name with `--server`, sync messages and bounces into a local SQLite archive that keeps what Postmark expires, and ask questions that span the account, like which server stopped sending or why one customer never got a password reset. It covers all 87 documented API operations, including message streams, webhooks, suppressions, and bulk email, which Postmark's official specs leave out.

Sending is safe to hand to an agent. Send commands print a preview until you add `--send`, `--sandbox` validates a message with Postmark's test token without delivering it, and `email send-once` won't send the same message twice when a script retries.

Learn more at [Postmark](https://postmarkapp.com).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `postmark-pp-cli` binary and the `pp-postmark` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install postmark
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install postmark --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install postmark --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install postmark --agent claude-code
npx -y @mvanhorn/printing-press-library install postmark --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/postmark/cmd/postmark-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postmark-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install postmark --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-postmark --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-postmark --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install postmark --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/postmark-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `POSTMARK_ACCOUNT_TOKEN` (all servers, pick one with `--server`) and/or `POSTMARK_SERVER_TOKEN` (a single server) when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/postmark/cmd/postmark-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "postmark": {
      "command": "postmark-pp-mcp",
      "env": {
        "POSTMARK_ACCOUNT_TOKEN": "<your-account-token>",
        "POSTMARK_SERVER_TOKEN": "<your-server-token>"
      }
    }
  }
}
```

</details>

## Authentication

Postmark has two kinds of token. A server token (`X-Postmark-Server-Token`) covers one server: sending, messages, bounces, templates, stats, streams, suppressions, and webhooks. The account token (`X-Postmark-Account-Token`) covers servers, domains, sender signatures, template pushes between servers, and data removals. The CLI sends only the header each endpoint accepts.

The simplest setup is the account token alone. Set `POSTMARK_ACCOUNT_TOKEN`, choose a server with `--server <name>` or `POSTMARK_SERVER`, and the CLI looks up that server's token through the account API. `servers use <name>` saves a default server.

If you only use one server, export `POSTMARK_SERVER_TOKEN` instead, or store it with `auth set-token`, which reads the token from stdin. `--server` and `POSTMARK_SERVER` take precedence over `POSTMARK_SERVER_TOKEN`, which takes precedence over the saved default.

`--sandbox` swaps in Postmark's `POSTMARK_API_TEST` token, so sends are validated but never delivered.

## Quick Start

```bash
# Check that your tokens work and the API is reachable.
postmark-pp-cli doctor

# List every server on the account with this week's sends and bounce rate.
postmark-pp-cli overview --agent

# Copy recent messages and bounces into the local archive. --max-pages 5 keeps the first run short.
postmark-pp-cli sync --resources messages,bounces --max-pages 5

# Flag any server whose sending stopped, dropped, or spiked against its own baseline.
postmark-pp-cli pulse --window 7d --agent

# Find out whether one person got their email, and what to run if they didn't.
postmark-pp-cli diagnose jane@example.com --server "Production" --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Account-wide oversight
- **`pulse`** — Compares each server, stream, or tag with its own recent weeks and flags any that stopped sending, dropped off, or spiked in bounces or spam, across every server on the account token.

  _A mailer that breaks quietly stops sending instead of throwing errors, so run this weekly to catch the drop._

  ```bash
  postmark-pp-cli pulse --window 7d --baseline 28d --agent
  ```
- **`recipient-domains`** — Groups sends, bounces by type, complaints, suppressions, and opens by the recipient's mailbox domain across every synced server, so one provider's problem stands out. Run sync first.

  _Start here when several missing-email reports come from the same provider or company._

  ```bash
  postmark-pp-cli recipient-domains --window 7d --min-sent 20 --agent
  ```
- **`servers bootstrap`** — Creates a new project's Postmark server with its message streams, webhooks, sending-domain DNS records, and templates copied from another server. Prints the plan until you add --apply.

  _Hand this to an agent setting up email for a new project. Reruns never create a duplicate server._

  ```bash
  postmark-pp-cli servers bootstrap "Lumen" --broadcast-stream --agent
  ```

### Safe sending
- **`email send-once`** — Sends an email at most once per key within a window (15 minutes by default), so an agent or script that retries after an error cannot deliver a second copy. Previews until you add --send.

  _Built for one-time codes, receipts, and follow-ups that an agent might retry._

  ```bash
  postmark-pp-cli email send-once --key otp-4821 --from "$SENDER" --to "$RECIPIENT" --subject "Your code" --text "Code: 4821" --agent
  ```

### Templates as code
- **`templates check`** — Validates every template and layout on a server or in a pulled folder, and exits non-zero on render errors, missing test-model keys, or broken layout links.

  _Run it in CI before pushing template changes to a server._

  ```bash
  postmark-pp-cli templates check --agent
  ```

## Workflow commands

On top of one command per API endpoint, the CLI adds these workflows. They take the same output flags (`--json`, `--agent`, `--select`, `--csv`) and the global `--server <name>` and `--sandbox` flags, and server tokens are masked in every response.

- `postmark-pp-cli diagnose [email]`: one recipient's recent messages, bounces, and suppressions with a verdict (delivered, sent, bounced, suppressed, spam_complaint, queued, not_found) and the next command to run. `--all-servers` checks every server.
- `postmark-pp-cli overview`: every server in the account with this window's sends, bounces, spam complaints, and stream count.
- `postmark-pp-cli servers tokens [name]`: print the full API token for one server or all servers; the only command that reveals tokens, never cached or stored, and not exposed over MCP.
- `postmark-pp-cli servers use [name]`: save the default server and optional default sender and stream for send commands.
- `postmark-pp-cli templates pull <dir>`: write every template and layout to disk in the official postmark-cli folder layout, with no template cap.
- `postmark-pp-cli templates push <dir>`: diff a template folder against the server; prints the plan unless `--yes` is given, and `--prune --yes` removes templates missing locally only after every local template was pushed.
- `postmark-pp-cli templates render [alias]`: render one template (remote or from a pulled folder) with its layout and test model.
- `postmark-pp-cli suppressions check <email>`: whether an address is suppressed on each message stream, with the command that removes it (SpamComplaint suppressions cannot be removed).
- `postmark-pp-cli bounces reactivate`: plan reactivation of inactive, reactivatable bounces filtered by type, domain, email, window, and `--stream` (default outbound); `--yes` reactivates them.
- `postmark-pp-cli bounces resend-blocked`: plan resending messages that hard-bounced, to the bounced recipient only, on `--stream` (default outbound); `--send` reactivates and resends.
- `postmark-pp-cli webhooks health`: 24-hour delivery statistics and verification status for each webhook, flagging failures.
- `postmark-pp-cli domains health`: DKIM, Return-Path, and sender confirmation status with the fix command for each failure.
- `postmark-pp-cli streams health`: bounce and spam rates per message stream against the 10% and 0.1% thresholds.
- `postmark-pp-cli service-status`: Postmark's public status page, current state and open notices.

The generated `email send`, `send-with-template`, `send-batch`, `send-batch-with-templates`, and `send-bulk` commands print the request they would make unless `--send` is given. `--sandbox` validates a send with Postmark's POSTMARK_API_TEST token and never delivers.

For these workflow commands, `--dry-run` only confirms the command resolves. To see what one would do, run it without `--send`, `--apply`, or `--yes`: `email send-once`, `bounces resend-blocked`, `servers bootstrap`, `templates push`, and `bounces reactivate` print their plan and change nothing until given that flag.

`sync --full` never prunes here, so messages Postmark has expired and rows synced from other servers stay in the local archive. `--no-prune=false` is refused; to start a fresh archive, sync into a new file with `--db <path>`.

Syncing a different server into the same database resets the sync checkpoints and reads that server from the beginning, without pruning. Syncs into one database take turns.

If `email send-once` cannot tell whether Postmark accepted a message (a timeout or a 5xx), it keeps the key reserved for the window and later runs report `delivery_unknown` instead of sending again.

Over MCP, `postmark_execute` returns a preview instead of calling the API for every DELETE and for data removals, stream archives, suppression deletes, and template pushes between servers, unless the call includes `confirm: true`.

## Recipes

### Monday health pass

```bash
postmark-pp-cli pulse --window 7d --baseline 28d --agent
```

Compares each server's last week against its prior four and lists anything that stopped, dropped, or spiked.

### Why didn't they get the reset email

```bash
postmark-pp-cli diagnose jane@example.com --server "Staging" --agent
```

Checks outbound messages, bounces, and suppressions for one address and returns a verdict with the fix command.

### Narrow a big message search

```bash
postmark-pp-cli messages list --recipient jane@example.com --count 50 --offset 0 --agent --select MessageID,Subject,Status,ReceivedAt
```

Keeps only the fields an agent needs from a large outbound search response.

### One-time code without double sends

```bash
postmark-pp-cli email send-once --key otp-4821 --server "Production" --from app@example.com --to jane@example.com --subject "Your code" --text "Code: 4821"
```

Prints the payload and dedupe decision; add --send to deliver, and a retry with the same key returns the original MessageID.

### Gate template changes in CI

```bash
postmark-pp-cli templates check --dir ./templates --server "Production"
```

Validates every template and layout in a pulled folder and exits non-zero on any failure.

### Spin up email for a new project

```bash
postmark-pp-cli servers bootstrap "Lumen" --domain lumen.example.com --broadcast-stream --agent
```

Prints the server, stream, and domain plan with the Return-Path DNS record; the DKIM record appears after --apply creates the domain, and a rerun never duplicates the server.

## Usage

Run `postmark-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml` and `data.db` (the local archive and send-once ledger) |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `POSTMARK_CONFIG_DIR`, `POSTMARK_DATA_DIR`, `POSTMARK_STATE_DIR`, or `POSTMARK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `POSTMARK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export POSTMARK_HOME=/srv/postmark
postmark-pp-cli doctor
```

Under `POSTMARK_HOME=/srv/postmark`, the four dirs resolve to `/srv/postmark/config`, `/srv/postmark/data`, `/srv/postmark/state`, and `/srv/postmark/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "postmark": {
      "command": "postmark-pp-mcp",
      "env": {
        "POSTMARK_HOME": "/srv/postmark"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `POSTMARK_DATA_DIR` overrides an explicit `--home` for that kind. Use `POSTMARK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `POSTMARK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `postmark-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### bounces

Manage bounces

- **`postmark-pp-cli bounces activate`** - Activate a bounce
- **`postmark-pp-cli bounces delivery-stats`** - Get delivery stats
- **`postmark-pp-cli bounces dump`** - Get bounce dump
- **`postmark-pp-cli bounces get`** - Get a single bounce
- **`postmark-pp-cli bounces list`** - Get bounces

### clicks

Manage clicks

- **`postmark-pp-cli clicks get`** - Retrieve Message Clicks
- **`postmark-pp-cli clicks list`** - Clicks for a all messages

### data_removals

Manage data removals

- **`postmark-pp-cli data-removals create`** - Request irreversible removal of a recipient's data (enabled on request by Postmark)
- **`postmark-pp-cli data-removals get`** - Check a data removal request

### domains

Manage domains

- **`postmark-pp-cli domains create`** - Create a Domain
- **`postmark-pp-cli domains delete`** - Delete a Domain
- **`postmark-pp-cli domains get`** - Get a Domain
- **`postmark-pp-cli domains list`** - List Domains
- **`postmark-pp-cli domains rotate-dkim`** - Creates a new DKIM key to replace your current key. Until the DNS entries are confirmed,
the new values will be in the `DKIMPendingHost` and `DKIMPendingTextValue` fields.
After the new DKIM value is verified in DNS, the pending values will migrate to
`DKIMTextValue` and `DKIMPendingTextValue` and Postmark will begin to sign emails
with the new DKIM key.
- **`postmark-pp-cli domains update`** - Update a Domain
- **`postmark-pp-cli domains verify-dkim`** - Request DNS Verification for DKIM
- **`postmark-pp-cli domains verify-return-path`** - Request DNS Verification for Return-Path
- **`postmark-pp-cli domains verify-spf`** - Request DNS Verification for SPF (deprecated by Postmark)

### email

Manage email

- **`postmark-pp-cli email bulk-list`** - List bulk requests on this server, newest first
- **`postmark-pp-cli email bulk-status`** - Get the status of one bulk request
- **`postmark-pp-cli email send`** - Send a single email
- **`postmark-pp-cli email send-batch`** - Send a batch of emails
- **`postmark-pp-cli email send-batch-with-templates`** - Send a batch of email using templates.
- **`postmark-pp-cli email send-bulk`** - Send one message to many recipients (bulk). Requires account approval.
- **`postmark-pp-cli email send-with-template`** - Send an email using a Template

### inbound

Manage inbound

- **`postmark-pp-cli inbound bypass`** - Bypass rules for a blocked inbound message
- **`postmark-pp-cli inbound get`** - Inbound message details
- **`postmark-pp-cli inbound list`** - Inbound message search
- **`postmark-pp-cli inbound retry`** - Retry a failed inbound message for processing

### inbound_rules

Manage inbound rules

- **`postmark-pp-cli inbound-rules create`** - Create an inbound rule trigger
- **`postmark-pp-cli inbound-rules delete`** - Delete a single trigger
- **`postmark-pp-cli inbound-rules list`** - List inbound rule triggers

### messages

Manage messages

- **`postmark-pp-cli messages dump`** - Outbound message dump
- **`postmark-pp-cli messages get`** - Outbound message details
- **`postmark-pp-cli messages list`** - Outbound message search

### opens

Manage opens

- **`postmark-pp-cli opens get`** - Retrieve Message Opens
- **`postmark-pp-cli opens list`** - Opens for all messages

### senders

Manage senders

- **`postmark-pp-cli senders create`** - Create a Sender Signature
- **`postmark-pp-cli senders delete`** - Delete a Sender Signature
- **`postmark-pp-cli senders get`** - Get a Sender Signature
- **`postmark-pp-cli senders list`** - List Sender Signatures
- **`postmark-pp-cli senders request-new-dkim`** - Requests a new DKIM key to be created. Until the DNS entries are confirmed,
the new values will be in the `DKIMPendingHost` and `DKIMPendingTextValue` fields.
After the new DKIM value is verified in DNS, the pending values will migrate to
`DKIMTextValue` and `DKIMPendingTextValue` and Postmark will begin to sign emails
with the new DKIM key.
- **`postmark-pp-cli senders resend-confirmation`** - Resend Signature Confirmation Email
- **`postmark-pp-cli senders update`** - Update a Sender Signature
- **`postmark-pp-cli senders verify-spf`** - Request DNS Verification for SPF (deprecated by Postmark)

### server

Manage server

- **`postmark-pp-cli server get`** - Get Server Configuration
- **`postmark-pp-cli server update`** - Edit Server Configuration

### servers

Manage servers

- **`postmark-pp-cli servers create`** - Create a Server
- **`postmark-pp-cli servers delete`** - Delete a Server
- **`postmark-pp-cli servers get`** - Get a Server
- **`postmark-pp-cli servers list`** - List servers
- **`postmark-pp-cli servers update`** - Edit a Server

### stats

Manage stats

- **`postmark-pp-cli stats bounces`** - Get bounce counts
- **`postmark-pp-cli stats click-browsers`** - Get browser usage by family
- **`postmark-pp-cli stats click-locations`** - Get clicks by body location
- **`postmark-pp-cli stats click-platforms`** - Get browser plaform usage
- **`postmark-pp-cli stats clicks`** - Get click counts
- **`postmark-pp-cli stats open-clients`** - Get email client usage
- **`postmark-pp-cli stats open-platforms`** - Get email platform usage
- **`postmark-pp-cli stats open-read-times`** - Open counts by read time (seconds spent reading)
- **`postmark-pp-cli stats opens`** - Get email open counts
- **`postmark-pp-cli stats overview`** - Get outbound overview
- **`postmark-pp-cli stats sends`** - Get sent counts
- **`postmark-pp-cli stats spam`** - Get spam complaints
- **`postmark-pp-cli stats tracked`** - Get tracked email counts

### streams

Manage streams

- **`postmark-pp-cli streams archive`** - Archive a message stream (purged 45 days later unless unarchived)
- **`postmark-pp-cli streams create`** - Create a message stream
- **`postmark-pp-cli streams get`** - Get one message stream
- **`postmark-pp-cli streams list`** - List message streams on this server
- **`postmark-pp-cli streams unarchive`** - Restore an archived message stream before its purge date
- **`postmark-pp-cli streams update`** - Edit a message stream

### suppressions

Manage suppressions

- **`postmark-pp-cli suppressions create`** - Suppress up to 50 addresses on a message stream
- **`postmark-pp-cli suppressions delete`** - Remove up to 50 suppressions from a message stream (SpamComplaint suppressions cannot be removed)
- **`postmark-pp-cli suppressions list`** - List suppressed addresses on a message stream

### templates

Manage templates

- **`postmark-pp-cli templates create`** - Create a Template
- **`postmark-pp-cli templates delete`** - Delete a Template
- **`postmark-pp-cli templates get`** - Get a Template
- **`postmark-pp-cli templates list`** - Get the Templates associated with this Server
- **`postmark-pp-cli templates push-between-servers`** - Push templates from one server to another
- **`postmark-pp-cli templates update`** - Update a Template
- **`postmark-pp-cli templates validate`** - Test Template Content

### webhooks

Manage webhooks

- **`postmark-pp-cli webhooks create`** - Create a webhook
- **`postmark-pp-cli webhooks delete`** - Delete a webhook
- **`postmark-pp-cli webhooks get`** - Get one webhook
- **`postmark-pp-cli webhooks list`** - List webhooks on this server
- **`postmark-pp-cli webhooks statistics`** - Delivery statistics for one webhook over the last 24 hours
- **`postmark-pp-cli webhooks update`** - Edit a webhook (omitted triggers stay unchanged)
- **`postmark-pp-cli webhooks verify`** - Send a test request for each enabled trigger and record verified or unverified


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`postmark-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`postmark-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`postmark-pp-cli learnings list`** - Inspect taught rows
- **`postmark-pp-cli learnings forget <query>`** - Undo a teach
- **`postmark-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`postmark-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`postmark-pp-cli teach-pattern`** - Install a query/resource template up front
- **`postmark-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `POSTMARK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `postmark-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
postmark-pp-cli bounces list --count 100 --offset 0

# JSON for scripting and agents
postmark-pp-cli bounces list --count 100 --offset 0 --json
# Filter to specific fields
postmark-pp-cli bounces list --count 100 --offset 0 --json --select BouncedAt,CanActivate,Content

# Dry run — show the request without sending
postmark-pp-cli bounces list --count 100 --offset 0 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
postmark-pp-cli bounces list --count 100 --offset 0 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
postmark-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `postmark-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/postmark-pp-cli/config.toml`; `--home`, `POSTMARK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POSTMARK_SERVER_TOKEN` | per_call | No | Server API token for one Postmark server (sending, messages, bounces, templates, stats, streams, webhooks). |
| `POSTMARK_ACCOUNT_TOKEN` | per_call | No | Account API token: servers, domains, senders, template push, data removals; also lets `--server <name>` look up server tokens. |
| `POSTMARK_SERVER` | config | No | Server name or ID to act on, same as `--server`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `postmark-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `postmark-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POSTMARK_SERVER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 401 with ErrorCode 10** — The wrong token type reached the endpoint: set POSTMARK_ACCOUNT_TOKEN for servers, domains, and senders commands, and POSTMARK_SERVER_TOKEN or --server <name> for everything else.
- **Send fails with ErrorCode 406 (inactive recipient)** — Run postmark-pp-cli diagnose <email> to see the bounce or suppression, then reactivate it with the command diagnose prints.
- **HTTP 429 during sync** — Reads back off automatically; rerun sync with a smaller --max-pages or fewer --resources.
- **Bulk send returns ErrorCode 14** — Bulk sending needs approval on your Postmark account; ask Postmark support to enable it.
- **Message search stops at 10,000 results** — Postmark caps count plus offset at 10,000; window the pull by date, e.g. postmark-pp-cli sync --resources messages --param fromdate=2026-09-01 --param todate=2026-09-15, then query the archive with search or sql.
- **sync --full --no-prune=false exits with a usage error** — Pruning is off because the archive keeps every synced server's rows and messages Postmark has expired. To start a fresh archive, sync into a new file with --db <path>; the current archive and send ledger stay as they are.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**postmark.js**](https://github.com/ActiveCampaign/postmark.js) — TypeScript (361 stars)
- [**postmarker**](https://github.com/Stranger6667/postmarker) — Python (149 stars)
- [**postmark-cli**](https://github.com/ActiveCampaign/postmark-cli) — TypeScript (88 stars)
- [**postmark-mcp**](https://github.com/ActiveCampaign/postmark-mcp) — JavaScript (57 stars)
- [**Postmark-Bounce-Removal**](https://github.com/arnaud-coral/Postmark-Bounce-Removal) — Shell (1 stars)
- [**pmx**](https://github.com/nicolasacchi/pmx) — Go
- [**agent-postmark**](https://github.com/shhac/agent-postmark) — Go
- [**retry-hard-bounces-postmark**](https://github.com/dszp/retry-hard-bounces-postmark) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
