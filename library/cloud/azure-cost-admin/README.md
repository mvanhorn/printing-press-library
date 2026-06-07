# Azure Cost Admin CLI

Read-only Azure cost and tag hygiene CLI for engineers, FinOps teams, analysts, and agents.

Printing Press library slug: `azure-cost-admin`. Category: `cloud`.

Azure bills can be hard to explain because spend is spread across subscriptions, services, resource groups, and tags. `azure-cost-admin-pp-cli` gives teams a direct terminal workflow for the common questions that come up during cost review:

- What did we spend this month?
- Which Azure services are driving cost?
- Which resource groups are expensive?
- Which owner, environment, or cost-center tags are missing?
- Did any service spend change sharply compared with the previous window?

The CLI reads Azure Cost Management and Azure Resource Graph data. It does not create budgets, edit tags, delete resources, create resources, or change Azure configuration.

## Install

After publication through Printing Press:

```bash
npx -y @mvanhorn/printing-press-library install azure-cost-admin
```

For local development:

```bash
go install ./cmd/azure-cost-admin-pp-cli
```

## Authentication

The recommended local path is Azure CLI authentication:

```bash
az login
az account set --subscription <subscription-id-or-name>
az account show --query "{name:name,id:id}" -o table
```

The CLI uses the active Azure CLI session for Cost Management and Resource Graph requests. Keep credentials, tenant IDs, subscription IDs, and account emails out of shared proof files and pull requests.

For automation, create a service principal with the least read access your organization allows, then sign in through Azure CLI before running the commands:

```bash
az ad sp create-for-rbac \
  --name azure-cost-admin \
  --role "Cost Management Reader" \
  --scopes /subscriptions/<subscription-id>
```

## Quick Start

```bash
# Confirm Azure auth and read-only API access.
azure-cost-admin-pp-cli doctor

# See month-to-date actual spend.
azure-cost-admin-pp-cli spend summary --timeframe MonthToDate

# Break actual spend down by Azure service.
azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --limit 10

# Break actual spend down by resource group.
azure-cost-admin-pp-cli spend by-resource-group --timeframe MonthToDate --limit 10

# Break actual spend down by a tag key.
azure-cost-admin-pp-cli spend by-tag --tag owner --timeframe MonthToDate

# Find resources missing an owner tag.
azure-cost-admin-pp-cli tags untagged --tag owner --limit 20

# Compare recent service spend with the previous window.
azure-cost-admin-pp-cli anomalies --days 7 --threshold-percent 25

# Search public retail prices for estimate support.
azure-cost-admin-pp-cli price search --service "Virtual Machines" --region eastus --sku "D2s" --currency USD
```

## Actual Spend vs Price Estimates

`spend` commands use Azure Cost Management. They report actual billed cost for the subscription and timeframe you can access.

`price search` uses the public Azure Retail Prices API. It is useful for estimate checks, but it is not the same as billed cost. Discounts, reservations, savings plans, negotiated pricing, credits, taxes, and marketplace charges can make real bills differ from public retail prices.

## Commands

### doctor

Checks Azure CLI authentication, active subscription access, Cost Management access, Resource Graph access, and Retail Prices reachability. Default output redacts sensitive identifiers.

```bash
azure-cost-admin-pp-cli doctor
azure-cost-admin-pp-cli doctor --json
```

### subscriptions

Lists Azure subscriptions visible to the current Azure CLI identity. Human output masks subscription IDs. JSON output includes fields useful for automation.

```bash
azure-cost-admin-pp-cli subscriptions
azure-cost-admin-pp-cli subscriptions --json
```

### spend summary

Shows total actual spend for a timeframe.

```bash
azure-cost-admin-pp-cli spend summary --timeframe MonthToDate
azure-cost-admin-pp-cli spend summary --timeframe Custom --from 2026-06-01 --to 2026-06-07
```

### spend by-service

Groups actual spend by Azure service.

```bash
azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --limit 10
```

### spend by-resource-group

Groups actual spend by resource group.

```bash
azure-cost-admin-pp-cli spend by-resource-group --timeframe MonthToDate --limit 10
```

### spend by-tag

Groups actual spend by a tag key.

```bash
azure-cost-admin-pp-cli spend by-tag --tag owner --timeframe MonthToDate
azure-cost-admin-pp-cli spend by-tag --tag environment --timeframe TheLastMonth
```

### anomalies

Compares current service spend with the previous window. For example, `--days 7` compares the latest seven-day window with the seven days before it.

```bash
azure-cost-admin-pp-cli anomalies --days 7 --threshold-percent 25 --limit 10
```

### tags untagged

Uses Azure Resource Graph to list resources missing a requested tag.

```bash
azure-cost-admin-pp-cli tags untagged --tag owner --limit 20
azure-cost-admin-pp-cli tags untagged --tag cost_center --resource-group <resource-group-name>
```

### price search

Searches the public Azure Retail Prices API for estimate support.

```bash
azure-cost-admin-pp-cli price search --service "Virtual Machines" --region eastus --sku "D2s" --currency USD
```

## Agent and Script Output

Every command supports:

- `--json` for structured output
- `--agent` for compact agent-friendly JSON output
- `--select` to keep only comma-separated JSON fields
- `--dry-run` to print the read-only request shape without calling Azure

Examples:

```bash
azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --agent
azure-cost-admin-pp-cli spend by-service --timeframe MonthToDate --agent --select total,currency
azure-cost-admin-pp-cli tags untagged --tag owner --dry-run
```

## Safety

The CLI is read-only. It should not print tokens, tenant IDs, account emails, full subscription IDs, or raw resource IDs in default human output. JSON output is intended for trusted local scripts and agents.

When sharing validation evidence publicly, redact:

- subscription IDs
- tenant IDs
- user emails
- resource IDs
- resource group names when they identify a customer or environment
- exact spend amounts
- tokens or credentials

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`spend summary`** — Queries Azure Cost Management for actual billed spend over a selected timeframe.
- **`spend by-service`** — Groups actual Azure spend by service, with sibling commands for resource groups and tag keys.
- **`tags untagged`** — Uses Azure Resource Graph to find resources missing a requested owner, environment, or cost-center tag.
- **`anomalies`** — Compares recent service spend against a prior window and reports large changes without modifying Azure.
- **`price search`** — Searches public Azure Retail Prices separately from actual billed spend so estimates do not get confused with invoice data.
