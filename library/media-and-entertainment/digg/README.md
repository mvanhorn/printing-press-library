# Digg AI CLI

**Tail the Digg AI 1000's news cycle from the terminal — read-only, with the full pipeline event stream and rank-history nobody else surfaces.**

Digg AI is a curated leaderboard of 1,000 AI accounts on X and the story clusters they surface. The web UI shows you today's snapshot. This CLI tails the pipeline events, keeps a local rank-history that survives daily overwrites, and exposes Digg's own replacement rationale and gravity components so an agent can answer 'why this story?' and 'what got dropped overnight?' with structured data.

## Install

The recommended path installs both the `digg-pp-cli` binary and the `pp-digg` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install digg
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install digg --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/digg-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-digg --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-digg --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-digg skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-digg. The skill defines how its required CLI can be installed.
```

## Authentication

No auth required. The CLI uses only public surfaces — the /ai page (HTML+RSC scrape) and /api/trending/status (public JSON). It does not use Clerk session cookies or any authenticated endpoint, by design: this is a read-only research tool, and Digg's parent platform was shut down over AI-agent abuse. The CLI is read-only and identifies itself with a clear User-Agent so Digg ops can rate-limit it cleanly.

## Quick Start

```bash
# Pull the current /ai feed and /api/trending/status events into the local store
digg-pp-cli sync


# Read today's top 10 clusters as structured JSON
digg-pp-cli top --limit 10 --json


# See which stories climbed the rankings in the last hour with explicit rank deltas
digg-pp-cli events --since 1h --type fast_climb


# What got knocked out of the rankings overnight and Digg's own rationale for each
digg-pp-cli replaced --since 24h


