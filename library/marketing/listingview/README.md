# ListingView CLI

**Every ListingView research tool in your terminal, plus a local database that adds drift tracking, niche scoring, and offline analysis no ListingView tool keeps.**

ListingView's keyword, listing, shop, and tag research — backed by a local SQLite store that turns point-in-time estimates into change-over-time intelligence. Run `niche` for a one-command go/no-go, `drift` to catch what moved since last week, `tags consensus` for the tag set winners actually use, and `opportunities` to rank everything you've researched without spending more of your monthly quota.

Learn more at [ListingView](https://app.listingview.io).

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `listingview-pp-cli` binary and the `pp-listingview` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install listingview
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install listingview --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install listingview --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install listingview --agent claude-code
npx -y @mvanhorn/printing-press-library install listingview --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/listingview/cmd/listingview-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/listingview-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install listingview --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-listingview --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-listingview --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install listingview --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
listingview-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/listingview-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/listingview/cmd/listingview-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "listingview": {
      "command": "listingview-pp-mcp"
    }
  }
}
```

</details>

For the HTTP MCP transport, set `PP_MCP_HTTP_TOKEN` in the server environment. It binds to `127.0.0.1:7777` by default. A non-loopback address also requires `--tls-cert` and `--tls-key`.

## Authentication

ListingView has no public API key. The CLI authenticates with your logged-in browser session: run `listingview-pp-cli auth login --chrome` to import your app.listingview.io cookies (the CLI derives the required X-CSRF-Token header from them automatically). The free plan covers the entire research surface.

## Quick Start

```bash
# Check config and reachability before doing anything else.
listingview-pp-cli doctor --dry-run

# Find the tags the top sellers actually use for a term.
listingview-pp-cli tags consensus "vinyl sticker" --agent

# Get a go/no-go verdict on a niche idea.
listingview-pp-cli niche "retro cat mom sweatshirt" --agent

# See what changed across your saved research since last week.
listingview-pp-cli drift --since 7d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Composite Etsy intelligence
- **`niche`** — Get a single go/no-go verdict on an Etsy niche, combining keyword demand, competition, best-seller density, price band, and how winnable the current top sellers are.

  _Reach for this before committing design time to a niche — one command replaces five hand-checks across separate tools._

  ```bash
  listingview-pp-cli niche "retro cat mom sweatshirt" --agent
  ```
- **`listings audit`** — Audit a listing's tags: extract the tags it uses and grade each by ListingView's opportunity, demand, competition, and velocity scores, flagging dead-weight tags and proposing stronger replacements.

  _Use this to revive underperforming listings — it tells you which specific tag to swap, not just that the listing is weak._

  ```bash
  listingview-pp-cli listings audit 1581100221 --agent
  ```
- **`tags consensus`** — Find the tags that repeatedly appear across the top-selling listings for a term, ranked by frequency weighted by each listing's revenue estimate.

  _Use this when writing a new listing's tags — it shows the tag set the winners actually use._

  ```bash
  listingview-pp-cli tags consensus "vinyl sticker" --agent
  ```
- **`gaps`** — Compare your shop's tags/keywords against a competitor's, ranked by the revenue those tags drive for the competitor.

  _Use this to find the SEO coverage a winning competitor has that you don't._

  ```bash
  listingview-pp-cli gaps MyShop RivalShop --agent
  ```

### Local state that compounds
- **`drift`** — Re-run your saved keyword/listing/shop queries and diff against the last snapshot: volume changes, new best-seller entrants, price drops, and SEO/rank movement.

  _Reach for this to catch the week a rival launches a bestseller or a keyword starts climbing — invisible in the web UI._

  ```bash
  listingview-pp-cli drift --since 7d --agent
  ```
- **`tags rising`** — Rank tags by ListingView's velocity score (accelerating demand) while competition is still low, surfacing early-mover windows.

  _Reach for this to get into a trend before it saturates._

  ```bash
  listingview-pp-cli tags rising "sticker" --min-competition-score 50 --agent
  ```
- **`opportunities`** — Scan everything you've already researched and rank the best untapped plays: high demand, low competition, rising velocity — with zero new API calls.

  _Reach for this to turn a month of scattered research into a ranked shortlist without spending more quota._

  ```bash
  listingview-pp-cli opportunities --limit 10 --agent
  ```

## Recipes


### Validate a niche before designing

```bash
listingview-pp-cli niche "personalized dog bandana" --agent
```

One composite verdict from keyword demand, best-seller density, price band, and winnability.

### Audit a listing's SEO and tags

```bash
listingview-pp-cli listings audit 1581100221 --agent
```

SEO score plus a per-tag grade with replacement suggestions.

### Find the tag set winners use

```bash
listingview-pp-cli tags consensus "vinyl sticker" --agent
```

Revenue-weighted consensus tags across the term's top sellers.

### Narrow a nested response for an agent

```bash
listingview-pp-cli tags consensus "vinyl sticker" --agent --select tags.tag,tags.share_pct
```

Pull only the fields you need from a nested response with dotted --select paths to save agent context.

### See what moved this week

```bash
listingview-pp-cli drift --since 7d --agent
```

Diff saved snapshots for volume changes, new best-sellers, and price drops.

