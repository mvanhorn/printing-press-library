---
name: pp-youtube-creator-analytics
description: "Printing Press CLI for Youtube. Combined CLI for multiple API services"
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - youtube-creator-analytics-pp-cli
---

# Youtube — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `youtube-creator-analytics-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install youtube --cli-only
   ```
2. Verify: `youtube-creator-analytics-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Combined CLI for multiple API services

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local analytics that compound
- **`decay`** — Flag underperforming videos by views-per-day vs the channel baseline (--metric velocity, default), with a --days window and a noise warning at <30 videos.

  _Tells the creator which videos to re-promote or re-edit, before the algorithm forgets them._

  ```bash
  youtube-creator-analytics-pp-cli decay --metric velocity --days 90 --json
  ```
- **`retention-leaderboard`** — Top videos by average view percentage from Analytics API. Statistically noisy with <30 videos in the lookback window — treat as directional.

  _Ranks content by attention, not just views — the actual retention signal._

  ```bash
  youtube-creator-analytics-pp-cli retention-leaderboard --days 90 --json
  ```
- **`sub-velocity`** — Daily subscriber gain/loss rate from Analytics, day-bucketed.

  _Pinpoints which days a creator added or lost subs so launch-day pulls become visible._

  ```bash
  youtube-creator-analytics-pp-cli sub-velocity --days 28 --json
  ```
- **`posting-cadence`** — Correlate publish day-of-week or hour-of-day with average views.

  _Surfaces the channel's actual best posting window from observed data, not folklore._

  ```bash
  youtube-creator-analytics-pp-cli posting-cadence --mode dow --json
  ```
- **`ctr-cohort`** — Impressions and click-through-rate per video (Analytics API).

  _Tells the creator which thumbnails earn the click before they reroll the entire batch._

  ```bash
  youtube-creator-analytics-pp-cli ctr-cohort --days 90 --json
  ```

### Cross-channel intelligence
- **`competitor-diff`** — Compare uploads/cadence/avg views across cached channels (your channel + competitors).

  _One call answers 'how does my niche move?' without leaving the terminal._

  ```bash
  youtube-creator-analytics-pp-cli competitor-diff --json
  ```
- **`topic-cluster`** — Group videos into topic clusters by tag overlap and rank by avg views.

  _Reveals which themes pull views, not just which individual videos won._

  ```bash
  youtube-creator-analytics-pp-cli topic-cluster --limit 12 --json
  ```

### Transcript & comment mining
- **`transcript search`** — Full-text search across scraped transcripts (offline, FTS5).

  _Lets an agent grep an entire channel's spoken content offline, with no quota cost._

  ```bash
  youtube-creator-analytics-pp-cli transcript search 'sueño infantil' --json
  ```
- **`theme-mine`** — Extract most-repeated bigrams/trigrams from cached comments (FAQ surface area).

  _Shows what audience actually talks about so the creator picks topics with proven demand._

  ```bash
  youtube-creator-analytics-pp-cli theme-mine --n 2 --limit 30 --json
  ```
- **`comment-faq`** — Surface most-liked question-shaped comments across cached videos.

  _Generates a ready-to-answer Q&A pipeline straight from the audience._

  ```bash
  youtube-creator-analytics-pp-cli comment-faq --limit 20 --json
  ```

### Quota-aware ops
- **`sync-plan`** — Estimate Data v3 quota units needed for a full refresh + comment sync, with daily-quota headroom.

  _Prevents the surprise 'quotaExceeded' that wipes a creator's tooling for the day._

  ```bash
  youtube-creator-analytics-pp-cli sync-plan --comments-per-video 2 --json
  ```

## Command Reference

**group-items** — Manage group items

- `youtube-creator-analytics-pp-cli group-items delete` — Removes an item from a group.
- `youtube-creator-analytics-pp-cli group-items insert` — Creates a group item.
- `youtube-creator-analytics-pp-cli group-items list` — Returns a collection of group items that match the API request parameters.

**groups** — Manage groups

- `youtube-creator-analytics-pp-cli groups delete` — Deletes a group.
- `youtube-creator-analytics-pp-cli groups insert` — Creates a group.
- `youtube-creator-analytics-pp-cli groups list` — Returns a collection of groups that match the API request parameters. For example, you can retrieve all groups that...
- `youtube-creator-analytics-pp-cli groups update` — Modifies a group. For example, you could change a group's title.

**reports** — Manage reports

- `youtube-creator-analytics-pp-cli reports` — Retrieve your YouTube Analytics reports.

**youtube** — Manage youtube