# The Digg AI 1000's top influencers by Digg's score
digg-pp-cli authors top --by influence --limit 25

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Topic search and per-post citations
- **`search`** — Topic search across Digg's full window. Live by default — hits `/api/search/stories`, the same server-side search that backs the di.gg/ai Cmd+K modal — with FTS5 fallback to the local store on network error or `--data-source local`.

  _Returns ranked clusters with engagement metadata (postCount, uniqueAuthors, firstPostAge); the load-bearing recipe for last30days-style research workflows._

  ```bash
  digg-pp-cli search "<topic>" --since 30d --agent --select clusterUrlId,title,rank,postCount,uniqueAuthors,firstPostAge
  ```
  - **`--since Nh|Nd|Nw|Nm`** — filter to clusters first posted within the window (live mode parses Digg's own `firstPostAge`; local mode reads `digg_clusters.first_post_at`).
- **`posts`** — X posts attached to one cluster, with author rank, body when rendered, media URLs, repost-context, and minted xUrl for one-click citation.

  _The citations recipe: surface the highest-credibility AI 1000 voices on a story, sortable by rank, type, or time._

  ```bash
  digg-pp-cli posts <clusterUrlId> --by rank --limit 5 --agent --select author.username,author.rank,post_type,xUrl,body
  ```

### Author lookup and roster browse
- **`authors get`** — Look up any X handle in Digg's full author universe (1000 + off-1000) via `/api/search/users`. For off-1000 handles, the response includes `subject_peer_follow_count`, the rank-1000 anchor's `peer_follow_count`, and a signed `peer_follow_gap` — the gap to the 1000 measured in AI-1000 peer follows (NOT raw X follower count).

  _The credibility lookup: an agent can decide whether to quote a handle by reading one structured record._

  ```bash
  digg-pp-cli authors get <handle> --agent
  ```

  Trimmed off-1000 example for `mvanhorn`:
  ```json
  {
    "username": "mvanhorn",
    "current_rank": null,
    "subject_peer_follow_count": 19,
    "nearest_in_1000": {"rank": 1000, "username": "...", "peer_follow_count": 90},
    "peer_follow_gap": 71
  }
  ```
  `peer_follow_gap` is the gap to rank-1000's `followed_by_count` (peer follows from inside the AI 1000). Do not read it as a raw X follower delta.
- **`authors list`** — Full ranked AI 1000 from `/ai/1000`, persisted with rich fields (rank, category, bio, vibeDistribution, GitHub URL).

  _Identify rising voices in a category, find authors who just joined the 1000, see who's falling fast — sortable, filterable, scriptable._

  ```bash
  # Biggest movers since the last snapshot
  digg-pp-cli authors list --by rankChange --limit 20 --agent

  # Newly listed (first appearance in the 1000)
  digg-pp-cli authors list --only-new --agent
  ```
  Sort with `--by rank|rankChange|category|followers`; filter with `--category "<name>"`, `--only-new`, `--only-fallers`.

### Live pipeline observability
- **`events`** — Tail Digg's ingestion pipeline in real time — see clusters as they're detected, stories fast-climbing the leaderboard with explicit rank deltas, X posts being processed, batch breakdowns.

  _When an agent needs 'tell me when story X just climbed N ranks' or 'what new clusters did Digg detect in the last hour', this is the only way._

  ```bash
  digg-pp-cli events --since 1h --type fast_climb --json --select clusterId,label,delta,currentRank,previousRank
  ```
- **`watch`** — Poll /ai, diff against last snapshot, alert when any cluster moves N+ ranks.

  _Read-only operational watcher; never writes anything back to Digg._

  ```bash
  digg-pp-cli watch --alert 'rank.delta>=10'
  ```
- **`pipeline status`** — One-screen view of /api/trending/status: isFetching, nextFetchAt, storiesToday, clustersToday, last 5 events.

  _Lets ops and power users see when a fresh batch is about to land and what's been ingested in the last hour._

  ```bash
  digg-pp-cli pipeline status --watch
  ```

### Local state that compounds
- **`replaced`** — Show stories that were knocked out of the rankings since the last sync, with Digg's own published replacement rationale.

  _Best-of-feed shifts faster than people remember. This makes 'what did Digg drop and why' queryable._

  ```bash
  digg-pp-cli replaced --since 24h --json
  ```
- **`crossref`** — Show this cluster's Hacker News and Techmeme mirrors when Digg has detected the story is being discussed there.

  _Removes the manual 'is HN talking about this too' step from any cross-aggregator research workflow._

  ```bash
  digg-pp-cli crossref iq7usf9e
  ```
- **`authors top`** — Top accounts in the Digg AI 1000 ranked by Digg's influence score, story count, or reach.

  _Investors and AI scouts care which accounts move the news cycle. Now queryable, sortable, scriptable._

  ```bash
  digg-pp-cli authors top --by influence --limit 50 --json
  ```
- **`history`** — Full trajectory of one cluster's currentRank, peakRank, and delta over local snapshot history.

  _'Entered at #18, peaked at #4 over 6h, dropped to #22 by 24h' is impossible to learn from the live site._

  ```bash
  digg-pp-cli history iq7usf9e --json
  ```
- **`author`** — Every cluster a given X account contributed to, with post type (original, retweet, quote, reply).

  _'Show me every story this account surfaced this week' is the investor-scout query._

  ```bash
  digg-pp-cli author Scobleizer --since 7d --json
  ```

### Transparency
- **`evidence`** — Print the full ranking transparency record for one cluster — scoreComponents, evidence array, numeratorLabel, percentAboveAverage.

  _When a user asks 'why is THIS the top story', the answer is structured data; agents can compose with it._

  ```bash
  digg-pp-cli evidence iq7usf9e --json
  ```
- **`sentiment`** — Read per-time-window positivity ratios (pos6h, pos12h, pos24h, posLast) for a cluster.

  _Tells an agent whether the conversation around a story is still net-positive or has soured; useful before quoting a story._

  ```bash
  digg-pp-cli sentiment iq7usf9e --window 6h --json
  ```

## Usage

Run `digg-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Top-level AI story feed (HTML page; CLI parses the embedded RSC stream)

- **`digg-pp-cli feed raw`** - Fetch the raw /ai HTML page. The CLI's sync command parses this; most users should run `sync` then `top` instead of calling this directly.
- **`digg-pp-cli feed story_raw`** - Fetch the raw /ai/{clusterUrlId} story detail page (HTML). The CLI's `story` command parses this; users should not need to call this directly.

### search

Topic search across the full Digg window

- **`digg-pp-cli search "<query>"`** - Live by default (`/api/search/stories`); FTS5 fallback to the local store. Flags: `--since Nh|Nd|Nw|Nm`, `--data-source live|local|auto`, `--limit`.

### authors

Inspect the Digg AI 1000 leaderboard

- **`digg-pp-cli authors get <handle>`** - Look up any X handle (1000 + off-1000); off-1000 records include `subject_peer_follow_count`, the rank-1000 `nearest_in_1000` anchor, and `peer_follow_gap`. Flag: `--limit` (fuzzy fallback).
- **`digg-pp-cli authors list`** - Full ranked AI 1000 from `/ai/1000`, persisted with rich fields. Flags: `--by rank|rankChange|category|followers`, `--category`, `--only-new`, `--only-fallers`, `--limit`.
- **`digg-pp-cli authors top`** - Top contributors by influence, post count, or reach.

### posts

X posts attached to one cluster

- **`digg-pp-cli posts <clusterUrlId>`** - Origins, replies, quotes, retweets with author rank, body when rendered, media URLs, minted xUrl. Flags: `--by rank|type|time`, `--type tweet|reply|quote|retweet`, `--limit`, `--no-cache`.

### story

Full cluster detail. The envelope now includes `posts` and `postsMeta` fields populated by the U5 RSC parser.

### trending

Public ingestion-pipeline status and event stream

- **`digg-pp-cli trending status`** - Read the current pipeline status: storiesToday, clustersToday, isFetching, nextFetchAt, and the recent event stream (cluster_detected, fast_climb, post_understanding, batch_started, batch_breakdown, posts_stored, embedding_progress).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
digg-pp-cli feed raw

# JSON for scripting and agents
digg-pp-cli feed raw --json

# Filter to specific fields
digg-pp-cli feed raw --json --select id,name,status

# Dry run — show the request without sending
digg-pp-cli feed raw --dry-run

# Agent mode — JSON + compact + no prompts in one flag
digg-pp-cli feed raw --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-digg -g
```

Then invoke `/pp-digg <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add digg digg-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/digg-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "digg": {
      "command": "digg-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
digg-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/digg-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **All commands return empty after install** — Run `digg-pp-cli sync` first — the local store is empty until the first sync.
- **`events` shows no fast_climb / cluster_detected events** — The pipeline batches every ~10 minutes. Wait for `nextFetchAt` from `digg-pp-cli pipeline status` or filter by a different `--type`.
- **HTTP 429 on sync** — Adaptive limiter backs off automatically. If it persists, lower the polling rate with `--interval 120s` on `watch` commands.
- **Story command returns 'cluster not found'** — Use `clusterUrlId` (the 8-char alphanumeric short ID), not the UUID-style clusterId. `digg-pp-cli top --json --select clusterUrlId` lists them.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**haxor-news**](https://github.com/donnemartin/haxor-news) — Python (4000 stars)
- [**circumflex**](https://github.com/bensadeh/circumflex) — Go (1900 stars)
- [**rafaelrinaldi/hn-cli**](https://github.com/rafaelrinaldi/hn-cli) — JavaScript (700 stars)
- [**brianlovin/hn-cli**](https://github.com/brianlovin/hn-cli) — TypeScript (250 stars)
- [**heartleo/hn-cli**](https://github.com/heartleo/hn-cli) — Rust (100 stars)
- [**hntop-cli**](https://github.com/nilic/hntop-cli) — Go (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
