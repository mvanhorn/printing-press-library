---
name: pp-movie-goat
description: "Find what to watch with Movie GOAT: movie and TV lookup, region-specific streaming availability, side-by-side comparisons, franchise marathons, people filmographies, and taste-bridging recommendations. Use when the user asks what to watch tonight, where something streams, how two movies compare, what else they might like based on favorites, or wants movie/TV data from TMDb with optional OMDb enrichment."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["movie-goat-pp-cli"],"env":["TMDB_API_KEY"]},"primaryEnv":"TMDB_API_KEY","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@latest","bins":["movie-goat-pp-cli"],"label":"Install via go install"}]}}'
---

# Movie Goat — Printing Press CLI

Movie GOAT is a movie and TV CLI built on TMDb, with optional OMDb enrichment for extra ratings and metadata. It is strongest when the user needs one of these workflows:

- a fast shortlist for tonight
- where a movie or show streams in a specific region
- a direct side-by-side comparison between two movies
- recommendations that bridge multiple favorites
- a franchise marathon plan with total runtime
- a sortable filmography for an actor or director

It can also sync data into a local SQLite archive for offline search and analytics.

## When to Use This CLI

Use Movie GOAT when the user asks about:

- movies or TV shows
- ratings or cross-source score comparisons
- streaming, rental, or purchase availability
- actors, directors, and filmographies
- what to watch next
- franchise watch order or marathon planning
- discovering titles by genre, year, rating, or provider

Do not use it when the user specifically needs:

- current critic-review text from a named publication or critic
- theatrical showtimes by local theater
- live box-office reporting beyond what OMDb/TMDb metadata can provide

## Best Command Mapping

- "What should I watch tonight?" → `movie-goat-pp-cli tonight --agent`
- "Give me a random good sci-fi movie" → `movie-goat-pp-cli blind --genre 878 --min-rating 7.5 --agent`
- "Where can I stream Oppenheimer?" → `movie-goat-pp-cli watch "Oppenheimer" --region US --agent`
- "Compare Dune and Blade Runner 2049" → `movie-goat-pp-cli versus "Dune" "Blade Runner 2049" --agent`
- "I liked Parasite and Oldboy, what else?" → `movie-goat-pp-cli recommend-for-me "Parasite" "Oldboy" --agent`
- "Show Denis Villeneuve's best work" → `movie-goat-pp-cli career "Denis Villeneuve" --sort rating --agent`
- "Plan a Lord of the Rings marathon" → `movie-goat-pp-cli marathon "Lord of the Rings" --agent`
- "Find action movies from 2024 on Netflix" → `movie-goat-pp-cli discover --with-genres 28 --primary-release-year 2024 --with-watch-providers 8 --watch-region US --agent`

## Unique Capabilities

### `movies get`

Get one detailed movie record with appended credits, videos, recommendations, watch providers, and external IDs. If `OMDB_API_KEY` is set and the title has an IMDb ID, the response can also include IMDb, Rotten Tomatoes, and Metacritic data.

```bash
movie-goat-pp-cli movies get 550 --agent
```

### `watch`

Look up where a movie or TV show is available to stream, rent, buy, or watch free/ad-supported in a given region. Accepts either a title or a TMDb ID.

```bash
movie-goat-pp-cli watch "Breaking Bad" --type tv --region US --agent
```

### `recommend-for-me`

Takes two or more movie titles, fetches recommendations and similar titles for each, and ranks candidates by how well they bridge the combined tastes.

```bash
movie-goat-pp-cli recommend-for-me "The Matrix" "Arrival" "Mad Max: Fury Road" --agent
```

### `versus`

Compares two movies side by side across TMDb data and, when configured, OMDb enrichment such as IMDb, Rotten Tomatoes, Metacritic, awards, and box office.

```bash
movie-goat-pp-cli versus "Barbie" "Oppenheimer" --agent
```

### `blind`

Picks a random high-quality movie using TMDb discover filters such as genre, decade, minimum rating, and watch provider.

```bash
movie-goat-pp-cli blind --genre 35 --decade 1990s --min-rating 7.0 --agent
```

### `tonight`

Builds a short ranked list from TMDb trending and popular results, optionally filtered by genre or media type.

```bash
movie-goat-pp-cli tonight --genre 28 --agent
movie-goat-pp-cli tonight --type tv --agent
```

### `career`

Returns a person's combined filmography with sortable output and summary stats.

```bash
movie-goat-pp-cli career "Cate Blanchett" --sort rating --agent
```

### `marathon`

Finds a TMDb collection, sorts its films by release date, totals runtime, and suggests break points.

```bash
movie-goat-pp-cli marathon "Harry Potter" --agent
```

### `sync` + `search` + `analytics`

Sync TMDb data into the local SQLite archive, then search or analyze it without re-hitting the API every time.

```bash
movie-goat-pp-cli sync
movie-goat-pp-cli search "time travel" --data-source local --agent
movie-goat-pp-cli analytics --agent
```

## Command Reference

Discovery and recommendations:

- `movie-goat-pp-cli tonight`
- `movie-goat-pp-cli blind`
- `movie-goat-pp-cli recommend-for-me <title1> [title2] [title3...]`
- `movie-goat-pp-cli discover [filters]`
- `movie-goat-pp-cli discover tv [filters]`
- `movie-goat-pp-cli trending`

