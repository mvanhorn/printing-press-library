# archive-is-pp-cli

Bypass paywalls and look up web archives via archive.today. Looks up existing snapshots before submitting new ones to save time and avoid rate limits. Extracts clean markdown from archived pages for piping into LLMs. Falls back to Wayback Machine automatically when archive.today is blocked or rate-limited.

**Primary use case:** "I want to read an article that's behind a paywall."

> **About archive.today:** On February 21, 2026, Wikipedia formally blacklisted archive.today after evidence of DDoS activity and snapshot tampering. This CLI is intended for personal paywall reading. Do NOT use it for legal evidence, academic citation, or anything requiring a trustworthy archive — use the Wayback Machine for that. This CLI ships with Wayback as a built-in fallback backend for that reason.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/other/archive-is/cmd/archive-is-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/archive-is-current).

### Authentication

archive.today has no official API and requires no credentials. Just run the CLI:

```bash
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article
```

For self-hosted or mirror override, set `ARCHIVE_IS_BASE_URL` (see [Configuration](#configuration)).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
archive-is-pp-cli doctor
```

This checks your configuration.

### 3. Read a paywalled article

```bash
# Find an existing archive (or create one), copy URL to clipboard
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article

# Pipe the article text straight into Claude
archive-is-pp-cli get https://www.nytimes.com/2026/04/10/article | claude "summarize this article"

# See every historical snapshot
archive-is-pp-cli history https://www.nytimes.com/

# Request an archive for a URL that isn't archived yet, and wait for it
archive-is-pp-cli request https://example.com/fresh-url --wait --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`get`** — Fetch the archived page and extract clean markdown text — ready to pipe into Claude, LLMs, or notes.
- **`read`** — Find existing snapshot with timegate first, submit only if nothing recent exists. Saves 60+ seconds per hit and avoids rate limits.
- **`read`** — Automatically retry across archive.ph, archive.md, archive.is, archive.fo, and other mirrors when one is blocked or rate-limited.
- **`read`** — When archive.is fails or is rate-limited, automatically try Wayback Machine. Hedges the February 2026 archive.today reputation risk.
- **`search`** — Full-text search across your past archive reads — find that article you saved last month without remembering the URL.
- **`bulk`** — Archive a list of URLs from a file or stdin, with built-in 10-second gaps and exponential backoff on 429s. Run it and walk away.

## Usage

```
archive-is-pp-cli looks up, fetches, and creates web page archives via archive.today (archive.ph).
Primary use case: bypass paywalls to read articles.

Usage:
  archive-is-pp-cli [command]

Available Commands:
  api         Browse all API endpoints by interface name
  auth        Manage authentication tokens
  bulk        Archive a list of URLs with built-in rate limiting
  captures    Submit a URL for archiving (synchronous, 30-120s)
  doctor      Check CLI health
  export      Export data to JSONL or JSON for backup, migration, or analysis
  feeds       Global RSS feed of recently archived URLs across all archive.today users
  get         Fetch the full archived article text as clean markdown
  history     List all known archive snapshots for a URL, oldest to newest
  import      Import data from JSONL file via API create/upsert calls
  read        Find or create an archive of a paywalled URL (the hero command)
  request     Request a new archive and optionally wait for it to be ready
  save        Archive a URL (submit a new capture to archive.today)
  snapshots   Find the best archive.today snapshot for a URL via the Memento timegate
  sync        Sync API data to local SQLite for offline search and analysis
  workflow    Compound workflows that combine multiple API operations

Global flags:
      --agent                --json --compact --no-input --no-color --yes in one flag
      --compact              Return only key fields for minimal token usage
      --config string        Config file path
      --csv                  Output as CSV
      --data-source string   auto | live | local (default "auto")
      --dry-run              Show request without sending
      --json                 Output as JSON
      --no-cache             Bypass response cache
      --no-input             Disable all interactive prompts
      --plain                Output as plain tab-separated text
      --quiet                Bare output, one value per line
      --rate-limit float     Max requests per second (0 to disable)
      --select string        Comma-separated fields to include in output
      --timeout duration     Request timeout (default 30s)
      --yes                  Skip confirmation prompts
```

## Paywall Reader Workflow

The six hand-built commands below are the hero features. They're optimized for one workflow: hit a paywalled URL, get the readable version back.

| Command | What it does | When to use it |
|---------|-------------|----------------|
| `read <url>` | Find existing archive (timegate) or create one. Copies URL to clipboard. | Default. Hit a paywall, run this, paste into browser. |
| `get <url>` | Same as read but fetches the article HTML and extracts clean text. | Pipe into LLMs, notes, or read in the terminal. Falls back to Wayback on CAPTCHA. |
| `history <url>` | List every known snapshot of the URL, oldest to newest. | Researchers tracking article changes. Also: 12,312 NYT homepage snapshots back to 1996. |
| `save <url>` | Force a fresh capture, bypassing the dedup window. | When you need the current version preserved before it changes. Slower (30-120s). |
| `request <url>` | Fire a capture request, optionally wait for it to be ready. | Submit-and-forget, or `--wait` for agent workflows that need to know when it's done. |
| `bulk <file>` | Archive a list of URLs with built-in rate limiting. | Save a reading list. 10-second gaps between requests by default. |

**Why lookup-before-submit matters:** `read` hits the Memento timegate endpoint first (< 500ms). Only if there's no recent snapshot does it submit a fresh capture (30-120s). Every other CLI for archive.today submits on every call, wasting 60+ seconds per URL and triggering rate limits.

**Why dual-backend fallback matters:** archive.today frequently serves CAPTCHA challenges for direct body fetches. `get` and `read` automatically fall back to the Wayback Machine when that happens. You never have to think about it.

## Commands

### Paywall reader (the hero commands)

| Command | Description |
|---------|-------------|
| `read <url>` | Find existing archive (timegate) or create one. Copies memento URL to clipboard. |
| `get <url>` | Find/create archive, fetch HTML, extract clean markdown for LLM piping. |
| `history <url>` | List every known snapshot for a URL (Memento timemap). |
| `save <url>` | Force a fresh capture (30-120s), bypassing the dedup window. |
| `request <url>` | Fire a capture request, optionally `--wait` until ready. |
| `request check <url>` | Poll once to see whether a submitted request is ready. |
| `bulk [file]` | Archive a list of URLs with built-in rate limiting. Reads stdin with `-`. |

### Archive.today resources

| Command | Description |
|---------|-------------|
| `snapshots <url>` | Memento timegate lookup for a URL (best snapshot). |
| `snapshots newest <url>` | Most recent snapshot for a URL. |
| `snapshots timemap <url>` | All known snapshots as a Memento link-format list. |
| `captures --url <url>` | Submit a URL for fresh archiving via `/submit/`. |
| `feeds` | Global RSS feed of recently archived URLs. |
| `feeds search <query>` | Search archived pages by keyword. |

### Data layer & workflows

| Command | Description |
|---------|-------------|
| `sync` | Sync recent archive metadata to local SQLite for offline search. |
| `workflow archive` | Run the full sync workflow across all resources. |
| `workflow status` | Show local archive status and sync state. |
| `export --format jsonl` | Export synced data to JSONL. |
| `import <file>` | Import JSONL back into the API. |
| `api` | Browse all API endpoints by interface name. |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check configuration, auth, and API reachability. |
| `auth` | Manage authentication tokens (archive.today needs none). |
| `version` | Print CLI version. |


## Output Formats

```bash
# Human-readable default (URL on stdout, metadata on stderr)
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article

# JSON for scripting and agents
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article --json

# Filter to specific fields
archive-is-pp-cli history https://example.com --json --select memento_url,captured_at

# Dry run — show the request without sending
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article --dry-run

# Agent mode — JSON + compact + no prompts in one flag
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/article --agent

# Quiet mode — one memento URL per line (perfect for xargs)
archive-is-pp-cli history https://www.nytimes.com/ --quiet
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | archive-is-pp-cli <resource> create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add archive-is archive-is-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "archive-is": {
      "command": "archive-is-pp-mcp"
    }
  }
}
```

## Cookbook

Real recipes built from the commands this CLI actually ships.

```bash
# 1. Read a paywalled article, copy the archive URL to the clipboard
archive-is-pp-cli read https://www.nytimes.com/2026/04/10/some-article

# 2. Summarize a paywalled article with Claude
archive-is-pp-cli get https://www.nytimes.com/2026/04/10/some-article \
  | claude "summarize this in three bullet points"

# 3. Fetch the raw HTML of the archived page
archive-is-pp-cli get https://example.com/article --raw > page.html

# 4. Show every snapshot ever taken of a homepage (timemap)
archive-is-pp-cli history https://www.nytimes.com/ --json \
  | jq '.snapshots[] | {captured_at, memento_url}'

# 5. Force a fresh capture before a page changes or disappears
archive-is-pp-cli save https://example.com/about --force

# 6. Submit an archive request and poll until it is ready (agent workflow)
archive-is-pp-cli request https://ft.com/content/xyz --wait --wait-timeout 3m --json

# 7. Fire-and-forget: submit now, check back later
archive-is-pp-cli request https://example.com/article
archive-is-pp-cli request check https://example.com/article

# 8. Archive a reading list with built-in rate limiting
archive-is-pp-cli bulk reading-list.txt --delay 15s

# 9. Pipe URLs from stdin instead of a file
grep -oE 'https?://[^ )]+' notes.md | archive-is-pp-cli bulk -

# 10. Prefer the archive.today mirror but fall back to Wayback automatically
archive-is-pp-cli read https://ft.com/content/xyz --backend archive-is,wayback

# 11. Get just the newest snapshot of a URL
archive-is-pp-cli snapshots newest https://example.com --json

# 12. Sync metadata from the global recent-archives feed for offline analysis
archive-is-pp-cli sync --full
archive-is-pp-cli workflow status

# 13. Export your local archive index for backup
archive-is-pp-cli export --format jsonl > archive-backup.jsonl

# 14. Check archive-is-pp-cli's auth and API reachability
archive-is-pp-cli doctor --json
```

## Health Check

```bash
$ archive-is-pp-cli doctor
  OK Config: ok
  WARN Auth: not required
  OK API: reachable
  config_path: /Users/you/.config/archive-is-pp-cli/config.toml
  base_url: https://archive.ph
  version: 1.0.0
```

archive.today has no API key — "not required" is the expected auth status. If `API: reachable` fails, archive.ph may be rate-limiting your IP or serving a CAPTCHA. The CLI will transparently fall back to other mirrors (archive.md, archive.is, archive.fo, archive.li, archive.vn) and, for body fetches, to the Wayback Machine.

## Configuration

Config file: `~/.config/archive-is-pp-cli/config.toml`

Environment variables:

| Variable | Purpose | Default |
|----------|---------|---------|
| `ARCHIVE_IS_CONFIG` | Path to an alternative config file | `~/.config/archive-is-pp-cli/config.toml` |
| `ARCHIVE_IS_BASE_URL` | Override the primary mirror used for timegate and submit | `https://archive.ph` |

archive.today requires no authentication — there is no API key to set.

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `archive-is-pp-cli doctor` to check credentials

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- If persistent, wait a few minutes and try again

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**oduwsdl/archivenow**](https://github.com/oduwsdl/archivenow) — python (400 stars)
- [**palewire/archiveis**](https://github.com/palewire/archiveis) — python (150 stars)
- [**HRDepartment/archivetoday**](https://github.com/HRDepartment/archivetoday) — typescript (80 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `archive-is-pp-cli profile save <name> --<flag> <value>`, then `archive-is-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.archive-is-pp-cli/feedback.jsonl`. When `ARCHIVE_IS_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `ARCHIVE_IS_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
