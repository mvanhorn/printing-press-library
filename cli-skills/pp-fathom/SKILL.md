---
name: pp-fathom
description: "Sync your Fathom meetings once, then search, analyze, and act on them forever — offline, at scale, without burning API quota. Trigger phrases: `what did I promise in my calls`, `how often has pricing come up in meetings`, `brief me on my Acme calls`, `which team members are in too many meetings`, `find a meeting where we discussed X`, `search my fathom meetings`, `use fathom`, `run fathom-pp-cli`."
author: "Nikica Jokic"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - fathom-pp-cli
    install:
      - kind: go
        bins: [fathom-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-cli
---

# Fathom — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `fathom-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install fathom --cli-only
   ```
2. Verify: `fathom-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

fathom-pp-cli pulls every meeting, transcript, summary, and action item into a local SQLite store, then unlocks cross-meeting intelligence no MCP server or web UI can provide: commitment tracking across all your calls, topic trend analysis over weeks, pre-call account briefs, pipeline velocity detection, and team meeting-load audits.

## When to Use This CLI

Use fathom-pp-cli when you need cross-meeting intelligence that the Fathom web UI and MCP servers cannot provide: commiment audits across all calls, topic trend analysis over weeks or months, pre-call account briefs, or pipeline cadence monitoring. Run sync once per day to keep the local store current, then all analysis commands run offline. Not a substitute for the Fathom web UI for reviewing individual recordings or managing your account.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`commitments`** — See every open action item you promised across all calls — grouped by meeting, assignee, and date — without opening a single recording.

  _Use when you need a weekly accountability audit of all meeting commitments before a 1:1 or team sync._

  ```bash
  fathom-pp-cli commitments --assignee me --since 30d --agent
  ```
- **`topics`** — Find out how often 'pricing,' 'onboarding,' or any keyword has surfaced in your meetings over the past N weeks — with week-over-week trend.

  _Use before a board meeting or quarterly review to synthesize what themes have been dominating customer calls._

  ```bash
  fathom-pp-cli topics --terms pricing,onboarding --weeks 12 --agent
  ```
- **`velocity`** — Track whether your meeting cadence with a key account is accelerating, stable, or stalling — month by month.

  _Use for pipeline health reviews: a stalling cadence with a key account is an early warning signal before the deal goes cold._

  ```bash
  fathom-pp-cli velocity --domain stripe.com --months 6 --agent
  ```
- **`workload`** — See which team members are spending the most hours in meetings per week and whether the load is worsening.

  _Use in 1:1 prep or team planning to identify who is in 'meeting hell' before assigning more collaborative work._

  ```bash
  fathom-pp-cli workload --team Engineering --weeks 4 --threshold 15 --agent
  ```

### Agent-native plumbing
- **`brief`** — Get a chronological history of every meeting with a specific person or company — past topics, open action items, last contact date — before you join a call.

  _Use immediately before a customer call to surface prior commitments and context without opening multiple browser tabs._

  ```bash
  fathom-pp-cli brief --domain acme.com --agent
  ```
- **`account`** — View a complete, domain-keyed history with any company: every meeting, topics discussed, action items, and cadence — in one structured output.

  _Use during account reviews, renewal prep, or CRM updates to get a full picture of all interactions with a company._

  ```bash
  fathom-pp-cli account --domain notion.so --agent
  ```

### Operational tooling
- **`stale`** — Find recordings that were captured but have no transcript, summary, or action items synced — useful for operators debugging pipeline gaps.

  _Use on Monday morning to audit which recordings from last week are missing data before your team needs them._

  ```bash
  fathom-pp-cli stale --since 7d --agent
  ```
- **`crm-gaps`** — Surface CRM-matched meetings where no action items were logged — calls that touched active deals but left no paper trail.

  _Use in RevOps audits to find sales calls where reps talked to prospects but forgot to log next steps in the CRM._

  ```bash
  fathom-pp-cli crm-gaps --since 30d --agent
  ```
- **`coverage`** — Track how reliably a recurring meeting (weekly planning, standup, 1:1) is being recorded over time.

  _Use to verify that mandatory-record meetings are actually being captured before auditing team performance._

  ```bash
  fathom-pp-cli coverage --pattern 'Weekly Planning' --weeks 10 --agent
  ```

## Command Reference

**meetings** — Meeting recordings with transcripts, summaries, and action items

- `fathom-pp-cli meetings` — List meetings with optional filters and included data

**recordings** — Individual recording data: transcripts and summaries

- `fathom-pp-cli recordings get-summary` — Get AI-generated meeting summary in markdown format
- `fathom-pp-cli recordings get-transcript` — Get full transcript for a recording with speaker attribution and timestamps

**team-members** — Members of your teams

- `fathom-pp-cli team-members` — List team members, optionally filtered by team name

**teams** — Teams your account has access to

- `fathom-pp-cli teams` — List all teams accessible to your account

**webhooks** — Webhooks for async meeting completion notifications

- `fathom-pp-cli webhooks create` — Create a webhook to receive meeting data on completion
- `fathom-pp-cli webhooks delete` — Delete a webhook by ID


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
fathom-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Export local store for use on another machine

```bash
# Export all synced meetings to JSONL (one record per line)
fathom-pp-cli db export --format jsonl --output ~/fathom-backup.jsonl

# Export as SQLite file
fathom-pp-cli db export --format sqlite --output ~/fathom-backup.db

# Restore on another machine (no API calls, no re-sync needed)
fathom-pp-cli db restore --format jsonl --input ~/fathom-backup.jsonl
fathom-pp-cli db restore --format sqlite --input ~/fathom-backup.db
```

Pure local operation — reads from SQLite, writes to file. Run `sync --full` once, then export and share. On the new machine: install `fathom-pp-cli`, set `FATHOM_PP_CLI_API_KEY`, restore from the file, and all commands work offline immediately.

### Find a meeting by topic or keyword

```bash
fathom-pp-cli search "data pipeline clearpack" --limit 5
fathom-pp-cli search "joey action items" --json --select title,date,url,snippet
```

Full-text search across all synced transcripts, summaries, action items, and titles. Returns ranked results with highlighted snippet showing where the term matched. Searches offline — no API quota. Run `sync --full` first.

### Pre-call brief for a customer

```bash
fathom-pp-cli brief --domain acme.com --agent --select meetings.title,meetings.date,open_action_items
```

Pull all history with Acme before joining a call — past topics, open commitments, last meeting date.

### Weekly commitment audit

```bash
fathom-pp-cli commitments --since 7d --agent --select meeting_title,description,assignee.name,completed
```

Surface every action item from last week's calls, grouped by meeting — know what's still open before Monday standup.

### Topic trend for board prep

```bash
fathom-pp-cli topics --terms 'pricing,churn,roadmap' --weeks 12 --agent
```

Show week-over-week frequency of key themes across all customer calls for the past quarter.

### Pipeline velocity check

```bash
fathom-pp-cli velocity --domain acme.com --months 6 --agent
```

See whether your meeting cadence with Acme has been increasing, flat, or stalling — early warning for deals going cold.

### Team meeting load audit

```bash
fathom-pp-cli workload --team Engineering --weeks 4 --threshold 15 --agent --select name,weekly_hours,trend
```

Find which engineers are spending over 15h/week in meetings and whether it's getting worse.

## Auth Setup

Fathom uses an API key passed via the X-Api-Key header. Generate one at fathom.video → User Settings → API Access. Set FATHOM_PP_CLI_API_KEY in your environment, then run doctor to verify.

Run `fathom-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  fathom-pp-cli meetings --agent --select id,name,status
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
fathom-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
fathom-pp-cli feedback --stdin < notes.txt
fathom-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.fathom-pp-cli/feedback.jsonl`. They are never POSTed unless `FATHOM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FATHOM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
fathom-pp-cli profile save briefing --json
fathom-pp-cli --profile briefing meetings
fathom-pp-cli profile list --json
fathom-pp-cli profile show briefing
fathom-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `fathom-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/fathom/cmd/fathom-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add fathom-pp-mcp -- fathom-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which fathom-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   fathom-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `fathom-pp-cli <command> --help`.
