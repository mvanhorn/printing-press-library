---
name: pp-youtube-creator
description: "Every YouTube Data, Analytics, and Reporting API endpoint, plus offline sync, the moderation verbs Studio hides, and... Trigger phrases: `moderate my youtube comments`, `youtube analytics digest`, `bulk update youtube video metadata`, `youtube channel backup`, `youtube ab thumbnail test`, `use youtube`, `run youtube`."
author: "JimPresting"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - youtube-creator-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/youtube-creator/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# YouTube — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `youtube-creator-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install youtube --cli-only
   ```
2. Verify: `youtube-creator-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator/cmd/youtube-creator-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps the three official Google YouTube APIs (Data v3, Analytics v2, Reporting v1) as one CLI with sub-modules, adds the high-frequency operations creators actually need (held-comment moderation queue, daily analytics digest, bulk metadata, PubSubHubbub upload triggers, A/B thumbnail testing, channel backup via yt-dlp), and persists state in a local SQLite with FTS5 search. Designed for n8n Execute Command nodes: every command emits JSON on stdout with typed exit codes.

## When to Use This CLI

Use youtube-creator-pp-cli when you need to automate YouTube channel management from a server, cron job, or n8n workflow — especially for the moderation queue, daily analytics, and bulk metadata operations that Studio either makes tedious or hides entirely. Best when your YouTube identity is OAuth-authenticated and you want one binary covering all three official APIs with a single auth store. Skip it if you're a casual viewer or only need to download videos (yt-dlp alone is enough for that).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Moderation as a verb
- **`mod queue`** — Pull every comment YouTube held for review, batch-approve or reject with optional author-ban in one verb.

  _When automating creator workflows, this is the highest-frequency moderation task and the single endpoint Studio's UX hides behind multiple panes._

  ```bash
  youtube-creator-pp-cli mod queue --since 7d --apply approve --ban-author
  ```
- **`mod auto`** — Apply YAML-defined regex/keyword rules (and optional LLM classify hook) to incoming or held comments.

  _Lets an agent enforce site-specific moderation policy without manual queue review._

  ```bash
  youtube-creator-pp-cli mod auto --rules rules.yaml --since 1h --apply
  ```

### Daily creator rituals
- **`digest analytics`** — Pre-baked Analytics API queries packaged as a Markdown report (top videos, CTR, watch time, traffic sources, revenue, Shorts vs long-form).

  _Drop into n8n Schedule Trigger → digest analytics → Send Email. One command replaces a hand-built dashboard._

  ```bash
  youtube-creator-pp-cli digest analytics --since 7d --markdown
  ```
- **`digest video`** — Single-video performance digest: retention curve, traffic sources, demographics, devices, end-screen impressions.

  _When a new upload spikes or flops, this is the first question creators ask._

  ```bash
  youtube-creator-pp-cli digest video dQw4w9WgXcQ --markdown
  ```

### Bulk operations
- **`bulk metadata`** — Filter the full video catalog by jq-style predicate, apply set/append/replace mutations, with dry-run diff.

  _Re-tag or footer-stamp an entire back catalog without manually paging through Studio._

  ```bash
  youtube-creator-pp-cli bulk metadata --category 22 --append-description footer.md --dry-run
  ```
- **`playlist hygiene`** — Auto-add new uploads to playlists by tag/title pattern; reorder existing playlists by view count.

  _Keeps playlists curated without Studio drag-and-drop._

  ```bash
  youtube-creator-pp-cli playlist hygiene --rules rules.yaml --apply
  ```

### Reachability and triggers
- **`pubsub subscribe`** — Subscribe a webhook callback to a channel's upload feed via WebSub; helper command verifies and prints the GET challenge response.

  _Switches uploads from polling (quota-burning, slow) to push-trigger in n8n. Real-time competitor-monitoring and own-upload pipelines._

  ```bash
  youtube-creator-pp-cli pubsub subscribe --channel UCxxx --callback https://n8n.example.com/webhook/yt --hub https://pubsubhubbub.appspot.com/
  ```

### Quota-aware operation
- **`quota meter`** — Print remaining quota estimate based on local-stored per-call cost log, and the cost of the next planned command.

  _Prevents the n8n-blow-the-daily-quota-in-one-execution disaster. Every other command honors --show-quota to print cost before execution._

  ```bash
  youtube-creator-pp-cli quota meter --json
  ```

### Archive and recovery
- **`backup`** — Wraps yt-dlp with cookies-from-browser to archive own videos, captions, thumbnails, info-json to local or S3-compatible target.

  _Weekly cron via n8n → never lose your channel._

  ```bash
  youtube-creator-pp-cli backup --since 30d --captions --thumbnails --info-json --out ./archive/
  ```

