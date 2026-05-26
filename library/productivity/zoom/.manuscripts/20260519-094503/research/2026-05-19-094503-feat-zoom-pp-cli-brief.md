# Zoom CLI Brief

A combo CLI spanning the locally-installed Zoom desktop app (primary headline) and the Zoom REST API v2 (secondary enrichment). The local surface is offline, free, and trivially scriptable; the cloud surface adds scheduling, recording management, user provisioning, and analytics for accounts that have Server-to-Server OAuth credentials.

## API Identity
- **Domain (primary):** Zoom desktop client on macOS/Windows/Linux — driven via the `zoommtg://` / `zoomus://` URL scheme, on-disk local recordings under `~/Documents/Zoom/`, and (on macOS) AppleScript automation of the running `zoom.us` process.
- **Domain (secondary):** Zoom REST API v2 at `https://api.zoom.us/v2/` — meetings, users, webinars, cloud recordings, reports, IM/chat, groups, accounts, webhooks, metrics. The official `zoom/api` OpenAPI v2 spec covers 103 paths / 155 operations; the full live API (per the official zoom/zoom-plugin SKILL) exposes 600+ endpoints across Phone, Chat, Rooms, Docs, Whiteboard, Events, Contact Center, and Scheduler that the published spec does not yet cover.
- **Users:** Knowledge workers running back-to-back meetings; macOS power users who want keyboard/Stream-Deck control over Zoom; ops/IT teams provisioning Zoom accounts; analysts auditing meeting usage and recording archives; people who want to find a quote inside last week's recording without watching it back.
- **Data profile:**
  - Local: meeting folders (one per session) containing `.mp4` (video), `.m4a` (audio-only), `.vtt`/`.txt` (transcript/closed captions), `chat.txt`, and timeline metadata; the desktop client also writes `double_click_to_convert` placeholder files for recordings interrupted mid-conversion.
  - Cloud: meetings (upcoming/past), participants, registrants, cloud recording files (MP4/M4A/transcript/chat), user accounts and roles, dashboards (DAU/MAU, ZoomRooms, webinars), webhooks, IM channels and messages, phone calls/voicemail (where covered).

