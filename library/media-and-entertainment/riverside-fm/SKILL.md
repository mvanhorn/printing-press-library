---
name: pp-riverside-fm
description: "The first programmatic way to download every transcript, audio track, and video from your own Riverside.com account... Trigger phrases: `download my riverside transcripts`, `grab my riverside recording`, `bulk export riverside studio`, `search riverside transcripts`, `harvest magic clips`, `use riverside-fm`, `run riverside-fm`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - riverside-fm-pp-cli
---

# Riverside — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `riverside-fm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install riverside-fm --cli-only
   ```
2. Verify: `riverside-fm-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Riverside.com makes you click through Studio → Project → Take → Transcript for every download, and locks the official API behind a custom-priced Business plan. This CLI imports your logged-in browser cookies and reaches the same internal API the web app uses, giving you priority-fallback grab, bulk studio export with resume, transcript search over your whole archive, and Magic Clips harvest with CloudFront URL refresh — features Riverside has never shipped to Pro users.

## When to Use This CLI

Use this CLI when an agent needs to programmatically access a Riverside.com account on Pro / Live / Webinar tier (i.e., no Business API key available). It's the right choice for backing up a creator's archive, batch-downloading transcripts/audio/video by date range, full-text-searching a podcast catalog, harvesting Magic Clips when AI generation finishes, or waiting on a recording to be ready before firing downstream automation. It is NOT the right choice for write operations (creating studios, inviting guests, posting webhooks) — those surfaces exist in the API but weren't exercised in this print.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 76 API entries from 76 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: cookie_session
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: create_logs — Derived from observed POST /api/logs traffic.; create_migrate — Derived from observed POST /api/v4/global-search/migrate traffic.; get_assets — Derived from observed GET /api/v4/take/{uuid}/assets traffic.; get_clip_assets — Derived from observed GET /api/v4/take/{uuid}/clip/69fcda9fba030a19ae93a63c/clip-assets traffic.; get_damienstevens_fqd4a6ckh — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-fqd4a6ckh traffic.; get_damienstevens_lmb0ml5mk — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-lmb0ml5mk traffic.; get_damienstevens_ms1to36g5 — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-ms1to36g5 traffic.; get_damienstevens_nev5txur8 — Derived from observed GET /api/v4/vod/{uuid}/damienstevens-nev5txur8 traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.; error_status_cluster: Endpoint cluster only observed error HTTP statuses.

## Command Reference

**ai** — AI/Magic features status checks

- `riverside-fm-pp-cli ai can_create_event` — Check whether the workspace can create scheduled events for a studio.
- `riverside-fm-pp-cli ai can_generate` — Check whether the workspace can run AI generation (magic clips, magic episodes) for a studio.

**clips** — Clips (Magic Clips, Magic Segments, manual edits)

- `riverside-fm-pp-cli clips get` — Get a clip with full take + clip metadata (export status, AI generation info, transcription language).
- `riverside-fm-pp-cli clips get_patches` — Get edit patches applied to a clip.

**productions** — Workspace-level productions

- `riverside-fm-pp-cli productions <productionId>` — List media board items (sound effects, intros, jingles) for a production. Includes signed CloudFront URLs.

**projects** — Riverside projects (episodes / sessions inside a studio)

- `riverside-fm-pp-cli projects ai_generation_status` — Get the AI generation status (magic clips, magic episodes) for a project.
- `riverside-fm-pp-cli projects get` — Get a single project with full metadata (title, scheduled events, AI generation status).
- `riverside-fm-pp-cli projects list_by_studio` — List projects (episodes) in a studio.
- `riverside-fm-pp-cli projects list_exports` — List exports (rendered MP4/WAV files) for a project.
- `riverside-fm-pp-cli projects list_takes` — List takes (recording sessions) in a project. Each take includes participant recordings and transcription session ID.

**recordings** — Individual recording files (per-participant audio + video)

- `riverside-fm-pp-cli recordings <recordingId>` — Get the cloud backup status for a single recording. Status values - none, processing, done.

