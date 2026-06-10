---
name: pp-sendfox
description: "The only SendFox CLI. Write a newsletter with AI, assign a list, send from one command. Trigger phrases: `send a SendFox newsletter`, `write an email campaign`, `schedule my newsletter`, `check my SendFox stats`, `use sendfox`, `add a contact to SendFox`."
author: "Dilip"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sendfox-pp-cli
---

# SendFox — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sendfox-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sendfox --cli-only
   ```
2. Verify: `sendfox-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the full SendFox campaign lifecycle in a terminal-native tool. The write command uses Claude to generate your subject line and email body from a plain-English brief, then creates the draft and sends it. Stats, trends, and contact management are all offline-capable after a sync.

## When to Use This CLI

Use sendfox-pp-cli when an agent or user needs to draft, schedule, or send a SendFox newsletter without opening a browser. Ideal for content creators who want AI-assisted writing and a single-command send workflow.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### AI-powered writing
- **`write`** — Describe a topic in plain English and get a complete, ready-to-send newsletter with subject line, preview text, and full HTML body.

  _Use this when the agent needs to draft and send a newsletter without human copywriting time._

  ```bash
  sendfox-pp-cli write --topic 'How I grew my newsletter to 5000 subscribers' --list 42 --send
  ```
- **`write`** — Get 3 AI-generated subject line options with a rationale for each, then pick one before the draft is created.

  _Use when the agent should let the user choose a subject line before committing to a send._

  ```bash
  sendfox-pp-cli write --topic 'My favorite tools of 2026' --ab-subjects 3
  ```
- **`write`** — Give it a file with one topic per line and it schedules one AI-written newsletter per topic, spaced out over coming weeks.

  _Use when the agent needs to pre-schedule an entire content calendar in one operation._

  ```bash
  sendfox-pp-cli write --from-topics topics.txt --schedule-weekly --list 42
  ```

### Local state that compounds
- **`stats trends`** — See how your open rates, click rates, and unsubscribes have changed over the last N days across all campaigns.

  _Use when the agent needs to assess whether engagement is improving or declining before changing send frequency._

  ```bash
  sendfox-pp-cli stats trends --days 30 --json
  ```
- **`stats funnel`** — A cross-campaign view of sent vs opened vs clicked vs unsubscribed to see your list health at a glance.

  _Use when the agent needs a quick pulse on list engagement before deciding to send or pause._

  ```bash
  sendfox-pp-cli stats funnel --json
  ```
- **`stats best-time`** — Analyzes historical campaign data to identify which days and times correlate with your highest open rates.

  _Use when scheduling a campaign and wanting to optimize the send time based on past performance._

  ```bash
  sendfox-pp-cli stats best-time
  ```

## Command Reference

**campaigns** — 

- `sendfox-pp-cli campaigns create` — Create a new campaign draft
- `sendfox-pp-cli campaigns delete` — Delete a draft campaign (only works on unsent drafts)
- `sendfox-pp-cli campaigns get` — Get a specific campaign by ID
- `sendfox-pp-cli campaigns list` — List all campaigns (100 per page)
- `sendfox-pp-cli campaigns send` — Send a draft campaign immediately (must have at least one list assigned)
- `sendfox-pp-cli campaigns stats` — Get performance stats for a campaign
- `sendfox-pp-cli campaigns update` — Update a draft campaign (only works on unsent drafts)

**contacts** — 

- `sendfox-pp-cli contacts create` — Create a new contact
- `sendfox-pp-cli contacts get` — Get a specific contact by ID
- `sendfox-pp-cli contacts list` — Get all contacts (paginated)
- `sendfox-pp-cli contacts remove_from_list` — Remove a contact from a specific list
- `sendfox-pp-cli contacts unsubscribe` — Unsubscribe a contact by email

**lists** — 

- `sendfox-pp-cli lists create` — Create a new subscriber list
- `sendfox-pp-cli lists get` — Get a specific list by ID
- `sendfox-pp-cli lists list` — Get all subscriber lists (paginated)

**me** — 

- `sendfox-pp-cli me` — Get authenticated user info


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sendfox-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Write and send a newsletter in one command

```bash
sendfox-pp-cli write --topic 'How I built my audience in 6 months' --list 42 --send
```

AI-generates subject, preview text, and HTML body, creates the draft, assigns list 42, and sends immediately.

### Schedule a week of newsletters from a topics file

```bash
sendfox-pp-cli write --from-topics topics.txt --schedule-weekly --list 42
```

Reads one topic per line, generates each campaign, and schedules them 7 days apart starting tomorrow.

### Check engagement trends with select fields

```bash
sendfox-pp-cli stats trends --days 30 --json --select open_rate,click_rate,date
```

Returns time-series open and click rate data from local SQLite for agent-readable trend analysis.

### Find the best time to send

```bash
sendfox-pp-cli stats best-time --agent
```

Analyzes past campaign send times against open rates and returns a ranked list of optimal send windows.

### Clone and remix a past campaign

```bash
sendfox-pp-cli campaigns clone --id 18 --subject 'Updated: My best tips for 2026'
```

Copies campaign 18 HTML into a new draft with a fresh subject line, ready to edit and resend.

## Auth Setup

Run sendfox-pp-cli setup on first use. It walks you through getting a Personal Access Token at sendfox.com/account/oauth, validates it, fetches your subscriber lists, and saves everything to ~/.config/sendfox/config.yaml. Every subsequent command reads from that config.

Run `sendfox-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sendfox-pp-cli campaigns list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sendfox-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sendfox-pp-cli feedback --stdin < notes.txt
sendfox-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.sendfox-pp-cli/feedback.jsonl`. They are never POSTed unless `SENDFOX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SENDFOX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sendfox-pp-cli profile save briefing --json
sendfox-pp-cli --profile briefing campaigns list
sendfox-pp-cli profile list --json
sendfox-pp-cli profile show briefing
sendfox-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sendfox-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add sendfox-pp-mcp -- sendfox-pp-mcp
```

Verify: `claude mcp list`

To run as an HTTP server instead of stdio, set `SENDFOX_MCP_PORT` (or `MCP_PORT`) before starting:

```bash
SENDFOX_MCP_PORT=8080 sendfox-pp-mcp
```

This starts a StreamableHTTP MCP server on the given port, useful for cloud-hosted agents.

## Direct Use

1. Check if installed: `which sendfox-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sendfox-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sendfox-pp-cli <command> --help`.
