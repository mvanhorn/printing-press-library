# Visit Detroit Blog CLI

**Detroit's official "Inside the D" blog as a CLI — every article searchable offline, sliced across category and neighborhood at once, with related-reads and exports the website can't produce.**

Inside the D CLI mirrors Visit Detroit's editorial blog into a local SQLite store, so you can full-text search every article body offline and then slice it by category, neighborhood, and date in a single query (blogs list) — something the site's single-facet instant-search can't do. blogs related surfaces articles that share the most topics and neighborhoods; blogs coverage maps where the blog is dense or thin; blogs reading-list exports a neutral, sponsored-free reading list for a team.

Created by [@stanrails](https://github.com/stanrails) (stanrails).

## Install

The recommended path installs both the `visit-detroit-blog-pp-cli` binary and the `pp-visit-detroit-blog` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install visit-detroit-blog
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install visit-detroit-blog --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install visit-detroit-blog --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install visit-detroit-blog --agent claude-code
npx -y @mvanhorn/printing-press-library install visit-detroit-blog --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/visit-detroit-blog/cmd/visit-detroit-blog-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/visit-detroit-blog-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-visit-detroit-blog --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-visit-detroit-blog --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install visit-detroit-blog --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/visit-detroit-blog-current).
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
    "visit-detroit-blog": {
      "command": "visit-detroit-blog-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Pull the full Inside the D blog into the local store. Run once; re-run to refresh.
visit-detroit-blog-pp-cli sync

# Ranked full-text search across every article body, offline.
visit-detroit-blog-pp-cli search "ethiopian food"

# The cross-axis filter the website's single-facet search can't do.
visit-detroit-blog-pp-cli blogs list --category Dining --region Corktown

# Read the full article body in your terminal — no browser.
visit-detroit-blog-pp-cli blogs get donuts

# Find related reads by shared category and neighborhood.
visit-detroit-blog-pp-cli blogs related donuts --limit 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Queries the website can't express
- **`blogs list`** — Filter Detroit articles by category AND neighborhood AND date window in one query — the slice the website's single-facet search can't express.

  _Reach for this when a request combines a topic and a place (and optionally recency) — e.g. 'recent dining articles in Corktown' — instead of issuing several single-facet searches and intersecting them by hand._

  ```bash
  inside-the-d-pp-cli blogs list --category Dining --region Corktown --since 2026-01-01 --agent
  ```
- **`blogs related`** — Find the articles that share the most categories and neighborhoods with a given post, ranked.

  _Use this to chain recommendations — after surfacing one good article, hand the user (or yourself) the next best reads without another keyword search._

  ```bash
  inside-the-d-pp-cli blogs related donuts --limit 5 --agent
  ```
- **`blogs coverage`** — Cross-tabulate article counts across every category and every neighborhood to see where coverage is dense or thin.

  _Use this to answer 'which neighborhoods have the most Outdoors coverage' or to spot gaps before recommending an under-covered area._

  ```bash
  inside-the-d-pp-cli blogs coverage --category Outdoors --agent
  ```

### Take it with you
- **`blogs reading-list`** — Materialize an ordered, deduped reading list (markdown/json/csv) from any filter to a file — with an option to drop sponsored posts for a neutral handout.

  _Use this when a person needs to hand a curated, source-stable list to a team or attendee — not a one-off query that disappears when the tab closes._

  ```bash
  inside-the-d-pp-cli blogs reading-list --region "Downtown Detroit" --category Culture --no-sponsored --output detroit-culture.md
  ```

## Usage

Run `visit-detroit-blog-pp-cli --help` for the full command reference and flag list.

## Commands

Run `visit-detroit-blog-pp-cli sync` once to populate the local store, then:

### blogs — browse, read, and analyze articles

- **`visit-detroit-blog-pp-cli blogs list`** - filter articles across category, neighborhood, and date (`--category`, `--region`, `--since`, `--until`, `--no-sponsored`, `--limit`)
- **`visit-detroit-blog-pp-cli blogs get <slug>`** - read a full article body by slug, URI, or id
- **`visit-detroit-blog-pp-cli blogs related <slug>`** - articles sharing the most categories and neighborhoods
- **`visit-detroit-blog-pp-cli blogs coverage`** - category × neighborhood cross-tab (`--category`, `--region`)
- **`visit-detroit-blog-pp-cli blogs reading-list`** - export an ordered md/json/csv reading list (`--output`)

### Top-level

- **`visit-detroit-blog-pp-cli search <query>`** - offline ranked full-text search
- **`visit-detroit-blog-pp-cli categories`** - list blog categories with article counts
- **`visit-detroit-blog-pp-cli regions`** - list neighborhoods/regions with article counts
- **`visit-detroit-blog-pp-cli recent`** - newest articles by post date
- **`visit-detroit-blog-pp-cli sync`** - pull all articles into the local store

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
visit-detroit-blog-pp-cli blogs list

# JSON for scripting and agents
visit-detroit-blog-pp-cli blogs list --json

# Filter to specific fields
visit-detroit-blog-pp-cli blogs list --json --select title,url,categories

# Dry run — show the request without sending
visit-detroit-blog-pp-cli blogs list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
visit-detroit-blog-pp-cli blogs list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select title,url,categories` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
visit-detroit-blog-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/visit-detroit-blog-pp-cli/config.json` (override with `VISIT_DETROIT_BLOG_CONFIG`).

No authentication is required — the blog is served through a public, search-only Algolia key embedded in the Visit Detroit site. If that key ever rotates, override it with the `VISIT_DETROIT_BLOG_ALGOLIA_API_KEY` (and `VISIT_DETROIT_BLOG_ALGOLIA_APP_ID`) environment variables.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search / blogs list returns nothing** — Run `visit-detroit-blog-pp-cli sync` first — search and list read the local store, which is empty until the first sync.
- **sync prints a sync_warning with status 403** — The public Algolia search key embedded in the site rotated. Update it via `VISIT_DETROIT_BLOG_ALGOLIA_API_KEY`, or re-run `/printing-press` to re-discover the current key.
- **a category or region filter returns 0 results** — Run `visit-detroit-blog-pp-cli categories` or `regions` to see the exact facet spellings (e.g. "Downtown Detroit", not "downtown").
- **results look stale vs the live site** — Re-run `visit-detroit-blog-pp-cli sync` to refresh the local store from Algolia.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
