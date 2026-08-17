# YNAB CLI

**Every YNAB feature, plus a local mirror that makes reconciliation, payee trend analysis, and a ready-made ProjectionLab export instant instead of manual.**

This CLI mirrors your budget locally so bank reconciliation and payee spend history become instant, offline, agent-native commands instead of manual math against a live API and a 200-request-per-hour ceiling. It also ships a one-shot export shaped for downstream net-worth tools like ProjectionLab.

## Install

The recommended path installs both the `ynab-pp-cli` binary and the `pp-ynab` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ynab
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ynab --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ynab --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ynab --agent claude-code
npx -y @mvanhorn/printing-press-library install ynab --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ynab/cmd/ynab-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ynab-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ynab --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ynab --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ynab --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ynab --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ynab-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YNAB_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/ynab/cmd/ynab-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ynab": {
      "command": "ynab-pp-mcp",
      "env": {
        "YNAB_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Uses a YNAB Personal Access Token (Account Settings -> Developer Settings -> New Token in the YNAB web app). Set YNAB_API_TOKEN, or run auth login to store it in the OS keychain.

## Quick Start

```bash
# Verify the binary and config resolve before touching the network.
ynab-pp-cli doctor --dry-run

# Find your plan (budget) ID — everything else is scoped to it.
ynab-pp-cli plans list

# See current-month category balances at a glance.
ynab-pp-cli categories get 550e8400-e29b-41d4-a716-446655440000

# Get current balances shaped for a downstream net-worth tool.
ynab-pp-cli export balances --format projectionlab --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`export balances`** — One-shot export of current account balances reshaped into ProjectionLab's expected schema, ready to sync into a net-worth projection tool.

  _Use this to feed another finance tool's import step; it's the direct replacement for a hand-maintained account-mapping script._

  ```bash
  ynab-pp-cli export balances --format projectionlab --agent
  ```

### Local state that compounds
- **`accounts reconcile`** — Diff the local cleared-transaction total against a bank statement balance and surface the transaction(s) most likely responsible for the gap.

  _Use this monthly instead of eyeballing a statement line by line._

  ```bash
  ynab-pp-cli accounts reconcile a1b2c3d4 --statement-balance 1042.17 --agent
  ```
- **`payees profile`** — Aggregated monthly spend stats, average transaction size, and typical categories for a single payee.

  _Use this to understand a recurring payee's spend pattern rather than scanning raw transaction lists._

  ```bash
  ynab-pp-cli payees profile a1b2c3d4 --period 6m --agent
  ```

## Recipes

### Feed ProjectionLab

```bash
ynab-pp-cli export balances --format projectionlab --agent --select accounts.name,accounts.balance,accounts.type
```

Narrow the ProjectionLab-shaped export down to just the fields the downstream tool needs, keeping agent context small.

### Batch-approve pending transactions

```bash
ynab-pp-cli transactions get 550e8400-e29b-41d4-a716-446655440000 --type unapproved --json
```

List everything awaiting approval before approving in bulk.

### Reconcile against a bank statement

```bash
ynab-pp-cli accounts reconcile <account-id> --statement-balance 1042.17 --agent
```

Surfaces the exact transaction(s) causing a mismatch instead of scanning line by line.

### Understand a recurring payee

```bash
ynab-pp-cli payees profile <payee-id> --period 6m --agent
```

Shows monthly spend, average transaction size, and typical categories for one payee instead of scanning raw transactions.

## Usage

Run `ynab-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `YNAB_CONFIG_DIR`, `YNAB_DATA_DIR`, `YNAB_STATE_DIR`, or `YNAB_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `YNAB_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export YNAB_HOME=/srv/ynab
ynab-pp-cli doctor
```

Under `YNAB_HOME=/srv/ynab`, the four dirs resolve to `/srv/ynab/config`, `/srv/ynab/data`, `/srv/ynab/state`, and `/srv/ynab/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ynab": {
      "command": "ynab-pp-mcp",
      "env": {
        "YNAB_HOME": "/srv/ynab"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `YNAB_DATA_DIR` overrides an explicit `--home` for that kind. Use `YNAB_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `YNAB_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `ynab-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

The accounts for a plan. Every transaction belongs to an account, and an account is either "on budget", where its activity is categorized and planned, or a tracking account, where only its balance is tracked (see the on_budget flag).

- **`ynab-pp-cli accounts create`** - Creates a new account
- **`ynab-pp-cli accounts get`** - Returns all accounts
- **`ynab-pp-cli accounts get-by-id`** - Returns a single account

### categories

The categories for a plan, organized into category groups. Category amounts (assigned, activity, available) are specific to a month, and can be read and updated through the month-scoped category endpoints.

- **`ynab-pp-cli categories create-category`** - Creates a new category
- **`ynab-pp-cli categories get`** - Returns all categories grouped by category group.  Amounts (assigned, activity, available, etc.) are specific to the current plan month (UTC).
- **`ynab-pp-cli categories get-category-by-id`** - Returns a single category.  Amounts (assigned, activity, available, etc.) are specific to the current plan month (UTC).
- **`ynab-pp-cli categories update-category`** - Update a category

### category-groups

Manage category groups

- **`ynab-pp-cli category-groups create`** - Creates a new category group
- **`ynab-pp-cli category-groups update`** - Update a category group

### money-movement-groups

Manage money movement groups

- **`ynab-pp-cli money-movement-groups <plan_id>`** - Returns all money movement groups

### money-movements

Money movements record money moved between two categories, or between a category and Ready to Assign, within a plan month. Movements performed together as a single action are linked by a money movement group.

- **`ynab-pp-cli money-movements <plan_id>`** - Returns all money movements

### months

Each plan contains one or more months, which is where Ready to Assign, Age of Money and category (assigned / activity / available) amounts are available.

- **`ynab-pp-cli months get-plan`** - Returns all plan months
- **`ynab-pp-cli months get-plan-plans`** - Returns a single plan month

### payee-locations

When you enter a transaction and specify a payee on the YNAB mobile apps, the GPS coordinates for that location are stored, with your permission, so that the next time you are in the same place (like the Grocery store) we can pre-populate nearby payees for you!  It’s handy and saves you time. This resource makes these locations available.  Locations will not be available for all payees.

- **`ynab-pp-cli payee-locations get`** - Returns all payee locations
- **`ynab-pp-cli payee-locations get-by-id`** - Returns a single payee location

### payees

The payees for a plan. Transfers between accounts are represented with special transfer payees; a payee with transfer_account_id set is the transfer payee for that account.

- **`ynab-pp-cli payees create`** - Creates a new payee
- **`ynab-pp-cli payees get`** - Returns all payees
- **`ynab-pp-cli payees get-by-id`** - Returns a single payee
- **`ynab-pp-cli payees update`** - Update a payee

### plans

Plans are the top-level container for your YNAB data; accounts, categories, payees, and transactions all belong to a plan. Most endpoints require a plan_id, which can be obtained from "Get all plans". Alternatively, "last-used" can be used in place of a plan_id to specify the most recently used plan, and "default" can be used if [default plan selection](https://api.ynab.com/#oauth-default-plan) is enabled.

- **`ynab-pp-cli plans get`** - Returns plans list with summary information
- **`ynab-pp-cli plans get-by-id`** - Returns a single plan with all related entities.  This resource is effectively a full plan export.

### scheduled-transactions

The scheduled transactions for a plan; upcoming and recurring transactions that have not yet been entered into an account. Each has a frequency along with the first and next dates it will occur (date_first and date_next).

- **`ynab-pp-cli scheduled-transactions create`** - Creates a single scheduled transaction (a transaction with a future date).
- **`ynab-pp-cli scheduled-transactions delete`** - Deletes a scheduled transaction
- **`ynab-pp-cli scheduled-transactions get`** - Returns all scheduled transactions
- **`ynab-pp-cli scheduled-transactions get-by-id`** - Returns a single scheduled transaction
- **`ynab-pp-cli scheduled-transactions update`** - Updates a single scheduled transaction

### settings

Manage settings

- **`ynab-pp-cli settings <plan_id>`** - Returns settings for a plan

### transactions

The transactions for a plan. Transaction amounts are specified in [milliunits format](https://api.ynab.com/#formats). Split transactions are represented with subtransactions, and transfers between accounts are represented with transfer payees.

- **`ynab-pp-cli transactions create`** - Creates a single transaction or multiple transactions.  If you provide a body containing a `transaction` object, a single transaction will be created and if you provide a body containing a `transactions` array, multiple transactions will be created.  Scheduled transactions (transactions with a future date) cannot be created on this endpoint.
- **`ynab-pp-cli transactions delete`** - Deletes a transaction
- **`ynab-pp-cli transactions get`** - Returns plan transactions, excluding any pending transactions
- **`ynab-pp-cli transactions get-by-id`** - Returns a single transaction
- **`ynab-pp-cli transactions import`** - Imports available transactions on all linked accounts for the given plan.  Linked accounts allow transactions to be imported directly from a specified financial institution and this endpoint initiates that import.  Sending a request to this endpoint is the equivalent of clicking "Import" on each account in the web application or tapping the "New Transactions" banner in the mobile applications.  The response for this endpoint contains the transaction ids that have been imported.
- **`ynab-pp-cli transactions update`** - Updates multiple transactions, by `id` or `import_id`.
- **`ynab-pp-cli transactions update-plans`** - Updates a single transaction

### user

The currently authenticated user

- **`ynab-pp-cli user`** - Returns authenticated user information


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ynab-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ynab-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ynab-pp-cli learnings list`** - Inspect taught rows
- **`ynab-pp-cli learnings forget <query>`** - Undo a teach
- **`ynab-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ynab-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ynab-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ynab-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `YNAB_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ynab-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ynab-pp-cli accounts get mock-value

# JSON for scripting and agents
ynab-pp-cli accounts get mock-value --json
# Filter to specific fields
ynab-pp-cli accounts get mock-value --json --select data

# Dry run — show the request without sending
ynab-pp-cli accounts get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ynab-pp-cli accounts get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ynab-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `ynab-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ynab-endpoints-pp-cli/config.toml`; `--home`, `YNAB_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YNAB_API_TOKEN` | per_call | No | Set to your API credential. |
| `YNAB_ENDPOINTS_TOKEN` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ynab-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ynab-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YNAB_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Re-check YNAB_API_TOKEN or run 'ynab-pp-cli auth login' — tokens don't expire but can be revoked from YNAB's Developer Settings.
- **429 / rate limited** — YNAB caps tokens at 200 requests/hour. Narrow --since-date on transaction/list calls and avoid re-fetching the same data in a tight loop.
- **Amounts look 1000x too large or too small** — The raw API uses milliunits; this CLI converts to/from whole currency units at the boundary automatically — don't do the math yourself.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ynab-mcp-server**](https://github.com/calebl/ynab-mcp-server) — TypeScript (140 stars)
- [**cli-for-ynab**](https://github.com/borsboom/cli-for-ynab) — Rust (59 stars)
- [**stephendolan/ynab-cli**](https://github.com/stephendolan/ynab-cli) — TypeScript (43 stars)
- [**mcp-ynab**](https://github.com/klauern/mcp-ynab) — Python (6 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
