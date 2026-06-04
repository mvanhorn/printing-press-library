---
name: pp-hotel-goat
description: "Free Google Hotels CLI — per-hotel data with deep booking links, agent-native JSON, and local wishlist. No API key. Trigger phrases: `find a hotel in <city>`, `search hotels for <city> <dates>`, `cheapest dates for <city>`, `hotels near <address>`, `compare hotel prices for <city>`, `what hotels are available in <city>`, `save this hotel`, `use hotel-goat`, `run hotel-goat`."
author: "kothari-nikunj"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hotel-goat-pp-cli
    install:
      - kind: go
        bins: [hotel-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/cmd/hotel-goat-pp-cli
---

# Google Hotels — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hotel-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install hotel-goat --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `hotel-goat-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/cmd/hotel-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for hotel-goat whenever a user needs to search hotels by location and date range, find the cheapest dates in a window, find hotels near a specific address, or save properties to a local wishlist. Pair it with flight-goat when composing an itinerary; both share the same flag and envelope shape so a future travel-pp-cli orchestrator is a thin merge layer. Skip it when the user wants to actually book (Google Hotels is metasearch — hotel-goat surfaces the OTA deep-link, then the user books on the OTA).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`hotels`** — Dotted-path selection through nested results.prices[], results.booking_urls, results.images[]. Returns only the fields the agent needs.

  _Use when you need only a couple of fields per hotel rather than the full nested response._

  ```bash
  hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.rating,results.price_per_night,results.booking_urls.primary
  ```

## Command Reference

**hotels** — Hotel search results from Google Hotels for a location + date range

- `hotel-goat-pp-cli hotels` — Search hotels by location and date range.

**hotel** — Single-property commands

- `hotel-goat-pp-cli hotel show <property-token>` — Full property detail by Google's property_token
- `hotel-goat-pp-cli hotel reviews <property-token>` — Review breakdown for one property


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hotel-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Filtered search with European currency

```bash
hotel-goat-pp-cli hotels "Paris" 2026-07-20 2026-07-23 --currency EUR --hotel-class 4,5 --max-price 300 --sort cheapest
```

Premium 4-5 star hotels in Paris under €300/night, sorted cheapest.

### Agent-friendly nested-field selection

```bash
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.rating,results.price_per_night,results.booking_urls.primary
```

Returns only the four fields the agent needs, not the full per-result payload.

### Property detail by token

```bash
hotel-goat-pp-cli hotel show <property-token>
```

Full property details (amenities, nearby places, full image gallery) from a property_token returned by the search.

### Save and recall properties

```bash
hotel-goat-pp-cli wishlist add <property-token> && hotel-goat-pp-cli wishlist list
```

Local-SQLite wishlist: save properties from searches, list them later.

### Health check

```bash
hotel-goat-pp-cli doctor
```

Confirms network reachability to Google Hotels and that the parser still finds AF_initDataCallback blobs.

## Auth Setup

No authentication required. Google Hotels SSR is publicly accessible. Set `Accept-Language` via the system locale; pass `--currency EUR` to get pricing in another currency.

Run `hotel-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.price_per_night,results.booking_urls.primary
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
hotel-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hotel-goat-pp-cli feedback --stdin < notes.txt
hotel-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.hotel-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `HOTEL_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOTEL_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hotel-goat-pp-cli profile save briefing --json
hotel-goat-pp-cli --profile briefing hotels "San Francisco" 2026-08-15 2026-08-17
hotel-goat-pp-cli profile list --json
hotel-goat-pp-cli profile show briefing
hotel-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hotel-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/cmd/hotel-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hotel-goat-pp-mcp -- hotel-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hotel-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hotel-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hotel-goat-pp-cli <command> --help`.
