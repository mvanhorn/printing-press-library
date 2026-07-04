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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/payments/splitwise/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Splitwise — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `splitwise-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install splitwise --cli-only
   ```
2. Verify: `splitwise-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/splitwise/cmd/splitwise-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

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

  Add `--by-group` to break your net position out **per group** (one row per group per currency, biggest absolute balance first) instead of per friend — answers "what's my outstanding balance by group?" directly from the synced store:

  ```bash
  splitwise-pp-cli balances --by-group --agent
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

### Upcoming-obligations forecast
- **`forecast`** — Project your next shared obligations from recurring spending patterns over your synced history. Clusters expenses by normalized description, finds the ones with a regular cadence (>= 3 dated occurrences, mean cadence 2-400 days, largest gap <= 3x the smallest), and projects the next expected date and amount. Returns charges whose next occurrence falls inside the window (default 35 days) or is already overdue, sorted by expected date.
  Reads from your synced local store; on large accounts, results can be incomplete until a full sync.

  _Use to answer "what shared bills are coming up?" or "what should I budget for next month?" — and to catch a regular charge that's overdue and hasn't been logged yet._

  ```bash
  splitwise-pp-cli forecast --agent
  splitwise-pp-cli forecast --days 60 --limit 20 --json
  ```

  Output shape (`--agent` / `--json`):

  ```json
  {
    "as_of": "2026-05-30",
    "window_days": 35,
    "upcoming": [
      {
        "description": "Rent",
        "group": "Roommates",
        "last_date": "2026-05-01",
        "expected_date": "2026-06-01",
        "expected_amount": 1850.00,
        "cadence_days": 30,
        "occurrences": 6,
        "overdue": false
      }
    ]
  }
  ```

  Payments and settlement rows (`settle all balances`, `settle up`, `payment`, `paid via …`) are excluded. Read-only; never posts.

### Multi-currency normalization
- **`normalize`** — Normalize multi-currency net position and spend into one base currency using user-supplied offline FX rates (`--rate` / `--rates-file`); historical/automatic FX lookup is intentionally out of scope.

  _Use to compare or total spend across trips in different currencies — supply the rates and get one base-currency number; a currency with no rate is surfaced as unconverted, never silently dropped._

  ```bash
  splitwise-pp-cli normalize --base USD --rate EUR=1.08 --agent
  ```

### Trip & period reports
- **`report`** — Export an offline trip/period spend report as Markdown, CSV, or JSON. Single-currency only: defaults to the most common filtered currency, excludes other currencies with an explicit excluded count, and supports `--currency` to pin one. PDF is intentionally out of scope in v1.

  Output shape (`--agent` / `--json`):

  ```json
  {
    "scope": "group:Tahoe Trip",
    "currency": "USD",
    "period_start": "2025-01-10",
    "period_end": "2025-02-01",
    "expense_count": 12,
    "excluded_other_currency": 0,
    "total_cost": 1840.00,
    "your_paid": 920.00,
    "your_owed": 613.33,
    "your_net": 306.67,
    "people": [
      { "user_id": 1, "name": "You", "paid": 920.00, "owed": 613.33, "net": 306.67 }
    ],
    "categories": [
      { "name": "Lodging", "total": 1200.00, "count": 2 }
    ],
    "expenses": [
      { "id": 9001, "date": "2025-01-10", "description": "Cabin", "cost": 1200.00, "currency_code": "USD", "payer": "You" }
    ],
    "truncated": false
  }
  ```

  Payment and deleted rows are excluded. Single-currency: other-currency expenses are excluded and counted in `excluded_other_currency`. Read-only; never posts.

### Fairness & collection risk
- **`fairness`** — Score who carries the group, who's a collection risk, and who to chase or write off — offline, from your synced history.

  _Turns "who still owes me, and will I ever see it" into an action list: nudge, chase, or write off (debt that is old **and** gone quiet). `--by contribution` shows who fronts cash vs. free-rides; `--by collectability` ranks by debt age and settle latency. New group members with no history are surfaced separately, never flagged as risks._

  ```bash
  splitwise-pp-cli fairness --by risk --agent
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

### Cross-group netting
- **`net`** — Net what you owe and are owed across every group and non-group into the fewest direct transfers to settle your entire account.

  _Use when you share groups with the same person and per-group settle-ups would mean paying them more than once — `net` collapses to one transfer per counterparty. Plan-only in v1; `--record` (auto-post the netted payments) is a planned future enhancement._

  ```bash
  splitwise-pp-cli net --agent
  ```

### Data-quality audit
- **`audit`** — Scan your synced expenses offline for likely duplicates (same description, cost, currency, date, and group) and per-category cost outliers (robust modified z-score using median/MAD; two-sided threshold |z| > 3.5, flagging items far above OR below the category median), grouped by finding type.

  _Use before reporting or settling to catch double-entered charges (e.g. repeated "Settle all balances" rows) and surface unusually expensive or unusually cheap entries (likely data-quality errors) an agent or user should review. Read-only; never mutates. `--limit` caps findings per type (default 50)._

  ```bash
  splitwise-pp-cli audit --agent
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

