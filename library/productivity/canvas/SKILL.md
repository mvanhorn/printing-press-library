---
name: pp-canvas-lms
description: "The first Canvas CLI that handles pagination, works across all your courses at once, and tells you what to work on next. Trigger phrases: `what assignments do I have due`, `check my Canvas grades`, `submit my assignment on Canvas`, `download files from Canvas`, `what should I work on for class`, `use canvas`, `run canvas cli`."
author: "Martin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - canvas-pp-cli
---

# Canvas LMS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `canvas-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install canvas --cli-only
   ```
2. Verify: `canvas-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/canvas/cmd/canvas-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

canvas-pp-cli sync populates your entire Canvas LMS state locally and exposes every course, assignment, submission, file, and grade through a fast Go CLI. Unlike every other Canvas tool, it handles Link-header pagination transparently, runs cross-course queries offline, and surfaces intelligence no single API call can produce — like which assignment costs you the most per hour of delay.

## When to Use This CLI

Use canvas-pp-cli when an agent needs to read or act on Canvas LMS data — checking deadlines, retrieving assignment details, submitting work, or running grade analysis across multiple courses. It is especially useful when the task requires combining data across courses (pressure, forecast, drift) or when offline/fast access to already-synced course data is needed. Do not use for New Quizzes (v2 engine) submissions, which require the LTI interface.

## MCP Fallback for Concluded / Access-Restricted Courses

Canvas restricts API access to courses after the term ends (HTTP 401 / `access_restricted_by_date`). The CLI cannot read these courses directly. **When a CLI command returns 401 or access-restricted for a course, automatically fall back to the Canvas MCP tools** for that operation.

### Fallback rules

| CLI exits with | Action |
|---|---|
| 401 or `access_restricted_by_date` | Re-run via `mcp__canvas__*` equivalent |
| 403 (permission) | Re-run via MCP if a matching tool exists |
| Any other error | Fail normally — MCP won't help |

### CLI → MCP equivalents for read operations

When CLI returns 401 or access-restricted, use the corresponding Canvas MCP tool instead:

- **courses list-your** → Canvas MCP: list_courses (with include_concluded=true)
- **courses get-single** → Canvas MCP: get_course_details
- **courses assignments list** → Canvas MCP: list_assignments
- **courses submissions** → Canvas MCP: get_my_submission_status
- **grade check** → Canvas MCP: get_my_course_grades
- **announcements** → Canvas MCP: list_announcements

### What MCP cannot do (CLI only)

- `pressure`, `impact`, `drift`, `going-dark`, `forecast`, `heads-up`, `gaps` — local SQLite cross-course analytics, no MCP equivalent
- Offline / post-sync queries
- File downloads, assignment submissions


## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`pressure`** — Ranks all upcoming assignments across all your courses by how much each costs per hour of delay.

  _Use when an agent needs to prioritize which assignment to work on next across multiple active courses._

  ```bash
  canvas-pp-cli pressure --days 14 --agent
  ```
- **`impact`** — Shows the max and min final grade still achievable in a course given your remaining unsubmitted work.

  _Use when an agent or student needs to know if passing a course is still possible before committing to remaining work._

  ```bash
  canvas-pp-cli impact --course 12345 --agent
  ```
- **`drift`** — Tracks your grade trajectory across syncs and flags courses where your score has dropped since last check.

  _Use when an agent monitors academic performance over time and needs to alert on declining grades._

  ```bash
  canvas-pp-cli drift --json --agent
  ```
- **`going-dark`** — Flags courses with unread announcements, no recent module completions, and upcoming deadlines — courses slipping off your radar.

  _Use when an agent monitors for courses a student has stopped engaging with before a deadline hits._

  ```bash
  canvas-pp-cli going-dark --agent
  ```
- **`gaps`** — Shows module completion ratio and flags incomplete items that block downstream graded assignments.

  _Use when an agent needs to identify what prerequisite work a student is missing before an assignment is due._

  ```bash
  canvas-pp-cli gaps --course 12345 --agent
  ```

### Pattern intelligence
- **`late-window`** — Analyzes your submission history to compute whether an instructor actually accepts late work and by how many days.

  _Use when an agent needs to decide whether to submit late or skip — answered from actual historical data._

  ```bash
  canvas-pp-cli late-window --course 12345 --agent
  ```
- **`forecast`** — Shows a per-day workload bar chart for the next N weeks across all courses, weighted by submission type effort.

  _Use when an agent plans a student's schedule and needs to avoid overloading any single day._

  ```bash
  canvas-pp-cli forecast --weeks 2 --agent --select day,load_score,assignments
  ```
- **`heads-up`** — Surfaces announcements posted within 72 hours of a due date for assignments you have not yet submitted.

  _Use when an agent checks for last-minute instructor guidance before a submission deadline._

  ```bash
  canvas-pp-cli heads-up --agent
  ```

## Command Reference

**accounts** — Manage accounts


**announcements** — Manage announcements

- `canvas-pp-cli announcements` — List announcements

**calendar-events** — Manage calendar events

