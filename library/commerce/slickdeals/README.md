# Slickdeals CLI

**Slickdeals live RSS surface + local-snapshot transcendence from your terminal — agent-native, MCP-compatible, SQLite-backed.**

v0.2 release. Wraps Slickdeals' live RSS feed and Nuxt JSON endpoints in a Go CLI with agent-native `--json` output, local SQLite snapshot store for deal-tracking across time, and a companion MCP server. v0.2 is the read surface + transcendence release. Authenticated write features (vote/comment/submit/DM) are deferred to v0.3.

Learn more at [Slickdeals](https://slickdeals.net).

## v0.2 highlights

- **`hot`** — top frontpage deals filtered by min-thumbs, sorted by community score
- **`frontpage-fresh`** — live unfiltered frontpage RSS (newest first)
- **`search "<q>" --live`** — client-side keyword filter on the live frontpage feed
- **`category <id|name>`** — browse deals by forum category (tech, gaming, home, etc.)
- **`coupons`** — live featured coupons via Nuxt JSON (RSS coupon filter is broken)
- **`watch <deal-id> --persist`** — fetch a single deal and snapshot it to SQLite
- **`digest`** — summarize captured snapshots over a time window
- **`deals`** — SQL compound query over local snapshots (store/category/since/thumbs)
- **`analytics top-stores`** — merchant leaderboard over a configurable window
- **`analytics thumbs-velocity <deal-id>`** — time-series of thumb counts with deltas

## Install

The recommended path installs both the `slickdeals-pp-cli` binary and the `pp-slickdeals` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install slickdeals
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install slickdeals --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install slickdeals --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install slickdeals --agent claude-code
npx -y @mvanhorn/printing-press install slickdeals --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/cmd/slickdeals-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slickdeals-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-slickdeals --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-slickdeals --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-slickdeals skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-slickdeals. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/slickdeals-current).
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
    "slickdeals": {
      "command": "slickdeals-pp-mcp"
    }
  }
}
```

</details>

## Authentication

v0.2 uses no authentication — all wrapped endpoints are public RSS feeds and Nuxt JSON endpoints. Authenticated write features (vote/comment/submit/DM) are deferred to v0.3 (a throwaway account was banned during v0.1 capture attempts; v0.3 will require camofox-browser stealth and a fresh account).

## Quick Start

```bash
# Verify the binary and HTTP transport are healthy.
slickdeals-pp-cli doctor

# Top 10 hottest deals right now (live RSS, sorted by thumbs).
slickdeals-pp-cli hot --json --limit 10

# Live unfiltered frontpage feed.
slickdeals-pp-cli frontpage-fresh --json --limit 25

# Browse tech category deals.
slickdeals-pp-cli category tech --json

# Featured coupons.
slickdeals-pp-cli coupons --json

# Snapshot a deal to the local store (run periodically to build history).
slickdeals-pp-cli watch 19510173 --persist --json

# Daily digest of captured deals.
slickdeals-pp-cli digest --since 24h --top 20 --merchant-cap 3 --json

# Compound SQL query over local snapshots.
slickdeals-pp-cli deals --store costco --since 24h --min-thumbs 50 --json

