---
name: pp-roadside-america
description: "Every offbeat roadside attraction on RoadsideAmerica. Trigger phrases: `quirky attractions near me`, `weird roadside stuff in Texas`, `world's largest things nearby`, `offbeat roadside attractions for my road trip`, `biggest tourist traps in a state`, `use roadside-america`, `run roadside-america`."
author: "David Bryson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - roadside-america-pp-cli
    install:
      - kind: go
        bins: [roadside-america-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/roadside-america/cmd/roadside-america-pp-cli
---

# Roadside America — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `roadside-america-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install roadside-america --cli-only
   ```
2. Verify: `roadside-america-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/roadside-america/cmd/roadside-america-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

RoadsideAmerica.com is the web's best catalog of quirky US & Canada tourist attractions, but it has no API and a paywalled app. This CLI turns it into an agent-native, pipe-friendly tool: find what's near a place or coordinates, browse a whole state, pull the full writeup, and slice by superlative categories like biggest/smallest/tallest/weird-food. Everything is cached locally (fresh-on-read), every record links back to its source, and the scraper stays a polite, attributing, user-initiated citizen of the site.

## When to Use This CLI

Use this CLI to discover offbeat, quirky, community-sourced roadside attractions in the US and Canada from RoadsideAmerica.com: what's near a place or coordinates, what's in a given state, the full writeup for a specific attraction, or superlative slices like the biggest/smallest/tallest things and weird food stops. It is ideal for road-trip planning and 'find me something weird nearby' tasks, and it caches everything locally for fast, repeatable, offline-friendly queries.

## Anti-triggers

Do not use this CLI for:
- Do not use for booking, ticketing, hours, or live availability — the data is descriptive, community-sourced, and not real-time.
- Do not treat addresses as authoritative for navigation; verify before driving.
- Do not use outside the US and Canada (the source only covers those).
- Do not use for mainstream tourism, restaurant reviews, or hotels — this is offbeat oddities only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local cache intelligence
- **`category`** — Find the biggest, smallest, tallest, or weird-food attractions (and more) by classifying cached attractions locally.

  _Reach for this when the user wants superlatives or a themed slice (giants, muffler-men, weird-food) rather than a place- or state-scoped list._

  ```bash
  roadside-america-pp-cli category biggest --json
  ```
- **`stats`** — Summarize the local cache: counts by state and by category, plus totals.

  _Use to understand coverage of the local cache before planning, or to answer 'which state has the most offbeat stuff cached'._

  ```bash
  roadside-america-pp-cli stats --agent
  ```
- **`random`** — Pick a random offbeat attraction, optionally constrained by state or category.

  _Use for serendipity or road-trip inspiration when the user has no specific target._

  ```bash
  roadside-america-pp-cli random --state TX
  ```

### Route & comparison
- **`trip`** — Collect quirky stops near a list of cities or coordinates in one call, deduped and labeled by stop.

  _Reach for this when planning a route and the user wants offbeat stops across several waypoints at once._

  ```bash
  roadside-america-pp-cli trip "Austin, TX" "Waco, TX" --radius 15 --json
  ```
- **`compare`** — Compare two states by offbeat-attraction count and surface a few top picks from each.

  _Use when the user is deciding between regions or wants a quick 'which state is weirder' answer._

  ```bash
  roadside-america-pp-cli compare TX CA
  ```

## Command Reference

**raw** — Raw RoadsideAmerica.com passthrough (HTML link/page extraction). Prefer the top-level near / state / show / category commands for structured output.

- `roadside-america-pp-cli raw by-state` — Raw attraction links for a US/Canada state (HTML fragment).
- `roadside-america-pp-cli raw detail` — Raw attraction detail page (HTML).
- `roadside-america-pp-cli raw nearby` — Raw nearby attraction links for coordinates (HTML fragment).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
roadside-america-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Quirky stops near coordinates, agent-friendly fields

```bash
roadside-america-pp-cli near 30.27,-97.74 --radius 25 --agent --select name,city,distance,source_url
```

Pass raw lat,lng to skip geocoding and select only the fields an agent needs.

### Weird food across a cached state

```bash
roadside-america-pp-cli category weird-food --json
```

Classifies cached attractions by food keywords; populate the cache with state/near first.

### Plan offbeat stops across a route

```bash
roadside-america-pp-cli trip "Austin, TX" "Waco, TX" "Dallas, TX" --radius 15 --json
```

Aggregates nearby attractions for each waypoint, deduped and labeled by stop.

### Full writeup with source attribution

```bash
roadside-america-pp-cli show 2055 --json
```

Returns structured name/address/writeup plus the RoadsideAmerica.com source URL.

## Auth Setup

No authentication required.

Run `roadside-america-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  roadside-america-pp-cli raw by-state --state example-value --agent --select id,name,status
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
roadside-america-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
roadside-america-pp-cli feedback --stdin < notes.txt
roadside-america-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/roadside-america-pp-cli/feedback.jsonl`. They are never POSTed unless `ROADSIDE_AMERICA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROADSIDE_AMERICA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
roadside-america-pp-cli profile save briefing --json
roadside-america-pp-cli --profile briefing raw by-state --state example-value
roadside-america-pp-cli profile list --json
roadside-america-pp-cli profile show briefing
roadside-america-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `roadside-america-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/roadside-america/cmd/roadside-america-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add roadside-america-pp-mcp -- roadside-america-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which roadside-america-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   roadside-america-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `roadside-america-pp-cli <command> --help`.
