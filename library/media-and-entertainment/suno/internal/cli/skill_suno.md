---
name: pp-suno
description: "The correct, offline-first Suno CLI — every feature the abandoned clients have, plus a local SQLite library Trigger phrases: `generate a song with suno`, `make music with suno`, `search my suno library`, `download my suno tracks`, `download a wav from suno`, `organize my suno tracks into a workspace`, `what are my top suno songs`, `use suno`, `run suno`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - suno-pp-cli
    install:
      - kind: go
        bins: [suno-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/cmd/suno-pp-cli
---

# Suno — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `suno-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install suno --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `suno-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/cmd/suno-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or script needs to generate Suno music programmatically, manage and search a personal Suno library offline, or run reproducible generate/poll/download pipelines with typed exit codes and JSON output. It is the right choice over the abandoned community wrappers because it tracks the current Suno contract and persists everything locally.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local library no Suno tool has
- **`grep`** — Find clips by remembered lyric, prompt, or tag phrases via local full-text search.

  _Reach for this when an agent needs to locate a specific past song by its content, not its title._

  ```bash
  suno-pp-cli grep "rain" --json --select id,title,tags
  ```
- **`analytics`** — Grouped roll-ups across your whole library: counts, average duration/bpm, total plays and upvotes by any clip field.

  _Use this to see which models or settings produced your most-played tracks before the next session._

  ```bash
  suno-pp-cli analytics --type clips --group-by model_name --json
  ```
- **`lineage`** — Render the iteration tree of a track: its extends, covers, and remixes, reconstructed from local links.

  _Reach for this to trace which seed a finished song descended from across many variations._

  ```bash
  suno-pp-cli lineage 7d869de4-9476-4a4d-a6f2-c0eec968a3e2 --json
  ```
- **`top`** — Ranked flat list of your best clips by plays, upvotes, or duration, with machine-readable output.

  _Use this to pipe your strongest tracks into a publishing or playlist workflow._

  ```bash
  suno-pp-cli top --by upvote_count --limit 10 --json
  ```
- **`sql`** — Run read-only SQL directly against your local synced clip store.

  _Reach for this when an agent needs an arbitrary query no fixed command covers._

  ```bash
  suno-pp-cli sql "SELECT title, duration FROM clips WHERE make_instrumental = 1 ORDER BY duration DESC LIMIT 5" --json
  ```

### Reachability awareness
- **`credits --forecast`** — Remaining credits plus your recent generation volume measured against Suno's captcha-throttle threshold.

  _Use this before a batch session to judge how many more generations you can run before hCaptcha kicks in._

  ```bash
  suno-pp-cli credits --forecast --json
  ```

## Command Reference

**billing** — Account credits, plan, and available models

- `suno-pp-cli billing` — Show credits, plan, and models
- `suno-pp-cli billing eligible-discounts` — Show discounts you're eligible for
- `suno-pp-cli billing usage-plan` — Show the plan comparison table
- `suno-pp-cli billing usage-plan-faq` — Show plan/usage FAQ

**clips** — Your Suno songs (clips) — list, fetch, manage

- `suno-pp-cli clips list` — List your library (walks the opaque next_cursor; use --all to drain). Shows a `workspace` column from the local membership index.
- `suno-pp-cli clips get` — Fetch clip(s) by ID (comma-separated; Suno batches 2 at a time)
- `suno-pp-cli clips set <clip_id>` — Update a clip's title, caption, or lyrics
- `suno-pp-cli clips publish <clip_id>` — Toggle a clip public or private
- `suno-pp-cli clips delete <clip_id>…` — Trash (or, with `--undo`, restore) one or more clips
- `suno-pp-cli clips timed-lyrics <clip_id>` — Word-level timestamped lyrics for a clip
- `suno-pp-cli clips stems <clip_id>` — Separate a clip into stems (vocals + instruments)
- `suno-pp-cli clips convert-wav <clip_id>` — Trigger WAV (lossless) conversion (Pro/Premier); poll wav-url
- `suno-pp-cli clips wav-url <clip_id>` — Get the WAV download URL (null until conversion finishes)
- `suno-pp-cli clips attribution <clip_id>` — Show a clip's sample/attribution info
- `suno-pp-cli clips comments <clip_id>` — List comments on a clip
- `suno-pp-cli clips parent <clip_id>` — Show a clip's parent (the clip it was derived from)
- `suno-pp-cli clips similar <clip_id>` — Find clips similar to a given clip
- `suno-pp-cli clips direct-children-count <clip_ids>` — Count direct children (extends/covers)

**generate** — Music, lyrics, and video generation jobs

- `suno-pp-cli generate create` — Generate a custom song from lyrics (captcha-gated). `--variation high|normal|subtle`, `--project <id>`.
- `suno-pp-cli generate describe` — Description-driven (inspiration) generation. `--variation`, `--instrumental`. No `--title` is needed; the model writes its own.

All captcha-gated generate commands accept `--wait-for-gate` (with `--gate-timeout`, default 30m): when Suno's adaptive gate is tripped (HTTP 422 `token_validation_failed`), the command backs off and retries until the gate reopens or the timeout elapses. Off by default. In `--agent`/JSON mode a gate failure is emitted as a structured envelope on stdout with `error_type: "captcha_required"` and `retriable: true`, so agents branch on a field rather than parsing prose.
- `suno-pp-cli generate extend <clip_id>` — Extend a clip from a timestamp
- `suno-pp-cli generate cover <clip_id>` — Cover / restyle a clip
- `suno-pp-cli generate remaster <clip_id>` — Remaster a clip
- `suno-pp-cli generate concat` — Finalize/concatenate an extended clip into a full song
- `suno-pp-cli generate lyrics` — Submit a lyrics-generation job (returns an id to poll)
- `suno-pp-cli generate lyrics-status <id>` — Poll a lyrics-generation job by id
- `suno-pp-cli generate video-status <id>` — Check video-generation status for a clip

**persona** — Voice personas

- `suno-pp-cli persona get <persona_id>` — View a voice persona
- `suno-pp-cli persona usage` — Show how many clips use each persona (and which are orphans)

**project** — Workspaces (Suno 'projects') — organize your tracks into collections

- `suno-pp-cli project list` — List your workspaces
- `suno-pp-cli project get <project_id>` — Show a workspace and its clips
- `suno-pp-cli project create` — Create a new workspace
- `suno-pp-cli project rename <project_id>` — Rename a workspace
- `suno-pp-cli project trash` — Trash (or restore) a workspace
- `suno-pp-cli project default` — Show your default workspace
- `suno-pp-cli project pinned-clips` — List clips pinned to your default workspace
- `suno-pp-cli project add <project_id>` — Add clip(s) to a workspace (repeatable `--clip`)
- `suno-pp-cli project remove <project_id>` — Remove clip(s) from a workspace (repeatable `--clip`)

**playlist / trending / user / notification** — library and account reads

- `suno-pp-cli playlist list` — List your playlists
- `suno-pp-cli trending list` — Show trending clips
- `suno-pp-cli user config` — Show your user config
- `suno-pp-cli user personalization` — Show your personalization settings
- `suno-pp-cli user personalization-memory` — Show your personalization memory
- `suno-pp-cli notification list` — List your notifications
- `suno-pp-cli notification badge-count` — Show unread notification badge count

**skill** — Install the Suno CLI as a coding-agent skill

- `suno-pp-cli skill install` — Write this SKILL into Claude/Codex/Cursor locations. `--agent claude|codex|cursor|all`, `--print`, `--path <file>`, `--force`.

**Restored local & utility commands** — parity with the 2026-05-15 build; these coexist with the novel commands above

- `suno-pp-cli vibes save <name>` — Save a local generation recipe (tags/prompt-template/persona/mv); replay with `vibes use <name> [topic]`. Also `vibes list|get|delete`.
- `suno-pp-cli burn --by tag|persona|model|hour` — Aggregate estimated generation credits across your local library
- `suno-pp-cli budget set daily|monthly <N>` — Set a local credit cap; `budget show` reports spend; caps block over-limit `generate` calls. `budget clear` removes the cap.
- `suno-pp-cli sessions` — Group synced clips into 30-minute-gap working sessions
- `suno-pp-cli ship <clip-id>` — Export an editor-ready bundle (audio + video + cover + `.lrc` + `.json` sidecar)
- `suno-pp-cli tail <resource> --interval 10s` — Poll the API and stream changes as NDJSON
- `suno-pp-cli tree <clip-id>` — Render a clip's local lineage tree (legacy view; see also `lineage`)
- `suno-pp-cli custom-model` — List pending custom-model training jobs


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
suno-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Generate a custom song and wait for it

```bash
suno-pp-cli generate create --title "Night Drive" --tags "synthwave, retro" --lyrics "Neon lights on wet asphalt" --token hc_demo_token --wait --json
```

Submits a custom generation, polls until complete, and prints the finished clip as JSON. Replace the token with a real hCaptcha token.

### Pull a deeply nested clip and select only what you need

```bash
suno-pp-cli clips get --ids 7d869de4-9476-4a4d-a6f2-c0eec968a3e2 --agent --select id,title,status,audio_url,metadata.duration,metadata.tags
```

Fetches a clip and narrows the verbose nested payload to just the fields an agent needs, saving context.

### Find and rank instrumentals offline

```bash
suno-pp-cli sql "SELECT title, duration FROM clips WHERE make_instrumental = 1 ORDER BY duration DESC LIMIT 5" --json
```

Queries the local store directly for your longest instrumental tracks, no API call.

### Check throttle headroom before a session

```bash
suno-pp-cli credits --forecast --json
```

Shows remaining credits and recent generation volume against the captcha-throttle threshold.

### Organize tracks into a workspace

```bash
suno-pp-cli workspace add ws_3f2a91 --clip clip_a1b2 --clip clip_c3d4
```

Adds clips to a Suno workspace (project) so they stay organized on suno.com.

### Download a lossless WAV

```bash
suno-pp-cli download clip_a1b2c3 --format wav --out ./tracks
```

Triggers WAV conversion, polls until ready, and saves the lossless file (Pro/Premier).

## Auth Setup

Suno uses Clerk session auth (auth.suno.com). Run `suno-pp-cli auth login --chrome` to capture your logged-in session cookie from Chrome; the CLI mints and refreshes the short-lived JWT for you. No password or API key is stored. Music generation is attempted optimistically with no token. Suno gates generation adaptively (an hCaptcha anti-bot challenge that usually fires only after sustained use), so many generations succeed outright; if Suno challenges a request, the CLI tells you to retry with `--token` carrying an hCaptcha token (e.g. solved via 2Captcha). All read, library, and metadata commands never need a captcha.

Run `suno-pp-cli doctor` to verify setup. Add `--probe-gate` to check the live generation gate specifically: the default health check only proves the billing API is reachable, which stays green even while generation is gated. `doctor --probe-gate` reports `tripped` (the adaptive hCaptcha gate is active) or `open`. WARNING: it issues a real generation — free when the gate is tripped, but it creates a clip and spends credits when the gate is open (the probe clip is best-effort trashed).

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  suno-pp-cli clips list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
suno-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
suno-pp-cli feedback --stdin < notes.txt
suno-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/suno-pp-cli/feedback.jsonl`. They are never POSTed unless `SUNO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUNO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
suno-pp-cli profile save briefing --json
suno-pp-cli --profile briefing clips list
suno-pp-cli profile list --json
suno-pp-cli profile show briefing
suno-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `suno-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/cmd/suno-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add suno-pp-mcp -- suno-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which suno-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   suno-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `suno-pp-cli <command> --help`.
