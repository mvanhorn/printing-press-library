# Riverside CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona 1: Sasha — indie podcaster with 3 years of unsearchable Riverside backlog**

Today: solo host on Riverside Pro, no Business API key. Every "grab that quote" moment means logging into riverside.com, clicking through Studio → Project → Take → Transcript pane, copy-pasting. Per-participant `.webm` masters exist only inside Riverside's player.

Weekly ritual: Tuesday-night editing prep. Pull last weekend's transcript into Descript, hand-grab guest audio out of the studio web app, queue Magic-Clips-generated reels for Instagram. Each step is a separate manual download.

Frustration: her own 3-year archive is opaque. Can't grep "the time my guest mentioned Bitcoin" across 90 episodes. Make.com and the Business API don't apply at Pro tier.

**Persona 2: Marcus — producer-of-record for 4 shows on shared Pro studios**

Today: post-production lead, not the host. Inherits takes from hosts who click "Stop Recording" and walk away. Half the time guest tracks are still uploading from bad hotel wifi when he wants to start cutting. Keeps a Google Sheet mirroring what's-ready-vs-still-processing across all four studios, manually refreshed.

Weekly ritual: Monday-morning intake. Open each studio, scan which takes finished backup, check which transcripts finished, pull the participant tracks for the ready ones, drop them into Pro Tools sessions on a shared drive.

Frustration: backup-status is per-recording — no cross-studio "what's ready right now" view. Pulling a take means 6-12 clicks: every participant's highest-quality local-record track, transcript with voice-activity timestamps for splicing, HLS manifest if a participant didn't upload a local file.

**Persona 3: Rae — social-clip operator on Magic Clips output**

Today: clip-and-distribute for a media brand. Lives in the Exports tab refreshing F5 to see when AI generation finishes. CloudFront signed URLs have a TTL — if she walks away for lunch the link rots and she has to refresh the clip detail page.

Weekly ritual: rapid-fire — each new recording fires AI-generation-status polling, then a dive into the Exports list, then per-clip page-loads to grab the CloudFront-signed URL before it expires.

Frustration: AI generation status, the export list, and the per-clip signed URL are three separate UI surfaces with no programmatic seam. Can't say "tell me the moment Project X has its Magic Clips ready AND give me fresh signed URLs for all of them."

## Candidates (pre-cut)

(See Phase 1.5 subagent return — 16 candidates total, sources labeled a/b/c/e.)

## Survivors and kills

### Survivors (10 features, all ≥7/10)

| # | Feature | Command | Score | How It Works | Persona | Evidence |
|---|---------|---------|-------|--------------|---------|----------|
| 1 | Priority-fallback grab | `grab <session-id>` | 9/10 | Tries transcript → audio tracks → HLS in order; writes first 200 to disk; exits with code naming tier reached | Sasha | Brief §User Vision: "priority order... a single command" |
| 2 | Bulk studio export | `bulk export --studio <slug> [--since <date>]` | 10/10 | Walks projects → takes → transcripts + per-participant assets + HLS manifests with resume cursor in `.runstate/` | Sasha, Marcus | Brief §Product Thesis: "no other tool automates bulk download" |
| 3 | Transcript convert | `transcripts convert <session-id> --format vtt\|srt\|txt\|json\|md` | 8/10 | Pure local transform on cached `editableWithVoiceActivity` JSON to formats Riverside doesn't expose | Sasha | Brief §Top Workflows: native formats are SRT/TXT only |
| 4 | Cross-studio readiness | `ready` | 8/10 | SQLite join `recordings.backup_status='ready'` ∧ `transcriptions.status='done'` ∧ no `uploading` participants across all synced studios | Marcus | Brief §Data Layer: per-recording only in UI |
| 5 | Signed-URL refresh | `media refresh --project <id> [--prefetch]` | 8/10 | Re-walks media + clip-assets to refresh CloudFront URLs; `--prefetch` downloads before TTL expires | Rae | Brief: CloudFront short-lived URLs |
| 6 | Transcript search | `search "<query>" [--speaker <name>] [--studio <slug>]` | 9/10 | SQLite FTS5 over cached transcription bodies; speaker filter joins to `speakers` array; emits `{session, project, line, timestamp}` | Sasha, Marcus | Brief §Data Layer: "FTS/search across transcription content" |
| 7 | Wait-until-ready | `wait <session-id> [--include transcript,assets,ai] [--timeout 30m]` | 7/10 | Polls backup-status + transcription + AI-generation-status with `cliutil.AdaptiveLimiter`; exits 0 ready, 2 timeout | Marcus, Rae | Brief: "block until tracks finish processing" |
| 8 | Magic Clips harvest | `clips harvest --project <id> [--since <date>] [--download]` | 8/10 | Gates on AI-status=ready, lists exports, refreshes each clip URL, optionally downloads | Rae | Brief §Data: AI + exports separate surfaces |
| 9 | Talktime stats | `transcripts talktime <session-id>` | 7/10 | Sums voice-activity durations per speaker; emits `{speaker, seconds, pct, longest_monologue_s, interrupts}` | Sasha, Marcus | Brief §Data Layer: voice-activity uniquely Riverside |
| 10 | Stale take detector | `stale [--threshold 24h]` | 7/10 | SQLite: recordings stuck in `uploading`/`processing`/`transcribing` past N hours; cross-studio | Marcus | Brief §Data: status least-baked-wins; stuck upload is real failure mode |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| `orphans` | Triggers monthly at best — fails weekly-use floor | `stale` |
| `transcripts diff` | No brief evidence; redo-take is rare | `transcripts convert` |
| `clips title` | LLM-dependency rule | `clips harvest` |
| `vod thumbnail` | Scope creep — requires ffmpeg | `bulk export` |
| `audit` | Monthly cadence at best | `ready` |
| `projects changes` | Subsumed by `stale` + `ready` + `bulk export` resume | `stale` |
