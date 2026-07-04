---
name: pp-workiz
description: "Every Workiz dispatch and pipeline workflow, plus crew utilization, revenue, and conversion views no other Workiz tool has. Trigger phrases: `workiz jobs`, `check crew schedule`, `dispatch board`, `lead conversion workiz`, `field service jobs`, `use workiz`, `run workiz`."
author: "Eldar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - workiz-pp-cli
    install:
      - kind: go
        bins: [workiz-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/cmd/workiz-pp-cli
---

# Workiz — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `workiz-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install workiz --cli-only
   ```
2. Verify: `workiz-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/cmd/workiz-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Workiz has no CLI, MCP server, or agent-native tool today — every existing integration is a thin polling script or a hand-rolled SDK wrapper with zero cross-entity intelligence. This CLI absorbs every job/lead/client/team/time-off operation from the SDK ecosystem, then adds local joins across your synced data to answer questions the live API simply can't: who's overbooked this week, which lead sources convert, and what changed since you last checked.

## When to Use This CLI

Use this CLI for FSM dispatch and pipeline work on Workiz data: listing/creating/scheduling jobs and leads, assigning crew, and answering cross-entity questions (crew utilization, lead conversion, revenue by source, data completeness) that require joining synced local data. It is the right tool whenever an agent needs to read or mutate Workiz jobs/leads/clients/team/time-off, or needs an aggregate view the live API doesn't expose.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time webhook delivery — Workiz has no webhook registration endpoint; every integration (including this one) polls CreatedDate.
- Do not use this CLI to delete jobs, leads, or clients — no delete endpoint exists anywhere in the Workiz API.
- Do not use this CLI for invoicing/payment processing details beyond the price fields Workiz exposes — that lives in Workiz's billing UI, not the API.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`team bottleneck --week`** — See per-crew scheduled load and catch double-bookings or time-off conflicts before they become no-shows.

  _Use this for dispatch planning when you need to know who's overbooked or double-booked this week, not just who's on the roster._

  ```bash
  workiz-pp-cli team bottleneck --week --agent
  ```
- **`lead funnel`** — See which lead sources actually turn into paid jobs, with conversion rate and average resulting job value per source.

  _Use this to decide where marketing spend should go, instead of manually eyeballing lead and job dates side by side._

  ```bash
  workiz-pp-cli lead funnel --since 30d --agent
  ```
- **`job revenue`** — Roll up total and outstanding job value by lead source and job status.

  _Use this for a dollar-value rollup by source/status. For lead-to-job conversion counts, use 'lead funnel' instead._

  ```bash
  workiz-pp-cli job revenue --group-by source --agent
  ```
- **`job search`** — Search job notes, lead notes, and comments for free text across your entire synced history.

  _Use this for free-text search inside notes/comments. For structured filtering by status/date/open, use 'job list'/'lead list' flags instead._

  ```bash
  workiz-pp-cli job search "leak" --agent
  ```

### Agent-native plumbing
- **`job audit`** — Find jobs, leads, and clients missing phone, email, amount, or crew fields that would block a downstream billing push.

  _Run this before pushing newly created jobs into a billing or CRM pipeline._

  ```bash
  workiz-pp-cli job audit --agent
  ```
- **`digest`** — See everything new or changed across jobs and leads since your last check, grouped by entity.

  _Use this instead of hand-maintaining your own polling cursor to avoid double-processing records._

  ```bash
  workiz-pp-cli digest --since 24h --agent
  ```

## Command Reference

**customer** — Customers (clients)

- `workiz-pp-cli customer create` — Create a new client
- `workiz-pp-cli customer get` — Get a client by id

**job** — Scheduled service calls (jobs)

- `workiz-pp-cli job assign` — Assign a crew member to a job
- `workiz-pp-cli job create` — Create a new job
- `workiz-pp-cli job get` — Get a job by UUID
- `workiz-pp-cli job list` — List jobs (paginated)
- `workiz-pp-cli job unassign` — Unassign a crew member from a job
- `workiz-pp-cli job update` — Update a job's schedule

**lead** — Pre-job estimates (leads)

- `workiz-pp-cli lead assign` — Assign a crew member to a lead
- `workiz-pp-cli lead create` — Create a new lead
- `workiz-pp-cli lead get` — Get a lead by UUID
- `workiz-pp-cli lead list` — List leads (paginated)
- `workiz-pp-cli lead unassign` — Unassign a crew member from a lead
- `workiz-pp-cli lead update` — Update a lead's schedule

**team** — Crew (team) members

- `workiz-pp-cli team get` — Get a team member by id
- `workiz-pp-cli team list` — List team members

**timeoff** — Crew time-off records

- `workiz-pp-cli timeoff get` — Get time-off records for a specific team member
- `workiz-pp-cli timeoff list` — List time-off records


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
workiz-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning dispatch check

```bash
workiz-pp-cli team bottleneck --week --agent --select crew,scheduled_hours,conflicts
```

Narrow a deeply-nested crew/job/timeoff join down to just the fields a dispatcher needs before assigning today's calls.

### Marketing spend review

```bash
workiz-pp-cli lead funnel --since 90d --agent
```

Rank lead sources by conversion rate and average resulting job value over the last quarter.

### Pre-billing sweep

```bash
workiz-pp-cli job audit --agent
```

Find jobs missing phone/email/amount before they get pushed into a billing pipeline.

### Catch up after time away

```bash
workiz-pp-cli digest --since 3d --agent
```

See every job/lead that changed while you weren't watching, without hand-maintaining a cursor.

## Auth Setup

Workiz uses a two-part credential: an API token embedded in every request URL, and an API secret sent in the body of write calls. Enable the Developer API add-on from Feature Center in the Workiz app, then find both under Settings > Integrations > Developer. Set WORKIZ_API_TOKEN and WORKIZ_API_SECRET (writes only need the secret), or persist the token on disk with 'workiz-pp-cli auth set-token <token>' (the secret still needs WORKIZ_API_SECRET -- there's no on-disk store for it).

Run `workiz-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  workiz-pp-cli customer get mock-value --agent --select id,name,status
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
workiz-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
workiz-pp-cli feedback --stdin < notes.txt
workiz-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/workiz-pp-cli/feedback.jsonl`. They are never POSTed unless `WORKIZ_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WORKIZ_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
workiz-pp-cli profile save briefing --json
workiz-pp-cli --profile briefing customer get mock-value
workiz-pp-cli profile list --json
workiz-pp-cli profile show briefing
workiz-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `workiz-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/workiz/cmd/workiz-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add workiz-pp-mcp -- workiz-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which workiz-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   workiz-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `workiz-pp-cli <command> --help`.
