---
name: pp-ticketmaster
description: "Every Discovery v2 endpoint plus offline search, multi-venue watchlists, residency dedup, and on-sale tracking no... Trigger phrases: `what concerts in <city> this weekend`, `what's playing at <venue>`, `where is <artist> playing`, `presale watch`, `ticketmaster events`, `use ticketmaster`, `run ticketmaster`."
author: "Omar Shahine"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ticketmaster-pp-cli
---

# Ticketmaster — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ticketmaster-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ticketmaster --cli-only
   ```
2. Verify: `ticketmaster-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI is the first single-binary tool for the Ticketmaster Discovery API. It absorbs the full read-only surface (events, venues, attractions, classifications, suggest) and adds a local SQLite store with FTS search, named watchlists, residency collapse, tour-view with on-sale flags, and markdown briefs — the workflows real users built scripts to handle.

## When to Use This CLI

Reach for this CLI when a user asks 'what's on at <venue/metro>', 'where is <artist> playing', or 'what concerts this weekend'. Best for repeat queries against curated venue/artist watchlists (offline FTS shines here), residency-heavy venues (opera, Broadway, comedy), and agent contexts where compact JSON output keeps token usage low. Skip when checkout/purchase is needed — that requires the Commerce API, which this CLI does not cover.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local-store aggregations
- **`events upcoming`** — Fan out across a venue ID file or list and return one merged, deduplicated, date-sorted event list — the watchlist primitive behind any curated 'what's on at my venues' workflow.

  _When the user has a curated list of venues they care about and wants one merged feed; replaces hand-rolled per-venue fan-out scripts._

  ```bash
  ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json
  ```
- **`events residency`** — Collapse runs of same-name + same-venue events into one row per residency with first_date, last_date, night_count, and id_list — so a 16-night opera season shows as one entry, not 16.

  _When listing upcoming events would otherwise show many near-duplicate rows for Broadway tours, opera seasons, or comedy residencies._

  ```bash
  ticketmaster-pp-cli events residency --window 28 --json
  ```
- **`events by-classification`** — Local join of events × classifications, grouped by segment and genre, with event count and three example events per leaf — the bucketed view newsletter authors and local-scene trackers reach for.

  _When summarizing 'what's on this month' broken down by music vs theatre vs comedy vs sports._

  ```bash
  ticketmaster-pp-cli events by-classification --dma 383 --window 60 --json
  ```
- **`events watchlist`** — Save, list, run, and remove named filter sets (venue IDs, attraction IDs, segments, DMA IDs) that persist across runs in the local SQLite store — the generic primitive any curated 'my venues' workflow composes from.

  _When the same curated venue/artist/genre filter recurs across many queries._

  ```bash
  ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA
  ```
- **`events price-bands`** — Bucket events by priceRanges.min into <$50 / $50-100 / $100-200 / $200+ bands and report count + sample events per band, grouped by classification.

  _When the user wants to know where the affordable shows are this month, or how a venue's pricing skews._

  ```bash
  ticketmaster-pp-cli events price-bands --dma 383 --window 30 --json
  ```

### Tour & on-sale tracking
- **`events tour`** — For a given attraction (artist/team/touring show), return every upcoming event sorted by date, with city, venue, on-sale status, and a flag for events going on-sale within 7 days.

  _When tracking an artist across cities or watching for presale windows._

  ```bash
  ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json
  ```
- **`events on-sale-soon`** — Local query for events whose public on-sale falls in the next N days, sorted ascending — the canonical 'presale watch' view that no API endpoint provides.

  _When the user wants to be alerted to upcoming on-sale dates without polling each artist manually._

  ```bash
  ticketmaster-pp-cli events on-sale-soon --window 7 --classification rock --json
  ```

### Agent-native plumbing
- **`events dedup`** — Read an event JSON array from stdin or the local store, apply a deduplication strategy (name+venue+date, or tour-leg), and write the deduped stream to stdout — composes with any upstream command.

  _When merging results from multiple queries or sources and the duplicates need to be removed before agent processing._

  ```bash
  ticketmaster-pp-cli events list --keyword phish --json | ticketmaster-pp-cli events dedup --strategy tour-leg
  ```
