---
name: pp-calibre-ebook
description: "Manage Calibre ebook libraries from the CLI. Use for library health checks, finding corrupted or malformed books, metadata cleanup, format auditing, series normalization, full-text search across book contents, format conversion, batch library maintenance, and ebook polishing. Works offline without the Calibre content server. Trigger phrases: `calibre library health`, `check calibre library`, `find corrupted books`, `fix calibre metadata`, `audit my ebooks`, `calibre cleanup`, `polish epub`, `convert ebook`, `calibre search`, `full text search calibre`."
author: "Eric Jung"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - calibre-ebook-pp-cli
    install:
      - kind: note
        message: "Requires Calibre desktop app installed (provides calibredb binary). Go CLI wraps calibredb for agent-native access."
---

# Calibre Ebook — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `calibre-ebook-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install calibre-ebook --cli-only
   ```
2. Verify: `calibre-ebook-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/calibre-ebook/cmd/calibre-ebook-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Prerequisites: Calibre

This CLI wraps Calibre's `calibredb`, `ebook-meta`, `ebook-convert`, and `ebook-polish` tools.

1. Install Calibre from https://calibre-ebook.com (required for underlying binaries)
2. Verify: `calibre-ebook-pp-cli doctor --library-path "/path/to/Calibre Library"`

The CLI auto-detects Calibre binaries at `/Applications/calibre.app/Contents/MacOS/` on macOS. Set `CALIBREDB_PATH`, `EBOOK_META_PATH`, etc. to override.

## When to Use This CLI

Use this CLI when the user asks about:

- Library health — corrupted files, missing covers, orphaned data, malformed paths
- Metadata cleanup — fixing titles, authors, series, tags, identifiers
- Series normalization — standardizing series naming and indexing
- Format auditing — finding books in unwanted formats, missing formats, oversized files
- Full-text search across book contents
- Format conversion — EPUB, MOBI, AZW3, PDF, etc.
- Book polishing — smartening punctuation, subsetting fonts, compressing images, removing unused CSS
- Batch library maintenance and reports
- Exporting catalogs or individual books

## Library Path

All commands accept `--library-path` to specify which Calibre library to target. You can also set `CALIBRE_LIBRARY_PATH` env var or configure it in `~/.config/calibre-ebook-pp-cli/config.toml`:

```toml
library_path = "/Users/ericjung/SynologyDrive/resources/Calibre Library/Library"
```

**Always close the Calibre GUI before running write operations** — concurrent access can corrupt the database.

## Command Reference

**books** — Manage books

- `calibre-ebook-pp-cli books add` — Add a book to the library
- `calibre-ebook-pp-cli books list` — List books in the library
- `calibre-ebook-pp-cli books remove` — Remove a book from the library
- `calibre-ebook-pp-cli books search` — Search for book IDs matching a query
- `calibre-ebook-pp-cli books set-metadata` — Update metadata for a book
- `calibre-ebook-pp-cli books show-metadata` — Show full metadata for a book

**convert** — Manage convert

- `calibre-ebook-pp-cli convert` — Convert an ebook between formats

**file-meta** — Manage file meta

- `calibre-ebook-pp-cli file-meta` — Read metadata from an ebook file (no library needed)

**fts** — Manage fts

- `calibre-ebook-pp-cli fts index-reindex` — Reindex books for full-text search
- `calibre-ebook-pp-cli fts index-status` — Check full-text search indexing status
- `calibre-ebook-pp-cli fts search` — Full-text search across book contents

**library** — Manage library

- `calibre-ebook-pp-cli library add-saved-search` — Add a saved search
- `calibre-ebook-pp-cli library backup-metadata` — Backup metadata to OPF files for all books
- `calibre-ebook-pp-cli library check` — Audit library for errors and inconsistencies
- `calibre-ebook-pp-cli library export-books` — Export books to the filesystem
- `calibre-ebook-pp-cli library list-categories` — List all tags, authors, series, publishers, etc.
- `calibre-ebook-pp-cli library list-custom-columns` — List custom metadata columns
- `calibre-ebook-pp-cli library list-saved-searches` — List saved searches
- `calibre-ebook-pp-cli library remove-saved-search` — Remove a saved search
- `calibre-ebook-pp-cli library restore-database` — Rebuild database from OPF files (destructive)
- `calibre-ebook-pp-cli library health-score` — Compute a 0-100 library health score with per-category deductions
- `calibre-ebook-pp-cli library duplicates` — Find duplicate books by title/author (exact/fuzzy/broad)
- `calibre-ebook-pp-cli library series-gaps` — Detect series with missing books by analyzing series_index
- `calibre-ebook-pp-cli library stats` — Aggregate library statistics (formats, authors, series, sizes)

**polish** — Manage polish

- `calibre-ebook-pp-cli polish` — Polish an ebook (EPUB/AZW3/KEPUB only)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
calibre-ebook-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `calibre-ebook-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  calibre-ebook-pp-cli books list --agent --select id,name,status
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
calibre-ebook-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
calibre-ebook-pp-cli feedback --stdin < notes.txt
calibre-ebook-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.calibre-ebook-pp-cli/feedback.jsonl`. They are never POSTed unless `CALIBRE_EBOOK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CALIBRE_EBOOK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
calibre-ebook-pp-cli profile save briefing --json
calibre-ebook-pp-cli --profile briefing books list
calibre-ebook-pp-cli profile list --json
calibre-ebook-pp-cli profile show briefing
calibre-ebook-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `calibre-ebook-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add calibre-ebook-pp-mcp -- calibre-ebook-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which calibre-ebook-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   calibre-ebook-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `calibre-ebook-pp-cli <command> --help`.
