# Canvas LMS CLI

**The first Canvas CLI that handles pagination, works across all your courses at once, and tells you what to work on next.**

canvas-pp-cli syncs your entire Canvas LMS state locally and exposes every course, assignment, submission, file, and grade through a fast Go CLI. Unlike every other Canvas tool, it handles Link-header pagination transparently, runs cross-course queries offline, and surfaces intelligence no single API call can produce — like which assignment costs you the most per hour of delay.

## Install

The recommended path installs both the `canvas-pp-cli` binary and the `pp-canvas-lms` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install canvas-lms
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install canvas-lms --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/canvas-lms-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-canvas-lms --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-canvas-lms --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-canvas-lms skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-canvas-lms. The skill defines how its required CLI can be installed.
```

## Authentication

Canvas uses personal access tokens. Generate one at Settings → Approved Integrations → New Access Token, then set CANVAS_API_TOKEN and CANVAS_BASE_URL (your institution's Canvas host, e.g. https://canvas.txstate.edu).

## Quick Start

```bash
# verify token and API reachability first
canvas-pp-cli doctor


# pull all courses, assignments, submissions, files into local SQLite
canvas-pp-cli sync


# see what costs you the most per hour of delay this week
canvas-pp-cli pressure --days 7


# check submission status for a specific course
canvas-pp-cli assignments list 12345 --json --select name,due_at,submission_status


# submit a file upload assignment
canvas-pp-cli submissions submit 12345 67890 --file ./report.pdf

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`pressure`** — Ranks all upcoming assignments across all your courses by how much each costs per hour of delay.

  _Use when an agent needs to prioritize which assignment to work on next across multiple active courses._

  ```bash
  canvas-lms-pp-cli pressure --days 14 --agent
  ```
- **`impact`** — Shows the max and min final grade still achievable in a course given your remaining unsubmitted work.

  _Use when an agent or student needs to know if passing a course is still possible before committing to remaining work._

  ```bash
  canvas-lms-pp-cli impact CS3398 --agent
  ```
- **`drift`** — Tracks your grade trajectory across syncs and flags courses where your score has dropped since last check.

  _Use when an agent monitors academic performance over time and needs to alert on declining grades._

  ```bash
  canvas-lms-pp-cli drift --json --agent
  ```
- **`going-dark`** — Flags courses with unread announcements, no recent module completions, and upcoming deadlines — courses slipping off your radar.

  _Use when an agent monitors for courses a student has stopped engaging with before a deadline hits._

  ```bash
  canvas-lms-pp-cli going-dark --agent
  ```
- **`gaps`** — Shows module completion ratio and flags incomplete items that block downstream graded assignments.

  _Use when an agent needs to identify what prerequisite work a student is missing before an assignment is due._

  ```bash
  canvas-lms-pp-cli gaps CS3398 --agent
  ```

### Pattern intelligence
- **`late-window`** — Analyzes your submission history to compute whether an instructor actually accepts late work and by how many days.

  _Use when an agent needs to decide whether to submit late or skip — answered from actual historical data._

  ```bash
  canvas-lms-pp-cli late-window CS3398 --agent
  ```
- **`forecast`** — Shows a per-day workload bar chart for the next N weeks across all courses, weighted by submission type effort.

  _Use when an agent plans a student's schedule and needs to avoid overloading any single day._

  ```bash
  canvas-lms-pp-cli forecast --weeks 2 --agent --select day,load_score,assignments
  ```
- **`heads-up`** — Surfaces announcements posted within 72 hours of a due date for assignments you have not yet submitted.

  _Use when an agent checks for last-minute instructor guidance before a submission deadline._

  ```bash
  canvas-lms-pp-cli heads-up --agent
  ```

## Usage

