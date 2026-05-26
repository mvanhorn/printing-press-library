---
name: pp-setlist-fm
description: "Every Setlist.fm endpoint, plus offline analytics no API call can return — tour shape, song frequency, what's overdue, setlist prediction. Trigger phrases: `predict the setlist`, `what songs are overdue for`, `look up setlist for`, `how often does X play Y`, `compare these two tours`, `use setlist-fm`, `run setlist-fm`."
author: "Dave Morin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - setlist-fm-pp-cli
    install:
      - kind: go
        bins: [setlist-fm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/cmd/setlist-fm-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/setlist-fm/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Setlist.fm — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `setlist-fm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install setlist-fm --cli-only
   ```
2. Verify: `setlist-fm-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/cmd/setlist-fm-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI whenever an agent or user needs to reason across many of an artist's setlists at once — predicting a show, finding an overdue song, counting how often a cover is played, comparing two tours, or building a dashboard of a user's attended history. Use the raw `search`/`get` commands for one-shot lookups; use the transcendence commands (predict, overdue, tour shape, compare, attended stats) when you need an aggregate the API cannot return in one call. The 2-RPS rate limit makes this CLI strictly faster than any live-API wrapper for repeated questions.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Predictive analytics
- **`predict`** — Generate a likely setlist for an upcoming show using recency-weighted per-song probability from the artist's recent tour.

  _When an agent or fan asks 'what will they play tonight', this answers it with confidence-ranked output instead of forcing a manual scrape of the last ten setlists._

  ```bash
  setlist-fm-pp-cli predict 'Radiohead' --last 10 --songs 22 --agent
  ```
- **`song-stats`** — For one song: total plays, first/last date, longest gap, average set position, percentage of shows that included it.

  _Answers the canonical fan question ('when was the last time they played X?') without burning 50 API calls._

  ```bash
  setlist-fm-pp-cli song-stats 'Radiohead' 'Paranoid Android' --agent
  ```
- **`overdue`** — Rank an artist's songs by shows-since-last-played to surface what is most due to return.

  _Lets an agent predict the 'wildcard' slot of a setlist without re-fetching the artist's whole tour._

  ```bash
  setlist-fm-pp-cli overdue 'Radiohead' --top 10 --agent
  ```
- **`song-gap`** — Biggest dry spells for one song and when the comeback happened.

  _Lets agents narrate band history without re-fetching the full setlist history._

  ```bash
  setlist-fm-pp-cli song-gap 'Radiohead' 'Creep' --agent
  ```

### Tour analytics
- **`tour-shape`** — Median set length, encore frequency, song-position histogram, top openers and closers for one tour.

  _Gives an agent a one-shot summary of how an artist is structuring a tour, replacing dozens of read calls._

  ```bash
  setlist-fm-pp-cli tour-shape 'Radiohead' --tour 'A Moon Shaped Pool Tour' --agent
  ```
- **`compare`** — Overlap percent, dropped songs, debuts, and set-position shifts between two named tours of one artist.

  _Surfaces tour evolution in one call for review/journalism workflows._

  ```bash
  setlist-fm-pp-cli compare 'Phoenix' --tour 'Ti Amo Tour' --tour 'Alpha Zulu Tour' --agent
  ```
- **`encore`** — Top encore openers, top encore closers, percent of shows that had an encore at all.

  _Lets agents answer 'what do they always close with?' offline._

  ```bash
  setlist-fm-pp-cli encore 'Radiohead' --agent
  ```
- **`venue-loyalty`** — Top venues an artist plays at, by frequency. Detects 'home venue' patterns.

  _Lets agents reason about artist-venue affinity without dozens of API calls._

  ```bash
  setlist-fm-pp-cli venue-loyalty 'Phish' --agent
  ```

### Discovery
- **`covers`** — All cover songs an artist has played live, ranked by frequency with the original artist.

  _Surfaces fan-discovery moments (rare covers, recurring covers) for journalism or curation._

  ```bash
  setlist-fm-pp-cli covers 'Phoebe Bridgers' --top 20 --agent
  ```
- **`setlist-diff`** — Side-by-side song diff of two setlist IDs.

  _Lets agents answer 'what changed between these two shows' without parsing two responses by hand._

  ```bash
  setlist-fm-pp-cli setlist-diff 53e3ab04 7be1aaa0 --agent
  ```
- **`debut`** — Songs an artist has played exactly once live.

  _Surfaces one-off oddities (one-time covers, abandoned originals) for rare-finding workflows._

  ```bash
  setlist-fm-pp-cli debut 'Phoenix' --agent
  ```

### Collector tools
- **`attended-stats`** — Total shows, unique artists/songs/venues/cities, biggest streak, longest gap, decade breakdown for a user.

  _Delivers a one-shot collector dashboard that the website does not produce._

  ```bash
  setlist-fm-pp-cli attended-stats dave42 --agent
  ```
- **`bingo`** — Printable bingo card of N most-likely-to-play songs for an upcoming show.

  _Fan-festival use case; gives a delightful tangible output the API was never going to ship._

  ```bash
  setlist-fm-pp-cli bingo 'Radiohead' --songs 25
  ```
- **`since`** — Setlist updates since a given timestamp; pair with sync for delta refresh.

  _Lets a daily-digest agent stay current without re-syncing the full history._

  ```bash
  setlist-fm-pp-cli since 2026-05-01T00:00:00Z --artist 'Radiohead' --agent
  ```
- **`playlist`** — Generate a Spotify playlist from an artist's most recent setlist (or merged last N setlists) — output as CSV, M3U, or Spotify-search URIs, or use the Spotify Web API to create the playlist directly.

  _Turns a concert log into something an agent or user can listen to in one command — the bridge between data and ear that no wrapper builds._

  ```bash
  setlist-fm-pp-cli playlist 'Radiohead' --last 1 --output csv > radiohead-last-show.csv
  ```

## Command Reference

### Artists & Setlists

- `setlist-fm-pp-cli artist resolve <name>` — Resolve an artist name to a MusicBrainz MBID
- `setlist-fm-pp-cli artist get <mbid>` — Get artist details by MBID
- `setlist-fm-pp-cli artist setlists <mbid>` — List setlists for an artist
- `setlist-fm-pp-cli setlist get <id>` — Get a specific setlist by ID
- `setlist-fm-pp-cli setlist version <versionId>` — Get a setlist version by ID
- `setlist-fm-pp-cli setlist-diff <idA> <idB>` — Side-by-side diff of two setlists

### Search

- `setlist-fm-pp-cli search artists --name <name>` — Search for artists
- `setlist-fm-pp-cli search venues --name <name>` — Search for venues
- `setlist-fm-pp-cli search cities --name <name>` — Search for cities
- `setlist-fm-pp-cli search countries` — List all supported countries
- `setlist-fm-pp-cli search setlists --artist <name>` — Search for setlists

### Venues, Cities & Users

- `setlist-fm-pp-cli venue get <id>` — Get venue details
- `setlist-fm-pp-cli venue setlists <id>` — List setlists at a venue
- `setlist-fm-pp-cli city get <geoId>` — Get city details by GeoNames ID
- `setlist-fm-pp-cli user get <userId>` — Get user details
- `setlist-fm-pp-cli user attended <userId>` — List setlists a user has attended
- `setlist-fm-pp-cli user edited <userId>` — List setlists a user has edited

### Analytics (offline, from local store)

- `setlist-fm-pp-cli predict <artist>` — Predicted setlist using recency-weighted probability
- `setlist-fm-pp-cli song-stats <artist> <song>` — Total plays, first/last date, gap, position, frequency
- `setlist-fm-pp-cli overdue <artist>` — Songs ranked by shows since last played
- `setlist-fm-pp-cli song-gap <artist> <song>` — Biggest gaps between plays of one song
- `setlist-fm-pp-cli tour-shape <artist>` — Set lengths, encores, openers, closers for one tour
- `setlist-fm-pp-cli compare <artist>` — Compare two tours: overlap, dropped, added, shifts
- `setlist-fm-pp-cli encore <artist>` — Top encore songs and encore frequency
- `setlist-fm-pp-cli covers <artist>` — Cover songs played live, ranked by frequency
- `setlist-fm-pp-cli debut <artist>` — Songs played exactly once live
- `setlist-fm-pp-cli venue-loyalty <artist>` — Top venues by frequency, home venue detection
- `setlist-fm-pp-cli tour-route <artist>` — Chronological route of a tour
- `setlist-fm-pp-cli bingo <artist>` — Bingo card of most-likely songs
- `setlist-fm-pp-cli playlist <artist>` — Export setlist as CSV, M3U, or Spotify search URIs
- `setlist-fm-pp-cli since <timestamp>` — Setlists updated since a given timestamp
- `setlist-fm-pp-cli attended-stats <userId>` — Collector dashboard: shows, artists, songs, venues, streaks

### Sync & Utilities

- `setlist-fm-pp-cli sync artist <name-or-mbid>` — Sync all setlists for an artist into local store
- `setlist-fm-pp-cli sync user <userId>` — Sync a user's attended setlists into local store
- `setlist-fm-pp-cli workflow archive` — Sync all resources to local store
- `setlist-fm-pp-cli workflow status` — Show local archive status and sync state
- `setlist-fm-pp-cli doctor` — Check CLI health: config, auth, connectivity
- `setlist-fm-pp-cli auth set-token <key>` — Save an API token to the config file
- `setlist-fm-pp-cli which <capability>` — Find the command that implements a capability


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
setlist-fm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Predict tonight's setlist

```bash
setlist-fm-pp-cli sync artist 'Radiohead' --max-pages 5 && setlist-fm-pp-cli predict 'Radiohead' --last 10 --songs 22 --agent --select songs.name,songs.probability,songs.last_played
```

Sync the recent tour into the store, then output a ranked setlist prediction with only the three fields an agent needs — drops a 30 KB response to ~2 KB.

### What's overdue for a comeback?

```bash
setlist-fm-pp-cli overdue 'Phish' --top 10 --agent
```

Lists the ten songs Phish has gone the longest without playing, ordered by show-count gap.

### Compare two tours

```bash
setlist-fm-pp-cli compare 'Phoenix' --tour 'Ti Amo Tour' --tour 'Alpha Zulu Tour' --agent
```

Shows song overlap, dropped songs, debuts, and set-position shifts between two named tours.

### Build a fan bingo card

```bash
setlist-fm-pp-cli bingo 'The National' --songs 25
```

Generates a printable 5×5 card of the most-likely-to-play songs for the next show.

### My concert dashboard

```bash
setlist-fm-pp-cli attended-stats myUsername --agent
```

Returns total shows, unique artists/songs/venues, biggest streak, longest gap — the dashboard the website doesn't render.

## Auth Setup

Get a free API key at https://www.setlist.fm/settings/api. The CLI looks for SETLISTFM_API_KEY in the environment and falls back to SETLIST_FM_API_KEY for compatibility with existing Python and JavaScript tooling. Run `setlist-fm-pp-cli auth set-token <key>` to persist it in the config file. All requests automatically throttle to 2 RPS and surface 429 responses with a backoff hint.

Run `setlist-fm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  setlist-fm-pp-cli artist get a74b1b7f-71a5-4011-9441-d0b5e4122711 --agent --select mbid,name
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
setlist-fm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
setlist-fm-pp-cli feedback --stdin < notes.txt
setlist-fm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.setlist-fm-pp-cli/feedback.jsonl`. They are never POSTed unless `SETLIST_FM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SETLIST_FM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
setlist-fm-pp-cli profile save briefing --json
setlist-fm-pp-cli --profile briefing artist get a74b1b7f-71a5-4011-9441-d0b5e4122711
setlist-fm-pp-cli profile list --json
setlist-fm-pp-cli profile show briefing
setlist-fm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `setlist-fm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/setlist-fm/cmd/setlist-fm-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add setlist-fm-pp-mcp -- setlist-fm-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which setlist-fm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   setlist-fm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `setlist-fm-pp-cli <command> --help`.
