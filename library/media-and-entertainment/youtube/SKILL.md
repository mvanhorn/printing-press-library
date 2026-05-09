---
name: pp-youtube
description: "Every YouTube Data API v3 endpoint, plus quota-aware planning, plus offline transcripts (no quota), plus an... Trigger phrases: `search YouTube for`, `transcript of YouTube video`, `what did <channel> say about`, `what's trending on YouTube`, `channel digest for`, `use youtube`, `run youtube`."
author: "Beomgithb"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - youtube-pp-cli
---

# YouTube — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `youtube-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install youtube --cli-only
   ```
2. Verify: `youtube-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Existing tools split the surface — yt-dlp owns downloads, MCP servers own metadata reads, no one lets an operator do all three plus track quota plus query a cached corpus. youtube-pp-cli mirrors all 23 Data API resources, shells out to yt-dlp for transcripts, and ships a local SQLite store with FTS5 across video metadata, transcripts, and comments. Use `quota plan` before running expensive sweeps and `corpus search` to query everything you've already synced without touching the API.

## When to Use This CLI

Use this CLI when an agent or operator needs the full YouTube Data API surface plus offline analysis. It excels at multi-channel research sweeps, quota-aware bulk operations, transcript corpora for retrieval, and time-series questions Studio cannot answer (velocity, trending diffs, topic crossover). Reach for it any time a question requires joining videos, channels, transcripts, comments, or trending lists across time — those are the gaps every other YouTube tool leaves open.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Quota awareness
- **`quota plan`** — Dry-run any command sequence and see projected unit cost vs. your remaining 10K-unit daily budget before pressing enter.

  _Pick this when you need to know if a planned scrape will fit today's budget. Returns total units, per-endpoint breakdown, and remaining budget._

  ```bash
  youtube-pp-cli quota plan search.list --times 5 --include-ledger --json --agent
  ```
- **`cost ledger`** — Show the rolling per-API-key ledger of every command's quota spend, aggregated by command, endpoint, and day.

  _Use after a quota-exceeded surprise to find the offending command, or daily to budget across multiple agents sharing one key._

  ```bash
  youtube-pp-cli cost ledger --last 24h --json --agent
  ```

### Local-state advantage
- **`corpus search`** — FTS5 search across all synced video metadata, transcripts, and comments in one query — no quota cost.

  _Reach for this when an agent needs to find which videos discuss a topic across a synced corpus without burning search.list quota (100 units per call)._

  ```bash
  youtube-pp-cli corpus search 'inference latency' --since 30d --json --select hits.video_id,hits.channel,hits.snippet --agent
  ```
- **`digest`** — For a tracked channel, list new uploads since a timestamp joined with transcript-FTS hits for a keyword set.

  _Use this for weekly newsletter sweeps, agent research loops, or any 'what did this creator say about X this week' question._

  ```bash
  youtube-pp-cli digest UCBJycsmduvYEL83R_U4JriQ --since 7d --keywords inference,RAG,Sora --json --agent
  ```

### Trend tracking
- **`trending diff`** — Diff today's `mostPopular` snapshot against an earlier date and show entries that entered, exited, or moved.

  _Pick this to detect rising or falling trending entries between two dates — useful for content trend reports and sentiment shifts._

  ```bash
  youtube-pp-cli trending diff --region US --since 2026-04-01 --json --agent
  ```
- **`velocity`** — Compute Δviews, Δlikes, and Δcomments per day per video over a rolling window from your local snapshot history.

  _Use when you need 'which uploads from the last 30 days are accelerating or dying' — a question Studio shows piecemeal but agents can't extract._

  ```bash
  youtube-pp-cli velocity UCBJycsmduvYEL83R_U4JriQ --window 30d --json --agent
  ```
- **`topic crossover`** — Find videos in trending snapshots whose title, description, or transcript mention a keyword, ranked by trending position.

  _Use to spot when a niche topic crosses into mainstream trending lists — a signal for editorial timing._

  ```bash
  youtube-pp-cli topic crossover 'AI safety' --regions US,GB --json --select results.video_id,results.title,results.trending_rank --agent
  ```

