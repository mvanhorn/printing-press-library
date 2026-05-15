# Mobbin CLI

**Every Mobbin screen, flow, and pattern — searchable offline, deckable in one command, with longitudinal drift no Mobbin tool ships.**

Wraps Mobbin's curated library of shipped UI (web + mobile) with a local SQLite mirror, full-res batch downloads from the Bytescale CDN, and time-windowed audits across apps. Built for the Wednesday design crit and the quarterly onboarding audit. Uses your existing Chrome session via `auth login --chrome` — no extra API key, no paid MCP.

Learn more at [Mobbin](https://mobbin.com).

## Install

The recommended path installs both the `mobbin-pp-cli` binary and the `pp-mobbin` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install mobbin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install mobbin --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mobbin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mobbin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mobbin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-mobbin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-mobbin. The skill defines how its required CLI can be installed.
```

## Authentication

Mobbin has no public API key. The CLI imports your logged-in Chrome cookies via pycookiecheat, cookies, or cookie-scoop-cli (one is required). Run `mobbin-pp-cli auth login --chrome` and the CLI handles the split Supabase JWT cookies (`sb-ujasntkfphywizsdaapi-auth-token.0` / `.1`) and refreshes them automatically. Free-tier sessions work for trending / discover / popular / filters; full content search and per-app HTML scraping need a Mobbin Pro session.

## Quick Start

```bash
# Import your logged-in Mobbin Chrome cookies (multi-profile aware)
mobbin-pp-cli auth login --chrome


# Verify auth + reachability before anything else
mobbin-pp-cli doctor


# Public content — works without auth too, smoke-test the install
mobbin-pp-cli apps popular --platform web --agent


# Discover category slugs before filtering search
mobbin-pp-cli filters list --agent --select data.appCategories


# Cross-app pattern search — the foundational deck-building call
mobbin-pp-cli screens --platform web --screen-patterns paywall --agent --select data.id,data.appName,data.imageUrl


# One-shot design crit deck with full-res PNGs + manifest CSV
mobbin-pp-cli deck "fintech paywalls" --platform web --limit 20 --export-zip ./deck.zip

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Design crit workflows
- **`deck`** — Build a design-crit deck for a pattern across an industry: searches matching screens, downloads full-res images from the Bytescale CDN, packages a zip with a manifest CSV.

  _Replaces the 20-tab screenshot-and-rename loop designers do every crit week with one command and a reproducible artifact._

  ```bash
  mobbin-pp-cli deck "fintech paywalls" --platform web --limit 20 --export-zip ./deck.zip --agent
  ```
- **`bench`** — Cross-app leaderboard for any pattern from the local store: count, last seen, top apps that ship it.

  _Answers "who has shipped this pattern lately?" instantly without re-scrolling Mobbin's UI._

  ```bash
  mobbin-pp-cli bench --pattern paywall --industry fintech --platform web --agent
  ```
- **`grab`** — Batch-download matching screens at 1920px from Bytescale with deterministic filenames and a manifest.json side-car for downstream tooling.

  _Replaces hand-dragging 20 PNGs into Finder. The manifest.json is what Figma/Storyboard plugins consume._

  ```bash
  mobbin-pp-cli grab --pattern empty-state --platform web --industry fintech --out ./refs --rename '{app}_{pattern}_{idx}.png'
  ```
- **`cross`** — Fan out a pattern query across web AND iOS for one app set; join results on app slug; output a side-by-side parity manifest.

  _Web product designers checking whether the iOS reference still matches the desktop pattern. The user's explicit dual-platform ask._

  ```bash
  mobbin-pp-cli cross "paywall" --apps stripe,linear,figma --agent
  ```

### Longitudinal analysis
- **`audit`** — Time-windowed flow audit across an industry: app, flow name, step count, captured_at — with --since support for delta-since-last-quarter reports.

  _Quarterly onboarding/checkout/empty-state audits stop being a manual diff against last quarter's Notion doc._

  ```bash
  mobbin-pp-cli audit onboarding --platform web --industry b2b-saas --since 60d --agent --select app,flow,step_count,captured_at
  ```
- **`drift`** — Diff an app's flows + screens between local snapshots; surface what changed (added/removed/screen count).

  _Tracks competitor product evolution. "What did Stripe ship since last month?" was unanswerable before._

  ```bash
  mobbin-pp-cli drift stripe-web --since 30d --agent
  ```

## Usage

Run `mobbin-pp-cli --help` for the full command reference and flag list.

## Commands

### apps

Browse and search apps in Mobbin's library (web, iOS, Android).

- **`mobbin-pp-cli apps discover`** - Paginated discover-page apps. Tabs are latest / popular / animations.
- **`mobbin-pp-cli apps list`** - List every app for a platform. Returns a flat array of {id, appName, platform, ...}. Large response (~1,000+ apps); cache locally for autocomplete.
- **`mobbin-pp-cli apps popular`** - Popular apps grouped by category with preview screenshots. Web's the default; pair with --platform ios or android for mobile.
- **`mobbin-pp-cli apps search`** - Authenticated app search with category filters. Requires Mobbin Pro session (run `mobbin-pp-cli auth login --chrome` first).

### autocomplete

Cross-entity autocomplete search across apps, screens, and flows.

- **`mobbin-pp-cli autocomplete query`** - Fast autocomplete across apps, screens, and flows. Returns matching IDs grouped by relevance. Requires Mobbin Pro session for full result quality.

### collections

Manage your saved Mobbin collections (decks of screens, flows, or apps). Read endpoints hit mobbin.com/api; write endpoints hit Supabase PostgREST directly (auth-token + apikey).

- **`mobbin-pp-cli collections add-app`** - Add an app to a collection.
- **`mobbin-pp-cli collections add-flow`** - Add a flow to a collection.
- **`mobbin-pp-cli collections add-screen`** - Add a screen to a collection.
- **`mobbin-pp-cli collections contents`** - Items inside a collection, paginated. Bucketed by --content-type and --platform-type.
- **`mobbin-pp-cli collections create`** - Create a new collection in the authenticated user's workspace. Writes hit Supabase PostgREST directly (Bearer + apikey headers).
- **`mobbin-pp-cli collections delete`** - Delete a collection. Uses PostgREST filter ?id=eq.<id>.
- **`mobbin-pp-cli collections list`** - All collections owned by the authenticated user.
- **`mobbin-pp-cli collections remove-screen`** - Remove a screen from a collection.

### filters

Browse the filter taxonomy — every app category, screen pattern, UI element, and flow action with definitions and content counts.

- **`mobbin-pp-cli filters list`** - Full filter taxonomy. Returns the dictionary that powers every search filter — patterns, elements, categories, flow actions, definitions, and content counts.

### flows

Search and browse user-flow recordings (onboarding, checkout, settings).

- **`mobbin-pp-cli flows search`** - Cross-app flow search. Filter by --flow-actions like 'creating-account' or 'subscribing'. Requires Mobbin Pro session.

### screens

Search and inspect individual screens. Filter by pattern, element, OCR keywords, and app category.

- **`mobbin-pp-cli screens`** - Cross-app screen search. Use --screen-patterns 'paywall' or --screen-elements 'search-bar' to filter. Requires Mobbin Pro session.

### sites

Web sites in Mobbin's library (the web-app equivalent of `apps` for mobile).

- **`mobbin-pp-cli sites list`** - Full searchable-sites list for the web experience.

### trending

Trending entities updated daily by Mobbin's editorial team.

- **`mobbin-pp-cli trending apps`** - Trending apps for a platform — what's hot this week on Mobbin.
- **`mobbin-pp-cli trending filter-tags`** - Trending filter tags — patterns, elements, or categories users search for right now.
- **`mobbin-pp-cli trending keywords`** - Trending OCR keywords found inside screenshots — what text-in-screenshots users are searching.
- **`mobbin-pp-cli trending sites`** - Trending web sites (web-only surface).

### workspaces

List your Mobbin workspaces. The default workspace is required to create collections.

- **`mobbin-pp-cli workspaces list`** - All workspaces the authenticated user belongs to.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mobbin-pp-cli apps list mock-value

# JSON for scripting and agents
mobbin-pp-cli apps list mock-value --json

# Filter to specific fields
mobbin-pp-cli apps list mock-value --json --select id,name,status

# Dry run — show the request without sending
mobbin-pp-cli apps list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mobbin-pp-cli apps list mock-value --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-mobbin -g
```

Then invoke `/pp-mobbin <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
mobbin-pp-cli auth login --chrome

claude mcp add mobbin mobbin-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
mobbin-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mobbin-current).
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
    "mobbin": {
      "command": "mobbin-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
mobbin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/mobbin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mobbin-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`auth login --chrome` says "No cookie extraction tool found"** — Install one: `pip install pycookiecheat` (recommended), `brew install barnardb/cookies/cookies`, or `cargo install cookie-scoop-cli`.
- **401 after working for a few minutes** — Supabase access token expired. Run `mobbin-pp-cli auth login --chrome` again to refresh; the daily-refresh fragility is a known Mobbin pattern.
- **`screens search` returns empty or fewer results than expected** — You may be on Mobbin's free tier (latest 4 apps only). `mobbin-pp-cli doctor` reports your plan. Upgrade to Pro at https://mobbin.com/pricing for full library access.
- **`grab` or `deck` downloads PNGs that look low-res** — Bytescale CDN width defaults to 1920px; pass `--width 2560` for higher-DPI captures.
- **`drift` reports "no prior snapshot"** — Drift compares local snapshots over time. Run `mobbin-pp-cli sync <slug>` at least twice (separated by enough days) before expecting a diff.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://mobbin.com/discover/apps/web/latest
- Capture coverage: 71 API entries from 2103 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Generation hints: weak_schema_confidence
- Candidate command ideas: create_fetch_dictionary_definitions — Derived from observed POST /api/filter-tags/fetch-dictionary-definitions traffic.; create_fetch_discover_page_apps — Derived from observed POST /api/discover/fetch-discover-page-apps traffic.; create_fetch_popular_apps_with_preview_screens — Derived from observed POST /api/popular-apps/fetch-popular-apps-with-preview-screens traffic.; create_fetch_searchable_sites — Derived from observed POST /api/search-bar/fetch-searchable-sites traffic.; create_fetch_trending_apps — Derived from observed POST /api/search-bar/fetch-trending-apps traffic.; create_fetch_trending_filter_tags — Derived from observed POST /api/search-bar/fetch-trending-filter-tags traffic.; create_fetch_trending_sites — Derived from observed POST /api/search-bar/fetch-trending-sites traffic.; create_fetch_trending_text_in_screenshot_keywords — Derived from observed POST /api/search-bar/fetch-trending-text-in-screenshot-keywords traffic.

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

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**underthestars-zhy/MobbinAPI**](https://github.com/underthestars-zhy/MobbinAPI) — Swift (39 stars)
- [**pdcolandrea/mobbin-mcp**](https://github.com/pdcolandrea/mobbin-mcp) — TypeScript (31 stars)
- [**ismailsaleekh/mobbin-agent**](https://github.com/ismailsaleekh/mobbin-agent) — TypeScript
- [**solejay/mobbin-cli**](https://lobehub.com/skills/solejay-mobbin-cli-mobbin-app-screens) — Node
- [**Mobbin Official MCP**](https://docs.mobbin.com/mcp/clients/claude-code) — HTTP
- [**YonasValentin/design-inspiration-mcp-server**](https://github.com/YonasValentin/design-inspiration-mcp-server) — TypeScript
- [**Batch Collector for Mobbin (Chrome extension)**](https://chromewebstore.google.com/detail/batch-collector-for-mobbi/kemapbomkjpphecdnhllopknkdgndpom) — JavaScript
- [**iamsahebgiri/mobbin-unlocked**](https://github.com/iamsahebgiri/mobbin-unlocked) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