### Growth automation
- **`ab thumbnails`** — Schedule thumbnail rotation via thumbnails.set, track CTR via Analytics API, compute statistical significance.

  _Existing third-party tools charge SaaS pricing for this. CLI replaces them._

  ```bash
  youtube-creator-pp-cli ab thumbnails start --video dQw4w9WgXcQ --variants a.png,b.png --rotate 24h
  ```
- **`chapters auto`** — Pull captions via yt-dlp, run an LLM provider to propose chapter timestamps, write back to description via videos.update.

  _Saves 10-15 minutes of manual chapter authoring per video._

  ```bash
  youtube-creator-pp-cli chapters auto dQw4w9WgXcQ --provider claude --apply
  ```

### Warehouse-style ingestion
- **`reporting sync`** — Enumerate report types, ensure jobs exist, poll for completed reports, download CSV via redirect.

  _Drop-in for a daily warehouse pull when Analytics API queries are too expensive at scale._

  ```bash
  youtube-creator-pp-cli reporting sync --types content_owner_a1,channel_basic_a2 --since 30d --out ./reports/
  ```

### n8n integration
- **`recipes n8n`** — Prints ready-to-paste n8n workflow JSON snippets for the six highest-value automations.

  _Closes the loop on n8n integration — the user-stated goal._

  ```bash
  youtube-creator-pp-cli recipes n8n print held-comment-mod
  ```

## Command Reference

**group-items** — Manage group items

- `youtube-creator-pp-cli group-items delete` — Removes an item from a group.
- `youtube-creator-pp-cli group-items insert` — Creates a group item.
- `youtube-creator-pp-cli group-items list` — Returns a collection of group items that match the API request parameters.

**groups** — Manage groups

- `youtube-creator-pp-cli groups delete` — Deletes a group.
- `youtube-creator-pp-cli groups insert` — Creates a group.
- `youtube-creator-pp-cli groups list` — Returns a collection of groups that match the API request parameters. For example, you can retrieve all groups that...
- `youtube-creator-pp-cli groups update` — Modifies a group. For example, you could change a group's title.

**media** — Manage media

- `youtube-creator-pp-cli media <resourceName>` — Method for media download. Download is supported on the URI `/v1/media/{+name}?alt=media`.

**report-types** — Manage report types

- `youtube-creator-pp-cli report-types` — Lists report types.

**reports** — Manage reports

- `youtube-creator-pp-cli reports` — Retrieve your YouTube Analytics reports.

**youtube** — Manage youtube

