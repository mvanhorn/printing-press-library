---
name: pp-lemonsqueezy
description: "Every Lemon Squeezy resource, plus a local SQLite mirror that surfaces MRR, churn, license seats, and discount-campaign pace. Trigger phrases: `check Lemon Squeezy revenue`, `show MRR trend`, `what subscriptions churned this week`, `find failed renewals`, `audit Lemon Squeezy webhooks`, `track Founding-Member sale capacity`, `disable license keys for refunded order`, `use lemonsqueezy`, `run lemonsqueezy-pp-cli`."
author: "Joseph Alvin Castillo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - lemonsqueezy-pp-cli
    install:
      - kind: go
        bins: [lemonsqueezy-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/cmd/lemonsqueezy-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/payments/lemonsqueezy/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Lemon Squeezy — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `lemonsqueezy-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install lemonsqueezy --cli-only
   ```
2. Verify: `lemonsqueezy-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/cmd/lemonsqueezy-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Lemon Squeezy's official SDKs are libraries, not CLIs, and the dashboard is a click-walk per store. This CLI mirrors all 19 LS resources to local SQLite with offline FTS5 and cross-entity SQL, then layers transcendence commands — `revenue-snapshot`, `mrr-trend`, `churn-watch`, `dunning-alert`, `license-rollup`, `refund-cascade`, `campaign-watch`, `webhook-audit` — that combine entities no single endpoint returns. Built for indie SaaS founders and license-key sellers who live in the LS state machine.

## When to Use This CLI

Reach for this CLI when running ongoing Lemon Squeezy operations — Monday-morning revenue/churn sweeps, post-refund license-key disables, live capacity tracking during a capped sale, or webhook coverage audits across multiple stores. The cross-entity SQL and transcendence commands are the differentiator. For one-shot dashboard tasks (creating one product, editing a single discount) the LS web UI is still faster.

## Anti-triggers

Do not use this CLI for:
- Building a custom checkout form in a web app — use the official `lemonsqueezy.js` JavaScript SDK (the CLI is for ops, not embedded billing).
- Receiving webhook payloads in production — this CLI can replay and audit webhooks, but the actual HTTP server lives in your application.
- Tax/VAT reporting — Lemon Squeezy is merchant-of-record and ships its own tax docs in the dashboard; this CLI doesn't replicate them.
- Customer-facing license validation in your desktop app — call the LS license API directly from the app, not via this CLI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Revenue + churn signal
- **`revenue-snapshot`** — Point-in-time revenue rollup combining LS's denormalized 30-day/lifetime store counters with refund-adjusted net from local orders.

  _Reach for this when you want one number for 'how is the store doing right now' without walking every order through the API._

  ```bash
  lemonsqueezy-pp-cli revenue-snapshot --agent
  ```
- **`mrr-trend`** — Weekly MRR over a sliding window, classified as new / renewal / refunded with a week-over-week net delta.

  _Pick this over a raw `list-subscriptions` call when you need to show how MRR moved week-over-week, not just where it stands today._

  ```bash
  lemonsqueezy-pp-cli mrr-trend --weeks 12 --json
  ```
- **`churn-watch`** — Lists subscriptions that flipped to past_due/unpaid/cancelled/expired in a window, with customer email and dollar exposure per row.

  _Reach for this on Monday-morning sweeps to see what changed since Friday — beats clicking through the dashboard's status filters._

  ```bash
  lemonsqueezy-pp-cli churn-watch --since 7d --json
  ```
- **`dunning-alert`** — Lists subscription-invoices with status=failed whose parent subscription is still active or past_due — the recoverable window.

  _Use this to find subs whose latest renewal failed but who haven't churned yet — the window where a Slack ping or grace email recovers revenue._

  ```bash
  lemonsqueezy-pp-cli dunning-alert --json
  ```

### License + refund ops
- **`license-rollup`** — Per-variant and per-key activation statistics joined across license-keys, license-key-instances, and variants.

  _Reach for this when you need to surface piracy-shaped seat distributions or just answer 'how many keys are active per variant'._

  ```bash
  lemonsqueezy-pp-cli license-rollup --json --select keys
  ```
- **`refund-cascade`** — Given a refunded order ID, walks order → order-items → license-keys → instances, then optionally disables the keys via --apply.

  _Use this after a refund hits, to make sure the buyer's license keys actually get disabled instead of staying active._

  ```bash
  lemonsqueezy-pp-cli refund-cascade order_3aBc --dry-run --json
  ```

### Campaign + integration ops
- **`campaign-watch`** — Per discount code: redemptions used vs cap, redemption velocity over last 24h, projected sellout time at current pace (needs >=6h of redemption activity to stabilise).

  _Run this during a capped sale (Founding-Member tier, Early Access) to know when a tier will sell out so you can expire the code before overselling._

  ```bash
  lemonsqueezy-pp-cli campaign-watch FOUNDING-LIFETIME FOUNDING-2YR FOUNDING-1YR --json
  ```
- **`webhook-audit`** — Cross-store webhook coverage matrix grouped by URL host, flagging stale destinations (localhost, ngrok, *.test, *.local).

  _Reach for this to catch orphaned webhooks pointing at last week's ngrok URL before they cause a missed event in production._

  ```bash
  lemonsqueezy-pp-cli webhook-audit --json
  ```

## Command Reference

**affiliates** — Manage affiliates

- `lemonsqueezy-pp-cli affiliates get` — Lemon Squeezy Retrieve an affiliate
- `lemonsqueezy-pp-cli affiliates list` — Lemon Squeezy List all affiliates

**checkouts** — Manage checkouts

- `lemonsqueezy-pp-cli checkouts create` — Lemon Squeezy Create a checkout
- `lemonsqueezy-pp-cli checkouts get` — Lemon Squeezy Retrieve a checkout
- `lemonsqueezy-pp-cli checkouts list` — Lemon Squeezy List all checkouts

**customers** — Manage customers

- `lemonsqueezy-pp-cli customers get` — Lemon Squeezy Retrieve a customer
- `lemonsqueezy-pp-cli customers list` — Lemon Squeezy List all customers

**discount-redemptions** — Manage discount redemptions

- `lemonsqueezy-pp-cli discount-redemptions get` — Lemon Squeezy Retrieve a discount redemption
- `lemonsqueezy-pp-cli discount-redemptions list` — Lemon Squeezy List all discount redemptions

**discounts** — Manage discounts

- `lemonsqueezy-pp-cli discounts create` — Lemon Squeezy Create a discount
- `lemonsqueezy-pp-cli discounts delete` — Lemon Squeezy Delete a discount
- `lemonsqueezy-pp-cli discounts get` — Lemon Squeezy Retrieve a discount
- `lemonsqueezy-pp-cli discounts list` — Lemon Squeezy List all discounts

**files** — Manage files

- `lemonsqueezy-pp-cli files get` — Lemon Squeezy Retrieve a file
- `lemonsqueezy-pp-cli files list` — Lemon Squeezy List all files

**health** — Manage health

- `lemonsqueezy-pp-cli health` — Lemon Squeezy Health

**license-key-instances** — Manage license key instances

- `lemonsqueezy-pp-cli license-key-instances get` — Lemon Squeezy Retrieve a license key instance
- `lemonsqueezy-pp-cli license-key-instances list` — Lemon Squeezy List all license key instances

**license-keys** — Manage license keys

- `lemonsqueezy-pp-cli license-keys get` — Lemon Squeezy Retrieve a license key
- `lemonsqueezy-pp-cli license-keys list` — Lemon Squeezy List all license keys

**order-items** — Manage order items

- `lemonsqueezy-pp-cli order-items get` — Lemon Squeezy Retrieve an order item
- `lemonsqueezy-pp-cli order-items list` — Lemon Squeezy List all order items

**orders** — Manage orders

- `lemonsqueezy-pp-cli orders get` — Lemon Squeezy Retrieve an order
- `lemonsqueezy-pp-cli orders list` — Lemon Squeezy List all orders

**prices** — Manage prices

- `lemonsqueezy-pp-cli prices get` — Lemon Squeezy Retrieve a price
- `lemonsqueezy-pp-cli prices list` — Lemon Squeezy List all prices

**products** — Manage products

- `lemonsqueezy-pp-cli products get` — Lemon Squeezy Retrieve a product
- `lemonsqueezy-pp-cli products list` — Lemon Squeezy List all products

**stores** — Manage stores

- `lemonsqueezy-pp-cli stores get` — Lemon Squeezy Retrieve a store
- `lemonsqueezy-pp-cli stores list` — Lemon Squeezy List all stores

**subscription-invoices** — Manage subscription invoices

- `lemonsqueezy-pp-cli subscription-invoices get` — Lemon Squeezy Retrieve a subscription invoice
- `lemonsqueezy-pp-cli subscription-invoices list` — Lemon Squeezy List all subscription invoices

**subscription-items** — Manage subscription items

- `lemonsqueezy-pp-cli subscription-items get` — Lemon Squeezy Retrieve a subscription item
- `lemonsqueezy-pp-cli subscription-items list` — Lemon Squeezy List all subscription items
- `lemonsqueezy-pp-cli subscription-items update` — Lemon Squeezy Update a subscription item

**subscriptions** — Manage subscriptions

- `lemonsqueezy-pp-cli subscriptions delete` — Lemon Squeezy Cancel a Subscription
- `lemonsqueezy-pp-cli subscriptions get` — Lemon Squeezy Retrieve a subscription
- `lemonsqueezy-pp-cli subscriptions list` — Lemon Squeezy List all subscriptions
- `lemonsqueezy-pp-cli subscriptions update` — Lemon Squeezy Update a subscription

**usage-records** — Manage usage records

- `lemonsqueezy-pp-cli usage-records create` — Lemon Squeezy Create a usage record
- `lemonsqueezy-pp-cli usage-records get` — Lemon Squeezy Retrieve a usage-record
- `lemonsqueezy-pp-cli usage-records list` — Lemon Squeezy List all usage records

**users** — Manage users

- `lemonsqueezy-pp-cli users` — Lemon Squeezy Retrieve the authenticated user

**variants** — Manage variants

- `lemonsqueezy-pp-cli variants get` — Lemon Squeezy Retrieve a variant
- `lemonsqueezy-pp-cli variants list` — Lemon Squeezy List all variants

**webhooks** — Manage webhooks

- `lemonsqueezy-pp-cli webhooks create` — Lemon Squeezy Create a webhook
- `lemonsqueezy-pp-cli webhooks delete` — Lemon Squeezy Delete a webhook
- `lemonsqueezy-pp-cli webhooks get` — Lemon Squeezy Retrieve a webhook
- `lemonsqueezy-pp-cli webhooks list` — Lemon Squeezy List all webhooks
- `lemonsqueezy-pp-cli webhooks update` — Lemon Squeezy Update a webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
lemonsqueezy-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monday-morning revenue + churn sweep

```bash
lemonsqueezy-pp-cli sync --resources stores,subscriptions,subscription-invoices,orders --since 14d && lemonsqueezy-pp-cli revenue-snapshot --json && lemonsqueezy-pp-cli churn-watch --since 7d --json
```

Refreshes the local mirror, then prints the revenue rollup and the past-week churn delta — the ritual that replaces the dashboard click-walk.

### Find recoverable failed renewals

```bash
lemonsqueezy-pp-cli dunning-alert --json --select rows.customer_email,rows.amount_usd
```

Lists every failed invoice whose subscription is still active or past_due — the dollar-recoverable window before the customer churns.

### Refund cascade for an order

```bash
lemonsqueezy-pp-cli refund-cascade order_3aBc --json
```

Walks the order → order-items → license-keys chain and previews which keys would be disabled. Add --apply on the rerun to actually disable them (the command refuses --apply unless LS itself reports the order as refunded).

### Track a capped Founding-Member sale

```bash
lemonsqueezy-pp-cli sync --resources discounts,discount-redemptions && lemonsqueezy-pp-cli campaign-watch FOUNDING-LIFETIME FOUNDING-2YR FOUNDING-1YR --json
```

Live capacity + redemption velocity per tier with sellout projection — run hourly during a launch to expire codes before overselling.

### Webhook coverage audit

```bash
lemonsqueezy-pp-cli sync --resources webhooks && lemonsqueezy-pp-cli webhook-audit --json --select hosts.url,hosts.stale,hosts.event_count
```

Groups every webhook across every store by URL host, flags localhost/ngrok/*.test destinations so orphans don't break production.

## Auth Setup

Lemon Squeezy uses HTTP Bearer auth. Create an API key at https://app.lemonsqueezy.com/settings/api, then export `LEMONSQUEEZY_API_KEY=<your-key>`. Verify with `lemonsqueezy-pp-cli doctor`.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  lemonsqueezy-pp-cli affiliates list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
lemonsqueezy-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
lemonsqueezy-pp-cli feedback --stdin < notes.txt
lemonsqueezy-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/lemonsqueezy-pp-cli/feedback.jsonl`. They are never POSTed unless `LEMONSQUEEZY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LEMONSQUEEZY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
lemonsqueezy-pp-cli profile save briefing --json
lemonsqueezy-pp-cli --profile briefing affiliates list
lemonsqueezy-pp-cli profile list --json
lemonsqueezy-pp-cli profile show briefing
lemonsqueezy-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `lemonsqueezy-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/cmd/lemonsqueezy-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add lemonsqueezy-pp-mcp -- lemonsqueezy-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which lemonsqueezy-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   lemonsqueezy-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `lemonsqueezy-pp-cli <command> --help`.
