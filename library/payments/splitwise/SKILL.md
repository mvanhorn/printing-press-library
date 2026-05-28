---
name: pp-splitwise
description: "Every Splitwise feature, plus an offline SQLite ledger that powers balance, debt-aging, spend analytics Trigger phrases: `what do I owe on splitwise`, `who owes me money`, `split this expense`, `settle up the trip`, `how much did we spend on food`, `use splitwise`, `run splitwise`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - splitwise-pp-cli
    install:
      - kind: go
        bins: [splitwise-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-cli
---

# Splitwise — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `splitwise-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install splitwise --cli-only
   ```
2. Verify: `splitwise-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

splitwise-pp-cli wraps the full Splitwise API — expenses, groups, friends, comments, settle-ups — and keeps a local copy of your whole ledger. That local store powers a net `balances` view, `debts --aged` (who never pays you back), `spend` rollups by category or month, offline `search`, a group `ledger` with running balances, and a `settle-up` plan that minimizes transfers. Fuzzy name resolution means you never paste a numeric ID.

## When to Use This CLI

Reach for splitwise-pp-cli when a task involves shared expenses, group trips, roommate bills, or settling up — logging an expense, checking who owes whom, rolling up spend by category, finding a past expense, or computing a settle-up plan. It is the right tool when you want offline analytics over a Splitwise account or scriptable expense automation, not a one-off live lookup.

## Unique Capabilities

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

## Command Reference

**add-user-to-group** — Manage add user to group

- `splitwise-pp-cli add-user-to-group` — **Note**: 200 OK does not indicate a successful response. You must check the `success` value of the response.

**create-comment** — Manage create comment

- `splitwise-pp-cli create-comment` — Create a comment

**create-expense** — Manage create expense

- `splitwise-pp-cli create-expense` — Creates an expense. You may either split an expense equally (only with `group_id` provided), or supply a list of shares.

**create-friend** — Manage create friend

- `splitwise-pp-cli create-friend` — Adds a friend. If the other user does not exist, you must supply `user_first_name`.

**create-friends** — Manage create friends

- `splitwise-pp-cli create-friends` — Add multiple friends at once.

**create-group** — Manage create group

- `splitwise-pp-cli create-group` — Creates a new group. Adds the current user to the group by default.

**delete-comment** — Manage delete comment

- `splitwise-pp-cli delete-comment <id>` — Deletes a comment. Returns the deleted comment.

**delete-expense** — Manage delete expense

- `splitwise-pp-cli delete-expense <id>` — **Note**: 200 OK does not indicate a successful response. The operation was successful only if `success` is true.

**delete-friend** — Manage delete friend

- `splitwise-pp-cli delete-friend <id>` — Given a friend ID, break off the friendship between the current user and the specified user.

**delete-group** — Manage delete group

- `splitwise-pp-cli delete-group <id>` — Delete an existing group. Destroys all associated records (expenses, etc.)

**get-categories** — Manage get categories

- `splitwise-pp-cli get-categories` — Returns a list of all categories Splitwise allows for expenses.

**get-comments** — Manage get comments

- `splitwise-pp-cli get-comments` — Get expense comments

**get-currencies** — Manage get currencies

- `splitwise-pp-cli get-currencies` — Returns a list of all currencies allowed by the system.

**get-current-user** — Manage get current user

- `splitwise-pp-cli get-current-user` — Get information about the current user

**get-expense** — Manage get expense

- `splitwise-pp-cli get-expense <id>` — Get expense information

**get-expenses** — Manage get expenses

- `splitwise-pp-cli get-expenses` — List the current user's expenses

**get-friend** — Manage get friend

- `splitwise-pp-cli get-friend <id>` — Get details about a friend

**get-friends** — Manage get friends

- `splitwise-pp-cli get-friends` — **Note**: `group` objects only include group balances with that friend.

**get-group** — Manage get group

- `splitwise-pp-cli get-group <id>` — Get information about a group

**get-groups** — Manage get groups

- `splitwise-pp-cli get-groups` — **Note**: Expenses that are not associated with a group are listed in a group with ID 0.

**get-notifications** — Manage get notifications

- `splitwise-pp-cli get-notifications` — Return a list of recent activity on the users account with the most recent items first.

**get-user** — Manage get user

- `splitwise-pp-cli get-user <id>` — Get information about another user

**remove-user-from-group** — Manage remove user from group

- `splitwise-pp-cli remove-user-from-group` — Remove a user from a group. Does not succeed if the user has a non-zero balance.

**undelete-expense** — Manage undelete expense

- `splitwise-pp-cli undelete-expense <id>` — **Note**: 200 OK does not indicate a successful response. The operation was successful only if `success` is true.

**undelete-group** — Manage undelete group

- `splitwise-pp-cli undelete-group <id>` — Restores a deleted group. **Note**: 200 OK does not indicate a successful response.

**update-expense** — Manage update expense

- `splitwise-pp-cli update-expense <id>` — Updates an expense.

**update-user** — Manage update user

- `splitwise-pp-cli update-user <id>` — Update a user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
splitwise-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Splitwise authenticates with a personal API key used as an HTTP Bearer token. Register an app at https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY. OAuth 2.0 (authorization-code) is also supported for multi-user apps, but a personal API key is the fastest path for a power-user CLI.

Run `splitwise-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  splitwise-pp-cli get-categories --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
splitwise-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
splitwise-pp-cli feedback --stdin < notes.txt
splitwise-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/splitwise-pp-cli/feedback.jsonl`. They are never POSTed unless `SPLITWISE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPLITWISE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
splitwise-pp-cli profile save briefing --json
splitwise-pp-cli --profile briefing get-categories
splitwise-pp-cli profile list --json
splitwise-pp-cli profile show briefing
splitwise-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `splitwise-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add splitwise-pp-mcp -- splitwise-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which splitwise-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   splitwise-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `splitwise-pp-cli <command> --help`.
