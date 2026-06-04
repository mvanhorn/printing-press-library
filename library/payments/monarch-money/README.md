# Monarch Money CLI

Monarch Money CLI generated with CLI Printing Press.

This CLI wraps Monarch's browser API/GraphQL interface for practical terminal and agent workflows: checking connectivity, listing accounts and tags, reviewing transactions, summarizing cashflow, creating and editing manual transactions, and running guarded read-only GraphQL queries.

Created by [@count](https://github.com/count) (Count).

## Install

The recommended path installs both the `monarch-money-pp-cli` binary and the `pp-monarch-money` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install monarch-money
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install monarch-money --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install monarch-money --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install monarch-money --agent claude-code
npx -y @mvanhorn/printing-press-library install monarch-money --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/monarch-money/cmd/monarch-money-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/monarch-money-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-monarch-money --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-monarch-money --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install monarch-money --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

- **Explicit transaction writes** — create, update, tag, and delete transaction workflows are exposed as first-class commands. Write commands dry-run by default and require `--yes` to apply.

  ```bash
  monarch-money-pp-cli transactions update TRANSACTION_ID --notes 'Reviewed by agent'
  monarch-money-pp-cli transactions update TRANSACTION_ID --notes 'Reviewed by agent' --yes
  ```

## Commands

- `login` — log in and save a local session token
- `status` — verify connectivity with a read-only GraphQL request
- `doctor` — check local auth and live connectivity
- `accounts` — list accounts with balances, type, and institution
- `tags` — list household transaction tags and counts
- `transactions` — list recent transactions with merchant, category, account, amount, and tags
- `transactions create` — create a manual transaction; dry-run unless `--yes` is passed
- `transactions update` — update a transaction by ID; dry-run unless `--yes` is passed
- `transactions set-tags` — replace all tags on a transaction; dry-run unless `--yes` is passed
- `transactions delete` — delete a transaction by ID; dry-run unless `--yes` is passed
- `cashflow` — summarize income, expenses, net savings, and savings rate for a date range
- `query` — run a read-only GraphQL query from a file; GraphQL mutations are refused

## Examples

```bash
monarch-money-pp-cli accounts
monarch-money-pp-cli tags --limit 20
monarch-money-pp-cli transactions --days 30 --limit 25
monarch-money-pp-cli transactions --start 2026-01-01 --end 2026-01-31 --json
monarch-money-pp-cli transactions create --date 2026-01-15 --account-id ACCOUNT_ID --amount -42.50 --merchant 'Coffee Shop' --category-id CATEGORY_ID
monarch-money-pp-cli transactions update TRANSACTION_ID --category-id CATEGORY_ID --notes 'Reviewed'
monarch-money-pp-cli transactions set-tags TRANSACTION_ID --tag-id TAG_ID --tag-id ANOTHER_TAG_ID
monarch-money-pp-cli cashflow --start 2026-01-01 --end 2026-01-31
```

## Safety model

GraphQL writes are exposed through explicit commands with narrow inputs. Transaction write commands print a dry-run payload by default and require `--yes` before sending a mutation to Monarch.

`query` performs a safety check and refuses query files containing `mutation`; it is not a raw write escape hatch.

## Known limitations

- Monarch Money does not publish an official public OpenAPI spec, so this implementation is based on observed browser/GraphQL behavior and the community Python client.
- Authentication may require MFA depending on the account.
- GraphQL schema changes upstream may require query updates.
