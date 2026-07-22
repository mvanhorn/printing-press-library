---
name: pp-stackadapt
description: "The first agent-native StackAdapt CLI: pacing, delivery drift Trigger phrases: `check my stackadapt campaigns`, `which campaigns are underpacing`, `are my stackadapt campaigns on budget`, `where is my ad budget being wasted`, `stackadapt campaign performance`, `use stackadapt`, `run stackadapt`."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - stackadapt-pp-cli
    install:
      - kind: go
        bins: [stackadapt-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/cmd/stackadapt-pp-cli
---

# StackAdapt — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `stackadapt-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install stackadapt --cli-only
   ```
2. Verify: `stackadapt-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/cmd/stackadapt-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI to analyze StackAdapt campaigns you already run: check pacing against budget, find under-performing or non-delivering campaigns, spot CTR/spend drift, and pull delivery reports. It is the right tool for read-only reporting and analysis, especially for agents and scripts that want JSON output.

## Anti-triggers

Do not use this CLI for:
- Creating, editing, pausing, or deleting campaigns, ads, or segments — this CLI is read-only; use the StackAdapt UI or the official SDK.
- Uploading audiences or creatives — out of scope.
- Real-time bidding or pixel firing — not what this CLI does.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Pacing & delivery observatory
- **`pacing`** — See which campaigns are under- or over-pacing against their budget, not just current spend.

  _Reach for this to catch campaigns that will overspend or underdeliver before the budget cycle ends._

  ```bash
  stackadapt-pp-cli pacing --advertiser adv_123 --agent
  ```
- **`delivery-drift`** — Track CTR, CPM, and spend drift week-over-week for a campaign and flag when performance degrades.

  _Reach for this to catch a slowly degrading campaign before it wastes budget._

  ```bash
  stackadapt-pp-cli delivery-drift --advertiser adv_123 --days 7 --agent
  ```

### Audience & efficiency
- **`bottleneck`** — Rank the highest-spend campaigns by worst ROAS or CPA, with a reason column.

  _Reach for this to find where ad budget is being wasted._

  ```bash
  stackadapt-pp-cli bottleneck --advertiser adv_123 --agent
  ```
- **`stale-campaigns`** — Find active campaigns with zero delivery in the last N days.

  _Reach for this to find live-but-not-delivering campaigns._

  ```bash
  stackadapt-pp-cli stale-campaigns --days 14 --agent
  ```

## Command Reference

**graphql** — Raw GraphQL query passthrough (advanced; prefer the typed commands)

- `stackadapt-pp-cli graphql` — Execute a raw GraphQL query against the StackAdapt API

**Offline store** — mirror objects locally, then search and query without API calls

- `stackadapt-pp-cli sync` — Pull advertisers, campaigns, campaign-groups, ads, and segments into a local SQLite store (`--resources a,b` to scope, `--limit N` per resource).
- `stackadapt-pp-cli search "<term>"` — Substring-search synced objects by name or any field offline (`--type <resource>` to scope).
- `stackadapt-pp-cli sql "<SELECT ...>"` — Run a read-only SQL query against the store's `resources` table (resource_type, id, name, data JSON, synced_at). Mutating statements are rejected.
- Any list command accepts `--data-source local` (store only) or `--data-source auto` (live, store fallback when the API is unreachable).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
stackadapt-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find under-pacing campaigns

```bash
stackadapt-pp-cli pacing --advertiser adv_123 --agent
```

Returns each campaign's pace (actual vs expected spend) so you can fix budget delivery before the cycle ends.

### Catch a degrading campaign

```bash
stackadapt-pp-cli delivery-drift --advertiser adv_123 --days 7 --agent
```

Shows CTR/CPM/spend drift week-over-week and flags when performance slips.

### Trim a verbose report for an agent

```bash
stackadapt-pp-cli report campaign-delivery --days 30 --agent --select records.campaign_name,records.metrics.cost,records.metrics.ctr
```

Delivery reports are large; --select with dotted paths returns only the fields an agent needs, saving context.

### Find where budget is wasted

```bash
stackadapt-pp-cli bottleneck --advertiser adv_123 --agent
```

Ranks high-spend campaigns by worst ROAS/CPA with a reason column.

## Auth Setup

This CLI uses the StackAdapt GraphQL API, which needs a GraphQL API token (the legacy REST key will not work). To get set up:

1. Ask your StackAdapt account manager for a GraphQL API token if you do not have one.
2. Export it so the CLI can read it:
   `export STACKADAPT_API_TOKEN=your-token`
3. Run `stackadapt-pp-cli doctor` to confirm it connects.

The token is sent as `Authorization: Bearer <token>`. The CLI is read-only and never modifies your campaigns.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  stackadapt-pp-cli graphql --query example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
stackadapt-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
stackadapt-pp-cli feedback --stdin < notes.txt
stackadapt-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/stackadapt-pp-cli/feedback.jsonl`. They are never POSTed unless `STACKADAPT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STACKADAPT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
stackadapt-pp-cli profile save briefing --json
stackadapt-pp-cli --profile briefing graphql --query example-value
stackadapt-pp-cli profile list --json
stackadapt-pp-cli profile show briefing
stackadapt-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `stackadapt-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/cmd/stackadapt-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add stackadapt-pp-mcp -- stackadapt-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which stackadapt-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   stackadapt-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `stackadapt-pp-cli <command> --help`.
