# Skool CLI

**Every Skool community feature, plus a local SQLite mirror, FTS, and cross-community ops no other Skool tool ships.**

Pulls every post, comment, member, course, lesson, and calendar event into a local SQLite store with FTS5 so you can query historical state, compute leaderboard deltas, and surface at-risk members the native UI cannot show. One auth_token cookie, two hosts (www.skool.com reads, api2.skool.com writes), zero CloudFront friction.

Created by [@quoxientzero](https://github.com/quoxientzero) (Zain Haseeb).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow), [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `skool-pp-cli` binary and the `pp-skool` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install skool
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install skool --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install skool --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install skool --agent claude-code
npx -y @mvanhorn/printing-press-library install skool --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/cmd/skool-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/skool-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install skool --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-skool --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-skool --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install skool --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
skool-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/skool-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/skool/cmd/skool-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "skool": {
      "command": "skool-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Skool has no public API. Authenticate with the auth_token JWT cookie from your logged-in browser session: `skool-pp-cli auth login` (writes ~/.config/skool-pp-cli/config.toml). Add `--chrome` to read the cookie straight out of a logged-in Chrome profile. `skool-pp-cli auth status` shows the current state and `skool-pp-cli auth logout` clears it. Same cookie covers reads and writes; CloudFront requires a realistic User-Agent which the CLI sets automatically.

## Quick Start

```bash
# Store your auth_token cookie once; lives in TOML config
skool-pp-cli auth login --chrome

# First-time sync of a community into the local store
skool-pp-cli sync --community <community-slug>

# List recent posts with field selection
skool-pp-cli posts list --community <community-slug> --limit 10 --json --select id,name,user.name

# List community members
skool-pp-cli members list --community <community-slug> --json

# Current leaderboard
skool-pp-cli leaderboard --community <community-slug> --top 25

# What's new in the last day across the community
skool-pp-cli digest since 24h --community <community-slug>

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`posts top`** — Rank recent posts by upvotes, comments, or engagement and return them with full content.

  _Pick this for a daily/weekly cron that surfaces the 3-5 most-engaging posts from any community — perfect for catching up without scrolling._

  ```bash
  skool-pp-cli posts top --community earlyaidopters --since 7d --top 5 --by engagement --json
  ```
- **`leaderboard`** — Top members by points for the community, with level and bio fields included.

  _Pick this when an agent needs the current community leaderboard in one call without scraping the page._

  ```bash
  skool-pp-cli leaderboard --community bewarethedefault --top 25 --json
  ```
- **`digest since`** — Aggregate everything new across posts, comments, members, and lessons since a timestamp.

  _Pick this when an agent needs a single brief of community activity for a daily/weekly cron._

  ```bash
  skool-pp-cli digest since 24h --json
  ```
- **`sql`** — Run read-only SQL across every community in your local store.

  _Pick this when an agent needs to compose a query across multiple Skool communities you own or operate._

  ```bash
  skool-pp-cli sql 'SELECT community, COUNT(*) FROM posts GROUP BY community'
  ```

- **`sync --resources posts,members`** — Hydrate the local SQLite mirror with a community's posts and members, unwrapped out of the Next.js page envelope.

  _Pick this before using `search` or `sql` — until the mirror holds posts and members, both have nothing to read._

  ```bash
  skool-pp-cli sync --community <community-slug> --resources posts,members
  skool-pp-cli search "onboarding" --limit 10
  ```

### Agent-native plumbing
- **`calendar export`** — Export upcoming community events to an .ics file for Google Cal / Outlook.

  _Pick this when a member wants community events on their personal calendar without manual entry._

  ```bash
  skool-pp-cli calendar export --ics > community.ics
  ```
- **`classroom export`** — Export an entire course to a markdown bundle (modules, lessons, attachments, video URLs).

  _Pick this when an agent needs to ingest a course for offline reference, search, or LLM retrieval._

  ```bash
  skool-pp-cli classroom export <course-slug> --out ./course/
  ```
- **`classroom courses` / `classroom course get`** — Read the classroom as a normalized tree: courses, their folders, and the pages inside each folder, with id, title, order, and access at every level.

  _Pick this when an agent needs the classroom's structure rather than a lesson's text — it is the shape `classroom push` diffs against._

  ```bash
  skool-pp-cli classroom course get reading-the-signals --community <slug> --json
  ```
- **`classroom push`** — Diff a directory of markdown lessons against the live classroom and print the ordered create/update plan, matched by title so the same bundle pushed twice updates the same pages.

  _Pick this when a course is authored as markdown in git and the classroom should follow it. Plan-only by default; `push` never deletes._

  ```bash
  skool-pp-cli classroom push ./lessons --community <slug> --json
  ```
- **`posts create --md` / `--from-md`** — Write a post, comment, or lesson body in markdown and let the CLI emit Skool's TipTap JSON (paragraphs, bold, ordered lists, bullet lists, links, H3 max).

  _Pick this instead of hand-building TipTap nodes; a malformed document is rejected by Skool with no indication of which block was at fault._

  ```bash
  skool-pp-cli posts create --group-id <id> --from-md ./announcement.md
  ```

## Recipes

### Author a course in markdown, then plan the push

```bash
skool-pp-cli classroom push ./lessons --community <community-slug> --json
```

One markdown file per lesson, with frontmatter naming the course and the order:

```markdown
---
course: "2. Reading the Signals"
course_order: 2
lesson: "2.1 Why the trend beats the day"
order: 1
access: open
---
The **trend** is the signal, not the day.

1. Load first
2. Then intensity
```

`push` parses the whole bundle before it touches the network, reads the live
classroom, and prints the ordered create/update plan. Courses and lessons are
matched by title (case- and whitespace-insensitively), so pushing the same
bundle twice updates the same pages instead of creating duplicates. It is
plan-only unless `--apply` is passed, it rate-limits itself to ~1 request per
second, it never deletes, and it refuses to send any request outside the
classroom surface. A live page the bundle does not mention is reported as an
orphan and left alone.

`access` accepts `open`, `level:<n>`, or `private`.

> **`--apply` is not live yet.** Skool's classroom write endpoints are not in
> this CLI's spec: only `POST https://api2.skool.com/courses` is confirmed to
> exist, and its request body plus the folder-create, page-create, page-update,
> reorder, and delete calls are still undiscovered. `--apply` fails closed with
> the capture recipe. Do live smoke tests against a throwaway course only.

### Read the classroom as a tree

```bash
skool-pp-cli classroom courses list --community <community-slug> --json
skool-pp-cli classroom course get reading-the-signals --community <community-slug> --json
```

`courses list` returns id, slug, title, order, and access per course. `course
get` returns the three-level tree (course → folders → pages) — the same shape
`push` diffs against. Add `--raw` to `course get` for the untouched Next.js
payload.

### Post markdown without hand-writing TipTap

```bash
skool-pp-cli posts create --group-id <group-id> --from-md ./announcement.md
skool-pp-cli posts comment <post-id> --md 'Nice **work** — see [the thread](https://example.com/t).'
```

Skool stores post, comment, and lesson bodies as TipTap JSON. `--md` takes
markdown inline, `--from-md` reads it from a file (`-` for stdin), and both
convert to TipTap covering paragraphs, bold, ordered lists, bullet lists,
links, and headings clamped to H3 — the subset Skool's own editor round-trips.
Anything outside it is carried through as literal text rather than guessed at.
Pair with `--dry-run` to inspect the exact payload before sending.


## Usage

Run `skool-pp-cli --help` for the full command reference and flag list.

## Commands

### calendar

Community calendar events

- **`skool-pp-cli calendar list`** - List upcoming and recent calendar events

### classroom

Classroom (courses, modules, lessons) for a community

- **`skool-pp-cli classroom get-course`** - Get a single course with its modules and lessons
- **`skool-pp-cli classroom list`** - List all courses in a community
- **`skool-pp-cli classroom courses list`** - List courses normalized (id, slug, title, order, access)
- **`skool-pp-cli classroom course get`** - Get a course as a normalized tree (course -> folders -> pages)
- **`skool-pp-cli classroom push`** - Plan (and, with --apply, upload) a directory of markdown lessons

### community

Community feed, settings, and metadata

- **`skool-pp-cli community about`** - About page (rules, owner, member count)
- **`skool-pp-cli community info`** - Get the community feed (posts, leaderboard summary, upcoming events, settings)
- **`skool-pp-cli community leaderboard-tab`** - Leaderboard tab (community page rendered with t=leaderboard)
- **`skool-pp-cli community members-tab`** - Members tab data (community page rendered with t=members)

### me

Current authenticated user dashboard

- **`skool-pp-cli me get`** - Get current user, joined communities, and dashboard state

### members

Community members and moderation

- **`skool-pp-cli members list`** - List members of a community
- **`skool-pp-cli members approve`** - Approve a pending member request
- **`skool-pp-cli members ban`** - Ban a member from the community
- **`skool-pp-cli members pending`** - List pending member join requests
- **`skool-pp-cli members reject`** - Reject a pending member request

### notifications

User notifications

- **`skool-pp-cli notifications list`** - List notifications for the authenticated user
- **`skool-pp-cli notifications mark-read`** - Mark notifications as read (empty ids = mark all)

### posts

Posts (forum threads) inside a community

- **`skool-pp-cli posts list`** - List posts in a community (newest first)
- **`skool-pp-cli posts comment`** - Add a comment to a post (use --md / --from-md for markdown)
- **`skool-pp-cli posts create`** - Create a new post (body = TipTap JSON; use --md / --from-md for markdown)
- **`skool-pp-cli posts delete`** - Delete a post
- **`skool-pp-cli posts get`** - Get a post detail page including comment tree
- **`skool-pp-cli posts like`** - Like (upvote) a post
- **`skool-pp-cli posts unlike`** - Unlike a post
- **`skool-pp-cli posts update`** - Update an existing post (use --md / --from-md for markdown)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
skool-pp-cli calendar mock-value --community example-value

# JSON for scripting and agents
skool-pp-cli calendar mock-value --community example-value --json

# Filter to specific fields
skool-pp-cli calendar mock-value --community example-value --json --select id,name,status

# Dry run — show the request without sending
skool-pp-cli calendar mock-value --community example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
skool-pp-cli calendar mock-value --community example-value --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SKOOL_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `skool-pp-cli notifications`
- `skool-pp-cli notifications list`
- `skool-pp-cli notifications mark-read`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SKOOL_COMMUNITY` resolves `{community}`
- `SKOOL_BUILDID` resolves `{buildId}`

Base URL: `https://www.skool.com`

Skool splits reads and writes across two hosts, and the CLI routes each request
to the right one automatically:

- **Reads** — `https://www.skool.com` Next.js data routes (`/_next/data/{buildId}/...`).
- **Writes** — `https://api2.skool.com` REST (posts, comments, upvotes, member
  moderation, notifications). Override with `SKOOL_API2_BASE_URL`; when
  `SKOOL_BASE_URL` points at a mock server, writes follow it there.

The same `auth_token` cookie covers both.

## Health Check

```bash
skool-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/skool-pp-cli/config.toml`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `skool-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **404 on read endpoints** — Run `skool-pp-cli doctor`; the buildId likely rotated. The CLI auto-refetches but you can force `skool-pp-cli buildid refresh`.
- **403 from CloudFront** — Your auth_token expired or User-Agent is missing. Run `skool-pp-cli auth status`, then `skool-pp-cli auth login --chrome` for a fresh cookie.
- **Empty leaderboard delta** — Need at least two snapshots. Run `skool-pp-cli sync --community <community-slug>` over multiple days to accumulate them.
- **`404 {"notFound":true}` on a read** — Skool rotates its Next.js buildId on every deploy. The CLI now drops the cached buildId and retries once automatically; if it persists, the community slug or course slug is wrong.
- **`sync` reports a failed resource with no reason** — Re-run with `--agent`; each failure now emits its own `{"event":"sync_error","resource":...,"error":...}` line before the summary.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**louiewoof2026/skool-mcp**](https://github.com/louiewoof2026/skool-mcp) — TypeScript
- [**cristiantala/skool-all-in-one-api**](https://apify.com/cristiantala/skool-all-in-one-api) — TypeScript
- [**FlowExtractAPI/skool-scraper-pro**](https://github.com/FlowExtractAPI/skool-scraper-pro) — TypeScript
- [**moon-home/scraper**](https://github.com/moon-home/scraper) — Python
- [**aperswal/Skool_Active_Members_Chrome_Extension**](https://github.com/aperswal/Skool_Active_Members_Chrome_Extension) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
