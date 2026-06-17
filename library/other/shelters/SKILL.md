---
name: pp-shelters
description: "Printing Press CLI for Shelters: real-time open-shelter information for the United States and its territories (FEMA National Shelter System unioned with the American Red Cross feed), for AI agents and people"
author: "Abe Diaz (@abe238)"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - shelters-pp-cli
    install:
      - kind: go
        bins: [shelters-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/shelters/cmd/shelters-pp-cli
---

# Shelters — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `shelters-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install shelters --cli-only
   ```
2. Verify: `shelters-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/shelters/cmd/shelters-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Gives agents and people the most comprehensive, credible open-shelter picture available: it unions FEMA's National Shelter System (NSS) OpenShelters feed with the American Red Cross Emergency-Action feed (the redcross.org map's source) and dedupes them, because FEMA is synchronized downstream of Red Cross and lags it by up to a day, so neither feed alone is complete. Coverage is the United States and its territories only (both FEMA NSS and the American Red Cross are US feeds), not other countries. FEMA rows are best-effort enriched with the richer FEMA_NSS layer (county/parish, the driving incident, the open date, generator and floodplain/surge attributes), and the CLI folds in live occupancy (current population, the general/medical/other/pet breakdown, capacity, and the driving incident) from the American Red Cross Open_Shelters layer, the one feed that reports a real headcount. Every shelter carries a source field recording which feeds it came from ('fema', 'redcross', or 'fema+redcross', with '+occupancy' appended once live population is merged in). It shows only publicly listed shelters: the Open_Shelters roster is used to fill population onto shelters already in the public feeds, never to add one, and any shelter the Red Cross keeps off its public map (hide_from_public) is dropped even when FEMA's feed lists it, so the tool stays conservative about never directing people to a site the Red Cross has not cleared for the public. It answers the questions people actually ask in a disaster, like 'the closest open shelter to me that allows pets' and 'which shelters are at capacity', filters by state, pets, accessibility, county, or generator, geocodes addresses (and bare ZIPs) when coordinates are missing, and never invents a number it does not have (a missed secondary fetch degrades to explicit null with a note, never a wrong value). Deep thanks to all first responders, emergency management practitioners, and relief nonprofit organizations for the work you do in communities when disaster strikes. This is an unofficial tool; in a life-threatening emergency call 911 and follow the official guidance and evacuation orders from FEMA, the American Red Cross, your local emergency management, and your local authorities.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Closest shelter
- **`near`** — Ranks open shelters by straight-line distance from a lat,lon, ZIP, or street address; geocodes shelters that are missing coordinates and reports any it cannot locate rather than dropping them; filterable by pets, ADA, wheelchair, county/parish, and confirmed onsite generator.

  _Answers 'the closest shelter to me that allows pets' in one call; add --pets, --ada, or --wheelchair._

  ```bash
  shelters-pp-cli near 78566 --pets --limit 3
  ```

### Capacity
- **`capacity`** — Computes utilization only where both a population and a capacity are reported, labeling the denominator (evacuation vs post-impact), and surfaces shelters reported FULL without asserting it as ground truth.

  _Answers 'which shelter is at capacity?' honestly, marking unknowns as unknown._

  ```bash
  shelters-pp-cli capacity --state TX
  ```

### One-call situational awareness
- **`brief`** — One command returns the open count, breakdowns by state and by the driving incident, pet-friendly and accessible counts, and the capacity picture, with an optional human briefing.

  _Reach for this first when an agent asks 'what is the shelter situation right now'._

  ```bash
  shelters-pp-cli brief --markdown
  ```

### Listings and detail
- **`shelters`** — Open shelters from the union of FEMA OpenShelters and the American Red Cross feed (deduped, each tagged with a source field: fema, redcross, or fema+redcross), flattened and filterable by state, pets, ADA, wheelchair, managing org, status, county/parish, and confirmed onsite generator; FEMA rows best-effort enriched with the richer FEMA_NSS/0 layer (county, the driving incident, generator and floodplain/surge attributes).

  _Use to narrow open shelters to the ones that match a person's needs._

  ```bash
  shelters-pp-cli shelters --state FL --ada --json
  ```
- **`shelter`** — Full detail for one shelter joined on the stable shelter_id rather than the churning objectid, with unreported fields as explicit null; enriched with FEMA_NSS/0 fields (county, the driving incident, open date, generator, floodplain/surge, and the population breakdown) when reported.

  _Use when you have a shelter_id and need its full record._

  ```bash
  shelters-pp-cli shelter 368133
  ```

## Recipes

### Closest pet-friendly shelter as compact JSON for an agent

```bash
shelters-pp-cli near 29.76,-95.37 --pets --limit 1 --json --select data.shelters
```

