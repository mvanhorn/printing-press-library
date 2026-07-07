---
name: pp-tripadvisor
description: "Every Tripadvisor Content API endpoint Trigger phrases: `best hotels in Paris on tripadvisor`, `compare these restaurants on tripadvisor`, `what's the tripadvisor rating for`, `find attractions near these coordinates`, `tripadvisor reviews for`, `use tripadvisor`, `run tripadvisor`."
author: "David Bryson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tripadvisor-pp-cli
    install:
      - kind: go
        bins: [tripadvisor-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/cmd/tripadvisor-pp-cli
---

# Tripadvisor — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tripadvisor-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tripadvisor --cli-only
   ```
2. Verify: `tripadvisor-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/cmd/tripadvisor-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search hotels, restaurants, and attractions and get rating, review count, and ranking up front so an agent can rank and choose. Beyond the raw five endpoints, `best` and `nearby-best` search-then-rank in one call, `compare` puts places side by side, and `drift` flags a rating that slipped since you last looked — all backed by a local SQLite cache so repeat lookups are free and offline.

## When to Use This CLI

Use this CLI when an agent needs to search Tripadvisor places and compare them by rating, review count, or ranking — picking the best hotel/restaurant/attraction in a city or near a point, putting specific options side by side, or pulling a place's details, recent traveler reviews, and photos. The local cache makes repeat comparisons fast and offline.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to book hotels, tables, or tours — the Content API is read-only and this tool performs no bookings.
- Do not use it to fetch full review histories; the free Content API returns at most 5 recent reviews per location.
- Do not use it to scrape tripadvisor.com directly; that is bot-protected and out of scope.
- Do not use it to summarize or judge review sentiment; it returns the raw user-generated text for the agent to interpret.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Compare options
- **`best`** — Search a place type, auto-fetch details for the top hits, and return them ranked by rating, review count, or Tripadvisor ranking in one table.

  _Reach for this when the task is 'find the best X in Y' instead of calling find then show ten times yourself._

  ```bash
  tripadvisor-pp-cli best "Paris" --category hotels --top 5 --sort rating --agent
  ```
- **`compare`** — Pull details (and subratings + trip-type mix) for 2-5 location IDs and emit one structured comparison table.

  _Reach for this to decide between specific places an agent already shortlisted._

  ```bash
  tripadvisor-pp-cli compare 93450 258705 1641927 --agent
  ```
- **`nearby-best`** — From a lat/long, find nearby places, batch-fetch details up to a scan cap, filter by category and minimum rating, return the top K.

  _Reach for this for 'best-rated <type> near here' from coordinates._

  ```bash
  tripadvisor-pp-cli nearby-best "48.8606,2.3376" --category restaurants --min-rating 4.0 --top 5 --agent
  ```
- **`fit`** — Rank search results by how well their trip-type mix matches a declared traveler profile (families, couples, solo, business).

  _Reach for this to bias a shortlist toward who is actually traveling._

  ```bash
  tripadvisor-pp-cli fit "Orlando" --category hotels --traveler families --top 5 --agent
  ```

### Local state that compounds
- **`drift`** — Compare a location's stored rating/review-count snapshot against a fresh fetch and report the delta, flagging drops past a threshold.

  _Reach for this to detect whether a place you tracked has slipped since you last checked._

  ```bash
  tripadvisor-pp-cli drift 93450 --threshold 0.2 --agent
  ```

### Agent-native plumbing
- **`digest`** — One location ID to a single agent-friendly payload combining details, top reviews (user-generated), and photo URLs.

  _Reach for this when you need the full picture of one place without three round trips._

  ```bash
  tripadvisor-pp-cli digest 93450 --reviews 3 --agent
  ```

## Command Reference

**find** — Search Tripadvisor for hotels, restaurants, attractions, or geos by name

- `tripadvisor-pp-cli find <searchQuery>` — Search locations by name. Returns location_id + name + address you can pass to show/reviews/photos.

**near** — Find Tripadvisor locations near a lat/long point

- `tripadvisor-pp-cli near <latLong>` — Find locations near 'lat,long'. Returns location_id + name + address.

**photos** — List photo URLs for a location (thumbnail/medium/large/original sizes)

- `tripadvisor-pp-cli photos <locationId>` — Photos for a location_id with thumbnail/medium/large/original URLs. Content API caps at 5.

**reviews** — List recent traveler reviews for a location (user-generated content; Content API returns up to 5)

- `tripadvisor-pp-cli reviews <locationId>` — Recent reviews for a location_id.

**show** — Show full details for a location: rating, review count, ranking, address, hours, price level, awards

- `tripadvisor-pp-cli show <locationId>` — Full details for one location_id. Leads with rating, num_reviews, and ranking so agents can compare options.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tripadvisor-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Best-rated restaurants in a city

```bash
tripadvisor-pp-cli best "Lisbon" --category restaurants --top 5 --sort rating --agent
```

Searches, auto-fetches details for the top hits, and returns them ranked by rating with review counts.

### Decide between two hotels

```bash
tripadvisor-pp-cli compare 93450 258705 --agent --select name,rating,num_reviews,ranking
```

Pulls both detail records and narrows the comparison to the fields that drive the choice.

### Best near a coordinate

```bash
tripadvisor-pp-cli nearby-best "48.8606,2.3376" --category attractions --min-rating 4.5 --top 5 --agent
```

Finds nearby attractions, ranks the highly-rated ones, all from a lat/long.

### Has this place slipped?

```bash
tripadvisor-pp-cli drift 93450 --threshold 0.2 --agent
```

Compares the cached rating snapshot against a fresh fetch and flags a meaningful drop.

### One-call full picture

```bash
tripadvisor-pp-cli digest 93450 --reviews 3 --agent --select name,rating,reviews
```

Combines details, recent reviews, and photos in a single payload narrowed to what you need.

## Auth Setup

Uses an official Tripadvisor Content API key passed as the `key` query parameter (set TRIPADVISOR_API_KEY). Create a key at tripadvisor.com/developers; you must add an IP restriction (your public IPv4 as a /32) or a domain restriction before the key is shown. Domain-restricted keys also require a Referer header on every request.

Run `tripadvisor-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tripadvisor-pp-cli find mock-value --agent --select id,name,status
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
tripadvisor-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tripadvisor-pp-cli feedback --stdin < notes.txt
tripadvisor-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/tripadvisor-pp-cli/feedback.jsonl`. They are never POSTed unless `TRIPADVISOR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRIPADVISOR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tripadvisor-pp-cli profile save briefing --json
tripadvisor-pp-cli --profile briefing find mock-value
tripadvisor-pp-cli profile list --json
tripadvisor-pp-cli profile show briefing
tripadvisor-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tripadvisor-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/tripadvisor/cmd/tripadvisor-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tripadvisor-pp-mcp -- tripadvisor-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tripadvisor-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tripadvisor-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tripadvisor-pp-cli <command> --help`.
