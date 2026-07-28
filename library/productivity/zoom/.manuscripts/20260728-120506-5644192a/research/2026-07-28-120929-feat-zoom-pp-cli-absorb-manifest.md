# Zoom CLI Absorb Manifest

> Sources catalogued: 5 competing CLIs, 5 MCP servers, 3 Python wrappers, 1 official OpenAPI v2 spec (103 paths / 155 ops), the macOS osascript surface (henrik gist + Stream Deck plugin + Alfred workflows), the `zoommtg://`/`zoomus://` URL scheme, and the local recordings filesystem layout at `~/Documents/Zoom/`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Status | Added Value |
|---|---------|-------------|--------------------|--------|-------------|
| 1 | Join meeting by ID + password | n44h/Cloe, tmonfre/zoom-cli | `zoom join <id-or-url> [--pwd] [--name]` constructs `zoommtg://` and shells out (`open`/`xdg-open`/`start`) | shipping | `--dry-run` prints URL; cross-platform; `--json`; no Zoom account needed |
| 2 | Save meeting bookmarks by name | tmonfre/zoom-cli, n44h/Cloe | `zoom saved add <name> <id> [--pwd] [--notes]`, `zoom saved list`, `zoom saved join <name>`, `zoom saved rm`, `zoom saved edit` | shipping | SQLite-backed, FTS5 search; `--json`; `--select`; agent-shaped |
| 3 | Start instant / personal meeting | yukiomoto/zoom-cli, gist scripts | `zoom start` (PMI), `zoom start instant`, `zoom start <id>` via URL scheme | shipping | Works offline once PMI cached; `--dry-run`; `--json` |
| 4 | List upcoming meetings | benbalter/zoom-go | `zoom meetings list --type upcoming` (cloud, spec-emitted) | shipping (cloud) | `--since`/`--until`; `--json`; `--select`; cached |
| 5 | List local recordings on disk | (gap — no existing CLI) | `zoom recordings local list [--since] [--folder] [--partial-only]` walks `~/Documents/Zoom/` | shipping | `--json`; `--select`; size + duration + partial-conversion flag |
| 6 | Sync local recordings → SQLite | (gap) | `zoom recordings local sync [--since]` walks folders, parses VTT cues, upserts | shipping | Idempotent; progress count; `--json` |
| 7 | List cloud recordings | yukiomoto/zoom-cli (partial), forayconsulting/zoom_transcript_mcp | `zoom recordings cloud list [--user] [--from] [--to]` (spec-emitted) | shipping (cloud) | `--json`; `--select`; paginated; SQLite cached |
| 8 | Download cloud recording / transcript / chat | forayconsulting/zoom_transcript_mcp `download_transcript` | `zoom recordings cloud download <meeting-uuid> [--type transcript\|audio\|video\|chat\|all] [--out <dir>]` | shipping (cloud) | Lands in `~/Documents/Zoom-cloud/`; `--json` status |
| 9 | Recent transcripts feed | forayconsulting/zoom_transcript_mcp `get_recent_transcripts` | `zoom recordings recent [--limit N] [--source local\|cloud\|both]` | shipping | Unified local+cloud; `--json`; `--select` |
| 10 | Search across transcripts (basic) | forayconsulting/zoom_transcript_mcp `search_transcripts` (cloud-downloaded only) | `zoom recordings search "<query>" [--limit] [--source]` returns matched VTT cues | shipping | FTS5; covers **local + cloud** in one search (vs cloud-only) |
| 11 | Mute / Unmute (macOS) | henrik gist, Alfred, Stream Deck | `zoom mute`, `zoom unmute`, `zoom mute toggle` (osascript) | shipping (macOS) | `--json` status; no-op safe when not in a meeting; `--dry-run` prints osascript |
| 12 | Start / Stop video (macOS) | Stream Deck plugin | `zoom video on`, `zoom video off`, `zoom video toggle` (osascript) | shipping (macOS) | Same safety + `--json` |
| 13 | Leave current meeting (macOS) | aaronsaray Alfred script | `zoom leave [--end]` (osascript closes meeting window; `--end` clicks End-for-all when host) | shipping (macOS) | `--json` status; `--dry-run` |
| 14 | In-meeting status probe (macOS) | henrik gist (implicit) | `zoom status` — running? in meeting? muted? video on? meeting topic? | shipping (macOS) | `--json` full struct; doctor-friendly |
| 15 | Users — list/get/create/update/delete + assistants/schedulers/permissions/settings/token/status | prschmid/zoomus, pyzoom, official spec | Spec-emitted: `zoom users …` | shipping (cloud) | SQLite-backed; `--json`; `--select`; `--dry-run` on writes |
| 16 | Meetings — list/get/create/update/delete + registrants/recordings/polls/livestream/invitation/status | prschmid/zoomus, pyzoom, spec | Spec-emitted: `zoom meetings …` | shipping (cloud) | Same |
| 17 | Past meetings + participants + polls/QA | spec | Spec-emitted: `zoom past-meetings …` | shipping (cloud) | Cross-joined with `meetings` rows |
| 18 | Webinars — full CRUD + registrants/panelists/polls/absentees/QA | spec, pyzoom | Spec-emitted: `zoom webinars …` | shipping (cloud) | Same |
| 19 | Reports — hosts/meetings/participants/webinars/phone/signin-out/operationlogs | spec | Spec-emitted: `zoom reports …` | shipping (cloud) | Same |
| 20 | Dashboards / metrics (meetings, webinars, zoom rooms, IM, client feedback) | spec | Spec-emitted: `zoom metrics …` | shipping (cloud) | Same |
| 21 | Groups + members + settings | spec | Spec-emitted: `zoom groups …` | shipping (cloud) | Same |
| 22 | Accounts (master + sub) + lock-settings/managed-domains/trusted-domains | spec | Spec-emitted: `zoom accounts …` | shipping (cloud) | Same |
| 23 | IM chat groups / sessions / messages | spec | Spec-emitted: `zoom im …` | shipping (cloud) | Same |
| 24 | Webhooks list/create/delete | spec | Spec-emitted: `zoom webhooks …` | shipping (cloud) | Same |
| 25 | H323 devices | spec | Spec-emitted: `zoom h323 …` | shipping (cloud) | Same |
| 26 | Doctor / health check | universal CLI pattern | `zoom doctor` checks: Zoom app installed; URL handler registered; Documents/Zoom path exists; S2S OAuth env vars set; access token valid; macOS accessibility permission for osascript | shipping | `--json`; specific remediation per failure |
| 27 | Auth: S2S OAuth set-token + refresh + status | pyzoom, zoomus | `zoom auth set-token`, `zoom auth status`, `zoom auth refresh` | shipping | Token cached locally; auto-refresh on 401 |

