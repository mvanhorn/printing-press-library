---
name: pp-yt-studio
description: "Every YouTube creator metric that matters, offline-queryable, with framework-audit binding to your scripts. The CLI... Trigger phrases: `audit this youtube video`, `what's the retention on this video`, `how did <video> retain its audience`, `compare my channel against my watchlist`, `what titles work on my channel`, `what should I cover next on youtube`, `framework audit my video`, `use yt-studio`, `run yt-studio-pp-cli`."
author: "Kamibot Upgrade Automation"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - yt-studio-pp-cli
    install:
      - kind: go
        bins: [yt-studio-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-cli
---

# YouTube Studio (Kami creator analytics) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `yt-studio-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install yt-studio --cli-only
   ```
2. Verify: `yt-studio-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

yt-studio-pp-cli is a hybrid YouTube Data API + Analytics API + Studio Innertube creator-side CLI. It caches channel videos, daily metrics, retention curves, demographics, and thumbnail CTR into a local SQLite. The killer command, framework-audit, joins retention drops to script Signal/Belief-Shift/Action-CTA lines so you have measurable evidence of which structural beat lost the audience.

## When to Use This CLI

Use yt-studio-pp-cli when an agent needs creator-side analytics: retention curves, per-video CTR, audience demographics, competitor benchmarks, or framework audits against published videos. This is the right tool for post-publish review, content-format ideation, and identifying which video structures earned (or lost) the audience. It is NOT the right tool for downloading videos, fetching transcripts for transcription, or publishing new content — those belong to yt-dlp, Whisper-based MCPs, and the publishing pipeline respectively.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`framework-audit`** — Cross-check whether a published video hits the Signal / Belief-Shift / Action-CTA structure by joining retention buckets against script lines from your local content directory.

  _When an agent reviews a published video, this is the only command that tells it which structural beat lost the audience._

  ```bash
  yt-studio-pp-cli framework-audit dQw4w9WgXcQ --script-dir ~/.openclaw/workspace/data --json
  ```

### Per-video signal extraction

- **`retention`** — Return the 100-bucket retention curve for a video as JSON or an ASCII sparkline, with the three sharpest drops auto-annotated.

  _Quickest way for an agent to identify which second of a video lost the audience._

  ```bash
  yt-studio-pp-cli retention dQw4w9WgXcQ --ascii
  ```
- **`retention-cohort`** — Average retention across videos whose title matches a regex (build-guide cohort vs rework cohort vs tier-list cohort).

  _Tells an agent whether a content format is structurally stronger than another._

  ```bash
  yt-studio-pp-cli retention-cohort --pattern "Rework" --days 30 --ascii
  ```
- **`ctr-decay`** — Compare first-72h CTR to day-30 CTR for a video; flags fast-decaying thumbnails.

  _Identifies which thumbnails earned their early lift and which are dying._

  ```bash
  yt-studio-pp-cli ctr-decay dQw4w9WgXcQ --json --compact
  ```

### Competitive context

- **`vs-watchlist`** — Compare own channel against the watchlist on CTR, retention, or upload cadence — normalized scale across all channels.

  _When an agent is planning what to publish next, this tells it which axis the channel is losing on._

  ```bash
  yt-studio-pp-cli vs-watchlist --metric ctr,retention,upload-cadence --period 28d
  ```
- **`title-patterns`** — Token-level analysis of words/phrases that correlate with above-median CTR (winners) or below-median (losers).

  _Drives an agent's title brainstorm with empirical evidence from this channel._

  ```bash
  yt-studio-pp-cli title-patterns --winners --losers --json
  ```
- **`idea-gap`** — Topics the watchlist covered in the last N days that the own channel has not.

  _Cheapest way for an agent to find what's underserved on this channel._

  ```bash
  yt-studio-pp-cli idea-gap --days 14 --json
  ```
- **`watchlist suggest`** — Auto-discover competitor channels for the watchlist via Search API; ranks by relevance and recent upload volume.

  _Saves an agent from manually curating a competitor list._

  ```bash
  yt-studio-pp-cli watchlist suggest --niche "poe2,last-epoch" --top 20 --json
  ```

### Agent-native plumbing

- **`sync`** — Incremental sync of own channel + watchlist with daily-quota budgeting; caps concurrency and prefers cache.

  _Prevents an agent from accidentally exhausting daily quota with one bad call._

  ```bash
  yt-studio-pp-cli sync --since 2026-05-01 --data-source auto --concurrency 2
  ```

## Command Reference

**captions** — Caption tracks (read-only; transcript fetch deferred)

- `yt-studio-pp-cli captions` — List caption tracks for a video

**categories** — YouTube video category metadata

- `yt-studio-pp-cli categories` — List video categories for a region

**channels** — YouTube channels (own channel + competitor watchlist)

- `yt-studio-pp-cli channels info` — Get the authenticated user's channel (--self) or a single channel by id
- `yt-studio-pp-cli channels list` — List channel resources by id, mine=true, or forHandle

**comments** — Video comment threads (read-only)

- `yt-studio-pp-cli comments` — List top-level comment threads for a video

**discover** — YouTube search.list API (100 quota units per request — use sparingly; named 'discover' because 'search' is a built-in framework command for offline FTS)

- `yt-studio-pp-cli discover` — Search for videos, channels, or playlists via the YouTube Data API search.list endpoint. WARNING: 100 quota units...

**playlistItems** — Items within a playlist

- `yt-studio-pp-cli playlistItems` — List items in a playlist

**playlists** — YouTube playlists

- `yt-studio-pp-cli playlists get` — Get a single playlist by ID
- `yt-studio-pp-cli playlists list` — List playlists by channelId or id

**videos** — YouTube videos (own + watchlist)

- `yt-studio-pp-cli videos get` — Get a single video by ID
- `yt-studio-pp-cli videos list` — List video metadata by id


**Hand-written commands**

- `yt-studio-pp-cli login` — Interactive: OAuth + Studio cookie capture (one-time setup)
- `yt-studio-pp-cli sync` — Incremental sync of own channel + watchlist into local SQLite (quota-aware)
- `yt-studio-pp-cli retention <video_id>` — Retention curve (JSON or ASCII sparkline) with 3 sharpest drops auto-annotated
- `yt-studio-pp-cli retention-cohort` — Composite retention across videos matching a title regex
- `yt-studio-pp-cli ctr-decay <video_id>` — First-72h CTR vs day-30 CTR; flags fast-decaying thumbnails
- `yt-studio-pp-cli vs-watchlist` — Compare own channel against watchlist on CTR / retention / upload cadence
- `yt-studio-pp-cli title-patterns` — Token-level analysis of words correlating with above-median CTR
- `yt-studio-pp-cli idea-gap` — Topics watchlist covered in last N days that own channel hasn't
- `yt-studio-pp-cli framework-audit <video_id>` — Join retention to script Signal/BeliefShift/CTA structure. The killer command.
- `yt-studio-pp-cli script-link <video_id> <script_path>` — Manually bind a script to a video for framework-audit
- `yt-studio-pp-cli watchlist` — Manage competitor watchlist (suggest / add / remove / list)
- `yt-studio-pp-cli quota` — Show today's Data API quota usage
- `yt-studio-pp-cli sniff-doctor` — Verify Studio Innertube response schema (drift detection)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
yt-studio-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Audit a specific upload

```bash
yt-studio-pp-cli framework-audit dQw4w9WgXcQ --json --select 'verdict,drops,signal_line,belief_shift_line,cta_line'
```

Audit a single video against the audience-obsession framework, returning only the verdict, drops, and the three structural lines.

### Find a cohort signal

```bash
yt-studio-pp-cli retention-cohort --pattern "Rework" --days 90 --json --select 'cohort_size,avg_retention_pct,worst_bucket'
```

Average retention across Rework-series videos in the last 90 days, returning only the cohort size, average retention percentage, and worst bucket.

### Title brainstorm with evidence

```bash
yt-studio-pp-cli title-patterns --winners --losers --json | jq '.winners[] | select(.lift_pct > 15)'
```

List title tokens with above-15% CTR lift; feeds an agent's title-generation prompt.

### Quota-safe morning sync

```bash
yt-studio-pp-cli sync --since 1d --data-source auto --concurrency 2 --quiet 2>&1 | grep 'quota_used'
```

Daily incremental, capped concurrency, with quota usage printed to stderr.

### Inspect Studio session health

```bash
yt-studio-pp-cli sniff-doctor --json
```

Probes the Studio Innertube session; returns exit 4 if the session has expired (re-run login).

## Auth Setup

One-time interactive login captures OAuth refresh tokens (Data API + Analytics API, scopes youtube.readonly + yt-analytics.readonly) and the Studio web session cookies. OAuth refresh is automatic on 401. Studio session expiry surfaces as typed exit 4 with a re-login hint. Tokens live in ~/.openclaw/state/yt-studio/ mode 600.

Run `yt-studio-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  yt-studio-pp-cli captions --video-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
yt-studio-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
yt-studio-pp-cli feedback --stdin < notes.txt
yt-studio-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.yt-studio-pp-cli/feedback.jsonl`. They are never POSTed unless `YT_STUDIO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YT_STUDIO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
yt-studio-pp-cli profile save briefing --json
yt-studio-pp-cli --profile briefing captions --video-id 550e8400-e29b-41d4-a716-446655440000
yt-studio-pp-cli profile list --json
yt-studio-pp-cli profile show briefing
yt-studio-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `yt-studio-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/cmd/yt-studio-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add yt-studio-pp-mcp -- yt-studio-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which yt-studio-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   yt-studio-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `yt-studio-pp-cli <command> --help`.
