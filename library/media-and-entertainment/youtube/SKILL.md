---
name: pp-youtube
description: "Search YouTube in bulk, grab transcripts, get embed snippets, fetch top comments, list a channel's recent uploads — for the photo-keywords-to-blog-post workflow. Trigger phrases: `search youtube for`, `find youtube videos about`, `get youtube transcript`, `find videos like`, `youtube embed for`, `top comments on`, `recent uploads from`, `latest videos from @`, `use youtube-pp`, `run youtube-pp`."
author: "Justin"
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

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install youtube --cli-only
   ```
2. Verify: `youtube-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/cmd/youtube-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when you have search terms (from photo tags, image labels, or notes) and want relevant YouTube videos back — with transcripts to verify relevance and embed snippets to drop into a blog draft. It's read-only and local; not a webapp backend, not a write tool.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Blog-post composition
- **`youtube search-bulk`** — Take a list of search terms from stdin or args, return top-N YouTube videos per term in one JSON document with titles, channels, embed URLs, and thumbnails.

  _When you have N search terms from an upstream pipeline (image labels, photo tags, scraped keywords), reach for this instead of looping single searches._

  ```bash
  youtube-pp-cli youtube search-bulk "sourdough scoring" "latte art" --top 3
  ```
- **`youtube videos-transcript`** — Fetch the spoken-content transcript of a YouTube video using the timedtext endpoint. Works for auto-generated and manual captions on any public video. Caches into the local store.

  _Read the transcript before deciding whether a candidate video actually fits the topic of the blog post or photo._

  ```bash
  youtube-pp-cli youtube videos-transcript dQw4w9WgXcQ --lang en --json
  ```
- **`youtube videos-embed`** — Print embed HTML, iframe, or markdown-style embed for a video ID. Direct copy-paste into a blog draft.

  _Once you've picked a video for the blog, get the embed snippet without remembering the exact iframe URL pattern._

  ```bash
  youtube-pp-cli youtube videos-embed dQw4w9WgXcQ --format markdown
  ```

### Reachability mitigation
- **`youtube videos-related`** — Find videos related to a target video using topicDetails + same-channel + tag overlap from the local store. Best-effort replacement for the deprecated relatedToVideoId parameter.

  _When one candidate video is on-topic but you want more like it, this is the working stand-in for the deprecated parameter._

  ```bash
  youtube-pp-cli youtube videos-related dQw4w9WgXcQ --limit 10 --json
  ```

### Audience signal
- **`youtube videos-comments`** — Fetch top comments on a video, ranked locally by likeCount. Pulls up to 5 pages from commentThreads.list and sorts so most-liked floats regardless of API order.

  _Use to gauge whether a video is actually well-received before embedding, or to surface viewer-supplied context the description doesn't mention._

  ```bash
  youtube-pp-cli youtube videos-comments dQw4w9WgXcQ --top 10
  ```

### Channel discovery
- **`youtube channel-uploads`** — List a channel's most recent uploads in one call. Resolves @handle or channelId, walks the auto-generated uploads playlist, returns video IDs + titles + publish times.

  _Replaces the manual two-step lookup. Pairs naturally with --for-handle workflows._

  ```bash
  youtube-pp-cli youtube channel-uploads @veritasium --top 10
  ```

## Command Reference

**youtube** — Manage youtube

- `youtube-pp-cli youtube channels-list` — Retrieves a list of resources, possibly filtered. Now supports `--for-handle @handle` for modern channel resolution.
- `youtube-pp-cli youtube comment-threads-list` — Retrieves top-level comment threads, filterable by video, channel, or thread id.
- `youtube-pp-cli youtube playlist-items-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube playlists-list` — Retrieves a list of resources, possibly filtered.
- `youtube-pp-cli youtube search-list` — Retrieves a list of search resources
- `youtube-pp-cli youtube videos-list` — Retrieves a list of resources, possibly filtered.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

> **Sample use case:** [justinwfu/pictovideo](https://github.com/justinwfu/pictovideo) puts the recipes below into a real photo-keywords → candidate-video → embedded blog draft pipeline. Look there if you want to see the commands wired end-to-end before composing your own flow.

### Photo keywords to candidate videos

```bash
cat photo-keywords.txt | youtube-pp-cli youtube search-bulk --stdin --top 5 --json --select "results[].videoId,results[].title,results[].channelTitle,results[].embedUrl"
```

Reads keywords (one per line), searches each, returns webapp-ready embed-URL JSON. Use --select with dotted paths to keep the output tight.

### Verify a candidate via transcript

```bash
youtube-pp-cli youtube videos-transcript dQw4w9WgXcQ --lang en --json --select "text" | head -c 2000
```

Pull the first 2KB of the transcript to confirm the video is actually about the topic before adding it to the blog.

### Get a markdown embed for the draft

```bash
youtube-pp-cli youtube videos-embed dQw4w9WgXcQ --format markdown
```

Drops a ready-to-paste markdown video reference into your editor buffer.

### Find more candidates like a picked one

```bash
youtube-pp-cli youtube videos-related dQw4w9WgXcQ --limit 5 --json --select "results[].videoId,results[].title"
```

Heuristic-based replacement for the deprecated relatedToVideoId. Uses topic + channel + tag overlap from synced local data.

### Bulk-research pipeline

```bash
cat photo-keywords.txt | youtube-pp-cli youtube search-bulk --stdin --top 3 --json | jq '.results[].videoId' | xargs -I {} youtube-pp-cli youtube videos-transcript {} --json
```

End-to-end: keywords in, transcripts out, ready for a quality filter before blog composition.

## Auth Setup

API-key only — set `YOUTUBE_API_KEY` and you're done. Read-only public-data operations only (10,000 quota units/day default). Write operations are not configured; this CLI is for discovery and research, not channel management.

Run `youtube-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  youtube-pp-cli youtube channels-list --agent --select id,name,status
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
youtube-pp-cli --profile briefing youtube channels-list
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
