# Mercury CLI

Streamline financial tasks with secure account management and transaction processing. Enables user registration, balance tracking, and payment handling.

Learn more at [Mercury](https://docs.mercury.com/reference).

## Install

The recommended path installs both the `mercury-pp-cli` binary and the `pp-mercury` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install mercury
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install mercury --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mercury/cmd/mercury-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercury-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mercury --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mercury --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-mercury skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-mercury. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
mercury-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export MERCURY_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
mercury-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
mercury-pp-cli account mock-value
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`workflow payment-plan`** — Builds a read-only approval plan with body, idempotency key, dry-run command, and execute command before payment or transfer writes.

  _Agents can prepare exact write commands without moving money._

  ```bash
  mercury-pp-cli workflow payment-plan --kind transfer --source-account-id acct_src --destination-account-id acct_dst --amount 25 --agent
  ```
- **`workflow archive`** — Syncs supported Mercury resources into a local SQLite store for offline search and analytics.

  _Reduces API calls and gives agents repeatable context._

  ```bash
  mercury-pp-cli workflow archive --agent
  ```
- **`agent-context`** — Emits machine-readable command metadata for agents and MCP hosts.

  _Improves autonomous command selection and reduces context waste._

  ```bash
  mercury-pp-cli agent-context --agent
  ```

## Usage

Run `mercury-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Manage bank accounts

- **`mercury-pp-cli account get`** - Get account by ID

### accounts

Manage bank accounts

- **`mercury-pp-cli accounts get`** - Retrieve a paginated list of accounts. Supports cursor-based pagination with limit, order, start_after, and end_before query parameters.

### ar

Manage ar

- **`mercury-pp-cli ar cancel-invoice`** - Cancel an invoice. This action cannot be undone.
- **`mercury-pp-cli ar create-customer`** - Create a new customer for the organization
- **`mercury-pp-cli ar create-invoice`** - Create a new invoice for the organization
- **`mercury-pp-cli ar delete-customer`** - Delete a customer. This action cannot be undone.
- **`mercury-pp-cli ar get-attachment`** - Retrieve attachment details including download URL
- **`mercury-pp-cli ar get-customer`** - Retrieve details of a specific customer by their ID
- **`mercury-pp-cli ar get-invoice`** - Retrieve details of an invoice by its ID
- **`mercury-pp-cli ar get-invoice-pdf`** - Downloads a PDF file for the specified invoice. The response includes a Content-Disposition header set to 'attachment' with the filename.
- **`mercury-pp-cli ar list-customers`** - Retrieve a paginated list of customers. Supports cursor-based pagination with limit, order, start_after, and end_before query parameters.
- **`mercury-pp-cli ar list-invoice-attachments`** - Retrieve a list of all attachments for a specific invoice
- **`mercury-pp-cli ar list-invoices`** - Retrieve a paginated list of invoices. Supports cursor-based pagination with limit, order, start_after, and end_before query parameters.
- **`mercury-pp-cli ar update-customer`** - Update an existing customer
- **`mercury-pp-cli ar update-invoice`** - Update an existing invoice

### books

Manage organization books

- **`mercury-pp-cli books delete-agent-coa-template`** - Delete a specific Chart of Accounts template.
- **`mercury-pp-cli books delete-agent-ledger-template`** - Delete an existing ledger within an agent-owned Chart of Accounts template.
- **`mercury-pp-cli books delete-journal-entries`** - Bulk delete journal entries
- **`mercury-pp-cli books get-agent-coa-template`** - Retrieve details of a specific Chart of Accounts template.
- **`mercury-pp-cli books get-agent-coa-templates`** - Retrieve a paginated list of all default and agent-owned Chart of Accounts templates. These templates can be used when creating new Books instances for clients.
- **`mercury-pp-cli books get-journal-entries`** - List all journal entries
- **`mercury-pp-cli books get-journal-entry`** - Retrieve a Journal Entry
- **`mercury-pp-cli books post-agent-coa-templates`** - Create a new agent-owned Chart of Accounts template. These templates can be used when creating new Books instances for clients.
- **`mercury-pp-cli books post-agent-ledger-templates`** - Create a new ledger within an agent-owned Chart of Accounts template.
- **`mercury-pp-cli books post-journal-entries`** - Create multiple Journal Entries
- **`mercury-pp-cli books put-agent-ledger-template`** - Update an existing ledger within an agent-owned Chart of Accounts template.
- **`mercury-pp-cli books put-journal-entries`** - Bulk update journal entries

### cards

Manage cards

- **`mercury-pp-cli cards create`** - Issue a new virtual card.
- **`mercury-pp-cli cards get`** - Retrieve details of a specific card by its ID.
- **`mercury-pp-cli cards list`** - Retrieve a paginated list of cards.
- **`mercury-pp-cli cards update`** - Update a card's nickname or spending limits.

### categories

Manage expense categories

- **`mercury-pp-cli categories create-category`** - Create a new custom expense category for the organization.
- **`mercury-pp-cli categories list`** - Retrieve a paginated list of all available custom expense categories for the organization. Supports cursor-based pagination with limit, order, start_after, and end_before query parameters.

### credit

Manage credit accounts

- **`mercury-pp-cli credit list`** - Retrieve a list of all credit accounts for the organization.

### events

Manage API events

- **`mercury-pp-cli events get`** - Get all events
- **`mercury-pp-cli events get-eventid`** - Get event by ID

### organization

Organization information

- **`mercury-pp-cli organization get`** - Retrieve information about your organization including EIN, legal business name, and DBAs.

### recipient

Manage payment recipients

- **`mercury-pp-cli recipient get`** - Retrieve details of a specific recipient by ID
- **`mercury-pp-cli recipient update`** - Edit information about a specific recipient

### recipients

Manage payment recipients

- **`mercury-pp-cli recipients create`** - Create a new recipient for making payments
- **`mercury-pp-cli recipients get`** - Retrieve a paginated list of all recipients. Use cursor parameters (start_after, end_before) for pagination.
- **`mercury-pp-cli recipients list-attachments`** - Retrieve a paginated list of all recipient tax form attachments across all recipients in the organization. Use cursor parameters (start_after, end_before) for pagination.

### request-send-money

Manage request send money

- **`mercury-pp-cli request-send-money get-send-money-approval-request`** - Get send money approval request by ID
- **`mercury-pp-cli request-send-money list-send-money-approval-requests`** - Retrieve a paginated list of send money approval requests for the authenticated organization. Supports filtering by account and status.

### safes

Manage SAFE (Simple Agreement for Future Equity) requests

- **`mercury-pp-cli safes get-request`** - Retrieve a specific SAFE request by its ID.
- **`mercury-pp-cli safes get-requests`** - Retrieve all SAFE (Simple Agreement for Future Equity) requests for your organization.

### statements

Download account statements

### transaction

Manage transactions

- **`mercury-pp-cli transaction get-by-id`** - Retrieve a single transaction by its ID. Returns full transaction details including attachments, check images, and related metadata.
- **`mercury-pp-cli transaction update`** - Update the note and/or category of an existing transaction. Use null values to clear existing data.

### transactions

Manage transactions

- **`mercury-pp-cli transactions list`** - Retrieve a paginated list of all transactions across all accounts. Supports advanced filtering by date ranges, status, categories, and cursor-based pagination.

### transfer

Manage transfer

- **`mercury-pp-cli transfer create-internal`** - Transfer funds between two accounts within the same organization. Supports transfers between depository accounts (checking/savings), from a depository account to a treasury/investment account, and from a treasury/investment account to a depository account. Creates paired debit and credit transactions.

### treasury

Manage treasury accounts and transactions

- **`mercury-pp-cli treasury get`** - Retrieve a paginated list of all treasury accounts associated with the authenticated organization. Use cursor parameters (start_after, end_before) for pagination.

### users

Manage organization team members

- **`mercury-pp-cli users get`** - Get all users
- **`mercury-pp-cli users get-userid`** - Get user by ID

### webhooks

Manage webhooks

- **`mercury-pp-cli webhooks create`** - Register a new webhook endpoint to receive event notifications
- **`mercury-pp-cli webhooks delete`** - Delete a webhook endpoint
- **`mercury-pp-cli webhooks get`** - Retrieve a paginated list of all webhook endpoints for your organization. Supports filtering by status.
- **`mercury-pp-cli webhooks get-webhookendpointid`** - Retrieve details of a specific webhook endpoint by ID
- **`mercury-pp-cli webhooks update`** - Update the configuration of an existing webhook endpoint. A webhook that has been disabled due to consecutive delivery failures can be reactivated by setting its status to 'active'.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mercury-pp-cli account mock-value

# JSON for scripting and agents
mercury-pp-cli account mock-value --json

# Filter to specific fields
mercury-pp-cli account mock-value --json --select id,name,status

# Dry run — show the request without sending
mercury-pp-cli account mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mercury-pp-cli account mock-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-mercury -g
```

Then invoke `/pp-mercury <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mercury/cmd/mercury-pp-mcp@latest
```

Then register it:

```bash
claude mcp add mercury mercury-pp-mcp -e MERCURY_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercury-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MERCURY_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mercury/cmd/mercury-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mercury": {
      "command": "mercury-pp-mcp",
      "env": {
        "MERCURY_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
mercury-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/mercury-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MERCURY_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mercury-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MERCURY_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