Run `canvas-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts


### announcements

Manage announcements

- **`canvas-pp-cli announcements list`** - List announcements

### calendar-events

Manage calendar events

- **`canvas-pp-cli calendar-events create`** - Create a calendar event
- **`canvas-pp-cli calendar-events delete`** - Delete a calendar event
- **`canvas-pp-cli calendar-events get-single-or-assignment`** - Get a single calendar event or assignment
- **`canvas-pp-cli calendar-events list`** - List calendar events
- **`canvas-pp-cli calendar-events save-enabled-account-calendars`** - Save enabled account calendars
- **`canvas-pp-cli calendar-events update`** - Update a calendar event

### courses

Manage courses

- **`canvas-pp-cli courses delete-conclude`** - Delete/Conclude a course
- **`canvas-pp-cli courses get-single`** - Get a single course
- **`canvas-pp-cli courses list-your`** - List your courses
- **`canvas-pp-cli courses update`** - Update a course

### files

Manage files

- **`canvas-pp-cli files delete`** - Delete file
- **`canvas-pp-cli files get`** - Get file
- **`canvas-pp-cli files update`** - Update file

### folders

Manage folders

- **`canvas-pp-cli folders delete`** - Delete folder
- **`canvas-pp-cli folders get`** - Get folder
- **`canvas-pp-cli folders update`** - Update folder

### groups

Manage groups


### rubrics

Manage rubrics

- **`canvas-pp-cli rubrics templated-file-for-importing`** - Templated file for importing a rubric

### sections

Manage sections

- **`canvas-pp-cli sections delete`** - Delete a section
- **`canvas-pp-cli sections edit`** - Edit a section
- **`canvas-pp-cli sections get-information`** - Get section information

### temporary-enrollment-status

Manage temporary enrollment status

- **`canvas-pp-cli temporary-enrollment-status bulk`** - Bulk Temporary Enrollment Status

### users

Manage users

- **`canvas-pp-cli users activity-stream-summary`** - Activity stream summary
- **`canvas-pp-cli users beta-get-batch-query-results`** - BETA - Get batch query results
- **`canvas-pp-cli users beta-initiate-batch-page-views-query`** - BETA - Initiate batch page views query
- **`canvas-pp-cli users beta-poll-batch-query-status`** - BETA - Poll batch query status
- **`canvas-pp-cli users clear-course-nicknames`** - Clear course nicknames
- **`canvas-pp-cli users edit`** - Edit a user
- **`canvas-pp-cli users get-course-nickname`** - Get course nickname
- **`canvas-pp-cli users get-pandata-events-jwt-token-and-its-expiration-date`** - Get a Pandata Events jwt token and its expiration date
- **`canvas-pp-cli users hide-all-stream-items`** - Hide all stream items
- **`canvas-pp-cli users hide-stream-item`** - Hide a stream item
- **`canvas-pp-cli users list-activity-stream-activity-stream`** - List the activity stream
- **`canvas-pp-cli users list-activity-stream-self`** - List the activity stream
- **`canvas-pp-cli users list-counts-for-todo-items`** - List counts for todo items
- **`canvas-pp-cli users list-course-nicknames`** - List course nicknames
- **`canvas-pp-cli users list-todo-items`** - List the TODO items
- **`canvas-pp-cli users list-upcoming-assignments-calendar-events`** - List upcoming assignments, calendar events
- **`canvas-pp-cli users log-out-of-all-mobile-apps-mobile-sessions`** - Log users out of all mobile apps
- **`canvas-pp-cli users remove-course-nickname`** - Remove course nickname
- **`canvas-pp-cli users set-course-nickname`** - Set course nickname
- **`canvas-pp-cli users show-details`** - Show user details


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
canvas-pp-cli announcements --context-codes example-value

# JSON for scripting and agents
canvas-pp-cli announcements --context-codes example-value --json

# Filter to specific fields
canvas-pp-cli announcements --context-codes example-value --json --select id,name,status

# Dry run — show the request without sending
canvas-pp-cli announcements --context-codes example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
canvas-pp-cli announcements --context-codes example-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-canvas-lms -g
```

Then invoke `/pp-canvas-lms <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add canvas-lms canvas-lms-pp-mcp -e CANVAS_API_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/canvas-lms-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CANVAS_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "canvas-lms": {
      "command": "canvas-lms-pp-mcp",
      "env": {
        "CANVAS_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
canvas-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/canvas-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CANVAS_API_TOKEN` | per_call | Yes | Set to your API credential. |
| `CANVAS_BASE_URL` | per_call | Yes | Your Canvas instance URL (e.g. https://canvas.txstate.edu) |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `canvas-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CANVAS_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — export CANVAS_API_TOKEN=<your-token> and export CANVAS_BASE_URL=https://canvas.yourinstitution.edu
- **Empty results from list commands** — run canvas-pp-cli sync first to populate local store, or add --live flag to bypass cache
- **Rate limit errors (403 with throttle body)** — canvas-pp-cli sync --delay 200 to add 200ms between requests
- **quiz submissions returns no data** — New Quizzes (v2 engine) is LTI-only and not available via REST API; this command covers legacy quizzes only

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**canvasapi**](https://github.com/ucfopen/canvasapi) — Python (660 stars)
- [**canvas-mcp (vishalsachdev)**](https://github.com/vishalsachdev/canvas-mcp) — TypeScript (129 stars)
- [**canvas-mcp (DMontgomery40)**](https://github.com/DMontgomery40/mcp-canvas-lms) — TypeScript (96 stars)
- [**node-canvas-api**](https://github.com/ubc/node-canvas-api) — JavaScript (72 stars)
- [**atomicjolt/canvasapi**](https://github.com/atomicjolt/canvasapi) — Go (3 stars)
- [**fuller**](https://github.com/grantlemons/fuller) — Rust (2 stars)
- [**canvas-cli-and-mcp**](https://github.com/1jehuang/canvas-cli-and-mcp) — TypeScript
- [**canvasctl**](https://github.com/vivekp-05/canvasctl) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
