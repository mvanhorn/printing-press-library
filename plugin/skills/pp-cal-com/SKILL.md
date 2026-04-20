---
name: pp-cal-com
description: "Use this skill whenever the user asks about their Cal.com schedule, bookings, upcoming meetings, event types, availability, calendar conflicts, booking analytics, or wants to create / reschedule / cancel a Cal.com booking. Cal.com CLI covering 285 API operations with offline search, booking analytics, conflict detection, and 7 insight commands for scheduling intelligence. Requires a Cal.com API key (CAL_COM_TOKEN). Triggers on phrasings like 'what's on my Cal.com today', 'any conflicts in my calendar next week', 'how many bookings did I have this month', 'show my no-shows', 'which event types convert best'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["cal-com-pp-cli"],"env":["CAL_COM_TOKEN"]},"primaryEnv":"CAL_COM_TOKEN","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@latest","bins":["cal-com-pp-cli"],"label":"Install via go install"}]}}'
---

# Cal.com — Printing Press CLI

Manage bookings, event types, schedules, and availability via the Cal.com API. Covers 285 API operations across 181 paths with a SQLite data layer for offline search and 7 insight commands no other Cal.com tool offers.

## When to Use This CLI

Reach for this when a user wants to manage or analyze their Cal.com scheduling from outside the web UI — checking today's agenda, spotting conflicts, reviewing no-shows, analyzing conversion rates by event type, or batch-managing bookings. Also useful for agent-driven workflows that schedule or reschedule via natural language.

Don't reach for this when the user wants a Google Calendar or Outlook lookup (use their respective CLIs / MCPs) or when the scheduling platform in question isn't Cal.com.

## Unique Capabilities

The 7 insight commands that require the local SQLite data layer.

### Scheduling intelligence

- **`today`** — Today's schedule with attendee details and conferencing links. The daily kickoff command.

  _Compresses the "check my day" ritual into a single JSON blob: meeting time, attendee emails, Zoom/Meet links, agenda notes._

- **`conflicts`** — Detects overlapping bookings or bookings inside blocked-off windows. Runs against the synced local store so it sees cross-calendar conflicts too.

- **`gaps`** — Free-time slots between bookings. Useful for "when can I take a break this week."

### Post-meeting analysis

- **`stats`** — Booking analytics: total bookings, no-show rate, average duration, by event-type breakdown.

- **`noshow`** — Dedicated no-show reporting with per-attendee history.

- **`workload`** — Calendar density analysis. How many hours booked per day, trend over time.

### Hygiene

- **`stale`** — Event types nobody's booked in N days. Candidates for removal or promotion.

## Command Reference

Core resources (each supports list/get/create/update/delete):

- `cal-com-pp-cli bookings` — Meetings (get, cancel, confirm, decline, reassign, reschedule, mark-absent, and nested `attendees`, `guests`, `references`, `calendar-links`, `conferencing-sessions`, `recordings`, `transcripts`)
- `cal-com-pp-cli event-types` — Bookable event types (with nested `webhooks` and `private-links`)
- `cal-com-pp-cli schedules` — Availability schedules
- `cal-com-pp-cli slots` — Available time slots for an event type (accepts `--event-type-slug` + `--username` or `--event-type-id`, plus `--start`/`--end`)
- `cal-com-pp-cli calendars` — Connected calendars (Google, Outlook, etc.), plus ICS feeds, free/busy, busy-times
- `cal-com-pp-cli conferencing` — Zoom/Meet/etc. integrations (connect, disconnect, default)
- `cal-com-pp-cli me` — Profile
- `cal-com-pp-cli teams` — Teams
- `cal-com-pp-cli webhooks` — Webhook subscriptions
- `cal-com-pp-cli api-keys` — API key management
- `cal-com-pp-cli oauth-clients` — OAuth clients (with nested `users` and `webhooks`)
- `cal-com-pp-cli organizations` — Org-level resources (teams, members, roles, `attributes`)
- `cal-com-pp-cli routing-forms` — Routing forms
- `cal-com-pp-cli destination-calendars` / `selected-calendars` — Destination/selected calendar config
- `cal-com-pp-cli stripe` / `verified-resources` — Stripe connection, verified emails/phones

Per-booking operations (all live under `bookings`):

- `cal-com-pp-cli bookings get-bookinguid <bookingUid>` — Detail for one booking
- `cal-com-pp-cli bookings attendees booking-add <bookingUid>` — Add attendee
- `cal-com-pp-cli bookings location booking-update-booking <bookingUid>` — Update location
- `cal-com-pp-cli bookings cancel bookings-booking <bookingUid>` / `bookings confirm bookings-booking` / `bookings decline bookings-booking` / `bookings reschedule bookings-booking`

Unique insight commands:

- `cal-com-pp-cli today` — Today's schedule
- `cal-com-pp-cli conflicts` — Overlap detection
- `cal-com-pp-cli gaps` — Free time
- `cal-com-pp-cli stats` — Analytics
- `cal-com-pp-cli noshow` — No-show report
- `cal-com-pp-cli workload` — Density analysis
- `cal-com-pp-cli stale` — Unused event types
- `cal-com-pp-cli analytics` — Custom queries over the locally synced data

Utility:

- `cal-com-pp-cli sync` — Pull API data into local SQLite
- `cal-com-pp-cli export` / `import` — JSONL/JSON dump + restore
- `cal-com-pp-cli tail <resource>` — Stream live changes as NDJSON via polling
- `cal-com-pp-cli search "<query>"` — Full-text across synced bookings, event types, attendees
- `cal-com-pp-cli auth set-token <CAL_COM_TOKEN>`
- `cal-com-pp-cli doctor` — Verify config, auth, and API reachability

