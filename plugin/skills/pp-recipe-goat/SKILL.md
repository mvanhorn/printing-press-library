---
name: pp-recipe-goat
description: "Find the best version of any recipe across curated cuisine-authority sites — then plan, shop, and cook with a local kitchen companion. Trigger phrases: `find me the best recipe for`, `what can I make with`, `substitute for`, `plan meals for the week`, `shopping list from my meal plan`, `use recipe-goat`, `run recipe-goat`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["recipe-goat-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat-pp-cli/cmd/recipe-goat-pp-cli@latest","bins":["recipe-goat-pp-cli"],"label":"Install via go install"}]}}'
---

# Recipe Goat — Printing Press CLI

Recipe GOAT aggregates a curated set of independent, cuisine-authoritative recipe sites — Nagi (RecipeTin Eats), Swasthi (Indian Healthy Recipes), Elaine (China Sichuan Food), The Woks of Life, Just One Cookbook, Sally's Baking Addiction, King Arthur Baking, Budget Bytes, BBC Food, and more — ranks results by merged trust + rating + review-count signals, and builds a local SQLite cookbook that powers pantry match, cook log, meal plans, and aisle-grouped shopping lists. When users paste URLs from bot-detection-gated sites (allrecipes, food52, etc.), archive.org's Wayback Machine is used to recover the content. Unique commands like `goat` (best-version ranker), `sub` (cross-site substitution aggregation), `tonight` (decision-fatigue killer), and `cookbook match --have` (pantry match) solve problems no single recipe site can.

## When to Use This CLI

Reach for Recipe GOAT when the user needs the best version of a dish without site-hopping, wants to query their own cookbook by pantry contents, needs unit-reconciled shopping lists for a meal plan, or wants substitution guidance grounded in authoritative baker knowledge. Especially useful for home cooks of any skill level — the CLI's ranking and review-digest features surface the collective wisdom that makes a 4-star recipe into a 5-star one.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-site intelligence

- **`goat`** — Query any dish across curated recipe sites and rank results by normalized rating × review count × site trust × recency.

  _Use this when you need the single best version of a dish — the agent gets structured results with provenance and trust signals instead of guessing from a web search._

  ```bash
  recipe-goat-pp-cli goat "chicken tikka masala" --limit 5 --json
  ```
- **`sub`** — Aggregate ingredient substitutions from King Arthur Baking and other trusted baking-science sources. Ranked by source trust with ratios and context.

  _When a recipe needs a sub, agents can pick the best one given the cooking context (baking vs marinade) instead of suggesting the first hit on Google._

  ```bash
  recipe-goat-pp-cli sub buttermilk --context baking
  ```
- **`recipe reviews`** — Surface the top modifications cooks actually made to a recipe ("added an egg: 22 cooks; baked 5 min less: 17; honey instead of sugar: 14").

  _Agents give the user the collective wisdom of reviewers instead of just star ratings._

  ```bash
  recipe-goat-pp-cli recipe reviews <id> --limit 10
  ```
- **`recipe get --nutrition`** — When a site omits nutrition, parse ingredients, match USDA FoodData Central IDs, compute per-serving macros locally.

  _Agents can answer 'is this recipe high-protein?' reliably even when the source doesn't publish macros._

  ```bash
  recipe-goat-pp-cli recipe get https://www.budgetbytes.com/creamy-mushroom-pasta/ --nutrition
  ```
- **`recipe get (auto)`** — Flag out-of-season ingredients inline ("⚠ asparagus is out of season in November — peak April–June") and suggest in-season swaps.

  _Agents surface cost + quality signals the user wouldn't otherwise see._

  ```bash
  recipe-goat-pp-cli recipe get <url>  # seasonal flag appears automatically
  ```

### Local state that compounds

- **`cookbook match`** — Find recipes in the local cookbook that you can make right now with listed ingredients, or with ≤N missing ingredients.

  _When the user says 'what can I make with what's in my fridge,' the agent gets ranked candidates with missing-ingredient counts instead of guessing._

  ```bash
  recipe-goat-pp-cli cookbook match --have "chicken,rice,broccoli" --missing-max 2
  ```
- **`tonight`** — Pick dinner in 2 seconds: filter cookbook by time budget, recency from cook log, and dietary/kid-friendly flags.

  _Ends the 20-minute 'what are we having' debate with three data-backed candidates._

  ```bash
  recipe-goat-pp-cli tonight --max-time 30m --no-repeat-within 7d --kid-friendly
  ```
