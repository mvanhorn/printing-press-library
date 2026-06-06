---
name: pp-twelvelabs
description: "Printing Press CLI for Twelvelabs. Use the TwelveLabs Video Understanding API to extract information from your videos and make it available to your"
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - twelvelabs-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/ai/twelvelabs/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Twelvelabs — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `twelvelabs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install twelvelabs --cli-only
   ```
2. Verify: `twelvelabs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/twelvelabs/cmd/twelvelabs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the Twelve Labs API and adds workflow commands for video upload, indexing waits, structured briefs, embeddings, and local clip cutting. Agents can go from a long source video to JSON or Markdown editing guidance without manually watching the whole file.

## Editor Workflow

For long videos, prefer the workflow commands before falling back to raw endpoint mirrors:

```bash
twelvelabs-pp-cli upload-video --index-id IDX --file ./long-video.mp4 --wait --json
twelvelabs-pp-cli video-brief --video-id VIDEO_ID --format json --out edit-plan.json
twelvelabs-pp-cli video-brief --video-id VIDEO_ID --format markdown --out edit-plan.md
twelvelabs-pp-cli clips --input ./long-video.mp4 --plan edit-plan.json --out ./clips
```

Use `upload-video` for local files or public URLs, `video-brief` for editor-ready JSON/Markdown with chapters, highlights, and recommended cuts, `embed` for video embeddings with optional wait polling, and `clips` for local `ffmpeg` cuts from a generated plan.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Editor workflows
- **`upload-video`** — Upload a local video file or register a public URL, then poll Twelve Labs until indexing reaches a terminal status.

  _Use this before analysis when you need the CLI to wait until the video is usable._

  ```bash
  twelvelabs-pp-cli upload-video --index-id IDX --file ./long-video.mp4 --wait --json --dry-run
  ```
- **`video-brief`** — Generate a deterministic JSON or Markdown editing plan with title, topics, hashtags, chapters, highlights, and recommended cuts.

  _Use this when you want to avoid manually watching a long video just to find the strongest moments._

  ```bash
  twelvelabs-pp-cli video-brief --video-id VID --format json --json --dry-run
  ```
- **`embed`** — Create a video embedding task from a local file or public URL and optionally wait for ready results.

  _Use this when you need embeddings for deeper search, matching, or semantic workflows._

  ```bash
  twelvelabs-pp-cli embed --video-file ./long-video.mp4 --model marengo3.0 --wait --json --dry-run
  ```
- **`clips`** — Cut local clip files with ffmpeg from a video-brief JSON plan without inventing timestamps.

  _Use this after video-brief when you have the source video locally and want clip files immediately._

  ```bash
  twelvelabs-pp-cli clips --dry-run --json
  ```

## Command Reference

**analyze** — Manage analyze

- `twelvelabs-pp-cli analyze` — This endpoint analyzes your videos and creates fully customizable text based on your prompts

**editor workflows** — High-level video editing helpers

- `twelvelabs-pp-cli upload-video` — Upload or register a video and optionally wait for indexing.
- `twelvelabs-pp-cli video-brief` — Create a deterministic editor-ready JSON or Markdown plan.
- `twelvelabs-pp-cli embed` — Create video embeddings from a local file or URL and optionally wait for readiness.
- `twelvelabs-pp-cli clips` — Cut local clips from a `video-brief` JSON plan using `ffmpeg`.

**assets** — Manage assets

- `twelvelabs-pp-cli assets create` — This method creates an asset by uploading a file to the platform.
- `twelvelabs-pp-cli assets create-multipart-upload` — This method creates a multipart upload session. **Supported content**: Video and audio **File size**: 4GB maximum.
- `twelvelabs-pp-cli assets delete` — This method deletes the specified asset. This action cannot be undone.
- `twelvelabs-pp-cli assets get-upload-status` — This method provides information about an upload session, including its current status, chunk-level progress
- `twelvelabs-pp-cli assets list` — This method returns a list of assets in your account.
- `twelvelabs-pp-cli assets list-incomplete-uploads` — This method returns a list of all incomplete multipart upload sessions in your account.
- `twelvelabs-pp-cli assets report-chunk-batch` — This method reports successfully uploaded chunks to the platform.
- `twelvelabs-pp-cli assets request-additional-presigned-urls` — This method generates new presigned URLs for specific chunks that require uploading.
- `twelvelabs-pp-cli assets retrieve` — This method retrieves details about the specified asset.

**embed** — Manage embed

- `twelvelabs-pp-cli embed create-text-image-audio-embedding` — <Note title='Note'> This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.
- `twelvelabs-pp-cli embed create-video-embedding-task` — <Note title='Note'> This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.
- `twelvelabs-pp-cli embed list-video-embedding-tasks` — <Note title='Note'> This method will be deprecated in a future version. Migrate to the [Embed API v2](/v1.
- `twelvelabs-pp-cli embed retrieve-video-embedding` — This method retrieves embeddings for a specific video embedding task.
- `twelvelabs-pp-cli embed retrieve-video-embedding-task` — <Note title='Note'> This endpoint will be deprecated in a future version. Migrate to the [Embed API v2](/v1.

**embed-v2** — Manage embed v2

- `twelvelabs-pp-cli embed-v2 create-async-embedding-task` — This endpoint creates embeddings for audio and video content asynchronously.
- `twelvelabs-pp-cli embed-v2 create-embeddings` — This endpoint synchronously creates embeddings for multimodal content and returns the results immediately in the
- `twelvelabs-pp-cli embed-v2 list-async-embedding-tasks` — This method returns a list of the async embedding tasks in your account.
- `twelvelabs-pp-cli embed-v2 retrieve-embeddings` — This method retrieves the status and the results of an async embedding task.

**entity-collections** — Manage entity collections

- `twelvelabs-pp-cli entity-collections create` — This method creates an entity collection.
- `twelvelabs-pp-cli entity-collections delete` — This method deletes the specified entity collection. This action cannot be undone.
- `twelvelabs-pp-cli entity-collections list` — This method returns a list of the entity collections in your account.
- `twelvelabs-pp-cli entity-collections retrieve` — This method retrieves details about the specified entity collection.
- `twelvelabs-pp-cli entity-collections update` — This method updates the specified entity collection.

**gist** — Manage gist

- `twelvelabs-pp-cli gist` — <Note title='Deprecation notice'> This endpoint will be sunset and removed on February 15, 2026.

**indexes** — Manage indexes

- `twelvelabs-pp-cli indexes create-index` — This method creates an index.
- `twelvelabs-pp-cli indexes delete-index` — This method deletes the specified index and all the videos within it. This action cannot be undone.
- `twelvelabs-pp-cli indexes list` — This method returns a list of the indexes in your account.
- `twelvelabs-pp-cli indexes retrieve-index` — This method retrieves details about the specified index.
- `twelvelabs-pp-cli indexes update-index` — This method updates the name of the specified index.

**summarize** — Manage summarize

- `twelvelabs-pp-cli summarize` — <Note title='Deprecation notice'> This endpoint will be sunset and removed. Use the [`POST`](/v1.

**tasks** — Manage tasks

- `twelvelabs-pp-cli tasks create-video-indexing` — This method creates a video indexing task that uploads and indexes a video in a single operation.
- `twelvelabs-pp-cli tasks delete-video-indexing` — This action cannot be undone.
- `twelvelabs-pp-cli tasks list-video-indexing` — This method returns a list of the video indexing tasks in your account.
- `twelvelabs-pp-cli tasks retrieve-video-indexing` — This method retrieves a video indexing task.

**video_search** — Manage video search

- `twelvelabs-pp-cli video-search any-to` — Use this endpoint to search for relevant matches in an index using text, media, or a combination of both as your query.
- `twelvelabs-pp-cli video-search any-to-video-retrieve-specific-page` — Use this endpoint to retrieve a specific page of search results.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
twelvelabs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `twelvelabs-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export TWELVELABS_X_API_KEY="<your-key>"
```

Or persist it in `~/.config/twelvelabs-video-understanding-pp-cli/config.toml`.

Run `twelvelabs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  twelvelabs-pp-cli assets list --agent --select id,name,status
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
twelvelabs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
twelvelabs-pp-cli feedback --stdin < notes.txt
twelvelabs-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/twelvelabs-pp-cli/feedback.jsonl`. They are never POSTed unless `TWELVELABS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TWELVELABS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
twelvelabs-pp-cli profile save briefing --json
twelvelabs-pp-cli --profile briefing assets list
twelvelabs-pp-cli profile list --json
twelvelabs-pp-cli profile show briefing
twelvelabs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `twelvelabs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add twelvelabs-pp-mcp -- twelvelabs-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which twelvelabs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   twelvelabs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `twelvelabs-pp-cli <command> --help`.
