# Novel Features Brainstorm — Zoom CLI (audit trail)

> Reprint check: prior research path is literal `none` — first print, so Pass 2(d) is skipped.
> Brief contains both `## User Vision` and `## Codebase Intelligence` sections, so (e) and (f) are in play.

## Customer model

### Persona 1: Maya — the back-to-back PM in Documents/Zoom hell

**Today (without this CLI):** Maya runs 6-9 Zoom meetings a day at a SaaS company. Recordings auto-save to `~/Documents/Zoom/`. When a stakeholder pings her in Slack three weeks later — "wait, didn't engineering commit to shipping that in Q2?" — she has to: open Finder, scroll through 40+ dated folders, guess which meeting it was, double-click an `.mp4`, scrub through 47 minutes of video, and usually give up and type "I'll check and get back to you." Her Documents folder is 38 GB and growing; she has no idea which recordings she's already exported to Google Drive vs which are local-only.

**Weekly ritual:** Mondays she triages last week's recordings — deletes the obvious throwaways, screenshots key whiteboard moments, sometimes downloads a transcript from the Zoom web UI for the one meeting that mattered. The act of finding a quote across a month of recordings is currently impossible.

**Frustration:** No offline grep across her own transcripts. The data is sitting on her laptop in `.vtt` files. There is literally no tool that lets her say "find the moment we discussed Q2 pricing" and get back `meeting-folder, 23:14, "...so we'd land at $99 for the Pro tier..."`.

### Persona 2: Dev — the macOS power user who lives in keyboard shortcuts and Stream Deck

**Today (without this CLI):** Dev has a Stream Deck with mute / unmute / leave buttons bound to a bespoke AppleScript he copy-pasted from a 2019 GitHub gist. The script breaks every time Zoom updates a menu label. He has Alfred workflows for "join the meeting on my next calendar event" that he wrote himself and never documented. When a coworker DMs him a meeting URL, he opens it in the browser, dismisses the "open in Zoom?" interstitial, dismisses the cookie banner, then finally lands in the meeting 8 seconds late.

**Weekly ritual:** Mute/unmute/video toggles fire 30+ times a day. He joins ~5 meetings/day from URLs pasted into Slack, email, or calendar.

**Frustration:** No single, maintained tool that wraps `zoommtg://`, the macOS osascript menu hooks, AND the recording layer. He's stitched together 4 different community gists, none of which know about each other.

### Persona 3: Sam — the IT ops lead provisioning Zoom for a 600-person org

**Today (without this CLI):** Sam uses a Postman collection and a half-broken Python script (`prschmid/zoomus`, last updated 2022) to bulk-create users when a new cohort onboards. Every quarter he runs a usage audit — who hasn't logged in, who has cloud recordings older than the retention policy, which webinars exceeded capacity. Each audit is a one-off Jupyter notebook that hits 8 endpoints and joins them in pandas.

**Weekly ritual:** Mondays — check the dashboard metrics endpoint, eyeball DAU. Quarterly — the audit. Daily — answer "can you give Joaquin a Zoom license?" tickets.

**Frustration:** Every existing Zoom CLI on GitHub stops at "join meeting." Nothing handles the admin surface AND the personal surface AND the recording layer. He'd kill for one tool that can do `zoom users create` and `zoom recordings cloud list --user joaquin@co.com` and have them share an auth + a SQLite cache.

### Persona 4: Riley — the analyst who needs to know "what did we actually agree to last week"

**Today (without this CLI):** Riley joins meetings as a silent observer to take notes for the leadership team. She has 12 recordings/week to comb through. Currently she uses Otter.ai for some, the Zoom cloud transcript download for others, and `grep`-on-`.vtt`-files for the local ones — three different interfaces. When the CEO asks "what did Marketing commit to in the offsite?", she has no unified way to search across local + cloud transcripts at once.

**Weekly ritual:** Pulls last week's recordings (local AND cloud), exports key passages to a shared doc, surfaces decisions/commitments to leadership.

**Frustration:** Local and cloud transcripts live in totally separate worlds. The community `zoom_transcript_mcp` only searches cloud transcripts she happens to have recently downloaded — it can't see her on-disk archive. There's no single search box.

## Candidates (pre-cut)

(see subagent original — full Pass 2 table preserved in run state)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | Find a quote across all transcripts | `zoom find "<quote>" [--speaker] [--before <s>] [--after <s>] [--source local\|cloud\|both] [--since]` | 9/10 | hand-code | FTS5 across local + cloud transcripts in one query; no tool does both layers |
| 2 | Storage audit of local recordings | `zoom storage [--by month\|topic\|partial] [--also-in-cloud]` | 8/10 | hand-code | Joins on-disk recording folders with cloud_recordings to flag safe-to-delete duplicates |
| 3 | Recording drift detector | `zoom recordings drift [--retention-days N]` | 8/10 | hand-code | Set-difference between local + cloud; flags retention-deadline cloud recordings + safe-to-clean local partials |
| 4 | Today's meeting load + conflicts | `zoom today [--with-recordings] [--since 7d]` | 7/10 | hand-code | UNION of cloud meetings + saved bookmarks + today's recordings with overlap detection |
| 5 | Bookmark from URL paste | `zoom saved add-from-url <name> <url>` | 7/10 | hand-code | Regex-parses every known Zoom URL shape (https/zoommtg/calendar-invite) into saved_meetings row |
| 6 | Schedule + bookmark in one shot | `zoom schedule <topic> --when <ts> [--duration N] [--save-as <name>]` | 6/10 | hand-code | Cloud POST + local cache write — round-trip combo no existing tool does |
| 7 | Speaker-time analytics on a recording | `zoom recordings analyze <local-or-cloud-id>` | 6/10 | hand-code | Speaker-labeled VTT cues (Zoom-specific) → per-speaker talk-time + interruption-count |
| 8 | Export a recording bundle | `zoom recordings export <id> [--with-transcript] [--with-chat] [--out <dir>]` | 5/10 | hand-code | One verb for local OR cloud — generates INDEX.md table of contents from VTT cues |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| C6 `zoom next` | Soft kill on weekly-use test; `zoom today` covers same slice + `zoom join <url>` covers URL paste case | C4 `zoom today` |
| C9 `zoom users audit` | Sam runs it quarterly, not weekly; spec-emitted `zoom reports users` covers it | spec-emitted `zoom reports users` |
| C10 `zoom doctor --fix` | Privileged-command execution risk; `doctor` already emits remediation text | absorb #26 `zoom doctor` |
| C11 `zoom captions tail` | Zoom doesn't write live captions to disk — would require UI scraping | C1 `zoom find` (post-meeting) |
| C12 `zoom webhooks tunnel` | Scope creep (app not command) + external service + auth gap | spec-emitted `zoom webhooks create` |
| C13 `zoom recordings summarize` | LLM dependency — reframe as `analyze` + `find` piped to `claude` | C7 + C1 |
| C14 `zoom recordings link` | Thin wrapper — `zoom find` already emits the `?startTime=` link | C1 `zoom find` |
| C16 `zoom week` | Same shape as `zoom today --since 7d` — verb sprawl | C4 `zoom today --since` |
