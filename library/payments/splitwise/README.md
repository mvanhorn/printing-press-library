# Splitwise CLI

**Every Splitwise feature, plus an offline SQLite ledger that powers balance, debt-aging, spend analytics, and full-text search no other Splitwise tool has.**

splitwise-pp-cli wraps the full Splitwise API — expenses, groups, friends, comments, settle-ups — and keeps a local copy of your whole ledger. That local store powers a net `balances` view, `debts --aged` (who never pays you back), `spend` rollups by category or month, offline `search`, a group `ledger` with running balances, and a `settle-up` plan that minimizes transfers. Fuzzy name resolution means you never paste a numeric ID.

## Install

The recommended path installs both the `splitwise-pp-cli` binary and the `pp-splitwise` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install splitwise
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install splitwise --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install splitwise --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install splitwise --agent claude-code
npx -y @mvanhorn/printing-press-library install splitwise --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/splitwise-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-splitwise --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-splitwise --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-splitwise skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-splitwise. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/splitwise-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SPLITWISE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "splitwise": {
      "command": "splitwise-pp-mcp",
      "env": {
        "SPLITWISE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Splitwise authenticates with a personal API key used as an HTTP Bearer token. Register an app at https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY. OAuth 2.0 (authorization-code) is also supported for multi-user apps, but a personal API key is the fastest path for a power-user CLI.

## Quick Start

```bash
# Confirm the binary, config path, and verify state without needing credentials.
splitwise-pp-cli doctor --dry-run

# Pull your groups, friends, expenses, comments, categories, and currencies into the local store.
splitwise-pp-cli sync

# See your net position across every friend and group at a glance.
splitwise-pp-cli balances

# Roll up your shared spend by category from synced history.
splitwise-pp-cli spend --group-by category

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Balances at a glance
- **`balances`** — See everything you owe and are owed across every friend and group in one net-position view.

  _Reach for this instead of N get_groups + get_friends calls when an agent needs the user's overall money position._

  ```bash
  splitwise-pp-cli balances --agent
  ```
- **`debts`** — List who owes you (and whom you owe) sorted by how long the balance has gone unsettled.

  _Use when the task is 'who never pays me back' or chasing stale IOUs._

  ```bash
  splitwise-pp-cli debts --aged --agent
  ```
- **`ledger`** — Every expense in a group, in date order, with a cumulative running balance per member.

  _Use to audit how a group's balances got to where they are, not just the snapshot._

  ```bash
  splitwise-pp-cli ledger "Tahoe Trip" --agent
  ```

### Offline spend intelligence
- **`spend`** — Total shared spend broken down by category, group, or month from your synced history.

  _Use for any 'how much did we spend on X' question instead of paging the whole expense list._

  ```bash
  splitwise-pp-cli spend --group-by category --agent
  ```
- **`search`** — Full-text search across your entire expense history, comments, and group/friend names — offline.

  _Use to find a specific past expense by keyword without paging the API._

  ```bash
  splitwise-pp-cli search "ramen" --agent
  ```
- **`recurring`** — Surface repeating charges (rent, utilities, subscriptions) from your synced history and flag a month missing an expected entry.

  _Use to catch a shared monthly bill nobody remembered to log this cycle._

  ```bash
  splitwise-pp-cli recurring --agent
  ```

### Reconcile and settle
- **`settle-up`** — Compute the minimum set of transfers that zeroes out balances in a group, then optionally record the payments.

  _Use when a group wants the fewest Venmo transfers to get everyone to zero._

  ```bash
  splitwise-pp-cli settle-up "Tahoe Trip" --agent
  ```
- **`activity`** — Show what changed since your last sync — new, edited, and deleted expenses to review.

  _Use to reconcile recent account activity before settling or reporting._

  ```bash
  splitwise-pp-cli activity --agent
  ```
- **`split`** — Build and preview the exact expense split (equal, exact, percentage, or shares) before recording it.

  _Reach for this to turn 'I paid $84, split equally with the trip' into a ready-to-record expense without hand-building the share arrays. Add --record to submit it._

  ```bash
  splitwise-pp-cli split "Tahoe Trip" --amount 84 --equal --agent
  ```

## Recipes


### Net position for an agent

```bash
splitwise-pp-cli balances --agent --select by_currency
```

Returns just the headline numbers an agent needs to report the user's overall money position.

### Inspect a group's members and debts (narrow a verbose payload)

```bash
splitwise-pp-cli get-groups --agent --select groups.name,groups.members.first_name,groups.simplified_debts.amount
```

get-groups returns deeply nested members + balance arrays; --select keeps only the fields you need so an agent doesn't burn context on the full payload.

### Find a forgotten expense

```bash
splitwise-pp-cli search "airbnb" --limit 10
```

Full-text search across your synced expense history for a keyword.

### Plan the fewest transfers to settle a trip

```bash
splitwise-pp-cli settle-up "Tahoe Trip"
```

Prints the minimum-transfer settle-up plan; add --record to create the payment expenses.

## Usage

Run `splitwise-pp-cli --help` for the full command reference and flag list.

## Commands

### add-user-to-group

Manage add user to group

- **`splitwise-pp-cli add-user-to-group`** - **Note**: 200 OK does not indicate a successful response. You must check the `success` value of the response.

### create-comment

Manage create comment

- **`splitwise-pp-cli create-comment`** - Create a comment

### create-expense

Manage create expense

- **`splitwise-pp-cli create-expense`** - Creates an expense. You may either split an expense equally (only with `group_id` provided),
or supply a list of shares.

When splitting equally, the authenticated user is assumed to be the payer.

When providing a list of shares, each share must include `paid_share` and `owed_share`, and must be identified by one of the following:
- `email`, `first_name`, and `last_name`
- `user_id`

**Note**: 200 OK does not indicate a successful response. The operation was successful only if `errors` is empty.

### create-friend

Manage create friend

- **`splitwise-pp-cli create-friend`** - Adds a friend. If the other user does not exist, you must supply `user_first_name`.
If the other user exists, `user_first_name` and `user_last_name` will be ignored.

### create-friends

Manage create friends

- **`splitwise-pp-cli create-friends`** - Add multiple friends at once.

For each user, if the other user does not exist, you must supply `users__{index}__first_name`.

**Note**: user parameters must be flattened into the format `users__{index}__{property}`, where
`property` is `first_name`, `last_name`, or `email`.

### create-group

Manage create group

- **`splitwise-pp-cli create-group`** - Creates a new group. Adds the current user to the group by default.

**Note**: group user parameters must be flattened into the format `users__{index}__{property}`, where
`property` is `user_id`, `first_name`, `last_name`, or `email`.
The user's email or ID must be provided.

### delete-comment

Manage delete comment

- **`splitwise-pp-cli delete-comment <id>`** - Deletes a comment. Returns the deleted comment.

### delete-expense

Manage delete expense

- **`splitwise-pp-cli delete-expense <id>`** - **Note**: 200 OK does not indicate a successful response. The operation was successful only if `success` is true.

### delete-friend

Manage delete friend

- **`splitwise-pp-cli delete-friend <id>`** - Given a friend ID, break off the friendship between the current user and the specified user.

**Note**: 200 OK does not indicate a successful response. You must check the `success` value of the response.

### delete-group

Manage delete group

- **`splitwise-pp-cli delete-group <id>`** - Delete an existing group. Destroys all associated records (expenses, etc.)

### get-categories

Manage get categories

- **`splitwise-pp-cli get-categories`** - Returns a list of all categories Splitwise allows for expenses. There are parent categories that represent groups of categories with subcategories for more specific categorization.
When creating expenses, you must use a subcategory, not a parent category.
If you intend for an expense to be represented by the parent category and nothing more specific, please use the "Other" subcategory.

### get-comments

Manage get comments

- **`splitwise-pp-cli get-comments`** - Get expense comments

### get-currencies

Manage get currencies

- **`splitwise-pp-cli get-currencies`** - Returns a list of all currencies allowed by the system. These are mostly ISO 4217 codes, but we do
sometimes use pending codes or unofficial, colloquial codes (like BTC instead of XBT for Bitcoin).

### get-current-user

Manage get current user

- **`splitwise-pp-cli get-current-user`** - Get information about the current user

### get-expense

Manage get expense

- **`splitwise-pp-cli get-expense <id>`** - Get expense information

### get-expenses

Manage get expenses

- **`splitwise-pp-cli get-expenses`** - List the current user's expenses

### get-friend

Manage get friend

- **`splitwise-pp-cli get-friend <id>`** - Get details about a friend

### get-friends

Manage get friends

- **`splitwise-pp-cli get-friends`** - **Note**: `group` objects only include group balances with that friend.

### get-group

Manage get group

- **`splitwise-pp-cli get-group <id>`** - Get information about a group

### get-groups

Manage get groups

- **`splitwise-pp-cli get-groups`** - **Note**: Expenses that are not associated with a group are listed in a group with ID 0.

### get-notifications

Manage get notifications

- **`splitwise-pp-cli get-notifications`** - Return a list of recent activity on the users account with the most recent items first.
`content` will be suitable for display in HTML and uses only the `<strong>`, `<strike>`, `<small>`,
`<br>` and `<font color="#FFEE44">` tags.

The `type` value indicates what the notification is about. Notification types may be added in the future
without warning. Below is an incomplete list of notification types.

| Type | Meaning |
| ---- | ------- |
| 0    | Expense added |
| 1    | Expense updated |
| 2	   | Expense deleted |
| 3	   | Comment added |
| 4	   | Added to group |
| 5	   | Removed from group |
| 6	   | Group deleted |
| 7	   | Group settings changed |
| 8	   | Added as friend |
| 9	   | Removed as friend |
| 10	 | News (a URL should be included) |
| 11	 | Debt simplification |
| 12	 | Group undeleted |
| 13	 | Expense undeleted |
| 14	 | Group currency conversion |
| 15	 | Friend currency conversion |

**Note**: While all parameters are optional, the server sets arbitrary (but large) limits
on the number of notifications returned if you set a very old `updated_after` value or `limit` of `0` for a
user with many notifications.

### get-user

Manage get user

- **`splitwise-pp-cli get-user <id>`** - Get information about another user

### remove-user-from-group

Manage remove user from group

- **`splitwise-pp-cli remove-user-from-group`** - Remove a user from a group. Does not succeed if the user has a non-zero balance.

**Note:** 200 OK does not indicate a successful response. You must check the success value of the response.

### undelete-expense

Manage undelete expense

- **`splitwise-pp-cli undelete-expense <id>`** - **Note**: 200 OK does not indicate a successful response. The operation was successful only if `success` is true.

### undelete-group

Manage undelete group

- **`splitwise-pp-cli undelete-group <id>`** - Restores a deleted group.

**Note**: 200 OK does not indicate a successful response. You must check the `success` value of the response.

### update-expense

Manage update expense

- **`splitwise-pp-cli update-expense <id>`** - Updates an expense. Parameters are the same as in `create_expense`, but you only need to include parameters
that are changing from the previous values. If any values is supplied for `users__{index}__{property}`, _all_
shares for the expense will be overwritten with the provided values.

**Note**: 200 OK does not indicate a successful response. The operation was successful only if `errors` is empty.

### update-user

Manage update user

- **`splitwise-pp-cli update-user <id>`** - Update a user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
splitwise-pp-cli get-categories

# JSON for scripting and agents
splitwise-pp-cli get-categories --json

# Filter to specific fields
splitwise-pp-cli get-categories --json --select id,name,status

# Dry run — show the request without sending
splitwise-pp-cli get-categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
splitwise-pp-cli get-categories --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
splitwise-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/splitwise-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPLITWISE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `splitwise-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `splitwise-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPLITWISE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on any command** — Set SPLITWISE_API_KEY to a key from https://secure.splitwise.com/apps, then run splitwise-pp-cli doctor.
- **balances / spend / search return nothing** — Run splitwise-pp-cli sync first — these read the local store, which is empty until synced.
- **Rate-limited (429) during a large sync** — Splitwise has conservative personal-use limits; re-run sync later, or use sync --since 7d for incremental pulls.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**namaggarwal/splitwise**](https://github.com/namaggarwal/splitwise) — Python
- [**tarunn2799/splitwise-mcp**](https://github.com/tarunn2799/splitwise-mcp) — Python
- [**keriwarr/splitwise**](https://github.com/keriwarr/splitwise) — JavaScript
- [**anvari1313/splitwise.go**](https://github.com/anvari1313/splitwise.go) — Go
- [**svarun115/splitwise-mcp-server**](https://github.com/svarun115/splitwise-mcp-server) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
