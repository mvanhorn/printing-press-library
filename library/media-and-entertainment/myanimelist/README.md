# MyAnimeList CLI

**Every public MyAnimeList surface in one binary, plus offline analytics the site itself cannot answer**

Search, inspect, and rank anime and manga from the terminal with no API key, no OAuth, and no browser. Everything you look at is cached in SQLite so it stays searchable offline, and local commands like `drift`, `week`, and `franchise gap` answer questions MyAnimeList has no page for: how a score moved, what airs tonight in your timezone, and which franchise entries you skipped.

## Install

The recommended path installs both the `myanimelist-pp-cli` binary and the `pp-myanimelist` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install myanimelist
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install myanimelist --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install myanimelist --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install myanimelist --agent claude-code
npx -y @mvanhorn/printing-press-library install myanimelist --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/cmd/myanimelist-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/myanimelist-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine ./myanimelist-pp-cli`. On Unix, mark it executable: `chmod +x ./myanimelist-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install myanimelist --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-myanimelist --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-myanimelist --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install myanimelist --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/myanimelist-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/cmd/myanimelist-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "myanimelist": {
      "command": "myanimelist-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credentials, ever. The CLI reads MyAnimeList's public pages, its own suggest endpoint, its RSS feeds, and its advertised sitemap over plain HTTP. There is no API key to set, no OAuth flow to complete, and no session cookie to keep alive. Requests are polite by default: a descriptive User-Agent, one request per second, and a 24-hour cache so repeat reads cost nothing.

## Quick Start

```bash
# Verify the runtime and cache are healthy before anything else.
myanimelist-pp-cli doctor --dry-run

# Pull a full title record by MyAnimeList id; the id is the only identifier you need.
myanimelist-pp-cli anime get 52991 --json

# Find ids fast with the site's own instant-search endpoint.
myanimelist-pp-cli instant --query frieren --entity-type anime --json

# See a whole season grouped by format.
myanimelist-pp-cli season list 2026 fall --json

# Start a local watch record so week, next, and franchise gap have data to work with.
myanimelist-pp-cli track add 52991 --status watching --progress 3

# Get this week's schedule for your own shows in your own timezone.
myanimelist-pp-cli week

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Reception analytics the site hides
- **`anime divisive`** — See whether a title is universally loved or bitterly split, instead of reading one average score.

  _Use it before committing to a long series when the average score hides a love-it-or-hate-it split._

  ```bash
  myanimelist-pp-cli anime divisive 5114 --json
  ```
- **`anime drop-risk`** — Judge how likely a title is to be abandoned before you commit dozens of episodes to it.

  _Use it to triage a plan-to-watch backlog by follow-through rather than by average score._

  ```bash
  myanimelist-pp-cli anime drop-risk 21 --json
  ```
- **`anime consistency`** — See whether a show improves, holds, or falls apart across its episodes using per-episode poll data.

  _Use it to decide whether to keep watching mid-season rather than finishing a dud._

  ```bash
  myanimelist-pp-cli anime consistency 52991 --json
  ```

### Local state that compounds
- **`drift`** — See how a title's score, member count, favorites, and rank have moved since you last looked.

  _Reach for it when a question depends on change over time rather than the current number._

  ```bash
  myanimelist-pp-cli drift --since 30d --json
  ```
- **`week`** — Get a timezone-correct seven-day grid of just the shows you track, with same-hour collisions flagged.

  _Use it to plan what to watch tonight without hand-converting Japan time for every show._

  ```bash
  myanimelist-pp-cli week --json
  ```

### Franchise intelligence
- **`adaptation`** — Learn whether an anime covered its source manga and how much source material is left.

  _Use it when a user asks where to pick up the manga after the anime._

  ```bash
  myanimelist-pp-cli adaptation 52991 --json
  ```
- **`franchise gap`** — Find the sequel, movie, or side story you skipped in every franchise you have already started.

  _Use it to catch missing entries before starting a new season of a franchise._

  ```bash
  myanimelist-pp-cli franchise gap --json
  ```

## Recipes

### Narrow a title record for an agent

```bash
myanimelist-pp-cli anime get 52991 --agent --select id,title,score,episodes,status,genres,studios
```

Title pages are large; --select keeps only the fields an agent needs.

### Trim nested instant-search results

```bash
myanimelist-pp-cli instant --query frieren --entity-type anime --agent --select categories.items.name,categories.items.payload.score
```

The suggest response nests categories and payloads; dotted --select flattens the useful fields.

### Is this show split or universally loved?

```bash
myanimelist-pp-cli anime divisive 5114
```

Turns the 1-10 vote distribution into a polarization reading.

### Audit a franchise you started

```bash
myanimelist-pp-cli franchise gap
```

Lists entries missing from franchises already in your local library.

### Plan the week in your own timezone

```bash
myanimelist-pp-cli week
```

Shows only your tracked shows, with same-hour collisions flagged.

## Usage

Run `myanimelist-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MYANIMELIST_CONFIG_DIR`, `MYANIMELIST_DATA_DIR`, `MYANIMELIST_STATE_DIR`, or `MYANIMELIST_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MYANIMELIST_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MYANIMELIST_HOME=/srv/myanimelist
myanimelist-pp-cli doctor
```

Under `MYANIMELIST_HOME=/srv/myanimelist`, the four dirs resolve to `/srv/myanimelist/config`, `/srv/myanimelist/data`, `/srv/myanimelist/state`, and `/srv/myanimelist/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "myanimelist": {
      "command": "myanimelist-pp-mcp",
      "env": {
        "MYANIMELIST_HOME": "/srv/myanimelist"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MYANIMELIST_DATA_DIR` overrides an explicit `--home` for that kind. Use `MYANIMELIST_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MYANIMELIST_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `myanimelist-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### anime

Browse, search, and inspect anime titles

- **`myanimelist-pp-cli anime characters`** - Characters and their Japanese/English voice actors for an anime
- **`myanimelist-pp-cli anime episodes`** - Episode table with titles, air dates, poll averages, and reply counts
- **`myanimelist-pp-cli anime forum`** - Discussion boards attached to an anime title
- **`myanimelist-pp-cli anime get`** - Fetch one anime title's full detail page by MyAnimeList id
- **`myanimelist-pp-cli anime list`** - Search anime with the site's full advanced filters (query, format, score, airing status, rating, year range, genre, sort)
- **`myanimelist-pp-cli anime moreinfo`** - Extended credits, external links, and full theme-song credits
- **`myanimelist-pp-cli anime pictures`** - Image gallery (posters, key visuals, screenshots) for an anime
- **`myanimelist-pp-cli anime recommendations`** - Community recommendations for an anime, with recommendation counts
- **`myanimelist-pp-cli anime reviews`** - Community reviews with scores, helpful counts, and spoiler flags
- **`myanimelist-pp-cli anime stats`** - Score distribution (1-10 with vote counts) and status distribution for an anime
- **``** - Streaming episodes and promos for an anime (the site's own video JSON, no auth) with a real pager
- **`myanimelist-pp-cli anime videos`** - Promotional videos and trailers for an anime

### character

Characters

- **`myanimelist-pp-cli character <id>`** - A character's profile with appearances and voice actors

### feed

RSS feeds published by MyAnimeList

- **``** - Featured-article RSS feed (XML)
- **``** - News RSS feed (XML)
- **``** - A member's public activity feed (needs ?u=; without it the endpoint 404s)

### forum

Public forum boards and topics

- **`myanimelist-pp-cli forum boards`** - All forum boards with their current topic counts
- **`myanimelist-pp-cli forum topic`** - One forum topic and its posts
- **`myanimelist-pp-cli forum topics`** - Topics in one forum board

### genre

Browse by genre, theme, or demographic

- **`myanimelist-pp-cli genre anime`** - Anime in a genre, theme, or demographic
- **`myanimelist-pp-cli genre manga`** - Manga in a genre, theme, or demographic

### instant

Instant cross-entity search

- **`myanimelist-pp-cli instant`** - Instant search across anime, manga, characters, and people (the site's own suggest API, JSON)

### magazine

Manga serialization magazines

- **`myanimelist-pp-cli magazine <id>`** - A magazine's page with every serialized title

### manga

Browse, search, and inspect manga, novels, and one-shots

- **`myanimelist-pp-cli manga characters`** - Characters credited in a manga
- **`myanimelist-pp-cli manga get`** - Fetch one manga title's full detail page by MyAnimeList id
- **`myanimelist-pp-cli manga list`** - Search manga with the site's advanced filters
- **`myanimelist-pp-cli manga moreinfo`** - Extended credits and external links for a manga
- **`myanimelist-pp-cli manga pictures`** - Image gallery for a manga
- **`myanimelist-pp-cli manga recommendations`** - Community recommendations for a manga
- **`myanimelist-pp-cli manga reviews`** - Community reviews for a manga
- **`myanimelist-pp-cli manga stats`** - Score distribution and status distribution for a manga

### member

Public MyAnimeList user profiles and lists

- **`myanimelist-pp-cli member anime-list`** - A public anime list as JSON rows (status, score, progress, tags); no credential required
- **`myanimelist-pp-cli member get`** - A public member profile: statistics, favorites, and recent activity (no credential required)
- **`myanimelist-pp-cli member manga-list`** - A public manga list as JSON rows; no credential required
- **`myanimelist-pp-cli member statistics`** - A member's public aggregate statistics

### news

Anime and manga news

- **`myanimelist-pp-cli news get`** - A single news article, body included
- **`myanimelist-pp-cli news list`** - Latest news headlines with pagination

### person

Voice actors and staff

- **`myanimelist-pp-cli person <id>`** - A person's page: animeography, voice roles, and staff credits

### ranking

Top anime and manga rankings

- **`myanimelist-pp-cli ranking anime`** - Top anime by type: all, airing, upcoming, tv, movie, ova, ona, special, bypopularity, favorite
- **`myanimelist-pp-cli ranking manga`** - Top manga by type: all, manga, novels, lightnovels, oneshots, doujin, manhwa, manhua, bypopularity, favorite

### season

Seasonal anime charts

- **`myanimelist-pp-cli season archive`** - Index of every past season
- **`myanimelist-pp-cli season current`** - The season airing right now
- **`myanimelist-pp-cli season later`** - Upcoming seasons announced but not yet airing (the `upcoming` path 404s; `later` is the real route)
- **`myanimelist-pp-cli season list`** - Full seasonal chart for a year and season, grouped by format
- **`myanimelist-pp-cli season schedule`** - The weekly airing schedule, optionally shifted to a timezone the site understands

### sitemap

The robots-advertised sitemap, for explicit id enumeration

- **``** - Sitemap index listing every catalog shard (XML)
- **``** - One sitemap shard, e.g. anime-001, manga-001, character-001, people-001, news-001 (XML)

### studio

Animation studios and licensors

- **`myanimelist-pp-cli studio <id>`** - A studio's or licensor's page with its full production list


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`myanimelist-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`myanimelist-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`myanimelist-pp-cli learnings list`** - Inspect taught rows
- **`myanimelist-pp-cli learnings forget <query>`** - Undo a teach
- **`myanimelist-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`myanimelist-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`myanimelist-pp-cli teach-pattern`** - Install a query/resource template up front
- **`myanimelist-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MYANIMELIST_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `myanimelist-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
myanimelist-pp-cli anime list

# JSON for scripting and agents
myanimelist-pp-cli anime list --json
# Filter to specific fields
myanimelist-pp-cli anime list --json --select id,title,url

# Dry run — show the request without sending
myanimelist-pp-cli anime list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
myanimelist-pp-cli anime list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Local-only writes** - the binary never writes to MyAnimeList; `track` writes your local SQLite library and `export` writes a file
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `MYANIMELIST_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- 48 endpoint commands across 15 resources (`myanimelist-pp-cli <resource> <endpoint>`)
- 13 hand-written commands: `anime show`, `manga show`, `anime divisive`, `anime drop-risk`,
  `anime consistency`, `adaptation`, `drift`, `week`, `airing`, `watch-order`, `track`,
  `next`, `suggest`, plus the local `export`
- Run `myanimelist-pp-cli --help` or `myanimelist-pp-cli <resource> --help` for the live tree;
  `myanimelist-pp-cli which "<capability>" --json` maps a capability to a command

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
myanimelist-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `myanimelist-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/myanimelist-pp-cli/config.toml`; `--home`, `MYANIMELIST_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A command returns HTML instead of structured fields** — The page shape changed; run the same command with --json to see the raw capture, and update the parser fixtures in internal/malhtml.
- **Requests start returning 429 or a bot-check page** — Slow down: add --delay 2s or lower concurrency. The CLI already defaults to one request per second.
- **Cached data looks stale** — Run `myanimelist-pp-cli sync --resources anime` or pass --refresh to refetch a single record past its TTL.
- **week or franchise gap prints nothing** — Those commands read the local library; add titles first with `myanimelist-pp-cli track add <id> --status watching`.
- **drift reports no history** — drift needs at least two snapshots over time; run `myanimelist-pp-cli sync --resources anime` periodically or on a schedule.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pystardust/ani-cli**](https://github.com/pystardust/ani-cli) — Shell (13700 stars)
- [**sdaqo/anipy-cli**](https://github.com/sdaqo/anipy-cli) — Python (515 stars)
- [**Wraient/curd**](https://github.com/Wraient/curd) — Go (324 stars)
- [**ryukinix/mal**](https://github.com/ryukinix/mal) — Python (110 stars)
- [**brunolm/mal-mcp**](https://github.com/brunolm/mal-mcp) — TypeScript (1 stars)
- [**cyanheads/anime-mcp-server**](https://github.com/cyanheads/anime-mcp-server) — TypeScript (1 stars)
- [**AdityaJ7/myanimelist-cli**](https://github.com/AdityaJ7/myanimelist-cli) — C (1 stars)
- [**trackma**](https://github.com/z411/trackma) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
