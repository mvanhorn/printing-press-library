---
name: pp-monday
description: "Every monday.com feature, plus offline SQL, FTS5 search, typed column-value handling, and bulk edits no other... Trigger phrases: `use monday-pp-cli`, `monday cli`, `what changed in monday since`, `who's overloaded on monday`, `monday sprint slip`, `monday bulk edit columns`, `monday column drift`, `monday complexity budget`, `find this in monday across boards`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - monday-pp-cli
    install:
      - kind: go
        bins: [monday-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-cli
---

# monday.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `monday-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install monday --cli-only
   ```
2. Verify: `monday-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mirrors monday.com's GraphQL API as ~40 typed commands, then transcends with a local SQLite store, FTS5 search across items/updates/docs/columns, complexity-aware retry, and 12 commands that only work because everything is in the store: cross-board activity windows, per-person workload, sprint slip reports, status-bottleneck dwell time, column-schema drift, contextual mentions, complexity-budget pre-flight, typed bulk edits with dry-run, mirror/formula resolution, CSV reconcile, board health, and item full-context dumps.

## When to Use This CLI

Reach for monday-pp-cli when an agent task needs to answer cross-board questions, mutate many items at once with a safe dry-run, or replay activity that the official MCP can't return in a single call. Prefer it over the official MCP when the answer requires joining multiple resources or running offline; prefer the official MCP for one-shot reads of a single resource.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — Tail every change across every synced board since a window or timestamp; filter by user, column, or board without burning API points.

  _When an agent needs to answer 'what changed since standup' or 'what did this user touch today' across multiple boards, this is the only command that returns a single ranked stream without N round-trips._

  ```bash
  monday-pp-cli since 2h --board sprint-q3 --json --select item.name,user.name,column.title,value
  ```
- **`whoami-load`** — Per-user open-item count weighted by status and overdue flag, computed across every board the user is assigned on.

  _Answers 'who's overloaded this sprint?' and 'what is my real plate?' in one call instead of filtering each board by Person column._

  ```bash
  monday-pp-cli whoami-load --board 12345,67890 --json --select person,total,by_status
  ```
- **`bottleneck`** — Per-status median and p90 dwell time per item, computed from activity-log transitions on a board.

  _Surfaces the actual bottleneck status ("In Review" sat 9 days p90) when an agent is asked why velocity dropped._

  ```bash
  monday-pp-cli bottleneck --board sprint-q3 --column status --json
  ```
- **`velocity`** — Per-sprint committed / completed / slipped / added-mid-sprint counts, plus per-person delivered, across the last N sprint cycles.

  _Replaces the "export to CSV every Friday and run three SQL queries" loop for engineering managers._

  ```bash
  monday-pp-cli velocity --board 12345 --json --select current_items_by_status,transitions_into_done
  ```
- **`resolve`** — For a given item or column, walks mirror and formula chains in the local store and prints the source board, source item, source column, and final resolved value.

  _Answers "why is this column showing this value?" without clicking through the source board manually._

  ```bash
  monday-pp-cli resolve --item 12345 --column mirror_account --json
  ```
- **`boards health`** — Per-board scorecard: % items with owner, % with due-date, % overdue, % updated in last 7d, count of empty status, count of broken mirror columns.

  _Friday status snapshot becomes a single command per board._

  ```bash
  monday-pp-cli boards health 12345678 --json
  ```

### Reachability mitigation
- **`column-drift`** — Reports columns added, removed, renamed, or retyped on a board since the last sync.

  _Catches "someone changed status to dropdown" before downstream automations break with no warning._

  ```bash
  monday-pp-cli column-drift --board sales-pipeline --json
  ```
- **`complexity-budget`** — Predicts the complexity-points cost of a query and the remaining account-minute budget without executing the body.

  _Lets an agent answer "can I run this bulk read right now?" before it gets a 429 mid-loop._

  ```bash
  monday-pp-cli complexity-budget --query-file ./bulk-status.graphql --json
  ```

### Agent-native plumbing
- **`mentions`** — FTS5 search across item names, update bodies, doc bodies, and text-typed column values; result rows hydrated with board, group, and owner context.

  _Cross-board reverse-lookup that the official universal search exposes only in the web UI and never as structured agent output._

  ```bash
  monday-pp-cli mentions "Acme Corp" --board 12345,67890 --updates --json
  ```
- **`bulk-edit`** — Read a CSV; validate every cell against the cached typed-column schema; print a unified diff in dry-run; on apply, run change_item_column_values per row with per-row exit codes and resume-on-failure.

  _RevOps integrators currently re-build dry-run themselves in Python every time; this is the safety net that makes Monday a first-class CSV-driven backend._

  ```bash
  monday-pp-cli bulk-edit --from updates.csv --column status,owner --dry-run
  ```
- **`reconcile`** — Joins a local board to an external CSV by a chosen column-value key; emits only-in-monday, only-in-csv, and diff sets.

  _Mid-week reconcile becomes a one-liner instead of a script._

  ```bash
  monday-pp-cli reconcile --against-csv salesforce-export.csv --key sf_account_id --json
  ```
- **`context`** — For one item-id, returns one JSON blob with the item, every column-value typed, every update, every reply, every linked doc, every asset metadata, every activity log entry, and every mirror source.

  _One command to give an agent everything about one item without N round-trips._

  ```bash
  monday-pp-cli context 12345 --json
  ```
- **`cross-ref`** — Joins Monday items to whichever column stores their cross-system ID (Linear id, Notion page-id, Slack thread-ts) and emits a structured list ready to pipe into another tool.

  _Lets an agent answer 'which Monday items map to which Linear issues?' in one call, then pipe straight into linear-pp-cli without writing glue._

  ```bash
  monday-pp-cli cross-ref --board sprint-q3 --link-column linear_id --json
  ```

## Command Reference

**account** — Account-level metadata and the current API user.

- `monday-pp-cli account get` — Return account-level information.
- `monday-pp-cli account me` — Return the current API user.


**Hand-written commands**

- `monday-pp-cli workspaces` — List, get, create, and update monday.com workspaces.
- `monday-pp-cli boards` — List, get, create, and inspect boards (groups, columns, full data, activity, insights, health).
- `monday-pp-cli groups` — List, create, and move groups on a board.
- `monday-pp-cli items` — List, get, create, update, delete, and move items, plus typed column-value edits.
- `monday-pp-cli columns` — List, create, update, delete columns, and inspect column-type schemas.
- `monday-pp-cli updates` — List and create item updates (the comment threads under each item).
- `monday-pp-cli docs` — List, get, create, and update monday.com docs.
- `monday-pp-cli users` — List and get users in the account.
- `monday-pp-cli teams` — List teams in the account.
- `monday-pp-cli notifications` — Send notifications to users (--dry-run, batch via --stdin).
- `monday-pp-cli meetings` — List notetaker meetings with summaries, action items, and transcripts.
- `monday-pp-cli sprints` — List sprint boards, sprint metadata, and sprint summaries (monday-dev).
- `monday-pp-cli assets` — List, download, and upload file assets attached to items.
- `monday-pp-cli sync` — Pull workspaces, boards, groups, items, columns, column_values, users, teams, updates, and activity logs into the...
- `monday-pp-cli search` — FTS5 search across items, updates, docs, and text-typed column values in the local store.
- `monday-pp-cli sql` — Run a read-only SELECT against the local SQLite store.
- `monday-pp-cli since` — Replay activity log changes across boards within a time window (e.g. since 2h, since 1d).
- `monday-pp-cli whoami-load` — Per-person workload across every synced board, weighted by status and overdue.
- `monday-pp-cli bottleneck` — Per-status median and p90 dwell time on a board, computed from local activity logs.
- `monday-pp-cli velocity` — Sprint velocity report: committed, completed, slipped, added-mid-sprint per cycle.
- `monday-pp-cli column-drift` — Detect added, removed, renamed, or retyped columns since the last sync.
- `monday-pp-cli mentions` — FTS5 search across items, updates, docs, and text columns with hydrated context.
- `monday-pp-cli complexity-budget` — Pre-flight cost probe: predict GraphQL complexity points and remaining account budget.
- `monday-pp-cli bulk-edit` — Apply CSV-driven column-value edits with typed validation, dry-run, and resume-on-failure.
- `monday-pp-cli resolve` — Walk mirror and formula chains to show source board, source item, and resolved value.
- `monday-pp-cli reconcile` — Compare a board against an external CSV; emit only-in-monday, only-in-csv, and diff sets.
- `monday-pp-cli cross-ref` — Export a board joined to its cross-system ID column (Linear id, Notion page-id) as JSON ready to pipe.
- `monday-pp-cli context` — Dump everything about one item: column values, updates, replies, docs, assets, activity log, mirror sources.
- `monday-pp-cli gql` — Run a saved or ad-hoc GraphQL query against monday.com, with stored params.
- `monday-pp-cli schema` — Print the cached GraphQL schema, or introspect a single type.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `MONDAY_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
monday-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find every item that mentions a customer across all your boards

```bash
monday-pp-cli mentions "Acme Corp" --board 12345,67890 --updates --json --select board_name,group_title,item_name,field,snippet
```

Cross-column substring search over items + (with --updates) recent comment bodies on every supplied board. Output rows include the board name, group title, item name, which field matched, and a context-rich snippet.

### Replay everything a teammate touched today

```bash
monday-pp-cli since 24h --board 12345,67890 --user-ids 9999 --json --select board_name,activity
```

Activity-log tail-fetch on the supplied boards, filtered to one user. Each row is a board with its activity_logs array; events include column-value changes, item creates, and moves.

### Stage a 200-row status update from a Salesforce export

```bash
monday-pp-cli bulk-edit --from sf-export.csv --board 12345 --column status,owner
```

Reads the CSV, parses each row into a typed column-value plan, and prints what would change without sending. Flip to --apply once the plan looks right.

### Show me sprint slip from the last four cycles

```bash
monday-pp-cli velocity --board 12345 --json
```

Combines current per-status item counts with activity-log status transitions over the last few pages: how many items entered Done vs left Done, plus a snapshot of what's where right now.

### Get every piece of context an LLM might need about one item

```bash
monday-pp-cli context 1234567890 --json
```

One SQLite join over items + column_values + updates + replies + docs + assets + activity_logs + mirror sources; the agent gets everything in a single call.

### Bridge Monday items to your Linear board

```bash
monday-pp-cli cross-ref --board sprint-q3 --link-column linear_id --json | jq '.[] | select(.linker != null)'
```

Joins Monday items to whichever column stores the Linear issue id; the output JSON is the contract for piping into linear-pp-cli without writing glue.

## Auth Setup

monday.com requires an API token from your account's Developer page. Set it via `MONDAY_API_TOKEN` and the CLI sends it as the `Authorization` header (no Bearer prefix). Tokens inherit the issuing user's permissions, so private boards your user can't see remain invisible to the CLI.

Run `monday-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  monday-pp-cli account get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
monday-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
monday-pp-cli feedback --stdin < notes.txt
monday-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.monday-pp-cli/feedback.jsonl`. They are never POSTed unless `MONDAY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MONDAY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
monday-pp-cli profile save briefing --json
monday-pp-cli --profile briefing account get
monday-pp-cli profile list --json
monday-pp-cli profile show briefing
monday-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `monday-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add monday-pp-mcp -- monday-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which monday-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   monday-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `monday-pp-cli <command> --help`.