## Recipes

### Morning schedule check

```bash
cal-com-pp-cli today --agent
cal-com-pp-cli conflicts --agent  # catch overlapping slots
```

`today` returns today's meetings with conferencing links inline; `conflicts` checks for any overlaps you might have missed.

### Weekly workload review

```bash
cal-com-pp-cli workload --period 7d --agent
cal-com-pp-cli gaps --days 7 --min-gap 60 --agent
```

Workload shows total booked hours per day; `gaps` with a 60-minute floor surfaces open blocks that could fit deep work. (Workload uses `--period` as a free-form window; gaps uses `--days` for the lookahead and `--min-gap` in minutes.)

### Post-month analytics for event-type tuning

```bash
cal-com-pp-cli stats --period 30d --agent
cal-com-pp-cli noshow --agent
cal-com-pp-cli stale --days 60 --agent
```

Stats shows which event types get booked most; no-show flags problematic attendees or event types; `stale` identifies event types to delete or promote.

### Stream live booking changes

```bash
# Poll every 30 seconds, emit NDJSON to stdout, filter for cancellations with jq:
cal-com-pp-cli tail bookings --interval 30s --agent | jq 'select(.status == "cancelled")'
```

`tail` polls the API on a configurable interval and emits one JSON object per change on stdout (status messages go to stderr). Useful for cron-free monitoring dashboards or piping into downstream automation. Default interval is 10s; pass `--follow=false` for a single poll.

### Offline search and analytics after a sync

```bash
cal-com-pp-cli sync                                 # one-time pull into SQLite
cal-com-pp-cli search "design review" --agent       # full-text search over synced data
cal-com-pp-cli analytics --type bookings --group-by status --limit 10 --agent
cal-com-pp-cli stats --period 30d --data-source local --agent   # skip the API entirely
```

`search` operates on the local SQLite store (run `sync` first). `analytics` aggregates any synced resource with `--type` + `--group-by`. For read commands, the root flag `--data-source local` forces offline-only, `--data-source live` forces API-only (bypassing the sync cache); default `auto` is live with local fallback.

### Find a gap and book

```bash
# List available slots between two ISO-8601 UTC datetimes for an event type:
cal-com-pp-cli slots \
  --event-type-slug "intro-call" --username alice \
  --start 2026-05-15T00:00:00Z --end 2026-05-15T23:59:59Z --agent

# Create a booking by piping a JSON body on stdin (bookings create is
# stdin-only — every field goes into the JSON payload):
cat <<'EOF' | cal-com-pp-cli bookings create --stdin --agent
{
  "start": "2026-05-15T14:00:00Z",
  "eventTypeId": 123,
  "attendee": {"name": "Alice", "email": "alice@example.com", "timeZone": "America/Los_Angeles"}
}
EOF
```

Ask for open slots by event-type slug + user + date window, then POST a booking body via stdin.

## Auth Setup

Cal.com uses API keys. Get one at [cal.com/settings/developer/api-keys](https://app.cal.com/settings/developer/api-keys).

```bash
export CAL_COM_TOKEN="cal_..."
cal-com-pp-cli auth set-token "$CAL_COM_TOKEN"
cal-com-pp-cli doctor
```

Optional:
- `CAL_COM_BASE_URL` — override API base (for self-hosted Cal.com v2 instances)
- `CAL_COM_CONFIG` — override config file path (default: `~/.config/cal-com-pp-cli/config.toml`)

## Agent Mode

Add `--agent` to any command. Expands to `--json --compact --no-input --no-color --yes`.

Useful root flags (all persistent):

- `--select id,status,start` — cherry-pick fields from the JSON response
- `--dry-run` — print the HTTP request without sending
- `--no-cache` — bypass the 5-minute GET cache
- `--rate-limit 2` — cap requests per second (useful under 429 pressure)
- `--data-source auto|live|local` — default `auto`; force `live` to skip the sync cache, `local` to run fully offline against synced data
- `--period 30d` / `--period 12w` — analysis window for `stats` and `workload`

Paginated commands also emit NDJSON progress events on stderr by default, so `--agent | jq` pipelines only see the final JSON on stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Not found (booking, event-type, user) |
| 4 | Auth required |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Installation

### CLI

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@main
cal-com-pp-cli auth set-token YOUR_CAL_COM_TOKEN
cal-com-pp-cli doctor
```

### MCP Server

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-mcp@main
claude mcp add -e CAL_COM_TOKEN=<token> cal-com-pp-mcp -- cal-com-pp-mcp
```

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → run `cal-com-pp-cli --help`
2. **`install`** → CLI; **`install mcp`** → MCP
3. **Agenda / today / schedule queries** → `today --agent`
4. **Conflict / overlap queries** → `conflicts --agent`
5. **Anything else** → check install + auth, match intent to a command, run with `--agent`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
cal-com-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
cal-com-pp-cli --profile <name> <command>

# List / inspect / remove
cal-com-pp-cli profile list
cal-com-pp-cli profile show <name>
cal-com-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
cal-com-pp-cli <command> --deliver file:/path/to/out.json
cal-com-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
cal-com-pp-cli feedback "what surprised you or tripped you up"
cal-com-pp-cli feedback list         # show local entries
cal-com-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.cal-com-pp-cli/feedback.jsonl` as JSON lines. When `CAL_COM_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `CAL_COM_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