### Settle the whole network in the fewest transfers — `net`

**One payment list that zeroes out everyone — across every group and non-group debt at once:**

```bash
splitwise-pp-cli net
```

Nets each friend's balances (cancelling A→B→C→A cycles) into the minimum set of real-world transfers, separated per currency, and reports how many transfers it saved vs. settling each group on its own. Add `--agent` for JSON.

### Catch bad data before you settle — `audit`

**Find likely duplicate expenses and per-category cost outliers:**

```bash
splitwise-pp-cli audit
```

Flags repeated near-identical expenses (same description, cost, date, currency, and group) and expenses far from their category baseline (either unusually expensive or unusually cheap) using a robust median/MAD score. Use `--limit N` to cap findings per type, `--agent` for JSON.

### See what's coming — `forecast`

**Project next month's recurring shared obligations:**

```bash
splitwise-pp-cli forecast
```

Detects regular charges (rent, utilities, subscriptions) from your synced history and projects the upcoming ones, flagging anything overdue or due soon. Set the window with `--days N` (default 35), add `--agent` for JSON.

### Normalize multi-currency spend — `normalize`

Convert mixed-currency balances/spend into one base currency with deterministic, user-supplied rates.

```bash
splitwise-pp-cli normalize --base USD --rate EUR=1.08 --rate GBP=1.27
```

Add `--agent` for JSON output in automation flows. A currency with no `--rate` is listed as unconverted (not mixed into the total); pin rates with repeated `--rate CUR=FACTOR` or a `--rates-file`.

### Export a trip/period report — `report`

Build a deterministic offline report from synced expenses for a trip/group or date window.

```bash
splitwise-pp-cli report --group "Tahoe Trip" --format md
splitwise-pp-cli report --since 2025-01-01 --until 2025-12-31 --format csv > 2025.csv
splitwise-pp-cli report --agent
```

### Collect what you're owed — the `fairness` cookbook

`fairness` turns your synced ledger into an action list. Default lens is **risk** (who to chase, worst first).

**Who should I chase, and what should I just write off?**

```bash
splitwise-pp-cli fairness --by risk
```

Ranks everyone who owes you by a 0–100 collection-risk score with a per-row action: 🟢 on track · 🟡 nudge · 🟠 chase · 🔴 write-off (old **and** gone quiet). The footer totals at-risk vs. write-off dollars.

**Who carries the group (fronts cash) vs. who free-rides?**

```bash
splitwise-pp-cli fairness --by contribution
```

Per person: paid, owed, net, carry-ratio, and a carrier/even/rider role. Informational — Splitwise settles regardless of who pays, so this is for when you still care who fronts the money.

**Who's slow to settle / a live collection risk?**

```bash
splitwise-pp-cli fairness --by collectability
```

Sorted by debt age, with average settle latency and days since they last settled; `--by collectability` now also shows a projected settle date (raw `projected_days_out` in JSON).

**Scope to one friend, or one group/trip:**

```bash
splitwise-pp-cli fairness --friend "Alex"
splitwise-pp-cli fairness --group "Tahoe Trip"
```

**Agent mode — the action list as JSON (raw day-counts for your own math):**

```bash
splitwise-pp-cli fairness --by risk --agent
```

Human tables print ages as `4y 3mo 8d`; JSON keeps raw `*_days` integers so tools convert themselves.

**Tune the write-off threshold** (default: 365 days old + 180 days silent):

```bash
splitwise-pp-cli fairness --by risk --write-off-days 730 --ghost-days 90
```

### Nudge a friend to pay — `fairness nudge`

Splitwise has no send-reminder endpoint, so this command posts a comment on a shared unsettled expense; Splitwise then notifies participants per their own notification settings.

Preview only by default:

```bash
splitwise-pp-cli fairness nudge "Alex"
```

Actually post the reminder comment:

```bash
splitwise-pp-cli fairness nudge "Alex" --send
```

Optional flags: `--message` to override reminder text, `--expense-id` to force a specific expense, and `--send` to post (otherwise preview only). Reachability caveat: this CLI's synced `Friend` shape does not include email/registration status, so v1 does not pre-gate on confirmed-account status.

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
splitwise-pp-cli search "sushi" --fuzzy        # typo-tolerant
splitwise-pp-cli search "hotel" --type get-expenses
```

Word-boundary search across the meaningful text of your synced history (descriptions, member and group names, categories). Add `--fuzzy` for typo tolerance, or `--type <resource>` to scope to one resource type.

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