## Transcendence (only possible with our approach)

Reprint 2026-07-28 under Press 4.29.0. Subagent adversarial cut kept 11; user overrode to keep all prior features (drops rejected at Phase 1.5 gate). Full audit trail: 2026-07-28-120929-novel-features-brainstorm.md. All rows hand-code.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| T1 | Find a quote everywhere | `zoom find "<q>" [--speaker] [--source local\|cloud\|notes\|both] [--since]` | hand-code | FTS5 across local VTT + cloud transcripts + ingested/synced notes in one query; timestamped deep links. | Use this command to search transcript and notes text. Do NOT use it to list recording files; use 'recordings local list'. |
| T2 | Storage audit | `zoom storage [--by month\|topic\|partial] [--also-in-cloud]` | hand-code | Joins on-disk recording folders with cloud_recordings to flag safe-to-delete duplicates and partials. | none |
| T3 | Recording drift detector | `zoom recordings drift [--retention-days N]` | hand-code | Set-difference local vs cloud meeting IDs; retention-deadline + partial flags. (user-kept) | Use this command for local-vs-cloud recording mismatch/retention checks. Do NOT use it for disk-size cleanup; use 'storage --also-in-cloud'. |
| T4 | Today + conflicts | `zoom today [--with-recordings]` | hand-code | UNION of cloud meetings, saved bookmarks, today's recordings with interval-overlap conflicts. | Use this command for a one-screen today view with conflicts. Do NOT use it to enumerate future meetings; use 'meetings list --type upcoming'. |
| T5 | Bookmark from URL paste | `zoom saved add-from-url <name> <url>` | hand-code | Regex-parses every Zoom URL shape into saved_meetings in one step. | none |
| T6 | Schedule + bookmark | `zoom schedule <topic> --when <ts> [--save-as <name>]` | hand-code | POST /users/me/meetings then local bookmark insert; offline join thereafter. | none |
| T7 | Speaker-time analytics | `zoom recordings analyze <id>` | hand-code | Per-speaker talk-seconds, longest monologue, interruptions from VTT cues. | Use this command for per-speaker talk-time math. Do NOT use it to search what was said; use 'find'. |
| T8 | Export a recording bundle | `zoom recordings export <id> [--out <dir>]` | hand-code | Local-first/cloud-fallback bundle mp4+vtt+chat+INDEX.md TOC. (user-kept) | none |
| T9 | Join next meeting | `zoom next [--dry-run]` | hand-code | NEW: fetch soonest upcoming meeting, launch via zoommtg:// (print-by-default). | Use this command to join the next scheduled meeting. Do NOT use it for a known ID/URL ('join') or bookmark ('saved join <name>'). |
| T10 | Notes web portal opener | `zoom notes web [meeting-id]` | hand-code | Opens zoom.us/notes (print-by-default, --launch opt-in). (user-kept) | none |
| T11 | AI Companion summary | `zoom notes summary <uuid>` | hand-code | Documented endpoint absent from spec; real client call (// pp:client-call). S2S OAuth. | Use this command for the AI Companion summary of one meeting. Do NOT use it to search notes; use 'find --source notes'. |
| T12 | AI Companion transcript | `zoom notes transcript <uuid>` | hand-code | Documented endpoint absent from spec (// pp:client-call). S2S OAuth. (user-kept) | Use this command for the AI Companion REST transcript (S2S OAuth). Do NOT use it with a Zoom web session; use 'notes docs transcript'. |
| T13 | Ingest Notes export | `zoom notes ingest <pdf-or-docx>` | hand-code | PDF/DOCX parse into FTS5 notes table. | none |
| T14 | Notes search + todos | `zoom notes search "<q>"`, `zoom notes todos [--since 7d]` | hand-code | FTS5 search + regex action-item extraction over ingested/synced notes. | Use 'notes todos' to extract action-item lines. Do NOT use it for free-text search; use 'notes search' or 'find --source notes'. |
| T15 | My Notes via web session (Chrome auth) | `zoom notes docs list`, `zoom notes docs transcript [id]`, `zoom notes docs sync` | hand-code | USER'S POST-PUBLISH AMEND, carried forward verbatim intent: reads My Notes + transcripts using the user's own Zoom web-session cookies (press-auth capture path; no S2S OAuth, no admin). docs sync indexes into notes table so notes search/todos work. | Use this command family to read My Notes with your Zoom web session. Do NOT use it for the S2S-OAuth REST path; use 'notes summary'/'notes transcript'. |
## Hand-code commitment

- **Absorbed:** 27 features. 14 shipping locally (no auth), 13 spec-emitted (cloud, gated on S2S OAuth env vars). Generator auto-emits the 13 cloud commands from the spec.
- **Transcendence:** 9 feature groups. All `hand-code` (~50-200 LoC each plus `root.go` wiring). The "My Notes" group (T9) is the largest at ~400 LoC since it bundles 6 subcommands. Total post-generate Go to author by hand: ~1,200-1,800 LoC.
- **macOS osascript bridge** (commands 11-14): hand-authored Go subprocess pattern shelling out to `osascript -e '...'`. ~150 LoC of helper code reused across all four commands. Tagged macOS-only with graceful "not supported on $GOOS" error on other platforms.
- **PDF/DOCX parsing** for T9 (`notes ingest`): pure-Go libraries `github.com/ledongthuc/pdf` (PDF) and `baliance.com/gooxml` or `github.com/lukasjarosch/go-docx` (DOCX). No CGo, no external converters.

## Stubs / known limitations

- **Phone API, Team Chat (V2), Rooms, Docs, Whiteboard, Events, Contact Center, Scheduler, Calendar, Mail, Tasks, Clips** endpoints are documented at developers.zoom.us but **not** in the OpenAPI v2 spec. Out of scope for v1. User explicitly chose "Use OpenAPI spec as-is" in Phase 1.5. If needed later, they can be added as hand-authored endpoint definitions.
- **Start-as-host via ZAK token from the URL scheme** is supported by the URL-scheme builder, but the ZAK token must be fetched via the cloud `/users/{userId}/token?type=zak` endpoint first (cloud-gated). Without S2S OAuth, the start-as-host path falls back to opening the meeting URL (Zoom client handles auth itself).
- **macOS osascript commands** (mute/unmute/video/leave/status) require macOS + accessibility permission for the Terminal/your shell. Doctor surfaces this; the commands return a typed error on Linux/Windows.
- **`zoom recordings cloud download` is gated on S2S OAuth.** Live smoke testing of the download path will be skipped in Phase 5 (user declined to provide credentials).
