# Lunch Money CLI

**A Go CLI for Lunch Money's official v2 OpenAPI — offline SQLite store, subscription detection, and bulk smart retag the web UI cannot do.**

Built on Lunch Money's published v2 alpha spec for forward compatibility — every endpoint exposed as a Cobra subcommand with --json, --select dotted-paths, --csv, --dry-run, and typed exit codes. Local SQLite store powers offline search, history queries, and joins (net worth at any date, stale balance audit, duplicate detection, subscription detective) that single API calls can't answer. A companion `lunch-money-pp-mcp` binary ships alongside for Claude Desktop / Claude Code MCP integration.

Created by [@salmonumbrella](https://github.com/salmonumbrella) (salmonumbrella).

## Install

The recommended path installs both the `lunch-money-pp-cli` binary and the `pp-lunch-money` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lunch-money
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lunch-money --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lunch-money --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lunch-money --agent claude-code
npx -y @mvanhorn/printing-press-library install lunch-money --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/lunch-money/cmd/lunch-money-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lunch-money-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lunch-money --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lunch-money --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lunch-money --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lunch-money --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lunch-money-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LUNCHMONEY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lunch-money": {
      "command": "lunch-money-pp-mcp",
      "env": {
        "LUNCHMONEY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

**Public API** (`api.lunchmoney.dev/v2`) — covers every top-level command. Get a bearer token from https://my.lunchmoney.app/developers and set it as `LUNCHMONEY_API_KEY`:

```bash
export LUNCHMONEY_API_KEY="<your-token>"
```

Or save it to the config file (`~/.config/lunch-money-pp-cli/config.toml`):

```bash
lunch-money-pp-cli auth set-token <your-token>
```

The value IS the bearer token — the CLI prepends `Authorization: Bearer ` for you.

**Internal API** (`api.lunchmoney.app`, the web-UI backend) — only required if you use the `internal` subcommand tree, which exposes rules, Plaid taxonomy, and bulk-edit primitives that the public API doesn't have. Seed once with `lunch-money-pp-cli internal auth set-cookie '<paste browser Cookie: header>'`, or set `LUNCHMONEY_INTERNAL_COOKIE` for a non-persistent override. See `internal/internalapi/README.md` for full setup.

## Quick Start

```bash
# Paste your token from my.lunchmoney.app/developers; the CLI writes it to config and points doctor at the live API.
lunch-money-pp-cli auth set-token YOUR_TOKEN_HERE

# Confirm the token is valid, the API is reachable, and the local store is initialized — before any sync.
lunch-money-pp-cli doctor

# Full pull of categories, tags, accounts, recurring items, and transactions into the local SQLite store with rate-limit-aware backoff.
lunch-money-pp-cli sync

# See unreviewed transactions with suggested categories computed from your own historical categorizations.
lunch-money-pp-cli triage --limit 20

# Preview a bulk retag before letting the bulk PUT /transactions endpoint apply it.
lunch-money-pp-cli transactions retag --match '^AMZN' --category-id 73 --dry-run

# Mid-month per-category burn-down with end-of-period projection — pipe into an agent or dashboard.
lunch-money-pp-cli budgets burn --period 2026-05 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local-join transcendence
- **`transactions subscriptions`** — Find regular-cadence merchant charges that are not yet linked to a recurring item, so you can promote them before subscription creep sneaks past you.

  _When a user asks 'what subscriptions are draining my account?', this surfaces unlabeled recurring charges in one local SQL pass._

  ```bash
  lunch-money-pp-cli transactions subscriptions --suspected-only --json --select 'payee,suggested_cadence,avg_amount,occurrences'
  ```
- **`recurring missing`** — List recurring items whose next expected date falls in the requested month but have no matching transaction yet, so you can catch missed bills.

  _Answers 'which subscriptions haven't hit my account yet?' which is a top-three Maya question with no API equivalent._

  ```bash
  lunch-money-pp-cli recurring missing --month 2026-05 --json
  ```
- **`transactions duplicates`** — Detect near-duplicate transactions in a window (same merchant, amount within tolerance, date within range) so you can group or delete them in bulk.

  _Use after a trip or when reviewing imports to catch the same Uber ride processed twice._

  ```bash
  lunch-money-pp-cli transactions duplicates --window 3 --tolerance 1.00 --json --select 'cluster_id,payee,amount,date,id'
  ```

### Net-worth tools
- **`accounts stale`** — Surface manual accounts and manual crypto whose balance has not been updated in N days, so net worth stays honest.

  _Answers 'what manual balances do I owe an update?' before quarterly NAV updates or Sunday reconciliation._

  ```bash
  lunch-money-pp-cli accounts stale --over 30d --json --select 'account_type,account_name,last_updated,days_stale'
  ```
- **`net-worth on`** — Reconstruct net worth at any historical date by walking balance_history across every account type, with last-known-balance carry-forward where needed.

  _Use for quarterly snapshots, retirement-plan inputs, or 'what was my net worth before I bought the car?' questions._

  ```bash
  lunch-money-pp-cli net-worth on 2026-03-31 --json --select 'as_of_date,total,by_account_type'
  ```

### Bulk transaction tools
- **`transactions retag`** — Match transactions by regex on payee/notes via local FTS, preview the affected set, then bulk-apply tag/category changes via the API.

  _Answers 'fix every Amazon transaction since January' in one command instead of pagination clicks._

  ```bash
  lunch-money-pp-cli transactions retag --match '^AMZN' --category-id 73 --dry-run --json
  ```
- **`triage`** — Pull all unreviewed transactions, compute a suggested category from prior categorizations of the same merchant, and optionally mass-apply the suggestion.

  _Sunday-morning ritual for power-user budgeters; agent-friendly suggested categories let an LLM bulk-approve high-confidence rows._

  ```bash
  lunch-money-pp-cli triage --limit 20 --json --select 'id,payee,amount,suggested_category,confidence'
  ```

### Budget intelligence
- **`budgets burn`** — For the current budget period, project end-of-month spend per category at the current linear rate and flag categories projected to overshoot.

  _Mid-month answer to 'am I on track?' that flows into agent workflows and dashboards._

  ```bash
  lunch-money-pp-cli budgets burn --period 2026-05 --json --select 'category_name,target,spent,projected_end_spend,over_under'
  ```

### Session orientation
- **`changed`** — List everything created or edited in the local store since a cutoff (e.g. --since 8h, --since 2026-05-01). Use at session start to orient before taking action.

  _Agents starting a fresh session can ask 'what changed overnight?' in one call instead of paginating multiple list endpoints._

  ```bash
  lunch-money-pp-cli changed --since 8h --json
  ```

## Usage

Run `lunch-money-pp-cli --help` for the full command reference and flag list.

## Commands

### balance-history

View and update historical account balances. Balance history is what drives the [Net Worth](https://my.lunchmoney.app/net-worth) views in the Lunch Money app. Balance history is generated for each account's balance on the first day of each month and can be edited in the Lunch Money app or via the API.

- **`lunch-money-pp-cli balance-history delete-entry`** - Delete a single monthly balance history entry by its id.
- **`lunch-money-pp-cli balance-history delete-for-account`** - Delete all historical balance entries for a single manual, Plaid, manual crypto, or deleted account. Crypto synced accounts require an additional `symbol` path parameter.
- **`lunch-money-pp-cli balance-history delete-for-crypto-synced`** - Delete all historical balance entries for a single synced crypto symbol stream.<br><br>
The path identifies both the synced crypto account and the symbol whose history should be deleted.
- **`lunch-money-pp-cli balance-history get`** - Retrieve historical balance entries.<br><br>
Balance history is monthly. When `start_date` and `end_date` are both provided, they must be first-of-month dates. `start_date` must not be in the future, while `end_date` may be in the future. If one of `start_date` or `end_date` is provided, the other is required. If neither is provided, all available balance history is returned.<br><br>
The response groups entries by source account. Each item in `balance_history` contains a `source` object plus a `balances` array containing one balance entry per month in the requested range, or all stored entries when no range is provided.<br><br>
Historical entries for accounts that have been deleted may still be returned. These entries use `source.type: deleted` and include `deleted_account_id`, archived display fields, and account metadata on the `source` object.
- **`lunch-money-pp-cli balance-history get-for-account`** - Retrieve historical balance entries for one manual, Plaid, manual crypto, or deleted account.  Crypto synced accounts require an additional `symbol` path parameter.<br><br>
The `account_type` path parameter identifies the type of account and the `account_id` path parameter identifies the specific id for that account type.<br><br>
When `start_date` and `end_date` are both provided, they must be first-of-month dates. `start_date` must not be in the future, while `end_date` may be in the future. If one of `start_date` or `end_date` is provided, the other is required. If neither is provided, all available history for the source is returned.
- **`lunch-money-pp-cli balance-history get-for-crypto-synced`** - Retrieve historical balance entries for a single synced crypto symbol stream.<br><br>
Use the `crypto_synced` account id together with a `symbol` path parameter to select one balance stream within that synced crypto account.<br><br>
When `start_date` and `end_date` are both provided, they must be first-of-month dates. `start_date` must not be in the future, while `end_date` may be in the future. If one of `start_date` or `end_date` is provided, the other is required. If neither is provided, all available history for the symbol stream is returned.
- **`lunch-money-pp-cli balance-history update-details`** - Update archived metadata for a deleted balance history source.<br><br>
Pass the `deleted` source id returned on `source.deleted_account_id`. This endpoint updates the stored deleted-source metadata used for all historical entries associated with that deleted source.
- **`lunch-money-pp-cli balance-history upsert-for-account`** - Upsert one or more historical balance entries for a single manual, Plaid, manual crypto, or deleted acount. Crypto synced accounts require an additional `symbol` path parameter.<br><br>
The `account_type` path parameter identifies the the type of account and the `account_id` path parameter identifies the specific id for that account type.<br><br>
Submit one or more entries in the `balances` array. Each entry must specify a `date` and `balance` value.<br><br>
Balance history is monthly. Each entry's `date` must be the first day of a month and must not be in the future.<br><br>
`currency` may be provided for any balance entry. If omitted, it defaults to the account currency for manual/Plaid accounts, or the user's primary currency for crypto/deleted accounts.<br><br>
`symbol` is optional for `crypto_manual` accounts, tolerated for `deleted` accounts, and invalid for `manual` or `plaid` accounts.<br><br>
`crypto_balance` may be provided for `crypto_manual` and `deleted` accounts, and is invalid for `manual` or `plaid` accounts.<br><br>
Use `PUT /v2/balance_history/deleted/{account_id}/details` to update deleted-source metadata.
- **`lunch-money-pp-cli balance-history upsert-for-crypto-synced`** - Upsert one or more historical balance entries for a single synced crypto symbol stream.<br><br>
The path identifies both the synced crypto account and the symbol being updated.<br><br>
Submit one or more entries in the `balances` array. Each entry must specify a `date` and `balance` value.<br><br>
Balance history is monthly. Each entry's `date` must be the first day of a month and must not be in the future.<br><br>
The request body must not include `symbol`; the symbol is supplied by the path.<br><br>
`currency` may be provided for any balance entry. If omitted, it defaults to the user's primary currency for synced crypto balances.<br><br>
`crypto_balance` may be provided for synced crypto balances.

### budgets

View settings and modify budget amounts.<p> Use the [/summary](#tag/summary) endpoint to view the budget details performance.

- **`lunch-money-pp-cli budgets delete`** - Removes the budget for the given category and period. If there already is no budget set for that period, the request still succeeds (idempotent).<p> Note that `start_date` **must** be a valid budget period start for the account (based on the account's budget period settings). If an invalid `start_date` is provided, the request will fail with an error that indicates what the previous and next valid start dates are.<p> Use the [budgets/settings](#tag/budgets/GET/budgets/settings) endpoint to view the account's budget settings.<br> To view details for existing budgets, use the [summary](#tag/summary) endpoint.
- **`lunch-money-pp-cli budgets get-settings`** - Returns the budget-related settings for the user's account.
- **`lunch-money-pp-cli budgets upsert`** - Create or update a budget for a category and period.<p>
If a budget already exists for the specified `start_date` and `category_id`, the `amount` (and optional `currency` and `notes`) are updated; otherwise a new budget entry is created.<p>

Note that `start_date` **must** be a valid budget period start for the account (based on the account's
budget period settings). If an invalid `start_date` is provided, the request will fail with an error that indicates what the previous and next valid start dates are.<p>

Use the [budgets/settings](#tag/budgets/GET/budgets/settings) endpoint to view the account's budget settings.<br>
To view details for existing budgets, use the [summary](#tag/summary) endpoint.

### categories

Work with categories

- **`lunch-money-pp-cli categories create-category`** - Creates a new category with the given name.<br> If the `is_group` attribute is set to true, a category group is created. In this case, the `children` attribute may be set to an array of category IDs to add to the newly created category group.
- **`lunch-money-pp-cli categories delete-category`** - Attempts to delete the single category or category group specified on the path. By default, this will only work if there are no dependencies, such as existing budgets for the category, categorized transactions, child categories for a category group, categorized recurring items, etc. If there are dependents, this endpoint will return an object that describes the number and type of existing dependencies.
- **`lunch-money-pp-cli categories get-all`** - Retrieve a list of all categories associated with the user's account.
- **`lunch-money-pp-cli categories get-category-by-id`** - Retrieve details of a specific category or category group by its ID.
- **`lunch-money-pp-cli categories update-category`** - Modifies the properties of an existing category or category group.<br><br>
You may submit the response from a `GET /categories/{id}` as the request body; however, only certain properties can be updated using this API. The following properties are accepted in the request body but their values will be ignored: `id`, `is_group`, `updated_at`, `created_at`, and `order`.<br><br>
It is also possible to provide only the properties to be updated in the request body, as long as the request includes at least one of the properties that is not listed above. For example, a request body that contains only a `name` property is valid.<br><br>
It is not possible to use this API to convert a category to a category group, or vice versa, so while submitting a request body with the `is_group` property is tolerated, it will result in an error response if the value is changed.<br><br>
It is possible to modify the children of an existing category group with this API by setting the `children` attribute. If this is set, it will replace the existing children with the newly specified children. If the intention is to add or remove a single category, it is more straightforward to update the child category by specifying the new `group_id` attribute. If the goal is to add multiple new children or remove multiple existing children, it is recommended to first call the `GET /categories/{id}` endpoint to get the existing children and then modify the list as desired.<br><br>

### crypto

Manage crypto

- **`lunch-money-pp-cli crypto create-manual`** - Create a manually managed crypto asset.<br><br>
If `display_name` is `null`, clients may derive one from `institution_name` + `name`.
- **`lunch-money-pp-cli crypto delete-manual`** - Delete a single manually managed crypto asset by ID.<p> If this crypto asset has a balance history, and you do not explicitly set the query parameter`keep_history`, a 422 response will be returned requesting you to explicitly set `keep_history` to `true` or `false`.
- **`lunch-money-pp-cli crypto get-all-manual`** - Retrieve all manually managed crypto balances associated with the user's account.
- **`lunch-money-pp-cli crypto get-all-synced`** - Retrieves all synced crypto accounts associated with the user's account.
- **`lunch-money-pp-cli crypto get-manual-by-id`** - Retrieve a single manually managed crypto balance by ID.
- **`lunch-money-pp-cli crypto get-synced-balance-by-symbol`** - Retrieves a single balance from the specified synced crypto account using the crypto symbol.
- **`lunch-money-pp-cli crypto get-synced-by-id`** - Retrieves the synced crypto account and all nested balances for the specified synced crypto account ID.
- **`lunch-money-pp-cli crypto refresh-synced`** - Trigger a balance refresh for the specified synced crypto account. Returns the refreshed synced crypto account.
- **`lunch-money-pp-cli crypto update-manual`** - Modify a manually managed crypto balance.<br><br>
You may submit the response from `GET /crypto/manual/{id}` as the request body. System-defined properties are accepted according to the `x-updatable` metadata in the update schema.

### cryptocurrencies

Manage cryptocurrencies

- **`lunch-money-pp-cli cryptocurrencies create-cryptocurrency`** - Adds a new cryptocurrency to the supported manual-crypto list.<br><br>
Lunch Money uses [CoinGecko](https://www.coingecko.com/us/coins/ethereum) to convert crypto balances to the user's primary currency. Users add a new supported cryptocurrency by submitting a CoinGecko coin-page URL. The server validates the URL, extracts the id from `/coins/{id}`, checks for an existing supported `coingecko_id`, validates the id against CoinGecko, then confirms the resolved symbol is not already supported before creating the new entry.
- **`lunch-money-pp-cli cryptocurrencies get-all`** - Retrieve the list of cryptocurrencies currently supported for manual tracking.<p>
When creating a new manual crypto balance via `POST /crypto/manual`, the `symbol` you specify must match the `symbol` of one of the entries returned by this endpoint.

### manual-accounts

Work with manually managed accounts (formerly called assets)

- **`lunch-money-pp-cli manual-accounts create`** - Create a new manually-managed account.
- **`lunch-money-pp-cli manual-accounts delete`** - Deletes the single manual account with the ID specified on the path. If any transactions exist with the `manual_account_id` property set to this account's ID they will appear with a warning when displayed in the web view.
- **`lunch-money-pp-cli manual-accounts get-all`** - Retrieve a list of all manually-managed accounts associated with the user's account.
- **`lunch-money-pp-cli manual-accounts get-by-id`** - Retrieve the details of the manual account with the specified ID.
- **`lunch-money-pp-cli manual-accounts update`** - Modifies the properties of an existing manual account.<br><br>
You may submit the response from a `GET /manual_accounts/{id}` as the request body, however only certain properties can be updated using this API. The following system set properties are accepted in the request body but their values will be ignored: `id`, `to_base`, `created_at`, and `updated_at`.<br><br>
It is also possible to provide only the properties to be updated in the request body, as long as the request includes at least one of the properties that is not listed above. For example a request body that contains only a `name` property is valid.<br><br>

### me

View details and settings for the current user and account

- **`lunch-money-pp-cli me get`** - Get details about the user associated with the supplied authorization token.

### plaid-accounts

Work with accounts synced through Plaid

- **`lunch-money-pp-cli plaid-accounts get-all`** - Retrieve a list of all synced accounts associated with the user's account.
- **`lunch-money-pp-cli plaid-accounts get-by-id`** - Retrieve the details of the plaid account with the specified ID.
- **`lunch-money-pp-cli plaid-accounts trigger-fetch`** - Use this endpoint to trigger a fetch for latest data from Plaid.<br><br>
Eligible accounts are those who last_fetch value is over 1 minute ago. (Although the limit is every minute, please use this endpoint sparingly!) Successive calls to this endpoint under a minute after the first will return a 425 TOO EARLY response.<br><br>
Successful calls will return a 202 ACCEPTED response. Note that fetching from Plaid is a background job. This endpoint simply queues up the job. You may track the `plaid_last_successful_update`, `last_fetch` and `last_import` properties to verify the results of the fetch. The `last fetch` property is updated when Plaid accepts a request to fetch data. The `plaid_last_successful_update`is updated when it successfully contacts the associated financial institution. The `last_import` field is updated only when new transactions have been imported.

### recurring-items

Work with recurring items

- **`lunch-money-pp-cli recurring-items get-all-recurring`** - Retrieve recurring items for a specified time frame.
- **`lunch-money-pp-cli recurring-items get-recurring-by-id`** - Retrieve the details of a specific recurring item with the specified ID.

### summary

View a summary of the user's budget

- **`lunch-money-pp-cli summary get-budget`** - Retrieves a summary of the user's budget. Use this endpoint to access budget configuration details and performance for a specified date range.<p> Use the [/budgets](#tag/budgets) endpoint to manage budget objects.

### tags

Work with tags

- **`lunch-money-pp-cli tags create`** - Creates a new tag with the given name
- **`lunch-money-pp-cli tags delete`** - Deletes the tag with the ID specified on the path.<br>
If transactions or rules exist with the tag, a dependents object is returned and the tag is not deleted. This behavior can be overridden by setting the `force` parameter to `true`.
- **`lunch-money-pp-cli tags get-all`** - Retrieve a list of all tags associated with the user's account.
- **`lunch-money-pp-cli tags get-by-id`** - Retrieve the details of a specific tag with the specified ID.
- **`lunch-money-pp-cli tags update`** - Updates an existing tag.<br><br>
You may submit the response from a `GET /tags/{id}` as the request body; however, only certain properties can be updated using this API. The following system set properties are accepted in the request body but their values will be ignored: `id`, `updated_at`, and `created_at`.<br><br>
It is also possible to provide only the properties to be updated in the request body, as long as the request includes at least one of the properties that is not listed above. For example, a request body that contains only a `name` attribute is valid.

### transactions

Work with transactions

- **`lunch-money-pp-cli transactions create-new`** - Use this endpoint to add transactions to a budget.<p>
The request body for this endpoint must include a list of transactions with at least one transaction and not more than 500 transactions to insert.<p>
The successful request to this endpoint will return a response body which will include two arrays:<br>  - `transactions`: A list of transactions that were successfully inserted.<br> - `skipped_duplicates`: A list of transactions that were duplicates of existing transactions and were not inserted.
- **`lunch-money-pp-cli transactions delete`** - Deletes the transaction with the IDs specified in the request body.<p>
If any of the specified transactions are a split transaction or a split parent, or if any are a grouped transactions or part of a transaction group, the request will fail with a suggestion on how to unsplit or ungroup the transaction(s) prior to deletion. This will also fail if any of the specified transaction IDs do not exist.<p>
Otherwise, the specified transactions are deleted.<p>
<span class="red-text"><strong>Use with caution. This action is not reversible!</strong></span>
- **`lunch-money-pp-cli transactions delete-attachment`** - Deletes a file attachment from a transaction.
- **`lunch-money-pp-cli transactions delete-by-id`** - Deletes the transaction with the ID specified on the path.<p>
If the specified transaction is a split transaction or a split parent, or if it is a grouped transactions or part of a transaction group, the request will fail with a suggestion on how to unsplit or ungroup the transaction(s) prior to deletion. Otherwise, the specified transaction is deleted. <p>
<span class="red-text"><strong>Use with caution. This action is not reversible!</strong></span>
- **`lunch-money-pp-cli transactions get-all`** - Retrieve a list of all transactions associated with a user's account. <br>If called with no parameters, this endpoint will return the most recent transactions, up to the specified `limit`.
- **`lunch-money-pp-cli transactions get-attachment-url`** - Returns a signed url that can be used to download the file attachment.
- **`lunch-money-pp-cli transactions get-by-id`** - Retrieves the details of a specific transaction by its ID, including the following properties which are not returned by default in the response to a `GET /transactions` request:<br>

- `plaid_metadata` will either be `null` or contain the metadata for transactions associated with an account that is synced via plaid. 
- `custom_metadata` will either be `null` or contain any custom_metadata added to transactions that were inserted or updated via the API.
- `files` will be a list of objects that describe any attachments to the transaction.

If `is_group_parent` is true in the returned transaction, the object will also include the `children` property which will contain a list of the  original transactions that make up the transaction group.<br>
If `is_split_parent` is true in the returned transaction, the object will also include the `children` property which will contain a list of the split transactions.
- **`lunch-money-pp-cli transactions group`** - Specify a set of existing transaction IDs to group together as a single grouped transaction. 
The new transaction will have an amount equal to the sum of the grouped transaction amounts. If the 
grouped transactions have different currencies, the new group transaction will be set in the user's
default currency.<br><br> 
After a transaction has been grouped, the original transactions are no longer shown on the 
transactions page or returned by a `GET /transactions` request. The newly created grouped 
transaction is returned instead.

To see the details of the original transactions that were used to create a transaction group, use the
`GET /transactions/{id}` endpoint and pass the ID of the grouped transaction. The grouped transactions will
be included in the `children` property of the transaction returned in the response
- **`lunch-money-pp-cli transactions split`** - Splits an existing transaction into a set of smaller child transactions.<br><br> After a transaction has been split, the original transaction is no longer shown on the transactions page or returned by a `GET /transactions` request. The newly created child transactions are returned instead.
To see the details of the original parent transaction after it has been split, use the `GET /transactions/{id}` endpoint and pass the value of the `split_parent_id` of one of the children.
- **`lunch-money-pp-cli transactions ungroup`** - Deletes the transaction group with the ID specified on the path.<br>
The transactions within the group are not removed and will subsequently be treated as "normal" ungrouped transactions.
- **`lunch-money-pp-cli transactions unsplit`** - Deletes the split children of a previously split transactions and restores the parent transactions to the normal unsplit state.<br><br>
Use the value of the `split_parent_id`property of a split transaction to specify the parent ID.
- **`lunch-money-pp-cli transactions update`** - Modifies the properties of multiple existing transactions in a single request.<br><br>
You may submit complete transaction objects from the response returned by a `GET /transactions` in the request body for each transaction, however only certain properties can be updated using this API. The following system set properties are accepted in the request body, but their values will be ignored: `id`, `to_base`, `is_pending`, `created_at`, `updated_at`, `source`, and `plaid_metadata`.<br><br>
Transactions that have been previously split or grouped may not be modified by this endpoint. Therefore the `is_split_parent`, `split_parent_id`, `is_group_parent`, `group_parent_id`, and `children` properties are also ignored when provided in the request body.<br><br>
Each transaction in the array **must** include an `id` property to identify which transaction to update, along with at least one other property to be updated. For example, a transaction object that contains only an `id` and `category_id` property is valid.<br><br>
The request can include between 1 and 500 transactions to update in a single call.
- **`lunch-money-pp-cli transactions update-id`** - Modifies the properties of an existing transaction.<br><br>
You may submit the response from a `GET /transactions/{id}` as the request body, however only certain properties can be updated using this API. The following system set properties are accepted in the request body but their values will be ignored: `id`, `to_base`, `is_pending`, `created_at`, `updated_at`, `source`, and `plaid_metadata`.<br><br>
Transactions that have been previously split or grouped may not be modified by this endpoint. Therefore the `is_split_parent`, `split_parent_id`, `is_group_parent`, `group_parent_id`, and `children` properties are also ignored when provided in the request body.<br><br>
It is also possible to provide only the properties to be updated in the request body, as long as the request includes at least one of the properties that is not listed above. For example a request body that contains only an `category_id` attribute is valid.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lunch-money-pp-cli balance-history get

# JSON for scripting and agents
lunch-money-pp-cli balance-history get --json

# Filter to specific fields
lunch-money-pp-cli balance-history get --json --select id,name,status

# Dry run — show the request without sending
lunch-money-pp-cli balance-history get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lunch-money-pp-cli balance-history get --agent
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

## Health Check

```bash
lunch-money-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/lunch-money-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LUNCHMONEY_API_KEY` | per_call | Yes (public API) | Bearer token from https://my.lunchmoney.app/developers. Used by every top-level command. |
| `LUNCHMONEY_INTERNAL_COOKIE` | per_call | Only for `internal` subcommands | Raw `Cookie:` header for `api.lunchmoney.app` (the web-UI backend). Non-persistent override of the cookie jar; needed only for rules, Plaid taxonomy, and the other `internal` commands. |

## Known Gaps

The CLI is built against Lunch Money OpenAPI spec **v2.11.0-preview.2** (the latest published spec). Per the spec preamble, "the latest **complete** implementation of the spec is for the v2.8.5 release" — the live API at `api.lunchmoney.dev/v2` has not yet deployed several alpha endpoints:

| Endpoint family | Spec version | Status today |
|-----------------|--------------|--------------|
| `/balance_history` (all operations) | v2.11.0 | `Cannot GET /v2/balance_history` (404) — alpha, not deployed |
| `/crypto/manual` (all operations) | v2.10.0 | 404 — alpha, not deployed |
| `/crypto/synced` (all operations) | v2.10.0 | 404 — alpha, not deployed |
| `/cryptocurrencies` | v2.10.0 | 404 — alpha, not deployed |

These commands exist in the CLI (`balance-history get`, `crypto get-all-manual`, `crypto get-all-synced`, `cryptocurrencies get-all`, plus their CRUD siblings) and will start working when the Lunch Money API server catches up to v2.10/v2.11. No CLI changes will be needed. Novel features that depend on these (`net-worth on`, `accounts stale`) will be more useful once the balance_history endpoints are live; today they work against the local store only.

Lunch Money has stated the v2 API will reach GA "early 2026" — track progress at https://lm-v2-api-next-a7fabcab8e9a.herokuapp.com/v2/version-history.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `lunch-money-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LUNCHMONEY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — Run `lunch-money-pp-cli auth status` to see whether the env var or config file is in scope; if neither, paste your token via `lunch-money-pp-cli auth set-token` or `export LUNCHMONEY_API_KEY`.
- **Rate limit 429 with retry-after** — Built-in backoff respects x-ratelimit-remaining and x-ratelimit-reset; if you hit a hard ceiling, reduce --limit on transactions list or stagger sync calls.
- **Stale data in local store** — Run `lunch-money-pp-cli sync` to pull only changes since last cursor (uses updated_since on transactions); pass --full to rebuild from scratch.
- **Plaid balance out of date** — Run `lunch-money-pp-cli plaid-accounts trigger-fetch` to trigger the Lunch Money backend Plaid fetch; rerun sync after a minute to pull new transactions.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**awesome-lunchmoney**](https://github.com/lunch-money/awesome-lunchmoney) — Markdown (71 stars)
- [**lunchable**](https://github.com/juftin/lunchable) — Python (52 stars)
- [**lunchtui**](https://github.com/Rshep3087/lunchtui) — Go (23 stars)
- [**lunchmoney (icco)**](https://github.com/icco/lunchmoney) — Go (10 stars)
- [**Lunch Money JS v2**](https://github.com/lunch-money/lunch-money-js-v2) — TypeScript (1 stars)
- [**lunchmoney-mcp-v2**](https://github.com/leafeye/lunchmoney-mcp-v2) — Python
- [**lunchmoney-mcp**](https://github.com/robshox/lunchmoney-mcp) — TypeScript
- [**akutishevsky lunchmoney-mcp**](https://github.com/akutishevsky/lunchmoney-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
