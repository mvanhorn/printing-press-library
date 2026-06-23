# RevenueCat CLI

**Every RevenueCat v2 endpoint, plus a local database, offline search, and revenue/churn/refund intelligence the dashboard charts can't compose.**

Drive the full RevenueCat Developer API v2 from the shell or an agent: customers, subscriptions, entitlements, products, offerings, purchases, invoices, and charts. On top of the typed endpoint surface it adds a local SQLite mirror and novel commands — revenue-snapshot with run-over-run diffs, mrr-trend with movement decomposition, churn-watch with dollar exposure, dunning-alert, entitlement-rollup, refund-cascade, and trial-funnel — that join synced data in ways no single API call returns.

Created by [@jcastillo725](https://github.com/jcastillo725) (Joseph Alvin Castillo).

## Install

The recommended path installs both the `revenuecat-pp-cli` binary and the `pp-revenuecat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install revenuecat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install revenuecat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install revenuecat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install revenuecat --agent claude-code
npx -y @mvanhorn/printing-press-library install revenuecat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/revenuecat/cmd/revenuecat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/revenuecat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install revenuecat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-revenuecat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-revenuecat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install revenuecat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/revenuecat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `REVENUECAT_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/revenuecat/cmd/revenuecat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "revenuecat": {
      "command": "revenuecat-pp-mcp",
      "env": {
        "REVENUECAT_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

RevenueCat v2 uses a server-side Secret API key sent as 'Authorization: Bearer <key>'. Create one in the RevenueCat dashboard under Project Settings -> API Keys -> Secret API keys (a read-only key is enough for every read command). This is distinct from the public SDK keys. Set REVENUECAT_API_KEY and REVENUECAT_PROJECT_ID in your environment; run 'revenuecat-pp-cli doctor' to confirm both resolve against your project.

## Quick Start

```bash
# Health check — confirms the binary and config wiring without calling the API.
revenuecat-pp-cli doctor --dry-run

# Pull the core entities into the local store so offline and join commands work.
revenuecat-pp-cli sync --resources customers,subscriptions,entitlements

# Current MRR/ARR/actives/revenue with deltas vs your last snapshot.
revenuecat-pp-cli revenue-snapshot --currency USD

# Who churned this week and the dollar exposure, as agent-native JSON.
revenuecat-pp-cli churn-watch --since 7d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Revenue intelligence
- **`revenue-snapshot`** — Point-in-time MRR, ARR, active subscriptions, trials, and revenue in your chosen currency, with deltas against your last snapshot.

  _Reach for this when an agent or operator asks 'where's revenue right now and how did it move since last check' in one call._

  ```bash
  revenuecat-pp-cli revenue-snapshot --currency USD --agent
  ```
- **`mrr-trend`** — MRR over time with new / expansion / contraction / churn movement decomposition and week-over-week deltas.

  _Reach for this to explain why MRR moved, not just that it moved._

  ```bash
  revenuecat-pp-cli mrr-trend --period week --limit 12 --agent
  ```
- **`trial-funnel`** — New trials to conversion-to-paying funnel with stage-to-stage drop-off.

  _Reach for this to see where trials leak before they convert._

  ```bash
  revenuecat-pp-cli trial-funnel --since 30d --agent
  ```

### Retention & risk
- **`churn-watch`** — Which subscriptions flipped to billing-issue, grace, expired, or cancelled in a window, with per-subscription dollar exposure.

  _Reach for this when you need who churned and how much MRR they took, not just the churn rate line._

  ```bash
  revenuecat-pp-cli churn-watch --since 7d --agent
  ```
- **`dunning-alert`** — Subscriptions still in grace or billing-issue state joined with their unpaid invoices, ranked by recoverable amount.

  _Reach for this to act on failed billing while the revenue is still recoverable._

  ```bash
  revenuecat-pp-cli dunning-alert --agent
  ```
- **`entitlement-rollup`** — Per-entitlement active-customer counts and product attachments, flagging customers whose entitlement state disagrees with their subscription state.

  _Reach for this to catch silent entitlement desync (a dropped webhook, a refund that didn't cascade) across the whole customer base._

  ```bash
  revenuecat-pp-cli entitlement-rollup --flag-disagreements --agent
  ```

### Operations
- **`refund-cascade`** — Traces a subscription or purchase through its transactions, refund history, and resulting entitlement loss; --apply issues the refund.

  _Reach for this to understand or execute a refund and see exactly which entitlements the customer loses._

  ```bash
  revenuecat-pp-cli refund-cascade sub1a2b3c4d --agent
  ```
- **`webhook-audit`** — Lists configured webhook integrations grouped by destination host, flagging duplicate or stale destinations.

  _Reach for this to confirm webhook delivery config before a dropped event silently desyncs entitlements._

  ```bash
  revenuecat-pp-cli webhook-audit --agent
  ```

## Recipes

### Morning standup revenue line

```bash
revenuecat-pp-cli revenue-snapshot --currency USD --agent --select mrr,active_subscriptions,mrr_delta
```

One pasteable line of MRR, active subs, and the change since the last snapshot.

### Recoverable failed billing

```bash
revenuecat-pp-cli dunning-alert --agent
```

Subscriptions still in grace or billing-issue joined with unpaid invoices, ranked by what you can still recover.

### Find entitlement desync

```bash
revenuecat-pp-cli entitlement-rollup --flag-disagreements --agent
```

Surfaces customers whose entitlement state disagrees with their subscription state across the whole base.

### Trace a refund's fallout

```bash
revenuecat-pp-cli refund-cascade sub1a2b3c4d --agent
```

Shows the transactions, refund history, and entitlements a customer loses before you issue the refund.

## Usage

Run `revenuecat-pp-cli --help` for the full command reference and flag list.

## Commands

### apps

Operations about apps.

- **`revenuecat-pp-cli apps create`** - This endpoint requires the following permission(s): <code>project_configuration:apps:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli apps delete`** - This endpoint requires the following permission(s): <code>project_configuration:apps:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli apps get`** - This endpoint requires the following permission(s): <code>project_configuration:apps:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli apps list`** - This endpoint requires the following permission(s): <code>project_configuration:apps:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli apps update`** - This endpoint requires the following permission(s): <code>project_configuration:apps:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### audit-logs

Operations about audit logs.

- **`revenuecat-pp-cli audit-logs <project_id>`** - This endpoint requires the following permission(s): <code>project_configuration:audit_logs:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### charts

Manage charts

- **`revenuecat-pp-cli charts <project_id>`** - Returns time-series data for a specific chart.

**Response Structure**

The response includes:
- Chart metadata (category, display_name, description)
- Time boundaries (start_date, end_date, last_computed_at)
- Data values (array of data points)
- Summary statistics
- Segment information (when segmented)

**Chart Types**

Different charts may return data in slightly different formats:
- Standard charts: values as arrays of data points with timestamps
- Cohort charts: values include cohort-specific data structures
- Segmented charts: include segment information in the response

**Filtering and Segmentation**

Use the `/charts/{chart_name}/options` endpoint to discover available
filters and segments for a specific chart before making requests.

Filter parameters vary by chart and can be passed as additional query parameters.

**Aggregation**

Use `aggregate` to request summary-only output for supported charts.
When `aggregate` is provided, `values` is returned as an empty array and
`summary` includes only the requested aggregate operations.

**Incomplete data**
For the most recent periods, data may be flagged as incomplete, and may not be appropriate to use for analysis.
 This endpoint requires the following permission(s): <code>charts_metrics:charts:read</code>. This endpoint belongs to the <strong>Charts & Metrics</strong> domain, which has a default rate limit of <strong>15 requests per minute</strong>.

### collaborators

Operations about collaborators.

- **`revenuecat-pp-cli collaborators <project_id>`** - This endpoint requires the following permission(s): <code>project_configuration:collaborators:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### customers

Operations about customers.

- **`revenuecat-pp-cli customers create`** - This endpoint requires the following permission(s): <code>customer_information:customers:read_write</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.
- **`revenuecat-pp-cli customers delete`** - This endpoint requires the following permission(s): <code>customer_information:customers:read_write</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.
- **`revenuecat-pp-cli customers get`** - This endpoint requires the following permission(s): <code>customer_information:customers:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.
- **`revenuecat-pp-cli customers list`** - This endpoint requires the following permission(s): <code>customer_information:customers:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.

### entitlements

Operations about entitlements.

- **`revenuecat-pp-cli entitlements create`** - This endpoint requires the following permission(s): <code>project_configuration:entitlements:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli entitlements delete`** - This endpoint requires the following permission(s): <code>project_configuration:entitlements:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli entitlements get`** - This endpoint requires the following permission(s): <code>project_configuration:entitlements:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli entitlements list`** - This endpoint requires the following permission(s): <code>project_configuration:entitlements:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli entitlements update`** - This endpoint requires the following permission(s): <code>project_configuration:entitlements:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### integrations

Operations about integrations.

- **`revenuecat-pp-cli integrations create-webhook`** - This endpoint requires the following permission(s): <code>project_configuration:integrations:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli integrations delete-webhook`** - This endpoint requires the following permission(s): <code>project_configuration:integrations:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli integrations get-webhook`** - This endpoint requires the following permission(s): <code>project_configuration:integrations:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli integrations list-webhook`** - This endpoint requires the following permission(s): <code>project_configuration:integrations:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli integrations update-webhook`** - This endpoint requires the following permission(s): <code>project_configuration:integrations:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### media-assets

Manage media assets

- **`revenuecat-pp-cli media-assets <project_id>`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### metrics

Manage metrics

- **`revenuecat-pp-cli metrics get-overview`** - This endpoint requires the following permission(s): <code>charts_metrics:overview:read</code>. This endpoint belongs to the <strong>Charts & Metrics</strong> domain, which has a default rate limit of <strong>15 requests per minute</strong>.
- **`revenuecat-pp-cli metrics get-revenue`** - Returns the total revenue for the project across all of its apps for the
given inclusive date range `[start_date, end_date]`. The value is expressed
in the project's primary currency unless `currency` is provided.

This endpoint is backed by the same realtime (v3) revenue chart that powers
the dashboard. It is intended for cases where an app needs an authoritative
revenue total without inferring it from the transaction list.

Note that the most recent day in the range may be partial if it includes
today, since transactions for today are still arriving.
 This endpoint requires the following permission(s): <code>charts_metrics:overview:read</code>. This endpoint belongs to the <strong>Charts & Metrics</strong> domain, which has a default rate limit of <strong>15 requests per minute</strong>.

### offerings

Operations about offerings.

- **`revenuecat-pp-cli offerings create`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli offerings delete`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli offerings get`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli offerings list`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli offerings update`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### packages

Operations about packages.

- **`revenuecat-pp-cli packages delete-from-offering`** - This endpoint requires the following permission(s): <code>project_configuration:packages:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli packages get`** - This endpoint requires the following permission(s): <code>project_configuration:packages:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli packages update`** - This endpoint requires the following permission(s): <code>project_configuration:packages:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### paywalls

Operations about paywalls.

- **`revenuecat-pp-cli paywalls create`** - Create a paywall draft for a project. You can either use the offering template shortcut or provide full draft components directly.
 This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli paywalls delete`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli paywalls get`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli paywalls list`** - This endpoint requires the following permission(s): <code>project_configuration:offerings:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli paywalls update`** - Update a paywall draft. If the paywall is already published, this updates its draft version without changing the published version.
 This endpoint requires the following permission(s): <code>project_configuration:offerings:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### products

Operations about products.

- **`revenuecat-pp-cli products create`** - <div class="theme-admonition theme-admonition-info alert alert--warning">
  <div class="heading">Warning</div>
  <div>This endpoint does not allow to create Web Billing products.</div>
This endpoint requires the following permission(s): <code>project_configuration:products:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli products delete`** - This endpoint requires the following permission(s): <code>project_configuration:products:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli products get`** - This endpoint requires the following permission(s): <code>project_configuration:products:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli products list`** - This endpoint requires the following permission(s): <code>project_configuration:products:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli products update`** - This endpoint requires the following permission(s): <code>project_configuration:products:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### projects

Operations about projects.

- **`revenuecat-pp-cli projects create`** - This endpoint requires the following permission(s): <code>project_configuration:projects:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli projects list`** - This endpoint requires the following permission(s): <code>project_configuration:projects:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

### purchases

Operations about purchases.

- **`revenuecat-pp-cli purchases get`** - This endpoint requires the following permission(s): <code>customer_information:purchases:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.
- **`revenuecat-pp-cli purchases search`** - Search for a one-time purchases by any of its associated `store_purchase_identifier` values.

For example, this may include the `transactionId` of any transaction in an Apple App Store purchase, or any order ID from a Google Play Store purchase.
 This endpoint requires the following permission(s): <code>customer_information:purchases:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.

### subscriptions

Operations about subscriptions.

- **`revenuecat-pp-cli subscriptions get`** - This endpoint requires the following permission(s): <code>customer_information:subscriptions:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.
- **`revenuecat-pp-cli subscriptions search`** - Search for a subscription by any of its associated `store_subscription_identifier` values, whether from a past or current subscription period.

For example, this may include the `transactionId` of any transaction in an Apple App Store subscription, or any order ID from a Google Play Store subscription.
 This endpoint requires the following permission(s): <code>customer_information:subscriptions:read</code>. This endpoint belongs to the <strong>Customer Information</strong> domain, which has a default rate limit of <strong>480 requests per minute</strong>.

### virtual-currencies

Manage virtual currencies

- **`revenuecat-pp-cli virtual-currencies create-virtual-currency`** - This endpoint requires the following permission(s): <code>project_configuration:virtual_currencies:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli virtual-currencies delete-virtual-currency`** - This endpoint requires the following permission(s): <code>project_configuration:virtual_currencies:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli virtual-currencies get-virtual-currency`** - This endpoint requires the following permission(s): <code>project_configuration:virtual_currencies:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli virtual-currencies list`** - This endpoint requires the following permission(s): <code>project_configuration:virtual_currencies:read</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.
- **`revenuecat-pp-cli virtual-currencies update-virtual-currency`** - This endpoint requires the following permission(s): <code>project_configuration:virtual_currencies:read_write</code>. This endpoint belongs to the <strong>Project Configuration</strong> domain, which has a default rate limit of <strong>60 requests per minute</strong>.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
revenuecat-pp-cli apps list mock-value

# JSON for scripting and agents
revenuecat-pp-cli apps list mock-value --json

# Filter to specific fields
revenuecat-pp-cli apps list mock-value --json --select id,name,status

# Dry run — show the request without sending
revenuecat-pp-cli apps list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
revenuecat-pp-cli apps list mock-value --agent
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
revenuecat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/developer-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `REVENUECAT_API_KEY` | per_call | No | Set to your API credential. |
| `DEVELOPER_BEARER_AUTH` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `revenuecat-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `revenuecat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $REVENUECAT_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports the API key is missing or invalid** — Set REVENUECAT_API_KEY to a v2 Secret key (starts with sk_) from Project Settings -> API Keys; the public SDK keys will not work.
- **Commands return a project-not-found or 404 error** — Set REVENUECAT_PROJECT_ID to your project id, or pass --project <id>; every v2 endpoint is project-scoped.
- **HTTP 429 rate-limit errors during sync or fan-out** — RevenueCat rate-limits per domain; re-run sync with a smaller --resources set or let the built-in adaptive limiter back off.