## Usage

Run `listingview-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LISTINGVIEW_CONFIG_DIR`, `LISTINGVIEW_DATA_DIR`, `LISTINGVIEW_STATE_DIR`, or `LISTINGVIEW_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LISTINGVIEW_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LISTINGVIEW_HOME=/srv/listingview
listingview-pp-cli doctor
```

Under `LISTINGVIEW_HOME=/srv/listingview`, the four dirs resolve to `/srv/listingview/config`, `/srv/listingview/data`, `/srv/listingview/state`, and `/srv/listingview/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "listingview": {
      "command": "listingview-pp-mcp",
      "env": {
        "LISTINGVIEW_HOME": "/srv/listingview"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LISTINGVIEW_DATA_DIR` overrides an explicit `--home` for that kind. Use `LISTINGVIEW_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LISTINGVIEW_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `listingview-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### account

Account: who am I, plan, and connected shops.

- **`listingview-pp-cli account`** - Show the authenticated user, plan, and connected Etsy shops.

### discover

Discovery helpers: popular search terms and your recent research history.

- **`listingview-pp-cli discover popular`** - Popular search terms for a research type.
- **`listingview-pp-cli discover recent`** - Your recent research history for a type.

### keywords

Search Term Analyzer — Etsy keyword research: search volume, competition, demand, recommendation score.

- **`listingview-pp-cli keywords`** - Search/rank keywords by volume, competition, and demand across ListingView's keyword database.

### listings

Listing research: search the 140M+ listing database and explore any listing's analytics.

- **`listingview-pp-cli listings`** - Search and rank Etsy listings by sales, revenue, and trend across ListingView's database.

### shops

Shop research: search the shop database and analyze any shop's performance.

- **`listingview-pp-cli shops listings`** - Top listings for a shop with sales/revenue, sortable.
- **`listingview-pp-cli shops search`** - Search and rank Etsy shops by sales, revenue, and trend.

### tags

Tag intelligence: search tags, analyze a tag, generate market-validated tags, and extract a listing's tags.

- **`listingview-pp-cli tags analytics`** - Trend analytics over time for a tag.
- **`listingview-pp-cli tags analyze`** - Tag Analyzer — opportunity/demand/competition/velocity scores for a tag.
- **`listingview-pp-cli tags extract`** - Tag Extractor — extract the tags a given listing uses.
- **`listingview-pp-cli tags generate`** - Tag Generator — generate market-validated tags from a keyword.
- **`listingview-pp-cli tags listings`** - Top listings using a tag.
- **`listingview-pp-cli tags search`** - Search and rank Etsy tags by opportunity, demand, competition, and velocity.
- **`listingview-pp-cli tags shops`** - Top shops using a tag.

### watchlist

Saved items: list watchlisted listings/shops/keywords and add or remove favourites.

- **`listingview-pp-cli watchlist list`** - List saved/favourited items (listings, shops, or keywords).
- **`listingview-pp-cli watchlist toggle`** - Add or remove an item from the watchlist.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
listingview-pp-cli keywords --search example-value

# JSON for scripting and agents
listingview-pp-cli keywords --search example-value --json

# Filter to specific fields
listingview-pp-cli keywords --search example-value --json --select id,name,status

# Dry run — show the request without sending
listingview-pp-cli keywords --search example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
listingview-pp-cli keywords --search example-value --agent
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
listingview-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `listingview-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/listingview-pp-cli/config.toml`; `--home`, `LISTINGVIEW_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LISTINGVIEW_COOKIES` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `listingview-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `listingview-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LISTINGVIEW_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / empty results on every command** — Your session expired. Re-run `listingview-pp-cli auth login --chrome` while logged in to app.listingview.io.
- **drift or opportunities returns nothing** — Those read the local store — run some research commands (or `sync`) first so there is history to compare.
- **429 / quota errors** — ListingView free tier allows ~50 uses/month per tool. Lean on cached results and `opportunities`/`drift`, which make zero new API calls.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://app.listingview.io
- Capture coverage: 20 API entries from 20 total network entries
- Reachability: browser_clearance_http (82% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: cookie_session_csrf; cookie — cookies: REDACTED
- Generation hints: browser_clearance_required, requires_browser_auth
- Candidate command ideas: create_add_remove_favourite — Derived from observed POST /api/proxy/api/integration/etsy/add-remove-favourite traffic.; create_analytics — Derived from observed POST /api/proxy/api/integration/etsy/listing-explorer/analytics traffic.; create_getFilteredKeywords — Derived from observed POST /api/proxy/api/integration/etsy/getFilteredKeywords traffic.; create_getFilteredListings — Derived from observed POST /api/proxy/api/integration/etsy/getFilteredListings traffic.; create_getFilteredTags — Derived from observed POST /api/proxy/api/integration/etsy/getFilteredTags traffic.; create_get_filtered_shops — Derived from observed POST /api/proxy/api/integration/etsy/get-filtered-shops traffic.; create_listings — Derived from observed POST /api/proxy/api/integration/etsy/shop-analyzer/listings traffic.; create_progress — Derived from observed POST /api/proxy/api/integration/etsy/shop-analyzer/progress traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
