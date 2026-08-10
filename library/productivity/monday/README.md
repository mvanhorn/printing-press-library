# monday.com CLI

**Every monday.com feature, plus offline SQL, FTS5 search, typed column-value handling, and bulk edits no other monday.com tool has.**

Mirrors monday.com's GraphQL API as ~40 typed commands, then transcends with a local SQLite store, FTS5 search across items/updates/docs/columns, complexity-aware retry, and 12 commands that only work because everything is in the store: cross-board activity windows, per-person workload, sprint slip reports, status-bottleneck dwell time, column-schema drift, contextual mentions, complexity-budget pre-flight, typed bulk edits with dry-run, mirror/formula resolution, CSV reconcile, board health, and item full-context dumps.

Learn more at [monday.com](https://developer.monday.com/api-reference/).

## Install

The recommended path installs both the `monday-pp-cli` binary and the `pp-monday` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install monday
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install monday --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/monday-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-monday --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-monday --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-monday skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-monday. The skill defines how its required CLI can be installed.
```

## Authentication

monday.com requires an API token from your account's Developer page. Set it via `MONDAY_API_TOKEN` and the CLI sends it as the `Authorization` header (no Bearer prefix). Tokens inherit the issuing user's permissions, so private boards your user can't see remain invisible to the CLI.

## Quick Start

```bash
# Save your API token (or export MONDAY_API_TOKEN) so every subsequent call is authenticated.
monday-pp-cli auth set-token YOUR_TOKEN_HERE


# Probe auth, reachability, and the current complexity budget so you know how many points the next sync will cost.
monday-pp-cli doctor


# Pull every workspace, board, group, item, column, update, and activity-log row into the local SQLite mirror.
monday-pp-cli sync --full


# FTS5 search across every synced surface; offline, no API points consumed.
monday-pp-cli mentions "Q3 launch" --json --select id,board.name,group.title


# What changed in the last two hours on the sprint board, ranked.
monday-pp-cli since 2h --board sprint-q3 --json


# Stage a typed column-value edit without sending it; flip --dry-run off to apply.
monday-pp-cli items set 12345 --board 99999 --column status=Done --column owner=alice --dry-run

```

## Known Gaps

- **MCP server runs over stdio only.** The MCP server (`monday-pp-mcp`) speaks stdio transport; HTTP/remote transport is not configured in this build. Agents that need cloud-hosted MCP access must wrap the binary themselves. Local stdio agents (Claude Code, Claude Desktop, OpenClaw, Cursor) work as expected.
- **Workflow manifest not bundled.** Compound multi-step workflows are not defined as a manifest; use `monday-pp-cli workflow archive` for the bundled archive workflow, or compose individual commands via the CLI/MCP surface.

## Unique Features

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

## Usage

Run `monday-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Account-level metadata and the current API user.

- **`monday-pp-cli account get`** - Return account-level information.
- **`monday-pp-cli account me`** - Return the current API user.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
monday-pp-cli account get

# JSON for scripting and agents
monday-pp-cli account get --json

# Filter to specific fields
monday-pp-cli account get --json --select id,name,status

# Dry run — show the request without sending
monday-pp-cli account get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
monday-pp-cli account get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `MONDAY_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-monday -g
```

Then invoke `/pp-monday <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-mcp@latest
```

Then register it:

```bash
claude mcp add monday monday-pp-mcp -e MONDAY_API_TOKEN=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/monday-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MONDAY_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/monday/cmd/monday-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "monday": {
      "command": "monday-pp-mcp",
      "env": {
        "MONDAY_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
monday-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/monday-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MONDAY_API_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `monday-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MONDAY_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **ComplexityException on a bulk read** — Run `monday-pp-cli complexity-budget --query-file <file>` first; if predicted points exceed the per-call 5M cap, narrow the field selection or paginate.
- **401 / Authentication failed** — monday.com tokens are sent raw in the `Authorization` header, no Bearer prefix. Re-issue via developer.monday.com -> Developer -> My Tokens and run `monday-pp-cli auth set-token`.
- **items list returns 25 rows when the board has thousands** — items list auto-paginates; if you see only one page check that --limit is not capped, and use `monday-pp-cli sync --board <id>` to land all rows in the local store.
- **column_value JSON looks different than yesterday's run** — An admin probably retyped the column. Run `monday-pp-cli column-drift --board <id>` to see exactly which columns changed shape since last sync.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**mondaycom/mcp**](https://github.com/mondaycom/mcp) — TypeScript (401 stars)
- [**@mondaydotcomorg/api**](https://github.com/mondaycom/monday-graphql-api) — TypeScript
- [**monday-api-python-sdk**](https://github.com/mondaycom/monday-api-python-sdk) — Python
- [**ProdPerfect/monday**](https://github.com/ProdPerfect/monday) — Python
- [**GearPlug/monday-python**](https://github.com/GearPlug/monday-python) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
