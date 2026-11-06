# Tella CLI

**Edit recorded lessons, review a live preview, and deliver video with captions.**

Prepare lesson recordings with repeatable recipes and reversible transcript cuts. Keep the review step separate from export and delivery.

Learn more at [Tella](https://www.tella.com/docs/versioning).

Created by [@gregce](https://github.com/gregce) (Greg Ceccarelli).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

The expanded command set in this reprint becomes available through the public installer after merge and release automation.

## Install

The recommended path installs both the `tella-pp-cli` binary and the `pp-tella` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tella
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tella --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tella --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tella --agent claude-code
npx -y @mvanhorn/printing-press-library install tella --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.10 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/cmd/tella-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tella-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tella --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tella --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tella --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tella --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tella-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TELLA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/cmd/tella-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "tella": {
      "command": "tella-pp-mcp",
      "env": {
        "TELLA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

### Remote MCP over HTTP

`tella-pp-mcp --transport http` (or `PP_MCP_TRANSPORT=http`) serves streamable HTTP at `http://127.0.0.1:7777/mcp`. It refuses to start until `TELLA_MCP_HTTP_TOKEN` is set to at least 32 characters, and every request must send `Authorization: Bearer <token>`. The command below prints 64 hex characters and meets that minimum. Plain HTTP is allowed only on a loopback `--addr`; any other bind, including `:7777` (every interface), also needs `--tls-cert` and `--tls-key`. Requests with no `Origin` header are accepted, as are loopback origins (`localhost`, `127.0.0.1`, or `[::1]`, any port). Any other `Origin` is refused unless it is listed in `TELLA_MCP_HTTP_ALLOWED_ORIGINS`, a comma-separated list of `scheme://host[:port]` values with no path. That list is not a CORS policy; the server does not emit CORS headers.

```bash
export TELLA_MCP_HTTP_TOKEN="$(openssl rand -hex 32)"
tella-pp-mcp --transport http
```

Set `TELLA_MCP_READ_ONLY=1` (exactly `1`) to drop mirrored commands that are not marked read-only and make the raw execute tool refuse state-changing calls. Commands marked local-write, which write only this CLI's own learn store, stay available.

## Quick Start

```bash
# Check credentials and connectivity.
tella-pp-cli doctor --json

# Find lesson recordings.
tella-pp-cli videos list --json --select videos.id,videos.name

```

## Unique Features

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

## Recipes

### Plan a lesson pass

```bash
tella-pp-cli lessons pass vid_lesson --dry-run --json
```

Inspect the house-style edit plan before applying it.

## Usage

Run `tella-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TELLA_CONFIG_DIR`, `TELLA_DATA_DIR`, `TELLA_STATE_DIR`, or `TELLA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TELLA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TELLA_HOME=/srv/tella
tella-pp-cli doctor
```

Under `TELLA_HOME=/srv/tella`, the four dirs resolve to `/srv/tella/config`, `/srv/tella/data`, `/srv/tella/state`, and `/srv/tella/cache`.

On Windows, set the same variable in PowerShell:

```powershell
$env:TELLA_HOME = "C:\srv\tella"
tella-pp-cli doctor
```

Under `$env:TELLA_HOME = "C:\srv\tella"`, the four dirs resolve to `C:\srv\tella\config`, `C:\srv\tella\data`, `C:\srv\tella\state`, and `C:\srv\tella\cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `TELLA_DATA_DIR` overrides an explicit `--home` for that kind. Use `TELLA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TELLA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `tella-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

### Local data

Reads are saved locally by default: in `--data-source auto` mode every successful read is also written to the SQLite store (`data.db`, or `data-<hash>.db` per credential) in the data directory. GET responses are cached for five minutes under `http/` in the cache directory. Credentials live in `credentials.toml` in the data directory. The learn journal lives under `learn/` in the state directory. `tella-pp-cli doctor` prints the resolved paths, every store file, and the response-cache size.

None of it is encrypted. New files are created owner-only; beyond that the data relies on OS account permissions and full-disk encryption (FileVault, BitLocker, LUKS).

- **Erase it:** `tella-pp-cli local-data purge` lists the store files, response cache, and learn journal it would delete; add `--yes` to delete them, or `--only store|cache|journal` to narrow it. Config and credentials are kept.
- **Run without a local copy:** set `TELLA_NO_LOCAL_DATA=1`. Reads go to the API (`--data-source live`; `--data-source local` still reads an existing store), the response cache is off (`--no-cache`), nothing new is written to the store, and `sync` and `workflow archive` refuse to run. Set it in an MCP host's `env` block and every command the server runs inherits it.
- **Choose the read source without flags:** `TELLA_DATA_SOURCE=auto|live|local` sets the `--data-source` default; an explicit flag wins.

## Commands

### account

The signed-in user, their workspace, its plan and agent credits

- **`tella-pp-cli account`** - The signed-in user, their workspace and role in it, the workspace's plan (and when a trial ends) and its agent credits: the balance, the monthly allowance and the packages an owner can buy. Use it to tell when a request needs a paid plan or more credits.

### agent-chats

Tella's editor agent, working in a video: start a chat and follow it

- **`tella-pp-cli agent-chats list`** - The editor agent chats in a video, most recently active first, each with its `status` and `lastReply`. Poll it to follow a chat you started: `idle` means its turn ended.
- **`tella-pp-cli agent-chats start`** - Start a chat with Tella's editor agent in a video you can edit, as if you had typed `prompt` in its agent chat; without `videoId` it first makes a new, empty video. The agent works in the background and spends the workspace's AI Credits. Follow it with `GET /v1/agent-chats?videoId=`.

### auto-captures

Screen walkthroughs an agent films of a web page with no one recording. Takes are started and read over the MCP server


### backgrounds

Personal, workspace, and default video backgrounds

- **`tella-pp-cli backgrounds`** - Returns personal uploads, workspace uploads, and Tella's default image and animated backgrounds in named categories. Each item includes a preview URL and a background value that can be passed directly to the clip update endpoint.

### library

Reusable media saved to a workspace, plus Tella's curated sound effect and background music catalogs — the same items the editor's media panels show

- **`tella-pp-cli library capture-screenshot`** - Deprecated: use `POST /v1/library/imports`, which takes the same `url` and `scroll` and also imports video links. Captures a public web page or a single post on X, Instagram, LinkedIn or TikTok and saves it to the caller's private library. Pages are captured above the fold; social posts are cropped to their embed card. Set scroll=true to record a scrolling web page as video (ignored for social posts). Returns a finished image or video library item with a sourceId for overlays or layout media. Unlike AI generation, capture completes in this request, with no polling; allow up to two minutes. Private pages and social profiles/feeds are not supported. Use an Idempotency-Key when retrying to avoid duplicate captures.
- **`tella-pp-cli library create-generation`** - Starts generating an image, a short video clip or a sound effect from a text prompt with the same AI generators the editor's media panels use. Generation is asynchronous: the response is a `generation` whose `id` is the private library item the result is saved as. Poll `GET /v1/library/generations/{id}` every few seconds (images usually take 30-90 seconds, sound effects 10-30, video several minutes) until `status` is `completed`, then pass `item.sourceId` anywhere a source is accepted — `POST /v1/videos/{id}/clips/{clipId}/overlays` for an image or video overlay, `POST /v1/videos/{id}/clips/{clipId}/layouts` with `media` for b-roll, `POST /v1/videos/{id}/clips/{clipId}/sound-effects` for a sound effect. The finished item also appears in `GET /v1/library?scope=private`. Each generation counts against the caller's weekly AI generation allowance, which depends on the plan; exceeding it answers 403 with the reset time. A video is also charged in the workspace's AI Credits and is refused with a 403 while the balance is empty.
- **`tella-pp-cli library create-item`** - Saves an uploaded source to the library so it can be listed and reused later, both here and in the editor's media panels. Create the source via `POST /v1/sources` and upload the bytes first, then pass its `sourceId`. Without this a source is only reachable through the clips that already reference it. To bring in an image you saved in a connected Pinterest, Are.na or mymind account instead, pass `from` with the service and `sourceId` set to the item's `id` from `GET /v1/library?scope=<service>`; Tella copies the image into your `private` library (default), the `workspace` library, or with `scope: "video"` the library of the video named by `videoId`. Only images can be brought in: Pinterest image pins, Are.na image blocks, and mymind images or the screenshot mymind took of a saved page.
- **`tella-pp-cli library delete-item`** - Removes an item from the library. This removes the library entry only — clips already using the underlying source keep working.
- **`tella-pp-cli library get-generation`** - Returns the current status of a generation started with `POST /v1/library/generations`. Poll until `status` is `completed` (then `item` holds the finished library item) or `failed` (then `error` says why; start a new generation).
- **`tella-pp-cli library get-import`** - Returns the status of an import started with `POST /v1/library/imports`. Pass the same `videoId` it was started with. Poll until `status` is `completed` (then `item` holds the library item) or `failed` (then `error` says why; start a new import to retry).
- **`tella-pp-cli library list-items`** - Returns a page of reusable media saved to the library — images, videos, screenshots, sound effects, music and LUTs — for one scope. Items added through the API include a `sourceId` you can pass anywhere a source is accepted, including images. Editor-added images, screenshots, music and LUTs expose a hosted `url` instead. URL captures from `POST /v1/library/screenshots` are placeable `image` or `video` items. Media still being generated in the editor is omitted. `scope=default` returns Tella's curated sound effect and background music catalogs instead of stored media: those items carry a `presetId` to place them with (`sound-effect` items via `POST /v1/videos/{id}/clips/{clipId}/sound-effects`, `music` items via `PUT /v1/videos/{id}/background-music`), a `category`, and a publicly fetchable `url` for previewing, and arrive as a single page; filter with `type=sound-effect` or `type=music`, or omit `type` for both. Tella's default backgrounds are not part of this catalog — they are placed as a background rather than as a source, and are listed by `GET /v1/backgrounds`. `scope=pinterest`, `scope=arena` and `scope=mymind` list the images you saved in that service once you connect it under Settings → Connections: your newest saves, or matches for `query`, as one page. Their `id` is the service's own ID; bring one into Tella with `POST /v1/library` and `from` before placing it. A service you haven't connected returns 503 `unavailable`.
- **`tella-pp-cli library start-import`** - Brings media into Tella from a link. A public web page is captured above the fold at 1920×1080 (or, with `scroll=true`, recorded scrolling as video); a single post on X, Instagram, LinkedIn or TikTok is cropped to its embed card. These finish in this request (allow up to two minutes) and answer 201 with `status: completed`. A link to a single video is downloaded in the background and answers 202 with `status: running`: poll `GET /v1/library/imports/{id}?videoId=…` every 5-10 seconds until it is `completed` or `failed`. Video links need `videoId` and land in that video's library; everything else lands in your private library unless you pass `videoId`. Place `item.sourceId` as an overlay or layout media. Only import media you own or have the rights to use. Private pages, social profiles and feeds, and private, members-only or age-restricted videos are not supported. Use an Idempotency-Key when retrying to avoid duplicate imports.

### motion

Manage motion

- **`tella-pp-cli motion apply-commands`** - Applies a batch of ops and the editor's own commands as one transaction and one undo step; nothing applies unless everything does. Prefer a command for anything the editor does (titles, shapes, media, keyframes and easing, effects, masks, text animation, fades), since it keeps the editor's conventions; use ops (`node.patch`, `node.move`, `node.insert`, …) for the rest. `GET /v1/motion/schema?schema=edits&type=…` gives each one's fields. Pass `baseRevision` to refuse the batch with 409 if the document changed since you read it, and `dryRun` to check it without saving.
- **`tella-pp-cli motion create-clip`** - Adds a clip at the end of a video that plays a blank Tella Motion document of its own, or your document `documentId` that no video plays yet, and returns the clip and its document. With `documentId` and no `videoId`, it makes a new video for the document, as large as its canvas, and returns its `videoId` and `editUrl`. Build what it shows with `POST /v1/motion/documents/{id}/commands`. The clip routes that edit layouts, cuts, zooms, overlays or the transcript refuse a Tella Motion clip; it can still be renamed, moved and deleted.
- **`tella-pp-cli motion create-document`** - Creates a blank document, 10 s long (the `canvas.set` op changes that). With `videoId` it is the video's, as large as the video unless sized, for an overlay or b-roll you build in place. Without it the document is yours and in no video yet (1920×1080 unless sized); the first video it is placed in takes it over. Edit it with `POST /v1/motion/documents/{id}/commands`, then show it with `POST /v1/videos/{id}/clips/{clipId}/overlays` (`motionDocumentId`) or a layout's `media: {type: "motion", documentId}`. It plays there live, so later edits show in the video as you make them.
- **`tella-pp-cli motion download-document`** - Returns the whole stored document and its `seq`. Large: pull out what you need rather than reading it in full. Its shape is `GET /v1/motion/schema?schema=document`.
- **`tella-pp-cli motion duplicate-document`** - Copies a document under the same owner and optionally edits the copy in the same request with `commands`. Use it to reuse a design with other content, such as the same title card with another heading. The copy is independent of the original.
- **`tella-pp-cli motion get-composition`** - Returns a composition asset's node outline and the instances that play it. Editing it changes all of them.
- **`tella-pp-cli motion get-document`** - Returns the document's canvas, the outline of its nodes, its compositions and assets, the editor's state per node and `seq`, the revision to pass as `baseRevision` when editing.
- **`tella-pp-cli motion get-document-thumbnail`** - Returns a signed CDN URL for a still image (jpg/png/webp) or animated preview (gif/mp4) of the document on its own, at a time on its own clock: any document you can edit, also one no video plays yet. Width and height follow the document's canvas; a time at or past its end shows its end. By default returns a 302 redirect — set `?response=json` to get `{url}` JSON instead.
- **`tella-pp-cli motion get-node`** - Returns one node with every stored field, its window, the composition it sits in and, for an instance, the outline of the composition it opens. Use it with ops such as `node.patch`; for the inspector's view with units and keyframes use `GET /v1/motion/documents/{id}/nodes`.
- **`tella-pp-cli motion get-schema`** - Returns the JSON schema of a document (`schema=document`), of every edit a commands batch takes (`schema=edits`), or of one op or command (`schema=edits&type=node.patch`).
- **`tella-pp-cli motion inspect-node`** - Returns a node as the editor's inspector shows it at a time: properties with units, values and keyframes, its effect stack, expressions and style. Read a node this way before changing its look, animation or sound.
- **`tella-pp-cli motion list-documents`** - Lists Tella Motion documents you can edit, newest first. With `storyId`, a video's: its Tella Motion clips' and the documents its overlays and b-roll play. Without it, your documents that no video plays yet. Every one plays live where the video uses it, so an edit shows in the video at once. Tella Motion media in the library is a frozen snapshot, which becomes a new document of the video's when placed.
- **`tella-pp-cli motion list-effects`** - Returns the effect kinds the `addEffect` command puts on a node's stack, each with its parameters' names, types, ranges, units and defaults, and its presets by name. `kind` returns one effect with its presets' values, and `kind` with `preset` one preset's.
- **`tella-pp-cli motion list-fonts`** - Returns the fonts a title can be set in: the catalog's by slug, and the fonts uploaded to your library or your workspace's by id.
- **`tella-pp-cli motion list-media`** - Returns the videos, sounds and images already in the document, with the `mediaId` the `addMedia` command places.

### playlist-groups

Sidebar groups for organizing playlists

- **`tella-pp-cli playlist-groups create`** - Create a sidebar group to organize playlists into. Use the groupId field on PATCH /v1/playlists/{id} to move playlists into it.
- **`tella-pp-cli playlist-groups delete`** - Delete a playlist group. Playlists in the group are not deleted, they just become ungrouped.
- **`tella-pp-cli playlist-groups list`** - Returns the sidebar playlist groups for the authenticated user, in sidebar order. Personal groups are only visible to you; org groups are shared with the whole workspace.
- **`tella-pp-cli playlist-groups update`** - Rename a playlist group and/or move it to a position in its sidebar section (0 = top)

### playlists

Playlist operations

- **`tella-pp-cli playlists create`** - Create a new playlist for the authenticated user
- **`tella-pp-cli playlists delete`** - Permanently delete a playlist. Videos in the playlist are not deleted. Only the playlist's creator or a workspace owner can delete it; other editors get `403`.
- **`tella-pp-cli playlists get`** - Returns detailed information about a playlist including its videos
- **`tella-pp-cli playlists list`** - Returns a list of all playlists for the authenticated user
- **`tella-pp-cli playlists update`** - Update a playlist's name and/or description. Changing the playlist's name, description, link access, password, or search engine indexing requires being the playlist's creator or a workspace owner; other editors get `403`.

### social-accounts

Manage social accounts

- **`tella-pp-cli social-accounts`** - Lists the social accounts you have connected in Tella that POST /v1/videos/{id}/publish can post with: each one's `id` (pass it as `accountId`), its name, and the destinations it posts to. Accounts are connected in Tella (Settings → Connections); this API can't connect one.

### sources

Manage sources

- **`tella-pp-cli sources create`** - Returns a `sourceId` and a pre-signed S3 URL. PUT the file bytes to `uploadUrl`, then reference `sourceId` from clips, layouts (b-roll media), overlays, or backgrounds. For a 3D .cube LUT use `kind: "lut"`, omit dimensions and duration, upload unchanged, then save it with `POST /v1/library` and apply the returned item id with `PATCH /v1/videos/{id}/clips/{clipId}/filters`. Use `kind: "image"` for images (no `duration`); Tella hosts the bytes for rendering, so you never deal with raw image URLs. To reuse an uploaded source in other videos, save it to the workspace library with `POST /v1/library` (`scope: "workspace"`); `GET /v1/library` then finds it from any video.
- **`tella-pp-cli sources get`** - Returns whether a source from `POST /v1/sources` is ready to use, however it was placed (clip, b-roll, overlay, background, background music or sound effect). Poll it after placing a newly uploaded source: while it is `processing`, frames and previews of videos that use it answer not-ready instead of rendering.

### subtitle-presets

Manage subtitle presets

- **`tella-pp-cli subtitle-presets list`** - Returns Tella's built-in subtitle presets and the authenticated user's saved presets. Each includes a captionStyle preview. Apply by presetId to restore the full preset, including typography and saved layout settings.
- **`tella-pp-cli subtitle-presets save`** - Saves a subtitle look as a personal preset, like the editor's "Save as preset". Pass `videoId` to snapshot that video's current subtitles (style, font, weight, size, position, grouping and lines per block), `captionStyle` to build one from scratch on the style's defaults, or both — explicit caption fields override the video's. Omit `name` for the editor's default (the matching built-in's name, numbered). The preset then appears in GET /v1/subtitle-presets and in the editor's Subtitles panel, and can be applied to other videos with PUT /v1/videos/{id}/subtitle-preset.

### tags

Tags for categorizing and filtering videos

- **`tella-pp-cli tags create`** - Create a workspace or private tag. When no color is given, one is derived deterministically from the name.
- **`tella-pp-cli tags delete`** - Permanently delete a tag and remove it from all videos. Private tags can only be deleted by their creator.
- **`tella-pp-cli tags list`** - Returns all tags visible to the authenticated user: the workspace's shared tags plus the user's private tags
- **`tella-pp-cli tags update`** - Update a tag's name, color and/or description. Private tags can only be updated by their creator.

### tella-public-analytics

Manage tella public analytics

- **`tella-pp-cli tella-public-analytics`** - Viewing analytics aggregated across videos: by default every video shared with the workspace, or with scope=mine only the videos you created. Includes the same summary, plays per day, retention, geography and referrer breakdowns as a single video, plus the videos ranked by plays. Only topVideos is paginated; pass pagination.nextCursor as cursor for the next page. Dates are whole UTC days; omit both for all time.

### text-presets

Manage text presets

- **`tella-pp-cli text-presets delete`** - Deletes one of the authenticated user's saved text presets. Built-in `default:` presets can't be deleted. Text overlays already styled with the preset keep their look.
- **`tella-pp-cli text-presets list`** - Returns Tella's built-in text styles and the authenticated user's saved text presets — the tiles the editor's Text panel shows. Pass a presetId as `presetId` when adding or updating a text overlay to style it in one call; `fontSizeScale` is relative to the editor's default size so a preset reads the same on every canvas.
- **`tella-pp-cli text-presets save`** - Saves a personal text preset, like the editor's "Save style" on a text overlay. The style starts from an existing text overlay when `videoId` and `textOverlayId` are given (keeping a custom font and reading the size relative to that video's canvas), else from the editor's default text look; every style field sent alongside overrides it. The preset then appears in GET /v1/text-presets and in the editor's Text panel.

### videos

Video operations

- **`tella-pp-cli videos create`** - Create a new video. From an uploaded source: create a source (POST /v1/sources), PUT the video bytes to its uploadUrl, then call this endpoint with the sourceId, which becomes the new video's first clip. Without a sourceId the video is empty, as the editor's New video makes; add clips with POST /v1/videos/{id}/clips (an uploaded source) or POST /v1/motion/clips (Tella Motion).
- **`tella-pp-cli videos delete`** - Permanently delete a video
- **`tella-pp-cli videos get`** - Returns detailed information about a video including chapters, transcript, and thumbnails
- **`tella-pp-cli videos list`** - Returns a paginated list of all videos for the authenticated user. Use playlistId query parameter to filter videos by playlist.
- **`tella-pp-cli videos search`** - Search the authenticated user's direct video library by title or indexed transcript sentences.
- **`tella-pp-cli videos update`** - Update a video's settings including viewer options, download permissions, access controls, and metadata. Some features require Premium plan.

### webhooks

Webhook endpoint management

- **`tella-pp-cli webhooks create-endpoint`** - Creates a new webhook endpoint owned by the authenticated API key's user. Events are routed to the user context that emits them; playlist.video_added is delivered to the user who adds the video, who may be a playlist editor rather than its owner. Returns the endpoint ID and signing secret.
- **`tella-pp-cli webhooks delete-endpoint`** - Permanently deletes a webhook endpoint
- **`tella-pp-cli webhooks get-endpoint-secret`** - Retrieves the signing secret for a webhook endpoint. Use this to verify incoming webhook payloads.
- **`tella-pp-cli webhooks get-message`** - Returns details of a specific webhook message by ID
- **`tella-pp-cli webhooks list-endpoints`** - Lists the webhook subscriptions owned by the authenticated API key's user. Endpoints are scoped per user rather than shared across the workspace. Events are routed to the user context that emits them; playlist.video_added is delivered to the user who adds the video, who may be a playlist editor rather than its owner.
- **`tella-pp-cli webhooks list-messages`** - Returns a list of recently sent webhook messages for debugging purposes
- **`tella-pp-cli webhooks update-endpoint`** - Updates the delivery URL, subscribed event types, or delivery status for a webhook endpoint.


### clips


- **`tella-pp-cli clips edit-pass`** - Plan or apply public filler, buffer, edge and silence edits across playlist clips; use find-retakes for transcript retakes and agent-chats for semantic edits.

- **`tella-pp-cli clips transcript-diff`** - Diff a clip's cut transcript against its uncut transcript to surface every word that editing removed (filler, silence, hand-edit) with timecodes.

- **`tella-pp-cli clips captions`** - Format a clip's cut transcript as an SRT or VTT subtitle file ready to attach to an embed or upload.

- **`tella-pp-cli clips find-retakes`** - Propose repeated sentence cuts with timestamps and apply reviewed cuts under snapshots.


### exports


- **`tella-pp-cli exports wait`** - Kick off exports for one or more videos and poll GET /v1/videos/{id}/exports/{exportId}, reporting progress and the download URL. Exit non-zero if an export fails or times out.


### lessons


- **`tella-pp-cli lessons pass`** - Prepare a lesson from a validated recipe and finish with a preview for review.

- **`tella-pp-cli lessons ship`** - Export, wait, and download a lesson MP4 and SRT or VTT captions.


### playlists


- **`tella-pp-cli playlists edit-pass`** - Plan a standard remove-fillers/remove-buffers/trim-edges/export recipe across every video in a playlist.


### sources


- **`tella-pp-cli sources upload-file`** - Create a Tella source and upload local media bytes to its pre-signed URL then return the source ID; use videos clips insert-file to upload and attach media to a video.


### transcripts


- **`tella-pp-cli transcripts search`** - FTS5 search across cached clip transcripts; returns video and clip references with matching text snippets.


### videos


- **`tella-pp-cli videos viewed`** - Roll up webhook view-count milestone events into a per-video summary over a window (e.g. which videos reached 25 views in the last 7 days).

- **`tella-pp-cli videos clips insert-file`** - Upload local media into a Tella source and attach it as a clip; use videos clips add-broll for B-roll layout media.

- **`tella-pp-cli videos clips cut-words`** - Plan or apply term-matched transcript-index cuts; use videos clips replace-word-ranges for replacement media.

- **`tella-pp-cli videos audit`** - Scan a video timeline for likely issues such as long silences before export or sharing.

- **`tella-pp-cli videos clips clean`** - Plan or apply official per-clip cleanup with automatic pre-apply cut snapshots and explicit recovery.

- **`tella-pp-cli videos clips undo-last-cuts`** - Restore a saved cuts snapshot (latest by default or selected with --snapshot); use videos clips restore-cuts for inline cuts JSON and retain divergence checks and exclusive-editing confirmation.

- **`tella-pp-cli videos auto`** - Apply selected automatic edits across clips with recovery snapshots and finish with a live preview.

- **`tella-pp-cli videos storyboard-image`** - Save video, clip, or source storyboard images for visual inspection.


### webhooks


- **`tella-pp-cli webhooks tail`** - webhooks tail streams or snapshots inbox events; webhooks replay separately replays prior messages to a local URL with valid HMAC headers.


### workspace


- **`tella-pp-cli workspace stats`** - Local aggregate of video count, clip count, total duration, transcript word count, and export count by month across the cached workspace.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`tella-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`tella-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`tella-pp-cli learnings list`** - Inspect taught rows
- **`tella-pp-cli learnings forget <query>`** - Undo a teach
- **`tella-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`tella-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`tella-pp-cli teach-pattern`** - Install a query/resource template up front
- **`tella-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TELLA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `tella-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tella-pp-cli account

# JSON for scripting and agents
tella-pp-cli account --json
# Filter to specific fields
tella-pp-cli account --json --select billingURL,credits,plan

# Dry run — show the request without sending
tella-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tella-pp-cli account --agent
```

`--csv` prefixes a leading `'` on cells and header keys that start with `=`, `+`, `-`, `@`, a tab, or a line break (including fullwidth `＝＋－＠`) so a spreadsheet does not run them as formulas. Plain signed numbers such as `-5` and `+2.5e3` are left unchanged.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
tella-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `tella-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/tella-public-pp-cli/config.toml` on macOS/Linux and `%USERPROFILE%\.config\tella-pp-cli\config.toml` on Windows; `--home`, `TELLA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

`--ca-cert`, `TELLA_CA_CERT`, or `ca_cert` adds a PEM CA bundle to the system trust roots. `--tls-server-name`, `TELLA_TLS_SERVER_NAME`, or `tls_server_name` is the name checked against the certificate when the dialed address does not match it. Precedence is flag, then environment, then the config file. A config file other users can change does not supply these settings. Certificate verification failures are not retried.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TELLA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tella-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tella-pp-cli doctor` to check credentials
- Verify the environment variable is set: `[ -n "$TELLA_API_KEY" ] && echo configured`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `tella-pp-cli videos --json` to see videos or inspect the matching resource with `--help`

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Tella official skills**](https://github.com/tellahq/skills) — Markdown
- [**tella-cli**](https://github.com/Melvynx/tella-cli) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Unique Features

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

- **`videos clips clean` / `videos clean`** — Preview and apply recoverable cleanup passes without opening the browser editor. The clip command accepts manual playback-time and transcript-word ranges; the video command discovers every clip through Tella's official timeline endpoint.

  ```bash
  # Preview, then apply the same clip cleanup.
  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --remove-fillers \
    --remove-silences natural \
    --dry-run
  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --remove-fillers \
    --remove-silences natural \
    --apply

  # Cut hand-selected playback and transcript ranges in the same pass.
  tella-pp-cli videos clips clean vid_abc cl_xyz \
    --range 1200:1800 --range 8400:9100 \
    --word-range 42:46 --dry-run

  # Apply the official cleanup tools to every clip in a video.
  tella-pp-cli videos clean vid_abc \
    --remove-fillers --remove-silences natural --dry-run
  tella-pp-cli videos clean vid_abc \
    --remove-fillers --remove-silences natural --apply
  ```

  Cleanup snapshots each clip's exact existing cuts before mutation and records the last known post-clean cuts returned by Tella. Manual time ranges are sent together in one `cut_clip` request so earlier cuts cannot shift later coordinates. Because Tella exposes no conditional cut update, a failed compound apply never restores automatically: it re-fetches touched clips, reports whether their state is unchanged, matches the last known cleanup result, is indeterminate, or conflicts. Explicit undo re-fetches the clip and refuses if its cuts diverged from that recorded post-clean state. This is an advisory read-time check, not an atomic server guarantee: snapshot restore apply requires `--confirm-exclusive-editing`, which you must pass only after ensuring nobody else can save that clip until the command finishes.

  Tella's official silence modes are `natural` (pauses longer than 800ms), `fast` (>500ms), and `faster` (>300ms). `--remove-buffers` remains available for backward compatibility and preserves the older threshold-based composition (`--buffer-min-ms`, default 200ms), so it is more aggressive than official `faster`; choose one or the other.

- **`clips edit-pass`** — Plan or apply public filler, buffer, edge and silence edits across playlist clips; use find-retakes for transcript retakes and agent-chats for semantic edits.

  _Use this to apply a creator's standard edit pass across an entire playlist without per-clip clicking._

  ```bash
  tella-pp-cli clips edit-pass --playlist plst_42 --remove-fillers --remove-buffers --trim-edges --dry-run
  ```

  Per-clip primitives also available individually:
  - `videos clips cut <vid> <clipId> --range 2000:3500 --range 8000:9200` — cut multiple time ranges in one request.
  - `videos clips cut-by-transcript <vid> <clipId> --word-range 12:17` — cut by stable transcript word indices; Tella resolves exact word timing.
  - `videos clips remove-silences <vid> <clipId> --mode natural --apply` — official Tella silence-removal modes with preview-by-default safety.
  - `videos clips remove-buffers <vid> <clipId> [--min-ms N]` — legacy threshold mode: cuts every silence ≥ `N` ms (default 200). Public API only.
  - `videos clips trim-edges <vid> <clipId>` — narrow primitive: cuts only the head and tail silences. Public API only.
  - `videos clips find-mistakes` — retired compatibility pointer; use `clips find-retakes` or `agent-chats start`.

- **`sources upload-file` + `videos clips add`** — Create a Tella source, upload local video bytes to the pre-signed URL, then add that source as a new clip.

  ```bash
  tella-pp-cli sources upload-file intro.mp4 --width 1920 --height 1080 --duration 42.5 --json
  tella-pp-cli videos clips add vid_abc --source-id src_123 --name Intro --json
  ```

### Retired cookie mistake detection

The cookie transport is removed. All old mistake flags fail before HTTP; use the reviewed retake workflow or the explicit credit-gated Agent.

- **`exports wait`** — Kick off exports for one or more videos and poll GET /v1/videos/{id}/exports/{exportId}, reporting progress and the download URL. Exit non-zero if an export fails or times out.

  _Use this in batch publishing scripts that need export URLs for downstream uploads._

  ```bash
  tella-pp-cli exports wait --video=vid_1 --video=vid_2 --timeout 10m --json
  ```

### Transcript tooling

- **Current official transcript reads and corrections** — `get-transcript` returns the cut-applied transcript with stable word indices. `update-transcript-words` changes transcript/caption text or visibility atomically; it never changes audio or timing.

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


## Novel editing workflows

These commands compose Tella's public edit primitives into agent-safe workflows:

```bash
# Upload a local file and add it as a new clip.
tella-pp-cli videos clips insert-file vid_abc intro.mp4 --width 1920 --height 1080 --duration 42.5 --dry-run

# Upload B-roll and attach it as layout media over a time range.
tella-pp-cli videos clips add-broll vid_abc cl_xyz broll.mp4 --width 1920 --height 1080 --duration 8.2 --start-ms 4000 --duration-ms 6000 --dry-run

# Read the full outline, clean a clip, then inspect the recovery preview.
tella-pp-cli videos timeline vid_abc --include cuts,words --json
tella-pp-cli videos clips clean vid_abc cl_xyz --remove-fillers --remove-silences natural --dry-run
tella-pp-cli videos clips clean vid_abc cl_xyz --remove-fillers --remove-silences natural --apply
tella-pp-cli videos clips undo-last-cuts vid_abc cl_xyz --dry-run

# Apply undo only while you have exclusive editing access to this clip.
tella-pp-cli videos clips undo-last-cuts vid_abc cl_xyz --apply --confirm-exclusive-editing

# Inspect edit substrate, then cut transcript words or replace word ranges.
tella-pp-cli videos clips silence-map vid_abc cl_xyz --json
tella-pp-cli videos clips cut-words vid_abc cl_xyz --term "mistake" --dry-run
tella-pp-cli videos clips replace-word-ranges vid_abc cl_xyz --word-ranges '[{"fromWordIndex":12,"toWordIndex":17}]' --dry-run

# Restore inline cuts explicitly, or preview a snapshot's advisory divergence check.
tella-pp-cli videos clips restore-cuts vid_abc cl_xyz --cuts '[{"startTimeMs":100,"durationMs":150}]' --dry-run
tella-pp-cli videos clips restore-cuts vid_abc cl_xyz --snapshot cuts.json --dry-run
tella-pp-cli videos clips restore-cuts vid_abc cl_xyz --snapshot cuts.json --apply --confirm-exclusive-editing

# Adjust documented per-clip audio tracks. "inherit" clears an override.
tella-pp-cli videos clips update vid_abc cl_xyz --microphone-volume 0.8 --system-audio-volume 0 --dry-run
tella-pp-cli videos clips update vid_abc cl_xyz --microphone-volume 0.8 --system-audio-volume 0 --apply
tella-pp-cli videos clips update vid_abc cl_xyz --microphone-volume inherit --system-audio-volume inherit --apply

# Enable Studio Voice video-wide, then optionally opt a clip out.
tella-pp-cli videos update vid_abc --studio-voice true --dry-run
tella-pp-cli videos update vid_abc --studio-voice true --apply
tella-pp-cli videos clips update vid_abc cl_xyz --studio-voice false --dry-run
tella-pp-cli videos clips update vid_abc cl_xyz --studio-voice false --apply

# Atomically add, update, or remove timeline resources. apply-edits cannot create cuts.
tella-pp-cli videos apply-edits vid_abc --operations '[{"type":"add_zoom","operationId":"zoom-1","clipId":"cl_xyz","zoomType":"manualZoom","startTimeMs":1000,"durationMs":1500,"scale":2}]' --dry-run
tella-pp-cli videos apply-edits vid_abc --operations '[{"type":"remove_zoom","operationId":"remove-1","clipId":"cl_xyz","zoomId":"zoom_abc"}]' --dry-run

# Export/apply a full video edit plan.
tella-pp-cli videos storyboard vid_abc --include-transcript --json > storyboard.json
tella-pp-cli videos apply-storyboard vid_abc --file storyboard.json --dry-run

# Presets, QA, and playlist-scale edit planning.
tella-pp-cli videos format vid_abc --preset shorts --dry-run
tella-pp-cli videos audit vid_abc --json
tella-pp-cli playlists edit-pass plst_42 --remove-fillers --remove-buffers --trim-edges --dry-run
```

Commands default to dry-run/plan mode when they perform compound edits; pass each command's `--apply` flag where available, or drop global `--dry-run`, only after reviewing the emitted plan.

The CLI uses Tella's user-facing Studio Voice name, while translating it to the public API's `studioSound` field. Enabling it on a video starts enhanced-audio generation asynchronously; playback and exports use the raw audio until that version is ready. The clip setting opts an individual clip out (`false`) or re-enables it (`true`) under the video's master switch. When `--stdin` is combined with typed audio or Studio Voice flags, the typed flags are validated and override the corresponding JSON fields.

Tella's editor added whole-clip mute in August 2026, but the current public `update_clip` schema and MCP reference do not expose that toggle. The CLI therefore does not guess a mute field. Use the documented microphone/system volume overrides above when those tracks exist; setting both to `0` is not presented as equivalent to Tella's undoable UI mute.

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
