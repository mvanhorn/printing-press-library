---
name: pp-movie-goat
description: "The movie CLI that combines TMDb's discovery engine with OMDb's multi-source ratings — and ships a SQLite watchlist that flags what's streaming on your services right now. Trigger phrases: `what should I watch tonight`, `where can I stream <title>`, `rate <title>`, `compare <title> and <title>`, `what's <person>'s filmography`, `plan a <franchise> marathon`, `use movie-goat`, `run movie-goat`."
version: "3.8.0"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - movie-goat-pp-cli
    install:
      - kind: go
        bins: [movie-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli
---

# Movie Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `movie-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install movie-goat --cli-only
   ```
2. Verify: `movie-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use Movie Goat when an agent needs to answer cinephile questions that require combining streaming availability with multi-source ratings. It is the right choice for tonight-picker scenarios, franchise marathon planning, and rated career timelines. It is not the right choice for box-office tracking, review sentiment analysis, or any workflow that needs LLM-style summaries of plot or reviews.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**discover** — Discover movies and TV shows with rich filters

- `movie-goat-pp-cli discover movies` — Discover movies by genre, year, rating, certification, cast, crew, streaming provider, and more
- `movie-goat-pp-cli discover tv` — Discover TV shows by genre, year, rating, network, and streaming provider

**genres** — Get genre lists for movies and TV

- `movie-goat-pp-cli genres movies` — Get the list of movie genres
- `movie-goat-pp-cli genres tv` — Get the list of TV genres

**movies** — Search and browse movies

- `movie-goat-pp-cli movies get` — Get detailed info about a movie including cast, ratings, and streaming availability
- `movie-goat-pp-cli movies now-playing` — Get movies currently in theaters
- `movie-goat-pp-cli movies popular` — Get current popular movies
- `movie-goat-pp-cli movies search` — Search for movies by title
- `movie-goat-pp-cli movies top-rated` — Get the highest rated movies
- `movie-goat-pp-cli movies upcoming` — Get movies coming soon to theaters

**multi** — Multi-search across movies, TV shows, and people

- `movie-goat-pp-cli multi <query>` — Search for movies, TV shows, and people in a single query

**people** — Search and browse people (actors, directors, crew)

- `movie-goat-pp-cli people get` — Get detailed info about a person including their filmography
- `movie-goat-pp-cli people popular` — Get popular people in entertainment
- `movie-goat-pp-cli people search` — Search for people by name

**trending** — Get trending movies, TV shows, and people

- `movie-goat-pp-cli trending all` — Get trending movies, TV, and people
- `movie-goat-pp-cli trending movies` — Get trending movies
- `movie-goat-pp-cli trending people` — Get trending people
- `movie-goat-pp-cli trending tv` — Get trending TV shows

**tv** — Search and browse TV shows

- `movie-goat-pp-cli tv airing-today` — Get TV shows with episodes airing today
- `movie-goat-pp-cli tv get` — Get detailed info about a TV show
- `movie-goat-pp-cli tv on-the-air` — Get TV shows currently on the air
- `movie-goat-pp-cli tv popular` — Get current popular TV shows
- `movie-goat-pp-cli tv search` — Search for TV shows by title
- `movie-goat-pp-cli tv top-rated` — Get the highest rated TV shows


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
movie-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Tonight, well-rated, on my services

```bash
movie-goat-pp-cli tonight --mood drama --max-runtime 130 --providers netflix,max,prime --region US --agent --select "results.title,results.year,results.rating,results.providers"
```

Streaming-filtered shortlist with only the high-gravity fields an agent needs to decide.

### Watchlist sweep

```bash
movie-goat-pp-cli watchlist list --available --providers netflix,max --region US --agent
```

Weekly check: which saved titles became streamable on services I have.

### Rated career deep dive

```bash
movie-goat-pp-cli career "Lynne Ramsay" --role director --agent --select "credits.title,credits.year,credits.rating_imdb,credits.rating_rt"
```

Agent-bounded chronological filmography with the cross-source rating columns that matter.

### Pick between two finalists

```bash
movie-goat-pp-cli versus 27205 87108 --region US --agent
```

Aligned compare card for Inception vs. Tenet; ratings, runtime, cast overlap, providers.

### Plan a franchise night

```bash
movie-goat-pp-cli marathon "Mission: Impossible" --order release --breaks-every 240 --agent
```

Ordered watchlist with total runtime and break suggestions.

## Auth Setup

TMDb v3 API key (free, https://www.themoviedb.org/settings/api) is required and goes in `TMDB_API_KEY` — used as a query parameter, not a Bearer header. OMDb key (free, http://www.omdbapi.com/apikey.aspx) is optional and goes in `OMDB_API_KEY`; without it, `ratings` and `versus` show TMDb-only and gracefully omit the IMDb/RT/Metacritic columns.

Run `movie-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  movie-goat-pp-cli movies get mock-value --agent --select id,name,status
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
movie-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
movie-goat-pp-cli feedback --stdin < notes.txt
movie-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.movie-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `MOVIE_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MOVIE_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
movie-goat-pp-cli profile save briefing --json
movie-goat-pp-cli --profile briefing movies get mock-value
movie-goat-pp-cli profile list --json
movie-goat-pp-cli profile show briefing
movie-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `movie-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add movie-goat-pp-mcp -- movie-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which movie-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   movie-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `movie-goat-pp-cli <command> --help`.