Returns just the nearest pet-friendly shelter so an agent does not burn context on the full feed.

### Shelters confirmed wheelchair accessible in one state

```bash
shelters-pp-cli shelters --state TX --wheelchair --json
```

Filters open shelters to those confirmed wheelchair accessible.

### Which shelters are at or over capacity

```bash
shelters-pp-cli capacity --json --select data.shelters
```

Reports utilization where computable and flags reported-full shelters, honestly marking unknowns.

### One-call situational briefing

```bash
shelters-pp-cli brief --markdown
```

Bundles open count, by-state, by-incident, accessibility, and capacity into a single human-readable briefing.

### Open shelters in one county that have a generator

```bash
shelters-pp-cli shelters --county Cameron --generator --json
```

Filters on the FEMA_NSS/0 enrichment fields (county/parish and confirmed onsite generator); matches nothing honestly when the enrichment is unavailable rather than guessing.

### Shelters the Red Cross feed has but FEMA does not yet

```bash
shelters-pp-cli shelters --json --select data.shelters
```

The union surfaces shelters tagged source 'redcross' (open on the redcross.org map but not yet in FEMA, which syncs downstream and lags); a consumer can filter on the source field.

### FEMA spine only, no Red Cross union or enrichment

```bash
shelters-pp-cli shelters --no-enrich --json
```

Skips both the Red Cross union and the FEMA_NSS/0 enrichment round-trips for the fastest, FEMA-OpenShelters-only answer.

## Command Reference

All commands are read-only and need no API key. They fetch the live union (deduped) of FEMA's OpenShelters feed and the American Red Cross open-shelter feed, then fill in live occupancy (current population and capacity) from the American Red Cross Open_Shelters layer (each shelter tagged `source` fema, redcross, or fema+redcross, with `+occupancy` appended where live population was merged in), or a `--fixture` file for offline use, and emit a `{source, fetched_at, data}` JSON envelope. Occupancy is a fill-only overlay on publicly listed shelters; a site the Red Cross keeps off the public map is never added, and any shelter the Red Cross hides from its public map (hide_from_public) is dropped even when FEMA lists it. Coverage is the United States and its territories only, not other countries.

- `shelters-pp-cli shelters` (alias `list`) — Open shelters, filterable by `--state`, `--pets`, `--ada`, `--wheelchair`, `--org`, `--status`, `--limit`.
- `shelters-pp-cli shelter <shelter_id>` — Full detail for one shelter, joined on the stable `shelter_id`.
- `shelters-pp-cli near <location>` — Closest open shelters to a `lat,lon`, ZIP, or address; geocodes shelters missing coordinates. Add `--pets` / `--ada` / `--wheelchair`, `--limit`, `--max-miles`. **Use for "the closest shelter to me that allows pets."**
- `shelters-pp-cli capacity` — Which shelters are at or near capacity, computed only where population and a capacity both exist (denominator labeled), plus any reported FULL. **Use for "which shelter is at capacity?"**
- `shelters-pp-cli brief [--markdown]` — One-call situational briefing: open count, by-state, pet-friendly + accessible counts, capacity picture.
- `shelters-pp-cli gis-links` — Stable FEMA service URLs and the full-NSS access path (link-out only).
- `shelters-pp-cli credits` — Gratitude to first responders / emergency managers / relief nonprofits, plus the unofficial-tool + safety disclaimer.

Quotas on honesty: coordinates are frequently null (geocoded by `near`, with anything unlocatable reported in a count), and capacity is never computed against a missing denominator.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shelters-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `shelters-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted paths descending into the envelope (the data is nested under `data`). Critical for keeping context small:

  ```bash
  shelters-pp-cli near 78566 --pets --agent --select data.shelters
  ```
- **Previewable** — `--dry-run` short-circuits without sending the request
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

The shelter commands wrap output in a small provenance envelope:

```json
{
  "source": "<live feed URL | fixture:path>",
  "fetched_at": "<ISO-8601 UTC, stamped client-side because the feed has none>",
  "data": { ... }
}
```

Parse `.data` for results and `.source` to know whether it was live or a fixture. A human listing is printed only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
shelters-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shelters-pp-cli feedback --stdin < notes.txt
shelters-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/shelters-pp-cli/feedback.jsonl`. They are never POSTed unless `SHELTERS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHELTERS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shelters-pp-cli profile save briefing --json
shelters-pp-cli --profile briefing shelters
shelters-pp-cli profile list --json
shelters-pp-cli profile show briefing
shelters-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shelters-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/shelters/cmd/shelters-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add shelters-pp-mcp -- shelters-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shelters-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shelters-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shelters-pp-cli <command> --help`.
