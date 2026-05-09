# Monarch Money CLI

Read-oriented Monarch Money CLI generated with CLI Printing Press.

This CLI wraps Monarch's browser API/GraphQL interface for practical terminal and agent workflows: checking connectivity, listing accounts and tags, reviewing transactions, summarizing cashflow, and running guarded read-only GraphQL queries.

## Install

The recommended path installs both the `monarch-money-pp-cli` binary and the `pp-monarch-money` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install monarch-money
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install monarch-money --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available, install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/monarch-money/cmd/monarch-money-pp-cli@latest
```

This installs the CLI only -- no skill.

## Authentication

The CLI supports either a saved session or an environment token. Prefer environment variables over putting credentials directly in shell history:

```bash
MONARCH_EMAIL='user@example.com' MONARCH_PASSWORD='...' monarch-money-pp-cli login
monarch-money-pp-cli status
```

If MFA is required:

```bash
MONARCH_EMAIL='user@example.com' MONARCH_PASSWORD='...' monarch-money-pp-cli login --mfa 123456
monarch-money-pp-cli status
```

Environment fallback:

```bash
export MONARCH_TOKEN='...'
monarch-money-pp-cli status
```

Session file:

```text
~/.monarch-pp-cli/session.json
```

The login flow requests Monarch's trusted-device `/auth/login/` token and refuses to save short-lived JWT-style feature tokens.

## Unique Features

- **Guarded GraphQL query runner** — `query` lets advanced users run custom read-only Monarch GraphQL query files while refusing files that contain GraphQL mutations.

  ```bash
  monarch-money-pp-cli query query.graphql --operation OperationName --variables '{"limit":10}'
  ```

## Commands

- `login` — log in and save a local session token
- `status` — verify connectivity with a read-only GraphQL request
- `doctor` — check local auth and live connectivity
- `accounts` — list accounts with balances, type, and institution
- `tags` — list household transaction tags and counts
- `transactions` — list recent transactions with merchant, category, account, amount, and tags
- `cashflow` — summarize income, expenses, net savings, and savings rate for a date range
- `query` — run a read-only GraphQL query from a file; GraphQL mutations are refused

## Examples

```bash
monarch-money-pp-cli accounts
monarch-money-pp-cli tags --limit 20
monarch-money-pp-cli transactions --days 30 --limit 25
monarch-money-pp-cli transactions --start 2026-01-01 --end 2026-01-31 --json
monarch-money-pp-cli cashflow --start 2026-01-01 --end 2026-01-31
```

## Safety model

This contribution is intentionally read-oriented first. Mutation workflows should be added only with explicit dry-run/confirmation semantics.

`query` performs a simple safety check and refuses query files containing `mutation`.

## Known limitations

- Monarch Money does not publish an official public OpenAPI spec, so this implementation is based on observed browser/GraphQL behavior and the community Python client.
- Authentication may require MFA depending on the account.
- GraphQL schema changes upstream may require query updates.
