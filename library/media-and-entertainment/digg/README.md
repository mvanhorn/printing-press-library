# Digg CLI

**Search and read Digg from the terminal: live story search across all seven topics, the ranked story leaderboard with Digg Scores, GitHub feeds, and a local rank history. Read-only.**

Digg is a news leaderboard covering seven topics: ai, tech, world-business, sports, science, gaming and entertainment. It groups posts on X and web pages into stories and ranks each story with a Digg Score. This CLI searches those stories live, reads the ranked feed for one topic or all of them, returns the sources behind a story, keeps a local rank history that outlives Digg's own snapshots, and exposes Digg's GitHub feeds and account rankings as structured data.

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `digg-pp-cli` binary and the `pp-digg` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install digg
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install digg --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install digg --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install digg --agent claude-code
npx -y @mvanhorn/printing-press-library install digg --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/cmd/digg-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/digg-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install digg --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-digg --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-digg --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install digg --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

## Authentication

No auth required. The CLI uses only public surfaces: Digg's public search endpoint at https://digg.com/mcp (story search and story detail), the public story feed at /api/feed/continuous (JSON), and topic and rankings pages such as /ai and /tech/x/rankings (HTML). It does not use session cookies or any authenticated endpoint. The CLI is read-only, paces its requests, and backs off when Digg answers with a rate limit.

## Quick Start

