# Splitwise CLI

**Every Splitwise feature, plus an offline SQLite ledger that powers balance, debt-aging, spend analytics, fairness, and full-text search no other Splitwise tool has.**

splitwise-pp-cli wraps the full Splitwise API — expenses, groups, friends, comments, settle-ups — and keeps a local copy of your whole ledger. That local store powers a net `balances` view, `debts --aged` (who never pays you back), `spend` rollups by category or month, offline `search`, a group `ledger` with running balances, `fairness` and `net` for who's carrying cost and how balances collapse across groups, and a `settle-up` plan that minimizes transfers. `brief` gives an agent one bounded state digest, and `reconcile` verifies the local store still matches Splitwise before you trust any of it. Fuzzy name resolution means you never paste a numeric ID.

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

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/splitwise-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install splitwise --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-splitwise --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-splitwise --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install splitwise --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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

Splitwise authenticates with a personal API key used as an HTTP Bearer token. Register an app at https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY. The Splitwise API also offers OAuth 2.0 (authorization-code) for multi-user apps, but this CLI authenticates as a single user with a personal API key only — there is no OAuth login flow in the binary.

## Quick Start

```bash
# Confirm the binary, config path, and verify state without needing credentials.
splitwise-pp-cli doctor --dry-run

# Pull the last 30 days of groups, friends, and expenses into the local store.
splitwise-pp-cli sync --resources get-groups,get-friends,get-expenses --since 30d

# See your net position across every friend and group at a glance.
splitwise-pp-cli balances

# Get one compact digest — net position, stalest debts, and recent changes — in a single call.
splitwise-pp-cli brief --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Money state that compounds locally
- **`balances`** — See everything you owe and are owed across every friend and group in one net-position view.

  _Reach for this instead of separate get-groups + get-friends calls when an agent needs the user's overall money position._

  ```bash
  splitwise-pp-cli balances --by-currency --agent
  ```
- **`debts`** — List who owes you (and whom you owe) sorted by how long the balance has gone unsettled.

  _Use when the task is 'who never pays me back' or chasing stale IOUs, not just the current balance._

  ```bash
  splitwise-pp-cli debts --aged --agent
  ```
- **`net`** — Collapse a person's balance across every group and non-group expense into the minimum set of real-world transfers.

  _Use when one person's balance is scattered across multiple groups and one-off expenses and you want the fewest real transfers, not a per-group snapshot._

  ```bash
  splitwise-pp-cli net --agent
  ```
- **`ledger`** — Every expense in a group, in date order, with a cumulative running balance per member.

  _Use to audit how a group's balances got to where they are, not just the snapshot. Add --friend to replay one person across every group instead of one group's members._

  ```bash
  splitwise-pp-cli ledger "Tahoe Trip" --agent
  ```
- **`balances`** — See one row per group per currency for every non-zero balance, without the noise of settled groups.

  _Use when the question is scoped to 'which groups do I still owe in', not the single net number._

  ```bash
  splitwise-pp-cli balances --by-group --agent
  ```

### Settle and record safely
- **`settle-up`** — Compute the minimum set of transfers that zeroes out balances in a group, then optionally record the payments (print-only by default; --record writes real payment expenses to your Splitwise account).

  _Use when a group wants the fewest transfers to get everyone to zero, previewed before anything is recorded._

  ```bash
  splitwise-pp-cli settle-up "Tahoe Trip" --agent
  ```
- **`audit`** — Catch duplicate settlement rows and abnormal expense amounts before you trust a settle-up plan.

  _Run this before settle-up or report so a duplicate settlement or an outlier expense doesn't get baked into a transfer plan._

  ```bash
  splitwise-pp-cli audit --since 90d --agent
  ```
- **`split`** — Build and preview the exact expense split (equal, exact, percentage, or shares) before recording it.

  _Reach for this to turn 'I paid $84, split equally with the trip' into a ready-to-record expense without hand-building the share arrays. Add --record to submit it._

  ```bash
  splitwise-pp-cli split "Tahoe Trip" --amount 84 --equal --agent
  ```
- **`fairness nudge`** — Post a payment reminder as a comment on the actual open expense thread, previewed before it sends (print-only by default; --send posts a real comment your friend will see).

  _Use to nudge one person about a specific unpaid expense instead of a generic message outside Splitwise._

  ```bash
  splitwise-pp-cli fairness nudge "Jordan"
  ```
- **`reconcile`** — Verify the local store actually matches Splitwise before you trust a settle-up or report (calls the live Splitwise API; needs SPLITWISE_API_KEY and network).

  _Run this before a settle-up or report when a number looks wrong, or as a routine pre-settle check._

  ```bash
  splitwise-pp-cli reconcile --since 30d --agent
  ```

### Analytics no endpoint offers
- **`spend`** — Total shared spend broken down by category, group, or month from your synced history.

  _Use for any 'how much did we spend on X' question instead of paging the whole expense list._

  ```bash
  splitwise-pp-cli spend --group-by category --since 30d --agent
  ```
- **`fairness`** — See who's carrying more than their share of cost, and how likely a stale balance is to actually get paid.

  _Use when the question is who's owed the most relative to what they've paid, not just the raw balance._

  ```bash
  splitwise-pp-cli fairness --by collectability --agent
  ```
- **`report`** — Turn synced trip or period spend into a shareable summary plus per-person and per-category export.

  _Use for an end-of-trip or end-of-month writeup, or as a generic JSON/CSV sink into an external workflow tool._

  ```bash
  splitwise-pp-cli report --group "Tahoe Trip" --format md
  ```
- **`recurring`** — Surface repeating charges (rent, utilities, subscriptions) from your synced history and flag a cycle missing an expected entry.

  _Use to catch a shared monthly bill nobody remembered to log this cycle._

  ```bash
  splitwise-pp-cli recurring --agent
  ```
- **`forecast`** — See what shared bills are expected next, projected from your recurring-expense history.

  _Use for 'what's coming up' instead of recurring, which only detects the pattern of bills already logged._

  ```bash
  splitwise-pp-cli forecast --agent
  ```
- **`normalize`** — Express multi-currency spend in one base currency, using rates you supply, with anything unconverted called out honestly.

  _Use when spend spans more than one currency and you want one honest number, not a silently-dropped or auto-converted total._

  ```bash
  splitwise-pp-cli normalize --base USD --rate EUR=1.08 --agent
  ```

### Agent-native plumbing
- **`brief`** — Get one compact digest of net position, the stalest debts, and what changed since last sync in a single call.

  _Reach for this first at the start of a session instead of three separate calls; use balances, debts --aged, or activity directly when you need the full detail behind one of these numbers._

  ```bash
  splitwise-pp-cli brief --agent --compact
  ```
- **`activity`** — Show what changed since your last sync — new, edited, and deleted expenses to review.

  _Use to reconcile recent account activity before settling or reporting._

  ```bash
  splitwise-pp-cli activity --agent
  ```

## Recipes

### Net position for an agent

```bash
splitwise-pp-cli balances --agent --select by_currency
```

Returns just the headline numbers an agent needs to report the user's overall money position.

### Inspect a group's members and balances (narrow a verbose payload)

```bash
splitwise-pp-cli get-groups --agent --select name,members.first_name,members.balance.amount
```

get-groups returns deeply nested members and balance arrays; --select with dotted paths keeps only the fields you need so an agent doesn't burn context on the full payload.

### Verify the local store before trusting it

```bash
splitwise-pp-cli reconcile --since 30d --agent
```

Diffs the local store against live get_expenses and reports anything missing, stale, or deleted remotely before you build a settle-up or report on it.

### One-shot agent state check

```bash
splitwise-pp-cli brief --agent --compact
```

Returns net position, the stalest debts, and recent activity in one bounded call instead of three separate fan-out calls.

### Plan the fewest transfers to settle a trip

```bash
splitwise-pp-cli settle-up "Tahoe Trip"
```

Prints the minimum-transfer settle-up plan; add --record to create the payment expenses.

### Who is carrying the cost in a group

```bash
splitwise-pp-cli fairness --by contribution --group "Tahoe Trip" --agent
```

Classifies each member as carrier or rider from paid vs owed shares; --by risk and --by collectability switch lenses. The --agent output is wrapped as {meta, results}.

### Turn a friend, group, or category name into its id

```bash
splitwise-pp-cli resolve "Alex Kim" --type friend --agent
```

Use resolve whenever you need an id for a name: it matches the local store (no network) and returns the candidate records. Do not page through get-friends or get-groups and grep for a name; resolve is the id lookup.

## Usage

Run `splitwise-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SPLITWISE_CONFIG_DIR`, `SPLITWISE_DATA_DIR`, `SPLITWISE_STATE_DIR`, or `SPLITWISE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SPLITWISE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SPLITWISE_HOME=/srv/splitwise
splitwise-pp-cli doctor
```

Under `SPLITWISE_HOME=/srv/splitwise`, the four dirs resolve to `/srv/splitwise/config`, `/srv/splitwise/data`, `/srv/splitwise/state`, and `/srv/splitwise/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "splitwise": {
      "command": "splitwise-pp-mcp",
      "env": {
        "SPLITWISE_HOME": "/srv/splitwise"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SPLITWISE_DATA_DIR` overrides an explicit `--home` for that kind. Use `SPLITWISE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SPLITWISE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `splitwise-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

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


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`splitwise-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`splitwise-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`splitwise-pp-cli learnings list`** - Inspect taught rows
- **`splitwise-pp-cli learnings forget <query>`** - Undo a teach
- **`splitwise-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`splitwise-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`splitwise-pp-cli teach-pattern`** - Install a query/resource template up front
- **`splitwise-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SPLITWISE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `splitwise-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
splitwise-pp-cli get-categories

# JSON for scripting and agents
splitwise-pp-cli get-categories --json
# Filter to specific fields by name
splitwise-pp-cli get-categories --json --select <field>[,<field>...]

# Dry run — show the request without sending
splitwise-pp-cli get-categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
splitwise-pp-cli get-categories --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SPLITWISE_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `splitwise-pp-cli get-categories`
- `splitwise-pp-cli get-categories get`
- `splitwise-pp-cli get-categories list`
- `splitwise-pp-cli get-categories search`
- `splitwise-pp-cli get-comments`
- `splitwise-pp-cli get-comments get`
- `splitwise-pp-cli get-comments list`
- `splitwise-pp-cli get-comments search`
- `splitwise-pp-cli get-currencies`
- `splitwise-pp-cli get-currencies get`
- `splitwise-pp-cli get-currencies list`
- `splitwise-pp-cli get-currencies search`
- `splitwise-pp-cli get-current-user`
- `splitwise-pp-cli get-current-user get`
- `splitwise-pp-cli get-current-user list`
- `splitwise-pp-cli get-current-user search`
- `splitwise-pp-cli get-expenses`
- `splitwise-pp-cli get-expenses get`
- `splitwise-pp-cli get-expenses list`
- `splitwise-pp-cli get-expenses search`
- `splitwise-pp-cli get-friends`
- `splitwise-pp-cli get-friends get`
- `splitwise-pp-cli get-friends list`
- `splitwise-pp-cli get-friends search`
- `splitwise-pp-cli get-groups`
- `splitwise-pp-cli get-groups get`
- `splitwise-pp-cli get-groups list`
- `splitwise-pp-cli get-groups search`
- `splitwise-pp-cli get-notifications`
- `splitwise-pp-cli get-notifications get`
- `splitwise-pp-cli get-notifications list`
- `splitwise-pp-cli get-notifications search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
splitwise-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `splitwise-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/splitwise-pp-cli/config.toml`; `--home`, `SPLITWISE_HOME`, and per-kind env vars can relocate it.

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
- **sync stopped after one page** — Run splitwise-pp-cli reconcile to see what's missing against the live API, then splitwise-pp-cli sync --full to force a complete re-pull.
- **`get-expenses --data-source local` (or the offline fallback) returns only 20 rows even with `--all`** — Pass an explicit `--limit`, e.g. `splitwise-pp-cli get-expenses --data-source local --limit 5000 --json`; this build applies the default page size to local reads and `--all` does not lift it. The hand-built commands (ledger, spend, report, audit, …) read the whole local mirror and are unaffected.
- **`net`, `balances --by-group`, `report` show `you_id: 0` / `Your net: 0.00`, or `split --equal` says `--paid-by is required when current user is not synced`** — Run `splitwise-pp-cli sync --resources get-current-user` once; the default `sync` covers the list resources but not the single current-user object.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**namaggarwal/splitwise**](https://github.com/namaggarwal/splitwise) — Python (213 stars)
- [**keriwarr/splitwise**](https://github.com/keriwarr/splitwise) — TypeScript (79 stars)
- [**anvari1313/splitwise.go**](https://github.com/anvari1313/splitwise.go) — Go (12 stars)
- [**tarunn2799/splitwise-mcp**](https://github.com/tarunn2799/splitwise-mcp) — Python (11 stars)
- [**svarun115/splitwise-mcp-server**](https://github.com/svarun115/splitwise-mcp-server) — Python (1 stars)
- [**vishnujayvel/splitwise-mcp**](https://github.com/vishnujayvel/splitwise-mcp) — Python
- [**rfdez/n8n-nodes-splitwise**](https://github.com/rfdez/n8n-nodes-splitwise) — TypeScript
- [**aanzolaavila/splitwise.go**](https://github.com/aanzolaavila/splitwise.go) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