- **`events brief`** — Render a markdown 'what's on' report grouped by night → venue → events with classification labels and price bands, suitable for newsletter, Obsidian, iMessage, or agent context.

  _When the user needs a paste-ready event summary for a chat thread, newsletter, or LLM context._

  ```bash
  ticketmaster-pp-cli events brief --dma 383 --window 7
  ```

## Command Reference

**attractions** — Manage attractions

- `ticketmaster-pp-cli attractions find` — Find attractions (artists, sports, packages, plays and so on) and filter your search by name, and much more.
- `ticketmaster-pp-cli attractions get` — Get details for a specific attraction using the unique identifier for the attraction.

**classifications** — Manage classifications

- `ticketmaster-pp-cli classifications get` — Get details for a specific segment, genre, or sub-genre using its unique identifier.
- `ticketmaster-pp-cli classifications get-genre` — Get details for a specific genre using its unique identifier.
- `ticketmaster-pp-cli classifications get-segment` — Get details for a specific segment using its unique identifier.
- `ticketmaster-pp-cli classifications get-subgenre` — Get details for a specific sub-genre using its unique identifier.
- `ticketmaster-pp-cli classifications list` — Find classifications and filter your search by name, and much more. Classifications help define the nature of...

**events** — Manage events

- `ticketmaster-pp-cli events get` — Get details for a specific event using the unique identifier for the event. This includes the venue and location,...
- `ticketmaster-pp-cli events list` — Find events and filter your search by location, date, availability, and much more.

**suggest** — Manage suggest

- `ticketmaster-pp-cli suggest` — Find search suggestions and filter your suggestions by location, source, etc.

**venues** — Manage venues

- `ticketmaster-pp-cli venues get` — Get details for a specific venue using the unique identifier for the venue.
- `ticketmaster-pp-cli venues list` — Find venues and filter your search by name, and much more.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ticketmaster-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Seattle watchlist (composes the generic primitives, no Seattle-specific code)

```bash
ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA,KovZpZAFEdeA,KovZpZAFkv1A
ticketmaster-pp-cli events watchlist run seattle --days 60 --json | ticketmaster-pp-cli events dedup --strategy name-venue-date --json
```

Save a named watchlist of Seattle venue IDs (Climate Pledge Arena, Paramount, Moore, etc.), run it across the next 60 days, and dedup any cross-venue duplicates — replaces a 595-line bash script that hand-rolled the same loop.

### Track an artist across all upcoming dates with on-sale flags

```bash
ticketmaster-pp-cli events tour 'Florence + The Machine' --on-sale-window 7 --json --select 'name,dates.start.localDate,_embedded.venues[0].name,_embedded.venues[0].city.name,sales.public.startDateTime'
```

Returns every upcoming tour stop with city, venue, and a flag for stops going on-sale within a week; the --select narrows the deeply-nested Discovery payload to just what an agent needs.

### Weekend brief for a metro

```bash
ticketmaster-pp-cli events brief --dma 383 --window 3 --classification music
```

Render a markdown brief of the next 3 days of music events in Seattle-Tacoma — paste-ready for Obsidian or an iMessage thread.

### On-sale watch for rock shows

```bash
ticketmaster-pp-cli events on-sale-soon --window 14 --classification rock --json
```

Two-week-out scan for rock events going on public sale; pipe to your alerting script.

## Auth Setup

Authentication is a single Ticketmaster Discovery API consumer key, passed as the `apikey` query parameter on every request. Register at https://developer-acct.ticketmaster.com and copy the Consumer Key from your My Apps dashboard. Set TICKETMASTER_API_KEY in your shell environment. The free tier allows 5000 requests/day at 5/second.

Run `ticketmaster-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ticketmaster-pp-cli attractions get mock-value --agent --select id,name,status
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
ticketmaster-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ticketmaster-pp-cli feedback --stdin < notes.txt
ticketmaster-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ticketmaster-pp-cli/feedback.jsonl`. They are never POSTed unless `TICKETMASTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TICKETMASTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ticketmaster-pp-cli profile save briefing --json
ticketmaster-pp-cli --profile briefing attractions get mock-value
ticketmaster-pp-cli profile list --json
ticketmaster-pp-cli profile show briefing
ticketmaster-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ticketmaster-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ticketmaster-pp-mcp -- ticketmaster-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ticketmaster-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ticketmaster-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ticketmaster-pp-cli <command> --help`.
