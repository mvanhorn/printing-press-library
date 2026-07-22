# Zoom CLI

**The first Zoom CLI that joins your local desktop app, your on-disk recordings, and your cloud account into one agent-native surface.**

Start and join meetings via the desktop URL scheme (no browser interstitial), search every transcript you've ever recorded locally or in the cloud with one query, drive mute/unmute/video/leave from the command line on macOS, and run the full Zoom REST API (users, meetings, webinars, recordings, reports, dashboards) when your Server-to-Server OAuth credentials are configured. SQLite-backed; --json, --select, --dry-run on every command; works offline for the local surface and on-disk recordings.

Learn more at [Zoom](https://developer.zoom.us/).

Created by [@Holajack](https://github.com/Holajack) (Jacken).

## Install

The recommended path installs both the `zoom-pp-cli` binary and the `pp-zoom` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zoom
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zoom --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zoom --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zoom --agent claude-code
npx -y @mvanhorn/printing-press-library install zoom --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zoom/cmd/zoom-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoom-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zoom --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zoom --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zoom --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zoom --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zoom-current).
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
    "zoom": {
      "command": "zoom-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Local commands (start, join, mute, leave, recordings local, search) require no Zoom account — just the Zoom desktop app installed. Cloud commands (meetings, users, webinars, recordings cloud, reports, metrics) need Server-to-Server OAuth credentials. Run `zoom-pp-cli auth set-token` to provide ZOOM_S2S_ACCOUNT_ID, ZOOM_S2S_CLIENT_ID, ZOOM_S2S_CLIENT_SECRET — the CLI caches the access token and auto-refreshes on 401.

## Quick Start

```bash
# Confirms the Zoom app is installed, the URL handler is registered, ~/Documents/Zoom exists, and surfaces macOS accessibility-permission gaps before they bite.
zoom-pp-cli doctor

# Walks ~/Documents/Zoom/, parses every VTT transcript into a local FTS5 index — usually a few seconds.
zoom-pp-cli recordings local sync

# The killer command: searches every local and cloud transcript at once with speaker filter and clickable timestamps.
zoom-pp-cli find "q2 pricing" --source both --json

# Composes your cloud calendar, saved bookmarks, and today's local recordings into one view with conflict detection.
zoom-pp-cli today --with-recordings --json

# Pastes any Zoom URL, opens it directly in the desktop app — no browser interstitial. --dry-run prints the zoommtg:// URL it would launch.
zoom-pp-cli join "https://zoom.us/j/85123456789?pwd=abc" --dry-run

# macOS only — flips mute via the running Zoom app's Meeting menu. Safe no-op if not in a meeting.
zoom-pp-cli mute toggle

# The My Notes pipeline: drop in an exported PDF/DOCX from zoom.us/notes, then extract action items across the last week.
zoom-pp-cli notes ingest ~/Downloads/zoom-notes-2026-05-12.pdf && zoom-pp-cli notes todos --since 7d --json

```

## Unique Features

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

## Usage

Run `zoom-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Account operations

- **`zoom-pp-cli accounts account`** - Retrieve a sub account under the master account. <aside>Your account must be a master account and have this privilege to read sub accounts. Zoom only assigns this privilege to trusted partners</aside>.
- **`zoom-pp-cli accounts accounts`** - List all the sub accounts under the master account
- **`zoom-pp-cli accounts create`** - Create a sub account under the master account. <aside>Your account must be a master account and have this privilege to create sub account. Zoom only assigns this privilege to trusted partners. The created user will not receive a confirmation email.</aside>.
- **`zoom-pp-cli accounts disassociate`** - Disassociate a sub account from the master account. This will leave the account intact but the sub account will not longer be associated with the master account.

### groups

Group operations

- **`zoom-pp-cli groups create`** - Create a group under your account
- **`zoom-pp-cli groups delete`** - Delete a group under your account
- **`zoom-pp-cli groups group`** - Retrieve a group under your account
- **`zoom-pp-cli groups groups`** - List groups under your account
- **`zoom-pp-cli groups update`** - Update a group under your account

### h323

Manage h323

- **`zoom-pp-cli h323 device-create`** - Create a H.323/SIP Device on your Zoom account
- **`zoom-pp-cli h323 device-delete`** - Delete a H.323/SIP Device on your Zoom account
- **`zoom-pp-cli h323 device-list`** - List H.323/SIP Devices on your Zoom account.
- **`zoom-pp-cli h323 device-update`** - Update a H.323/SIP Device on your Zoom account

### im

Manage im

- **`zoom-pp-cli im chat-messages`** - Retrieve IM Chat messages for a specified period <aside>This API only supports oauth2.</aside>
- **`zoom-pp-cli im chat-sessions`** - Retrieve IM Chat sessions for a specified period <aside>This API only supports oauth2.</aside>
- **`zoom-pp-cli im group`** - Retrieve an IM Group under your account
- **`zoom-pp-cli im group-create`** - Create a IM Group under your account
- **`zoom-pp-cli im group-delete`** - Delete an IM Group under your account
- **`zoom-pp-cli im group-members`** - List an IM Group's members under your account
- **`zoom-pp-cli im group-members-create`** - Add members to an IM Group under your account
- **`zoom-pp-cli im group-members-delete`** - Delete a member from an IM Group under your account
- **`zoom-pp-cli im group-update`** - Update an IM Group under your account
- **`zoom-pp-cli im groups`** - List IM groups under your account

### meetings

Meeting operations

- **`zoom-pp-cli meetings delete`** - Delete a meeting
- **`zoom-pp-cli meetings meeting`** - Retrieve a meeting's details
- **`zoom-pp-cli meetings update`** - Update a meeting's details

### metrics

Manage metrics

- **`zoom-pp-cli metrics dashboard-crc`** - Get CRC Port usage hour by hour for a specified time period <aside class='notice'>We will report a maximum of one month. For example, if "from" is set to "2017-08-05" and "to" is "2017-10-10" we will adjust "from" to "2017-09-10"</aside>.
- **`zoom-pp-cli metrics dashboard-im`** - Retrieve metrics of Zoom IM
- **`zoom-pp-cli metrics dashboard-meeting-detail`** - Retrieve live or past meetings detail
- **`zoom-pp-cli metrics dashboard-meeting-participant-qos`** - Retrieve live or past meetings participant quality of service
- **`zoom-pp-cli metrics dashboard-meeting-participant-share`** - Retrieve sharing/recording details of live or past meetings participant
- **`zoom-pp-cli metrics dashboard-meeting-participants`** - Retrieve live or past meetings participants
- **`zoom-pp-cli metrics dashboard-meeting-participants-qos`** - Retrieve list of live or past meetings participants quality of service
- **`zoom-pp-cli metrics dashboard-meetings`** - List live meetings or past meetings for a specified period
- **`zoom-pp-cli metrics dashboard-webinar-detail`** - Retrieve live  or past webinars detail
- **`zoom-pp-cli metrics dashboard-webinar-participant-qos`** - Retrieve live or past webinar participant quality of service
- **`zoom-pp-cli metrics dashboard-webinar-participant-share`** - Retrieve sharing/recording details of live or past webinar participant
- **`zoom-pp-cli metrics dashboard-webinar-participants`** - Retrieve live or past webinar participants
- **`zoom-pp-cli metrics dashboard-webinar-participants-qos`** - Retrieve list of live or past webinar participants quality of service
- **`zoom-pp-cli metrics dashboard-webinars`** - List live webinars or past webinars for a specified period
- **`zoom-pp-cli metrics dashboard-zoom-room`** - Retrieve zoom room on account
- **`zoom-pp-cli metrics dashboard-zoom-rooms`** - List all zoom rooms on account

### past-meetings

Manage past meetings

- **`zoom-pp-cli past-meetings <meetingUUID>`** - Retrieve ended meeting details

### past-webinars

Manage past webinars

### report

Report operations

- **`zoom-pp-cli report cloud-recording`** - Retrieve cloud recording usage report for a specified period. You can only get cloud recording reports for the most recent period of 6 months. The date gap between from and to dates should be smaller or equal to 30 days.
- **`zoom-pp-cli report daily`** - Retrieve daily report for one month, can only get daily report for recent 6 months
- **`zoom-pp-cli report meeting-details`** - Retrieve ended meeting details report
- **`zoom-pp-cli report meeting-participants`** - Retrieve ended meeting participants report
- **`zoom-pp-cli report meeting-polls`** - Retrieve ended meeting polls report
- **`zoom-pp-cli report meetings`** - Retrieve ended meetings report for a specified period
- **`zoom-pp-cli report telephone`** - Retrieve telephone report for a specified period <aside>Toll Report option would be removed.</aside>.
- **`zoom-pp-cli report users`** - Retrieve active or inactive hosts report for a specified period
- **`zoom-pp-cli report webinar-details`** - Retrieve ended webinar details report
- **`zoom-pp-cli report webinar-participants`** - Retrieve ended webinar participants report
- **`zoom-pp-cli report webinar-polls`** - Retrieve ended webinar polls report
- **`zoom-pp-cli report webinar-qa`** - Retrieve ended webinar Q&A report

### tracking-fields

Tracking Field operations

- **`zoom-pp-cli tracking-fields create`** - Create a Tracking Field on your Zoom account
- **`zoom-pp-cli tracking-fields delete`** - Delete a Tracking Field on your Zoom account
- **`zoom-pp-cli tracking-fields get`** - Retrieve a tracking field
- **`zoom-pp-cli tracking-fields list`** - List Tracking Fields on your Zoom account.
- **`zoom-pp-cli tracking-fields update`** - Update a Tracking Field on your Zoom account

### tsp

TSP operations

- **`zoom-pp-cli tsp tsp`** - Retrieve TSP information on account level
- **`zoom-pp-cli tsp update`** - Update TSP information on account level

### users

User operations

- **`zoom-pp-cli users create`** - Create a user on your account
- **`zoom-pp-cli users delete`** - Delete a user on your account
- **`zoom-pp-cli users email`** - Check if the user email exists
- **`zoom-pp-cli users update`** - Update a user on your account
- **`zoom-pp-cli users user`** - Retrieve a user on your account
- **`zoom-pp-cli users users`** - List users on your account
- **`zoom-pp-cli users vanity-name`** - Check if the user's personal meeting room name exists
- **`zoom-pp-cli users zpk`** - Check if the zpk is expired. The zpk is used to authenticate a user.

### webhooks

Webhook operations

- **`zoom-pp-cli webhooks create`** - Create a webhook for a account
- **`zoom-pp-cli webhooks delete`** - Delete a webhook
- **`zoom-pp-cli webhooks switch`** - Switch webhook version
- **`zoom-pp-cli webhooks update`** - Update a webhook
- **`zoom-pp-cli webhooks webhook`** - Retrieve a webhook
- **`zoom-pp-cli webhooks webhooks`** - List webhooks for a account

### webinars

Webinar operations

- **`zoom-pp-cli webinars delete`** - Delete a webinar
- **`zoom-pp-cli webinars update`** - Update a webinar
- **`zoom-pp-cli webinars webinar`** - Retrieve a webinar

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zoom-pp-cli tracking-fields list

# JSON for scripting and agents
zoom-pp-cli tracking-fields list --json

# Filter to specific fields
zoom-pp-cli tracking-fields list --json --select id,name,status

# Dry run — show the request without sending
zoom-pp-cli tracking-fields list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zoom-pp-cli tracking-fields list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
zoom-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/zoom-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`open: zoommtg://...: No application knows how to open URL`** — Run `zoom-pp-cli doctor` — it tells you whether the URL handler is registered. Reinstalling the Zoom desktop app re-registers it.
- **`mute/unmute` returns `osascript: not supported on this platform`** — These commands are macOS-only. On Linux/Windows, use the Zoom client's keyboard shortcuts directly.
- **`mute/unmute` exits 0 but nothing happens** — Grant accessibility permission to your terminal: System Settings → Privacy & Security → Accessibility → add Terminal/iTerm. `zoom-pp-cli doctor` checks this.
- **Cloud commands return `auth: ZOOM_S2S_* not set`** — Run `zoom-pp-cli auth set-token` or export `ZOOM_S2S_ACCOUNT_ID`, `ZOOM_S2S_CLIENT_ID`, `ZOOM_S2S_CLIENT_SECRET`. Get them from your Zoom App Marketplace Server-to-Server OAuth app.
- **Cloud requests return 429** — The CLI honors `Retry-After`; if you're hitting daily caps, narrow `--from`/`--to` windows or use the local cache (`--data-source local`).
- **`recordings local sync` skips files** — Skipped files are usually `double_click_to_convert` partials. Run `zoom-pp-cli recordings local list --partial-only` to see them; double-click in Finder to trigger Zoom's conversion.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**echelon-ai-labs/zoom-mcp**](https://github.com/echelon-ai-labs/zoom-mcp) — Python (26 stars)
- [**n44h/Cloe**](https://github.com/n44h/Cloe) — Python (2 stars)
- [**zoom/zoom-plugin**](https://github.com/zoom/zoom-plugin) — TypeScript
- [**forayconsulting/zoom_transcript_mcp**](https://github.com/forayconsulting/zoom_transcript_mcp) — Python
- [**mattcoatsworth/zoom-mcp-server**](https://github.com/mattcoatsworth/zoom-mcp-server) — TypeScript
- [**prschmid/zoomus**](https://github.com/prschmid/zoomus) — Python
- [**licht1stein/pyzoom**](https://github.com/licht1stein/pyzoom) — Python
- [**tmonfre/zoom-cli**](https://github.com/tmonfre/zoom-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
