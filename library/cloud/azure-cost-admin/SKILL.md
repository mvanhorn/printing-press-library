---
name: pp-azure-cost-admin
description: "Printing Press CLI for read-only Azure cost review. Use Azure Cost Management, Resource Graph, simple anomaly checks, and public retail price lookup support from the terminal."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - azure-cost-admin-pp-cli
    install:
      - kind: go
        bins: [azure-cost-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/azure-cost-admin/cmd/azure-cost-admin-pp-cli
---

# Azure Cost Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `azure-cost-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install azure-cost-admin --cli-only
   ```
2. Verify: `azure-cost-admin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/azure-cost-admin/cmd/azure-cost-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## What This CLI Does

Azure Cost Admin is a read-only CLI for Azure FinOps questions. Use it to inspect actual billed spend, service and resource-group cost breakdowns, tag hygiene, simple cost changes, and public Azure retail price metadata.

It reads Azure Cost Management, Azure Resource Graph, and Azure Retail Prices data. It does not create budgets, edit tags, delete resources, create resources, or change Azure configuration.

## When Not to Use This CLI

Do not use this CLI for creating, updating, deleting, tagging, purchasing, changing budgets, changing alerts, granting access, or modifying Azure resources. It is for read-only inspection and estimate support.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`spend summary`** — Queries Azure Cost Management for actual billed spend over a selected timeframe.
- **`spend by-service`** — Groups actual Azure spend by service, with sibling commands for resource groups and tag keys.
- **`tags untagged`** — Uses Azure Resource Graph to find resources missing a requested owner, environment, or cost-center tag.
- **`anomalies`** — Compares recent service spend against a prior window and reports large changes without modifying Azure.
- **`price search`** — Searches public Azure Retail Prices separately from actual billed spend so estimates do not get confused with invoice data.

## Auth Setup

Use Azure CLI authentication:

```bash
az login
az account set --subscription <subscription-id-or-name>
```

Never paste Azure tokens, tenant IDs, subscription IDs, account emails, raw resource IDs, exact spend amounts, or customer names into public artifacts.

## Command Reference

**doctor** — Check local Azure access and read-only API reachability.

- `azure-cost-admin-pp-cli doctor --dry-run` — Shows the checks that would run.
- `azure-cost-admin-pp-cli doctor --json` — Runs Azure account, Cost Management, Resource Graph, and Retail Prices checks.

**subscriptions** — List reachable subscriptions.

- `azure-cost-admin-pp-cli subscriptions --json --select name,state` — Emits subscription names and states in JSON.

**spend summary** — Show actual Azure spend totals for a timeframe.

- `azure-cost-admin-pp-cli spend summary --timeframe MonthToDate --dry-run` — Shows the Cost Management request shape.
- `azure-cost-admin-pp-cli spend summary --timeframe MonthToDate --agent --select timeframe,currency` — Runs the query while keeping output small.

**spend by-service** — Group actual spend by Azure service.

- `azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --dry-run` — Shows the service grouping request shape.
- `azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --agent --select timeframe,currency` — Runs the service spend query with selected fields.

**spend by-resource-group** — Group actual spend by resource group.

- `azure-cost-admin-pp-cli spend by-resource-group --timeframe MonthToDate --dry-run` — Shows the resource-group grouping request shape.

**spend by-tag** — Group actual spend by a tag key.

- `azure-cost-admin-pp-cli spend by-tag --tag owner --timeframe MonthToDate --dry-run` — Shows the tag grouping request shape.

**anomalies** — Compare recent service spend with the previous window.

- `azure-cost-admin-pp-cli anomalies --days 7 --threshold-percent 25 --dry-run` — Shows the comparison plan.

**tags untagged** — Find resources missing a requested tag key.

- `azure-cost-admin-pp-cli tags untagged --tag owner --limit 20 --dry-run` — Shows the Resource Graph query shape.
- `azure-cost-admin-pp-cli tags untagged --tag owner --limit 1 --agent --select type` — Runs the query while keeping output minimal.

**price search** — Search public Azure Retail Prices for estimate support.

- `azure-cost-admin-pp-cli price search --service "Virtual Machines" --region eastus --currency USD --dry-run` — Shows the public price URL.
- `azure-cost-admin-pp-cli price search --service "Virtual Machines" --region eastus --currency USD --limit 1 --agent --select serviceName,region,currencyCode` — Runs a public lookup with selected fields.

## Agent Mode

Add `--agent` to emit compact JSON for agents and scripts. Use `--select` to keep only comma-separated JSON fields. Use `--dry-run` to preview a read-only request before calling Azure.

Examples:

```bash
azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --agent --select timeframe,currency
azure-cost-admin-pp-cli tags untagged --tag owner --limit 1 --agent --select type
```

## Safety Guidance

Default human output masks sensitive identifiers where practical. JSON output is intended for trusted local scripts and agents.

When sharing proof, redact subscription IDs, tenant IDs, account emails, resource IDs, resource group names, customer names, exact spend amounts, and tokens.