- `youtube-creator-analytics-pp-cli youtube abuse-reports-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube activities-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube captions-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube captions-download` — Downloads a caption track.
- `youtube-creator-analytics-pp-cli youtube captions-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube captions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube captions-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube channel-banners-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube channel-sections-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube channel-sections-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube channel-sections-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube channel-sections-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube channels-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube channels-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube comment-threads-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube comment-threads-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube comments-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube comments-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube comments-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube comments-mark-as-spam` — Expresses the caller's opinion that one or more comments should be flagged as spam.
- `youtube-creator-analytics-pp-cli youtube comments-set-moderation-status` — Sets the moderation status of one or more comments.
- `youtube-creator-analytics-pp-cli youtube comments-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube i18n-languages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube i18n-regions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-bind` — Bind a broadcast to a stream.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-delete` — Delete a given broadcast.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-insert` — Inserts a new stream for the authenticated user.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-insert-cuepoint` — Insert cuepoints in a broadcast
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-list` — Retrieve the list of broadcasts associated with the given channel.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-transition` — Transition a broadcast to a given status.
- `youtube-creator-analytics-pp-cli youtube live-broadcasts-update` — Updates an existing broadcast for the authenticated user.
- `youtube-creator-analytics-pp-cli youtube live-chat-bans-delete` — Deletes a chat ban.
- `youtube-creator-analytics-pp-cli youtube live-chat-bans-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube live-chat-messages-delete` — Deletes a chat message.
- `youtube-creator-analytics-pp-cli youtube live-chat-messages-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube live-chat-messages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube live-chat-moderators-delete` — Deletes a chat moderator.
- `youtube-creator-analytics-pp-cli youtube live-chat-moderators-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube live-chat-moderators-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube live-streams-delete` — Deletes an existing stream for the authenticated user.
- `youtube-creator-analytics-pp-cli youtube live-streams-insert` — Inserts a new stream for the authenticated user.
- `youtube-creator-analytics-pp-cli youtube live-streams-list` — Retrieve the list of streams associated with the given channel. --
- `youtube-creator-analytics-pp-cli youtube live-streams-update` — Updates an existing stream for the authenticated user.
- `youtube-creator-analytics-pp-cli youtube members-list` — Retrieves a list of members that match the request criteria for a channel.
- `youtube-creator-analytics-pp-cli youtube memberships-levels-list` — Retrieves a list of all pricing levels offered by a creator to the fans.
- `youtube-creator-analytics-pp-cli youtube playlist-items-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube playlist-items-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube playlist-items-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube playlist-items-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube playlists-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube playlists-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube playlists-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube playlists-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube search-list` — Retrieves a list of search resources
- `youtube-creator-analytics-pp-cli youtube subscriptions-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube subscriptions-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube subscriptions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube super-chat-events-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube tests-insert` — POST method.
- `youtube-creator-analytics-pp-cli youtube third-party-links-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube third-party-links-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube third-party-links-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube third-party-links-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube thumbnails-set` — As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which...
- `youtube-creator-analytics-pp-cli youtube update-comment-threads` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube video-abuse-report-reasons-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube video-categories-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube videos-delete` — Deletes a resource.
- `youtube-creator-analytics-pp-cli youtube videos-get-rating` — Retrieves the ratings that the authorized user gave to a list of specified videos.
- `youtube-creator-analytics-pp-cli youtube videos-insert` — Inserts a new resource into this collection.
- `youtube-creator-analytics-pp-cli youtube videos-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-analytics-pp-cli youtube videos-rate` — Adds a like or dislike rating to a video or removes a rating from a video.
- `youtube-creator-analytics-pp-cli youtube videos-report-abuse` — Report abuse for a video.
- `youtube-creator-analytics-pp-cli youtube videos-update` — Updates an existing resource.
- `youtube-creator-analytics-pp-cli youtube watermarks-set` — Allows upload of watermark image and setting it for a channel.
- `youtube-creator-analytics-pp-cli youtube watermarks-unset` — Allows removal of channel watermark.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-creator-analytics-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `youtube-creator-analytics-pp-cli auth setup` for the URL and steps to register an OAuth app. Then either run the OAuth flow:

```bash
youtube-creator-analytics-pp-cli auth login --client-id <id> --client-secret <secret>
```

Or set `YOUTUBE_DATA_OAUTH2C` as an environment variable with an existing access token.

Run `youtube-creator-analytics-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  youtube-creator-analytics-pp-cli group-items list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
youtube-creator-analytics-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
youtube-creator-analytics-pp-cli feedback --stdin < notes.txt
youtube-creator-analytics-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.youtube-creator-analytics-pp-cli/feedback.jsonl`. They are never POSTed unless `YOUTUBE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YOUTUBE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
youtube-creator-analytics-pp-cli profile save briefing --json
youtube-creator-analytics-pp-cli --profile briefing group-items list
youtube-creator-analytics-pp-cli profile list --json
youtube-creator-analytics-pp-cli profile show briefing
youtube-creator-analytics-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `youtube-creator-analytics-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add youtube-creator-analytics-pp-mcp -- youtube-creator-analytics-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which youtube-creator-analytics-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   youtube-creator-analytics-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `youtube-creator-analytics-pp-cli <command> --help`.
