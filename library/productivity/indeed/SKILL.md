---
name: pp-indeed
description: "Search Indeed from your terminal — with a local job history no scraper keeps Trigger phrases: `search indeed for jobs`, `find software engineer jobs`, `what's new on my job search`, `look up a job on indeed`, `use indeed`, `run indeed`."
author: "Isaac Quintero"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - indeed-pp-cli
    install:
      - kind: go
        bins: [indeed-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/indeed/cmd/indeed-pp-cli
---

# Indeed — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `indeed-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install indeed --cli-only
   ```
2. Verify: `indeed-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/indeed/cmd/indeed-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A read-only job-search CLI for Indeed. Search with all the filters (location, radius, date posted, job type, remote, salary floor), pull full job descriptions, and research companies. Every job you see is stored locally, so `new` shows only fresh postings for a saved search and `find` does offline full-text search across your whole history.

## When to Use This CLI

Reach for this CLI when an agent or user needs to search Indeed job listings, pull a full job description by key, research a company's rating/size, or track new postings for a recurring search over time. It is read-only: it never applies to jobs or submits resumes.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`new`** — Re-run a saved search and show only the postings that appeared since you last looked.

  _Reach for this to poll a query on a schedule without re-reading jobs you've already triaged._

  ```bash
  indeed-pp-cli new daily-remote --json
  ```
- **`find`** — Full-text search across every job you've ever synced, with no network call.

  _Use when you want to grep your whole job history, not just one fresh query._

  ```bash
  indeed-pp-cli find "rust kubernetes" --json --select key,title,company
  ```
- **`saved save`** — Persist a named query with all its filters so you can re-run it by name.

  _Use to define the recurring searches you care about once, then poll them._

  ```bash
  indeed-pp-cli saved save daily-remote "software engineer" --location Remote --posted 1 --sort date
  ```
- **`track`** — Add a job to a local shortlist you can list and annotate later.

  _Use to keep a working set of interesting jobs across sessions without logging in._

  ```bash
  indeed-pp-cli track 3700db3b90d43bc1
  ```

### Smarter filtering

- **`search`** — Drop results whose parsed salary falls below a threshold.

  _Use to cut low-paying noise that Indeed's own filters miss because salaries are free-text._

  ```bash
  indeed-pp-cli search "data engineer" --location "Austin, TX" --min-salary 120000 --json
  ```
- **`search`** — Run one keyword across several locations in a single command and dedup by job key.

  _Use when you're open to multiple metros and want one merged, deduped list._

  ```bash
  indeed-pp-cli search "product manager" --location "Austin,Dallas,Remote" --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**related** — Jobs related to a given job (Indeed "competitors jobs" feed).

- `indeed-pp-cli related` — List jobs similar to a given job key.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
indeed-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Fresh remote roles today

```bash
indeed-pp-cli search "backend engineer" --location Remote --posted 1 --sort date --json
```

Newest remote backend roles posted in the last day, as JSON.

### Narrow a verbose result set

```bash
indeed-pp-cli search "data scientist" --location "New York, NY" --json --select key,title,company,salary,formattedLocation
```

Use --select to keep only the fields you need from each job and avoid burning context on the full payload.

### Salary floor

```bash
indeed-pp-cli search "ml engineer" --location Remote --min-salary 150000 --csv
```

Drop anything under $150k (by parsed salary) and export to CSV.

### Poll a saved search

```bash
indeed-pp-cli new daily-remote --json
```

Only the jobs that are new since the last time you ran this saved search.

## Auth Setup

No authentication required.

Run `indeed-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  indeed-pp-cli related --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
indeed-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
indeed-pp-cli feedback --stdin < notes.txt
indeed-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.indeed-pp-cli/feedback.jsonl`. They are never POSTed unless `INDEED_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INDEED_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
indeed-pp-cli profile save briefing --json
indeed-pp-cli --profile briefing related
indeed-pp-cli profile list --json
indeed-pp-cli profile show briefing
indeed-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `indeed-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/indeed/cmd/indeed-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add indeed-pp-mcp -- indeed-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which indeed-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   indeed-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `indeed-pp-cli <command> --help`.