Movie and TV lookup:

- `movie-goat-pp-cli movies`
- `movie-goat-pp-cli movies search "<title>"`
- `movie-goat-pp-cli movies get <movieId>`
- `movie-goat-pp-cli tv`
- `movie-goat-pp-cli tv search "<title>"`
- `movie-goat-pp-cli tv get <seriesId>`
- `movie-goat-pp-cli tv seasons get <seriesId> <seasonNumber>`
- `movie-goat-pp-cli genres`
- `movie-goat-pp-cli genres tv`

People and planning:

- `movie-goat-pp-cli people`
- `movie-goat-pp-cli people search "<name>"`
- `movie-goat-pp-cli people get <personId>`
- `movie-goat-pp-cli career <person-name-or-id>`
- `movie-goat-pp-cli watch <title-or-id>`
- `movie-goat-pp-cli versus <movie1> <movie2>`
- `movie-goat-pp-cli marathon <collection-name>`

Local and power-user commands:

- `movie-goat-pp-cli sync`
- `movie-goat-pp-cli search "<query>"`
- `movie-goat-pp-cli analytics`
- `movie-goat-pp-cli workflow archive`
- `movie-goat-pp-cli workflow status`
- `movie-goat-pp-cli export`
- `movie-goat-pp-cli import`
- `movie-goat-pp-cli tail <resource>`
- `movie-goat-pp-cli api`
- `movie-goat-pp-cli doctor`
- `movie-goat-pp-cli auth status|set-token|logout`

## Practical Recipes

### What should I watch tonight?

```bash
movie-goat-pp-cli tonight --agent
```

Use `--genre` or `--type tv` when the user already has a narrower intent.

### Compare two movies before committing

```bash
movie-goat-pp-cli versus "Inception" "Tenet" --agent
```

Good for "which should I watch first?" or "which one wins on ratings and box office?"

### Find where something streams

```bash
movie-goat-pp-cli watch "The Dark Knight" --region US --agent
movie-goat-pp-cli watch "Breaking Bad" --type tv --region GB --agent
```

Use titles when the user does not know IDs. Use `--type tv` when the title is a show.

### Discover by structured filters

```bash
movie-goat-pp-cli genres --agent
movie-goat-pp-cli discover --with-genres 878 --primary-release-year 2005 --vote-average-gte 7.5 --with-watch-providers 8 --watch-region US --agent
```

Use `genres` first if the user gives natural-language genre names and you need the TMDb numeric IDs.

### Research a director or actor

```bash
movie-goat-pp-cli career "Denis Villeneuve" --sort rating --agent
```

Use `--sort popularity` for broader career overview and `--limit` when the answer should stay short.

## Auth Setup

TMDb credentials are required for core usage.

```bash
export TMDB_API_KEY="<your-key>"
movie-goat-pp-cli auth set-token <your-key>
movie-goat-pp-cli doctor
```

Optional:

- `OMDB_API_KEY` adds IMDb, Rotten Tomatoes, Metacritic, awards, and box-office enrichment where available.

## Agent Mode

Add `--agent` to commands you want optimized for machine consumption.

It expands to:

- `--json`
- `--compact`
- `--no-input`
- `--no-color`
- `--yes`

Useful companion flags:

- `--select <fields>`
- `--dry-run`
- `--no-cache`
- `--rate-limit <n>`
- `--data-source auto|live|local`

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 2 | Usage error |
| 3 | Not found |
| 4 | Auth error |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Argument Parsing

Given `$ARGUMENTS`:

1. Empty, `help`, or `--help` → run `movie-goat-pp-cli --help`
2. `install` → install CLI
3. `install mcp` → install MCP server
4. Anything else → map the user request to the best command above and run it with `--agent`

## CLI Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-cli@main
movie-goat-pp-cli --version
```

## MCP Server Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movie-goat/cmd/movie-goat-pp-mcp@main
claude mcp add movie-goat movie-goat-pp-mcp -e TMDB_API_KEY=<your-key>
```

If OMDb enrichment is desired in MCP clients too:

```bash
claude mcp add movie-goat movie-goat-pp-mcp -e TMDB_API_KEY=<your-key> -e OMDB_API_KEY=<your-omdb-key>
```

## Direct Use

1. Check whether `movie-goat-pp-cli` is installed.
2. If not installed, offer CLI installation.
3. If `TMDB_API_KEY` is missing, guide the user through auth setup before trying to use the CLI.
4. Choose the command that matches the user's intent most directly.
5. Run with `--agent` unless the user explicitly wants human-formatted output.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
movie-goat-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
movie-goat-pp-cli --profile <name> <command>

# List / inspect / remove
movie-goat-pp-cli profile list
movie-goat-pp-cli profile show <name>
movie-goat-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
movie-goat-pp-cli <command> --deliver file:/path/to/out.json
movie-goat-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
movie-goat-pp-cli feedback "what surprised you or tripped you up"
movie-goat-pp-cli feedback list         # show local entries
movie-goat-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.movie-goat-pp-cli/feedback.jsonl` as JSON lines. When `MOVIE_GOAT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `MOVIE_GOAT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

