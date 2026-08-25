# Quentli CLI

**Every Quentli payments, invoicing, and SAT tax-billing endpoint, plus offline collection, reconciliation, and at-risk-subscription intelligence no other Quentli tool has.**

Quentli is the Stripe-like payments, cobranza, and facturación platform for Latin America. This CLI mirrors your customers, invoices, payments, subscriptions, tax invoices (SAT CFDI), and webhook events into a local database, then answers the questions ops and finance actually ask: who's behind on an invoice, which subscriptions are about to fail, which completed payments lack a valid SAT timbre, and what your net revenue really is — all offline and agent-native.

## Install

The recommended path installs both the `quentli-pp-cli` binary and the `pp-quentli` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install quentli
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install quentli --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install quentli --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install quentli --agent claude-code
npx -y @mvanhorn/printing-press-library install quentli --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/quentli/cmd/quentli-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quentli-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install quentli --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-quentli --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-quentli --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install quentli --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/quentli-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `QUENTLI_SECRET_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/quentli/cmd/quentli-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "quentli": {
      "command": "quentli-pp-mcp",
      "env": {
        "QUENTLI_SECRET_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with an organization API key. Set QUENTLI_SECRET_KEY to your sk_... secret key; every command sends Authorization: Bearer <key>. Do not share or commit the key.

## Quick Start

```bash
# Health check — confirms the CLI, config, and auth wiring work without hitting the API.
quentli-pp-cli doctor --dry-run

# Pull your portfolio into the local mirror so offline commands have data.
quentli-pp-cli sync --resources customers,invoices,payments,subscriptions,tax-invoices --full

# See every customer with an outstanding or overdue invoice and the next collection action.
quentli-pp-cli dunning --since 1w

# Drill into one customer's full financial snapshot before contacting them.
quentli-pp-cli customer balance cus_abc123

# Catch completed payments missing a valid SAT CFDI before monthly filing.
quentli-pp-cli reconcile --period 1m

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Collections that compound
- **`dunning`** — See every customer with an outstanding or overdue invoice, how much is owed, and the next collection action (send reminder, retry payment, or resend pay link).

  _Reach for this to answer who owes money and what to do next without firing dozens of API calls._

  ```bash
  quentli-pp-cli dunning --since 1w --json
  ```
- **`customer balance`** — Render a one-screen financial snapshot for a single customer: outstanding invoices, active subscriptions, payment methods, and CFDI linkage.

  _Reach for this before calling a customer about money so you have their full financial picture in one command._

  ```bash
  quentli-pp-cli customer balance cus_abc123 --json
  ```

### Churn prevention
- **`subs at-risk`** — Surface active subscriptions whose saved payment method is expired, unconfirmed, or deleted, or that have recent failed collection attempts, ranked by recovery urgency.

  _Pick this before renewal season to fix expiring cards instead of discovering silent churn after the fact._

  ```bash
  quentli-pp-cli subs at-risk --json
  ```

### SAT tax compliance
- **`reconcile`** — Cross-check completed or refunded payments and paid invoices against SAT tax-invoice status (VALID/CANCELED/PENDING) to find timbre gaps before monthly SAT filing.

  _Use before SAT filing to catch completed payments missing a VALID CFDI instead of eyeballing two exported lists._

  ```bash
  quentli-pp-cli reconcile --period 1m --json
  ```

### Revenue ops
- **`revenue`** — Aggregate payments and refunds by status, type, and currency over a window; see net collected vs returned in localized minor-currency amounts.

  _Reach for this to answer how much money actually landed last month and what came back in refunds._

  ```bash
  quentli-pp-cli revenue --since 30d --csv
  ```

### Operational alerts
- **`webhooks health`** — Aggregate webhook delivery events by status per endpoint and surface PAYMENT_ATTEMPT_FAILED events as an operational alert with a retry path.

  _Use on-call to spot failed webhook deliveries and refund/failure signals fast instead of polling dashboards._

  ```bash
  quentli-pp-cli webhooks health --since 24h --json
  ```

## Recipes

### Monday collections run

```bash
quentli-pp-cli dunning --since 1w --json --select customer.email,outstanding,next_action
```

Dump the whole collection queue as compact JSON narrowed to the fields a reminder script needs.

### At-risk subscriptions before renewals

```bash
quentli-pp-cli subs at-risk --json
```

List active subscriptions tied to expired or unconfirmed payment methods so you can fix cards before they churn.

### Monthly SAT filing check

```bash
quentli-pp-cli reconcile --period 1m --json
```

Find every completed payment that still lacks a valid SAT CFDI timbre before filing.

### Net revenue pull

```bash
quentli-pp-cli revenue --since 30d --csv
```

Export last month's net collected-vs-returned revenue for your books.

### On-call delivery health

```bash
quentli-pp-cli webhooks health --since 24h
```

Spot failed webhook deliveries and payment-failure signals in the last day.

## Usage

Run `quentli-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `QUENTLI_CONFIG_DIR`, `QUENTLI_DATA_DIR`, `QUENTLI_STATE_DIR`, or `QUENTLI_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `QUENTLI_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export QUENTLI_HOME=/srv/quentli
quentli-pp-cli doctor
```

Under `QUENTLI_HOME=/srv/quentli`, the four dirs resolve to `/srv/quentli/config`, `/srv/quentli/data`, `/srv/quentli/state`, and `/srv/quentli/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "quentli": {
      "command": "quentli-pp-mcp",
      "env": {
        "QUENTLI_HOME": "/srv/quentli"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `QUENTLI_DATA_DIR` overrides an explicit `--home` for that kind. Use `QUENTLI_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `QUENTLI_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `quentli-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### auth-links

Manage auth links