## Reachability Risk
- **Low.** `https://api.zoom.us/v2/users/me` returns the expected 401 without a token (proves reachability); no Cloudflare/WAF challenge layer. The official OpenAPI v2 spec downloads cleanly from `raw.githubusercontent.com/zoom/api/master/openapi.v2.json` (HTTP 200, 481 KB). The desktop URL scheme has been stable since 2017 and is widely used by community CLIs (Cloe, zoom-go, zoom-launcher). macOS AppleScript hooks into `zoom.us` menu items have been the standard automation surface for years (Stream Deck plugin, Alfred workflows, henrik's mute-toggle gist).
- Caveats:
  1. Joining password-protected meetings via the URL scheme works only with the unencrypted `pwd` parameter (Zoom-marketplace "encrypted_password" cannot be used here). Joining as authenticated host requires a ZAK token.
  2. The published OpenAPI spec lags the live API — Phone/Chat/Rooms/Docs/Whiteboard endpoints aren't in 103-path file. Cloud Phone/Chat commands will be added by hand against documented HTTP shapes if the user wants them.
  3. AppleScript surface only exists on macOS. Windows uses `start zoommtg://...`; Linux uses `xdg-open`. AppleScript-dependent commands will be macOS-only.

## Top Workflows
1. **"Join the meeting in this Zoom link / calendar invite right now without futzing with the browser interstitial."** Paste URL → CLI opens directly in the desktop app.
2. **"Start my next meeting / instant meeting / personal meeting room."** One command, no clicks.
3. **"Find that one thing someone said in last week's recording."** Search across local VTT transcripts in `~/Documents/Zoom/`, return matched moments with timestamps and the source recording.
4. **"Mute / unmute / leave the meeting I'm in right now from a hotkey."** macOS osascript driver.
5. **"List/manage my upcoming meetings, schedule a new one, fetch recordings from cloud."** Cloud REST commands.
6. **"Show me which scheduled meetings I have today plus who is on each, and surface conflicts."** Cross-resource compose using locally cached data.
7. **"Audit recording storage on disk — what's eating my Documents folder?"** Local recordings walk with size, age, and partial-conversion detection.

## Data Layer
- **Primary local entities (no auth):** `local_recordings` (one row per recording folder: path, meeting topic, start_time, duration, total_size, files=[mp4, m4a, vtt, chat], partial=bool), `local_transcript_segments` (one row per VTT cue: recording_id, start_ms, end_ms, speaker, text — FTS5 indexed for offline search), `saved_meetings` (name → meeting_id/password/url, user's personal bookmarks à la `tmonfre/zoom-cli`).
- **Cloud entities (auth required):** `users`, `meetings`, `past_meetings`, `webinars`, `cloud_recordings`, `cloud_recording_files`, `report_meetings`, `report_participants`, `dashboard_meetings`, `im_channels`, `im_messages`, `groups`, `accounts`, `webhooks`.
- **Sync cursor:** `users` and `meetings` paginated via `next_page_token`; cloud recordings paginated by `from`/`to` date window + `next_page_token`. Store cursor per resource.
- **FTS/search:** SQLite FTS5 on (a) `local_transcript_segments.text` for the killer offline transcript search, (b) `meetings.topic + agenda`, (c) `cloud_recordings.topic`, (d) `saved_meetings.name + notes`.

## Codebase Intelligence
- **Source:** Official `zoom/zoom-plugin` SKILL.md (the canonical Anthropic-shipped Zoom skill) and the `zoom/api` OpenAPI v2 spec; community wrappers (`prschmid/zoomus` Python, `licht1stein/pyzoom`, `GearPlug/zoom-python`) for endpoint patterns; community CLIs (`benbalter/zoom-go`, `tmonfre/zoom-cli`, `n44h/Cloe`) for join-flow ergonomics; community MCPs (`echelon-ai-labs/zoom-mcp`, `forayconsulting/zoom_transcript_mcp`, `mattcoatsworth/zoom-mcp-server`) for tool shapes.
- **Auth:** Server-to-Server OAuth 2.0 (`account_credentials` grant) — POST `https://zoom.us/oauth/token` with HTTP Basic (`client_id:client_secret`) and body `grant_type=account_credentials&account_id=<ID>`. Returns a 1-hour access token; CLI must cache and refresh. Env vars: `ZOOM_S2S_ACCOUNT_ID`, `ZOOM_S2S_CLIENT_ID`, `ZOOM_S2S_CLIENT_SECRET`.
- **Data model:** Resources are workspace-scoped (one account → many users → many meetings/webinars/recordings). UUIDs and numeric IDs both used. Recording `download_url`s need the same Bearer token (redirect-following).
- **Rate limiting:** Tiered by endpoint category (Light/Medium/Heavy/Resource-intensive). Daily caps; documented in `X-RateLimit-Type` and `X-RateLimit-Remaining` headers. 429 responses include `Retry-After`.
- **Architecture:** REST + Bearer token; cloud recordings paginated by date window; webhooks for real-time deltas. Local surface is just files + URL handler + (on macOS) accessibility-driven app automation.

## User Vision
Confirmed in briefing:
- Combo CLI spanning the local desktop app and the cloud REST API.
- **Local first, cloud second** — headline commands are `zoom start`, `zoom join <meeting-id>`, `zoom recordings local`. Cloud commands (`zoom meetings list`, `zoom recordings cloud`) are secondary.
- User did not volunteer additional vision text beyond the scope/priority choice.

## Source Priority
- **Primary:** `zoom-desktop-local` — no spec (browser-sniff not applicable; the URL scheme + osascript surfaces are publicly documented). Auth: **none**.
- **Secondary:** `zoom-cloud-api` — official Swagger 2.0 spec at `https://raw.githubusercontent.com/zoom/api/master/openapi.v2.json` (103 paths, 155 operations). Auth: **paid/account-required (Server-to-Server OAuth)**.
- **Economics:** Primary is free and fully local — no Zoom account required, no network round-trips. Secondary requires the user to provision an S2S OAuth app in their own Zoom account; CLI cloud commands gate on `ZOOM_S2S_*` env vars and emit a typed error when missing. No paid-key bleed into the headline.
- **Inversion risk:** The secondary has a clean OpenAPI spec; the primary has none. The generator's spec-first bias would silently promote `cloud` to the headline if not held in check. The build manifest in 1.5 must keep `start` / `join` / `recordings local` / `mute` / `leave` at the top of the README's Quick Start and the SKILL trigger phrases, with cloud commands grouped under a clearly labeled secondary section.

## Product Thesis
- **Name:** `zoom-pp-cli` (binary `zoom-pp-cli`, slug `zoom`, brand "Zoom").
- **Why it should exist:**
  1. Every existing Zoom CLI on GitHub does **one** thing well (join next meeting, or save bookmarks, or list recordings). None of them join the dots between local desktop control, offline transcript search, and cloud account management.
  2. There is **no CLI** that lets you grep across all your locally-recorded Zoom transcripts to find a quote — and that is a daily annoyance for anyone with a Documents/Zoom folder bigger than 10 GB.
  3. Existing MCPs are cloud-only and require S2S OAuth setup; the primary surface here works for any Zoom user with the desktop app, no account-admin setup required.
  4. Agent-native shape (`--json`, `--select`, `--dry-run`, typed exit codes, SQLite FTS) layered on top means Claude/Cursor/etc. can drive the entire Zoom surface — start a meeting, then summarize last week's recordings — in one tool, on-device.

## Build Priorities
1. **Local URL-scheme join/start commands** (`zoom start`, `zoom start instant`, `zoom join <id>`, `zoom join <url>`) — cross-platform shell-out via `open` / `xdg-open` / `start`, with parameter normalization, password support, display-name override, dry-run that prints the URL it would launch.
2. **Local recordings indexer + FTS** (`zoom recordings local sync`, `zoom recordings local list`, `zoom recordings local search <query>`) — walk `~/Documents/Zoom/`, parse meeting folders + VTT cues into SQLite, expose FTS5 search over transcripts with `--since`, `--folder`, `--select` filters.
3. **macOS app automation** (`zoom mute`, `zoom unmute`, `zoom video on|off`, `zoom leave`, `zoom status`) — osascript bridge into Zoom's Meeting menu; degrade gracefully on Linux/Windows.
4. **Saved meeting bookmarks** (`zoom saved add <name> <id> [--password]`, `zoom saved list`, `zoom saved join <name>`, `zoom saved rm`) — local SQLite, no auth needed; replaces `tmonfre/zoom-cli`.
5. **Cloud commands from the OpenAPI spec** — generated mirrors for users, meetings, webinars, recordings, reports, dashboards, IM, groups, accounts. Gated on `ZOOM_S2S_*`; typed env-var-missing error when not configured.
6. **Transcendence: cross-source compose** (`zoom today`, `zoom recordings drift`, `zoom find "<quote>"`, `zoom storage`, `zoom recordings export <id>`) — commands that only work because local + cloud + saved bookmarks live in one SQLite store.
7. **Doctor + agent-context surfaces** — `zoom doctor` reports: Zoom app installed? URL handler registered? Documents/Zoom path exists? S2S creds set? macOS accessibility permission for osascript? Token still valid?
