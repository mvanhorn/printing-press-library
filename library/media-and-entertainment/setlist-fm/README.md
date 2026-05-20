# Setlist.fm CLI

**Every Setlist.fm endpoint, plus offline analytics no API call can return — tour shape, song frequency, what's overdue, setlist prediction.**

Setlist.fm rate-limits to 2 requests per second and 1,440 per day, which makes any real analytics workflow impossible against the live API. This CLI syncs an artist's full setlist history to a local SQLite store once, then runs every transcendence query — predict, overdue, tour shape, song gap, covers, attended stats — instantly and offline. Six SDK wrappers exist across five languages; none of them store anything. This one does.

## Install

The recommended path installs both the `setlist-fm-pp-cli` binary and the `pp-setlist-fm` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install setlist-fm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install setlist-fm --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/setlist-fm/cmd/setlist-fm-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/setlist-fm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-setlist-fm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-setlist-fm --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-setlist-fm skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-setlist-fm. The skill defines how its required CLI can be installed.
```

## Authentication

Get a free API key at https://www.setlist.fm/settings/api. The CLI looks for `SETLISTFM_API_KEY` in the environment and falls back to `SETLIST_FM_API_KEY` for compatibility with existing Python and JavaScript tooling. Run `setlist-fm-pp-cli auth set-token <key>` to persist it in the config file. All requests automatically throttle to 2 RPS and surface 429 responses with a backoff hint.

## Quick Start

```bash
# Persist your API key once; every later command reads it from the config.
setlist-fm-pp-cli auth set-token $SETLISTFM_API_KEY

# Get the MusicBrainz ID without copy-pasting from the website.
setlist-fm-pp-cli artist resolve 'Radiohead'

# Pull the last few pages of setlists into the local SQLite store under your daily-request budget.
setlist-fm-pp-cli sync artist 'Radiohead' --max-pages 5

# Run the first transcendence query: which songs are most due for a comeback.
setlist-fm-pp-cli overdue 'Radiohead' --top 10 --agent

# Generate a predicted setlist for the next show from the recent tour.
setlist-fm-pp-cli predict 'Radiohead' --last 10 --songs 22 --agent
```

## Unique Features

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

## Commands

### Artists & Setlists

| Command | Description |
| --- | --- |
| `artist resolve <name>` | Resolve an artist name to a MusicBrainz MBID |
| `artist get <mbid>` | Get artist details by MBID |
| `artist setlists <mbid>` | List setlists for an artist |
| `setlist get <id>` | Get a specific setlist by ID |
| `setlist version <versionId>` | Get a setlist version by ID |
| `setlist-diff <idA> <idB>` | Side-by-side diff of two setlists |

### Search

| Command | Description |
| --- | --- |
| `search artists --name <name>` | Search for artists |
| `search venues --name <name>` | Search for venues |
| `search cities --name <name>` | Search for cities |
| `search countries` | List all supported countries |
| `search setlists --artist <name>` | Search for setlists |

### Venues & Cities

| Command | Description |
| --- | --- |
| `venue get <id>` | Get venue details |
| `venue setlists <id>` | List setlists at a venue |
| `city get <geoId>` | Get city details by GeoNames ID |

### Users

| Command | Description |
| --- | --- |
| `user get <userId>` | Get user details |
| `user attended <userId>` | List setlists a user has attended |
| `user edited <userId>` | List setlists a user has edited |
| `attended-stats <userId>` | Collector dashboard: shows, artists, songs, venues, streaks |

### Analytics (offline, from local store)

| Command | Description |
| --- | --- |
| `predict <artist>` | Generate a predicted setlist using recency-weighted probability |
| `song-stats <artist> <song>` | Total plays, first/last date, gap, position, frequency |
| `overdue <artist>` | Rank songs by shows since last played |
| `song-gap <artist> <song>` | Biggest dry spells between plays of one song |
| `tour-shape <artist>` | Set lengths, encores, openers, closers for one tour |
| `compare <artist>` | Compare two tours: overlap, dropped, added, position shifts |
| `encore <artist>` | Top encore songs and encore frequency |
| `covers <artist>` | All cover songs played live, ranked by frequency |
| `debut <artist>` | Songs played exactly once live |
| `venue-loyalty <artist>` | Top venues by frequency, home venue detection |
| `tour-route <artist>` | Chronological route of a tour: dates, cities, venues |
| `bingo <artist>` | Bingo card of most-likely songs for an upcoming show |
| `playlist <artist>` | Export setlist as CSV, M3U, or Spotify search URIs |
| `since <timestamp>` | Setlists updated since a given timestamp |

### Sync & Utilities

| Command | Description |
| --- | --- |
| `sync artist <name-or-mbid>` | Sync all setlists for an artist into local store |
| `sync user <userId>` | Sync a user's attended setlists into local store |
| `workflow archive` | Sync all resources to local store |
| `workflow status` | Show local archive status and sync state |
| `doctor` | Check CLI health: config, auth, connectivity |
| `auth set-token <key>` | Save an API token to the config file |
| `auth status` | Show authentication status |
| `auth logout` | Clear stored credentials |
| `which <capability>` | Find the command that implements a capability |
| `import <resource>` | Import data from JSONL file |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
setlist-fm-pp-cli artist resolve 'Radiohead'

# JSON for scripting and agents
setlist-fm-pp-cli artist resolve 'Radiohead' --json

# Filter to specific fields
setlist-fm-pp-cli artist resolve 'Radiohead' --json --select mbid,name

# CSV output
setlist-fm-pp-cli overdue 'Radiohead' --csv

# Dry run — show the request without sending
setlist-fm-pp-cli artist get a74b1b7f-71a5-4011-9441-d0b5e4122711 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
setlist-fm-pp-cli predict 'Radiohead' --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-setlist-fm -g
```

