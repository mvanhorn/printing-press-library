---
name: pp-motohunt
description: "Search motorcycle and ATV listings from the terminal — with MotoHunt's MSRP/average-price/deal-rating data exposed as Trigger phrases: `search motohunt`, `find a motorcycle for sale`, `is this bike a good deal`, `search atvhunt`, `watch motorcycle listings`, `use motohunt`, `run motohunt`."
author: "richardadonnell"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - motohunt-pp-cli
    install:
      - kind: go
        bins: [motohunt-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/motohunt/cmd/motohunt-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/motohunt/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# MotoHunt — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `motohunt-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install motohunt --cli-only
   ```
2. Verify: `motohunt-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/motohunt/cmd/motohunt-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

MotoHunt has no public API; this CLI scrapes its server-rendered HTML with one HTTP GET per page and returns clean JSON. It surfaces the price-research data (base MSRP, average listing price, deal rating) that makes 'is this a good deal?' answerable, ranks synced inventory by under-market gap with `deal`, watches saved searches for new listings and price drops, and covers the ATV sister site via `--site atv`.

## When to Use This CLI

Use this CLI when an agent or script needs to search used motorcycle or ATV inventory, pull a listing's specs and price-research, rank a search by under-market deal gap, or watch a saved search for new listings and price drops. It is the only programmatic access to MotoHunt's MSRP/ALP/deal-rating data.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Price intelligence
- **`get`** — See a listing's MSRP, average listing price, and deal rating as typed fields, not buried prose.

  _Reach for this to answer 'is this listing actually a good price?' without reading the page._

  ```bash
  motohunt-pp-cli get 13256655 --agent --select base_msrp,alp,deal_rating
  ```
- **`deal`** — Rank synced listings by how far the asking price sits below the average listing price.

  _Reach for this to surface the biggest under-market deals across a whole search._

  ```bash
  motohunt-pp-cli deal --make Harley-Davidson --location 33705 --limit 20 --agent
  ```

### Local state that compounds
- **`watch run`** — Re-run saved searches and report new listings and price drops since the last run.

  _Reach for this to monitor a hunt over time instead of re-searching by hand._

  ```bash
  motohunt-pp-cli watch run --agent
  ```

### Coverage
- **`search`** — Search motorcycles (motohunt.com) or ATV/UTV/SxS (atvhunt.com) from one binary via --site.

  _Reach for --site atv when the hunt is four-wheelers instead of bikes._

  ```bash
  motohunt-pp-cli search --site atv --location 33705 --agent
  ```

## Command Reference

**listings** — Search and inspect motorcycle/ATV listings

- `motohunt-pp-cli listings get` — Fetch a single listing detail page
- `motohunt-pp-cli listings search` — Search listings; returns links to listing detail pages (use the hand-built `search` command for parsed cards)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
motohunt-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Best-deal Harleys near me

```bash
motohunt-pp-cli search --make Harley-Davidson --location 33705 --sort c --limit 40 --agent --select title,price,deal_rating,location
```

Top under-market Harleys, slimmed to the fields that matter.

### Is this listing a good price?

```bash
motohunt-pp-cli get 13256655 --agent --select price,base_msrp,alp,deal_rating
```

Compare the ask against MSRP and the average listing price.

### Watch a hunt for price drops

```bash
motohunt-pp-cli watch add --name 'gs near me' --make BMW --model R-1250-GS --location 33705 && motohunt-pp-cli watch run --agent
```

Save a search, then diff it over time for new listings and price drops.

## Auth Setup

No authentication required.

Run `motohunt-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  motohunt-pp-cli listings get mock-value --agent --select id,name,status
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
motohunt-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
motohunt-pp-cli feedback --stdin < notes.txt
motohunt-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/motohunt-pp-cli/feedback.jsonl`. They are never POSTed unless `MOTOHUNT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MOTOHUNT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
motohunt-pp-cli profile save briefing --json
motohunt-pp-cli --profile briefing listings get mock-value
motohunt-pp-cli profile list --json
motohunt-pp-cli profile show briefing
motohunt-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `motohunt-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/motohunt/cmd/motohunt-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add motohunt-pp-mcp -- motohunt-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which motohunt-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   motohunt-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `motohunt-pp-cli <command> --help`.