- **`search --kid-friendly`** — Filter recipes against an editable ingredient-exclusion list (capers, anchovies, excess heat, raw fish, etc.). Personalizable per-child.

  _Parents get results actually edible by their kids, not marketing's idea of 'kid-friendly'._

  ```bash
  recipe-goat-pp-cli search "chicken dinner" --kid-friendly --limit 10
  ```
- **`meal-plan shopping-list`** — Aggregate ingredients across planned meals, reconcile units (2 cup + 1 cup milk → 3 cup), group by grocery aisle.

  _The agent hands the user a complete shopping list ready for grocery day, aisle-grouped._

  ```bash
  recipe-goat-pp-cli meal-plan shopping-list --week --export md
  ```
- **`recipe cost`** — Rough cost per serving using Budget Bytes line-item data plus USDA retail averages as fallback. Always shows an honesty band (±30%).

  _Agents can triage recipes by rough cost without pretending to precision grocery data doesn't provide._

  ```bash
  recipe-goat-pp-cli recipe cost <id>  # output: '$6–$9 for 4 servings (±30%)'
  ```

## Command Reference

**foods** — USDA FoodData Central — ingredient nutrition lookups

- `recipe-goat-pp-cli foods get` — Get a specific food by FDC ID
- `recipe-goat-pp-cli foods list` — List foods paginated
- `recipe-goat-pp-cli foods search` — Search USDA FoodData Central for foods matching a query


## Recipes


### Find the best version of a dish

```bash
recipe-goat-pp-cli goat "carbonara" --limit 5
```

Query the curated corpus, return the top 5 ranked by trust + rating + reviews with source attribution.

### Save and tag for weeknight

```bash
recipe-goat-pp-cli save <url> --tags weeknight,pasta
```

Persist the recipe locally with tags you'll filter on later.

### Plan 5 dinners with kid-safe ingredients

```bash
recipe-goat-pp-cli tonight --max-time 30m --kid-friendly --limit 5
```

Three candidates from your cookbook that match time and ingredient constraints.

### Aggregate shopping list for the week

```bash
recipe-goat-pp-cli meal-plan shopping-list --week --aisle
```

Shopping list grouped by grocery aisle with unit reconciliation.

### Substitute an out-of-stock ingredient

```bash
recipe-goat-pp-cli sub eggs --context baking --vegan
```

Context-aware substitutions ranked by source trust.

## Auth Setup (optional)

**All core commands work without any setup.** The API key below is only needed to unlock `recipe get --nutrition` backfill.

### When to recommend setting the key

Only when the user wants accurate per-serving macros for recipes whose source site omits nutrition data (≈30% of recipes across the supported sites). For all other workflows — best-version ranking, pantry match, meal plans, substitutions, cookbook search — no key is needed.

### How to set it up

1. User signs up at https://fdc.nal.usda.gov/api-key-signup (free, 3,500 req/hour, no payment).
2. Export: `export USDA_FDC_API_KEY=<their-key>` — or persist with `recipe-goat-pp-cli auth set-token <their-key>`.
3. Verify: `recipe-goat-pp-cli doctor` — Auth should show `INFO Auth: optional — not configured` before, `OK Auth: configured` after.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select id,name` returns a subset of fields
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag

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

1. **Empty, `help`, or `--help`** → show `recipe-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat-pp-cli/cmd/recipe-goat-pp-cli@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat-pp-cli/cmd/recipe-goat-pp-cli@main
   ```
3. Verify: `recipe-goat-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat-pp-cli/cmd/recipe-goat-pp-mcp@latest
   
   # If `@latest` installs a stale build (Go module proxy cache lag), install from main:
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat-pp-cli/cmd/recipe-goat-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add recipe-goat-pp-mcp -- recipe-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which recipe-goat-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   recipe-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `recipe-goat-pp-cli <command> --help`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
recipe-goat-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
recipe-goat-pp-cli --profile <name> <command>

# List / inspect / remove
recipe-goat-pp-cli profile list
recipe-goat-pp-cli profile show <name>
recipe-goat-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
recipe-goat-pp-cli <command> --deliver file:/path/to/out.json
recipe-goat-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
recipe-goat-pp-cli feedback "what surprised you or tripped you up"
recipe-goat-pp-cli feedback list         # show local entries
recipe-goat-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.recipe-goat-pp-cli/feedback.jsonl` as JSON lines. When `RECIPE_GOAT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `RECIPE_GOAT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