- **`quentli-pp-cli auth-links`** - Generates a one-time authenticated URL that signs the customer into the payment portal.

### customer-portal-session

Manage customer portal session

- **`quentli-pp-cli customer-portal-session`** - Generates a one-time authenticated URL that signs the customer into the portal at a supported destination.

### customers

Manage customers

- **`quentli-pp-cli customers create`** - Creates a new customer.
- **`quentli-pp-cli customers get-by-id`** - Returns a customer by id.
- **`quentli-pp-cli customers list`** - Returns customers.
- **`quentli-pp-cli customers update`** - Updates editable fields of an existing customer by id.

### discounts

Manage discounts

- **`quentli-pp-cli discounts create`** - Creates a new reusable discount.
- **`quentli-pp-cli discounts delete`** - Deletes a discount and all associated discount codes.
- **`quentli-pp-cli discounts get-by-id`** - Returns a single discount by id.
- **`quentli-pp-cli discounts list`** - Returns reusable discounts for the organization. One-off discounts are excluded.
- **`quentli-pp-cli discounts update`** - Updates editable fields of an existing discount by id.

### invoices

Manage invoices

- **`quentli-pp-cli invoices cancel`** - Cancels an unpaid invoice by id.
- **`quentli-pp-cli invoices create`** - Creates a new invoice for a customer.
- **`quentli-pp-cli invoices get-by-id`** - Returns an invoice by id.
- **`quentli-pp-cli invoices list`** - Returns invoices.
- **`quentli-pp-cli invoices update`** - Updates editable fields of an existing invoice by id.

### payment-concepts

Manage payment concepts

- **`quentli-pp-cli payment-concepts create`** - Creates a new product or service in the catalog.
- **`quentli-pp-cli payment-concepts get-by-id`** - Returns a single payment concept by id.
- **`quentli-pp-cli payment-concepts list`** - Returns payment concepts for the organization. One-off concepts are excluded.
- **`quentli-pp-cli payment-concepts update`** - Updates editable fields of an existing payment concept by id.

### payment-methods

Manage payment methods

- **`quentli-pp-cli payment-methods <id>`** - Deletes a payment method by id.

### payment-sessions

Manage payment sessions

- **`quentli-pp-cli payment-sessions create`** - Creates a hosted payment session. Resolves or creates the customer and returns an authenticated payment URL.
- **`quentli-pp-cli payment-sessions get-by-id`** - Returns a payment session by id.

### payments

Manage payments

- **`quentli-pp-cli payments create`** - Creates a direct payment against a saved payment method. Optionally attempts to process it immediately.
- **`quentli-pp-cli payments get-by-id`** - Returns a payment by id.
- **`quentli-pp-cli payments list`** - Returns payments for the organization.

### refunds

Manage refunds

- **`quentli-pp-cli refunds`** - Refunds a completed payment, fully or partially.

### setup-sessions

Manage setup sessions

- **`quentli-pp-cli setup-sessions`** - Creates a hosted session for enrolling a payment method. Resolves or creates the customer and returns an authenticated setup URL.

### subscriptions

Manage subscriptions

- **`quentli-pp-cli subscriptions cancel`** - Cancels an active subscription by id.
- **`quentli-pp-cli subscriptions create`** - Creates a new recurring subscription for a customer.
- **`quentli-pp-cli subscriptions get-by-id`** - Returns a subscription by id.
- **`quentli-pp-cli subscriptions list`** - Returns subscriptions.
- **`quentli-pp-cli subscriptions update`** - Updates editable fields of an existing subscription by id.

### tax-invoices

Manage tax invoices

- **`quentli-pp-cli tax-invoices create`** - Creates a tax invoice for an invoice or payment.
- **`quentli-pp-cli tax-invoices get-by-id`** - Returns a tax invoice by id.
- **`quentli-pp-cli tax-invoices list`** - Returns tax invoices for the organization.

### webhook-events

Manage webhook events

- **`quentli-pp-cli webhook-events`** - Returns webhook delivery events for the organization.

### webhooks

Manage webhooks

- **`quentli-pp-cli webhooks create`** - Creates a webhook endpoint for the organization.
- **`quentli-pp-cli webhooks delete`** - Soft-deletes a webhook endpoint.
- **`quentli-pp-cli webhooks get-by-id`** - Returns a webhook by id.
- **`quentli-pp-cli webhooks list`** - Returns webhook endpoints for the organization.
- **`quentli-pp-cli webhooks update`** - Updates a webhook endpoint.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`quentli-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`quentli-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`quentli-pp-cli learnings list`** - Inspect taught rows
- **`quentli-pp-cli learnings forget <query>`** - Undo a teach
- **`quentli-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`quentli-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`quentli-pp-cli teach-pattern`** - Install a query/resource template up front
- **`quentli-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `QUENTLI_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `quentli-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
quentli-pp-cli customers list

# JSON for scripting and agents
quentli-pp-cli customers list --json

# Filter to specific fields
quentli-pp-cli customers list --json --select id,name,status

# Dry run — show the request without sending
quentli-pp-cli customers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
quentli-pp-cli customers list --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
quentli-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `quentli-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/quentli-pp-cli/config.toml`; `--home`, `QUENTLI_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `QUENTLI_SECRET_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `quentli-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `quentli-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $QUENTLI_SECRET_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command returns 401 "You are not authenticated"** — Your QUENTLI_SECRET_KEY is missing or wrong. Set it and run quentli-pp-cli doctor.
- **Offline commands report a missing or stale mirror** — Run quentli-pp-cli sync --resources customers,invoices,payments,subscriptions,tax-invoices,webhook-events --full and pass --db if you moved the database.
- **Amounts look like large integers (150000)** — All amounts are in minor currency units; these commands render them as MXN 1,500.00 automatically.
