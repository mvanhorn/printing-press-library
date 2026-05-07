# Hacker News CLI

**Hacker News from your terminal — with a local SQLite store, snapshot history, and agent-native output no other HN tool has.**

Combines the Firebase real-time API and the Algolia search API in one CLI. Sync once and run cross-month hiring digests, topic pulses, repost lookups, and rank-over-time queries against a local store — offline, scriptable, MCP-ready. Every command supports --json and --select; the API itself is read-only, so this CLI is too.

Learn more at [Hacker News](https://news.ycombinator.com).

## Install

The recommended path installs both the `hackernews-pp-cli` binary and the `pp-hackernews` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install hackernews
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install hackernews --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hackernews-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hackernews --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hackernews --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-hackernews skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-hackernews. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication needed — both the Firebase and Algolia HN APIs are free and public.

## Quick Start

```bash
# Pull current top/new/best/show/ask/job lists plus recent items into the local store
hackernews-pp-cli sync

# Browse the freshest top stories
hackernews-pp-cli stories top --limit 10

# Track a topic's per-day mentions, score, and comment volume
hackernews-pp-cli pulse rust --days 7 --agent

# See exactly what changed on the front page since the last sync
hackernews-pp-cli since --json

# Aggregate the last 3 months of Who-is-Hiring — languages, remote ratio, top companies
hackernews-pp-cli hiring stats --months 3 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local snapshots that compound
- **`since`** — See exactly what climbed, fell, appeared, or dropped off the front page since your last sync.

  _Reach for this when an agent wakes up daily and needs to know what shifted on HN since yesterday — without re-fetching 500 items every poll._

  ```bash
  hackernews-pp-cli since --json
  ```
- **`controversial`** — Stories ranked by the highest comment-to-point ratio over a recent window — the discussions everyone is arguing about.

  _Reach for this when you want stories with high engagement-to-approval — heated debate signal — instead of just popularity._

  ```bash
  hackernews-pp-cli controversial --window 7d --json
  ```
- **`velocity`** — Show a story's rank trajectory over time from local snapshots — climb, plateau, or fall.

  _Reach for this when an agent asks 'is this story still gaining traction or already cresting' — only meaningful answer comes from snapshots._

  ```bash
  hackernews-pp-cli velocity 47998158 --json
  ```
- **`sync`** — Pull top/new/best/show/ask/job lists and recently-changed items into local SQLite for offline use and snapshot history.

  _Run this once per day (or per hour for agents) — it is the foundation that turns since/velocity/controversial/users-stats from impossible into one SQL query._

  ```bash
  hackernews-pp-cli sync --resources updates --agent
  ```

### Algolia leverage
- **`pulse`** — See per-day mentions, average score, and comment volume for any topic over the last N days.

  _Reach for this when the question is 'is this topic heating up or cooling down' rather than 'what's the top story right now'._

  ```bash
  hackernews-pp-cli pulse rust --days 7 --agent
  ```
- **`repost`** — Has this URL been posted before? Lists every prior submission, with score, comments, and date.

  _Reach for this before submitting a Show HN — duplicate URLs flame out instantly; you want to know how a prior post did first._

  ```bash
  hackernews-pp-cli repost https://example.com/article
  ```
- **`search local`** — Offline full-text search over every story and comment you have ever synced — corpus grows with use.

  _Reach for this when investigating long-tail topics or replaying last quarter's research — Algolia might rank it down or drop it; your local corpus will not._

  ```bash
  hackernews-pp-cli search local "vector database" --limit 20 --json
  ```

### Hiring-thread mining
- **`hiring stats`** — Aggregate the last N monthly Who-is-Hiring threads: top languages, remote ratio, top companies, location distribution.

  _Reach for this when you need quarterly or seasonal hiring trends — language popularity, remote-share shifts, location density — not just this month's listings._

  ```bash
  hackernews-pp-cli hiring stats --months 3 --agent
  ```
- **`hiring companies`** — Companies that posted in M of the last N hiring threads, with first-seen, last-seen, and months-posted count.

  _Reach for this when sourcing or trend-tracking — which companies are persistent hirers vs one-off posters — without scraping HNHIRING.com._

  ```bash
  hackernews-pp-cli hiring companies --months 6 --min-posts 3 --agent
  ```

### Cross-entity local queries
- **`users stats`** — Median and p90 score across a user's submissions, plus traction buckets and hour-of-day score distribution.

  _Reach for this before posting your own work to learn your traction patterns, or when sizing up a poster's history before engaging._

  ```bash
  hackernews-pp-cli users stats pg --json
  ```

## Usage

Run `hackernews-pp-cli --help` for the full command reference and flag list.

## Commands

### items

Fetch any HN item (story, comment, job, poll) by ID

- **`hackernews-pp-cli items get`** - Get details for a specific story, comment, job, or poll

### maxitem

Current maximum item ID

- **`hackernews-pp-cli maxitem get`** - Returns the largest item ID currently assigned by Hacker News

### stories

Browse top, new, and best Hacker News stories

- **`hackernews-pp-cli stories ask`** - Get the latest Ask HN posts
- **`hackernews-pp-cli stories best`** - Get the highest-voted stories on Hacker News
- **`hackernews-pp-cli stories job`** - Get the latest Hacker News job postings
- **`hackernews-pp-cli stories new`** - Get the newest stories on Hacker News
- **`hackernews-pp-cli stories show`** - Get the latest Show HN posts
- **`hackernews-pp-cli stories top`** - Get the current top stories on Hacker News

### updates

Recently changed items and profiles

- **`hackernews-pp-cli updates list`** - Items and user profiles that have changed recently

### users

Look up Hacker News user profiles

- **`hackernews-pp-cli users get`** - Get a user's profile including karma and submission history

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hackernews-pp-cli items 47998158

# JSON for scripting and agents
hackernews-pp-cli items 47998158 --json

# Filter to specific fields
hackernews-pp-cli items 47998158 --json --select id,name,status

# Dry run — show the request without sending
hackernews-pp-cli items 47998158 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hackernews-pp-cli items 47998158 --agent
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

## Cookbook

Recipes for everyday HN tracking, agent loops, and analysis pipelines. All commands are read-only.

### Daily front-page diff for an agent

```bash
hackernews-pp-cli sync
hackernews-pp-cli since --list topstories --json
```

Run `sync` once per day (or per hour), then `since` returns only stories that climbed, fell, appeared, or dropped off — no need to re-fetch 500 items.

### Track a topic over the last week

```bash
hackernews-pp-cli pulse rust --days 7 --agent
```

Per-day mention count, average score, and comment volume. `--agent` packages it as compact JSON for piping.

### Find heated debate, not just popular stories

```bash
hackernews-pp-cli controversial --window 7d --min-comments 100 --json
```

Stories with the highest comment-to-point ratio over the last 7 days, filtered to substantial discussions only.

### Has this URL been posted before?

```bash
hackernews-pp-cli repost https://example.com/article --json
hackernews-pp-cli repost https://example.com/article --include-comments --json
```

Lists every prior submission with score, comment count, and date. Add `--include-comments` to also catch URLs mentioned in comment threads.

### Track a story's rank trajectory

```bash
hackernews-pp-cli sync                       # populate snapshots over time
hackernews-pp-cli velocity 47998158 --json   # climb, plateau, or fall
```

Only meaningful after multiple syncs — velocity reads from local snapshot history.

### Aggregate the last 6 months of hiring threads

```bash
hackernews-pp-cli hiring stats --months 6 --json
hackernews-pp-cli hiring companies --months 6 --min-posts 3 --json
```

`stats` returns top languages, remote ratio, top companies. `companies` returns only companies that posted in 3+ of the scanned months — repeat-poster signal.

### Profile a user's posting timing

```bash
hackernews-pp-cli users stats dang --limit 200 --json
```

Median and p90 submission score, traction buckets, hour-of-day histogram, and best-hour UTC.

### Live Algolia search with filters

```bash
hackernews-pp-cli search "rust async" --json
hackernews-pp-cli search openai --tag story --since 7d --by-date
hackernews-pp-cli search "kubernetes" --min-points 100 --json
```

Algolia keeps everything since 2006. Use `--by-date` for chronological results, `--min-points` to filter to high-signal posts.

### Offline full-text search across everything synced

```bash
hackernews-pp-cli sync --full
hackernews-pp-cli search local "vector database" --type stories --json
```

After sync, `search local` runs FTS5 against your accumulated corpus — no network needed, and your search history grows with every sync.

### Stream JSON to jq for custom shaping

```bash
hackernews-pp-cli stories top --limit 50 --json | jq -r '.[] | select(.score > 200) | .url'
```

Combine `--json` output with jq filters to build pipelines no curl call to the raw API can match.

### Force live data over the local store for one command

```bash
hackernews-pp-cli stories top --data-source live --json
```

`auto` (the default) prefers the local SQLite store with bounded freshness; `live` always hits the API; `local` never refreshes. Set `HACKERNEWS_NO_AUTO_REFRESH=1` for the same effect across the session.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `HACKERNEWS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `hackernews-pp-cli stories`
- `hackernews-pp-cli stories ask`
- `hackernews-pp-cli stories best`
- `hackernews-pp-cli stories job`
- `hackernews-pp-cli stories new`
- `hackernews-pp-cli stories show`
- `hackernews-pp-cli stories top`
- `hackernews-pp-cli updates`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-hackernews -g
```

Then invoke `/pp-hackernews <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-mcp@latest
```

Then register it:

```bash
claude mcp add hackernews hackernews-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hackernews-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/cmd/hackernews-pp-mcp@latest
```

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

</details>

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
- **items thread <id> times out on huge threads** — Use --depth 2 to cap tree depth, or --flat for a linear view
- **since returns nothing** — Run sync at least twice with a delay; since needs at least two snapshots in the lists table

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**haxor-news**](https://github.com/donnemartin/haxor-news) — Python (3900 stars)
- [**circumflex**](https://github.com/bensadeh/circumflex) — Go (2300 stars)
- [**hacker-feeds-cli**](https://github.com/Mayandev/hacker-feeds-cli) — JavaScript (700 stars)
- [**hn-cli**](https://github.com/rafaelrinaldi/hn-cli) — JavaScript (400 stars)
- [**cyanheads/hacker-news-mcp-server**](https://github.com/cyanheads/hacker-news-mcp-server) — TypeScript
- [**GeorgeNance/hackernews-mcp**](https://github.com/GeorgeNance/hackernews-mcp) — TypeScript
- [**node-hn-api**](https://github.com/heychazza/node-hn-api) — TypeScript
- [**pyhn**](https://github.com/toxinu/pyhn) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
