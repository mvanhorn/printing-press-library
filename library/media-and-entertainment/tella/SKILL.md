---
name: pp-tella
description: "Every Tella API operation behind one CLI, with a local SQLite store, FTS5 transcript search, and webhook tooling... Trigger phrases: `search tella transcripts`, `find tella videos`, `list tella playlists`, `tail tella webhooks`, `export tella captions`, `use tella`, `run tella`."
author: "Greg Ceccarelli"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tella-pp-cli
---

# Tella — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tella-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install tella --cli-only
   ```
2. Verify: `tella-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Tella ships an API and an official MCP server; this CLI gives you both surfaces in one binary, plus a local-first store that makes cross-video transcript search, view-milestone rollups, and webhook replay actually fast. Every endpoint is a Cobra command, every command emits structured JSON, and every mutation supports --dry-run.

## When to Use This CLI

Reach for tella-pp-cli when you need agent-shaped Tella operations: scripted batch edits across a playlist, cross-video transcript search for an agent, webhook handler development without a tunnel, or programmatic export pipelines. The local SQLite store makes it ideal when an agent will issue many small queries against the same workspace.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local-first transcendence
- **`transcripts search`** — FTS5 search across every cached clip transcript in your workspace; returns video, clip, and timecode hits in milliseconds.

  _Use this when an agent or human needs to find every video that mentioned a topic without rehydrating the workspace from the API._

  ```bash
  tella-pp-cli transcripts search "pricing change" --json --limit 10
  ```
- **`videos viewed`** — Roll up webhook view-milestone events into a per-video summary over a window (e.g. who crossed 75% in the last 7 days).

  _Use this in a sales follow-up loop to triage prospects by engagement without scanning the dashboard._

  ```bash
  tella-pp-cli videos viewed --since 7d --milestone 75 --json
  ```
- **`workspace stats`** — Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.

  _Use this for monthly creator-economy reports without dashboard scraping._

  ```bash
  tella-pp-cli workspace stats --json
  ```

### Webhook tooling
- **`webhooks tail`** — Stream new webhook events from the inbox to stdout, and replay any prior message to a local URL with valid HMAC headers — no public tunnel needed.

  _Use this when developing a webhook handler against Tella without exposing localhost via a tunnel._

  ```bash
  tella-pp-cli webhooks tail --follow --json
  ```

### Bulk operations
- **`clips edit-pass`** — Apply a chained set of edits (remove-fillers, trim-silences-gt N, blur preset) across every clip in a playlist in one command.

  _Use this to apply a creator's standard edit pass across an entire playlist without per-clip clicking._

  ```bash
  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --trim-silences-gt 1s --dry-run
  ```
- **`exports wait`** — Kick off exports for one or more videos and block until each is ready, short-circuiting on the Export ready webhook event.

  _Use this in batch publishing scripts that need export URLs for downstream uploads._

  ```bash
  tella-pp-cli exports wait --video vid_1 --video vid_2 --timeout 10m --json
  ```

### Transcript tooling
- **`clips transcript-diff`** — Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

  _Use this to audit what an automated edit pass actually changed before publishing._

  ```bash
  tella-pp-cli clips transcript-diff clp_abc --json
  ```
- **`clips captions`** — Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

  _Use this to ship caption files alongside video embeds without round-tripping through a separate caption tool._

  ```bash
  tella-pp-cli clips captions clp_abc --format srt > captions.srt
  ```

## Command Reference

**playlists** — Playlist operations

- `tella-pp-cli playlists create` — Create a new playlist for the authenticated user
- `tella-pp-cli playlists delete` — Permanently delete a playlist. Videos in the playlist are not deleted.
- `tella-pp-cli playlists get` — Returns detailed information about a playlist including its videos
- `tella-pp-cli playlists list` — Returns a list of all playlists for the authenticated user
- `tella-pp-cli playlists update` — Update a playlist's name and/or description

**videos** — Video operations

- `tella-pp-cli videos delete` — Permanently delete a video
- `tella-pp-cli videos get` — Returns detailed information about a video including chapters, transcript, and thumbnails
- `tella-pp-cli videos list` — Returns a paginated list of all videos for the authenticated user. Use playlistId query parameter to filter videos...
- `tella-pp-cli videos update` — Update a video's settings including viewer options, download permissions, access controls, and metadata. Some...

**webhooks** — Webhook endpoint management

- `tella-pp-cli webhooks create-endpoint` — Creates a new webhook endpoint to receive events. Returns the endpoint ID and signing secret.
- `tella-pp-cli webhooks delete-endpoint` — Permanently deletes a webhook endpoint
- `tella-pp-cli webhooks get-endpoint-secret` — Retrieves the signing secret for a webhook endpoint. Use this to verify incoming webhook payloads.
- `tella-pp-cli webhooks get-message` — Returns details of a specific webhook message by ID
- `tella-pp-cli webhooks list-messages` — Returns a list of recently sent webhook messages for debugging purposes


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tella-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find every video where a topic was discussed

```bash
tella-pp-cli transcripts search "checkout flow" --json --select hits.video_id,hits.clip_id,hits.snippet
```

FTS5 over cached transcripts; --select narrows the response to the fields an agent needs.

### Roll up which prospects watched 75% this week

```bash
tella-pp-cli videos viewed --since 7d --milestone 75 --json
```

Reads cached webhook view-milestone events grouped by video and viewer.

### Apply a creator's standard edit pass to a playlist

```bash
tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --trim-silences-gt 1s --dry-run
```

Iterates clips in a playlist and chains real mutation endpoints; --dry-run shows the plan.

### Develop a webhook handler without ngrok

```bash
tella-pp-cli webhooks tail --once --json
tella-pp-cli webhooks replay <msg-id> --to http://localhost:8080/webhooks
```

`webhooks tail` snapshots the inbox; `webhooks replay <msg-id>` re-POSTs that message to a local URL with valid HMAC headers via the endpoint signing secret.

### Export captions for a clip

```bash
tella-pp-cli clips captions clp_abc --format srt > captions.srt
```

Formats the cut transcript as a standard SRT file.

## Auth Setup

Set TELLA_API_KEY to your Tella account API key (Account → Settings → API). Sent as Authorization: Bearer on every call. No OAuth, no refresh — it's a single static token.

Run `tella-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tella-pp-cli playlists list --agent --select id,name,status
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
tella-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tella-pp-cli feedback --stdin < notes.txt
tella-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tella-pp-cli/feedback.jsonl`. They are never POSTed unless `TELLA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TELLA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tella-pp-cli profile save briefing --json
tella-pp-cli --profile briefing playlists list
tella-pp-cli profile list --json
tella-pp-cli profile show briefing
tella-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tella-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tella-pp-mcp -- tella-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tella-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tella-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tella-pp-cli <command> --help`.