Then invoke `/pp-setlist-fm <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/setlist-fm/cmd/setlist-fm-pp-mcp@latest
```

Then register it:

```bash
claude mcp add setlist-fm setlist-fm-pp-mcp -e SETLIST_FM_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/setlist-fm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SETLIST_FM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/setlist-fm/cmd/setlist-fm-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "setlist-fm": {
      "command": "setlist-fm-pp-mcp",
      "env": {
        "SETLIST_FM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

```bash
# Sync an artist and immediately run predictions
setlist-fm-pp-cli sync artist 'Radiohead' --max-pages 10 && \
  setlist-fm-pp-cli predict 'Radiohead' --last 20 --songs 22

# Which songs haven't been played in the longest time?
setlist-fm-pp-cli overdue 'Radiohead' --top 15 --json

# Compare two tours side by side
setlist-fm-pp-cli compare 'Phoenix' --tour 'Ti Amo Tour' --tour 'Alpha Zulu Tour' --json

# What does a typical show look like on this tour?
setlist-fm-pp-cli tour-shape 'Radiohead' --tour 'A Moon Shaped Pool Tour'

# Generate a bingo card for tonight's show
setlist-fm-pp-cli bingo 'Phish' --songs 25 --last 30

# Export last show as a Spotify-searchable playlist
setlist-fm-pp-cli playlist 'Radiohead' --last 1 --output spotify-search

# What covers does this artist play?
setlist-fm-pp-cli covers 'Phoebe Bridgers' --top 20

# What songs has this artist only played once?
setlist-fm-pp-cli debut 'Phoenix' --json

# How did two shows differ?
setlist-fm-pp-cli setlist-diff 53e3ab04 7be1aaa0

# Daily digest: what changed since yesterday?
setlist-fm-pp-cli since 2026-05-19T00:00:00Z --artist 'Radiohead' --agent

# Collector stats for a user
setlist-fm-pp-cli attended-stats dave42 --json

# Agent pipeline: sync, then answer a question
setlist-fm-pp-cli sync artist 'Radiohead' --max-pages 5 && \
  setlist-fm-pp-cli song-stats 'Radiohead' 'Karma Police' --agent
```

## Health Check

```bash
setlist-fm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/setlist-fm-pp-cli/config.toml`

Environment variables:

| Name | Required | Description |
| --- | --- | --- |
| `SETLISTFM_API_KEY` | Yes | Primary API key (matches setlistfm-js convention). |
| `SETLIST_FM_API_KEY` | No | Fallback API key (matches setlist-fm-client convention). |
| `SETLIST_FM_BASE_URL` | No | Override API base URL (default: `https://api.setlist.fm/rest`). |
| `SETLIST_FM_CONFIG` | No | Override config file path. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `setlist-fm-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SETLISTFM_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `search artists --name <name>` or `artist resolve <name>` to find valid IDs

### API-specific

- **HTTP 403 on every call** — Your API key is missing or invalid. Run `setlist-fm-pp-cli doctor` to confirm `SETLISTFM_API_KEY` is loaded.
- **HTTP 429 Too Many Requests** — You exceeded 2 req/sec or 1440/day. The client already throttles, but parallel runs will trip it. Wait, then re-run; consider `sync --max-pages N` to budget.
- **Empty results from `predict`/`overdue`/`song-stats`** — These commands read from the local store. Run `setlist-fm-pp-cli sync artist <name>` first.
- **`predict` results feel random** — Increase `--last N` (default 20) so more shows feed the probability model; sync more pages first.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**setlistfm-js**](https://github.com/terhuerne/setlistfm-js) — JavaScript
- [**setlist-fm-client**](https://github.com/zschumacher/setlist-fm-client) — Python
- [**repertorio**](https://github.com/jtmolon/repertorio) — Python
- [**nucleos/setlistfm**](https://github.com/nucleos/setlistfm) — PHP
- [**SetlistNet**](https://github.com/MolinRE/SetlistNet) — C#
- [**SetListR**](https://github.com/fusionet24/SetListR) — R

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