# Merchant leaderboard.
slickdeals-pp-cli analytics top-stores --window 7d --json
```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`hot`** — Top live frontpage deals filtered by min-thumbs, sorted thumbs DESC.
- **`frontpage-fresh`** — Live unfiltered frontpage RSS feed (today's drops).
- **`search --live`** — Keyword filter on the live frontpage RSS as a fallback for Slickdeals' broken server-side search.
- **`category`** — Curated forum-id -> keyword map (tech, gaming, home, grocery, apparel, sports) for client-side filtering of the live frontpage.
- **`coupons`** — Live featured coupon list with optional --store merchant filter.
- **`watch`** — Fetch a single deal from the live frontpage RSS, optionally persisting a snapshot row for time-series analytics.
- **`digest`** — Summarize the top-N captured snapshots over a window, optionally capped per merchant and grouped by merchant/category.
- **`deals`** — Flagship SQL compound query over captured snapshots: --store costco --since 24h --min-thumbs 50.
- **`analytics top-stores`** — Merchant aggregation over a time window: deal_count, avg_thumbs, max_thumbs, first/last seen.
- **`analytics thumbs-velocity`** — Chronological thumb-count observations for a deal with per-step delta — momentum signal for arbitrage and auto-snipe.

## Usage

Run `slickdeals-pp-cli --help` for the full command reference and flag list.

## Commands

### v0.1 Commands (Nuxt endpoint wrappers)

#### ad-stats

Operations on ad-events

- **`slickdeals-pp-cli ad-stats create_ad_events`** - POST /ad-stats/{id}/ad-events

#### ajax

Operations on bSubNavPlacement.php

- **`slickdeals-pp-cli ajax create_threadrate.php`** - POST /ajax/threadrate.php
- **`slickdeals-pp-cli ajax list_bSubNavPlacement.php`** - GET /ajax/bSubNavPlacement.php

#### frontpage

Operations on recommendations

- **`slickdeals-pp-cli frontpage list_json`** - GET /frontpage/promoted-content/json
- **`slickdeals-pp-cli frontpage list_recommendations`** - GET /frontpage/recommendation-carousel/recommendations

#### web-api

Operations on missed-deals

- **`slickdeals-pp-cli web-api list_featured_coupons`** - GET /web-api/frontpage/featured-coupons/
- **`slickdeals-pp-cli web-api list_missed_deals`** - GET /web-api/frontpage/missed-deals/

### v0.2 Commands (Live RSS + local snapshot transcendence)

#### hot

Top Slickdeals frontpage deals by thumb count (live RSS). Filters client-side by `--min-thumbs` (default 20), sorted descending.

- **`slickdeals-pp-cli hot [--min-thumbs N] [--limit N]`** - Pulls live frontpage RSS and surfaces high-thumb deals

#### frontpage-fresh

Fresh Slickdeals frontpage RSS feed (live, unfiltered). Newest-first, direct from `/newsearch.php?mode=frontpage&rss=1`.

- **`slickdeals-pp-cli frontpage-fresh [--limit N]`** - Live unfiltered frontpage RSS

#### search

Full-text search on locally synced data or live RSS. `--live` hits the RSS search endpoint (client-side keyword filter on frontpage feed; Slickdeals ignores server-side `search=` params).

- **`slickdeals-pp-cli search "<query>" [--live] [--limit N]`** - FTS5 local search or live RSS keyword filter

#### category

Browse deals by Slickdeals forum category. Pass a numeric forum ID or friendly name. `--list` prints the built-in category→forum-ID map.

- **`slickdeals-pp-cli category <id|name> [--limit N] [--list]`** - Category-filtered live RSS deals

#### coupons

Live featured coupons via the Nuxt JSON endpoint. RSS coupon filter (`f2=1`) does not work.

- **`slickdeals-pp-cli coupons [--store <name>] [--limit N]`** - Featured coupons from `/web-api/frontpage/featured-coupons/`

#### watch

Fetch a single deal by ID from the live frontpage RSS. Use `--persist` to write the snapshot to the local SQLite store.

- **`slickdeals-pp-cli watch <deal-id> [--persist] [--once]`** - Snapshot a deal from the live frontpage feed

#### digest

Summarize top deals from the local snapshot store over a time window.

- **`slickdeals-pp-cli digest [--since 24h] [--top N] [--merchant-cap N] [--grouped-by merchant|category]`** - Window-based deal summary from local snapshots

#### deals

SQL compound query over the local `deal_snapshots` table.

- **`slickdeals-pp-cli deals [--store <name>] [--category <name>] [--since <dur>] [--min-thumbs N] [--deal-id <id>] [--latest]`** - Compound query over local snapshots

#### analytics

Analyze local snapshot data with aggregation sub-commands.

- **`slickdeals-pp-cli analytics top-stores [--window 30d] [--limit N]`** - Merchant leaderboard by deal count and max thumbs
- **`slickdeals-pp-cli analytics thumbs-velocity <deal-id>`** - Chronological thumb-count series with per-observation deltas

## Local SQLite snapshot store

v0.2 introduces a local `deal_snapshots` table (default: `~/.local/share/slickdeals-pp-cli/data.db`). Each row records one observation of a deal at a point in time.

**Schema columns:** `deal_id`, `title`, `link`, `merchant`, `category`, `thumbs`, `captured_at`.

**Populating snapshots:**

```bash
# One-shot: fetch deal 19510173 from the live frontpage and save it
slickdeals-pp-cli watch 19510173 --persist --json

