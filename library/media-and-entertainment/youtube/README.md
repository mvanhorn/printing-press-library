# YouTube CLI

**A self-maintained competitor-monitoring machine for YouTube: the read surfaces that matter for market and competitor research plus a local databank of channel histories, snapshots, comments, and packaging assets - market data hours old, not weeks.**

The YouTube Data API v3 read surfaces that matter for market and competitor research with complete parameter wiring, feeding a local SQLite databank designed for competitor monitoring: `watch` your ~15 competitors, `monitor` refreshes them for ~20-40 quota units per run, and `velocity`, `growth`, `breakouts`, `comments-mine`, and `packaging` turn the accumulated snapshots into current market intelligence that lagging analytics platforms deliver one to two weeks late.

Learn more at [YouTube](https://www.youtube.com).

Created by [@justinwfu](https://github.com/justinwfu) (Justin).
Contributors: [@vieGPT](https://github.com/vieGPT) (Maximus).

## Install

The recommended path installs both the `youtube-pp-cli` binary and the `pp-youtube` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install youtube
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install youtube --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install youtube --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install youtube --agent claude-code
npx -y @mvanhorn/printing-press-library install youtube --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install youtube --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install youtube --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "youtube": {
      "command": "youtube-pp-mcp",
      "env": {
        "YOUTUBE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set YOUTUBE_API_KEY to a YouTube Data API v3 key (create one at console.cloud.google.com under APIs & Services > Credentials), or store it once with `auth set-token`. A key in the environment overrides the stored one - if `doctor` shows auth_source env and calls fail with HTTP 400 'API key not valid', the environment copy is stale: unset it or update it. Read-only public-data operations only; no OAuth anywhere.

## Quick Start

```bash
# Health check: key present, API reachable, cache status - works before any credential is set
youtube-pp-cli doctor --dry-run

# Resolve a competitor channel and see its lifetime totals
youtube-pp-cli youtube channels-list --part statistics,snippet --for-handle @mkbhd

# Put the channel on the watchlist (drop --dry-run to save); backfill seeds its full history
youtube-pp-cli watch add @mkbhd --dry-run

# Preview the refresh of every watched channel; run it on a schedule to keep the databank current
youtube-pp-cli monitor --dry-run

# Which tracked videos are gaining views fastest right now
youtube-pp-cli velocity --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Competitor monitoring machine
- **`watch`** — Register the competitor channels your monitoring machine tracks, in a typed watchlist table you own.

  _Defines the tracked market once; every later monitoring command runs against it without re-specifying channels._

  ```bash
  youtube-pp-cli watch add @mkbhd --json
  ```
- **`monitor`** — Refresh every watched channel in one run: stats snapshot, new uploads, re-snapshot of recent video statistics.

  _One command keeps the databank current, so market answers are hours old instead of weeks old._

  ```bash
  youtube-pp-cli monitor --json
  ```
- **`velocity`** — See which tracked videos are gaining views fastest right now, computed from real between-snapshot deltas.

  _Current market movement - what is taking off today, not what took off two weeks ago._

  ```bash
  youtube-pp-cli velocity --json
  ```
- **`growth`** — Channel-level subscriber, view, and upload-count deltas between dated local snapshots.

  _Tells an agent whether a competitor is accelerating without any external history service._

  ```bash
  youtube-pp-cli growth @mkbhd --json
  ```
- **`backfill`** — Pull a channel's complete upload history with statistics into the local databank in one command.

  _Run once per competitor; every later question about that channel is answered offline for free._

  ```bash
  youtube-pp-cli backfill @mkbhd --json
  ```
- **`workspace`** — Named databanks: keep the competitor machine in one database and explore a new niche in another, switching instantly.

  _Lets an agent spin up a clean research sandbox per niche without risking the production watchlist databank._

  ```bash
  youtube-pp-cli workspace list --json
  ```
- **`auth keys`** — Store multiple named YouTube API keys, switch between them instantly, and optionally fail over automatically via --rotate when one runs out of quota.

  _An agent can finish large collection jobs without human intervention when the first key's daily quota is spent._

  ```bash
  youtube-pp-cli auth keys list --json
  ```

### Fresh market discovery
- **`breakouts`** — Chain search filters into a matrix (terms x upload window x duration x region), join results to channel size, and rank fresh high-momentum videos.

  _Finds niche breakouts days after upload, weeks before they reach lagging analytics platforms._

  ```bash
  youtube-pp-cli breakouts "berlin history" --days 14 --json
  ```
- **`comments-mine`** — Sync comments into a typed full-text-searchable table and report top-liked comments, keyword frequencies, and audience questions.

  _Fast audience signal from data you own - what viewers praise, ask, and complain about across a channel._

  ```bash
  youtube-pp-cli comments-mine @mkbhd --json
  ```
- **`packaging`** — Collect titles, thumbnails (downloaded as local image files), and hook text from transcript openings into a packaging table.

  _Hands a multimodal agent everything it needs for thumbnail and hook analysis without any scraping or manual collection._

  ```bash
  youtube-pp-cli packaging @mkbhd --json
  ```

## Recipes

### Stand up the monitoring machine

```bash
youtube-pp-cli watch add @mkbhd --json
```

Add each competitor once; backfill seeds history and every monitor run keeps them current

### What moved today

```bash
youtube-pp-cli velocity --agent --select items.title,items.views_per_day
```

Between-snapshot view velocity for tracked videos, narrowed to the fields an agent needs

### Fresh breakouts in a niche

```bash
youtube-pp-cli breakouts "berlin history" --days 14 --json
```

Chained filter matrix joined to channel size - high views-per-subscriber uploads from the last two weeks

### Packaging dossier for the agent

```bash
youtube-pp-cli packaging @mkbhd --json
```

Titles, local thumbnail files, and hook text side by side, ready for multimodal packaging analysis

### What the audience keeps asking

```bash
youtube-pp-cli comments-mine @mkbhd --json
```

Top-liked comments, keyword frequencies, and extracted questions from the synced comment table

## Usage

Run `youtube-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `YOUTUBE_CONFIG_DIR`, `YOUTUBE_DATA_DIR`, `YOUTUBE_STATE_DIR`, or `YOUTUBE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `YOUTUBE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export YOUTUBE_HOME=/srv/youtube
youtube-pp-cli doctor
```

Under `YOUTUBE_HOME=/srv/youtube`, the four dirs resolve to `/srv/youtube/config`, `/srv/youtube/data`, `/srv/youtube/state`, and `/srv/youtube/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "youtube": {
      "command": "youtube-pp-mcp",
      "env": {
        "YOUTUBE_HOME": "/srv/youtube"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `YOUTUBE_DATA_DIR` overrides an explicit `--home` for that kind. Use `YOUTUBE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `YOUTUBE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `youtube-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### youtube

Manage youtube

- **`youtube-pp-cli youtube activities-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube captions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channel-sections-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube channels-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube comment-threads-list`** - Retrieves a list of top-level comment threads, filterable by video, channel, or thread id. Public read-only when filtered by videoId, channelId, or id; moderationStatus filtering is OAuth-only and not exposed here.
- **`youtube-pp-cli youtube i18n-languages-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube i18n-regions-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlist-items-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube playlists-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube search-list`** - Retrieves a list of search resources
- **`youtube-pp-cli youtube video-categories-list`** - Retrieves a list of resources, possibly filtered.
- **`youtube-pp-cli youtube videos-list`** - Retrieves a list of resources, possibly filtered.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`youtube-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`youtube-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`youtube-pp-cli learnings list`** - Inspect taught rows
- **`youtube-pp-cli learnings forget <query>`** - Undo a teach
- **`youtube-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`youtube-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`youtube-pp-cli teach-pattern`** - Install a query/resource template up front
- **`youtube-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `YOUTUBE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `youtube-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## The analyst databank (SQL schema)

Every analyst command writes into one local SQLite databank — the file `doctor --json` reports
as the store path. The filename is scoped by the active API key (`data-<hash>.db`); `workspace`
switches between entirely separate databank files. Agents query it two ways: the MCP `sql` tool
(read-only, validated) and the MCP `search` / CLI `search` full-text surface.

| Table | One row per | Key columns |
|---|---|---|
| `yt_watchlist` | tracked competitor channel | `channel_id`, `handle`, `title`, `note`, `added_at`, `last_monitored_at` |
| `yt_channel_snapshots` | channel per capture time | `channel_id`, `captured_at`, `subscriber_count`, `view_count`, `video_count` |
| `yt_videos` | known video (dimension table) | `video_id`, `channel_id`, `title`, `published_at`, `duration_seconds`, `is_short`, `description` |
| `yt_video_snapshots` | video per capture time | `video_id`, `captured_at`, `view_count`, `like_count`, `comment_count` |
| `yt_comments` | synced comment | `comment_id`, `video_id`, `channel_id`, `author`, `text`, `like_count`, `published_at`, `is_reply` |
| `yt_comments_fts` | FTS5 index over `yt_comments.text` | `MATCH` queries; kept in sync by insert/update/delete triggers |
| `yt_packaging` | collected packaging asset | `video_id`, `channel_id`, `title`, `thumb_url`, `thumb_path`, `hook_text`, `view_count`, `captured_at`, `hook_error`, `thumb_error` |
| `yt_monitor_runs` | one `monitor` run | `run_id`, `started_at`, `finished_at`, `channels`, `new_videos`, `video_snapshots`, `comments_synced`, `quota_units_est` |

`monitor`, `backfill`, `breakouts`, `comments-mine`, and `packaging` feed these tables
automatically (write-through); `velocity` and `growth` are computed from consecutive
`yt_video_snapshots` / `yt_channel_snapshots` rows — two runs on different days are the
minimum for a non-empty answer.

Example queries (all verified against a live-populated store):

```sql
-- What moved: views per video from the latest snapshots
SELECT s.video_id, v.title, s.view_count, s.captured_at
FROM yt_video_snapshots s JOIN yt_videos v USING(video_id)
ORDER BY s.captured_at DESC, s.view_count DESC LIMIT 20;

-- Audience signal: most-liked comments mentioning a topic (FTS5)
SELECT c.like_count, c.author, c.text
FROM yt_comments_fts f JOIN yt_comments c ON c.rowid = f.rowid
WHERE yt_comments_fts MATCH 'gemini' ORDER BY c.like_count DESC LIMIT 10;

-- Growth rate per tracked channel between first and last snapshot
SELECT channel_id,
       MAX(subscriber_count) - MIN(subscriber_count) AS subs_delta,
       MIN(captured_at) AS first_seen, MAX(captured_at) AS last_seen
FROM yt_channel_snapshots GROUP BY channel_id;
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-pp-cli youtube activities-list --part snippet

# JSON for scripting and agents
youtube-pp-cli youtube activities-list --part snippet --json
# Filter to specific fields
youtube-pp-cli youtube activities-list --part snippet --json --select contentDetails,etag,id

# Dry run — show the request without sending
youtube-pp-cli youtube activities-list --part snippet --dry-run

# Agent mode — JSON + compact + no prompts in one flag
youtube-pp-cli youtube activities-list --part snippet --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
youtube-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `youtube-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/youtube-pp-cli/config.toml`; `--home`, `YOUTUBE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `YOUTUBE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `youtube-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `youtube-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $YOUTUBE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 400 'API key not valid' although auth set-token succeeded** — An environment YOUTUBE_API_KEY is overriding the stored key - run doctor to check auth_source, then unset or update the env var
- **velocity or growth report 'need at least 2 snapshots'** — They compare dated snapshots - run `youtube-pp-cli monitor` again later (or schedule it); deltas appear from the second run onward
- **search or breakouts stop working while other commands still do** — search.list has its own 100-calls-per-day bucket (each page and each matrix cell costs one call) - narrow the filter matrix or wait for the daily reset
- **packaging rows missing thumbnails or hook text** — Thumbnail download needs network access and hook text needs an available transcript - rerun `packaging` for those videos; rows record which asset failed
- **quota exhausted mid-backfill or mid-sweep** — Add a second key with `auth keys add <name> <key>` and switch with `auth keys use <name>` (or pass --rotate on the long-running command) - quota is per key

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**youtube-data-cli**](https://github.com/Bin-Huang/youtube-data-cli) — JavaScript
- [**youtube-mcp-server**](https://github.com/pauling-ai/youtube-mcp-server) — Python
- [**youtube-data-mcp-server**](https://github.com/icraft2170/youtube-data-mcp-server) — TypeScript
- [**youtube-data-api**](https://github.com/SMAPPNYU/youtube-data-api) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