### Subscription mirror
- **`subscriptions sweep`** — OAuth-gated: rebuild the subscription feed locally — list subscribed channels' new uploads since a timestamp in chronological order.

  _Pick this for an agent-readable replacement of the in-app subscription feed: filterable, sortable, replayable._

  ```bash
  youtube-pp-cli subscriptions sweep --since 7d --json --agent
  ```

## Command Reference

**youtube** — Manage youtube

- `youtube-pp-cli youtube abuse-reports-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube activities-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube captions-delete` — Deletes a resource.
- `youtube-pp-cli youtube captions-download` — Downloads a caption track.
- `youtube-pp-cli youtube captions-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube captions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube captions-update` — Updates an existing resource.
- `youtube-pp-cli youtube channel-banners-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube channel-sections-delete` — Deletes a resource.
- `youtube-pp-cli youtube channel-sections-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube channel-sections-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube channel-sections-update` — Updates an existing resource.
- `youtube-pp-cli youtube channels-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube channels-update` — Updates an existing resource.
- `youtube-pp-cli youtube comment-threads-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube comment-threads-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube comments-delete` — Deletes a resource.
- `youtube-pp-cli youtube comments-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube comments-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube comments-mark-as-spam` — Expresses the caller's opinion that one or more comments should be flagged as spam.
- `youtube-pp-cli youtube comments-set-moderation-status` — Sets the moderation status of one or more comments.
- `youtube-pp-cli youtube comments-update` — Updates an existing resource.
- `youtube-pp-cli youtube i18n-languages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube i18n-regions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-broadcasts-bind` — Bind a broadcast to a stream.
- `youtube-pp-cli youtube live-broadcasts-delete` — Delete a given broadcast.
- `youtube-pp-cli youtube live-broadcasts-insert` — Inserts a new stream for the authenticated user.
- `youtube-pp-cli youtube live-broadcasts-insert-cuepoint` — Insert cuepoints in a broadcast
- `youtube-pp-cli youtube live-broadcasts-list` — Retrieve the list of broadcasts associated with the given channel.
- `youtube-pp-cli youtube live-broadcasts-transition` — Transition a broadcast to a given status.
- `youtube-pp-cli youtube live-broadcasts-update` — Updates an existing broadcast for the authenticated user.
- `youtube-pp-cli youtube live-chat-bans-delete` — Deletes a chat ban.
- `youtube-pp-cli youtube live-chat-bans-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-messages-delete` — Deletes a chat message.
- `youtube-pp-cli youtube live-chat-messages-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-messages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-chat-moderators-delete` — Deletes a chat moderator.
- `youtube-pp-cli youtube live-chat-moderators-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube live-chat-moderators-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube live-streams-delete` — Deletes an existing stream for the authenticated user.
- `youtube-pp-cli youtube live-streams-insert` — Inserts a new stream for the authenticated user.
- `youtube-pp-cli youtube live-streams-list` — Retrieve the list of streams associated with the given channel. --
- `youtube-pp-cli youtube live-streams-update` — Updates an existing stream for the authenticated user.
- `youtube-pp-cli youtube members-list` — Retrieves a list of members that match the request criteria for a channel.
- `youtube-pp-cli youtube memberships-levels-list` — Retrieves a list of all pricing levels offered by a creator to the fans.
- `youtube-pp-cli youtube playlist-items-delete` — Deletes a resource.
- `youtube-pp-cli youtube playlist-items-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube playlist-items-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube playlist-items-update` — Updates an existing resource.
- `youtube-pp-cli youtube playlists-delete` — Deletes a resource.
- `youtube-pp-cli youtube playlists-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube playlists-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube playlists-update` — Updates an existing resource.
- `youtube-pp-cli youtube search-list` — Retrieves a list of search resources
- `youtube-pp-cli youtube subscriptions-delete` — Deletes a resource.
- `youtube-pp-cli youtube subscriptions-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube subscriptions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube super-chat-events-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube tests-insert` — POST method.
- `youtube-pp-cli youtube third-party-links-delete` — Deletes a resource.
- `youtube-pp-cli youtube third-party-links-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube third-party-links-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube third-party-links-update` — Updates an existing resource.
- `youtube-pp-cli youtube thumbnails-set` — As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which...
- `youtube-pp-cli youtube update-comment-threads` — Updates an existing resource.
- `youtube-pp-cli youtube video-abuse-report-reasons-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube video-categories-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube videos-delete` — Deletes a resource.
- `youtube-pp-cli youtube videos-get-rating` — Retrieves the ratings that the authorized user gave to a list of specified videos.
- `youtube-pp-cli youtube videos-insert` — Inserts a new resource into this collection.
- `youtube-pp-cli youtube videos-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube videos-rate` — Adds a like or dislike rating to a video or removes a rating from a video.
- `youtube-pp-cli youtube videos-report-abuse` — Report abuse for a video.
- `youtube-pp-cli youtube videos-update` — Updates an existing resource.
- `youtube-pp-cli youtube watermarks-set` — Allows upload of watermark image and setting it for a channel.
- `youtube-pp-cli youtube watermarks-unset` — Allows removal of channel watermark.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly research sweep with select narrowing

