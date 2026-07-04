---
name: pp-zoom
description: "The first Zoom CLI that joins your local desktop app, your on-disk recordings, and your cloud account into one... Trigger phrases: `join the zoom meeting`, `start my zoom`, `mute zoom`, `what zoom meetings do i have today`, `find that quote from last week's zoom`, `what's in my documents/zoom folder`, `schedule a zoom meeting`, `search my zoom recordings`, `use zoom`, `run zoom`, `extract todos from my zoom notes`, `search my zoom notes`, `ingest zoom notes pdf`, `open zoom notes`."
author: "Jacken"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zoom-pp-cli
---

# Zoom — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zoom-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zoom --cli-only
   ```
2. Verify: `zoom-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zoom/cmd/zoom-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs to reach across the user's Zoom surface holistically — joining the right meeting now, surfacing what's on the calendar today, finding what was said in a past meeting, or managing the cloud account. Particularly powerful when local desktop recordings and cloud-recorded meetings need to be queried together. Not the right pick for embedding live Zoom video into a custom app (use the Meeting SDK) or for hosting bot participants (use Recall.ai or the Meeting SDK).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`find`** — Search every locally-recorded and cloud-recorded Zoom transcript at once, with speaker filter, context windows, and a clickable deep link back to the exact second.

  _When you need to recover a specific commitment or decision from a meeting, you don't know in advance whether it was recorded locally or to the cloud. One search box._

  ```bash
  zoom-pp-cli find "q2 pricing" --source both --speaker "Maya" --after 30 --json
  ```
- **`storage`** — Group everything under ~/Documents/Zoom/ by month, topic, or partial-conversion status; cross-check against the cloud recordings list to flag duplicates safe to delete locally.

  _When the Documents folder is the biggest thing on the laptop, agents need to surface which gigabytes are safely reclaimable._

  ```bash
  zoom-pp-cli storage --by month --also-in-cloud --json
  ```
- **`recordings drift`** — Set-difference between local and cloud recordings; flags cloud recordings approaching the org retention deadline and local double_click_to_convert partials whose cloud version is already complete.

  _Cloud retention silently deletes recordings. Local partials silently fail. The agent needs to surface both before they bite._

  ```bash
  zoom-pp-cli recordings drift --retention-days 90 --json
  ```
- **`recordings analyze`** — Per-speaker total talk-seconds, longest monologue, and cue-overlap interruption count, computed from VTT cue timestamps and speaker labels.

  _When the agent is asked 'who dominated the meeting' or 'did everyone get a chance to speak,' it needs the answer without an LLM transcription pass._

  ```bash
  zoom-pp-cli recordings analyze meeting-2026-05-12-1400 --json --select per_speaker.name,per_speaker.talk_seconds,per_speaker.longest_monologue_sec
  ```

### Agent-native composition
- **`today`** — One screen: every meeting on your calendar today, every saved bookmark scheduled for today, every recording made today, and any overlapping intervals flagged as conflicts.

  _When the agent is asked 'what's on my plate today and is anything double-booked,' it needs the answer in one call._

  ```bash
  zoom-pp-cli today --with-recordings --json --select topic,start_time,join_url,conflict_with
  ```
