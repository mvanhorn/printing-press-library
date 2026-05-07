# Allrecipes CLI

**Every Allrecipes recipe in your terminal — cached as data, with pantry-aware search, Bayesian-smoothed ranking, one-line grocery lists, and Cloudflare clearance.**

Search Allrecipes' 250k-recipe corpus from the command line, fetch a full recipe as parsed JSON-LD (ingredients with quantity+unit+name, instructions, nutrition, ratings, Made-It count), aggregate grocery lists from a meal plan, scale recipes, and export to clean markdown. Every recipe you fetch lands in a local SQLite store, which unlocks `pantry` (which recipes can I cook with what I have), `with-ingredient` (reverse index), `top-rated` with Bayesian smoothing (no more 1-review 5-star noise), and `cookbook` (export a category as a personal cookbook). Recipe detail pages are walled by Cloudflare; one-time `auth login --chrome` captures a clearance cookie from your browser — no Allrecipes account needed.

Learn more at [Allrecipes](https://www.allrecipes.com).

## Install

The recommended path installs both the `allrecipes-pp-cli` binary and the `pp-allrecipes` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install allrecipes
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install allrecipes --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/allrecipes-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

Browse and search work without any auth. Recipe detail pages need a Cloudflare clearance cookie — run `allrecipes-pp-cli auth login --chrome` once to capture one from your browser. This is bot-protection clearance, not an Allrecipes account login: no username, no password, no token tied to a user. The CLI does not support saved-recipes / meal-plan / profile features that require an actual Allrecipes account.

## Quick Start

```bash
# One-time: capture a Cloudflare clearance cookie from your browser. Required for recipe detail pages.
allrecipes-pp-cli auth login --chrome


# Search the live site; results land in your local cache for offline reuse. Browse/search work without clearance.
allrecipes-pp-cli search "brownies" --limit 5 --agent


# Fetch a full recipe as parsed JSON-LD; --select narrows the payload.
allrecipes-pp-cli recipe https://www.allrecipes.com/recipe/9599/quick-and-easy-brownies/ --agent --select recipeIngredient,totalTime,aggregateRating


# Rescale ingredients by servings.
allrecipes-pp-cli scale https://www.allrecipes.com/recipe/9599/quick-and-easy-brownies/ --servings 16


# Match cached recipes against what you already have.
allrecipes-pp-cli pantry --pantry-file ~/pantry.txt --query chicken --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`pantry`** — Score Allrecipes recipes against your pantry — see which ones you can actually cook tonight without a grocery run.

  _When the user says 'what can I make with what I've got', this is the only command that knows the answer._

  ```bash
  allrecipes-pp-cli pantry --pantry-file ~/pantry.txt --query brownies --agent
  ```
- **`with-ingredient`** — Find every cached recipe that uses a given ingredient — a SQL view across your local corpus.

  _Use this when the user starts from an ingredient they want to use up, not a dish name._

  ```bash
  allrecipes-pp-cli with-ingredient buttermilk --top 10 --agent
  ```
- **`dietary`** — Filter cached recipes by gluten-free / vegan / low-carb using JSON-LD keywords plus ingredient-name patterns.

  _Use this when dietary restrictions are non-negotiable and the user wants more than what the site's diet category page surfaces._

  ```bash
  allrecipes-pp-cli dietary --type gluten-free --top 20 --agent
  ```

### Ranking that beats the website
- **`top-rated`** — Rank recipes by Bayesian-smoothed rating — proven popular wins over 1-review 5-star noise.

  _Pick this over raw search when the agent wants proven recipes, not freshly-uploaded 5-star outliers._

  ```bash
  allrecipes-pp-cli top-rated brownies --smooth-c 200 --limit 10 --agent
  ```
- **`quick`** — Top-rated recipes that fit a strict time cap — Allrecipes' UI cannot enforce one, but the local cache can.

  _Use this when the user's constraint is time, not dish — 'what can I make in 25 minutes that's actually good'._

  ```bash
  allrecipes-pp-cli quick --max-minutes 30 --query chicken --agent
  ```

### Agent-native plumbing
- **`cookbook`** — Compile a top-rated category into a single markdown cookbook with TOC, ingredients, and instructions.

  _When the user asks for a curated bundle (gifts, meal-plan packs), this builds it in one command._

  ```bash
  allrecipes-pp-cli cookbook --category italian --top 20 --output italian-cookbook.md
  ```
- **`grocery-list`** — Aggregate ingredients from many recipes into a deduped, agent-readable shopping list.

  _Use this at the end of a meal plan — one call replaces five scrolls through ingredient lists._

  ```bash
  allrecipes-pp-cli grocery-list https://www.allrecipes.com/recipe/9599/quick-and-easy-brownies/ https://www.allrecipes.com/recipe/16354/easy-meatloaf/ --agent
  ```

### Reachability mitigation
- **`doctor`** — Health check that names the Cloudflare 'Just a moment...' interstitial by inspecting the response body, then advises the browser-chrome transport.

  _When the CLI breaks because of bot detection, the agent gets a specific, actionable error rather than a generic timeout._

  ```bash
  allrecipes-pp-cli doctor
  ```

## Usage

Run `allrecipes-pp-cli --help` for the full command reference and flag list.

## Commands

### Search & Browse

| Command | Description |
|---------|-------------|
| `search <query>` | Search Allrecipes for recipes (live + cache) |
| `top-rated <query>` | Search and rank by Bayesian-smoothed rating |
| `category <slug>` | Browse recipes in a category (e.g. dessert, weeknight) |
| `cuisine <slug>` | Browse recipes by cuisine (e.g. italian, mexican, thai) |
| `ingredient <name>` | Browse recipes featuring a primary ingredient |
| `occasion <slug>` | Browse recipes by occasion (holiday, weeknight, party, etc.) |

### Recipe Detail

| Command | Description |
|---------|-------------|
| `recipe <url-or-id>` | Fetch a full recipe; returns parsed JSON-LD |
| `ingredients <url-or-id>` | Show parsed ingredients (`--parsed` for qty+unit+name) |
| `instructions <url-or-id>` | Show numbered instructions |
| `nutrition <url-or-id>` | Show nutrition (per serving by default) |
| `reviews <url-or-id>` | Show aggregate rating, review count, "Made It" summary |
| `scale <url-or-id>` | Rescale ingredients to a target serving count |
| `export-recipe <url-or-id>` | Export recipe as markdown to stdout or `--output` |

### Local-Cache Transcendence

| Command | Description |
|---------|-------------|
| `pantry` | Rank cached recipes by overlap with `--pantry-file` or `--pantry` |
| `with-ingredient <name>` | Reverse index: cached recipes that use a given ingredient |
| `quick` | Top-rated cached recipes that fit `--max-minutes` |
| `dietary` | Filter cached recipes by gluten-free / vegan / low-carb / etc. |
| `cookbook` | Compile a top-N category/cuisine into a markdown cookbook |
| `grocery-list <urls...>` | Aggregate ingredients into a deduped shopping list |

### Other Pages

| Command | Description |
|---------|-------------|
| `article <url>` | Extract metadata from an Allrecipes article page |
| `gallery <url>` | Extract recipe links from a round-up gallery |
| `cook <slug>` | Show a cook profile and the recipes they've published |

### Cache & Data Pipeline

| Command | Description |
|---------|-------------|
| `cache stats` | Show local cache size and on-disk path |
| `cache list` | List cached recipes (sort by `--order` rating/recency/etc.) |
| `cache clear` | Wipe the local cache (requires `--yes`) |
| `sync` | Sync API data to local SQLite |
| `export` | Export data to JSONL or JSON |
| `import` | Import data from JSONL via API create/upsert |

### Account & Diagnostics

| Command | Description |
|---------|-------------|
| `auth login --chrome` | Capture a Cloudflare clearance cookie from your browser |
| `doctor` | Check CLI health (auth, cache, Cloudflare-clearance state) |
| `agent-context` | Emit structured JSON describing this CLI for agents |
| `which <capability>` | Find the command that implements a capability |
| `profile` | Named sets of flags saved for reuse |
| `workflow` | Compound workflows that combine multiple operations |
| `version` / `feedback` / `completion` | Standard utilities |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
allrecipes-pp-cli search brownies --limit 5

# JSON for scripting and agents
allrecipes-pp-cli search brownies --limit 5 --json

# Filter to specific fields
allrecipes-pp-cli recipe 9599/quick-and-easy-brownies --json --select name,recipeIngredient,totalTime

# Dry run — show the request without sending
allrecipes-pp-cli recipe 9599/quick-and-easy-brownies --dry-run

# Agent mode — JSON + compact + no prompts in one flag
allrecipes-pp-cli search brownies --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `ALLRECIPES_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-allrecipes -g
```

Then invoke `/pp-allrecipes <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-mcp@latest
```

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
allrecipes-pp-cli auth login --chrome

claude mcp add allrecipes allrecipes-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-allrecipes --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-allrecipes --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-allrecipes skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-allrecipes. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
allrecipes-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/allrecipes-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "allrecipes": {
      "command": "allrecipes-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
allrecipes-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/allrecipes-pp-cli/config.toml`

Environment variables:
- `ALLRECIPES_COOKIES`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `allrecipes-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ALLRECIPES_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports clearance_required: true** — Run `allrecipes-pp-cli auth login --chrome` to capture a fresh Cloudflare clearance cookie from your browser. Cookies expire periodically; re-run when doctor flags it again.
- **recipe fetch returns 403 with cf-mitigated: challenge** — Same as above — your clearance cookie is missing or expired. Run `allrecipes-pp-cli auth login --chrome` to refresh it. If your network IP is flagged independently, try a different connection.
- **search returns 0 results** — Allrecipes search is text-only; URL-style filter syntax is not supported. Use plain words, then narrow with `--max-minutes` or pipe to `top-rated` for client-side ranking.
- **grocery-list output has duplicate ingredients with different units** — Unit normalization is best-effort; pass `--raw-quantities` to see source strings and aggregate manually for edge cases.
- **pantry/with-ingredient returns no results despite expected matches** — These commands query the local cache only — run `sync` or fetch a few recipes first to populate it. Use `cache stats` to confirm the corpus is non-empty.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport over HTTP/3 for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hhursev/recipe-scrapers**](https://github.com/hhursev/recipe-scrapers) — Python (1900 stars)
- [**jadkins89/Recipe-Scraper**](https://github.com/jadkins89/Recipe-Scraper) — JavaScript (100 stars)
- [**ryojp/recipe-scraper**](https://github.com/ryojp/recipe-scraper) — Go (80 stars)
- [**remaudcorentin-dev/python-allrecipes**](https://github.com/remaudcorentin-dev/python-allrecipes) — Python (65 stars)
- [**cookbrite/Recipe-to-Markdown**](https://github.com/cookbrite/Recipe-to-Markdown) — Python (30 stars)
- [**marcon29/CLI-dinner-finder-grocery-list**](https://github.com/marcon29/CLI-dinner-finder-grocery-list) — Ruby (5 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
