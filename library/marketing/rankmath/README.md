# Rank Math CLI

**Rank Math SEO from the terminal: bulk meta edits, site-wide SEO audits, Search Console analytics, and IndexNow — with no bridge plugin.**

Talks to Rank Math's own REST routes on your WordPress site using a WordPress application password. Adds a local store so audit, coverage, and reindex can answer site-wide questions that the Rank Math admin answers one post at a time.

Learn more at [Rank Math](https://rankmath.com).

Created by [@ElizabethcStone](https://github.com/ElizabethcStone).

## Install

The recommended path installs both the `rankmath-pp-cli` binary and the `pp-rankmath` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rankmath
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rankmath --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rankmath --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rankmath --agent claude-code
npx -y @mvanhorn/printing-press-library install rankmath --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/rankmath/cmd/rankmath-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rankmath-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install rankmath --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rankmath --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-rankmath --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install rankmath --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rankmath-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RANKMATH_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/rankmath/cmd/rankmath-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "rankmath": {
      "command": "rankmath-pp-mcp",
      "env": {
        "RANKMATH_SITE": "<site>",
        "RANKMATH_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Rank Math has no hosted API: each WordPress site exposes its own routes under /wp-json/rankmath/v1. Set RANKMATH_SITE to the site host (for example example.com). For commands that need an account, create an Application Password in wp-admin (Users → Profile → Application Passwords) and export RANKMATH_USERNAME and RANKMATH_APP_PASSWORD, or save them with the auth set-credentials subcommand (username, then application password) and check them with auth status. Without credentials, only posts, pages, sync, audit, head get, and doctor modules work. gsc, indexing, coverage, opportunities, portfolio, link-genius, and every meta/schema/redirection/settings write need an account with Rank Math capabilities. The doctor modules subcommand (pass the site hosts) shows which Rank Math modules (Analytics, Instant Indexing, Link Genius, headless getHead) a site exposes.

## Quick Start

```bash
# Check config and credentials without network access
rankmath-pp-cli doctor --dry-run

# Pull the post inventory into the local store
rankmath-pp-cli sync --resources posts,pages --since 7d

# List SEO tag problems across the synced posts
rankmath-pp-cli audit --agent --select issues.url,issues.issue

# Preview which edited posts would be sent to IndexNow
rankmath-pp-cli reindex --since 7d

```

## Unique Features

### Site-wide SEO health
- **`audit`** — One list of every synced post with a missing or duplicate SEO title or description, long title, canonical problem, accidental noindex, or no schema. Works on public pages without credentials; run sync first.

  _Reach for this before any bulk meta edit to get a structured fix list instead of scraping pages one by one._

  ```bash
  rankmath-pp-cli audit --agent --select issues.url,issues.issue
  ```
- **`coverage`** — Find published posts Google has not indexed or never inspected, excluding intentionally noindexed pages. Needs credentials and Rank Math Analytics connected to Search Console.

  _Use it to answer which real content is missing from Google rather than reading raw inspection rows._

  ```bash
  rankmath-pp-cli coverage --agent --select gaps.url,gaps.verdict
  ```

### Edit, verify, index
- **`reindex`** — Preview the posts edited since a given window that are not yet in the IndexNow log; --confirm plus credentials submits them.

  _Run after a batch of edits to get fresh pages crawled without resubmitting the whole site._

  ```bash
  rankmath-pp-cli reindex --since 7d
  ```
- **`opportunities`** — Queries ranking in positions 4-20 whose post title or description does not mention the query. Needs credentials and Rank Math Analytics connected to Search Console.

  _Use it to pick which titles to rewrite next with evidence from real search demand._

  ```bash
  rankmath-pp-cli opportunities --min-impressions 100 --agent
  ```

### Multi-site
- **`portfolio`** — Clicks, impressions, and average position for each site in --sites (or RANKMATH_SITES) in one table. Needs credentials per site.

  _Use it for a multi-site weekly check instead of logging into each wp-admin._

  ```bash
  rankmath-pp-cli portfolio --agent
  ```

## Recipes

### Fix list for an agent

```bash
rankmath-pp-cli audit --agent --select issues.url,issues.issue
```

After sync, a structured list of posts with SEO tag problems, narrowed to the two fields an agent needs.

### Post-edit indexing

```bash
rankmath-pp-cli reindex --since 24h
```

Preview which posts edited in the last day would go to IndexNow; add --confirm to send.

### What Google is missing

```bash
rankmath-pp-cli coverage --agent --select gaps.url,gaps.verdict
```

Published posts that are not indexed, excluding intentional noindex (needs credentials and Search Console connected).

### Search the local post inventory

```bash
rankmath-pp-cli search "retreat" --type posts --limit 10
```

Offline full-text search across posts pulled in by sync.

### Bulk-edit SEO titles from a CSV

```bash
rankmath-pp-cli meta import --file titles.csv --json
```

Preview the rows a CSV of id,title,description,focus_keyword would send to Rank Math's bulk meta route; add --apply (with credentials) to write.

## Usage

Run `rankmath-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `RANKMATH_CONFIG_DIR`, `RANKMATH_DATA_DIR`, `RANKMATH_STATE_DIR`, or `RANKMATH_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `RANKMATH_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export RANKMATH_HOME=/srv/rankmath
rankmath-pp-cli doctor
```

Under `RANKMATH_HOME=/srv/rankmath`, the four dirs resolve to `/srv/rankmath/config`, `/srv/rankmath/data`, `/srv/rankmath/state`, and `/srv/rankmath/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "rankmath": {
      "command": "rankmath-pp-mcp",
      "env": {
        "RANKMATH_HOME": "/srv/rankmath"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `RANKMATH_DATA_DIR` overrides an explicit `--home` for that kind. Use `RANKMATH_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `RANKMATH_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `rankmath-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

- **`rankmath-pp-cli doctor modules`** - Which Rank Math modules (Analytics, Instant Indexing, Link Genius, headless getHead) each site in --sites exposes, from the public route index

### ai_visibility

Manage ai visibility

- **`rankmath-pp-cli ai-visibility brands-generate-queries`** - Rank Math ai-visibility: brands generate queries
- **`rankmath-pp-cli ai-visibility create-brands`** - Rank Math ai-visibility: create brands
- **`rankmath-pp-cli ai-visibility get-brands`** - Rank Math ai-visibility: get brands
- **`rankmath-pp-cli ai-visibility get-brands-insights`** - Rank Math ai-visibility: get brands insights
- **`rankmath-pp-cli ai-visibility get-brands-queries`** - Rank Math ai-visibility: get brands queries
- **`rankmath-pp-cli ai-visibility overview`** - Rank Math ai-visibility: overview
- **`rankmath-pp-cli ai-visibility update-brands`** - Rank Math ai-visibility: update brands
- **`rankmath-pp-cli ai-visibility update-brands-queries`** - Rank Math ai-visibility: update brands queries

### backup

Manage backup

- **`rankmath-pp-cli backup export-settings`** - Rank Math backup: export settings
- **`rankmath-pp-cli backup get-view-data`** - Rank Math backup: get view data
- **`rankmath-pp-cli backup run`** - Rank Math backup: run backup

### dashboard

Manage dashboard

- **`rankmath-pp-cli dashboard search-intent`** - Rank Math dashboard: search intent
- **`rankmath-pp-cli dashboard widget`** - Rank Math dashboard: dashboard widget

### gsc

Manage gsc

- **`rankmath-pp-cli gsc add-track-keyword`** - Rank Math gsc: add track keyword
- **`rankmath-pp-cli gsc analytics-summary`** - Rank Math gsc: analytics summary
- **`rankmath-pp-cli gsc auto-add-focus-keywords`** - Rank Math gsc: auto add focus keywords
- **`rankmath-pp-cli gsc dashboard`** - Rank Math gsc: dashboard
- **`rankmath-pp-cli gsc delete-tracked-keywords`** - Rank Math gsc: delete tracked keywords
- **`rankmath-pp-cli gsc get-desktop-pagespeed`** - Rank Math gsc: get desktop pagespeed
- **`rankmath-pp-cli gsc get-keyword-pages`** - Rank Math gsc: get keyword pages
- **`rankmath-pp-cli gsc get-mobile-pagespeed`** - Rank Math gsc: get mobile pagespeed
- **`rankmath-pp-cli gsc get-page-seoscore`** - Rank Math gsc: get page seoscore
- **`rankmath-pp-cli gsc get-post`** - Rank Math gsc: get post
- **`rankmath-pp-cli gsc get-tracked-keyword-summary`** - Rank Math gsc: get tracked keyword summary
- **`rankmath-pp-cli gsc get-tracked-keywords`** - Rank Math gsc: get tracked keywords
- **`rankmath-pp-cli gsc get-tracked-keywords-rows`** - Rank Math gsc: get tracked keywords rows
- **`rankmath-pp-cli gsc inspection-results`** - Rank Math gsc: inspection results
- **`rankmath-pp-cli gsc inspection-stats`** - Rank Math gsc: inspection stats
- **`rankmath-pp-cli gsc keywords-overview`** - Rank Math gsc: keywords overview
- **`rankmath-pp-cli gsc keywords-rows`** - Rank Math gsc: keywords rows
- **`rankmath-pp-cli gsc keywords-summary`** - Rank Math gsc: keywords summary
- **`rankmath-pp-cli gsc posts-overview`** - Rank Math gsc: posts overview
- **`rankmath-pp-cli gsc posts-rows`** - Rank Math gsc: posts rows
- **`rankmath-pp-cli gsc posts-rows-by-objects`** - Rank Math gsc: posts rows by objects
- **`rankmath-pp-cli gsc posts-summary`** - Rank Math gsc: posts summary
- **`rankmath-pp-cli gsc remove-track-keyword`** - Rank Math gsc: remove track keyword
- **`rankmath-pp-cli gsc tracked-keywords-overview`** - Rank Math gsc: tracked keywords overview

### head

Manage head

- **`rankmath-pp-cli head`** - Rendered Rank Math head tags for a URL (needs headless support enabled)
- **`rankmath-pp-cli head get`** - Rendered SEO head tags for any URL: uses getHead when headless support is on, otherwise parses the page (no credentials needed)

### indexing

Manage indexing

- **`rankmath-pp-cli indexing clear-log`** - Rank Math indexing: clear log
- **`rankmath-pp-cli indexing get-log`** - Rank Math indexing: get log
- **`rankmath-pp-cli indexing submit-urls`** - Rank Math indexing: submit urls

### link_genius

Manage link genius

- **`rankmath-pp-cli link-genius audit-mark-safe`** - Rank Math link-genius: audit mark safe
- **`rankmath-pp-cli link-genius audit-recheck`** - Rank Math link-genius: audit recheck
- **`rankmath-pp-cli link-genius audit-start`** - Rank Math link-genius: audit start
- **`rankmath-pp-cli link-genius audit-stats`** - Rank Math link-genius: audit stats
- **`rankmath-pp-cli link-genius bulk-actions-restore`** - Rank Math link-genius: bulk actions restore
- **`rankmath-pp-cli link-genius bulk-update-apply`** - Rank Math link-genius: bulk update apply
- **`rankmath-pp-cli link-genius bulk-update-preview`** - Rank Math link-genius: bulk update preview
- **`rankmath-pp-cli link-genius bulk-update-preview-results`** - Rank Math link-genius: bulk update preview results
- **`rankmath-pp-cli link-genius bulk-update-rollback`** - Rank Math link-genius: bulk update rollback
- **`rankmath-pp-cli link-genius create-keyword-maps`** - Rank Math link-genius: create keyword maps
- **`rankmath-pp-cli link-genius delete`** - Rank Math link-genius: delete
- **`rankmath-pp-cli link-genius delete-bulk-update-history`** - Rank Math link-genius: delete bulk update history
- **`rankmath-pp-cli link-genius delete-keyword-maps`** - Rank Math link-genius: delete keyword maps
- **`rankmath-pp-cli link-genius delete-keyword-maps-variations`** - Rank Math link-genius: delete keyword maps variations
- **`rankmath-pp-cli link-genius export`** - Rank Math link-genius: export
- **`rankmath-pp-cli link-genius get-bulk-update-changes`** - Rank Math link-genius: get bulk update changes
- **`rankmath-pp-cli link-genius get-keyword-maps`** - Rank Math link-genius: get keyword maps
- **`rankmath-pp-cli link-genius get-keyword-maps-variations`** - Rank Math link-genius: get keyword maps variations
- **`rankmath-pp-cli link-genius get-links-details`** - Rank Math link-genius: get links details
- **`rankmath-pp-cli link-genius item-keyword-maps-generate-variations`** - Rank Math link-genius: item keyword maps generate variations
- **`rankmath-pp-cli link-genius keyword-maps-accept-variations`** - Rank Math link-genius: keyword maps accept variations
- **`rankmath-pp-cli link-genius keyword-maps-bulk-delete`** - Rank Math link-genius: keyword maps bulk delete
- **`rankmath-pp-cli link-genius keyword-maps-bulk-toggle`** - Rank Math link-genius: keyword maps bulk toggle
- **`rankmath-pp-cli link-genius keyword-maps-execute`** - Rank Math link-genius: keyword maps execute
- **`rankmath-pp-cli link-genius keyword-maps-generate-variations`** - Rank Math link-genius: keyword maps generate variations
- **`rankmath-pp-cli link-genius keyword-maps-history`** - Rank Math link-genius: keyword maps history
- **`rankmath-pp-cli link-genius keyword-maps-preview`** - Rank Math link-genius: keyword maps preview
- **`rankmath-pp-cli link-genius keyword-maps-preview-results`** - Rank Math link-genius: keyword maps preview results
- **`rankmath-pp-cli link-genius keyword-maps-variations`** - Rank Math link-genius: keyword maps variations
- **`rankmath-pp-cli link-genius link-opportunities`** - Rank Math link-genius: link opportunities
- **`rankmath-pp-cli link-genius link-suggestions`** - Rank Math link-genius: link suggestions
- **`rankmath-pp-cli link-genius list-bulk-update-history`** - Rank Math link-genius: list bulk update history
- **`rankmath-pp-cli link-genius list-keyword-maps`** - Rank Math link-genius: list keyword maps
- **`rankmath-pp-cli link-genius list-links`** - Rank Math link-genius: list links
- **`rankmath-pp-cli link-genius operations-cancel`** - Rank Math link-genius: operations cancel
- **`rankmath-pp-cli link-genius operations-clear`** - Rank Math link-genius: operations clear
- **`rankmath-pp-cli link-genius operations-progress`** - Rank Math link-genius: operations progress
- **`rankmath-pp-cli link-genius posts`** - Rank Math link-genius: posts
- **`rankmath-pp-cli link-genius posts-stats`** - Rank Math link-genius: posts stats
- **`rankmath-pp-cli link-genius regenerate`** - Rank Math link-genius: regenerate
- **`rankmath-pp-cli link-genius related-posts`** - Rank Math link-genius: related posts
- **`rankmath-pp-cli link-genius remove-nofollow`** - Rank Math link-genius: remove nofollow
- **`rankmath-pp-cli link-genius stats`** - Rank Math link-genius: stats
- **`rankmath-pp-cli link-genius undo-delete`** - Rank Math link-genius: undo delete
- **`rankmath-pp-cli link-genius update`** - Rank Math link-genius: update
- **`rankmath-pp-cli link-genius update-keyword-maps`** - Rank Math link-genius: update keyword maps

### meta

Manage meta

- **`rankmath-pp-cli meta update`** - Rank Math meta: update meta
- **`rankmath-pp-cli meta update-bulk`** - Rank Math meta: update meta bulk
- **`rankmath-pp-cli meta update-seo-score`** - Rank Math meta: update seo score
- **`rankmath-pp-cli meta import`** - Preview, then with --apply send, SEO title/description/focus keyword edits from a CSV (id + columns) via updateMetaBulk (--file)

### pages

Manage pages

- **`rankmath-pp-cli pages`** - List published WordPress pages (id, link, modified)

### posts

Manage posts

- **`rankmath-pp-cli posts`** - List published WordPress posts (id, link, modified)

### redirections

Manage redirections

- **`rankmath-pp-cli redirections`** - Rank Math redirections: update redirection

### schema

Manage schema

- **`rankmath-pp-cli schema save-template`** - Rank Math schema: save template
- **`rankmath-pp-cli schema update`** - Rank Math schema: update schemas

### settings

Manage settings

- **`rankmath-pp-cli settings save-module`** - Rank Math settings: save module
- **`rankmath-pp-cli settings update`** - Rank Math settings: update settings


### audit


- **`rankmath-pp-cli audit`** - One list of every synced post with a missing or duplicate SEO title or description, long title, canonical problem, accidental noindex, or no schema. Works on public pages without credentials; run sync first.


### coverage


- **`rankmath-pp-cli coverage`** - Find published posts Google has not indexed or never inspected, excluding intentionally noindexed pages. Needs credentials and Rank Math Analytics connected to Search Console.


### opportunities


- **`rankmath-pp-cli opportunities`** - Queries ranking in positions 4-20 whose post title or description does not mention the query. Needs credentials and Rank Math Analytics connected to Search Console.


### portfolio


- **`rankmath-pp-cli portfolio`** - Clicks, impressions, and average position for each site in --sites (or RANKMATH_SITES) in one table. Needs credentials per site.


### reindex


- **`rankmath-pp-cli reindex`** - Preview the posts edited since a given window that are not yet in the IndexNow log; --confirm plus credentials submits them.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`rankmath-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`rankmath-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`rankmath-pp-cli learnings list`** - Inspect taught rows
- **`rankmath-pp-cli learnings forget <query>`** - Undo a teach
- **`rankmath-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`rankmath-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`rankmath-pp-cli teach-pattern`** - Install a query/resource template up front
- **`rankmath-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `RANKMATH_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `rankmath-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rankmath-pp-cli head --url https://example.com/resource

# JSON for scripting and agents
rankmath-pp-cli head --url https://example.com/resource --json
# Filter to specific fields
rankmath-pp-cli head --url https://example.com/resource --json --select head,success

# Dry run — show the request without sending
rankmath-pp-cli head --url https://example.com/resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rankmath-pp-cli head --url https://example.com/resource --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `RANKMATH_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `rankmath-pp-cli ai_visibility`
- `rankmath-pp-cli ai_visibility get`
- `rankmath-pp-cli ai_visibility list`
- `rankmath-pp-cli ai_visibility search`
- `rankmath-pp-cli dashboard`
- `rankmath-pp-cli dashboard get`
- `rankmath-pp-cli dashboard list`
- `rankmath-pp-cli dashboard search`
- `rankmath-pp-cli gsc`
- `rankmath-pp-cli gsc get`
- `rankmath-pp-cli gsc list`
- `rankmath-pp-cli gsc search`
- `rankmath-pp-cli gsc-dashboard`
- `rankmath-pp-cli gsc-dashboard get`
- `rankmath-pp-cli gsc-dashboard list`
- `rankmath-pp-cli gsc-dashboard search`
- `rankmath-pp-cli gsc-get-keyword-pages`
- `rankmath-pp-cli gsc-get-keyword-pages get`
- `rankmath-pp-cli gsc-get-keyword-pages list`
- `rankmath-pp-cli gsc-get-keyword-pages search`
- `rankmath-pp-cli gsc-get-tracked-keyword-summary`
- `rankmath-pp-cli gsc-get-tracked-keyword-summary get`
- `rankmath-pp-cli gsc-get-tracked-keyword-summary list`
- `rankmath-pp-cli gsc-get-tracked-keyword-summary search`
- `rankmath-pp-cli gsc-get-tracked-keywords`
- `rankmath-pp-cli gsc-get-tracked-keywords get`
- `rankmath-pp-cli gsc-get-tracked-keywords list`
- `rankmath-pp-cli gsc-get-tracked-keywords search`
- `rankmath-pp-cli gsc-get-tracked-keywords-rows`
- `rankmath-pp-cli gsc-get-tracked-keywords-rows get`
- `rankmath-pp-cli gsc-get-tracked-keywords-rows list`
- `rankmath-pp-cli gsc-get-tracked-keywords-rows search`
- `rankmath-pp-cli gsc-inspection-results`
- `rankmath-pp-cli gsc-inspection-results get`
- `rankmath-pp-cli gsc-inspection-results list`
- `rankmath-pp-cli gsc-inspection-results search`
- `rankmath-pp-cli gsc-inspection-stats`
- `rankmath-pp-cli gsc-inspection-stats get`
- `rankmath-pp-cli gsc-inspection-stats list`
- `rankmath-pp-cli gsc-inspection-stats search`
- `rankmath-pp-cli gsc-keywords-overview`
- `rankmath-pp-cli gsc-keywords-overview get`
- `rankmath-pp-cli gsc-keywords-overview list`
- `rankmath-pp-cli gsc-keywords-overview search`
- `rankmath-pp-cli gsc-keywords-rows`
- `rankmath-pp-cli gsc-keywords-rows get`
- `rankmath-pp-cli gsc-keywords-rows list`
- `rankmath-pp-cli gsc-keywords-rows search`
- `rankmath-pp-cli gsc-keywords-summary`
- `rankmath-pp-cli gsc-keywords-summary get`
- `rankmath-pp-cli gsc-keywords-summary list`
- `rankmath-pp-cli gsc-keywords-summary search`
- `rankmath-pp-cli gsc-posts-overview`
- `rankmath-pp-cli gsc-posts-overview get`
- `rankmath-pp-cli gsc-posts-overview list`
- `rankmath-pp-cli gsc-posts-overview search`
- `rankmath-pp-cli gsc-posts-rows`
- `rankmath-pp-cli gsc-posts-rows get`
- `rankmath-pp-cli gsc-posts-rows list`
- `rankmath-pp-cli gsc-posts-rows search`
- `rankmath-pp-cli gsc-posts-rows-by-objects`
- `rankmath-pp-cli gsc-posts-rows-by-objects get`
- `rankmath-pp-cli gsc-posts-rows-by-objects list`
- `rankmath-pp-cli gsc-posts-rows-by-objects search`
- `rankmath-pp-cli gsc-posts-summary`
- `rankmath-pp-cli gsc-posts-summary get`
- `rankmath-pp-cli gsc-posts-summary list`
- `rankmath-pp-cli gsc-posts-summary search`
- `rankmath-pp-cli gsc-tracked-keywords-overview`
- `rankmath-pp-cli gsc-tracked-keywords-overview get`
- `rankmath-pp-cli gsc-tracked-keywords-overview list`
- `rankmath-pp-cli gsc-tracked-keywords-overview search`
- `rankmath-pp-cli link_genius`
- `rankmath-pp-cli link_genius get`
- `rankmath-pp-cli link_genius list`
- `rankmath-pp-cli link_genius search`
- `rankmath-pp-cli link_genius-history`
- `rankmath-pp-cli link_genius-history get`
- `rankmath-pp-cli link_genius-history list`
- `rankmath-pp-cli link_genius-history search`
- `rankmath-pp-cli link_genius-keyword-maps`
- `rankmath-pp-cli link_genius-keyword-maps get`
- `rankmath-pp-cli link_genius-keyword-maps list`
- `rankmath-pp-cli link_genius-keyword-maps search`
- `rankmath-pp-cli link_genius-links`
- `rankmath-pp-cli link_genius-links get`
- `rankmath-pp-cli link_genius-links list`
- `rankmath-pp-cli link_genius-links search`
- `rankmath-pp-cli link_genius-posts`
- `rankmath-pp-cli link_genius-posts get`
- `rankmath-pp-cli link_genius-posts list`
- `rankmath-pp-cli link_genius-posts search`
- `rankmath-pp-cli link_genius-posts-stats`
- `rankmath-pp-cli link_genius-posts-stats get`
- `rankmath-pp-cli link_genius-posts-stats list`
- `rankmath-pp-cli link_genius-posts-stats search`
- `rankmath-pp-cli link_genius-progress`
- `rankmath-pp-cli link_genius-progress get`
- `rankmath-pp-cli link_genius-progress list`
- `rankmath-pp-cli link_genius-progress search`
- `rankmath-pp-cli link_genius-results`
- `rankmath-pp-cli link_genius-results get`
- `rankmath-pp-cli link_genius-results list`
- `rankmath-pp-cli link_genius-results search`
- `rankmath-pp-cli link_genius-stats`
- `rankmath-pp-cli link_genius-stats get`
- `rankmath-pp-cli link_genius-stats list`
- `rankmath-pp-cli link_genius-stats search`
- `rankmath-pp-cli pages`
- `rankmath-pp-cli pages get`
- `rankmath-pp-cli pages list`
- `rankmath-pp-cli pages search`
- `rankmath-pp-cli posts`
- `rankmath-pp-cli posts get`
- `rankmath-pp-cli posts list`
- `rankmath-pp-cli posts search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `RANKMATH_SITE` resolves `{site}`

Base URL: `https://{site}/wp-json`

## Health Check

```bash
rankmath-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `rankmath-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/rank-math-seo-pp-cli/config.toml`; `--home`, `RANKMATH_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RANKMATH_SITE` | endpoint | Yes |  |
| `RANKMATH_USERNAME` | per_call | Yes |  |
| `RANKMATH_APP_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `rankmath-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `rankmath-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RANKMATH_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 rest_forbidden on rankmath routes** — Set the WordPress username and application password; the account needs Rank Math capabilities for that module.
- **404 rest_no_route for a Pro route (link-genius, rank tracking)** — Run rankmath-pp-cli doctor to see which modules this site exposes; the module is off or Rank Math Pro is not installed.
- **head --url returns 404 rest_no_route** — Headless support is off (Rank Math → General Settings → Others). Use rankmath-pp-cli head get <url>, which falls back to parsing the rendered page.
- **RANKMATH_SITE not set** — export RANKMATH_SITE=example.com (host only, no https://).

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**rank-math-api-manager**](https://github.com/Devora-AS/rank-math-api-manager) — PHP (47 stars)
- [**seo-by-rank-math**](https://github.com/rankmath/seo-by-rank-math) — PHP

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
