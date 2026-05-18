# Tella CLI Absorb Manifest

## Absorbed (51 OpenAPI operations as 1:1 Cobra commands)

The generator emits these directly from the OpenAPI spec; no per-command rows enumerated below. Resource-grouped summary:

| Resource | Operations | Generated commands |
|----------|-----------|-------------------|
| **Playlists** | 7 | `playlists list`, `playlists get`, `playlists create`, `playlists update`, `playlists delete`, `playlists add-video`, `playlists remove-video` |
| **Videos** | 8 | `videos list`, `videos get`, `videos update`, `videos delete`, `videos duplicate`, `videos exports`, `videos thumbnail`, `videos collaborators add` |
| **Clips** | 11 read/mutate | `clips list/get/update/delete`, `clips cut`, `clips duplicate`, `clips reorder`, `clips remove-fillers`, `clips silences`, `clips sources list`, `clips source thumbnail`, `clips source waveform`, `clips thumbnail`, `clips transcript cut`, `clips transcript uncut` |
| **Clip effects** | 16 (CRUD × 4 effect types) | `clips blurs list/add/update/remove`, `clips highlights list/add/update/remove`, `clips layouts list/add/update/remove`, `clips zooms list/add/update/remove` |
| **Webhooks** | 5 | `webhooks endpoint create/delete`, `webhooks endpoint secret`, `webhooks messages list`, `webhooks messages get` |

All absorbed operations get `--json`, `--select`, typed exit codes, dry-run on mutations, and MCP tool exposure for free via the generator. Read-only operations get `mcp:read-only: true` annotations automatically (`GET` → read-only, `DELETE` → destructive, etc.).

**Differentiation vs Tella's official MCP server:** every absorbed operation is also CLI-callable, scriptable, and pipeable. The MCP is agent-only; this binary is both human + agent.

## Transcendence (only possible with our approach)

Persona key: SAS=Sasha (sales SDR), FIO=Fiona (founder), SAM=Sam (support engineer), CAM=Cam (creator)

| # | Feature | Command | Score | How It Works | Evidence | Persona |
|---|---------|---------|-------|--------------|----------|---------|
| 1 | Cross-video transcript search | `transcripts search <query>` | 9/10 | FTS5 index over cached `clips.transcript_cut.text` populated by `sync`; returns video_id + clip_id + matching timecode rows. Offline, sub-millisecond. | Brief Data Layer "FTS5 winner"; Product Thesis differentiation #1 vs official MCP | SAM |
| 2 | Watch-milestone digest | `videos viewed --since 7d --milestone 75` | 8/10 | Reads cached `webhook_messages` rows where `event = View milestone reached` and `data.milestone >= N`, joins `videos` for title; window filter on `received_at` | Brief webhook event list; Sasha persona ritual | SAS |
| 3 | Webhook tail + replay | `webhooks tail`, `webhooks replay <msg-id>` | 8/10 | Polls real `GET /webhooks/messages` inbox, streams new entries to stdout; `replay` re-POSTs a stored message body to a local URL with reproduced HMAC headers (using endpoint signing secret) | Brief Build Priorities P2; Product Thesis "no ngrok needed for dev" | CAM |
| 4 | Bulk standard edit pass | `clips edit-pass --playlist <id> --remove-fillers --trim-silences-gt 1s` | 8/10 | For each clip in playlist: real `POST clips/{id}/remove-fillers`, real `GET clips/{id}/silences`, real `POST clips/{id}/cut` for ranges over threshold; `--dry-run` prints planned mutations | Brief Build Priorities P2; Fiona persona weekly repetition | FIO |
| 5 | Transcript diff (cut vs uncut) | `clips transcript-diff <clip-id>` | 7/10 | Calls real `GET clips/{id}/transcript/cut` and `GET clips/{id}/transcript/uncut`, diffs token streams, returns removed-segment list with timecodes | Brief: cut/uncut pair is unique Tella content shape | FIO, SAM |
| 6 | Exports waitlist | `exports wait --video <id>... --timeout 10m` | 7/10 | Real `POST videos/{id}/exports` for each, then poll status; short-circuits when `webhook_messages` shows `Export ready` for the export_id | Brief webhook events include Export ready; P2 named explicitly | FIO, CAM |
| 7 | Caption-file export | `clips captions <clip-id> --format srt|vtt` | 7/10 | Calls real `GET clips/{id}/transcript/cut`, deterministically formats SRT/VTT cues from word/segment timecodes | Cam/Sasha need caption files for embed/sequence workflows | CAM, SAS |
| 8 | Workspace stats | `workspace stats` | 6/10 | Local SQLite aggregate over `videos`, `clips`, `transcripts`, `webhook_messages`; counts, total duration, words, exports, by month | Brief Build Priorities P2; Local-first pillar | FIO, CAM |

### Killed candidates (audit trail)
- Silence atlas — subsumed by `clips edit-pass`
- Effect-preset library — scope creep
- Stale detector — monthly, not weekly
- Viewer-engagement compare — sibling overlap with milestone digest
- Reorder by transcript topic — niche + LLM-adjacent
- Webhook signature verifier — once-per-integration, not weekly
- AI chapter markers / sentiment trend — LLM dependency, no mechanical fallback
