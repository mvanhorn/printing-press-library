# KDP Niche Finder CLI

**Every KDP Niche Finder bucket as a local research database — rank niches across buckets, track whether revenue is rising or fading, and export shortlists, none of which the web tool does.**

KDP Niche Finder is a click-to-browse web tool that shows one curated niche bucket at a time with no history and no export. This CLI mirrors all four buckets into local SQLite so you can rank niches by opportunity across buckets (rank), see which niches are rising or fading since your last refresh (drift), spot books that appear in multiple buckets (dupes), gauge publisher saturation, and export KDP-ready CSVs — all offline and agent-native.

Learn more at [KDP Niche Finder](https://kdpnichefinder.com).

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).

## Install

The recommended path installs both the `kdpnichefinder-pp-cli` binary and the `pp-kdpnichefinder` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder --agent claude-code
npx -y @mvanhorn/printing-press-library install kdpnichefinder --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/cmd/kdpnichefinder-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kdpnichefinder-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-kdpnichefinder --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-kdpnichefinder --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install kdpnichefinder --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
kdpnichefinder-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kdpnichefinder-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/cmd/kdpnichefinder-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kdpnichefinder": {
      "command": "kdpnichefinder-pp-mcp"
    }
  }
}
```

</details>

## Authentication

KDP Niche Finder uses a Laravel session login (no API key). Run `kdpnichefinder-pp-cli auth login --chrome` to capture your logged-in kdpnichefinder.com browser session; reads use the session cookie and saves/folders additionally send the CSRF token the CLI composes for you.

## Quick Start

```bash
# Check config and reachability before anything else (no auth needed).
kdpnichefinder-pp-cli doctor --dry-run

# Mirror all four niche buckets into the local store (also snapshots revenue for drift).
kdpnichefinder-pp-cli refresh

# Rank the best low-price niches across every bucket at once.
kdpnichefinder-pp-cli rank --max-price 9.99 --sort value

# After a second refresh on a later day, see which niches are gaining revenue.
kdpnichefinder-pp-cli drift --since 7d --sort rising

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`rank`** — Rank niches across all four buckets at once by a composite of estimated revenue, sales, and price.

  _Pick this when an agent needs the single best niche by opportunity, not a bucket-by-bucket browse._

  ```bash
  kdpnichefinder-pp-cli rank --max-price 9.99 --sort value --agent
  ```
- **`drift`** — Show which synced niches are rising or fading in estimated revenue versus an earlier snapshot.

  _Pick this for timing decisions (publish now vs skip) that the source tool cannot answer at all._

  ```bash
  kdpnichefinder-pp-cli drift --since 7d --sort rising --agent
  ```
- **`keywords`** — Tokenize synced book titles and count keyword frequency to surface hot terms for a niche.

  _Pick this to seed KDP backend keyword fields from what is actually selling._

  ```bash
  kdpnichefinder-pp-cli keywords --type evergreen --min-count 3 --agent
  ```

### Cross-bucket analysis
- **`dupes`** — Find books that appear in more than one niche bucket (same ASIN) and show which buckets.

  _Pick this to spot niches surfacing in multiple buckets, a strong cross-validated signal._

  ```bash
  kdpnichefinder-pp-cli dupes --agent
  ```
- **`saturation`** — Per bucket, show how concentrated estimated revenue is among publishers (whale vs fragmented).

  _Pick this to tell an open niche from one a single publisher already dominates._

  ```bash
  kdpnichefinder-pp-cli saturation --type hidden_gems --agent
  ```
- **`competitors`** — For a focus book, list same-publisher and same-price-band competitors using the extracted ASIN.

  _Pick this to study who else is winning a specific niche before committing to it._

  ```bash
  kdpnichefinder-pp-cli competitors 2584 --agent
  ```

### Agent-native plumbing
- **`export`** — Export title, ASIN, price, estimated sales, and revenue as CSV for KDP backend keyword and cover work.

  _Pick this to hand a shortlist off to keyword/cover tooling without retyping titles._

  ```bash
  kdpnichefinder-pp-cli export --csv
  ```

## Recipes


### Best low-price niche across all buckets

```bash
kdpnichefinder-pp-cli rank --max-price 9.99 --sort value --agent
```

Ranks every synced book by revenue-per-dollar so cheap, high-return niches surface first.

### Narrow a verbose niche list to the fields that matter

```bash
kdpnichefinder-pp-cli niches hidden_gems --search journal --agent --select title,estimated_monthly_revenue,amazon_url
```

Uses dotted --select paths to pull just title, revenue, and Amazon link from the paginated bucket response.

### Spot rising niches week over week

```bash
kdpnichefinder-pp-cli drift --since 7d --sort rising --agent
```

Diffs the latest snapshot against one a week ago to show niches gaining estimated revenue.

### Export a saved shortlist for keyword work

```bash
kdpnichefinder-pp-cli export --csv
```

Emits title/ASIN/price/sales/revenue as CSV for KDP backend keyword and cover tooling.

## Usage

Run `kdpnichefinder-pp-cli --help` for the full command reference and flag list.

## Commands

### books

Save and unsave niche books

- **`kdpnichefinder-pp-cli books <book_id>`** - Toggle save/unsave a book; optionally into a folder

### categories

Niche bucket metadata

- **`kdpnichefinder-pp-cli categories`** - List niche bucket categories (key, name, description)

### folders

Organize saved niches into folders

- **`kdpnichefinder-pp-cli folders create`** - Create a folder
- **`kdpnichefinder-pp-cli folders list`** - List your folders

### niches

Browse curated KDP niche buckets (real Amazon books with estimated sales/revenue)

- **`kdpnichefinder-pp-cli niches <type>`** - Browse a niche bucket: evergreen, fresh_money, hidden_gems, or high_ticket

### saved

Your saved niche books

- **`kdpnichefinder-pp-cli saved`** - List your saved books

### user

Authenticated account

- **`kdpnichefinder-pp-cli user`** - Show the authenticated user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kdpnichefinder-pp-cli categories

# JSON for scripting and agents
kdpnichefinder-pp-cli categories --json

# Filter to specific fields
kdpnichefinder-pp-cli categories --json --select id,name,status

# Dry run — show the request without sending
kdpnichefinder-pp-cli categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
kdpnichefinder-pp-cli categories --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
kdpnichefinder-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/kdpnichefinder-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `kdpnichefinder-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return 401 / redirect to login** — Your session expired; re-run `kdpnichefinder-pp-cli auth login --chrome`.
- **drift shows nothing** — drift needs at least two refreshes on different dates; run `refresh` again tomorrow.
- **Saving a book fails with a CSRF/419 error** — Re-run `auth login --chrome` to refresh the XSRF token cookie.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://kdpnichefinder.com/dashboard
- Capture coverage: 60 API entries from 273 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence), html_scrape (55% confidence)
- Candidate command ideas: create_folders — Derived from observed POST /api/folders traffic.; create_toggle_save — Derived from observed POST /api/books/{book_id}/toggle-save traffic.; list_categories — Derived from observed GET /api/categories traffic.; list_evergreen — Derived from observed GET /app/category/evergreen traffic.; list_folders — Derived from observed GET /api/folders traffic.; list_fresh_money — Derived from observed GET /app/category/fresh_money traffic.; list_hidden_gems — Derived from observed GET /app/category/hidden_gems traffic.; list_high_ticket — Derived from observed GET /app/category/high_ticket traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
