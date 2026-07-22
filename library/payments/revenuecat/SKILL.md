---
name: pp-revenuecat
description: "Every RevenueCat v2 endpoint, plus a local database, offline search Trigger phrases: `what's my MRR`, `revenue snapshot`, `who churned this week`, `show failed billing`, `find entitlement desync`, `trace this refund`, `use revenuecat`, `run revenuecat`."
author: "Joseph Alvin Castillo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - revenuecat-pp-cli
    install:
      - kind: go
        bins: [revenuecat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/revenuecat/cmd/revenuecat-pp-cli
---

# RevenueCat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `revenuecat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install revenuecat --cli-only
   ```
2. Verify: `revenuecat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/revenuecat/cmd/revenuecat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you need programmatic or agent-driven visibility into RevenueCat mobile subscription state: revenue and MRR/ARR snapshots, churn and dunning analysis, entitlement reconciliation, refund tracing, and webhook config audits. It is the mobile-rail companion to a Lemon Squeezy CLI for the web rail.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for the in-app purchase flow itself; that is the RevenueCat mobile SDK, not the server API.
- Do not use this CLI for browsing RevenueCat blog posts, docs, or State-of-Subscription reports; that is the marketing site, not the Developer API.
- Do not use this CLI for v1-only operations that v2 does not yet cover; fall back to the v1 API for those.

## Unique Capabilities

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

## Command Reference

**apps** — Operations about apps.

- `revenuecat-pp-cli apps create` — This endpoint requires the following permission(s): project_configuration:apps:read_write .
- `revenuecat-pp-cli apps delete` — This endpoint requires the following permission(s): project_configuration:apps:read_write .
- `revenuecat-pp-cli apps get` — This endpoint requires the following permission(s): project_configuration:apps:read .
- `revenuecat-pp-cli apps list` — This endpoint requires the following permission(s): project_configuration:apps:read .
- `revenuecat-pp-cli apps update` — This endpoint requires the following permission(s): project_configuration:apps:read_write .

**audit-logs** — Operations about audit logs.

- `revenuecat-pp-cli audit-logs <project_id>` — This endpoint requires the following permission(s): project_configuration:audit_logs:read .

**charts** — Manage charts

- `revenuecat-pp-cli charts <project_id>` — Returns time-series data for a specific chart.

**collaborators** — Operations about collaborators.

- `revenuecat-pp-cli collaborators <project_id>` — This endpoint requires the following permission(s): project_configuration:collaborators:read .

**customers** — Operations about customers.

- `revenuecat-pp-cli customers create` — This endpoint requires the following permission(s): customer_information:customers:read_write .
- `revenuecat-pp-cli customers delete` — This endpoint requires the following permission(s): customer_information:customers:read_write .
- `revenuecat-pp-cli customers get` — This endpoint requires the following permission(s): customer_information:customers:read .
- `revenuecat-pp-cli customers list` — This endpoint requires the following permission(s): customer_information:customers:read .

**entitlements** — Operations about entitlements.

- `revenuecat-pp-cli entitlements create` — This endpoint requires the following permission(s): project_configuration:entitlements:read_write .
- `revenuecat-pp-cli entitlements delete` — This endpoint requires the following permission(s): project_configuration:entitlements:read_write .
- `revenuecat-pp-cli entitlements get` — This endpoint requires the following permission(s): project_configuration:entitlements:read .
- `revenuecat-pp-cli entitlements list` — This endpoint requires the following permission(s): project_configuration:entitlements:read .
- `revenuecat-pp-cli entitlements update` — This endpoint requires the following permission(s): project_configuration:entitlements:read_write .

**integrations** — Operations about integrations.

- `revenuecat-pp-cli integrations create-webhook` — This endpoint requires the following permission(s): project_configuration:integrations:read_write .
- `revenuecat-pp-cli integrations delete-webhook` — This endpoint requires the following permission(s): project_configuration:integrations:read_write .
- `revenuecat-pp-cli integrations get-webhook` — This endpoint requires the following permission(s): project_configuration:integrations:read .
- `revenuecat-pp-cli integrations list-webhook` — This endpoint requires the following permission(s): project_configuration:integrations:read .
- `revenuecat-pp-cli integrations update-webhook` — This endpoint requires the following permission(s): project_configuration:integrations:read_write .

**media-assets** — Manage media assets

- `revenuecat-pp-cli media-assets <project_id>` — This endpoint requires the following permission(s): project_configuration:offerings:read_write .

**metrics** — Manage metrics

- `revenuecat-pp-cli metrics get-overview` — This endpoint requires the following permission(s): charts_metrics:overview:read .
- `revenuecat-pp-cli metrics get-revenue` — Returns the total revenue for the project across all of its apps for the given inclusive date range `[start_date

**offerings** — Operations about offerings.

- `revenuecat-pp-cli offerings create` — This endpoint requires the following permission(s): project_configuration:offerings:read_write .
- `revenuecat-pp-cli offerings delete` — This endpoint requires the following permission(s): project_configuration:offerings:read_write .
- `revenuecat-pp-cli offerings get` — This endpoint requires the following permission(s): project_configuration:offerings:read .
- `revenuecat-pp-cli offerings list` — This endpoint requires the following permission(s): project_configuration:offerings:read .
- `revenuecat-pp-cli offerings update` — This endpoint requires the following permission(s): project_configuration:offerings:read_write .

**packages** — Operations about packages.

- `revenuecat-pp-cli packages delete-from-offering` — This endpoint requires the following permission(s): project_configuration:packages:read_write .
- `revenuecat-pp-cli packages get` — This endpoint requires the following permission(s): project_configuration:packages:read .
- `revenuecat-pp-cli packages update` — This endpoint requires the following permission(s): project_configuration:packages:read_write .

**paywalls** — Operations about paywalls.

- `revenuecat-pp-cli paywalls create` — Create a paywall draft for a project.
- `revenuecat-pp-cli paywalls delete` — This endpoint requires the following permission(s): project_configuration:offerings:read_write .
- `revenuecat-pp-cli paywalls get` — This endpoint requires the following permission(s): project_configuration:offerings:read .
- `revenuecat-pp-cli paywalls list` — This endpoint requires the following permission(s): project_configuration:offerings:read .
- `revenuecat-pp-cli paywalls update` — Update a paywall draft.

**products** — Operations about products.

- `revenuecat-pp-cli products create` — Warning This endpoint does not allow to create Web Billing products.
- `revenuecat-pp-cli products delete` — This endpoint requires the following permission(s): project_configuration:products:read_write .
- `revenuecat-pp-cli products get` — This endpoint requires the following permission(s): project_configuration:products:read .
- `revenuecat-pp-cli products list` — This endpoint requires the following permission(s): project_configuration:products:read .
- `revenuecat-pp-cli products update` — This endpoint requires the following permission(s): project_configuration:products:read_write .

**projects** — Operations about projects.

- `revenuecat-pp-cli projects create` — This endpoint requires the following permission(s): project_configuration:projects:read_write .
- `revenuecat-pp-cli projects list` — This endpoint requires the following permission(s): project_configuration:projects:read .

**purchases** — Operations about purchases.

- `revenuecat-pp-cli purchases get` — This endpoint requires the following permission(s): customer_information:purchases:read .
- `revenuecat-pp-cli purchases search` — Search for a one-time purchases by any of its associated `store_purchase_identifier` values.

**subscriptions** — Operations about subscriptions.

- `revenuecat-pp-cli subscriptions get` — This endpoint requires the following permission(s): customer_information:subscriptions:read .
- `revenuecat-pp-cli subscriptions search` — Search for a subscription by any of its associated `store_subscription_identifier` values

**virtual-currencies** — Manage virtual currencies

- `revenuecat-pp-cli virtual-currencies create-virtual-currency` — This endpoint requires the following permission(s): project_configuration:virtual_currencies:read_write .
- `revenuecat-pp-cli virtual-currencies delete-virtual-currency` — This endpoint requires the following permission(s): project_configuration:virtual_currencies:read_write .
- `revenuecat-pp-cli virtual-currencies get-virtual-currency` — This endpoint requires the following permission(s): project_configuration:virtual_currencies:read .
- `revenuecat-pp-cli virtual-currencies list` — This endpoint requires the following permission(s): project_configuration:virtual_currencies:read .
- `revenuecat-pp-cli virtual-currencies update-virtual-currency` — This endpoint requires the following permission(s): project_configuration:virtual_currencies:read_write .


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
revenuecat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

RevenueCat v2 uses a server-side Secret API key sent as 'Authorization: Bearer <key>'. Create one in the RevenueCat dashboard under Project Settings -> API Keys -> Secret API keys (a read-only key is enough for every read command). This is distinct from the public SDK keys. Set REVENUECAT_API_KEY and REVENUECAT_PROJECT_ID in your environment; run 'revenuecat-pp-cli doctor' to confirm both resolve against your project.

Run `revenuecat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  revenuecat-pp-cli apps list mock-value --agent --select id,name,status
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
revenuecat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
revenuecat-pp-cli feedback --stdin < notes.txt
revenuecat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/revenuecat-pp-cli/feedback.jsonl`. They are never POSTed unless `REVENUECAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `REVENUECAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
revenuecat-pp-cli profile save briefing --json
revenuecat-pp-cli --profile briefing apps list mock-value
revenuecat-pp-cli profile list --json
revenuecat-pp-cli profile show briefing
revenuecat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `revenuecat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/revenuecat/cmd/revenuecat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add revenuecat-pp-mcp -- revenuecat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which revenuecat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   revenuecat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `revenuecat-pp-cli <command> --help`.