- `youtube-creator-pp-cli youtube abuse-reports-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube activities-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube captions-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube captions-download` — Downloads a caption track.
- `youtube-creator-pp-cli youtube captions-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube captions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube captions-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube channel-banners-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube channel-sections-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube channel-sections-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube channel-sections-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube channel-sections-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube channels-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube channels-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube comment-threads-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube comment-threads-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube comments-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube comments-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube comments-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube comments-mark-as-spam` — Expresses the caller's opinion that one or more comments should be flagged as spam.
- `youtube-creator-pp-cli youtube comments-set-moderation-status` — Sets the moderation status of one or more comments.
- `youtube-creator-pp-cli youtube comments-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube i18n-languages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube i18n-regions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube live-broadcasts-bind` — Bind a broadcast to a stream.
- `youtube-creator-pp-cli youtube live-broadcasts-delete` — Delete a given broadcast.
- `youtube-creator-pp-cli youtube live-broadcasts-insert` — Inserts a new stream for the authenticated user.
- `youtube-creator-pp-cli youtube live-broadcasts-insert-cuepoint` — Insert cuepoints in a broadcast
- `youtube-creator-pp-cli youtube live-broadcasts-list` — Retrieve the list of broadcasts associated with the given channel.
- `youtube-creator-pp-cli youtube live-broadcasts-transition` — Transition a broadcast to a given status.
- `youtube-creator-pp-cli youtube live-broadcasts-update` — Updates an existing broadcast for the authenticated user.
- `youtube-creator-pp-cli youtube live-chat-bans-delete` — Deletes a chat ban.
- `youtube-creator-pp-cli youtube live-chat-bans-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube live-chat-messages-delete` — Deletes a chat message.
- `youtube-creator-pp-cli youtube live-chat-messages-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube live-chat-messages-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube live-chat-moderators-delete` — Deletes a chat moderator.
- `youtube-creator-pp-cli youtube live-chat-moderators-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube live-chat-moderators-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube live-streams-delete` — Deletes an existing stream for the authenticated user.
- `youtube-creator-pp-cli youtube live-streams-insert` — Inserts a new stream for the authenticated user.
- `youtube-creator-pp-cli youtube live-streams-list` — Retrieve the list of streams associated with the given channel. --
- `youtube-creator-pp-cli youtube live-streams-update` — Updates an existing stream for the authenticated user.
- `youtube-creator-pp-cli youtube members-list` — Retrieves a list of members that match the request criteria for a channel.
- `youtube-creator-pp-cli youtube memberships-levels-list` — Retrieves a list of all pricing levels offered by a creator to the fans.
- `youtube-creator-pp-cli youtube playlist-items-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube playlist-items-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube playlist-items-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube playlist-items-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube playlists-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube playlists-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube playlists-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube playlists-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube search-list` — Retrieves a list of search resources
- `youtube-creator-pp-cli youtube subscriptions-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube subscriptions-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube subscriptions-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube super-chat-events-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube tests-insert` — POST method.
- `youtube-creator-pp-cli youtube third-party-links-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube third-party-links-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube third-party-links-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube third-party-links-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube thumbnails-set` — As this is not an insert in a strict sense (it supports uploading/setting of a thumbnail for multiple videos, which...
- `youtube-creator-pp-cli youtube update-comment-threads` — Updates an existing resource.
- `youtube-creator-pp-cli youtube video-abuse-report-reasons-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube video-categories-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube videos-delete` — Deletes a resource.
- `youtube-creator-pp-cli youtube videos-get-rating` — Retrieves the ratings that the authorized user gave to a list of specified videos.
- `youtube-creator-pp-cli youtube videos-insert` — Inserts a new resource into this collection.
- `youtube-creator-pp-cli youtube videos-list` — Retrieves a list of resources, possibly filtered.
- `youtube-creator-pp-cli youtube videos-rate` — Adds a like or dislike rating to a video or removes a rating from a video.
- `youtube-creator-pp-cli youtube videos-report-abuse` — Report abuse for a video.
- `youtube-creator-pp-cli youtube videos-update` — Updates an existing resource.
- `youtube-creator-pp-cli youtube watermarks-set` — Allows upload of watermark image and setting it for a channel.
- `youtube-creator-pp-cli youtube watermarks-unset` — Allows removal of channel watermark.

**youtube-reporting-jobs** — Manage youtube reporting jobs

- `youtube-creator-pp-cli youtube-reporting-jobs create` — Creates a job and returns it.
- `youtube-creator-pp-cli youtube-reporting-jobs delete` — Deletes a job.
- `youtube-creator-pp-cli youtube-reporting-jobs get` — Gets a job.
- `youtube-creator-pp-cli youtube-reporting-jobs list` — Lists jobs.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-creator-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily moderation digest into Slack

```bash
youtube-creator-pp-cli mod queue --since 24h --json --select 'items.snippet.topLevelComment.snippet.{authorDisplayName,textOriginal,publishedAt}'
```

Held-for-review comments emitted as filtered JSON for piping into n8n's Slack node.

### Weekly analytics roll-up to email

```bash
youtube-creator-pp-cli digest analytics --since 7d --markdown > /tmp/yt-week.md
```

Markdown digest ready for n8n Send Email body — covers views, watch time, top videos, CTR, and revenue.

### Append legal footer to every video posted this month

```bash
youtube-creator-pp-cli bulk metadata --published-after 2026-05-01 --append-description ./footer.md --dry-run
```

Dry-run prints the videos that would be updated; remove `--dry-run` to apply. Quota-cheap because it enumerates via playlistItems.list.

### PubSubHubbub upload-trigger into n8n

```bash
youtube-creator-pp-cli pubsub subscribe --channel UCxxx --callback https://n8n.example.com/webhook/yt-upload --hub https://pubsubhubbub.appspot.com/
```

Switches from polling for new uploads (quota-burning) to push triggers.

### Backup the last 30 days of your channel weekly

```bash
youtube-creator-pp-cli backup --since 30d --captions --thumbnails --info-json --out s3://backups/yt/
```

Wraps yt-dlp with the right auth cookies; the index is recorded in the local SQLite store so future runs skip already-archived videos.

## Auth Setup

Authenticates via OAuth2 device flow (run `auth login` once, refresh token persisted at ~/.config/youtube-creator-pp-cli/auth.json) or via a read-only YouTube API key for public-data commands. The auth flow handles scope escalation — re-running `auth login --scope yt-analytics-monetary.readonly` adds revenue access without re-doing the full flow. n8n calls the binary as a subprocess; the binary loads the persisted token. members.list requires a Google access-approval form separately; doctor surfaces this status.

Run `youtube-creator-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  youtube-creator-pp-cli group-items list --agent --select id,name,status
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
youtube-creator-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
youtube-creator-pp-cli feedback --stdin < notes.txt
youtube-creator-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.youtube-creator-pp-cli/feedback.jsonl`. They are never POSTed unless `YOUTUBE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YOUTUBE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
youtube-creator-pp-cli profile save briefing --json
youtube-creator-pp-cli --profile briefing group-items list
youtube-creator-pp-cli profile list --json
youtube-creator-pp-cli profile show briefing
youtube-creator-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `youtube-creator-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add youtube-creator-pp-mcp -- youtube-creator-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which youtube-creator-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   youtube-creator-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `youtube-creator-pp-cli <command> --help`.
