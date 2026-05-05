# Hacker News CLI

**Hacker News from your terminal — with a local store, full-text search, and agent-native output no other HN tool has.**

Combines the Firebase real-time API and the Algolia search API in one CLI. Sync once and run searches, diffs, and topic pulses against a local SQLite store — offline, scriptable, and agent-friendly. Every command supports --json and --select; mutations don't apply because Hacker News is read-only.

Learn more at [Hacker News](https://news.ycombinator.com).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hackernews-current).

## Authentication

No authentication needed — both the Firebase and Algolia APIs are free and public.

## Quick Start

```bash
# Pull current top/best/new stories into the local store
hackernews-pp-cli sync


# Browse the freshest top stories
hackernews-pp-cli stories top --limit 10


# See HN's recent take on a topic, computed locally
hackernews-pp-cli pulse rust --days 7 --agent


# Diff against last sync — only what changed
hackernews-pp-cli since --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — Show what changed on the front page since last check — stories that appeared, disappeared, or moved.

  _Agents tracking HN signal need delta-mode, not full re-fetch._

  ```bash
  hackernews-pp-cli since --json
  ```
- **`controversial`** — Find stories with the highest comment-to-point ratio — the polarizing discussions.

  _Surfaces dissent, not just consensus, which the homepage hides._

  ```bash
  hackernews-pp-cli controversial --limit 10 --json
  ```
- **`velocity`** — Show a story's rank trajectory from local snapshots (climb, fall, stalled).

  _Agents asking 'is this gaining traction' get a trend, not a moment-in-time score._

  ```bash
  hackernews-pp-cli velocity 12345678 --json
  ```
- **`local-search`** — Offline FTS5 search across every story and comment you've touched.

  _Agents replaying past investigations don't re-hit Algolia._

  ```bash
  hackernews-pp-cli local-search "open source ai" --select title,url,score
  ```
- **`sync`** — Pull top/best/new lists into local SQLite for offline use and snapshot history.

  _First run makes the rest cheap; agents call once and read locally._

  ```bash
  hackernews-pp-cli sync --full
  ```

### Compound queries
- **`pulse`** — What HN is saying about a topic this week — score, comment, frequency by day.

  _One call replaces N Algolia paginations and the math an agent would otherwise do._

  ```bash
  hackernews-pp-cli pulse rust --days 7 --agent
  ```
- **`my`** — Track a user's submissions with score buckets, traction rate, and best posting time hints.

  _Replaces manual per-id fetches when an agent profiles a contributor._

  ```bash
  hackernews-pp-cli my pg --agent
  ```
- **`hiring-stats`** — Aggregate Who's Hiring across recent months: languages, remote ratio, top companies.

  _Agents matching jobs to a profile get the breakdown without scraping the threads themselves._

  ```bash
  hackernews-pp-cli hiring-stats --months 3 --agent --select languages
  ```
- **`repost`** — Has this URL been posted on HN before? Lists prior submissions with scores and dates.

  _Pre-flight check before posting; avoids dupe submissions._

  ```bash
  hackernews-pp-cli repost https://example.com/article
  ```

### Agent-native plumbing
- **`tldr`** — Deterministic thread digest: top authors by reply count, root vs reply ratio, comment heat metric.

  _Agents skimming a 500-comment thread get measurable signals, not opinion._

  ```bash
  hackernews-pp-cli tldr 12345678 --agent
  ```

## Usage

Run `hackernews-pp-cli --help` for the full command reference and flag list.

## Commands

### Story lists

- **`hackernews-pp-cli stories top [--limit N]`** - Current top stories
- **`hackernews-pp-cli stories new [--limit N]`** - Newest stories
- **`hackernews-pp-cli stories best [--limit N]`** - Highest-voted stories
- **`hackernews-pp-cli stories get <id>`** - Item details (story, comment, job, or poll)
- **`hackernews-pp-cli ask [--limit N]`** - Latest Ask HN posts
- **`hackernews-pp-cli show [--limit N]`** - Latest Show HN posts
- **`hackernews-pp-cli jobs [--limit N]`** - Latest job postings
- **`hackernews-pp-cli updates`** - Recently changed items and profiles
- **`hackernews-pp-cli maxitem`** - Largest item ID currently assigned
- **`hackernews-pp-cli users <username>`** - User profile (karma, about, submission history)

### Search

- **`hackernews-pp-cli search <query>`** - FTS5 against the local store, with live fallback in `auto` mode
- **`hackernews-pp-cli live-search <query>`** - Algolia live search (relevance or `--by-date`)
- **`hackernews-pp-cli local-search <query>`** - Offline-only FTS5 search
- **`hackernews-pp-cli comments <id>`** - Comment tree via Algolia's `/items` endpoint

### Hand-built

- **`hackernews-pp-cli sync [--full]`** - Pull top/best/new lists into the local SQLite store
- **`hackernews-pp-cli since [--list top|best|new]`** - Diff the front page against the last snapshot
- **`hackernews-pp-cli pulse <topic> [--days N]`** - Per-day score and comment volume for a topic
- **`hackernews-pp-cli my <username> [--limit N]`** - Submission history with score buckets and best posting hour
- **`hackernews-pp-cli hiring [regex]`** - Filter the latest Who's Hiring thread
- **`hackernews-pp-cli freelance [regex]`** - Filter the latest Freelancer thread
- **`hackernews-pp-cli hiring-stats [--months N]`** - Cross-month aggregate: languages, remote ratio, top companies
- **`hackernews-pp-cli controversial [--limit N]`** - Top stories by comment-to-point ratio
- **`hackernews-pp-cli velocity <id>`** - Rank trajectory across local snapshots
- **`hackernews-pp-cli repost <url>`** - Has this URL been posted on HN before?
- **`hackernews-pp-cli tldr <id>`** - Deterministic thread digest (top authors, depth histogram, heat metric)
- **`hackernews-pp-cli open <id> [--launch] [--hn]`** - Print or launch a story URL or HN thread
- **`hackernews-pp-cli bookmark add|list|rm`** - Local-only bookmarks
- **`hackernews-pp-cli doctor`** - Self-diagnostic (config, API reachability, store)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hackernews-pp-cli ask

# JSON for scripting and agents
hackernews-pp-cli ask --json

# Filter to specific fields (HN items use id, title, by, score, url, time, descendants)
hackernews-pp-cli stories top --json --select id,title,url,score

# Dry run — show the request without sending
hackernews-pp-cli ask --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hackernews-pp-cli ask --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `HACKERNEWS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `hackernews-pp-cli ask`
- `hackernews-pp-cli jobs`
- `hackernews-pp-cli show`
- `hackernews-pp-cli stories`
- `hackernews-pp-cli stories top`
- `hackernews-pp-cli stories new`
- `hackernews-pp-cli stories best`
- `hackernews-pp-cli stories get <id>`
- `hackernews-pp-cli updates`
- `hackernews-pp-cli search <query>`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add hackernews hackernews-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hackernews": {
      "command": "hackernews-pp-mcp"
    }
  }
}
```

## Health Check

```bash
hackernews-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/hackernews-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty results after sync** — Confirm both APIs respond: hackernews-pp-cli doctor
- **search returns no recent items** — Algolia indexes lag a few minutes; use stories top for the freshest list
- **comments command timing out on huge threads** — Use --depth 2 to cap the tree, or --flat for a linear view

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**circumflex**](https://github.com/bensadeh/circumflex) — Go
- [**haxor-news**](https://github.com/donnemartin/haxor-news) — Python
- [**hnterminal**](https://github.com/poseidon-code/hnterminal) — Python
- [**mcp-hacker-news**](https://github.com/erithwik/mcp-hn) — TypeScript
- [**hackernews-mcp**](https://github.com/punkpeye/hackernews-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
