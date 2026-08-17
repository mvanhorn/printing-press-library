# passage CLI

**A contemplative daily reading practice — open book APIs as your library, real public-domain passages to sit with, and a reading journal that's yours.**

Reading trackers log what you read; ebook CLIs fetch files. passage makes a daily practice out of real public-domain texts: today serves an opinionated pick, sit shows a real Project Gutenberg passage and takes your reflection, journal keeps them, and shelf/next/stats track your reading. Everything is local SQLite — the practice is yours.

## Install

The recommended path installs both the `passage-pp-cli` binary and the `pp-passage` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install passage
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install passage --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install passage --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install passage --agent claude-code
npx -y @mvanhorn/printing-press-library install passage --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/passage-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install passage --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-passage --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-passage --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install passage --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/passage-current).
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
    "passage": {
      "command": "passage-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No key required. Open Library and Gutendex (Project Gutenberg) are keyless; Google Books works anonymously. Set GOOGLE_BOOKS_API_KEY only to lift the anonymous rate limit.

## Quick Start

```bash
# find a book; results flag which have free full text
passage search "marcus aurelius"

# an opinionated public-domain pick for today
passage today

# read a real Gutenberg passage and capture a reflection
passage sit 2680 --note "start slow"

# read back your reflections
passage journal

# add a book to your want-to-read shelf
passage shelf add OL45804W --to want

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### The practice
- **`today`** — An opinionated public-domain book or passage to read today, rotated against what you've recently sat with.

  _Reach for this to start a daily reading practice without deciding what to read._

  ```bash
  passage today --agent
  ```
- **`sit`** — Fetch a real Project Gutenberg passage, show an excerpt to read, and capture your reflection to your local journal.

  _Use this for the core contemplative loop: read a real passage, write what it left you with._

  ```bash
  passage sit 1342 --note "Austen's irony still lands"
  ```
- **`journal`** — Your reflections over time, newest first — the record of your reading practice.

  _Use this to read back what your reading has left you with._

  ```bash
  passage journal --limit 20
  ```

### Your shelf
- **`next`** — Ranks your want-to-read shelf against your reading history to suggest what to pick up next.

  _Use this when you've finished a book and want the next one off your own shelf._

  ```bash
  passage next
  ```
- **`stats`** — Your reading pace, top subjects, and rating distribution from your local log.

  _Use this to see your reading habits over time._

  ```bash
  passage stats
  ```

## Recipes

### The daily loop

```bash
passage today && passage sit <id> --note "..."
```

Get today's pick, then sit with its passage and journal a reflection.

### Find something with free text

```bash
passage search "stoicism" --json --select docs.title,docs.has_fulltext
```

Search and narrow to which results have free full text.

### Read back the practice

```bash
passage journal --limit 30
```

Your recent reflections, newest first.

## Usage

Run `passage-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BOOK_GOAT_CONFIG_DIR`, `BOOK_GOAT_DATA_DIR`, `BOOK_GOAT_STATE_DIR`, or `BOOK_GOAT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BOOK_GOAT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BOOK_GOAT_HOME=/srv/passage
passage-pp-cli doctor
```

Under `BOOK_GOAT_HOME=/srv/passage`, the four dirs resolve to `/srv/passage/config`, `/srv/passage/data`, `/srv/passage/state`, and `/srv/passage/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "passage": {
      "command": "passage-pp-mcp",
      "env": {
        "BOOK_GOAT_HOME": "/srv/passage"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BOOK_GOAT_DATA_DIR` overrides an explicit `--home` for that kind. Use `BOOK_GOAT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BOOK_GOAT_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `passage-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### authors

Manage authors

- **`passage-pp-cli authors <authorId>`** - Biographical metadata for an Open Library author.

### search-json

Manage search json

- **`passage-pp-cli search-json`** - Full-text search over Open Library's catalog. Returns works with title, author, year, cover, and full-text availability.

### works

Manage works

- **`passage-pp-cli works <workId>`** - Full metadata for an Open Library work (description, subjects, covers, links).


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`passage-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`passage-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`passage-pp-cli learnings list`** - Inspect taught rows
- **`passage-pp-cli learnings forget <query>`** - Undo a teach
- **`passage-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`passage-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`passage-pp-cli teach-pattern`** - Install a query/resource template up front
- **`passage-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BOOK_GOAT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `passage-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
passage-pp-cli authors mock-value

# JSON for scripting and agents
passage-pp-cli authors mock-value --json

# Filter to specific fields
passage-pp-cli authors mock-value --json --select id,name,status

# Dry run — show the request without sending
passage-pp-cli authors mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
passage-pp-cli authors mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
passage-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `passage-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/passage-pp-cli/config.toml`; `--home`, `BOOK_GOAT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 or rate-limited from Open Library / Gutendex** — These keyless APIs allow ~1 request/second anonymously; passage backs off — retry, or slow bulk syncs.
- **sit says no text available** — Only public-domain books on Project Gutenberg have full text — use a Gutenberg id (the number in a gutendex /books result), not an Open Library id.
- **Google Books returns 'not available in this country'** — passage sends country=US by default; enrichment is optional and off the core loop.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bookcut**](https://github.com/costis94/bookcut) — Python (120 stars)
- [**libro**](https://github.com/mkaz/libro) — Python (40 stars)
- [**goodreads-cli**](https://github.com/yareeh/goodreads-cli) — JavaScript (20 stars)
- [**pgberg**](https://github.com/gutenbergtools) — Python (15 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