```bash
youtube-pp-cli digest UCBJycsmduvYEL83R_U4JriQ --since 7d --keywords inference,RAG --json --agent --select hits.video_id,hits.title,hits.transcript_match.snippet,hits.transcript_match.ts_offset
```

Joins last 7 days of uploads against transcript FTS for two keywords, then narrows the deeply-nested response to the four fields an agent actually needs — keeps payload under 4 KB even with 50 hits.

### Plan before you scrape

```bash
youtube-pp-cli quota plan search.list videos.list --include-ledger --json
```

Sums the projected unit cost of a search.list (100u) plus a videos.list batch (1u) against your remaining daily budget so you know if today's run will fit.

### Trending diff over a week

```bash
youtube-pp-cli trending diff --region US --since 2026-04-01 --json
```

Compares the latest trending_snapshots row against the snapshot taken on April 1st and emits entered/exited/moved buckets.

### Cross-corpus search agent path

```bash
youtube-pp-cli corpus search 'AI safety' --since 30d --agent --json --select hits.video_id,hits.channel,hits.snippet,hits.kind
```

FTS5 union across videos, transcripts, and comments — `hits.kind` tells the agent whether each match came from metadata, a transcript, or a comment, so it can weight relevance accordingly.

### Find rising videos in a channel

```bash
youtube-pp-cli velocity UCBJycsmduvYEL83R_U4JriQ --window 30d --order delta_views --json
```

Compares last 30 days of cached snapshots and ranks the channel's videos by view-velocity (views/day) so you can spot the breakouts.

## Auth Setup

Read commands need a YouTube Data API key (10K-unit/day quota by default). Set `YOUTUBE_API_KEY` or pass `--api-key`. Write commands (playlist edits, comments, ratings, uploads, channel section CRUD, subscriptions, captions, thumbnails) require OAuth2 — run `youtube-pp-cli auth login` to harvest a token into `~/.config/youtube-pp-cli/oauth.json`. Transcripts use yt-dlp and require neither.

Run `youtube-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  youtube-pp-cli youtube abuse-reports-insert --part example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
youtube-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
youtube-pp-cli feedback --stdin < notes.txt
youtube-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.youtube-pp-cli/feedback.jsonl`. They are never POSTed unless `YOUTUBE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YOUTUBE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
youtube-pp-cli profile save briefing --json
youtube-pp-cli --profile briefing youtube abuse-reports-insert --part example-value
youtube-pp-cli profile list --json
youtube-pp-cli profile show briefing
youtube-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `youtube-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add youtube-pp-mcp -- youtube-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which youtube-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   youtube-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `youtube-pp-cli <command> --help`.
