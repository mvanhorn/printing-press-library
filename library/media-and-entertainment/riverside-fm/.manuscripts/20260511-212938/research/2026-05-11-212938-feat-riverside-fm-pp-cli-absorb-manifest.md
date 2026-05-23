# Riverside CLI — Absorb Manifest

> Run: 20260511-212938 · API slug: riverside-fm · Tier-target: Pro (cookie auth via riverside.com/api/v4)

## Absorbed (parity with every existing tool and the Riverside web app)

The competitive landscape is thin at the Pro tier: there are NO existing CLIs, MCP servers, npm/PyPI wrappers, or Zapier/Make integrations that work for non-Business users. The "competitors" we match are (a) Riverside's own web app and (b) Business-tier tools (Make.com community, dlt) that this user can't access. Every row below is a Riverside web app surface re-implemented as a CLI command using the cookie-authed v4 surface.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | Get current user | Riverside web (profile menu) | `user get` via `/user` | --json, agent-friendly account audit |
| 2 | Get studio overview by slug | Riverside web (studio sidebar) | `studios get <slug>` via `/api/v4/studio/{slug}/overview` | --json, --select, offline cache after sync |
| 3 | Get studio v3 detail | Riverside web (legacy) | `studios get-v3 <slug>` via `/api/v3/studio/{slug}` | Richer config payload, --json |
| 4 | List projects in a studio | Riverside web (Projects tab) | `projects list-by-studio <slug>` via `/api/v4/projects/studio/{slug}` | --offset, --limit, --sort-by, --order, --json |
| 5 | Get project detail | Riverside web (project header) | `projects get <project-id>` via `/api/v4/projects/{id}` | --select for nested AI-generation status |
| 6 | Get project AI-generation status | Riverside web (AI banner) | `projects ai-generation-status <project-id>` via `/api/v4/projects/{id}/ai-generation-status` | --json |
| 7 | List exports for a project | Riverside web (Exports tab) | `projects list-exports <project-id>` via `/api/v4/projects/{id}/clips/exports` | --json, pagination |
| 8 | List takes in a project | Riverside web (Recordings tab) | `projects list-takes <project-id>` via `/api/v4/projects/{id}/takes` | --json, --select, full recording metadata |
| 9 | Get take assets | Riverside web (Take detail) | `takes get-assets <session-id>` via `/api/v4/take/{sessionId}/assets` | --json, per-participant track table |
| 10 | Get clip assets for a take | Riverside web (Magic Clips view) | `takes get-clip-assets <session-id> <clip-id>` via `/api/v4/take/{sessionId}/clip/{clipId}/clip-assets` | --json |
| 11 | Get recording backup status | Riverside web (Recordings row) | `recordings get-backup-status <recording-id>` via `/api/v4/recording/{id}/backup-status` | --json |
| 12 | Get clip detail | Riverside web (Clip page) | `clips get <clip-id>` via `/api/v4/clip/{id}` | --json, exports[premiere/finalcut/davinci/tracks] visible |
| 13 | Get clip patches | Riverside web (Edit history) | `clips get-patches <clip-id>` via `/api/v4/clip/{id}/patches` | --json |
| 14 | Get transcription with voice-activity | Riverside web (Transcript pane) | `transcriptions get <session-id>` via `/api/v4/transcriptions/editableWithVoiceActivity/{sessionId}` | --json, --select on speakers/segments |
| 15 | Get HLS video manifest | Riverside web (preview player) | `vod manifest <session-id> <participant-handle>` via `/api/v4/vod/{sid}/{handle}` | --output to save .m3u8 |
| 16 | List production media | Riverside web (Media board) | `productions get-media <production-id>` via `/api/v4/production/{id}/media` | --json, CloudFront URLs extracted |
| 17 | Check AI generation eligibility | Riverside web | `ai can-generate <studio-slug>` via `/api/v4/recording-ai/{slug}/can-generate` | --json |
| 18 | Check event-creation eligibility | Riverside web | `ai can-create-event <studio-slug>` via `/api/v4/studio/{slug}/event/can-create-event` | --json |
| 19 | Cookie session import from Chrome | (no alternative — Pro tier has no API key) | `auth login --chrome` imports `riverside_auth_access`, `riverside_auth_refresh`, `sweetsesh`, `cloudfront_signed_url` from local Chrome profile | First programmatic auth for non-Business Riverside users; no API key needed |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Priority-fallback grab | `grab <session-id>` | 9/10 | The user's stated goal as one command: try transcript first, then audio tracks, then HLS video; write whichever exists; exit code names the tier reached |
| 2 | Bulk studio export | `bulk export --studio <slug> [--since <date>]` | 10/10 | Walks every project / take / asset with resume cursor in `.runstate/`; the killer workflow no Pro-tier tool offers |
| 3 | Transcript reformatter | `transcripts convert <session-id> --format vtt\|srt\|txt\|json\|md` | 8/10 | Convert the cached voice-activity JSON to formats Riverside doesn't expose (VTT, JSON, speaker-grouped Markdown — Riverside UI only emits SRT/TXT) |
| 4 | Cross-studio readiness | `ready` | 8/10 | Local SQLite join `recordings.backup_status='ready'` ∧ `transcriptions.status='done'` ∧ no `uploading` participants across all synced studios — Riverside's UI surfaces this only per-recording |
| 5 | Signed-URL refresh | `media refresh --project <id> [--prefetch]` | 8/10 | Re-walks media + clip-assets to refresh CloudFront URLs; `--prefetch` downloads before TTL expires; nothing in Riverside's UI exposes URL age |
| 6 | Transcript full-text search | `search "<query>" [--speaker <name>] [--studio <slug>]` | 9/10 | SQLite FTS5 over cached transcription bodies; the only way to grep your own podcast catalog |
| 7 | Wait-until-ready | `wait <session-id> [--include transcript,assets,ai] [--timeout 30m]` | 7/10 | Polls backup-status + transcription + AI-generation-status with adaptive backoff; exit 0 ready / 2 timeout |
| 8 | Magic Clips harvest | `clips harvest --project <id> [--since <date>] [--download]` | 8/10 | Gates on `ai-generation-status=ready`, lists exports, refreshes each clip's signed URL, optionally downloads — combines 3 separate UI surfaces |
| 9 | Talktime stats | `transcripts talktime <session-id>` | 7/10 | Sums voice-activity durations per speaker; emits `{speaker, seconds, pct, longest_monologue_s, interrupts}` — Riverside captures this data but never surfaces it as metrics |
| 10 | Stale take detector | `stale [--threshold 24h]` | 7/10 | SQLite query: recordings stuck in `uploading`/`processing`/`transcribing` past N hours, cross-studio — catches the dropped-guest-wifi failure mode |

## Stubs (none)

Every shipping-scope row above is implementable from the captured + validated surface; no stubs planned.