- `canvas-pp-cli calendar-events create` — Create a calendar event
- `canvas-pp-cli calendar-events delete` — Delete a calendar event
- `canvas-pp-cli calendar-events get-single-or-assignment` — Get a single calendar event or assignment
- `canvas-pp-cli calendar-events list` — List calendar events
- `canvas-pp-cli calendar-events save-enabled-account-calendars` — Save enabled account calendars
- `canvas-pp-cli calendar-events update` — Update a calendar event

**courses** — Manage courses

- `canvas-pp-cli courses delete-conclude` — Delete/Conclude a course
- `canvas-pp-cli courses get-single` — Get a single course
- `canvas-pp-cli courses list-your` — List your courses
- `canvas-pp-cli courses update` — Update a course

**files** — Manage files

- `canvas-pp-cli files delete` — Delete file
- `canvas-pp-cli files get` — Get file
- `canvas-pp-cli files update` — Update file

**folders** — Manage folders

- `canvas-pp-cli folders delete` — Delete folder
- `canvas-pp-cli folders get` — Get folder
- `canvas-pp-cli folders update` — Update folder

**groups** — Manage groups


**rubrics** — Manage rubrics

- `canvas-pp-cli rubrics` — Templated file for importing a rubric

**sections** — Manage sections

- `canvas-pp-cli sections delete` — Delete a section
- `canvas-pp-cli sections edit` — Edit a section
- `canvas-pp-cli sections get-information` — Get section information

**temporary-enrollment-status** — Manage temporary enrollment status

- `canvas-pp-cli temporary-enrollment-status` — Bulk Temporary Enrollment Status

**users** — Manage users

- `canvas-pp-cli users activity-stream-summary` — Activity stream summary
- `canvas-pp-cli users beta-get-batch-query-results` — BETA - Get batch query results
- `canvas-pp-cli users beta-initiate-batch-page-views-query` — BETA - Initiate batch page views query
- `canvas-pp-cli users beta-poll-batch-query-status` — BETA - Poll batch query status
- `canvas-pp-cli users clear-course-nicknames` — Clear course nicknames
- `canvas-pp-cli users edit` — Edit a user
- `canvas-pp-cli users get-course-nickname` — Get course nickname
- `canvas-pp-cli users get-pandata-events-jwt-token-and-its-expiration-date` — Get a Pandata Events jwt token and its expiration date
- `canvas-pp-cli users hide-all-stream-items` — Hide all stream items
- `canvas-pp-cli users hide-stream-item` — Hide a stream item
- `canvas-pp-cli users list-activity-stream-activity-stream` — List the activity stream
- `canvas-pp-cli users list-activity-stream-self` — List the activity stream
- `canvas-pp-cli users list-counts-for-todo-items` — List counts for todo items
- `canvas-pp-cli users list-course-nicknames` — List course nicknames
- `canvas-pp-cli users list-todo-items` — List the TODO items
- `canvas-pp-cli users list-upcoming-assignments-calendar-events` — List upcoming assignments, calendar events
- `canvas-pp-cli users log-out-of-all-mobile-apps-mobile-sessions` — Log users out of all mobile apps
- `canvas-pp-cli users remove-course-nickname` — Remove course nickname
- `canvas-pp-cli users set-course-nickname` — Set course nickname
- `canvas-pp-cli users show-details` — Show user details


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
canvas-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### What should I work on right now?

```bash
canvas-pp-cli pressure --days 7 --json --select name,course_id,due_date,pressure_score
```

Ranks unsubmitted assignments by cost-per-hour-of-delay across all your enrolled courses.

### Can I still pass this course?

```bash
canvas-pp-cli impact --course 12345 --agent --select course_id,current_score,max_achievable,min_achievable
```

Computes max and min achievable final grade given remaining unsubmitted work.

### Catch last-minute instructor updates

```bash
canvas-pp-cli heads-up --agent --select course_id,announcement_title,assignment_name,hours_before_due
```

Surfaces announcements posted within 72h of an upcoming due date you haven't submitted yet.

### Plan the week ahead

```bash
canvas-pp-cli forecast --weeks 1 --agent --select date,load_score,assignment_count
```

Daily workload bar chart weighted by submission effort type across all courses.

### Grade trajectory alert

```bash
canvas-pp-cli drift --json --select course_id,current_score,prev_score,delta
```

Shows which courses have had the biggest grade drops since last sync.

## Auth Setup

Canvas uses personal access tokens. Generate one at Settings → Approved Integrations → New Access Token, then set CANVAS_API_TOKEN and CANVAS_BASE_URL (your institution's Canvas host, e.g. https://canvas.txstate.edu).

Run `canvas-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  canvas-pp-cli announcements --context-codes example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
canvas-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
canvas-pp-cli feedback --stdin < notes.txt
canvas-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.canvas-pp-cli/feedback.jsonl`. They are never POSTed unless `CANVAS_LMS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CANVAS_LMS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
canvas-pp-cli profile save briefing --json
canvas-pp-cli --profile briefing announcements --context-codes example-value
canvas-pp-cli profile list --json
canvas-pp-cli profile show briefing
canvas-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `canvas-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add canvas-lms-pp-mcp -- canvas-lms-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which canvas-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   canvas-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `canvas-pp-cli <command> --help`.