- **`saved add-from-url`** — Paste any Zoom URL shape (https://zoom.us/j/<id>?pwd=, zoommtg://, calendar-invite formats) and the CLI extracts ID + unencrypted password into a named saved bookmark in one step.

  _URLs land in Slack/email constantly. Closing the parse-it-yourself gap turns 'save this for later' into one command._

  ```bash
  zoom-pp-cli saved add-from-url team-standup "https://us02web.zoom.us/j/85123456789?pwd=abc" --json
  ```

### Cross-source round-trips
- **`schedule`** — Create a cloud meeting (POST /users/me/meetings) and immediately persist the resulting ID + password into local saved_meetings, so future `zoom saved join <name>` works offline.

  _Scheduling and joining are usually two separate tools. Pairing them lets agents create-then-recall meetings without re-querying the cloud._

  ```bash
  zoom-pp-cli schedule "Q3 Planning" --when "2026-08-12T14:00:00Z" --duration 60 --save-as q3-planning --json
  ```
- **`recordings export`** — Resolve a recording ID against local first, fall back to cloud (downloading if needed); package mp4 + vtt + chat.txt + a generated INDEX.md with timestamped table of contents into one folder.

  _When the agent is asked to 'pull together everything we have on Tuesday's planning call,' it needs one verb that doesn't care whether the source is local or cloud._

  ```bash
  zoom-pp-cli recordings export meeting-2026-05-12-1400 --with-transcript --with-chat --out ~/Drive/q2-planning --json
  ```

### My Notes integration
- **`notes web`** — Open https://zoom.us/notes (optionally scoped to a meeting) in the user's default browser — the only path to the live Notes UI since Zoom has no public REST endpoint for the My Notes feature.

  _When an agent needs to send the user to their notes, this is the single command that always works regardless of auth state._

  ```bash
  zoom-pp-cli notes web --json --dry-run
  ```
- **`notes summary`** — Fetches Zoom AI Companion's auto-generated meeting summary for a meeting UUID via the documented `/meetings/{uuid}/meeting_summary` endpoint (S2S OAuth gated).

  _When an agent needs the canonical post-meeting recap without manually exporting a PDF._

  ```bash
  zoom-pp-cli notes summary abc123== --json --select summary_title,summary_overview,summary_details
  ```
- **`notes transcript`** — Fetches the AI Companion full transcript for a meeting UUID via the documented `/meetings/{uuid}/transcript` endpoint (S2S OAuth gated).

  _Agents that need to verify a summary or quote can pull verbatim transcript without opening the web portal._

  ```bash
  zoom-pp-cli notes transcript abc123== --json
  ```
- **`notes ingest`** — Parses a Notes PDF or DOCX exported from the Zoom web portal, extracts text + meeting metadata + headings, indexes them in a local SQLite `notes` table (FTS5 enabled).

  _Lets agents build a searchable, persistent corpus of the user's meeting notes from manual exports._

  ```bash
  zoom-pp-cli notes ingest ~/Downloads/zoom-notes-2026-05-12.pdf --json --select meeting_topic,note_count,word_count
  ```
- **`notes search`** — FTS5 query across every Notes file that has been ingested, returns `meeting_topic + note_excerpt + source_file + match_offset` with optional `--since` / `--meeting-id` filters.

  _Agents asked 'what did I write down about X' can answer instantly from the ingested corpus._

  ```bash
  zoom-pp-cli notes search "q2 launch plan" --since 30d --json --select meeting_topic,start_time,note_excerpt,source_file
  ```
- **`notes todos`** — Scans ingested notes for action-item patterns (`TODO:`, `Action:`, `[ ]`, `- [ ]`, `Action Item:`, `Next:`, `Follow up:`, `Owner:`) and emits a structured to-do list with source meeting topic + date + checkbox state.

  _Turns the My Notes archive into a queryable backlog — the killer feature for agents that need to surface 'what do I still owe people from last week's meetings.'_

  ```bash
  zoom-pp-cli notes todos --since 7d --json --select text,meeting_topic,start_time,owner,checked
  ```

## Command Reference

**accounts** — Account operations

- `zoom-pp-cli accounts account` — Retrieve a sub account under the master account. <aside>Your account must be a master account and have this...
- `zoom-pp-cli accounts accounts` — List all the sub accounts under the master account
- `zoom-pp-cli accounts create` — Create a sub account under the master account. <aside>Your account must be a master account and have this privilege...
- `zoom-pp-cli accounts disassociate` — Disassociate a sub account from the master account. This will leave the account intact but the sub account will not...

**groups** — Group operations

- `zoom-pp-cli groups create` — Create a group under your account
- `zoom-pp-cli groups delete` — Delete a group under your account
- `zoom-pp-cli groups group` — Retrieve a group under your account
- `zoom-pp-cli groups groups` — List groups under your account
- `zoom-pp-cli groups update` — Update a group under your account

**h323** — Manage h323

- `zoom-pp-cli h323 device-create` — Create a H.323/SIP Device on your Zoom account
- `zoom-pp-cli h323 device-delete` — Delete a H.323/SIP Device on your Zoom account
- `zoom-pp-cli h323 device-list` — List H.323/SIP Devices on your Zoom account.
- `zoom-pp-cli h323 device-update` — Update a H.323/SIP Device on your Zoom account

**im** — Manage im

- `zoom-pp-cli im chat-messages` — Retrieve IM Chat messages for a specified period <aside>This API only supports oauth2.</aside>
- `zoom-pp-cli im chat-sessions` — Retrieve IM Chat sessions for a specified period <aside>This API only supports oauth2.</aside>
- `zoom-pp-cli im group` — Retrieve an IM Group under your account
- `zoom-pp-cli im group-create` — Create a IM Group under your account
- `zoom-pp-cli im group-delete` — Delete an IM Group under your account
- `zoom-pp-cli im group-members` — List an IM Group's members under your account
- `zoom-pp-cli im group-members-create` — Add members to an IM Group under your account
- `zoom-pp-cli im group-members-delete` — Delete a member from an IM Group under your account
- `zoom-pp-cli im group-update` — Update an IM Group under your account
- `zoom-pp-cli im groups` — List IM groups under your account

**meetings** — Meeting operations

- `zoom-pp-cli meetings delete` — Delete a meeting
- `zoom-pp-cli meetings meeting` — Retrieve a meeting's details
- `zoom-pp-cli meetings update` — Update a meeting's details

**metrics** — Manage metrics

- `zoom-pp-cli metrics dashboard-crc` — Get CRC Port usage hour by hour for a specified time period <aside class='notice'>We will report a maximum of one...
- `zoom-pp-cli metrics dashboard-im` — Retrieve metrics of Zoom IM
- `zoom-pp-cli metrics dashboard-meeting-detail` — Retrieve live or past meetings detail
- `zoom-pp-cli metrics dashboard-meeting-participant-qos` — Retrieve live or past meetings participant quality of service
- `zoom-pp-cli metrics dashboard-meeting-participant-share` — Retrieve sharing/recording details of live or past meetings participant
- `zoom-pp-cli metrics dashboard-meeting-participants` — Retrieve live or past meetings participants
- `zoom-pp-cli metrics dashboard-meeting-participants-qos` — Retrieve list of live or past meetings participants quality of service
- `zoom-pp-cli metrics dashboard-meetings` — List live meetings or past meetings for a specified period
- `zoom-pp-cli metrics dashboard-webinar-detail` — Retrieve live or past webinars detail
- `zoom-pp-cli metrics dashboard-webinar-participant-qos` — Retrieve live or past webinar participant quality of service
- `zoom-pp-cli metrics dashboard-webinar-participant-share` — Retrieve sharing/recording details of live or past webinar participant
- `zoom-pp-cli metrics dashboard-webinar-participants` — Retrieve live or past webinar participants
- `zoom-pp-cli metrics dashboard-webinar-participants-qos` — Retrieve list of live or past webinar participants quality of service
- `zoom-pp-cli metrics dashboard-webinars` — List live webinars or past webinars for a specified period
- `zoom-pp-cli metrics dashboard-zoom-room` — Retrieve zoom room on account
- `zoom-pp-cli metrics dashboard-zoom-rooms` — List all zoom rooms on account

**past-meetings** — Manage past meetings

- `zoom-pp-cli past-meetings <meetingUUID>` — Retrieve ended meeting details

**past-webinars** — Manage past webinars


**report** — Report operations

- `zoom-pp-cli report cloud-recording` — Retrieve cloud recording usage report for a specified period. You can only get cloud recording reports for the most...
- `zoom-pp-cli report daily` — Retrieve daily report for one month, can only get daily report for recent 6 months
- `zoom-pp-cli report meeting-details` — Retrieve ended meeting details report
- `zoom-pp-cli report meeting-participants` — Retrieve ended meeting participants report
- `zoom-pp-cli report meeting-polls` — Retrieve ended meeting polls report
- `zoom-pp-cli report meetings` — Retrieve ended meetings report for a specified period
- `zoom-pp-cli report telephone` — Retrieve telephone report for a specified period <aside>Toll Report option would be removed.</aside>.
- `zoom-pp-cli report users` — Retrieve active or inactive hosts report for a specified period
- `zoom-pp-cli report webinar-details` — Retrieve ended webinar details report
- `zoom-pp-cli report webinar-participants` — Retrieve ended webinar participants report
- `zoom-pp-cli report webinar-polls` — Retrieve ended webinar polls report
- `zoom-pp-cli report webinar-qa` — Retrieve ended webinar Q&A report

**tracking-fields** — Tracking Field operations

- `zoom-pp-cli tracking-fields create` — Create a Tracking Field on your Zoom account
- `zoom-pp-cli tracking-fields delete` — Delete a Tracking Field on your Zoom account
- `zoom-pp-cli tracking-fields get` — Retrieve a tracking field
- `zoom-pp-cli tracking-fields list` — List Tracking Fields on your Zoom account.
- `zoom-pp-cli tracking-fields update` — Update a Tracking Field on your Zoom account

**tsp** — TSP operations

- `zoom-pp-cli tsp tsp` — Retrieve TSP information on account level
- `zoom-pp-cli tsp update` — Update TSP information on account level

**users** — User operations

- `zoom-pp-cli users create` — Create a user on your account
- `zoom-pp-cli users delete` — Delete a user on your account
- `zoom-pp-cli users email` — Check if the user email exists
- `zoom-pp-cli users update` — Update a user on your account
- `zoom-pp-cli users user` — Retrieve a user on your account
- `zoom-pp-cli users users` — List users on your account
- `zoom-pp-cli users vanity-name` — Check if the user's personal meeting room name exists
- `zoom-pp-cli users zpk` — Check if the zpk is expired. The zpk is used to authenticate a user.

**webhooks** — Webhook operations

- `zoom-pp-cli webhooks create` — Create a webhook for a account
- `zoom-pp-cli webhooks delete` — Delete a webhook
- `zoom-pp-cli webhooks switch` — Switch webhook version
- `zoom-pp-cli webhooks update` — Update a webhook
- `zoom-pp-cli webhooks webhook` — Retrieve a webhook
- `zoom-pp-cli webhooks webhooks` — List webhooks for a account

**webinars** — Webinar operations

- `zoom-pp-cli webinars delete` — Delete a webinar
- `zoom-pp-cli webinars update` — Update a webinar
- `zoom-pp-cli webinars webinar` — Retrieve a webinar


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zoom-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find a quote from any past meeting

```bash
zoom-pp-cli find "customer churn" --source both --speaker "Riley" --after 45 --json --select recording_path,start_ms,speaker,text,deep_link
```

Searches every local + cloud transcript for the phrase, scoped to one speaker, with 45 seconds of follow-up context per match — and returns a clickable timestamped deep link.

### Triage Monday morning

```bash
zoom-pp-cli today --with-recordings --since 7d --json --select topic,start_time,join_url,conflict_with,recording_path
```

Pulls the last week of meetings + today's calendar + today's recordings in one query, flags any back-to-back overlaps, and gives the agent everything it needs to draft a weekly recap.

### Clean up Documents/Zoom safely

```bash
zoom-pp-cli storage --by month --also-in-cloud --json --select month,total_gb,safe_to_delete_gb
```

Groups local recordings by month and flags which are also in the cloud (and therefore safe to delete locally) — agents can recommend reclaimable disk without risking the only copy.

### Schedule a meeting and stash it

```bash
zoom-pp-cli schedule "Sprint planning" --when 2026-05-21T15:00:00Z --duration 60 --save-as sprint --json
```

Creates the cloud meeting and immediately writes a local bookmark, so a later `zoom-pp-cli saved join sprint` works without re-hitting the API.

### Audit speaker time on yesterday's offsite

```bash
zoom-pp-cli recordings analyze offsite-2026-05-18 --json --select per_speaker
```

Computes per-speaker talk-time, longest monologue, and interruption count from the VTT cues without an LLM call.

### Build a to-do list from your meeting notes

```bash
zoom-pp-cli notes ingest ~/Downloads/notes-q2-planning.pdf && zoom-pp-cli notes todos --since 14d --json --select text,meeting_topic,owner,checked
```

Ingest an exported Notes PDF, then extract every action-item pattern across the last two weeks of ingested notes — Zoom has no public API for My Notes, so manual export plus this pipeline is the only path.

## Auth Setup

No authentication required.

Run `zoom-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zoom-pp-cli tracking-fields list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zoom-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zoom-pp-cli feedback --stdin < notes.txt
zoom-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zoom-pp-cli/feedback.jsonl`. They are never POSTed unless `ZOOM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZOOM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zoom-pp-cli profile save briefing --json
zoom-pp-cli --profile briefing tracking-fields list
zoom-pp-cli profile list --json
zoom-pp-cli profile show briefing
zoom-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `zoom-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zoom-pp-mcp -- zoom-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zoom-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zoom-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zoom-pp-cli <command> --help`.
