# Riverside CLI

**The first programmatic way to download every transcript, audio track, and video from your own Riverside.com account — no Business plan or API key required.**

Riverside.com makes you click through Studio → Project → Take → Transcript for every download, and locks the official API behind a custom-priced Business plan. This CLI imports your logged-in browser cookies and reaches the same internal API the web app uses, giving you priority-fallback grab, bulk studio export with resume, transcript search over your whole archive, and Magic Clips harvest with CloudFront URL refresh — features Riverside has never shipped to Pro users.

Learn more at [Riverside](https://riverside.com).

Printed by [@dstevens](https://github.com/dstevens) (Damien Stevens).

## Install

The recommended path installs both the `riverside-fm-pp-cli` binary and the `pp-riverside-fm` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install riverside-fm
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install riverside-fm --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/riverside-fm-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-riverside-fm --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-riverside-fm --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-riverside-fm skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-riverside-fm. The skill defines how its required CLI can be installed.
```

## Authentication

Riverside Pro / Live / Webinar tiers don't get an API key — but the web app you log into uses an internal API gated only by HttpOnly session cookies. Run `riverside-fm-pp-cli auth login --chrome` once: the CLI reads `riverside_auth_access`, `riverside_auth_refresh`, `sweetsesh`, and `cloudfront_signed_url` from your local Chrome profile and reuses them. The Business API at platform.riverside.fm is NOT used by this CLI — it requires a Bearer key that Pro plans cannot issue, and rejects cookies outright.

## Quick Start

```bash
# Import your Riverside session cookies from local Chrome (no password, no API key)
riverside-fm-pp-cli auth login --chrome


# Verify the cookies authenticate against /user
riverside-fm-pp-cli doctor


# Pull projects, takes, recordings, transcripts into the local SQLite store
riverside-fm-pp-cli sync


# Priority-fallback download: transcript first, then audio, then video
riverside-fm-pp-cli grab bf487406-af40-4bb4-b7f9-a6b49047b55d


# FTS over every cached transcript
riverside-fm-pp-cli search "compounding loops" --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Priority-aware downloads
- **`grab`** — Get whichever exists for a recording in priority order: transcript first, then audio tracks, then HLS video — your stated goal as a single command.

  _When an agent has a recording session ID, this is the one-shot command that gets the most useful asset that exists without polling three separate endpoints first._

  ```bash
  riverside-fm-pp-cli grab bf487406-af40-4bb4-b7f9-a6b49047b55d --agent
  ```
- **`bulk export`** — Walk every project / take / asset in a studio (or date range) and download transcripts + per-participant audio + HLS manifests with a resume cursor on `.runstate/`. The killer workflow no Pro-tier tool offers.

  _For an agent backing up a creator's archive, this is the only way to get every asset out of Riverside without 6-12 manual clicks per take._

  ```bash
  riverside-fm-pp-cli bulk export --studio damien-stevenss-studio --since 2026-04-01 --out ./archive
  ```
- **`media refresh`** — Re-walks production media + clip-assets to refresh short-lived CloudFront signed URLs; --prefetch downloads every asset body before TTL expires.

  _When an agent needs to archive a project's media assets, this command races the TTL clock instead of failing midway through a download chain._

  ```bash
  riverside-fm-pp-cli media refresh --project 69fcda9fba030a19ae93a526 --prefetch --out ./media
  ```

### Transcript intelligence
- **`transcripts convert`** — Convert Riverside's editableWithVoiceActivity JSON (speakers + voice-activity timestamps) to VTT, SRT, plain text, JSON, or speaker-grouped Markdown — formats Riverside's own UI doesn't expose.

  _When an agent needs a transcript in WebVTT for a web player or JSON for downstream NLP, this command converts the locally-cached transcript without re-hitting the API._

  ```bash
  riverside-fm-pp-cli transcripts convert bf487406-af40-4bb4-b7f9-a6b49047b55d --format vtt --out episode-12.vtt
  ```
- **`search`** — SQLite FTS5 over locally-cached transcription bodies; speaker filter joins the speakers array; output names the session, project, matched line, and timestamp.

  _Agents finding a quote or moment in a creator's backlog use this instead of opening Riverside studios one by one._

  ```bash
  riverside-fm-pp-cli search "compounding loop" --json
  ```
- **`transcripts talktime`** — From the cached voice-activity timestamps, compute seconds spoken per speaker, % of total, longest monologue, and interrupt count.

  _Agents grading interview pacing or producer-host balance use this to answer talktime questions without re-watching takes._

  ```bash
  riverside-fm-pp-cli transcripts talktime bf487406-af40-4bb4-b7f9-a6b49047b55d --json
  ```

### Production ops
- **`ready`** — List every take across all your synced studios that is fully ready: cloud backup done, transcription finished, no participant track still uploading.

  _Agents helping a producer with multiple shows need one call to see what's ready to cut — not 4 studios x N projects of manual clicking._

  ```bash
  riverside-fm-pp-cli ready --json --select studio,project,take_id,duration
  ```
- **`wait`** — Block until a take's backup, transcription, and/or AI generation are done; --include selects which facets to wait on; --timeout caps the wait. Exits 0 on ready, 2 on timeout.

  _Lets an agent pipeline depend on Riverside readiness without busy-loops or hardcoded sleeps._

  ```bash
  riverside-fm-pp-cli wait bf487406-af40-4bb4-b7f9-a6b49047b55d --include transcript,assets,ai --timeout 30m
  ```
- **`clips harvest`** — Gates on AI-generation-status=ready, lists Magic Clip exports for a project, refreshes each clip's signed URL, optionally downloads MP4s.

  _When an agent automates social-clip distribution, this is the one command to pull every Magic Clip with fresh, downloadable URLs._

  ```bash
  riverside-fm-pp-cli clips harvest --project 69fcda9fba030a19ae93a526 --download --out ./clips
  ```
- **`stale`** — Cross-studio query: which recordings have been stuck in uploading / processing / transcribing past N hours? Catches dropped guest-wifi uploads and abandoned takes.

  _Production agents catching stuck takes before the host has moved on use this to triage what needs guest re-upload._

  ```bash
  riverside-fm-pp-cli stale --days 1 --json
  ```

## Usage

Run `riverside-fm-pp-cli --help` for the full command reference and flag list.

## Commands

### ai

AI/Magic features status checks

- **`riverside-fm-pp-cli ai can_create_event`** - Check whether the workspace can create scheduled events for a studio.
- **`riverside-fm-pp-cli ai can_generate`** - Check whether the workspace can run AI generation (magic clips, magic episodes) for a studio.

### clips

Clips (Magic Clips, Magic Segments, manual edits)

- **`riverside-fm-pp-cli clips get`** - Get a clip with full take + clip metadata (export status, AI generation info, transcription language).
- **`riverside-fm-pp-cli clips get_patches`** - Get edit patches applied to a clip.

### productions

Workspace-level productions

- **`riverside-fm-pp-cli productions get_media`** - List media board items (sound effects, intros, jingles) for a production. Includes signed CloudFront URLs.

### projects

Riverside projects (episodes / sessions inside a studio)

- **`riverside-fm-pp-cli projects ai_generation_status`** - Get the AI generation status (magic clips, magic episodes) for a project.
- **`riverside-fm-pp-cli projects get`** - Get a single project with full metadata (title, scheduled events, AI generation status).
- **`riverside-fm-pp-cli projects list_by_studio`** - List projects (episodes) in a studio.
- **`riverside-fm-pp-cli projects list_exports`** - List exports (rendered MP4/WAV files) for a project.
- **`riverside-fm-pp-cli projects list_takes`** - List takes (recording sessions) in a project. Each take includes participant recordings and transcription session ID.

### recordings

Individual recording files (per-participant audio + video)

- **`riverside-fm-pp-cli recordings get_backup_status`** - Get the cloud backup status for a single recording. Status values - none, processing, done.

### studios

Riverside studios (top-level workspaces for a series of content)

- **`riverside-fm-pp-cli studios get`** - Get the studio overview by slug (includes production ID, members, recent activity).
- **`riverside-fm-pp-cli studios get_v3`** - Get the v3 studio detail by slug (legacy endpoint, returns richer config).

### takes

Riverside takes (a single recording attempt grouping per-participant tracks + a session-level transcription)

- **`riverside-fm-pp-cli takes get_assets`** - Get all track assets for a take by session ID (filenames, resolution, recording status, device info).
- **`riverside-fm-pp-cli takes get_clip_assets`** - Get clip-specific assets for a take + clip pair.

### transcriptions

Per-session transcripts with speaker labels and voice activity timestamps

- **`riverside-fm-pp-cli transcriptions get`** - Get the editable transcript with voice activity for a take. Returns speakers, segments, timestamps. This is the raw data the CLI converts to TXT/SRT/VTT/JSON output.

### user

Current authenticated Riverside user

- **`riverside-fm-pp-cli user get`** - Get the current authenticated user profile (id, role, account, plan flags).

### vod

HLS video-on-demand manifests per participant per take

- **`riverside-fm-pp-cli vod manifest`** - Get the HLS m3u8 manifest for a participant's video. Used to stream or transcode.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
riverside-fm-pp-cli clips get mock-value

# JSON for scripting and agents
riverside-fm-pp-cli clips get mock-value --json

# Filter to specific fields
riverside-fm-pp-cli clips get mock-value --json --select id,name,status

# Dry run — show the request without sending
riverside-fm-pp-cli clips get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
riverside-fm-pp-cli clips get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-riverside-fm -g
```

Then invoke `/pp-riverside-fm <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
riverside-fm-pp-cli auth login --chrome

claude mcp add riverside-fm riverside-fm-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
riverside-fm-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/riverside-fm-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "riverside-fm": {
      "command": "riverside-fm-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
riverside-fm-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/riverside-fm-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `riverside-fm-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 'User is not authenticated'** — Cookies expired — re-run `auth login --chrome` after reopening riverside.com in Chrome
- **403 on download URL after a successful list** — CloudFront signed URL TTL expired — run `media refresh --project <id>` to fetch fresh signed URLs
- **Transcription returns empty speakers[]** — Take is too short to transcribe or transcription is still processing — run `wait <session-id> --include transcript`
- **Business API 401 "The Api Key is required"** — This CLI uses the cookie-authed riverside.com surface, not the Business API. You don't need an API key. If you see this, you're hitting platform.riverside.fm by mistake — file a bug.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://riverside.com/dashboard
- Capture coverage: 76 API entries from 76 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: cookie_session
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: create_logs — Derived from observed POST /api/logs traffic.; create_migrate — Derived from observed POST /api/v4/global-search/migrate traffic.; get_assets — Derived from observed GET /api/v4/take/{uuid}/assets traffic.; get_clip_assets — Derived from observed GET /api/v4/take/{uuid}/clip/69fcda9fba030a19ae93a63c/clip-assets traffic.; get_damienstevens_fqd4a6ckh — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-fqd4a6ckh traffic.; get_damienstevens_lmb0ml5mk — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-lmb0ml5mk traffic.; get_damienstevens_ms1to36g5 — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-ms1to36g5 traffic.; get_damienstevens_nev5txur8 — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-nev5txur8 traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- error_status_cluster: Endpoint cluster only observed error HTTP statuses.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**PodInvite/riverside-api-docs**](https://github.com/PodInvite/riverside-api-docs) — YAML
- [**dlt source: riverside**](https://dlthub.com/context/source/riverside) — Python
- [**Make.com Riverside integration**](https://apps.make.com/riverside-goh4mb) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
