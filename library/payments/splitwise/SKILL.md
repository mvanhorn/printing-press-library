---
name: pp-splitwise
description: "Every Splitwise feature, plus an offline SQLite ledger that powers balance, debt-aging, spend analytics, fairness, and full-text search no other Splitwise tool has. Trigger phrases: `what do I owe on splitwise`, `who owes me money`, `split this expense`, `settle up the trip`, `how much did we spend on food`, `use splitwise`, `run splitwise`."
author: "Vinny Pasceri"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - splitwise-pp-cli
---

# Splitwise — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `splitwise-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install splitwise --cli-only
   ```
2. Verify: `splitwise-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

splitwise-pp-cli wraps the full Splitwise API — expenses, groups, friends, comments, settle-ups — and keeps a local copy of your whole ledger. That local store powers a net `balances` view, `debts --aged` (who never pays you back), `spend` rollups by category or month, offline `search`, a group `ledger` with running balances, `fairness` and `net` for who's carrying cost and how balances collapse across groups, and a `settle-up` plan that minimizes transfers. `brief` gives an agent one bounded state digest, and `reconcile` verifies the local store still matches Splitwise before you trust any of it. Fuzzy name resolution means you never paste a numeric ID.

## When to Use This CLI

Reach for splitwise-pp-cli when a task involves shared expenses, group trips, roommate bills, or settling up — logging an expense, checking who owes whom, aging a stale debt, rolling up spend by category, finding a past expense, checking who's carrying more than their share, or computing a settle-up plan. It is the right tool when you want offline analytics or a verified local copy of a Splitwise account, not a one-off live lookup. Under --agent every command wraps its payload as {"meta": {...}, "results": ...}; --json returns the bare payload.

## Anti-triggers

Do not use this CLI for:
- do not use for live FX conversion — normalize takes user-supplied rates
- do not use to pay anyone — it records settlements in Splitwise only
- do not use for multi-user OAuth apps — the CLI authenticates as a single user with a personal API key
- do not use for real-time push notifications — sync and reconcile are both pull-based, not a live feed

## Unique Capabilities

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


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SPLITWISE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

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

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
splitwise-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

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

## Auth Setup

Splitwise authenticates with a personal API key used as an HTTP Bearer token. Register an app at https://secure.splitwise.com/apps to get your key, then set SPLITWISE_API_KEY. The Splitwise API also offers OAuth 2.0 (authorization-code) for multi-user apps, but this CLI authenticates as a single user with a personal API key only — there is no OAuth login flow in the binary.

Run `splitwise-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  splitwise-pp-cli get-categories --agent
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SPLITWISE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SPLITWISE_CONFIG_DIR`, `SPLITWISE_DATA_DIR`, `SPLITWISE_STATE_DIR`, `SPLITWISE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SPLITWISE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `splitwise-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SPLITWISE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SPLITWISE_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
splitwise-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "splitwise-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `splitwise-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `splitwise-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `splitwise-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
splitwise-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
splitwise-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
splitwise-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
splitwise-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`splitwise-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SPLITWISE_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
splitwise-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
splitwise-pp-cli feedback --stdin < notes.txt
splitwise-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SPLITWISE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPLITWISE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

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

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add splitwise-pp-mcp -- splitwise-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which splitwise-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   splitwise-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `splitwise-pp-cli <command> --help`.
