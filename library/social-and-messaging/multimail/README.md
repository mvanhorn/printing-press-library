# MultiMail CLI

**Every MultiMail feature plus oversight velocity, trust ladder status, and cross-mailbox search no other tool has.**

The only CLI that makes MultiMail's trust ladder a first-class operator surface. Sync mailboxes, emails, and audit events to local SQLite, then query oversight velocity, allowlist coverage, and inbox health across your entire fleet — insights no single API call provides.

Created by [@H179922](https://github.com/H179922) (H179922).

## Install

The recommended path installs both the `multimail-pp-cli` binary and the `pp-multimail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install multimail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install multimail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install multimail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install multimail --agent claude-code
npx -y @mvanhorn/printing-press-library install multimail --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/cmd/multimail-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/multimail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install multimail --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install multimail --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/multimail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MULTIMAIL_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/cmd/multimail-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "multimail": {
      "command": "multimail-pp-mcp",
      "env": {
        "MULTIMAIL_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

MultiMail uses Bearer token auth with prefixed keys: mm_live_* for production, mm_test_* for staging. Get your key from the MultiMail dashboard or via the API key management endpoints.

## Quick Start

```bash
# Verify installation and connectivity
multimail-pp-cli doctor --dry-run

# Sync recent mailbox and email data to local store
multimail-pp-cli sync --resources mailboxes,emails --since 7d

# Check what emails are waiting for approval
multimail-pp-cli oversight list --json

# Search synced emails across all mailboxes
multimail-pp-cli search 'invoice' --type emails --limit 10 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`fleet health`** — Single-command account-wide health snapshot: mailbox count, oversight queue depth, webhook delivery rate, domain verification status, usage vs plan limits.

  _When an agent needs to know if its MultiMail integration is healthy before sending, one command replaces checking 5 separate endpoints._

  ```bash
  multimail-pp-cli fleet health --json --agent --select mailboxes.total,oversight.pending_count,webhooks.success_rate
  ```
- **`oversight bulk-decide`** — Approve or reject all pending emails matching a filter (by mailbox, sender, or age) in one command.

  _When an agent's test queue has 20 pending emails, one command clears them instead of 20 individual approve calls._

  ```bash
  multimail-pp-cli oversight bulk-decide --approve --mailbox support-agent --json
  ```
- **`oversight velocity`** — See approval/rejection rates and median decision latency per mailbox across your entire fleet.

  _When an agent's sends are stuck in approval queues, this pinpoints which mailbox's operator is the bottleneck._

  ```bash
  multimail-pp-cli oversight velocity --json --days 7
  ```
- **`trust status`** — Fleet-wide view of each mailbox's oversight mode, time-at-level, and upgrade eligibility.

  _Before requesting a trust upgrade, an agent should know which mailboxes are ready and which have been at their current level longest._

  ```bash
  multimail-pp-cli trust status --json --agent --select name,oversight_mode,time_at_level
  ```
- **`trust timeline`** — Per-mailbox chronological history of every oversight mode change with timestamps and who triggered it.

  _When evaluating whether to request a trust upgrade, an agent needs to show its history of responsible operation at each level._

  ```bash
  multimail-pp-cli trust timeline --mailbox support-agent --json
  ```
- **`emails send-and-wait`** — Send an email and block until a reply arrives or timeout — the agent test loop in one command.

  _When an agent needs to send and process the reply in one workflow step, this replaces a manual send-poll-read loop._

  ```bash
  multimail-pp-cli emails send-and-wait --mailbox test-agent --to user@example.com --subject 'Test' --body 'Hello' --timeout 5m --json
  ```

### Local state that compounds
- **`mailboxes allowlist coverage`** — See what percentage of recent recipients are covered by allowlist patterns vs gated.

  _Before adding allowlist entries, an agent should know which recipients are already covered and which cause the most gating friction._

  ```bash
  multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json --agent --select pattern,match_count,percentage
  ```
- **`inbox health`** — Per-mailbox health snapshot: unread count, oldest unread age, reply rate, and thread depth.

  _An agent monitoring its own inbox health can detect when it is falling behind on replies before the operator notices._

  ```bash
  multimail-pp-cli inbox health --json --agent --select mailbox,unread_count,oldest_unread_age,reply_rate
  ```
- **`threads stale`** — List conversation threads with no reply in N days — surfaces dropped conversations.

  _A dropped conversation thread is a customer-facing failure; this is the agent's early warning system._

  ```bash
  multimail-pp-cli threads stale --days 3 --json --agent --select thread_id,subject,last_reply_age,mailbox
  ```
- **`audit compliance`** — Cross-entity compliance report: oversight bypass count, approval/rejection counts, decision latency percentiles per mailbox.

  _When an auditor needs to verify that agents operated within their approved trust levels, this replaces manual log correlation._

  ```bash
  multimail-pp-cli audit compliance --days 30 --json
  ```
- **`webhooks health`** — Per-webhook success rate, failure count, last delivery timestamp, and consecutive failure streak.

  _When an agent's real-time event pipeline depends on webhooks, this surfaces delivery health before silent failures accumulate._

  ```bash
  multimail-pp-cli webhooks health --json --agent --select webhook_id,url,success_rate,consecutive_failures
  ```

## Recipes

### Approve all pending emails for a mailbox

```bash
multimail-pp-cli oversight pending --json --select id,subject,to | multimail-pp-cli oversight decide --approve --stdin
```

Pipe pending email IDs into batch approval — useful for clearing a gated mailbox queue.

### Fleet trust ladder overview

```bash
multimail-pp-cli trust status --json --agent --select name,oversight_mode,time_at_level
```

See which mailboxes are at which trust level and how long they have been there.

### Cross-mailbox search with field selection

```bash
multimail-pp-cli search 'quarterly report' --type emails --json --agent --select subject,from,mailbox,received_at
```

Find a specific email across all synced mailboxes with narrow field output for agent context efficiency.

### Sync and check inbox health

```bash
multimail-pp-cli sync --resources mailboxes,emails --since 24h && multimail-pp-cli inbox health --json
```

Refresh local data then check unread counts, reply rates, and oldest unread age per mailbox.

### Allowlist coverage analysis

```bash
multimail-pp-cli mailboxes allowlist coverage --mailbox primary --days 30 --json --agent --select pattern,match_count,percentage
```

See what percentage of recent recipients match allowlist patterns — find which addresses cause the most gating friction.

## Usage

Run `multimail-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage account

- **`multimail-pp-cli account create`** - Requires a solved proof-of-work challenge. Creates a pending signup and sends a confirmation email. Response is always identical for privacy (anti-enumeration). Honors an optional Idempotency-Key request header (UUID) to safely retry without creating duplicate pending_signups rows.
- **`multimail-pp-cli account create-challenge`** - Returns an ALTCHA challenge. Solve it and include the solution as pow_solution in POST /v1/account. Challenge expires in 5 minutes.
- **`multimail-pp-cli account create-resendconfirmation`** - Public endpoint (no auth required). Resends the activation email with a new code for unconfirmed accounts. Rate limited to 1 request per 10 minutes.
- **`multimail-pp-cli account delete`** - Hard-deletes all tenant data (mailboxes, emails, API keys, usage, audit log). Frees the slug for re-registration. Requires admin scope.
- **`multimail-pp-cli account list`** - Get current tenant info and usage
- **`multimail-pp-cli account update`** - Update tenant settings

### admin

Manage admin

- **`multimail-pp-cli admin`** - Admin-only. Creates a new API key and emails it to the tenant's oversight email. Used when welcome email failed or KV expired before key retrieval.

### agent

Manage agent

- **`multimail-pp-cli agent create`** - Initiates agent registration using verified_email identity assertion. Sends a 6-digit OTP to the provided email and returns a claim_token for completing the registration.
- **`multimail-pp-cli agent create-auth`** - Completes the auth.md registration by validating the claim_token and OTP. On success, atomically creates the tenant account and returns API credentials.
- **`multimail-pp-cli agent list`** - Human-facing page that displays the 6-digit OTP for agent registration. Linked from the verification email.

### api-keys

Manage api keys

- **`multimail-pp-cli api-keys create`** - Requires admin scope. The raw key is returned only once in the response.
- **`multimail-pp-cli api-keys delete`** - Requires admin scope. Returns 202 with pending_approval on first call; resend with approval_code to complete.
- **`multimail-pp-cli api-keys list`** - Requires admin scope. Returns key prefix, scopes, and metadata.
- **`multimail-pp-cli api-keys update`** - Update API key name or scopes

### approve

Manage approve

- **`multimail-pp-cli approve create`** - Process approval/rejection from hosted page
- **`multimail-pp-cli approve get`** - Render hosted approval page for oversight decisions

### audit-log

Manage audit log

- **`multimail-pp-cli audit-log`** - Returns audit log entries with cursor pagination. Requires admin scope.

### auth-md

Manage auth md

- **`multimail-pp-cli auth-md`** - Returns a markdown document describing MultiMail's agent registration flow, trust ladder, and scope model. Used by agents following the auth.md protocol.

### billing

Manage billing

- **`multimail-pp-cli billing create`** - Requires admin scope. Sets cancel_at_period_end on the Stripe subscription so the tenant retains access until the current billing period ends.
- **`multimail-pp-cli billing create-checkout`** - Create a Stripe checkout session for plan upgrade
- **`multimail-pp-cli billing create-coinbasewebhook`** - Coinbase Commerce webhook handler (public, signature-verified)
- **`multimail-pp-cli billing create-cryptocheckout`** - Create a Coinbase Commerce checkout (crypto payment)
- **`multimail-pp-cli billing create-portal`** - Requires admin scope. Returns a URL to the Stripe-hosted billing portal for self-service invoice, payment method, and plan management.
- **`multimail-pp-cli billing create-pricingcheckout`** - Creates an inactive tenant, provisions a default mailbox, and returns a Stripe checkout URL. After payment, call GET /v1/billing/session-key to retrieve the API key. Honors an optional Idempotency-Key request header (UUID); the same key is forwarded to Stripe so duplicate Sessions are not created on retry.
- **`multimail-pp-cli billing create-stripewebhook`** - Stripe webhook handler (public, signature-verified)
- **`multimail-pp-cli billing list`** - Public endpoint. Returns the API key stored during pricing-checkout, then deletes it. Key expires after 1 hour if not retrieved.

### confirm

Manage confirm

- **`multimail-pp-cli confirm create`** - JSON response includes: status, name, oversight_mode, api_key, mailbox_id, mailbox_address, oversight_email, use_case. Browser form submissions redirect to /welcome.
- **`multimail-pp-cli confirm get`** - Redirect to frontend confirmation page with code prefilled
- **`multimail-pp-cli confirm list`** - Redirect to frontend confirmation page at multimail.dev/confirm

### contacts

Manage contacts

- **`multimail-pp-cli contacts create`** - Add a contact to the address book. Requires send scope.
- **`multimail-pp-cli contacts delete`** - Requires admin scope.
- **`multimail-pp-cli contacts list`** - Search address book by name or email. Omit query to list all. Requires read scope.

### data_export

Manage data export

- **`multimail-pp-cli data-export`** - Requires admin scope. Rate limited to 1 request per hour.

### domains

Manage domains

- **`multimail-pp-cli domains create`** - Add a custom domain (Pro/Scale only)
- **`multimail-pp-cli domains delete`** - Delete a custom domain
- **`multimail-pp-cli domains get`** - Get custom domain detail
- **`multimail-pp-cli domains list`** - Requires admin scope.

### emails

Manage emails

- **`multimail-pp-cli emails`** - Requires read scope. Without a status filter, returns spam_flagged and spam_quarantined emails across all tenant mailboxes.

### funnel

Manage funnel

- **`multimail-pp-cli funnel`** - Pricing page beacon hit via navigator.sendBeacon to track open/submit/error events on the signup modal. Fire-and-forget; counters are best-effort (KV is non-atomic). IP-rate-limited to 30 req/min.

### health

Manage health

- **`multimail-pp-cli health`** - Verifies D1 and R2 connectivity. No auth required.

### mailboxes

Manage mailboxes

- **`multimail-pp-cli mailboxes create`** - Requires admin scope. Address can be a local part (appended to tenant subdomain) or full address on a verified custom domain.
- **`multimail-pp-cli mailboxes delete`** - Requires admin scope.
- **`multimail-pp-cli mailboxes list`** - Requires read scope.
- **`multimail-pp-cli mailboxes update`** - Requires admin scope. Oversight mode can only be downgraded here; upgrades require the upgrade flow.

### operator

Manage operator

- **`multimail-pp-cli operator create`** - Requires admin scope. Clears the operator-session cookie.
- **`multimail-pp-cli operator create-startsession`** - Requires admin scope. Sends a one-time code to the oversight email and begins the operator-session OTP flow.
- **`multimail-pp-cli operator create-verifysession`** - Requires admin scope. Exchanges a one-time code for a short-lived HttpOnly operator-session cookie.
- **`multimail-pp-cli operator list`** - Requires admin scope. Reports whether the current browser has an active operator-session cookie.

### oversight

Manage oversight

- **`multimail-pp-cli oversight create`** - Requires oversight scope. Approved outbound emails are sent immediately.
- **`multimail-pp-cli oversight list`** - List emails pending oversight approval

### slug-check

Manage slug check

- **`multimail-pp-cli slug-check <slug>`** - Check if a slug is available for registration. Returns suggestions if taken or reserved. No auth required.

### support

Manage support

- **`multimail-pp-cli support`** - Public endpoint. Requires a solved ALTCHA proof-of-work payload. Sends a message to support@multimail.dev.

### suppression

Manage suppression

- **`multimail-pp-cli suppression delete`** - Allows future emails to be sent to this address again. Requires admin scope.
- **`multimail-pp-cli suppression list`** - Returns addresses suppressed due to bounces, spam complaints, or manual unsubscribes. Requires admin scope.

### unsubscribe

Manage unsubscribe

- **`multimail-pp-cli unsubscribe create`** - Process unsubscribe request
- **`multimail-pp-cli unsubscribe get`** - Render unsubscribe page (CAN-SPAM)

### usage

Manage usage

- **`multimail-pp-cli usage`** - Requires read scope. Returns usage counts for the current billing period.

### webhook-deliveries

Manage webhook deliveries

- **`multimail-pp-cli webhook-deliveries`** - Returns recent webhook delivery attempts. Requires admin scope.

### webhooks

Manage webhooks

- **`multimail-pp-cli webhooks create`** - Subscribe to email events. Returns the signing secret (shown only on creation). Requires admin scope.
- **`multimail-pp-cli webhooks create-postmark`** - Postmark bounce/complaint/delivery webhook handler
- **`multimail-pp-cli webhooks create-postmarkinbound`** - Receives inbound emails from Postmark. Authenticated via HTTP Basic Auth with the Postmark webhook secret. Not a consumer API endpoint.
- **`multimail-pp-cli webhooks delete`** - Delete a webhook subscription
- **`multimail-pp-cli webhooks get`** - Includes signing secret. Requires admin scope.
- **`multimail-pp-cli webhooks list`** - Requires admin scope. Signing secrets are not included in the list.

### well-known

Manage well known

- **`multimail-pp-cli well-known get`** - Rate-limited to 10 lookups per IP per hour.
- **`multimail-pp-cli well-known list`** - Returns the ECDSA P-256 public key used to sign X-MultiMail-Identity headers.
- **`multimail-pp-cli well-known list-wellknown`** - Returns OAuth authorization server metadata with an agent_auth extension block describing the auth.md agent registration flow.
- **`multimail-pp-cli well-known list-wellknown-2`** - Returns metadata about MultiMail as an OAuth-protected resource, including supported scopes and authorization servers. Part of the auth.md agent registration protocol.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
multimail-pp-cli account list

# JSON for scripting and agents
multimail-pp-cli account list --json

# Filter to specific fields
multimail-pp-cli account list --json --select id,name,status

# Dry run — show the request without sending
multimail-pp-cli account list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
multimail-pp-cli account list --agent
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

## Health Check

```bash
multimail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/multimail-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MULTIMAIL_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `multimail-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `multimail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MULTIMAIL_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every request** — Run: multimail-pp-cli auth set-token <your-mm_live-key>. Keys start with mm_live_ (prod) or mm_test_ (staging).
- **Rate limited (429) during sync** — Use --max-pages 5 to limit sync depth, or add --since 24h to narrow the window.
- **Oversight decide returns 404** — The email may have already been decided. Check: multimail-pp-cli audit-log list --json | head
- **Empty search results after sync** — Confirm sync completed: multimail-pp-cli doctor. If stale, re-sync: multimail-pp-cli sync --resources emails --full

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**multimail-mcp**](https://github.com/multimail-dev/mcp-server) — TypeScript
- [**agenticmail**](https://github.com/agenticmail/agenticmail) — JavaScript
- [**agentmail-mcp**](https://github.com/agentmail-to/agentmail-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
