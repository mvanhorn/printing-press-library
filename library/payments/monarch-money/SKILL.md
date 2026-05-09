---
name: pp-monarch-money
description: "Use Monarch Money CLI for personal finance account balances, transaction tags, transactions, and cashflow summaries. Use when the user asks about Monarch Money data, recent spending, tagged transactions, account balances, or cashflow from a terminal or agent workflow."
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

## Prerequisites: install the CLI

Verify the CLI exists before invoking it:

```bash
monarch-money-pp-cli --version
```

If missing and this project has been contributed to the Printing Press library:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/monarch-money/cmd/monarch-money-pp-cli@latest
```

From a local source checkout:

```bash
go build -o monarch-money-pp-cli ./cmd/monarch-money-pp-cli
install -m 0755 monarch-money-pp-cli /opt/homebrew/bin/monarch-money-pp-cli
```

## Authentication

Preferred login:

```bash
monarch-money-pp-cli login --email "$MONARCH_EMAIL" --password "$MONARCH_PASSWORD" --mfa 123456
```

Alternative for already-managed environments:

```bash
export MONARCH_TOKEN='...'
```

Then verify:

```bash
monarch-money-pp-cli doctor
```

## Best command mapping

- "Are we connected to Monarch?" → `monarch-money-pp-cli status`
- "Show account balances" → `monarch-money-pp-cli accounts`
- "What tags exist?" → `monarch-money-pp-cli tags --limit 200`
- "Recent transactions" → `monarch-money-pp-cli transactions --days 30 --limit 50`
- "Cashflow this month" → `monarch-money-pp-cli cashflow`
- "Cashflow in January" → `monarch-money-pp-cli cashflow --start 2026-01-01 --end 2026-01-31`
- "Need raw output for analysis" → add `--json`

## Command reference

```bash
monarch-money-pp-cli accounts [--json]
monarch-money-pp-cli tags [--search text] [--limit 200] [--json]
monarch-money-pp-cli transactions [--days 30] [--limit 50] [--start YYYY-MM-DD --end YYYY-MM-DD] [--tag-id ID] [--account-id ID] [--search text] [--json]
monarch-money-pp-cli cashflow [--start YYYY-MM-DD --end YYYY-MM-DD] [--json]
monarch-money-pp-cli query file.graphql --operation OperationName --variables '{"key":"value"}'
monarch-money-pp-cli doctor
monarch-money-pp-cli status
```

## Safety notes

This CLI is read-oriented. It does not expose Monarch mutations as first-class commands. The advanced `query` command refuses GraphQL files containing `mutation`.

Do not print or expose `MONARCH_TOKEN`, saved session contents, email/password values, or raw authentication responses.
