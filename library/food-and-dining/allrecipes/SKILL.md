---
name: pp-allrecipes
description: "Every Allrecipes recipe in your terminal — cached as data, with pantry-aware search, Bayesian-smoothed ranking, one-line grocery lists, and Cloudflare clearance. Trigger phrases: `search Allrecipes for X`, `find a recipe for brownies`, `scale this Allrecipes recipe`, `build a grocery list from these recipes`, `what can I cook with what I have`, `use allrecipes-pp-cli`, `run allrecipes`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["allrecipes-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-cli@latest","bins":["allrecipes-pp-cli"],"label":"Install via go install"}]}}'
---

# Allrecipes — Printing Press CLI

Search Allrecipes' 250k-recipe corpus from the command line, fetch a full recipe as parsed JSON-LD (ingredients with quantity+unit+name, instructions, nutrition, ratings, Made-It count), aggregate grocery lists from a meal plan, scale recipes, and export to clean markdown. Every recipe you fetch lands in a local SQLite store, which unlocks `pantry` (which recipes can I cook with what I have), `with-ingredient` (reverse index), `top-rated` with Bayesian smoothing (no more 1-review 5-star noise), and `cookbook` (export a category as a personal cookbook). Recipe detail pages are walled by Cloudflare; one-time `auth login --chrome` captures a clearance cookie from your browser — no Allrecipes account needed.

## When to Use This CLI

Reach for this CLI when an agent or user needs Allrecipes data as data, not as a webpage: searching for proven recipes, fetching ingredient lists in structured form, building grocery lists across multiple recipes, scaling for different serving counts, or filtering by what's in the pantry. The local SQLite cache makes iterative meal-planning workflows fast — every recipe fetched is queryable forever via `pantry`, `with-ingredient`, `top-rated`, and `dietary`. Skip this CLI when the user wants a multi-site comparison (different tool) or when they need authenticated features like saved recipes or meal plans (intentionally not supported).

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport over HTTP/3 for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**recipes** — Public Allrecipes recipe pages with Schema.org Recipe JSON-LD markup

- `allrecipes-pp-cli recipes get` — Fetch a recipe by ID + slug; returns parsed JSON-LD Recipe. Requires Cloudflare clearance cookie via `auth login...
- `allrecipes-pp-cli recipes search` — Search Allrecipes for recipes matching a query (no Cloudflare clearance required).


**Hand-written commands**

- `allrecipes-pp-cli recipe <url-or-id>` — Fetch and render a single recipe by URL or ID
- `allrecipes-pp-cli search <query>` — Search Allrecipes for recipes (live + cache)
- `allrecipes-pp-cli top-rated <query>` — Search and rank by Bayesian-smoothed rating (prior 4.0, default C=200)
- `allrecipes-pp-cli quick` — Recipes from cache that fit a strict time cap and are top-rated
- `allrecipes-pp-cli pantry` — Score cached recipes by overlap with a pantry file
- `allrecipes-pp-cli with-ingredient <ingredient>` — Reverse index: cached recipes that use a given ingredient
- `allrecipes-pp-cli dietary` — Filter cached recipes by gluten-free / vegan / low-carb (strict ingredient blocklist)
- `allrecipes-pp-cli cookbook` — Compile a top-rated category or cuisine into a markdown cookbook
- `allrecipes-pp-cli grocery-list <urls...>` — Aggregate ingredients across many recipes into a deduped shopping list (subtract --pantry-file if given)
- `allrecipes-pp-cli scale <url>` — Rescale a recipe to a target serving count
- `allrecipes-pp-cli nutrition <url>` — Show nutrition for a recipe (per serving and total)
- `allrecipes-pp-cli ingredients <url>` — Show parsed ingredients for a recipe
- `allrecipes-pp-cli instructions <url>` — Show numbered instructions for a recipe
- `allrecipes-pp-cli reviews <url>` — Show review summary for a recipe
- `allrecipes-pp-cli category <slug>` — Browse recipes in a category (e.g. dessert, weeknight)
- `allrecipes-pp-cli cuisine <slug>` — Browse recipes by cuisine
- `allrecipes-pp-cli ingredient <name>` — Browse recipes featuring a primary ingredient
- `allrecipes-pp-cli occasion <slug>` — Browse recipes by occasion (holiday, weeknight, party)
- `allrecipes-pp-cli article <url>` — Extract the body of an Allrecipes article page
- `allrecipes-pp-cli gallery <url>` — Extract recipe links from an Allrecipes round-up gallery
- `allrecipes-pp-cli cook <slug>` — Show a cook profile and their recipes
- `allrecipes-pp-cli export <url>` — Export a recipe as markdown
- `allrecipes-pp-cli sync` — Refresh cached recipes that are stale
- `allrecipes-pp-cli cache` — Inspect, list, or clear the local recipe cache
- `allrecipes-pp-cli doctor` — Diagnose connectivity, Cloudflare clearance state, and cache health


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `ALLRECIPES_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
allrecipes-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Top 5 brownies, narrowed to ranking fields only

```bash
allrecipes-pp-cli top-rated brownies --limit 5 --agent --select rank,title,url,rating,reviewCount,madeItCount
```

Bayesian smoothing kills 1-review 5-star outliers; --select trims the payload to fields agents actually use.

### Plan three weeknight dinners and one grocery list

```bash
allrecipes-pp-cli quick --max-minutes 30 --query chicken --limit 3 --agent | xargs allrecipes-pp-cli grocery-list --pantry-file ~/pantry.txt --agent
```

Compose two commands: quick weeknight selection feeds straight into pantry-aware grocery aggregation.

### Compile a personal Italian cookbook

```bash
allrecipes-pp-cli cookbook --category italian --top 20 --output ~/italian-cookbook.md
```

Pulls the top-20 by Bayesian rating and writes a markdown cookbook with TOC.

### Cook with what's in the pantry

```bash
allrecipes-pp-cli pantry --pantry-file ~/pantry.txt --max-missing 2 --agent --select title,url,have,missing
```

Local index ranks cached recipes by ingredient overlap; --max-missing 2 keeps recipes that need at most two extra ingredients.

### Strict gluten-free with field narrowing

```bash
allrecipes-pp-cli dietary --type gluten-free --top 20 --agent --select title,url,rating,recipeIngredient
```

Strict ingredient blocklist + JSON-LD tag filter; --select pulls only the title, url, rating, and ingredients for agent verification.

## Auth Setup

Browse and search work without any auth. Recipe detail pages need a Cloudflare clearance cookie — run `allrecipes-pp-cli auth login --chrome` once to capture one from your browser. This is bot-protection clearance, not an Allrecipes account login: no username, no password, no token tied to a user. The CLI does not support saved-recipes / meal-plan / profile features that require an actual Allrecipes account.

Run `allrecipes-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  allrecipes-pp-cli recipes get mock-value mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
allrecipes-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
allrecipes-pp-cli feedback --stdin < notes.txt
allrecipes-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.allrecipes-pp-cli/feedback.jsonl`. They are never POSTed unless `ALLRECIPES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ALLRECIPES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
allrecipes-pp-cli profile save briefing --json
allrecipes-pp-cli --profile briefing recipes get mock-value mock-value
allrecipes-pp-cli profile list --json
allrecipes-pp-cli profile show briefing
allrecipes-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `allrecipes-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-cli@latest
   ```
3. Verify: `allrecipes-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/cmd/allrecipes-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add allrecipes-pp-mcp -- allrecipes-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which allrecipes-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   allrecipes-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `allrecipes-pp-cli <command> --help`.
