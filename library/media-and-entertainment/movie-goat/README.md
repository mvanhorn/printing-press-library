# Movie Goat CLI

**The movie CLI that combines TMDb's discovery engine with OMDb's multi-source ratings — and ships a SQLite watchlist that flags what's streaming on your services right now.**

Movie Goat unifies the workflows that today require four browser tabs: discovery (TMDb), multi-source ratings (TMDb + IMDb + RT + Metacritic via OMDb), where-to-watch (TMDb watch providers), and a local watchlist that knows your streaming services. Flagship transcendence: `tonight` picks something well-rated that's actually streaming on your services; `ratings` shows the four canonical scores for any title; `marathon` totals runtime across a franchise.

Learn more at [Movie Goat](https://www.themoviedb.org).

## Install

The recommended path installs both the `movie-goat-pp-cli` binary and the `pp-movie-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install movie-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install movie-goat --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/movie-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

TMDb v3 API key (free, https://www.themoviedb.org/settings/api) is required and goes in `TMDB_API_KEY` — used as a query parameter, not a Bearer header. OMDb key (free, http://www.omdbapi.com/apikey.aspx) is optional and goes in `OMDB_API_KEY`; without it, `ratings` and `versus` show TMDb-only and gracefully omit the IMDb/RT/Metacritic columns.

## Quick Start

```bash
# The fastest path to the value: a streaming-filtered, well-rated shortlist for tonight.
movie-goat-pp-cli tonight --providers "Netflix,Max" --region US


# Multi-type search across movies, TV, and people.
movie-goat-pp-cli multi "the bear"


# Cross-source rating card for Fight Club (TMDb id 550).
movie-goat-pp-cli ratings 550


# Add Inception to the local SQLite watchlist.
movie-goat-pp-cli watchlist add 27205 --kind movie


# Show which saved titles are streamable on your services right now.
movie-goat-pp-cli watchlist list --available --providers netflix,max --region US


# Plan a franchise marathon with totalled runtime.
movie-goat-pp-cli marathon "The Avengers" --order release

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cinephile rituals
- **`tonight`** — Pick what to watch tonight from trending titles actually streaming on your services.

  _Use this when an agent needs a streaming-filtered shortlist; one call replaces tab-bouncing across TMDb/RT/JustWatch._

  ```bash
  movie-goat-pp-cli tonight --mood thriller --max-runtime 120 --providers netflix,max --region US --json
  ```
- **`ratings`** — TMDb + IMDb + Rotten Tomatoes + Metacritic ratings for any title in one card.

  _Use when an agent needs the canonical multi-source rating for a title; degrades gracefully to TMDb-only if OMDB_API_KEY is unset._

  ```bash
  movie-goat-pp-cli ratings 550 --json
  ```
- **`marathon`** — Plan a franchise marathon with watch order, total runtime, and suggested breaks.

  _Use when planning an event watch; agent can dump the schedule to share with a group._

  ```bash
  movie-goat-pp-cli marathon "The Avengers" --order release --breaks-every 240 --json
  ```
- **`career`** — Explore any actor or director's full filmography with ratings and chronology.

  _Use when an agent needs a rated chronological filmography; replaces flat IMDb lists with cross-source ratings._

  ```bash
  movie-goat-pp-cli career "Christopher Nolan" --since 2010 --role director --json
  ```
- **`versus`** — Compare two movies or shows side-by-side across ratings, cast, runtime, and streaming.

  _Use when an agent has to pick between two finalists; one command shows where they differ on every axis._

  ```bash
  movie-goat-pp-cli versus 550 27205 --region US --json
  ```
- **`collaborators`** — List people who appear in 2+ of a person's credits, with count and titles.

  _Use when an agent is researching a filmmaker's circle; surfaces recurring DPs/composers/actors mechanically._

  ```bash
  movie-goat-pp-cli collaborators "Christopher Nolan" --min-count 3 --role crew --json
  ```

### Local state that compounds
- **`watchlist list`** — Local SQLite watchlist; flag rows that are streamable on your services.

  _Use weekly to surface streamable items from a saved list; eliminates ad-hoc JustWatch checks per title._

  ```bash
  movie-goat-pp-cli watchlist list --available --providers netflix,max --region US --json
  ```
- **`queue`** — Suggest next-watch picks derived from your watchlist's recommendations and similars.

  _Use when an agent needs a fresh queue derived from saved interests; combines local state with API recommendations._

  ```bash
  movie-goat-pp-cli queue --limit 20 --providers netflix,max --region US --json
  ```

## Usage

Run `movie-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### discover

Discover movies and TV shows with rich filters

- **`movie-goat-pp-cli discover movies`** - Discover movies by genre, year, rating, certification, cast, crew, streaming provider, and more
- **`movie-goat-pp-cli discover tv`** - Discover TV shows by genre, year, rating, network, and streaming provider

### genres

Get genre lists for movies and TV

- **`movie-goat-pp-cli genres movies`** - Get the list of movie genres
- **`movie-goat-pp-cli genres tv`** - Get the list of TV genres

### movies

Search and browse movies

- **`movie-goat-pp-cli movies get`** - Get detailed info about a movie including cast, ratings, and streaming availability
- **`movie-goat-pp-cli movies now-playing`** - Get movies currently in theaters
- **`movie-goat-pp-cli movies popular`** - Get current popular movies
- **`movie-goat-pp-cli movies search`** - Search for movies by title
- **`movie-goat-pp-cli movies top-rated`** - Get the highest rated movies
- **`movie-goat-pp-cli movies upcoming`** - Get movies coming soon to theaters

### multi

Multi-search across movies, TV shows, and people

- **`movie-goat-pp-cli multi search`** - Search for movies, TV shows, and people in a single query

### people

Search and browse people (actors, directors, crew)

- **`movie-goat-pp-cli people get`** - Get detailed info about a person including their filmography
- **`movie-goat-pp-cli people popular`** - Get popular people in entertainment
- **`movie-goat-pp-cli people search`** - Search for people by name

### trending

Get trending movies, TV shows, and people

- **`movie-goat-pp-cli trending all`** - Get trending movies, TV, and people
- **`movie-goat-pp-cli trending movies`** - Get trending movies
- **`movie-goat-pp-cli trending people`** - Get trending people
- **`movie-goat-pp-cli trending tv`** - Get trending TV shows

### tv

Search and browse TV shows

- **`movie-goat-pp-cli tv airing-today`** - Get TV shows with episodes airing today
- **`movie-goat-pp-cli tv get`** - Get detailed info about a TV show
- **`movie-goat-pp-cli tv on-the-air`** - Get TV shows currently on the air
- **`movie-goat-pp-cli tv popular`** - Get current popular TV shows
- **`movie-goat-pp-cli tv search`** - Search for TV shows by title
- **`movie-goat-pp-cli tv top-rated`** - Get the highest rated TV shows


## Cookbook

Real-world recipes using verified flag names. Pipe `--json` and chain with
`jq` for scripting; pair with `--select` to keep only the fields you need.

```bash
# 1. Movie-night picker: thrillers under 2h that are streaming on your services tonight.
movie-goat-pp-cli tonight --mood thriller --max-runtime 120 --providers netflix,max --region US

# 2. Multi-source ratings card for any title (TMDb, IMDb, RT, Metacritic).
movie-goat-pp-cli ratings 550

# 3. Plan a marathon, skipping unreleased entries (default), with a break every ~3.5 hrs.
movie-goat-pp-cli marathon "The Lord of the Rings" --order release --breaks-every 210

# 4. Compare two finalists side-by-side before committing.
movie-goat-pp-cli versus "Fight Club" "Inception" --region US --json

# 5. Filmmaker's recurring crew — useful for casting/producer research.
movie-goat-pp-cli collaborators "Christopher Nolan" --role crew --min-count 3

# 6. Director's filmography since 2010, sorted chronologically.
movie-goat-pp-cli career "Christopher Nolan" --since 2010 --role director

# 7. Save a watchlist locally and check what's streamable now.
movie-goat-pp-cli watchlist add 27205 --kind movie
movie-goat-pp-cli watchlist list --available --providers netflix,max --region US

# 8. Discover trending titles, JSON-only, top 5 fields.
movie-goat-pp-cli trending all --json --select results.title,results.popularity | jq '.results[:5]'

# 9. Search and pipe straight into `jq` for the top result.
movie-goat-pp-cli movies search "Dune" --json | jq '.results[0] | {id, title, year: .release_date[0:4]}'

# 10. Get a movie by id with credits + watch providers in one call.
movie-goat-pp-cli movies get 550 --append-to-response credits,watch/providers,videos

# 11. Discover-by-filter: action thrillers from 2020 onwards, sorted by vote count.
movie-goat-pp-cli discover movies --with-genres 28,53 --primary-release-date-gte 2020-01-01 --vote-count-gte 1000 --sort-by vote_average.desc

# 12. Recommendation queue from your watchlist (suggests next-watch picks).
movie-goat-pp-cli queue --providers netflix,max --region US --limit 10
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
movie-goat-pp-cli movies search "Inception"

# JSON for scripting and agents
movie-goat-pp-cli movies search "Inception" --json

# Filter to specific fields
movie-goat-pp-cli movies get 27205 --json --select id,title,runtime

# Dry run — show the request without sending
movie-goat-pp-cli movies get 27205 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
movie-goat-pp-cli movies get 27205 --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-movie-goat -g
```

Then invoke `/pp-movie-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add movie-goat movie-goat-pp-mcp -e TMDB_API_KEY=<your-key>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-movie-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-movie-goat --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-movie-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-movie-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/movie-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TMDB_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "movie-goat": {
      "command": "movie-goat-pp-mcp",
      "env": {
        "TMDB_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
movie-goat-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/movie-goat-pp-cli/config.toml`

Environment variables:
- `TMDB_API_KEY`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `movie-goat-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TMDB_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized: "Invalid API key: You must be granted a valid key."** — TMDb v3 keys (32-char hex) must be sent as a query parameter, not as a Bearer header. The CLI does this automatically — re-check that TMDB_API_KEY is set to the v3 key from https://www.themoviedb.org/settings/api, not a v4 read access token.
- **`ratings` shows only the TMDb score and skips IMDb/RT/Metacritic columns.** — Set OMDB_API_KEY to a free key from http://www.omdbapi.com/apikey.aspx. Without it the CLI degrades to TMDb-only by design.
- **`watchlist list --available` returns rows but every row says `not available`.** — Pass `--region` (e.g. `--region US`) and confirm `--providers` matches TMDb's canonical provider names exactly (e.g. `"Netflix"`, `"Max"`, `"Amazon Prime Video"`). Use `movie-goat-pp-cli movies get <id> --select watch_providers` to see the canonical names for any title.
- **429 Too Many Requests when running `career` or `marathon` on long filmographies/collections.** — TMDb's rate limit is ~40 req/10s. The CLI's adaptive limiter ramps down on 429 automatically; rerun, and the ceiling will be discovered. For very long careers, use `--since YEAR` to scope the fan-out.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tmdb-mcp**](https://github.com/xdwanj/tmdb-mcp) — Go
- [**imdb-mcp-server**](https://github.com/uzaysozen/imdb-mcp-server) — Python
- [**tmdb-cli (degerahmet)**](https://github.com/degerahmet/tmdb-cli) — Go
- [**TMDB_CLI (illegalbyte)**](https://github.com/illegalbyte/TMDB_CLI) — Python
- [**mediascore**](https://github.com/dkorunic/mediascore) — Go
- [**tmdbv3api**](https://github.com/AnthonyBloomer/tmdbv3api) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
