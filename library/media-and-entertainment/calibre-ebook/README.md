# Calibre Ebook CLI

A printing-press CLI that wraps Calibre's `calibredb`, `ebook-meta`, `ebook-convert`, and `ebook-polish` command-line tools into an agent-native interface with insight analytics, caching, and structured JSON output.

**Not a REST API wrapper.** This CLI calls Calibre's local binaries directly via `exec.Command` — no content server, no HTTP transport, no API key. Works entirely offline. Requires Calibre to be installed.

Printed by [@e-jung](https://github.com/e-jung) (Eric Jung).

## Install

The recommended path installs both the `calibre-ebook-pp-cli` binary and the `pp-calibre-ebook` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install calibre-ebook
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install calibre-ebook --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install calibre-ebook --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install calibre-ebook --agent claude-code
npx -y @mvanhorn/printing-press-library install calibre-ebook --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/calibre-ebook-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-calibre-ebook --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-calibre-ebook --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-calibre-ebook skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-calibre-ebook. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/calibre-ebook-current).
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
    "calibre-ebook": {
      "command": "calibre-ebook-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
calibre-ebook-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
calibre-ebook-pp-cli books list
```

## Usage

Run `calibre-ebook-pp-cli --help` for the full command reference and flag list.

## Commands

### books

Manage books

- **`calibre-ebook-pp-cli books add`** - Add a book to the library
- **`calibre-ebook-pp-cli books list`** - List books in the library
- **`calibre-ebook-pp-cli books remove`** - Remove a book from the library
- **`calibre-ebook-pp-cli books search`** - Search for book IDs matching a query
- **`calibre-ebook-pp-cli books set-metadata`** - Update metadata for a book
- **`calibre-ebook-pp-cli books show-metadata`** - Show full metadata for a book

### convert

Manage convert

- **`calibre-ebook-pp-cli convert`** - Convert an ebook between formats

### file-meta

Manage file meta

- **`calibre-ebook-pp-cli file-meta`** - Read metadata from an ebook file (no library needed)

### fts

Manage fts

- **`calibre-ebook-pp-cli fts index-reindex`** - Reindex books for full-text search
- **`calibre-ebook-pp-cli fts index-status`** - Check full-text search indexing status
- **`calibre-ebook-pp-cli fts search`** - Full-text search across book contents

### library

Manage library

- **`calibre-ebook-pp-cli library add-saved-search`** - Add a saved search
- **`calibre-ebook-pp-cli library backup-metadata`** - Backup metadata to OPF files for all books
- **`calibre-ebook-pp-cli library check`** - Audit library for errors and inconsistencies
- **`calibre-ebook-pp-cli library export-books`** - Export books to the filesystem
- **`calibre-ebook-pp-cli library list-categories`** - List all tags, authors, series, publishers, etc.
- **`calibre-ebook-pp-cli library list-custom-columns`** - List custom metadata columns
- **`calibre-ebook-pp-cli library list-saved-searches`** - List saved searches
- **`calibre-ebook-pp-cli library remove-saved-search`** - Remove a saved search
- **`calibre-ebook-pp-cli library restore-database`** - Rebuild database from OPF files (destructive)
- **`calibre-ebook-pp-cli library health-score`** - Compute a 0-100 library health score
- **`calibre-ebook-pp-cli library duplicates`** - Find duplicate books (exact/fuzzy/broad)
- **`calibre-ebook-pp-cli library series-gaps`** - Detect series with missing books
- **`calibre-ebook-pp-cli library stats`** - Aggregate library statistics

### polish

Manage polish

- **`calibre-ebook-pp-cli polish`** - Polish an ebook (EPUB/AZW3/KEPUB only)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
calibre-ebook-pp-cli books list

# JSON for scripting and agents
calibre-ebook-pp-cli books list --json

# Filter to specific fields
calibre-ebook-pp-cli books list --json --select id,name,status

# Dry run — show the request without sending
calibre-ebook-pp-cli books list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
calibre-ebook-pp-cli books list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` database locked (close Calibre GUI), `7` rate limited, `10` config error, `11` calibre not found.

## Health Check

```bash
calibre-ebook-pp-cli doctor
```

Checks that Calibre binaries (`calibredb`, `ebook-meta`, `ebook-convert`, `ebook-polish`) are discoverable and the library path is valid.

## Configuration

Config file: `~/.config/calibre-ebook-library-pp-cli/config.toml`

Set your library path via config, `--library-path` flag, or `CALIBRE_LIBRARY_PATH` environment variable.

## How It Works

Unlike most printing-press CLIs that wrap REST APIs over HTTP, this CLI wraps Calibre's local command-line tools. The generated command files call standard `Get`/`Post`/`Put`/`Delete` client methods — but the client intercepts these and routes them to `exec.Command` calls to `calibredb`, `ebook-meta`, etc. This preserves the printing-press architecture while operating entirely locally.

Key implications:
- **No API key needed** — all operations are local
- **No network required** — works offline
- **Calibre must be installed** — `doctor` verifies binary discovery
- **Close Calibre GUI before writes** — the SQLite DB is locked while the GUI is open. The CLI retries automatically with exponential backoff on DB-lock errors
- **In-memory cache** — GET responses are cached with per-resource TTLs (5-60 min). Use `--no-cache` to bypass

## Insight Commands

Six analytics commands provide library intelligence beyond what Calibre offers natively:

| Command | Description |
|---------|-------------|
| `library health-score` | 0-100 composite score across metadata completeness, format coverage, series integrity, duplicate rate |
| `library stats` | Aggregate counts: books, authors, series, formats, tags, languages, size |
| `library duplicates` | Find duplicate books via exact title/author match, fuzzy matching, or ISBN cross-check |
| `library series-gaps` | Detect series with missing index numbers |
| `library similar` | Find books similar to a given title/author using tag/author overlap |
| `coverage` | Per-field metadata coverage analysis (what % of books have ISBN, publisher, tags, etc.) |

## Troubleshooting
**Database locked errors (exit code 6)**
- Close the Calibre GUI before running write commands
- The CLI retries 3 times with exponential backoff

**Not found errors (exit code 3)**
- Check the book ID is correct
- Run `books list` to see available items

**Calibre not found (exit code 11)**
- Install Calibre or set `CALIBREDB_PATH` to the full path
- On macOS: `/Applications/calibre.app/Contents/MacOS/calibredb`

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
