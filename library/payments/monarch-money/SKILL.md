---
name: pp-monarch-money
description: "Use Monarch Money CLI for personal finance account balances, tags, transactions, cashflow summaries, explicit transaction CRUD workflows, and guarded read-only GraphQL queries. Use when the user asks about Monarch Money data, recent spending, tagged transactions, account balances, cashflow, or transaction cleanup from a terminal or agent workflow."
author: "Count"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - monarch-money-pp-cli
    install:
      - kind: go
        bins: [monarch-money-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/monarch-money/cmd/monarch-money-pp-cli
---

# Monarch Money — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `monarch-money-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install monarch-money --cli-only
   ```
2. Verify: `monarch-money-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/monarch-money/cmd/monarch-money-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when the user asks about:

- Monarch Money account balances
- Monarch Money transaction tags
- recent or date-filtered Monarch transactions
- monthly or custom-period cashflow
- creating, updating, tagging, or deleting a specific Monarch transaction
- a specific read-only Monarch GraphQL query

Do not use it for rule changes, account refreshes, budgeting mutations, or broad remote writes. Transaction writes are available only through explicit first-class commands that dry-run by default and require `--yes` to apply.

## Authentication

The CLI supports a saved session or an environment token. Prefer environment variables over putting secrets directly in shell history:

```bash
MONARCH_EMAIL='user@example.com' MONARCH_PASSWORD='...' monarch-money-pp-cli login
```

If MFA is required:

```bash
MONARCH_EMAIL='user@example.com' MONARCH_PASSWORD='...' monarch-money-pp-cli login --mfa 123456
```

Alternatively, set an existing token:

```bash
export MONARCH_TOKEN='...'
```

Then verify:

```bash
monarch-money-pp-cli doctor
```

## Unique Capabilities

These capabilities aren't available in generic shell access to Monarch data.

- **`query`** — Run custom read-only GraphQL query files while refusing files that contain GraphQL mutations.

  _Use when the built-in account, tag, transaction, and cashflow commands are too narrow but the workflow still needs a read-only guard._

```bash
monarch-money-pp-cli query query.graphql --operation OperationName --variables '{"limit":10}'
```

- **Transaction CRUD commands** — Create, update, tag, and delete individual transactions through narrow mutation commands.

  _Use these for transaction cleanup after the user has identified the intended transaction. Omit `--yes` first to inspect the dry-run payload._

```bash
monarch-money-pp-cli transactions update TRANSACTION_ID --notes 'Reviewed'
monarch-money-pp-cli transactions update TRANSACTION_ID --notes 'Reviewed' --yes
```

## Best command mapping

- "Are we connected to Monarch?" → `monarch-money-pp-cli status`
- "Show account balances" → `monarch-money-pp-cli accounts`
- "What tags exist?" → `monarch-money-pp-cli tags --limit 200`
- "Recent transactions" → `monarch-money-pp-cli transactions --days 30 --limit 50`
- "Cashflow this month" → `monarch-money-pp-cli cashflow`
- "Cashflow in January" → `monarch-money-pp-cli cashflow --start 2026-01-01 --end 2026-01-31`
- "Update this transaction note" → `monarch-money-pp-cli transactions update TRANSACTION_ID --notes 'NOTE'`, then rerun with `--yes` after review
- "Put these tags on this transaction" → `monarch-money-pp-cli transactions set-tags TRANSACTION_ID --tag-id TAG_ID`, then rerun with `--yes` after review
- "Need raw output for analysis" → add `--json`

## Command reference

**accounts** — List financial accounts with balances, account type, and institution.

- `monarch-money-pp-cli accounts` — List account balances in a table.
- `monarch-money-pp-cli accounts --limit 10` — Display the first 10 accounts.
- `monarch-money-pp-cli accounts --json` — Return the raw GraphQL account payload.

**tags** — List household transaction tags and transaction counts.

- `monarch-money-pp-cli tags` — List tags and transaction counts.
- `monarch-money-pp-cli tags --search travel --limit 20` — Search tags by name.

**transactions** — List recent transactions with date, merchant, category, account, amount, and tags.

- `monarch-money-pp-cli transactions` — List recent transactions using the default 30-day window.
- `monarch-money-pp-cli transactions --days 7 --limit 25` — List recent transactions for a shorter window.
- `monarch-money-pp-cli transactions --start 2026-01-01 --end 2026-01-31 --json` — Return a custom date range as JSON.
- `monarch-money-pp-cli transactions --tag-id TAG_ID` — Filter by Monarch tag ID.

**transactions create** — Create a manual transaction. Dry-runs unless `--yes` is passed.

- `monarch-money-pp-cli transactions create --date 2026-01-15 --account-id ACCOUNT_ID --amount -42.50 --merchant 'Coffee Shop' --category-id CATEGORY_ID` — Preview a create mutation.
- Add `--yes` to apply after reviewing the dry-run payload.

**transactions update** — Update a transaction by ID. Dry-runs unless `--yes` is passed.

- `monarch-money-pp-cli transactions update TRANSACTION_ID --category-id CATEGORY_ID --notes 'Reviewed'` — Preview a transaction update.
- Supported update flags include `--category-id`, `--merchant`, `--goal-id`, `--amount`, `--date`, `--hide-from-reports true|false`, `--needs-review true|false`, and `--notes`.

**transactions set-tags** — Replace all tags on a transaction. Dry-runs unless `--yes` is passed.

- `monarch-money-pp-cli transactions set-tags TRANSACTION_ID --tag-id TAG_ID --tag-id ANOTHER_TAG_ID` — Preview replacing tags.
- `monarch-money-pp-cli transactions set-tags TRANSACTION_ID --clear` — Preview clearing tags.

**transactions delete** — Delete a transaction by ID. Dry-runs unless `--yes` is passed.

- `monarch-money-pp-cli transactions delete TRANSACTION_ID` — Preview deleting a transaction.

**cashflow** — Summarize income, expenses, net savings, and savings rate for a date range.

- `monarch-money-pp-cli cashflow` — Summarize the current month.
- `monarch-money-pp-cli cashflow --start 2026-01-01 --end 2026-01-31` — Summarize a custom period.

**query** — Run a read-only GraphQL query from a file for advanced/debug workflows.

- `monarch-money-pp-cli query query.graphql --operation OperationName --variables '{"limit":10}'` — Run a read-only query file.

**doctor** — Check local configuration and Monarch connectivity.

- `monarch-money-pp-cli doctor` — Validate auth and connectivity.

**status** — Verify the current Monarch Money session by making a read-only GraphQL request.

- `monarch-money-pp-cli status` — Confirm the current session can reach Monarch.

## Safety notes

Transaction write commands dry-run by default and require `--yes` before sending a mutation to Monarch. Use transaction IDs for writes; do not guess ambiguous targets from merchant names or amounts.

The advanced `query` command refuses GraphQL files containing `mutation`; do not use it as a write escape hatch.

Do not print or expose `MONARCH_TOKEN`, saved session contents, email/password values, or raw authentication responses.
