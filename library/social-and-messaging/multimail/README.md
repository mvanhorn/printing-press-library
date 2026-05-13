# Multimail CLI

CLI for [MultiMail](https://multimail.dev) — verifiable identity and graduated oversight for AI agents over email. Five autonomy modes from read-only to fully autonomous. Inbound email → markdown, outbound markdown → HTML.

## Install

The recommended path installs both the `multimail-pp-cli` binary and the `pp-multimail` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install multimail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install multimail --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/multimail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-multimail --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-multimail skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-multimail. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
multimail-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export MULTIMAIL_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
multimail-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
multimail-pp-cli account list
```

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

- **`multimail-pp-cli admin create`** - Admin-only. Creates a new API key and emails it to the tenant's oversight email. Used when welcome email failed or KV expired before key retrieval.

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

- **`multimail-pp-cli audit-log list`** - Returns audit log entries with cursor pagination. Requires admin scope.

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

### domains

Manage domains

- **`multimail-pp-cli domains create`** - Add a custom domain (Pro/Scale only)
- **`multimail-pp-cli domains delete`** - Delete a custom domain
- **`multimail-pp-cli domains get`** - Get custom domain detail
- **`multimail-pp-cli domains list`** - Requires admin scope.

### emails

Manage emails

- **`multimail-pp-cli emails list`** - Requires read scope. Without a status filter, returns spam_flagged and spam_quarantined emails across all tenant mailboxes.

### funnel

Manage funnel

- **`multimail-pp-cli funnel create`** - Pricing page beacon hit via navigator.sendBeacon to track open/submit/error events on the signup modal. Fire-and-forget; counters are best-effort (KV is non-atomic). IP-rate-limited to 30 req/min.

### mailboxes

Manage mailboxes

- **`multimail-pp-cli mailboxes create`** - Requires admin scope. Address can be a local part (appended to tenant subdomain) or full address on a verified custom domain.
- **`multimail-pp-cli mailboxes delete`** - Requires admin scope.
- **`multimail-pp-cli mailboxes list`** - Requires read scope.
- **`multimail-pp-cli mailboxes update`** - Requires admin scope. Oversight mode can only be downgraded here; upgrades require the upgrade flow.

### multimail-export

Manage multimail export

- **`multimail-pp-cli multimail-export list`** - Requires admin scope. Rate limited to 1 request per hour.

### multimail-health

Manage multimail health

- **`multimail-pp-cli multimail-health list`** - Verifies D1 and R2 connectivity. No auth required.

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

- **`multimail-pp-cli slug-check get`** - Check if a slug is available for registration. Returns suggestions if taken or reserved. No auth required.

### support

Manage support

- **`multimail-pp-cli support create`** - Public endpoint. Requires a solved ALTCHA proof-of-work payload. Sends a message to the operator's support address.

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

- **`multimail-pp-cli usage list`** - Requires read scope. Returns usage counts for the current billing period.

### webhook-deliveries

Manage webhook deliveries

- **`multimail-pp-cli webhook-deliveries list`** - Returns recent webhook delivery attempts. Requires admin scope.

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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-multimail -g
```

Then invoke `/pp-multimail <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add multimail multimail-pp-mcp -e MULTIMAIL_BEARER_AUTH=<your-token>
```

</details>

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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `multimail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MULTIMAIL_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
