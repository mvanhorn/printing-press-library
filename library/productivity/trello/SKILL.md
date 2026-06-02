---
name: pp-trello
description: "Every Trello feature, plus a local SQLite mirror, offline search, and cross-board analytics no other Trello tool has. Trigger phrases: `check my trello`, `what's overdue in trello`, `trello workload`, `sync my trello boards`, `use trello`, `run trello`."
author: "Tam Nguyen"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trello-pp-cli
    install:
      - kind: go
        bins: [trello-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/trello/cmd/trello-pp-cli
---

# Trello — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trello-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install trello --cli-only
   ```
2. Verify: `trello-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/trello/cmd/trello-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

trello-pp-cli mirrors your boards, lists, cards, checklists, and activity into local SQLite, so you can grep, SQL-query, and run cross-board analytics offline. On top of full CRUD parity with every existing Trello CLI and MCP, it adds overdue sweeps, member workload balance, cycle time, bottleneck detection, and velocity trends that require a local join no single API call provides.

## When to Use This CLI

Use this CLI when an agent or user needs Trello data the UI can't surface in one query: overdue work across every board, who is overloaded, how long cards take, or offline search over the whole workspace. It is the right tool for status reports, sprint planning, and cleanup passes.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time collaboration or notifications; use the Trello UI.
- Do not use it to build in-board Power-Up widgets; that needs the Power-Up iframe API, not the REST CLI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-board analytics
- **`overdue`** — Every past-due card across all your boards, ranked by lateness and owner.

  _Reach for this when you need the full blast radius of slipped work across everything, not one board's slice._

  ```bash
  trello-pp-cli overdue --agent
  ```
- **`workload`** — Open and due-soon card load per member across every board.

  _Reach for this before assigning new work, when 'who has capacity' can't come from one board._

  ```bash
  trello-pp-cli workload --window 7d --agent
  ```
- **`velocity`** — Cards completed per week over the last N weeks, per board or member, with trend.

  _Reach for this to sanity-check a deadline with real historical throughput._

  ```bash
  trello-pp-cli velocity --weeks 8 --agent
  ```

### Flow diagnostics
- **`cycletime`** — How long cards take from started to done, with median and p90 per list or label.

  _Reach for this to quantify where work stalls and set realistic SLAs._

  ```bash
  trello-pp-cli cycletime --board Eng --agent
  ```
- **`bottleneck`** — Which list is clogged now, by card count and how long cards have aged in place.

  _Reach for this in standup to name the exact stage starving throughput._

  ```bash
  trello-pp-cli bottleneck --board Eng --agent
  ```
- **`churn`** — Cards that bounce backward between lists, revealing rework and unstable requirements.

  _Reach for this in a retro to find where work keeps getting kicked back._

  ```bash
  trello-pp-cli churn --weeks 4 --agent
  ```

### Hidden-state analytics
- **`checklist-progress`** — Real card-level progress from checkitem completion across boards.

  _Reach for this to catch cards nearing a deadline with unchecked subtasks._

  ```bash
  trello-pp-cli checklist-progress --below 80 --agent
  ```
- **`blocked`** — Cards flagged blocked by label, checkitem text, or comment, and how long they've sat.

  _Reach for this to assemble an unblock list scattered across labels, checklists, and comments._

  ```bash
  trello-pp-cli blocked --over 3d --agent
  ```

## Command Reference

**actions** — https://trello.com/docs/api/action/index.html

- `trello-pp-cli actions delete-by-id` — Delete actions by id action()
- `trello-pp-cli actions get-by-id` — Get actions by id action()
- `trello-pp-cli actions get-by-id-by-field` — Get actions by id action by field()
- `trello-pp-cli actions update-by-id` — Update actions by id action()

**batch** — https://trello.com/docs/api/batch/index.html

- `trello-pp-cli batch` — Get batch()

**boards** — https://trello.com/docs/api/board/index.html

- `trello-pp-cli boards add` — Add boards()
- `trello-pp-cli boards get-by-id` — Get boards by id board()
- `trello-pp-cli boards get-by-id-by-field` — Get boards by id board by field()
- `trello-pp-cli boards update-by-id` — Update boards by id board()

**cards** — https://trello.com/docs/api/card/index.html

- `trello-pp-cli cards add` — Add cards()
- `trello-pp-cli cards delete-by-id` — Delete cards by id card()
- `trello-pp-cli cards get-by-id` — Get cards by id card()
- `trello-pp-cli cards get-by-id-by-field` — Get cards by id card by field()
- `trello-pp-cli cards update-by-id` — Update cards by id card()

**checklists** — https://trello.com/docs/api/checklist/index.html

- `trello-pp-cli checklists add` — Add checklists()
- `trello-pp-cli checklists delete-by-id` — Delete checklists by id checklist()
- `trello-pp-cli checklists get-by-id` — Get checklists by id checklist()
- `trello-pp-cli checklists get-by-id-by-field` — Get checklists by id checklist by field()
- `trello-pp-cli checklists update-by-id` — Update checklists by id checklist()

**labels** — https://trello.com/docs/api/label/index.html

- `trello-pp-cli labels add` — Add labels()
- `trello-pp-cli labels delete-by-id` — Delete labels by id label()
- `trello-pp-cli labels get-by-id` — Get labels by id label()
- `trello-pp-cli labels update-by-id` — Update labels by id label()

**lists** — https://trello.com/docs/api/list/index.html

- `trello-pp-cli lists add` — Add lists()
- `trello-pp-cli lists get-by-id` — Get lists by id list()
- `trello-pp-cli lists get-by-id-by-field` — Get lists by id list by field()
- `trello-pp-cli lists update-by-id` — Update lists by id list()

**members** — https://trello.com/docs/api/member/index.html

- `trello-pp-cli members get-by-id` — If you specify 'me' as the username
- `trello-pp-cli members get-by-id-by-field` — Get members by id member by field()
- `trello-pp-cli members update-by-id` — Update members by id member()

**notifications** — https://trello.com/docs/api/notification/index.html

- `trello-pp-cli notifications add-all-read` — Add notifications all read()
- `trello-pp-cli notifications get-by-id` — Get notifications by id notification()
- `trello-pp-cli notifications get-by-id-by-field` — Get notifications by id notification by field()
- `trello-pp-cli notifications update-by-id` — Update notifications by id notification()

**organizations** — https://trello.com/docs/api/organization/index.html

- `trello-pp-cli organizations add` — Add organizations()
- `trello-pp-cli organizations delete-by-id-org` — Delete organizations by id org()
- `trello-pp-cli organizations get-by-id-org` — Get organizations by id org()
- `trello-pp-cli organizations get-by-id-org-by-field` — Get organizations by id org by field()
- `trello-pp-cli organizations update-by-id-org` — Update organizations by id org()

**search_resource** — Manage search resource

- `trello-pp-cli search-resource get-search` — Get search()
- `trello-pp-cli search-resource get-search-members` — Get search members()

**sessions** — https://trello.com/docs/api/session/index.html

- `trello-pp-cli sessions add` — Add sessions()
- `trello-pp-cli sessions get-socket` — This is the route for WebSocket requests. See the socket API reference for a description of WebSocket usage.
- `trello-pp-cli sessions update-by-id` — Update sessions by id session()

**tokens** — https://trello.com/docs/api/token/index.html

- `trello-pp-cli tokens delete-by` — Delete tokens by token()
- `trello-pp-cli tokens get-by` — Get tokens by token()
- `trello-pp-cli tokens get-by-by-field` — Get tokens by token by field()

**types_resource** — Manage types resource

- `trello-pp-cli types-resource <id>` — Get types by id()

**webhooks** — https://trello.com/docs/api/webhook/index.html

- `trello-pp-cli webhooks add` — Add webhooks()
- `trello-pp-cli webhooks delete-by-id` — Delete webhooks by id webhook()
- `trello-pp-cli webhooks get-by-id` — Get webhooks by id webhook()
- `trello-pp-cli webhooks get-by-id-by-field` — Get webhooks by id webhook by field()
- `trello-pp-cli webhooks update` — Update webhooks()
- `trello-pp-cli webhooks update-by-id` — Update webhooks by id webhook()


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trello-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning triage

```bash
trello-pp-cli overdue --agent --select cards.name,cards.due,cards.board
```

Narrow the overdue sweep to just name, due date, and board for a quick agent-readable list.

### Sprint capacity check

```bash
trello-pp-cli workload --window 7d --agent
```

See per-member load before assigning new cards.

### Find the bottleneck

```bash
trello-pp-cli bottleneck --board Eng --agent
```

Name the clogged stage in one shot during standup.

## Auth Setup

Trello auth is a key plus token pair passed as query params. Get both at https://trello.com/app-key. Set TRELLO_API_KEY and TRELLO_API_TOKEN, then run 'trello-pp-cli doctor' to verify.

Run `trello-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trello-pp-cli batch --urls https://example.com/resource --key your-token-here --token your-token-here --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
trello-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trello-pp-cli feedback --stdin < notes.txt
trello-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/trello-pp-cli/feedback.jsonl`. They are never POSTed unless `TRELLO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRELLO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
trello-pp-cli profile save briefing --json
trello-pp-cli --profile briefing batch --urls https://example.com/resource --key your-token-here --token your-token-here
trello-pp-cli profile list --json
trello-pp-cli profile show briefing
trello-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `trello-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/trello/cmd/trello-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add trello-pp-mcp -- trello-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trello-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trello-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trello-pp-cli <command> --help`.
