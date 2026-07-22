---
name: wolt
description: "Browse Wolt food delivery from the terminal: list cities, find nearby venues, check delivery ETA + fees, browse menus + prices, search dishes within a venue, compare venues side-by-side, find cuisine bottlenecks. Read-only — no order placement. Use when the user mentions Wolt, asks 'what should I order on Wolt', or wants menu/price/ETA info for a Wolt venue."
author: "Amit"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools:
  - Read
  - Bash
metadata:
  openclaw:
    requires:
      bins:
        - wolt-pp-cli
---

# Wolt — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wolt-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wolt --cli-only
   ```
2. Verify: `wolt-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/wolt/cmd/wolt-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

### Discovery (find venues)

- `wolt-pp-cli cities --json` — List every city Wolt operates in (slug, country code, timezone, coordinates).
- `wolt-pp-cli pages list-restaurants-near --lat <lat> --lon <lon> --json` — Restaurants near a coordinate. Returns sections with name, slug, online, ETA, delivery fee, rating, tags.
- `wolt-pp-cli pages search --q "<query>" --target venues --lat <lat> --lon <lon> --json` — Search venues by query + location.
- `wolt-pp-cli pages search --q "<query>" --target items --lat <lat> --lon <lon> --json` — Search dishes across nearby venues.

### Venue details (single venue)

- `wolt-pp-cli order-xp <slug> --json` — Per-venue snapshot: open status, next-close time, delivery ETA, delivery fee, order minimum, delivery configs.

### Menu (per venue, full data) — the most useful endpoint

- `wolt-pp-cli menu show <slug> --json` — Full menu payload: categories + items + prices + dietary preferences + availability.
- `wolt-pp-cli menu items <slug> --json` — Just the items (id, name, price, category, dietary tags). Use `--category "<name>"` to filter, `--limit N` to cap.
- `wolt-pp-cli menu categories <slug> --json` — Just the categories with item counts.
- `wolt-pp-cli menu search <slug> --q "<term>" --max-price <cents> --json` — Substring search within a venue's menu, optionally cap price (in cents, sorted cheapest first).
- All `menu *` commands accept `--lang en|fi|sv|...` for multilingual menus.

### Aggregations (novel)

- `wolt-pp-cli venues-now --lat <lat> --lon <lon> --max-eta 25 --cuisine sushi --json` — "What's open right now near me, delivering within N minutes, matching cuisine tag." Replaces clicking through cards on the Wolt SPA.
- `wolt-pp-cli venues-compare --slugs <a>,<b>,<c> --json` — Side-by-side open status, next-close, delivery configs, order minimum across multiple venues.
- `wolt-pp-cli cuisine-bottleneck --lat <lat> --lon <lon> --top 10 --json` — Which cuisines have the longest average ETA right now near a coordinate. Useful for "what's slow tonight."
- `wolt-pp-cli track <share-link>` — Parse a Wolt order share URL and emit the order id. **Stub** — the live tracking JSON endpoint is undocumented; the command extracts the id and points the user to the browser.

### Worked example — order what you actually want

```bash
# 1. Find venues open right now within 25 min that serve noodles
wolt-pp-cli venues-now --lat 60.1699 --lon 24.9384 --max-eta 25 --cuisine asian --json --select count,venues.slug,venues.name,venues.estimate_min

# 2. Pick a slug from the result, then browse the menu
wolt-pp-cli menu show noodle-story-kamppi --json --select categories.name,categories.item_count

# 3. Search the menu for what you want, under €15
wolt-pp-cli menu search noodle-story-kamppi --q noodle --max-price 1500 --json --select items.name,items.price,items.category
```


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wolt-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `wolt-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wolt-pp-cli cities --agent --select id,name,status
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
wolt-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wolt-pp-cli feedback --stdin < notes.txt
wolt-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.wolt-pp-cli/feedback.jsonl`. They are never POSTed unless `WOLT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WOLT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wolt-pp-cli profile save briefing --json
wolt-pp-cli --profile briefing cities
wolt-pp-cli profile list --json
wolt-pp-cli profile show briefing
wolt-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wolt-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add wolt-pp-mcp -- wolt-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wolt-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wolt-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wolt-pp-cli <command> --help`.
