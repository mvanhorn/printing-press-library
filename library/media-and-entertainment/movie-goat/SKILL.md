---
name: pp-movie-goat
description: "The movie CLI that combines TMDb's discovery engine with OMDb's multi-source ratings — and ships a SQLite watchlist that flags what's streaming on your services right now. Trigger phrases: `what should I watch tonight`, `where can I stream <title>`, `rate <title>`, `compare <title> and <title>`, `what's <person>'s filmography`, `plan a <franchise> marathon`, `use movie-goat`, `run movie-goat`."
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

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install movie-goat --cli-only
   ```
2. Verify: `movie-goat-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

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
- **Remake-aware title resolution** — Every command that accepts a title instead of a TMDb id says so out loud when the title is shared.

  _TMDb's search ranks by a proprietary relevance score, so `"Sabrina"` resolves to the 1995 remake even though the 1954 Wilder original is better rated. When a title has more than one well-rated match, the CLI reports it on **both** channels: a human notice on stderr, and a `meta.ambiguous` record in the JSON on stdout for consumers that never read stderr. Pin the one you meant with `--year`, a `"Title (YYYY)"` suffix, or the id._

  ```bash
  movie-goat-pp-cli ratings "Sabrina" --year 1954 --json
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

**auth** — Manage TMDB_API_KEY and OMDB_API_KEY credentials

- `movie-goat-pp-cli auth status` — Show authentication status for both the TMDb and OMDb credentials
- `movie-goat-pp-cli auth set-token` — Save the TMDb API token to the config file
- `movie-goat-pp-cli auth set-omdb-token` — Save the optional OMDb API token to the config file
- `movie-goat-pp-cli auth logout` — Clear stored credentials

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

### Store both API keys without touching the shell environment

```bash
movie-goat-pp-cli auth set-token YOUR_TMDB_API_KEY
movie-goat-pp-cli auth set-omdb-token YOUR_OMDB_API_KEY
movie-goat-pp-cli auth status --agent
```

Writes both credentials to `~/.config/movie-goat-pp-cli/config.toml` (mode `0600`) so every agent and shell on the machine sees them, not just the one that exported an environment variable. `auth status` reports `authenticated`, `omdb_configured`, and `omdb_source` so an agent can tell whether the IMDb / RT / Metacritic columns will be populated before it runs `ratings`.

### Pin the original, not the remake

```bash
movie-goat-pp-cli ratings "Sabrina" --year 1954 --agent
movie-goat-pp-cli ratings "Sabrina (1954)" --agent
movie-goat-pp-cli versus "Sabrina (1954)" "Sabrina (1995)" --agent
```

`ratings`, `marathon`, and `watchlist add` take `--year`; every title-taking
command (including the two-positional `versus`) accepts the inline `"Title (YYYY)"`
suffix. Without either, the CLI still takes TMDb's top-ranked result but prints
the rival ids on stderr:

```
warn: "Sabrina" matches 3 titles on TMDb; using id 11860 — Sabrina (1995).
      TMDb's search relevance put it first, but Sabrina (1954) has more ratings (1373 vs 703).
      Other matches:
        6620  Sabrina (1954)
        503902  Sabrina (2018)
      Disambiguate with --year <YYYY>, a "title (YYYY)" suffix, or the TMDb id.
```

`/search/*` orders results by a relevance score TMDb does not expose. It is not
the vote count and not the `popularity` field — in this very case the 1954 entry
leads on both (1373 vs 703 ratings, 4.25 vs 3.60 popularity) and still comes back
second. That is why the top result can differ from the canonical edition, and why
the notice compares vote counts: they are the only ranking input you can read.

Unrated same-title obscurities never trigger it, so `ratings "Inception"` stays
silent.

**A script that never reads stderr still finds out.** The same event is recorded
in the JSON response under `meta.ambiguous`, so a cron job or a pipeline running
with `2>/dev/null` — the case where nobody is watching and a wrong year is most
dangerous — can detect it:

```bash
movie-goat-pp-cli ratings "Sabrina" --agent 2>/dev/null | jq '.meta.ambiguous'
```

```json
[
  {
    "query": "Sabrina",
    "kind": "titles",
    "match_count": 3,
    "signal": "alternative_better_rated",
    "chosen":  { "tmdb_id": 11860, "title": "Sabrina", "year": "1995", "vote_count": 703 },
    "alternatives": [
      { "tmdb_id": 6620,   "title": "Sabrina", "year": "1954", "vote_count": 1373 },
      { "tmdb_id": 503902, "title": "Sabrina", "year": "2018", "vote_count": 194 }
    ],
    "hint": "Disambiguate with --year <YYYY>, a \"title (YYYY)\" suffix, or the TMDb id."
  }
]
```

`signal` is `alternative_better_rated` when the entry TMDb's search ranked first
is *not* the best-rated one — treat that as "stop and pin an id" — or
`multiple_exact_matches` when the top pick is also the best-rated. `meta.ambiguous`
is a list because one command can resolve several titles (`versus` resolves two).

`popularity` is TMDb's trending score, passed through as reported. It is *not*
the order the results came back in, and it is not what the signal compares —
that is `vote_count`.

Rules worth knowing:

- **Additive.** The field appears only when the stderr notice fires. Unambiguous
  lookups carry no `meta` key at all, so nothing changes for existing parsers.
- **`--select` cannot filter it out.** `--select title,ratings` still returns
  `meta.ambiguous` when a lookup was ambiguous — narrowing the field list is
  exactly the habit that would otherwise hide it.
- **`--compact` / `--agent` keep it.** The compact allow-list applies to arrays;
  these responses are objects, whose compaction is a blocklist that leaves `meta`
  alone.
- **`--quiet` silences the stderr notice but not the record.** They are separate
  channels with separate audiences. Note that on these commands `--quiet`
  suppresses stdout entirely (pre-existing behavior), so use `--json` without
  `--quiet` to read the field.

### Plan a franchise night

```bash
movie-goat-pp-cli marathon "Mission: Impossible" --order release --breaks-every 240 --agent
```

Ordered watchlist with total runtime and break suggestions.

## Auth Setup

Movie Goat uses two API keys, and each can live in the config file or the environment.

| Key | Required | Save to config | Environment variable |
|---|---|---|---|
| TMDb v3 (free, https://www.themoviedb.org/settings/api) | yes | `movie-goat-pp-cli auth set-token <token>` | `TMDB_API_KEY` |
| OMDb (free, http://www.omdbapi.com/apikey.aspx) | no | `movie-goat-pp-cli auth set-omdb-token <token>` | `OMDB_API_KEY` |

Both keys are stored in `~/.config/movie-goat-pp-cli/config.toml` (mode `0600`) as `api_key` and `omdb_api_key`. The environment variable wins over the saved value for either key, so an existing environment-based setup keeps working unchanged.

The TMDb key is sent as a query parameter, not a Bearer header. Without an OMDb key, `ratings`, `versus`, and `career` show TMDb-only and gracefully omit the IMDb / RT / Metacritic columns.

`movie-goat-pp-cli auth status` reports both credentials and where each resolved from; `movie-goat-pp-cli auth logout` clears both from the config file.

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
   claude mcp add -e TMDB_API_KEY=<your-tmdb-key> -e OMDB_API_KEY=<your-omdb-key> movie-goat-pp-mcp -- movie-goat-pp-mcp
   ```
   `OMDB_API_KEY` is optional, but it enables IMDb, Rotten Tomatoes, and Metacritic enrichment in `ratings`, `versus`, and `career`.
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
