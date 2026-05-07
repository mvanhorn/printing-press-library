---
name: pp-food52
description: "Search, browse, and read Food52 from your terminal — with offline FTS, pantry matching, recipe scaling, and the editorial signals other tools throw away. Trigger phrases: `find me a food52 recipe for X`, `scale this food52 recipe to N servings`, `what can I cook from food52 with what's in my pantry`, `use food52`, `run food52`."
version: "3.2.1"
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - food52-pp-cli
    install:
      - kind: go
        bins: [food52-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/cmd/food52-pp-cli
---

# Food52 — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `food52-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install food52 --cli-only
   ```
2. Verify: `food52-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/cmd/food52-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent needs Food52-quality recipes (community-curated, editor-tested) without rendering a browser or scraping HTML. The pantry match and offline FTS commands turn it into an edit-friendly cookbook the agent can keep on disk between sessions. Use the live `recipes search` for one-off lookups; sync + offline `search` for repeated queries against the same recipes.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Protocols: ssr_embedded_data (100% confidence), next_data_json (100% confidence), rest_json (100% confidence), schema_org_jsonld (100% confidence)
- Auth signals: none
- Generation hints: browser_http_transport
- Candidate command ideas: recipes search; recipes browse; recipes get; tags list; verticals list; articles browse; articles get
- Caveats: scope_note: Hotline (/hotline, /hotline/questions/<topic>) returns only siteSettings in pageProps and renders no question/answer data in the DOM. The community Q&A is effectively unreachable for unauthenticated read scrapers. Excluded from CLI scope.; scope_note: Shop endpoints exist on shop.food52.com and food52.myshopify.com/api/2025-01/graphql.json but require Shopify Storefront API token discovery; deferred from v1.; scope_note: buildId (Pq8fQj0nm7uTx90i5u0si at the time of browser-sniff) and _app.js bundle hash change on every Food52 deploy. The CLI's runtime discovery (buildId from __NEXT_DATA__.buildId, key from regex over the active _app-<hash>.js) handles rotation transparently.; scope_note: Typesense /collections endpoint requires the admin key (search-only key returns 403 there). The CLI does not list collections; it queries the known collection name 'recipes_production_food52_current' directly.

## Command Reference

**articles** — Browse and read Food52 stories (articles) from the food and life verticals

- `food52-pp-cli articles browse` — Browse the latest Food52 articles in a vertical (food, life)
- `food52-pp-cli articles get` — Get a Food52 article (story) by slug

**recipes** — Browse Food52 recipes by tag and fetch single recipe details (extracted from Next.js __NEXT_DATA__ embedded in SSR HTML)

- `food52-pp-cli recipes browse` — Browse Food52 recipes filtered by a tag (e.g. chicken, breakfast, vegetarian)
- `food52-pp-cli recipes get` — Get full structured details for a single Food52 recipe by slug


**Hand-written commands**

- `food52-pp-cli recipes search <query>` — Search Food52 recipes via Typesense (host + search-only key auto-discovered from the public JS bundle)
- `food52-pp-cli recipes top <tag>` — Show only Test-Kitchen-approved recipes for a tag, with an optional rating floor (--min-rating)
- `food52-pp-cli articles browse-sub <vertical> <subvertical>` — Browse a Food52 article subvertical (e.g. food baking, food drinks, life travel)
- `food52-pp-cli articles for-recipe <slug>` — Find synced articles whose relatedReading mentions the given recipe slug
- `food52-pp-cli tags list` — List Food52 recipe tags discovered from the homepage navigation (chicken, breakfast, vegetarian, dessert, pasta, ...)
- `food52-pp-cli sync recipes <tag> [<tag>...]` — Pull recipes for one or more tags into the local FTS-indexed store
- `food52-pp-cli sync articles <vertical> [<subvertical>]` — Pull article listings for a vertical (or subvertical) into the local store
- `food52-pp-cli search <query>` — Search the local store across recipes and articles (FTS5, offline)
- `food52-pp-cli pantry add <ingredient> [<ingredient>...]` — Add ingredients to your local pantry
- `food52-pp-cli pantry list` — Show your local pantry
- `food52-pp-cli pantry remove <ingredient>` — Remove an ingredient from your pantry
- `food52-pp-cli pantry match` — Find synced recipes whose ingredients match (or mostly match) your pantry
- `food52-pp-cli scale <slug-or-url>` — Scale a recipe's ingredients to a different number of servings (parses recipeYield from the recipe's JSON-LD)
- `food52-pp-cli print <slug-or-url>` — Print a clean cooking-friendly view of a recipe (ingredients + numbered steps, no nav, no images, ready to paste or...
- `food52-pp-cli open <slug-or-url>` — Print a Food52 recipe or article URL; pass --launch to actually open it in the default browser


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
food52-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find a TK-approved chicken recipe under 5 ingredients

```bash
food52-pp-cli recipes search chicken --tag 5-ingredients-or-fewer --json --select 'hits.document.title,hits.document.slug,hits.document.testKitchenApproved'
```

Typesense search filtered by tag, projecting just the fields an agent needs to pick a recipe.

### Get the full recipe in JSON for downstream meal planning

```bash
food52-pp-cli recipes get mom-s-japanese-curry-chicken-with-radish-and-cauliflower --json --select 'recipe.title,recipe.recipeDetails.ingredients,recipe.recipeDetails.instructions,recipe.averageRating'
```

Pulls structured ingredients + steps for piping into a meal planner or shopping list builder.

### Build an offline weeknight cookbook in one shot

```bash
food52-pp-cli sync recipes weeknight quick-and-easy 30-minutes-or-fewer && food52-pp-cli search 'weeknight' --json
```

Pulls three high-signal tags into the local store, then queries via FTS5 — works on a plane.

### What can I make right now?

```bash
food52-pp-cli pantry match --min-coverage 0.6 --json --select 'matches.title,matches.coverage,matches.missing_ingredients'
```

Joins your local pantry against every synced recipe, returning the ones you can mostly make.

## Auth Setup

No authentication required.

Run `food52-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  food52-pp-cli articles get best-mothers-day-gift-ideas --agent --select id,name,status
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
food52-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
food52-pp-cli feedback --stdin < notes.txt
food52-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.food52-pp-cli/feedback.jsonl`. They are never POSTed unless `FOOD52_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FOOD52_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
food52-pp-cli profile save briefing --json
food52-pp-cli --profile briefing articles get best-mothers-day-gift-ideas
food52-pp-cli profile list --json
food52-pp-cli profile show briefing
food52-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `food52-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/cmd/food52-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add food52-pp-mcp -- food52-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which food52-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   food52-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `food52-pp-cli <command> --help`.
