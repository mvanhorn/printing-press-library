---
name: pp-hotelist
description: "Every Hotelist feature, plus a local SQLite mirror that ranks rating-per-dollar across whole countries Trigger phrases: `find AI-rated hotels in`, `best value hotel in`, `hotels with a real gym in`, `compare hotel chains`, `rank hotels by rating per dollar`, `use hotelist`, `run hotelist`."
author: "David Bryson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hotelist-pp-cli
    install:
      - kind: go
        bins: [hotelist-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/hotelist/cmd/hotelist-pp-cli
---

# Hotelist — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hotelist-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install hotelist --cli-only
   ```
2. Verify: `hotelist-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotelist/cmd/hotelist-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Hotelist.com (by @levelsio) rates hotels with AI: it reads real traveler reviews, scores actual room photos, and photo-verifies claimed amenities so you can filter on a gym with real weights or a bathtub that actually exists. This CLI puts that data in your terminal and in agent context with --json/--select output, an offline cache, and cross-location commands the single-map website can't express: rank-country, chain-compare, corridor, and a watch/diff drift tracker. Data is scraped from Hotelist (community/AI-rated, not an official API).

## When to Use This CLI

Use this CLI when an agent or user wants AI-rated, photo-verified hotel discovery without pay-to-play bias, especially for cross-location questions: best value across a country, chain-vs-chain comparison, a multi-city route, or tracking how a place changes over time. It shines when the answer requires aggregating or joining hotel data that Hotelist's one-city-at-a-time map cannot.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to book a hotel or check live room availability — Hotelist is discovery-only with no booking or dated pricing.
- Do not use it for date-specific nightly quotes; prices are AI-estimated averages.
- Do not use it as a general Booking.com/Expedia client — it only exposes Hotelist's AI-rated dataset.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-location value the map UI can't express
- **`rank-country`** — Rank the best hotels across an entire country by Hotelist rating-per-dollar, with compound amenity and price filters.

  _When an agent needs the single best-value hotel in a whole country under hard constraints, reach for this instead of paging city-by-city search._

  ```bash
  hotelist-pp-cli rank-country thailand --min-rating 8 --max-price 150 --amenities pool,coworking --json
  ```
- **`chain-compare`** — Compare hotel chains head-to-head on mean rating, median price, and rating-per-dollar in a country.

  _Answers 'which chain is actually worth it here?' in one call instead of a dozen manual map filters._

  ```bash
  hotelist-pp-cli chain-compare --chains marriott,hilton,hyatt --country japan --metric best-value --json
  ```
- **`corridor`** — Find the best hotel in each stop of a multi-city route in one pass, with shared filters.

  _Plans a nomad's annual route in one command; pick this over N separate searches when the user names several cities._

  ```bash
  hotelist-pp-cli corridor --cities "Chiang Mai,Lisbon,Medellin" --min-rating 7.5 --max-price 120 --amenities coworking --json
  ```

### Local state that compounds
- **`watch`** — Snapshot a saved location over time and diff which hotels improved, declined, or changed price since you last checked.

  _The only way to answer 'did this city's hotels get better or more expensive since last time?' — pick it for any change-over-time question._

  ```bash
  hotelist-pp-cli watch diff lisbon --since 2026-01-01 --metric both --json
  ```
- **`chain-consistency`** — Compute mean, median, and spread of a single chain's ratings across a country to see if the brand is reliably good or full of outliers.

  _Use before trusting a loyalty brand in an unfamiliar region; surfaces hidden outlier risk a single listing can't show._

  ```bash
  hotelist-pp-cli chain-consistency --chain marriott --country thailand --json
  ```
- **`price-cliff`** — Find the price point in a city where rating-per-extra-dollar collapses — the cheapest hotel that's still legitimately good.

  _Turns 'spend the least without sacrificing quality' into one number plus the hotels just below the cliff._

  ```bash
  hotelist-pp-cli price-cliff bangkok --min-rating 7 --json
  ```

## Command Reference

**hotel** — A single AI-rated hotel from Hotelist.

- `hotelist-pp-cli hotel <hotel_id>` — Fetch the raw detail-modal HTML for one hotel (Hotelist Score, verified amenities, AI rating breakdown).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hotelist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Best-value hotel with a real gym in a city

```bash
hotelist-pp-cli filter lisbon --gym-weights --json --select hotels.name,hotels.hotellist_rating,hotels.price
```

Photo-verified weightlifting gym, narrowed to just name/rating/price so an agent doesn't parse the full payload.

### National value leaderboard

```bash
hotelist-pp-cli rank-country thailand --min-rating 8 --max-price 150 --top 10 --json
```

Top 10 hotels by rating-per-dollar across the whole country in one call.

### Compare loyalty chains before committing

```bash
hotelist-pp-cli chain-compare --chains marriott,hyatt --country japan --metric best-value --json
```

Head-to-head mean rating, median price, and value per chain.

### Plan a nomad route

```bash
hotelist-pp-cli corridor --cities "Chiang Mai,Lisbon,Tbilisi" --min-rating 7.5 --max-price 120 --json
```

Best hotel per stop that clears all filters, in one pass.

### Deep-dive one hotel's AI breakdown

```bash
hotelist-pp-cli show KYLCGAVE --json
```

Full Hotelist Score, AI photo and review ratings, consensus, verified amenities, pros and cons.

## Auth Setup

No authentication required.

Run `hotelist-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hotelist-pp-cli hotel mock-value --agent --select id,name,status
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
hotelist-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hotelist-pp-cli feedback --stdin < notes.txt
hotelist-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/hotelist-pp-cli/feedback.jsonl`. They are never POSTed unless `HOTELIST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOTELIST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hotelist-pp-cli profile save briefing --json
hotelist-pp-cli --profile briefing hotel mock-value
hotelist-pp-cli profile list --json
hotelist-pp-cli profile show briefing
hotelist-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hotelist-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/hotelist/cmd/hotelist-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hotelist-pp-mcp -- hotelist-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hotelist-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hotelist-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hotelist-pp-cli <command> --help`.
