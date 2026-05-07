# Food52 CLI

**Search, browse, and read Food52 from your terminal — with offline FTS, pantry matching, recipe scaling, and the editorial signals other tools throw away.**

Every recipe and article on Food52, queryable without a browser. Ships with `pantry match` (find recipes from what you already have), `search` (offline FTS over your synced cookbook), `recipes top` (Test-Kitchen approved + rating-floored), and `scale` (resize ingredient lists via JSON-LD). The only existing Food52 CLI is a 2018-era Ruby HTML scraper that no longer runs against today's Vercel-protected site; this is a clean rebuild on Surf with Chrome TLS impersonation.

Learn more at [Food52](https://food52.com).

## Install

The recommended path installs both the `food52-pp-cli` binary and the `pp-food52` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install food52
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install food52 --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/cmd/food52-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/food52-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-food52 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-food52 --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-food52 skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-food52. The skill defines how its required CLI can be installed.
```

## Authentication

No Food52 sign-in required. Food52 sits behind Vercel bot mitigation, but the challenge is passive (TLS-fingerprint), not JS-active — Surf with Chrome impersonation clears it without cookies or setup. Search uses a Typesense search-only key the CLI auto-discovers from Food52's public JS bundle, so users never see a key prompt or env var.

## Quick Start

```bash
# Live Typesense search; sub-second results.
food52-pp-cli recipes search "brownies" --limit 5 --json

# Full structured recipe (ingredients, steps, ratings, kitchen notes) for one recipe.
food52-pp-cli recipes get sarah-fennel-s-best-lunch-lady-brownie-recipe --json

# Seed the local store with two tags worth of recipes — required for offline search and pantry match.
food52-pp-cli sync recipes chicken vegetarian --limit 100

# Tell the CLI what's in your kitchen.
food52-pp-cli pantry add chicken garlic lemon thyme

# Find synced recipes you can mostly make right now.
food52-pp-cli pantry match --min-coverage 0.6 --json

# Clean cooking-friendly view, ready to pipe to lp or paste.
food52-pp-cli print sarah-fennel-s-best-lunch-lady-brownie-recipe

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`pantry match`** — Find Food52 recipes whose ingredients overlap your local pantry, ranked by coverage.

  _Reach for this when the user asks 'what can I make with what I have' rather than searching for one dish at a time._

  ```bash
  food52-pp-cli pantry match --min-coverage 0.7 --json
  ```
- **`search`** — Full-text search across every recipe and article you have synced, with type filtering.

  _Use this for lookups that can't justify a Typesense round trip or when offline._

  ```bash
  food52-pp-cli search "miso" --type recipe --json
  ```
- **`sync recipes`** — Pull recipes for one or more tags into the local FTS-indexed store.

  _Run before pantry match or offline search to seed the local cookbook._

  ```bash
  food52-pp-cli sync recipes chicken vegetarian --limit 100
  ```
- **`articles for-recipe`** — Find synced articles that mention a given recipe in their relatedReading.

  _Use when the user wants the editorial context behind a recipe they've found._

  ```bash
  food52-pp-cli articles for-recipe sarah-fennel-s-best-lunch-lady-brownie-recipe
  ```

### Editorial signals others ignore
- **`recipes top`** — Show only Food52 Test-Kitchen-approved recipes for a tag, with a rating floor.

  _Pick this over a broad search when the user wants 'a recipe Food52's editors signed off on,' not just any community recipe._

  ```bash
  food52-pp-cli recipes top chicken --min-rating 4 --limit 5 --json
  ```

### Recipe transforms
- **`scale`** — Scale a recipe's ingredients to a different number of servings using its Schema.org recipeYield.

  _Use when the user is cooking for a different headcount than the recipe's default yield._

  ```bash
  food52-pp-cli scale mom-s-japanese-curry-chicken-with-radish-and-cauliflower --servings 8 --json
  ```
- **`print`** — Render a recipe as ingredients + numbered steps with no nav, no images, no ads, no comments — ready to pipe to lp or paste into notes.

  _Use when the user wants to actually cook from the recipe rather than browse it._

  ```bash
  food52-pp-cli print sarah-fennel-s-best-lunch-lady-brownie-recipe
  ```

## Usage

Run `food52-pp-cli --help` for the full command reference and flag list.

## Commands

### articles

Browse and read Food52 stories (articles) from the food and life verticals

- **`food52-pp-cli articles browse`** - Browse the latest Food52 articles in a vertical (food, life)
- **`food52-pp-cli articles get`** - Get a Food52 article (story) by slug

### recipes

Browse Food52 recipes by tag and fetch single recipe details (extracted from Next.js __NEXT_DATA__ embedded in SSR HTML)

- **`food52-pp-cli recipes browse`** - Browse Food52 recipes filtered by a tag (e.g. chicken, breakfast, vegetarian)
- **`food52-pp-cli recipes get`** - Get full structured details for a single Food52 recipe by slug

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
food52-pp-cli articles get best-mothers-day-gift-ideas

# JSON for scripting and agents
food52-pp-cli articles get best-mothers-day-gift-ideas --json

# Filter to specific fields
food52-pp-cli articles get best-mothers-day-gift-ideas --json --select id,name,status

# Dry run — show the request without sending
food52-pp-cli articles get best-mothers-day-gift-ideas --dry-run

# Agent mode — JSON + compact + no prompts in one flag
food52-pp-cli articles get best-mothers-day-gift-ideas --agent
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

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add food52 food52-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "food52": {
      "command": "food52-pp-mcp"
    }
  }
}
```

## Health Check

```bash
food52-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/food52-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 or 'Vercel Security Checkpoint' on every request** — Confirm you are running the printed CLI (which uses Surf + Chrome impersonation) and not your own curl. Plain HTTP is blocked by design; the CLI's transport handles it.
- **recipes search returns 'Typesense key discovery failed'** — Run `food52-pp-cli doctor` — it re-fetches the public _app.js bundle. Food52 rotates the bundle hash on each deploy; the CLI auto-recovers.
- **recipes get returns 404 for a slug that exists** — Food52 sometimes renames slugs. Try `recipes search '<title-fragment>'` to find the current canonical slug.
- **pantry match returns nothing** — Run `sync recipes <tag>` first — pantry match operates on the synced store, not the live site. Try `food52-pp-cli search` to confirm sync wrote rows.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://food52.com/
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_http (95% confidence)
- Protocols: ssr_embedded_data (100% confidence), next_data_json (100% confidence), rest_json (100% confidence), schema_org_jsonld (100% confidence)
- Auth signals: none
- Protection signals: vercel_bot_mitigation_passive_tls (100% confidence)
- Generation hints: browser_http_transport
- Candidate command ideas: recipes search; recipes browse; recipes get; tags list; verticals list; articles browse; articles get

Warnings from discovery:
- scope_note: Hotline (/hotline, /hotline/questions/<topic>) returns only siteSettings in pageProps and renders no question/answer data in the DOM. The community Q&A is effectively unreachable for unauthenticated read scrapers. Excluded from CLI scope.
- scope_note: Shop endpoints exist on shop.food52.com and food52.myshopify.com/api/2025-01/graphql.json but require Shopify Storefront API token discovery; deferred from v1.
- scope_note: buildId (Pq8fQj0nm7uTx90i5u0si at the time of browser-sniff) and _app.js bundle hash change on every Food52 deploy. The CLI's runtime discovery (buildId from __NEXT_DATA__.buildId, key from regex over the active _app-<hash>.js) handles rotation transparently.
- scope_note: Typesense /collections endpoint requires the admin key (search-only key returns 403 there). The CLI does not list collections; it queries the known collection name 'recipes_production_food52_current' directly.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hhursev/recipe-scrapers**](https://github.com/hhursev/recipe-scrapers) — Python (1900 stars)
- [**imRohan/food52-cli**](https://github.com/imRohan/food52-cli) — Ruby

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
