---
name: pp-ufo
description: "The declassified UAP file archive in your terminal — browse, search, and download 162+ files from the PURSUE initiative Trigger phrases: `check UFO files`, `search declassified UAP`, `download PURSUE files`, `war.gov UFO`, `browse UFO archive`, `use ufo`."
author: "Dave Morin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ufo-goat-pp-cli
    install:
      - kind: go
        bins: [ufo-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/ufo/cmd/ufo-goat-pp-cli
---

# War.gov UFO — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ufo-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ufo-goat --cli-only
   ```
2. Verify: `ufo-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/ufo-goat/cmd/ufo-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The first CLI for the War.gov/UFO declassified files portal. Search across all four agencies (DoW, FBI, NASA, State), download files with resume support, track new release tranches, and discover video-PDF pairings — all from a single binary with offline SQLite storage.

## When to Use This CLI

Use this CLI when you need to browse, search, or download declassified UAP files from the War.gov PURSUE archive. Ideal for researchers tracking new releases, journalists investigating specific incidents, or agents that need structured access to government UFO data.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Automatically detect and fetch new file tranches as the government releases them on a rolling basis

  _Agents monitoring the PURSUE release schedule get notified of new files without manual checking_

  ```bash
  ufo-goat-pp-cli sync
  ```
- **`new`** — See exactly which files were added since your last sync — the 'what did I miss' command for rolling releases

  _When an agent needs to check for new declassified files without re-scanning the entire archive_

  ```bash
  ufo-goat-pp-cli new --since 7d
  ```

### Cross-agency intelligence
- **`timeline`** — View a chronological incident timeline spanning 1944-2025 across all four agencies

  _Researchers need to see the full picture: FBI case from 1947 next to a DoW mission report from 2024_

  ```bash
  ufo-goat-pp-cli timeline --after 1960 --before 1970
  ```
- **`pairs`** — Find video-PDF pairings so researchers can locate the document that accompanies a video and vice versa

  _41 videos have paired documents — this command surfaces the connections instantly_

  ```bash
  ufo-goat-pp-cli pairs --agent
  ```
- **`agencies`** — See which agency contributed what: file counts, types, date ranges, and coverage analysis

  _Quick answer to 'what did the FBI release vs NASA vs DoW'_

  ```bash
  ufo-goat-pp-cli agencies --json --select name,file_count,types
  ```
- **`locations`** — Aggregate incidents by geographic location for mapping and spatial analysis

  _25 incidents in Western US, 12 in Syria, 9 on the Moon — spatial patterns emerge from aggregation_

  ```bash
  ufo-goat-pp-cli locations --json
  ```

### Agent-native plumbing
- **`download`** — Download files with resume support, verification, and progress tracking for the 2.3 GB archive

  _The archive is 2.3 GB of PDFs alone — agents need reliable batch downloads with state tracking_

  ```bash
  ufo-goat-pp-cli download --agency FBI --resume
  ```

## Command Reference

**agencies** — Government agencies contributing to the PURSUE release

- `ufo-goat-pp-cli agencies` — List all contributing agencies with file counts

**files** — Declassified UAP files (PDFs, videos, images) from FBI, DoD, NASA, State Department

- `ufo-goat-pp-cli files get` — Get details of a specific file
- `ufo-goat-pp-cli files list` — List all declassified UAP files
- `ufo-goat-pp-cli files search` — Full-text search across file titles, descriptions, and locations


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ufo-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find all Moon incidents

```bash
ufo-goat-pp-cli files list --location Moon --json --select title,agency,incident_date
```

Surfaces Apollo-era lunar UAP observations from NASA files

### Download all FBI files

```bash
ufo-goat-pp-cli download --agency FBI --resume
```

Batch download with resume support for the 56 FBI contributions

### Cross-agency timeline for the 1960s

```bash
ufo-goat-pp-cli timeline --after 1960 --before 1970 --agent
```

See incidents across all agencies during the peak UFO era

### Find video-document pairs

```bash
ufo-goat-pp-cli pairs --json --select video_title,pdf_title,agency
```

Discover which videos have accompanying PDF reports

### Track new releases

```bash
ufo-goat-pp-cli sync && ufo-goat-pp-cli new
```

Fetch latest manifest and see what's been added since last check

## Auth Setup

No authentication required.

Run `ufo-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ufo-goat-pp-cli agencies --agent --select id,name,status
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
ufo-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ufo-goat-pp-cli feedback --stdin < notes.txt
ufo-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ufo-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `UFO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UFO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ufo-goat-pp-cli profile save briefing --json
ufo-goat-pp-cli --profile briefing agencies
ufo-goat-pp-cli profile list --json
ufo-goat-pp-cli profile show briefing
ufo-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ufo-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/ufo-goat/cmd/ufo-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ufo-pp-mcp -- ufo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ufo-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ufo-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ufo-goat-pp-cli <command> --help`.
