---
name: pp-cricapi
description: "Live cricket scores, fixtures, and fantasy data from CricAPI / Cricket Data, with offline SQLite and the first..."
author: "Rai"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cricapi-pp-cli
---

# Cricapi — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cricapi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cricapi --cli-only
   ```
2. Verify: `cricapi-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Live cricket scores, fixtures, players, series, and fantasy data — with first-class MCP and an offline SQLite store.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**countries** — Country / team roster

- `cricapi-pp-cli countries` — All supported countries / international teams

**matches** — Cricket matches — live, upcoming, and historical

- `cricapi-pp-cli matches current` — Live and about-to-start matches across all cricket
- `cricapi-pp-cli matches info` — Detailed information for a single match
- `cricapi-pp-cli matches list` — Match list with optional team/series name search
- `cricapi-pp-cli matches points` — Fantasy points for a match
- `cricapi-pp-cli matches score` — Quick live-score snapshot across all live matches
- `cricapi-pp-cli matches scorecard` — Full scorecard with batting/bowling per innings
- `cricapi-pp-cli matches squad` — Both team squads for a match

**players** — Player roster and career data

- `cricapi-pp-cli players info` — Career stats for a single player
- `cricapi-pp-cli players search` — Player search by name

**series** — Cricket series and tournaments

- `cricapi-pp-cli series info` — Series details including matches, squads, and points table
- `cricapi-pp-cli series list` — Recent and current series


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cricapi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Set your API key via environment variable:

```bash
export CRICAPI_API_KEY="<your-key>"
```

Or persist it in `~/.config/cricapi-pp-cli/config.toml`.

Run `cricapi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cricapi-pp-cli countries --agent --select id,name,status
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
cricapi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cricapi-pp-cli feedback --stdin < notes.txt
cricapi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cricapi-pp-cli/feedback.jsonl`. They are never POSTed unless `CRICAPI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CRICAPI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cricapi-pp-cli profile save briefing --json
cricapi-pp-cli --profile briefing countries
cricapi-pp-cli profile list --json
cricapi-pp-cli profile show briefing
cricapi-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cricapi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cricapi-pp-mcp -- cricapi-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cricapi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cricapi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cricapi-pp-cli <command> --help`.