# Run periodically (e.g. via cron) to build a history for velocity tracking
# Example: every 30 minutes for a deal you're watching
# */30 * * * * slickdeals-pp-cli watch 19510173 --persist
```

**Querying snapshots:**

```bash
# Compound SQL filter
slickdeals-pp-cli deals --store amazon --since 24h --min-thumbs 30 --json

# Window-based digest with merchant deduplication
slickdeals-pp-cli digest --since 7d --top 20 --merchant-cap 3 --json

# Merchant leaderboard
slickdeals-pp-cli analytics top-stores --window 30d --json

# Thumb velocity for a single deal
slickdeals-pp-cli analytics thumbs-velocity 19510173 --json
```

The snapshot store is append-only per observation. `deals` deduplicates to the latest snapshot per `deal_id` by default (`--latest=true`); pass `--latest=false` to see every observation.

## Architecture decisions

- **Client-side RSS filtering:** Slickdeals' RSS endpoint (`/newsearch.php`) does not honor server-side `search=` or `forumid=` filters when `mode=frontpage` is set — it returns the full frontpage feed regardless. v0.2's `search`, `category`, and `hot` commands all filter client-side against the same live frontpage RSS feed.

- **Coupons use the Nuxt endpoint:** The RSS coupon-filter pattern (`newsearch.php?filter=f2=1&rss=1`) does not work — Slickdeals ignores the filter and returns the frontpage feed. The `coupons` command uses `/web-api/frontpage/featured-coupons/` instead, which returns the actual featured-coupon set.

- **Authenticated write features deferred to v0.3:** Vote/comment/submit/DM actions require an authenticated session. A throwaway account was banned during v0.1 capture attempts. v0.3 will require camofox-browser stealth mode and a fresh account to avoid fingerprinting.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
slickdeals-pp-cli ad-stats mock-value --breakpoint example-value

# JSON for scripting and agents
slickdeals-pp-cli ad-stats mock-value --breakpoint example-value --json

# Filter to specific fields
slickdeals-pp-cli ad-stats mock-value --breakpoint example-value --json --select id,name,status

# Dry run — show the request without sending
slickdeals-pp-cli ad-stats mock-value --breakpoint example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
slickdeals-pp-cli ad-stats mock-value --breakpoint example-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
slickdeals-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/slickdeals-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports HTTP transport failure** — Slickdeals occasionally throttles per-IP. Wait a minute and retry; the discovered endpoints work via plain HTTPS (no clearance cookie needed per probe-reachability).
- **Empty results from list commands** — Run `slickdeals-pp-cli sync` first to populate the local cache, then retry.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**slickdeals-clean-url**](https://github.com/schrauger/slickdeals-clean-url) — JavaScript (11 stars)
- [**Slickdeals-Discord-Bot**](https://github.com/Unwoundd/Slickdeals-Discord-Bot) — Python (7 stars)
- [**bargainer-mcp-client**](https://github.com/karthiksivaramms/bargainer-mcp-client) — JavaScript (4 stars)
- [**SlickDealsScraper**](https://github.com/carvalhe/SlickDealsScraper) — Python (4 stars)
- [**slickdeals-affiliate-link-remover**](https://github.com/norrism/slickdeals-affiliate-link-remover) — JavaScript (4 stars)
- [**slickdeals (MinweiShen)**](https://github.com/MinweiShen/slickdeals) — Python (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