```bash
# Search every topic for stories on a subject, live, with Digg Scores
digg-pp-cli search "world cup" --since 7d --sort score --json

# Limit a search to some topics
digg-pp-cli search "claude" --topic ai,tech --since 30d --agent

# The highest-scoring stories across every topic right now
digg-pp-cli top --limit 10 --json

# One topic's stories in rank order, or what is trending this week
digg-pp-cli top --topic sports --limit 10
digg-pp-cli top --topic ai --view trending --range 7d --json

# The sources behind one story: text, author, URL and time
digg-pp-cli posts https://digg.com/sports/65idu2x5 --agent

# Snapshot ranked lists on a schedule to build rank history
digg-pp-cli sync --topics ai,tech,sports

# Top repos by starring activity from Digg-tracked accounts
digg-pp-cli github stars --limit 10 --json

# Repos starred by two or more distinct Digg-tracked accounts
digg-pp-cli github stars --min-starrers 2 --json

# Full ranking of people and companies (initial-HTML slice)
digg-pp-cli rankings list --limit 20 --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Live search and story sources
- **`search`** — Search Digg stories with Digg's own search, live. It needs no prior `sync` and reads no local copy. Without `--topic` every topic is searched; `--topic` takes one or more of ai, tech, world-business, sports, science, gaming, entertainment, all.

  _Each result carries `clusterId` (the story's UUID), `clusterUrlId` (the short id from its URL), `title`, `tldr`, `description`, `url`, `topic`, `diggScore`, `sourceCount`, `postCount` (same number as `sourceCount`), `uniqueAuthors`, `publishedAt`, `firstSourceAt` and `firstPostAge`. `meta` carries `source: "live"`, `stale: false`, `totalMatches`, `window` and `sort`._

  ```bash
  digg-pp-cli search "<topic>" --since 30d --agent --select title,diggScore,sourceCount,url
  ```
  - **`--since`, `--until`** — a duration back from now (`12h`, `7d`, `2w`, `1m` for 30 days) or an ISO date (`2026-11-01`). They filter on `publishedAt`. Digg search reaches back 90 days.
  - **`--sort relevance|score|newest`** — `score` puts the highest Digg Score first.
  - **`--limit`** — may exceed one page of 50; search follows Digg's cursor.
  - A failed search exits non-zero. With `--json` the envelope still prints, with the reason in `meta.error`.
- **`posts`** — Every source Digg attributes one story to, in any topic. Each row has the source's text (`body`), URL (`xUrl`), time (`posted_at`) and `kind`: `x` for an X post (with `author.username` and `author.display_name`) or `webpage` (with `title`).

  ```bash
  digg-pp-cli posts <story> --limit 5 --agent --select author.username,xUrl,body
  ```
  `<story>` is the story's UUID, its short id, or its URL. A short id or URL resolves once `search` has returned that story; one `search` has not seen fails with a message that says so. `--by time` sorts oldest first. `--by rank`, `--by type` and `--type` are accepted and have no effect, because Digg no longer reports author rank or post type.
- **`story`** — One story as Digg publishes it: title, summary, topic, URL and dates, the earlier and later stories in the same thread, and its first 20 sources. Takes the same `<story>` argument as `posts`.
- **`open`** — Print the story's digg.com URL under its own topic; `--launch` opens it in a browser.

### Ranked stories, live
- **`top`** — Digg's ranked story feed, read live with no sync. `--topic` returns one topic in rank order. Without it, `top` reads the first page (20 stories) of every topic and merges them by Digg Score.

  _Output is a `{meta, results}` envelope. Each row has `rank` (within its own topic), `diggScore`, `postCount`, `uniqueAuthors`, `publishedAt`, `topic`, `url`, `title`, `description`, `clusterId` and `clusterUrlId`._

  ```bash
  digg-pp-cli top --topic ai --view trending --range 7d --json --select results.title,results.diggScore,results.url
  ```
  `--view top|trending`, `--range 24h|7d|30d|all`, `--limit`.

### Local state that compounds
- **`sync`** — Take one snapshot of each topic's ranked list (`--topics`, default `ai`) and save it to a local SQLite database. `history`, `watch`, `stats`, `author` and `authors top` read these snapshots. `search` and `top` do not need it.
- **`history`** — Full trajectory of one cluster's currentRank, peakRank, and delta over local snapshot history.

  _'Entered at #18, peaked at #4 over 6h, dropped to #22 by 24h' is impossible to learn from the live site._

  ```bash
  digg-pp-cli history iq7usf9e --json
  ```
- **`watch`** — Poll one Digg topic (`--topic`, default `ai`), diff against the last snapshot, and alert when any cluster moves N+ ranks.

  _Read-only operational watcher; never writes anything back to Digg._

  ```bash
  digg-pp-cli watch --topic sports --alert 'rank.delta>=10'
  ```
- **`authors top`** — Top accounts in the local store, ranked by Digg's influence score, story count, or reach.

  ```bash
  digg-pp-cli authors top --by influence --limit 50 --json
  ```
- **`author`** — Every synced cluster a given X account contributed to.

  ```bash
  digg-pp-cli author Scobleizer --since 7d --json
  ```

### Account roster
- **`authors list`** — Digg's ranked roster of accounts on X, fetched from its rankings page and saved locally with rank, category, bio, vibeDistribution and GitHub URL.

  ```bash
  # Biggest movers since the last snapshot
  digg-pp-cli authors list --by rankChange --limit 20 --agent

  # Newly listed accounts
  digg-pp-cli authors list --only-new --agent
  ```
  Sort with `--by rank|rankChange|category|followers`; filter with `--category "<name>"`, `--only-new`, `--only-fallers`.
- **`authors get`** — Look up any X handle through `/api/search/users`, ranked or not. A handle with no rank comes back with `current_rank: null` and `tier_status: "off_1000"`, plus `nearest_in_1000`: the account at rank 1000 with its `peer_follow_count`, for comparison.

  ```bash
  digg-pp-cli authors get <handle> --agent
  ```

## Usage

Run `digg-pp-cli --help` for the full command reference and flag list.

## Commands

### feed

Raw Digg pages, as HTML.

- **`digg-pp-cli feed raw`** - Fetch the raw HTML of Digg's AI topic page (/ai). Most users should run `top` instead.
- **`digg-pp-cli feed story_raw`** - Fetch the raw HTML of one story's page under /ai. Most users should run `story` or `posts` instead.

### github

GitHub feeds Digg surfaces alongside the X-account leaderboard. Four flavors, each parsed from the embedded RSC stream.

- **`digg-pp-cli github stars`** - Top repos ranked by starring activity from Digg-tracked accounts. Returns repo name, language, stargazers_count, recent starrer list, breakout_score, novel_score, ai_related_score, and the model's one-sentence classification. Flag: `--min-starrers N` filters to repos starred by >= N distinct accounts.
- **`digg-pp-cli github new`** - Recently first-seen repos with the Digg-tracked creator/starrer who first put them on Digg's radar (event_id, event_created_at, repo_full_name, creator).
- **`digg-pp-cli github activity`** - Top GitHub contributor leaderboard: per-author rank, contribution count, and distinct repos.
- **`digg-pp-cli github recent`** - Live activity feed: per-event entries with the GitHub URL and the user who acted.

### rankings

Digg's ranking of people and companies on X (`/tech/x/rankings`).

- **`digg-pp-cli rankings list`** - Full ranking of people and companies. Server-paginated; returns the initial-HTML slice. `--limit` caps. `--max-skip-ratio` (default 0.10) makes the command exit non-zero when too many rows fail to decode.

### search

Search Digg stories live, across one or more topics.

- **`digg-pp-cli search "<query>"`** - Calls Digg's search. Flags: `--topic`, `--since`, `--until`, `--sort relevance|score|newest`, `--limit`.

### top

Digg's ranked stories right now.

- **`digg-pp-cli top`** - Flags: `--topic`, `--view top|trending`, `--range 24h|7d|30d|all`, `--limit`.

### posts, story, open

One story, in any topic. Each takes the story's UUID, short id or URL.

- **`digg-pp-cli posts <story>`** - The story's sources: text, author, URL and time. Flags: `--by time`, `--limit`.
- **`digg-pp-cli story <story>`** - The story's detail, thread links and first 20 sources.
- **`digg-pp-cli open <story>`** - Print the story's URL; `--launch` opens it.

### sync, history, watch, stats

Local rank history.

- **`digg-pp-cli sync`** - Snapshot each topic's ranked list. Flags: `--topics`, `--db`.
- **`digg-pp-cli history <clusterUrlId>`** - One cluster's rank trajectory across snapshots.
- **`digg-pp-cli watch`** - Poll one topic and alert on rank moves. Flags: `--topic`, `--interval`, `--min-delta`, `--alert`, `--iterations`.
- **`digg-pp-cli stats`** - Cluster count, author count and snapshot depth of the local store.

### authors

Inspect the accounts Digg tracks.

- **`digg-pp-cli authors get <handle>`** - Look up any X handle; an unranked handle includes the rank-1000 `nearest_in_1000` anchor. Flag: `--limit` (fuzzy fallback).
- **`digg-pp-cli authors list`** - Full ranked roster from Digg's rankings page, saved with rich fields. Flags: `--by rank|rankChange|category|followers`, `--category`, `--only-new`, `--only-fallers`, `--limit`.
- **`digg-pp-cli authors top`** - Top contributors in the local store by influence, post count, or reach.
- **`digg-pp-cli authors overlap <userA> <userB>`** - Synced clusters both accounts contributed to.

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
- **Local history** - `sync` snapshots feed `history`, `watch`, `stats` and `sql` from a local SQLite store
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
digg-pp-cli doctor
```

Verifies configuration and reports two connectivity checks: `api` (digg.com answers) and `search` (Digg's search endpoint at https://digg.com/mcp answers a one-result search). Each reads `reachable` or `unreachable: <reason>`. `--fail-on error` makes an unreachable check exit non-zero.

## Configuration

Config file: `~/.config/digg-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`history`, `watch`, `stats`, `author` or `authors top` say there is no data** — These read local snapshots. Run `digg-pp-cli sync` first. `search`, `top`, `posts` and `story` read Digg live and need no sync.
- **`posts`, `story` or `open` says it does not know a short id** — Run `digg-pp-cli search` for the story first, or pass the story's UUID (`clusterId` in search results).
- **`search` fails** — Run `digg-pp-cli doctor` and read the `search` line. A failed search exits non-zero; it never prints an empty result as success.
- **A command from an older version is missing** — Digg stopped serving the data behind several commands, and they were removed instead of returning empty results. Run `digg-pp-cli --help` for the current list. `top --view trending` shows which stories are climbing.
- **HTTP 429** — The CLI backs off on its own. If the limit persists the command exits with code 7; lower the polling rate with `--interval 120s` on `watch`.

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