**studios** — Riverside studios (top-level workspaces for a series of content)

- `riverside-fm-pp-cli studios get` — Get the studio overview by slug (includes production ID, members, recent activity).
- `riverside-fm-pp-cli studios get_v3` — Get the v3 studio detail by slug (legacy endpoint, returns richer config).

**takes** — Riverside takes (a single recording attempt grouping per-participant tracks + a session-level transcription)

- `riverside-fm-pp-cli takes get_assets` — Get all track assets for a take by session ID (filenames, resolution, recording status, device info).
- `riverside-fm-pp-cli takes get_clip_assets` — Get clip-specific assets for a take + clip pair.

**transcriptions** — Per-session transcripts with speaker labels and voice activity timestamps

- `riverside-fm-pp-cli transcriptions <sessionId>` — Get the editable transcript with voice activity for a take. Returns speakers, segments, timestamps. This is the raw...

**user** — Current authenticated Riverside user

- `riverside-fm-pp-cli user` — Get the current authenticated user profile (id, role, account, plan flags).

**vod** — HLS video-on-demand manifests per participant per take

- `riverside-fm-pp-cli vod <sessionId> <participantHandle>` — Get the HLS m3u8 manifest for a participant's video. Used to stream or transcode.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
riverside-fm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Bulk-export a whole studio's archive

```bash
riverside-fm-pp-cli bulk export --studio damien-stevenss-studio --since 2026-04-01 --out ./archive
```

Walks projects, takes, transcripts, asset metadata, HLS manifests; resumes if interrupted.

### Find every mention of a topic across years of podcasts

```bash
riverside-fm-pp-cli search "network effects" --json --select id,resource_type,hit
```

FTS5 across every cached transcript with snippet context.

### Wait until a take is ready, then harvest its Magic Clips

```bash
riverside-fm-pp-cli clips harvest --project 69fcda9fba030a19ae93a526 --wait --download --out ./clips
```

Blocks on AI-generation-status, then refreshes signed URLs and downloads every Magic Clip.

### Compute per-speaker talktime stats

```bash
riverside-fm-pp-cli transcripts talktime bf487406-af40-4bb4-b7f9-a6b49047b55d --json
```

Computes seconds, % of total, longest monologue, and interrupt count per speaker from voice-activity timestamps.

### Pull a clean WebVTT transcript for a take

```bash
riverside-fm-pp-cli transcripts convert bf487406-af40-4bb4-b7f9-a6b49047b55d --format vtt --out ep12.vtt
```

Converts the voice-activity JSON to WebVTT (a format Riverside's UI doesn't expose) for embedding in a web player.

## Auth Setup

Riverside Pro / Live / Webinar tiers don't get an API key — but the web app you log into uses an internal API gated only by HttpOnly session cookies. Run `riverside-fm-pp-cli auth login --chrome` once: the CLI reads `riverside_auth_access`, `riverside_auth_refresh`, `sweetsesh`, and `cloudfront_signed_url` from your local Chrome profile and reuses them. The Business API at platform.riverside.fm is NOT used by this CLI — it requires a Bearer key that Pro plans cannot issue, and rejects cookies outright.

Run `riverside-fm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  riverside-fm-pp-cli clips get mock-value --agent --select id,name,status
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
riverside-fm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
riverside-fm-pp-cli feedback --stdin < notes.txt
riverside-fm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.riverside-fm-pp-cli/feedback.jsonl`. They are never POSTed unless `RIVERSIDE_FM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RIVERSIDE_FM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
riverside-fm-pp-cli profile save briefing --json
riverside-fm-pp-cli --profile briefing clips get mock-value
riverside-fm-pp-cli profile list --json
riverside-fm-pp-cli profile show briefing
riverside-fm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `riverside-fm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add riverside-fm-pp-mcp -- riverside-fm-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which riverside-fm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   riverside-fm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `riverside-fm-pp-cli <command> --help`.
