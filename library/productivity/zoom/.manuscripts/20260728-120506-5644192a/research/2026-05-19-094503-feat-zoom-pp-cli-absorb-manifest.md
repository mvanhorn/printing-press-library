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

Eight features below survived the novel-features subagent's adversarial cut; full audit trail is in `2026-05-19-094503-novel-features-brainstorm.md`. Every row is `hand-code` — there is no spec-emitted shortcut for cross-layer (local + cloud + macOS app) commands.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| T1 | Find a quote across all transcripts | `zoom find "<quote>" [--speaker] [--before <s>] [--after <s>] [--source local\|cloud\|both] [--since]` | hand-code | FTS5 MATCH across local + cloud transcript segments in one query, returns `recording_path + start_ms + speaker + context_window` and emits a deep link (cloud `?startTime=`, local `vlc --start-time=`). No existing tool searches both layers. |
| T2 | Storage audit of `~/Documents/Zoom/` | `zoom storage [--by month\|topic\|partial] [--also-in-cloud]` | hand-code | Joins on-disk recording folders with `cloud_recordings` table to flag safe-to-delete duplicates and `double_click_to_convert` partials. Pure local + cross-source insight no API gives you. |
| T3 | Recording drift detector | `zoom recordings drift [--retention-days N]` | hand-code | Set-difference between `local_recordings.meeting_id` and `cloud_recordings.meeting_id`; flags cloud recordings approaching org retention deadline and local partials whose cloud version is complete. Requires both layers in SQLite. |
| T4 | Today's meeting load + conflict detection | `zoom today [--with-recordings] [--since 7d]` | hand-code | UNION of cloud meetings, saved bookmarks, today's recordings; computes overlapping intervals → conflict list with `zoommtg://` join URLs inline. No single endpoint composes all three. |
| T5 | Bookmark from URL paste | `zoom saved add-from-url <name> <url>` | hand-code | Regex-parses every known Zoom URL shape (`https://*.zoom.us/j/<id>?pwd=`, `zoommtg://zoom.us/join?confno=`, calendar-invite formats with embedded encrypted password) and extracts ID + unencrypted password into a `saved_meetings` row in one step. |
| T6 | Schedule + bookmark in one shot | `zoom schedule <topic> --when <ts> [--duration N] [--save-as <name>]` | hand-code | Wraps `POST /users/me/meetings` (cloud, spec), parses the response, inserts ID + password into local `saved_meetings`; subsequent `zoom saved join <name>` works offline forever. Cloud write + local cache round-trip. |
| T7 | Speaker-time analytics on a recording | `zoom recordings analyze <local-or-cloud-id>` | hand-code | Parses VTT cues (Zoom-specific speaker labels per cue), computes per-speaker total talk-seconds, longest monologue, cue-overlap interruption count. Mechanical computation on Zoom's unique transcript shape. |
| T8 | Export a recording bundle | `zoom recordings export <id> [--with-transcript] [--with-chat] [--out <dir>]` | hand-code | Resolves `<id>` against `local_recordings` first, falls back to `cloud_recordings` (downloading if needed); packages mp4 + vtt + chat.txt + generated `INDEX.md` (timestamped TOC derived from VTT cues) ready to drop in Drive/Notion. |
| T9 | **My Notes** — full integration stack (user-requested killer feature) | `zoom notes web [meeting-id]`, `zoom notes summary <uuid>`, `zoom notes transcript <uuid>`, `zoom notes ingest <pdf-or-docx>`, `zoom notes search "<query>"`, `zoom notes todos [--meeting-id <uuid>]` | hand-code | Zoom has **no public REST endpoint for the "My Notes" feature** (confirmed by Zoom devs in 2024 and 2025 forum threads). This row combines three honest paths: (a) `notes web` opens `https://zoom.us/notes` in the user's default browser for manual review/export; (b) `notes summary` / `notes transcript` hit the documented AI Companion endpoints (`/meetings/{uuid}/meeting_summary` and `/meetings/{uuid}/transcript`) — S2S OAuth gated; (c) `notes ingest` parses an exported Notes PDF or DOCX into SQLite, then `notes search` (FTS5) and `notes todos` (regex extraction of `TODO:`, `Action:`, `[ ]`, `- [ ]`, `Action Item:`, `Next:` patterns) produce searchable indexed content and auto-built action item lists. No existing CLI or MCP offers this end-to-end. |

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
