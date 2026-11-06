---
name: pp-tella
description: "Edit recorded lessons, review a live preview, and deliver video with captions. Trigger phrases: `edit my Tella lesson`, `preview a Tella video`, `export Tella captions`, `use tella`, `run tella`."
author: "Greg Ceccarelli"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tella-pp-cli
    install:
      - kind: go
        bins: [tella-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/cmd/tella-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/tella/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Tella — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tella-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tella --cli-only
   ```
2. Verify: `tella-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/cmd/tella-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Prepare lesson recordings with repeatable recipes and reversible transcript cuts. Keep the review step separate from export and delivery.

## When to Use This CLI

Use for editing recorded lessons, reviewing live previews, and delivering exports with captions.

## Anti-triggers

Do not use this CLI for:
- Recording a screen or camera directly
- Publishing videos without explicit approval

## Unique Capabilities

### More
- **`transcripts search`** — FTS5 search across cached clip transcripts; returns video and clip references with matching text snippets.
- **`videos viewed`** — Roll up webhook view-count milestone events into a per-video summary over a window (e.g. which videos reached 25 views in the last 7 days).
- **`webhooks tail`** — webhooks tail streams or snapshots inbox events; webhooks replay separately replays prior messages to a local URL with valid HMAC headers.
- **`clips edit-pass`** — Plan or apply public filler, buffer, edge and silence edits across playlist clips; use find-retakes for transcript retakes and agent-chats for semantic edits.
- **`clips transcript-diff`** — Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

  ```bash
  tella-pp-cli clips transcript-diff cl_example --video=vid_example --json
  ```
- **`sources upload-file`** — Create a Tella source and upload local media bytes to its pre-signed URL then return the source ID; use videos clips insert-file to upload and attach media to a video.
- **`exports wait`** — Kick off exports for one or more videos and poll GET /v1/videos/{id}/exports/{exportId}, reporting progress and the download URL. Exit non-zero if an export fails or times out.

  ```bash
  tella-pp-cli exports wait --video=vid_example_one --video=vid_example_two --timeout 10m --json
  ```
- **`clips captions`** — Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

  ```bash
  tella-pp-cli clips captions cl_example --video=vid_example --format srt > captions.srt
  ```
- **`workspace stats`** — Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.
- **`videos storyboard`** — Export editable video timeline and state only; use videos apply-storyboard --file PLAN.json --apply to apply a reviewed JSON edit plan.
- **`videos clips insert-file`** — Upload local media into a Tella source and attach it as a clip; use videos clips add-broll for B-roll layout media.
- **`videos clips cut-words`** — Plan or apply term-matched transcript-index cuts; use videos clips replace-word-ranges for replacement media.
- **`videos audit`** — Scan a video timeline for likely issues such as long silences before export or sharing.
- **`playlists edit-pass`** — Plan a standard remove-fillers/remove-buffers/trim-edges/export recipe across every video in a playlist.
- **`videos clips clean`** — Plan or apply official per-clip cleanup with automatic pre-apply cut snapshots and explicit recovery.
- **`videos clips undo-last-cuts`** — Restore a saved cuts snapshot (latest by default or selected with --snapshot); use videos clips restore-cuts for inline cuts JSON and retain divergence checks and exclusive-editing confirmation.

### Browser-free lessons
- **`videos auto`** — Apply selected automatic edits across clips with recovery snapshots and finish with a live preview.
- **`lessons pass`** — Prepare a lesson from a validated recipe and finish with a preview for review.
- **`lessons ship`** — Export, wait, and download a lesson MP4 and SRT or VTT captions.
- **`clips find-retakes`** — Propose repeated sentence cuts with timestamps and apply reviewed cuts under snapshots.
- **`videos storyboard-image`** — Save video, clip, or source storyboard images for visual inspection.

## Command Reference

**account** — The signed-in user, their workspace, its plan and agent credits

- `tella-pp-cli account` — The signed-in user, their workspace and role in it, the workspace's plan (and when a trial ends) and its agent credits

**agent-chats** — Tella's editor agent, working in a video: start a chat and follow it

- `tella-pp-cli agent-chats list` — The editor agent chats in a video, most recently active first, each with its `status` and `lastReply`.
- `tella-pp-cli agent-chats start` — Start a chat with Tella's editor agent in a video you can edit, as if you had typed `prompt` in its agent chat

**auto-captures** — Screen walkthroughs an agent films of a web page with no one recording. Takes are started and read over the MCP server


**backgrounds** — Personal, workspace, and default video backgrounds

- `tella-pp-cli backgrounds` — Returns personal uploads, workspace uploads, and Tella's default image and animated backgrounds in named categories.

**library** — Reusable media saved to a workspace, plus Tella's curated sound effect and background music catalogs — the same items the editor's media panels show

- `tella-pp-cli library capture-screenshot` — Deprecated: use `POST /v1/library/imports`, which takes the same `url` and `scroll` and also imports video links.
- `tella-pp-cli library create-generation` — Starts generating an image
- `tella-pp-cli library create-item` — Saves an uploaded source to the library so it can be listed and reused later
- `tella-pp-cli library delete-item` — Removes an item from the library.
- `tella-pp-cli library get-generation` — Returns the current status of a generation started with `POST /v1/library/generations`.
- `tella-pp-cli library get-import` — Returns the status of an import started with `POST /v1/library/imports`. Pass the same `videoId` it was started with.
- `tella-pp-cli library list-items` — Returns a page of reusable media saved to the library — images, videos, screenshots, sound effects
- `tella-pp-cli library start-import` — Brings media into Tella from a link.

**motion** — Manage motion

- `tella-pp-cli motion apply-commands` — Applies a batch of ops and the editor's own commands as one transaction and one undo step
- `tella-pp-cli motion create-clip` — Adds a clip at the end of a video that plays a blank Tella Motion document of its own
- `tella-pp-cli motion create-document` — Creates a blank document, 10 s long (the `canvas.set` op changes that).
- `tella-pp-cli motion download-document` — Returns the whole stored document and its `seq`. Large: pull out what you need rather than reading it in full.
- `tella-pp-cli motion duplicate-document` — Copies a document under the same owner and optionally edits the copy in the same request with `commands`.
- `tella-pp-cli motion get-composition` — Returns a composition asset's node outline and the instances that play it. Editing it changes all of them.
- `tella-pp-cli motion get-document` — Returns the document's canvas, the outline of its nodes, its compositions and assets
- `tella-pp-cli motion get-document-thumbnail` — Returns a signed CDN URL for a still image (jpg/png/webp) or animated preview (gif/mp4) of the document on its own
- `tella-pp-cli motion get-node` — Returns one node with every stored field, its window, the composition it sits in and, for an instance
- `tella-pp-cli motion get-schema` — Returns the JSON schema of a document (`schema=document`), of every edit a commands batch takes (`schema=edits`)
- `tella-pp-cli motion inspect-node` — Returns a node as the editor's inspector shows it at a time: properties with units, values and keyframes
- `tella-pp-cli motion list-documents` — Lists Tella Motion documents you can edit, newest first.
- `tella-pp-cli motion list-effects` — Returns the effect kinds the `addEffect` command puts on a node's stack, each with its parameters' names, types, ranges
- `tella-pp-cli motion list-fonts` — Returns the fonts a title can be set in: the catalog's by slug
- `tella-pp-cli motion list-media` — Returns the videos, sounds and images already in the document, with the `mediaId` the `addMedia` command places.

**playlist-groups** — Sidebar groups for organizing playlists

- `tella-pp-cli playlist-groups create` — Create a sidebar group to organize playlists into.
- `tella-pp-cli playlist-groups delete` — Delete a playlist group. Playlists in the group are not deleted, they just become ungrouped.
- `tella-pp-cli playlist-groups list` — Returns the sidebar playlist groups for the authenticated user, in sidebar order.
- `tella-pp-cli playlist-groups update` — Rename a playlist group and/or move it to a position in its sidebar section (0 = top)

**playlists** — Playlist operations

- `tella-pp-cli playlists create` — Create a new playlist for the authenticated user
- `tella-pp-cli playlists delete` — Permanently delete a playlist. Videos in the playlist are not deleted.
- `tella-pp-cli playlists get` — Returns detailed information about a playlist including its videos
- `tella-pp-cli playlists list` — Returns a list of all playlists for the authenticated user
- `tella-pp-cli playlists update` — Update a playlist's name and/or description.

**social-accounts** — Manage social accounts

- `tella-pp-cli social-accounts` — Lists the social accounts you have connected in Tella that POST /v1/videos/{id}/publish can post with

**sources** — Manage sources

- `tella-pp-cli sources create` — Returns a `sourceId` and a pre-signed S3 URL.
- `tella-pp-cli sources get` — Returns whether a source from `POST /v1/sources` is ready to use, however it was placed (clip, b-roll, overlay

**subtitle-presets** — Manage subtitle presets

- `tella-pp-cli subtitle-presets list` — Returns Tella's built-in subtitle presets and the authenticated user's saved presets.
- `tella-pp-cli subtitle-presets save` — Saves a subtitle look as a personal preset, like the editor's 'Save as preset'.

**tags** — Tags for categorizing and filtering videos

- `tella-pp-cli tags create` — Create a workspace or private tag. When no color is given, one is derived deterministically from the name.
- `tella-pp-cli tags delete` — Permanently delete a tag and remove it from all videos. Private tags can only be deleted by their creator.
- `tella-pp-cli tags list` — Returns all tags visible to the authenticated user: the workspace's shared tags plus the user's private tags
- `tella-pp-cli tags update` — Update a tag's name, color and/or description. Private tags can only be updated by their creator.

**tella-public-analytics** — Manage tella public analytics

- `tella-pp-cli tella-public-analytics` — Viewing analytics aggregated across videos: by default every video shared with the workspace

**text-presets** — Manage text presets

- `tella-pp-cli text-presets delete` — Deletes one of the authenticated user's saved text presets. Built-in `default:` presets can't be deleted.
- `tella-pp-cli text-presets list` — Returns Tella's built-in text styles and the authenticated user's saved text presets — the tiles the editor's Text
- `tella-pp-cli text-presets save` — Saves a personal text preset, like the editor's 'Save style' on a text overlay.

**videos** — Video operations

- `tella-pp-cli videos create` — Create a new video.
- `tella-pp-cli videos delete` — Permanently delete a video
- `tella-pp-cli videos get` — Returns detailed information about a video including chapters, transcript, and thumbnails
- `tella-pp-cli videos list` — Returns a paginated list of all videos for the authenticated user.
- `tella-pp-cli videos search` — Search the authenticated user's direct video library by title or indexed transcript sentences.
- `tella-pp-cli videos update` — Update a video's settings including viewer options, download permissions, access controls, and metadata.

**webhooks** — Webhook endpoint management

- `tella-pp-cli webhooks create-endpoint` — Creates a new webhook endpoint owned by the authenticated API key's user.
- `tella-pp-cli webhooks delete-endpoint` — Permanently deletes a webhook endpoint
- `tella-pp-cli webhooks get-endpoint-secret` — Retrieves the signing secret for a webhook endpoint. Use this to verify incoming webhook payloads.
- `tella-pp-cli webhooks get-message` — Returns details of a specific webhook message by ID
- `tella-pp-cli webhooks list-endpoints` — Lists the webhook subscriptions owned by the authenticated API key's user.
- `tella-pp-cli webhooks list-messages` — Returns a list of recently sent webhook messages for debugging purposes
- `tella-pp-cli webhooks update-endpoint` — Updates the delivery URL, subscribed event types, or delivery status for a webhook endpoint.


**clips**


- `tella-pp-cli clips edit-pass` — Plan or apply public filler, buffer, edge and silence edits across playlist clips; use find-retakes for transcript retakes and agent-chats for semantic edits.

- `tella-pp-cli clips transcript-diff` — Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

- `tella-pp-cli clips captions` — Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

- `tella-pp-cli clips find-retakes` — Propose repeated sentence cuts with timestamps and apply reviewed cuts under snapshots.


**exports**


- `tella-pp-cli exports wait` — Kick off exports for one or more videos and poll GET /v1/videos/{id}/exports/{exportId}, reporting progress and the download URL. Exit non-zero if an export fails or times out.


**lessons**


- `tella-pp-cli lessons pass` — Prepare a lesson from a validated recipe and finish with a preview for review.

- `tella-pp-cli lessons ship` — Export, wait, and download a lesson MP4 and SRT or VTT captions.


**playlists**


- `tella-pp-cli playlists edit-pass` — Plan a standard remove-fillers/remove-buffers/trim-edges/export recipe across every video in a playlist.


**sources**


- `tella-pp-cli sources upload-file` — Create a Tella source and upload local media bytes to its pre-signed URL then return the source ID; use videos clips insert-file to upload and attach media to a video.


**transcripts**


- `tella-pp-cli transcripts search` — FTS5 search across cached clip transcripts; returns video and clip references with matching text snippets.


**videos**


- `tella-pp-cli videos viewed` — Roll up webhook view-count milestone events into a per-video summary over a window (e.g. which videos reached 25 views in the last 7 days).

- `tella-pp-cli videos clips insert-file` — Upload local media into a Tella source and attach it as a clip; use videos clips add-broll for B-roll layout media.

- `tella-pp-cli videos clips cut-words` — Plan or apply term-matched transcript-index cuts; use videos clips replace-word-ranges for replacement media.

- `tella-pp-cli videos audit` — Scan a video timeline for likely issues such as long silences before export or sharing.

- `tella-pp-cli videos clips clean` — Plan or apply official per-clip cleanup with automatic pre-apply cut snapshots and explicit recovery.

- `tella-pp-cli videos clips undo-last-cuts` — Restore a saved cuts snapshot (latest by default or selected with --snapshot); use videos clips restore-cuts for inline cuts JSON and retain divergence checks and exclusive-editing confirmation.

- `tella-pp-cli videos auto` — Apply selected automatic edits across clips with recovery snapshots and finish with a live preview.

- `tella-pp-cli videos storyboard-image` — Save video, clip, or source storyboard images for visual inspection.


**webhooks**


- `tella-pp-cli webhooks tail` — webhooks tail streams or snapshots inbox events; webhooks replay separately replays prior messages to a local URL with valid HMAC headers.


**workspace**


- `tella-pp-cli workspace stats` — Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tella-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Plan a lesson pass

```bash
tella-pp-cli lessons pass vid_lesson --dry-run --json
```

Inspect the house-style edit plan before applying it.

## Auth Setup

Run `tella-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
echo "$TOKEN" | tella-pp-cli auth set-token
```

Or set `TELLA_API_KEY` as an environment variable.

Run `tella-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tella-pp-cli account --agent --select billingURL,credits,plan
  ```
- **Previewable** — `--dry-run` shows the request without sending. A completed dry run exits 0 with `"success": true` and `"dry_run": true` and no HTTP `status`; validation failures still exit non-zero with `"success": false`.
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TELLA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TELLA_CONFIG_DIR`, `TELLA_DATA_DIR`, `TELLA_STATE_DIR`, `TELLA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TELLA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `tella-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "tella": {
        "command": "tella-pp-mcp",
        "env": {
          "TELLA_HOME": "/srv/tella"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TELLA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TELLA_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `TELLA_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `TELLA_DATA_SOURCE` sets the `--data-source` default.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
tella-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "tella-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `tella-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `tella-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `tella-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
tella-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
tella-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
tella-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
tella-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`tella-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TELLA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tella-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tella-pp-cli feedback --stdin < notes.txt
tella-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TELLA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TELLA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
tella-pp-cli profile save briefing --json
tella-pp-cli --profile briefing account
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
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tella-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/cmd/tella-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tella-pp-mcp -- tella-pp-mcp
   ```
3. Verify: `claude mcp list`

For a remote host, `tella-pp-mcp --transport http` requires `TELLA_MCP_HTTP_TOKEN` set to at least 32 characters (callers send `Authorization: Bearer <token>`; `openssl rand -hex 32` prints 64 hex characters and meets that minimum), binds `127.0.0.1:7777` by default, and needs `--tls-cert`/`--tls-key` for any non-loopback `--addr`. Requests with no `Origin` header are accepted. Loopback origins are accepted; any other `Origin` is refused unless listed in `TELLA_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). `TELLA_MCP_READ_ONLY=1` drops mirrored write commands (local-write commands that only touch the CLI's own learn store stay) and makes the raw execute tool refuse state-changing calls.

## Direct Use

1. Check if installed: `which tella-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tella-pp-cli account --agent
   ```
4. If ambiguous, drill into subcommand help: `tella-pp-cli clips captions --help`.

## Unique Capabilities

Workflows provided by this CLI.

### Local-first transcendence

- **`transcripts search`** — FTS5 search across cached clip transcripts; returns video and clip references with matching text snippets.

  _Use this when an agent or human needs to find every video that mentioned a topic without rehydrating the workspace from the API._

  ```bash
  tella-pp-cli transcripts search "pricing change" --json --limit 10
  ```

- **`videos viewed`** — Roll up webhook view-count milestone events into a per-video summary over a window (e.g. which videos reached 25 views in the last 7 days).

  `video.milestone` events fire at 1, 5, 10, 25, 50, 100, and 500 views.

  _Use this in a sales follow-up loop to triage prospects by engagement without scanning the dashboard._

  ```bash
  tella-pp-cli videos viewed --since 7d --milestone 25 --json
  ```

- **`workspace stats`** — Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.

  _Use this for monthly creator-economy reports without dashboard scraping._

  ```bash
  tella-pp-cli workspace stats --json
  ```

### Webhook tooling

- **`webhooks tail`** — webhooks tail streams or snapshots inbox events; webhooks replay separately replays prior messages to a local URL with valid HMAC headers.

  _Use this when developing a webhook handler against Tella without exposing localhost via a tunnel._

  ```bash
  tella-pp-cli webhooks tail --follow --json
  ```

### Bulk operations

- **`videos clips clean` / `videos clean`** — Preview and apply recoverable cleanup passes for one clip or every clip discovered from a video's official timeline.

  ```bash
  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --remove-fillers --remove-silences natural \
    --dry-run
  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --remove-fillers --remove-silences natural \
    --apply

  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --range 1200:1800 --word-range 42:46 --dry-run

  tella-pp-cli videos clean vid_abc \
    --remove-fillers --remove-silences natural --dry-run
  tella-pp-cli videos clean vid_abc \
    --remove-fillers --remove-silences natural --apply
  ```

  `clean` snapshots existing cuts before applying, records the last cuts returned by successful cleanup, and batches manual playback ranges. Tella has no conditional cut update, so partial failures never auto-restore: the CLI re-fetches touched clips, reports recovery state, and keeps each snapshot. Undo first re-fetches and refuses if current cuts diverged from the snapshot's recorded post-clean state, but that check is advisory rather than atomic. Preview with `videos clips undo-last-cuts <vid> <clipId> --dry-run`; apply only with exclusive access by adding `--apply --confirm-exclusive-editing`.

  Official silence modes are `natural` (>800ms), `fast` (>500ms), and `faster` (>300ms). Legacy `--remove-buffers` remains supported with its exact `--buffer-min-ms` threshold (default 200ms), which is more aggressive than official `faster`; do not pass both options.

- **`clips edit-pass`** — Plan or apply public filler, buffer, edge and silence edits across playlist clips; use find-retakes for transcript retakes and agent-chats for semantic edits.

  _Use this to apply a creator's standard edit pass across an entire playlist without per-clip clicking._

  ```bash
  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --remove-buffers --trim-edges --dry-run
  ```

  Per-clip primitives also available individually:
  - `videos clips remove-silences <vid> <clipId> --mode natural --apply` — official Tella silence removal, preview by default.
  - `videos clips remove-buffers <vid> <clipId>` — legacy threshold behavior: cuts every silence ≥ `--min-ms` (default 200). Public API only.
  - `videos clips trim-edges <vid> <clipId>` — narrow primitive: cuts only the head and tail silences. Public API only.
  - `videos clips find-mistakes` — retired compatibility pointer; use `clips find-retakes` or `agent-chats start`.

### Retired cookie mistake detection

The cookie transport is removed. All old mistake flags fail before HTTP; use the reviewed retake workflow or the explicit credit-gated Agent.

- **`exports wait`** — Kick off exports for one or more videos and poll GET /v1/videos/{id}/exports/{exportId}, reporting progress and the download URL. Exit non-zero if an export fails or times out.

  _Use this in batch publishing scripts that need export URLs for downstream uploads._

  ```bash
  tella-pp-cli exports wait --video=vid_1 --video=vid_2 --timeout 10m --json
  ```

### Transcript tooling

- **Official transcript read/correction flow** — Read stable cut-applied word indices, then preview and apply atomic transcript/caption corrections.

  All transcript reads use GET /v1/videos/{id}/clips/{clipId}/transcript. Choose `--view words` (default), `sentences`, or `text`; `--include-cut` is words-only and includes removed words and source times, as does `get-uncut-transcript`. Time windows use playback times by default and source times with `--include-cut`; `--end-ms` is exclusive. `transcripts sync` fails on a fetch error and reports the clip and progress.

  ```bash
  tella-pp-cli videos clips get-transcript vid_abc cl_xyz --json
  tella-pp-cli videos clips get-transcript vid_abc cl_xyz --view sentences --json
  tella-pp-cli videos clips get-transcript vid_abc cl_xyz --view words --include-cut --json
  tella-pp-cli videos clips get-transcript vid_abc cl_xyz --start-ms 1000 --end-ms 5000 --json
  tella-pp-cli videos clips update-transcript-words vid_abc cl_xyz \
    --words '[{"index":12,"text":"Tella"},{"index":18,"hidden":true}]' \
    --dry-run
  tella-pp-cli videos clips update-transcript-words vid_abc cl_xyz \
    --words '[{"index":12,"text":"Tella"},{"index":18,"hidden":true}]' \
    --apply
  ```

- **`clips transcript-diff`** — Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

  _Use this to audit what an automated edit pass actually changed before publishing._

  ```bash
  tella-pp-cli clips transcript-diff cl_xyz --video=vid_abc --json
  ```

- **`clips captions`** — Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

  _Use this to ship caption files alongside video embeds without round-tripping through a separate caption tool._

  ```bash
  tella-pp-cli clips captions cl_xyz --video=vid_abc --format srt > captions.srt
  ```

## Browser-free lesson workflow

Plan first, review the preview, then invoke delivery separately:

```bash
tella-pp-cli lessons pass vid_example --recipe lesson-recipe.example.yaml --json
tella-pp-cli lessons pass vid_example --recipe lesson-recipe.example.yaml --apply
# Watch the returned preview before this separate delivery invocation.
tella-pp-cli lessons ship vid_example --output-dir ./delivery --format srt
```

`lessons pass` defaults to planning. Its default edits remove fillers and natural silences; it keeps existing caption styling, music, crop and zooms unless the recipe asks to change them. It saves cuts plus reference editor state before applying. After timing edits and optional source insertion it re-reads the timeline, proposes retakes for review, applies an optional subtitle preset and caption settings, drafts chapters from sentence starts plus each fresh clip's actual **timelineStartMs** playback offset, sets a thumbnail, and creates a preview. Chapter drafts use sentence text as single-line titles (at most 255 characters), a description and integer seconds, spaced at least 60 seconds apart, ascending, unique, and capped at 100. Applying replaces existing chapters. Retakes are never silently cut. No export or publishing happens in pass.

A preview contains a bearer token, including its `#token` fragment. Preserve the URL exactly; it works without signing in for **24 hours**, until the returned `expiresAt`. Share it only with the person reviewing, and keep preview and signed CDN URLs out of manuscripts, committed proofs and logs.

```bash
tella-pp-cli videos preview vid_example
tella-pp-cli videos auto vid_example --zooms --zoom-style subtle --crop --fillers --silences natural
tella-pp-cli videos auto vid_example --zooms --crop --fillers --silences natural --apply
tella-pp-cli videos timeline vid_example
tella-pp-cli videos timeline vid_example --include cuts,layouts,zooms --json
tella-pp-cli videos storyboard-image vid_example --clip cl_example --output ./storyboard.webp
tella-pp-cli videos storyboard-image --source src_example --output ./source.webp
```

`videos auto` also plans by default. Global `--dry-run` always wins over `--apply`. Ordinary dry runs read current clips; verifier/dogfood dry-run sweeps without fixtures short-circuit without HTTP. Zoom generation sends `replaceExisting:false`, keeping existing tracking and manual zooms. `--zoom-style` accepts gentle, subtle, moderate, focused, intense, and the API aliases slow, medium, fast. `--layouts` fails before writes because no public automatic-layout endpoint exists: use the retained `videos storyboard` / `videos apply-storyboard` JSON plan or the credit-gated Agent. `storyboard-image` is separate from that editable storyboard plan. It downloads a real WebP, checks status/type/signature, and reports its absolute path, bytes, columns, rows and intervalMs; width/height appear only if the platform decoder succeeds. Clip and source selectors are mutually exclusive.

Recipe YAML is parsed by yaml.v3, rejects unknown and duplicate keys, multiple documents, invalid types/enums/numerics and conflicting caption sizes **before the first write**. Files are read through a max-plus-one byte limit; anything over 1 MiB is rejected before YAML decoding, including trailing padding/documents. See [lesson-recipe.example.yaml](lesson-recipe.example.yaml); its optional style/media settings are examples, not Cathryn's brand choices.

| Field | Type and allowed values | Default / effect |
| --- | --- | --- |
| `silences` | natural, fast, faster, or empty string | natural; empty disables silence cleanup |
| `fillers` | boolean | true |
| `zooms`, `crop` | booleans | false; preserve existing styling |
| `zoomStyle` | gentle/subtle/moderate/focused/intense/slow/medium/fast | omitted; specifying it enables zoom generation |
| `subtitlePreset` | nonblank preset ID | omitted; preserve current preset |
| `captions.fontFamily` | nonblank Tella catalog/built-in/current uploaded family | omitted; API validates availability |
| `captions.fontSize` | finite number 40–200 | omitted; exclusive with size |
| `captions.size` | small, medium, large | omitted; exclusive with fontSize |
| `captions.enabled`, `captions.defaultEnabled` | booleans | omitted; defaultEnabled:true conflicts with enabled:false |
| `backgroundMusic.presetId` / `sourceId` | exactly one existing preset/audio-source ID | omitted; preserves music |
| `backgroundMusic.volume` | required finite number 0–1 when music specified | explicit volume |
| `intro.sourceId`, `outro.sourceId` | existing uploaded source IDs; local paths rejected | omitted; source reads checked before edits |
| `intro.name`, `outro.name` | optional single-line clip name, at most 255 characters | API default |
| `thumbnail.rule` | first-spoken or fixed | first-spoken: first sentence start in final playback; 0 if no speech |
| `thumbnail.inpointMs` | nonnegative integer, required only for fixed, inside final playback | mutually exclusive with first-spoken |

Intro uses the existing source insertion helper and moves the inserted clip to position 0; outro appends. No arbitrary local path is uploaded. Recipe changes to presets, music and source IDs still depend on availability and permissions reported by Tella.

For conservative retakes, save and review the proposed earlier sentence cuts, then apply the same file:

```bash
tella-pp-cli clips find-retakes vid_example cl_example --save-plan retakes.json --json
tella-pp-cli clips find-retakes vid_example cl_example --plan retakes.json --apply
# videos clips find-retakes is also supported.
```

Only adjacent sentences with at least five exact normalized prefix words and at most a four-second gap qualify. The earlier complete fragment is proposed; the last matching take stays. Ordinary short repetitions are ignored. This cannot judge semantic mistakes or whether repetition was intentional. Plans include stable API word indices, playback timestamps and a transcript-plus-cuts fingerprint. Applying requires the same video/clip/version and transcript/cuts fingerprint. Review may remove candidates, including all candidates; the remaining entries must be exact, duplicate-free members of the fresh plan. Added or altered ranges and stale plans are refused. Inclusive fromWordIndex/toWordIndex ranges go to the official cut-by-transcript endpoint.

`find-mistakes`, `clean --find-mistakes` and `edit-pass --find-mistakes` are retired and refuse before reading cookies or performing writes, even with `--unofficial`. For semantic review, use the official Agent deliberately:

```bash
tella-pp-cli agent-chats list --video-id vid_example --json
tella-pp-cli agent-chats start --video-id vid_example --prompt 'Review the pacing and suggest improvements' --dry-run
tella-pp-cli agent-chats start --video-id vid_example --prompt 'Review the pacing and suggest improvements' --yes
```

Start verifies the target and reads uncached account credits before the paid POST, then reports before/after balances. An explicit `--yes` set to true is required; `--agent` implied consent does not count. Use `--new-video` only to intentionally create an empty video; omitted video targets never silently create one. Start is disabled for verifier/dogfood mutation runs. On an ambiguous failure inspect `agent-chats list` before manually retrying.

Recovery is deliberately bounded. Cut snapshots restore **cuts only**, using the existing divergence check and exclusive-editing acknowledgement:

```bash
tella-pp-cli videos clips undo-last-cuts vid_example cl_example --dry-run
tella-pp-cli videos clips undo-last-cuts vid_example cl_example --apply --confirm-exclusive-editing
```

Auto/pass errors print recovery file paths and stop. Local 0600 reference snapshots retain pre-edit video/clip settings and the full timeline (including crop/layout/zoom state); these support manual recovery, not a full editor undo. Cut restore cannot undo crop/zoom/style/music/chapters/thumbnail changes or inserted intro/outro clips. Recovery checks are advisory because Tella has no atomic conditional update; avoid concurrent editing and never blindly replay a failed pass.

`lessons ship` is a separate explicit export invocation. It checks the video, creates a full-video 1080p/30fps/1x export, polls uncached status with the encoded export ID, and downloads actual `<video>.mp4` plus `<video>.srt` or `.vtt` in `--output-dir` (default delivery). `--poll-interval` is positive and at most 1m; `--wait-timeout` is positive and at most 1h (defaults 5s/10m). Missing/failed statuses and timeouts are errors. Media uses a credential-free download client; redirects cannot leave the signed origin. MP4/WebP signatures and caption text are checked. Private temporary files are removed on failure; completed files are atomic and 0600. Existing targets are refused unless `--overwrite` explicitly allows replacing regular files. Media defaults to a 10-minute transfer timeout; explicit global `--timeout` overrides it, including 0 for no whole-transfer deadline (caller cancellation still wins). Paths print only after successful downloads. Dry-run previews local targets/options without export or download. No publishing endpoint is used.

Every Tella API POST/PUT/PATCH/DELETE gets a new random UUID Idempotency-Key **once per logical request**, stable across retries. Nonempty case-insensitive header overrides win. Empty aliases cannot erase generated keys, conflicting aliases are rejected, and caller header maps are not mutated. Keys are not added to reads or to presigned CDN upload requests, and no generated key is saved in global configuration.

Runtime MCP mirrors these workflows. `agent_chats_start` requires an explicit boolean `yes:true`; string values and implied agent defaults do not authorize spend. Thin `tella_execute` keeps its confirm:true contract and refuses Agent starts after its no-write preview branch; use the runtime mirror for the credit check. MCP workflow filenames are plain basenames resolved under `tella-artifacts/` in the server working directory: output (.webp), recipe (.yaml/.yml), plan/save-plan (.json), and ship output-dir. Path traversal and artifact symlinks are refused; put reviewed recipes/plans there before invoking them. CLI paths remain explicit operator paths. `--json`, `--select`, `--compact` and `--dry-run` use the existing output/preview helpers.

See [BROWSER-ONLY.md](BROWSER-ONLY.md) for the documented browser boundary. Local fixtures verify these contracts. Scoped live evidence on owned test copies includes an HTTP 200 preview and validated MP4/caption and WebP artifacts. Validation does not claim universal command or positive retake-apply coverage.
