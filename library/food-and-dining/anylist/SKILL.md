---
name: pp-anylist
description: "Every AnyList feature in your terminal — plus offline search, store routing, and cron-safe automations no mobile... Trigger phrases: `add to my grocery list`, `what recipes can I make with`, `build my shopping list from meal plan`, `check off items on anylist`, `use anylist`, `run anylist`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - anylist-pp-cli
---

# AnyList — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `anylist-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install anylist --cli-only
   ```
2. Verify: `anylist-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/cmd/anylist-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use anylist-pp-cli when you need to automate grocery and meal planning workflows from the shell or in agent pipelines. It excels at cron-based shopping list generation from meal plans, offline recipe search by ingredient or metadata, and store-optimized shopping output. It is the right choice when you need JSON output from AnyList operations for downstream processing with jq, or when you want to build Home Assistant / n8n automations beyond what the basic hacs-anylist integration supports.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`recipes search --ingredient`** — Find every recipe that uses a given ingredient instantly — no scrolling, no guessing.

  _Use this when an agent needs to suggest meals from available pantry items without making multiple API calls._

  ```bash
  anylist-pp-cli recipes search --query chicken --ingredient --agent
  ```
- **`recipes filter`** — Filter your entire recipe library by prep time, rating, serving count, and collection simultaneously.

  _Use this when an agent needs to find recipes that fit a time or quality constraint for meal planning._

  ```bash
  anylist-pp-cli recipes filter --max-prep 30 --min-rating 4 --collection Weeknight --agent
  ```
- **`lists by-store`** — Split a shopping list into per-store groups sorted by store aisle order — ready for multi-store shopping trips.

  _Use this when an agent needs to generate a structured shopping route across multiple stores._

  ```bash
  anylist-pp-cli lists by-store --name Groceries --agent
  ```
- **`recipes missing`** — See exactly which ingredients you still need to buy before adding a recipe — skip what's already on your list.

  _Use this before adding a recipe to a list to avoid redundant items and show users only the net-new ingredients they need._

  ```bash
  anylist-pp-cli recipes missing --recipe "Pasta Bake" --list Groceries --agent
  ```
- **`meal summary`** — Render a Mon–Sun meal plan grid with Breakfast/Lunch/Dinner labels — pasteable into messages or scripts.

  _Use this when an agent needs to present the week's meal plan in a human-readable format for sharing or review._

  ```bash
  anylist-pp-cli meal summary --week | pbcopy
  ```
- **`items search`** — Find any item across all your shopping lists at once — shows which list it's on and whether it's checked.

  _Use this when an agent needs to locate an item without knowing which list it was added to._

  ```bash
  anylist-pp-cli items search --query "almond milk" --agent
  ```

### Agent-native plumbing

- **`meal add-to-list`** — Automatically build this week's shopping list from your meal plan — idempotent and safe to run on a schedule.

  _Use this in a cron job or agent workflow to pre-populate the shopping list before a weekly grocery trip._

  ```bash
  anylist-pp-cli meal add-to-list --week --list Groceries --dry-run
  ```
- **`recipes add-to-list`** — Add a recipe's ingredients to your list while avoiding duplicate unchecked items already on the target list.

  _Use this when adding recipes to a list without creating duplicate unchecked line items._

  ```bash
  anylist-pp-cli recipes add-to-list --recipe "Pasta Bake" --list Groceries --scale 4 --merge
  ```
- **`lists reset`** — Clear all checked items from a list in one command — idempotent and safe for cron to run after every shopping trip.

  _Use this in a post-trip automation to reset the list for the next week without manually unchecking each item._

  ```bash
  anylist-pp-cli lists reset --name Groceries --keep-unchecked
  ```
- **`sync status`** — Report how fresh your local cache is per entity type, with exit code 1 if any data is stale.

  _Use this as a cron preflight check to ensure agent operations run against up-to-date local data._

  ```bash
  anylist-pp-cli sync status --stale-after 24h || anylist-pp-cli sync
  ```

## Command Reference

**categories** — View item categories

- `anylist-pp-cli categories` — List all item categories

**collections** — Manage recipe collections

- `anylist-pp-cli collections add` — Add a recipe to a collection
- `anylist-pp-cli collections create` — Create a new recipe collection
- `anylist-pp-cli collections delete` — Delete a recipe collection
- `anylist-pp-cli collections list` — List all recipe collections
- `anylist-pp-cli collections remove` — Remove a recipe from a collection

**favorites** — View favorite items

- `anylist-pp-cli favorites` — List favorite items across all lists

**folders** — Organize shopping lists into folders

- `anylist-pp-cli folders create` — Create a new list folder
- `anylist-pp-cli folders delete` — Delete a list folder
- `anylist-pp-cli folders list` — List all list folders

**items** — Manage items within a shopping list

- `anylist-pp-cli items add` — Add an item to a shopping list
- `anylist-pp-cli items check` — Mark one or more items as checked (bought)
- `anylist-pp-cli items list` — List items in a shopping list
- `anylist-pp-cli items recent` — Show recently added items across all lists
- `anylist-pp-cli items remove` — Remove an item from a shopping list
- `anylist-pp-cli items search` — Search for items by name across all shopping lists
- `anylist-pp-cli items uncheck` — Mark one or more items as unchecked
- `anylist-pp-cli items update` — Update an existing item's quantity or notes

**lists** — Manage shopping lists

- `anylist-pp-cli lists by-store` — Display a shopping list grouped by store with aisle ordering
- `anylist-pp-cli lists create` — Create a new shopping list
- `anylist-pp-cli lists delete` — Delete a shopping list
- `anylist-pp-cli lists list` — List all shopping lists
- `anylist-pp-cli lists reset` — Clear all checked items from a list to prepare for the next shopping trip
- `anylist-pp-cli lists settings` — View or update settings for a shopping list

**meal** — Manage the meal planning calendar

- `anylist-pp-cli meal add` — Add an event to the meal planning calendar
- `anylist-pp-cli meal add-to-list` — Add all recipe ingredients from the meal plan to a shopping list
- `anylist-pp-cli meal delete` — Delete a meal plan event
- `anylist-pp-cli meal labels` — List meal plan labels (Breakfast, Lunch, Dinner, Snack, etc.)
- `anylist-pp-cli meal show` — Show meal plan events for a date range (defaults to current week)
- `anylist-pp-cli meal summary` — Display the meal plan as a Mon–Sun grid with Breakfast/Lunch/Dinner labels

**recipes** — Manage recipes — import, organize, and add to shopping lists

- `anylist-pp-cli recipes add-to-list` — Add recipe ingredients to a shopping list, avoiding duplicate unchecked items
- `anylist-pp-cli recipes batch-add` — Add ingredients from multiple recipes to a list at once
- `anylist-pp-cli recipes create` — Create a new recipe
- `anylist-pp-cli recipes delete` — Delete a recipe
- `anylist-pp-cli recipes filter` — Filter recipes by prep time, rating, servings, and collection
- `anylist-pp-cli recipes import` — Import a recipe from a URL
- `anylist-pp-cli recipes list` — List all recipes
- `anylist-pp-cli recipes missing` — Show which recipe ingredients are not already on a shopping list
- `anylist-pp-cli recipes scale` — Scale a recipe's ingredient quantities to a target serving count
- `anylist-pp-cli recipes search` — Search recipes by name or ingredient (uses local SQLite cache)
- `anylist-pp-cli recipes show` — Show full recipe details including ingredients and preparation steps

**starters** — Manage starter list items (template items for new lists)

- `anylist-pp-cli starters` — List starter list items

**stores** — View and manage stores and store filters

- `anylist-pp-cli stores` — List all stores and store filters


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
anylist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sunday meal-prep: build grocery list from this week's plan

```bash
anylist-pp-cli meal add-to-list --week --list Groceries --dry-run && anylist-pp-cli meal add-to-list --week --list Groceries
```

Preview then execute: adds this week's meal plan ingredients to the Groceries list idempotently.

### Find quick weeknight recipes with available chicken

```bash
anylist-pp-cli recipes search --query chicken --ingredient --agent | anylist-pp-cli recipes filter --max-prep 30 --min-rating 4
```

Search by ingredient then filter by metadata — pure offline SQLite, no API calls.

### Check what you still need before adding a recipe

```bash
anylist-pp-cli recipes missing --recipe "Thai Green Curry" --list Groceries --agent
```

Shows only the ingredients not already on your shopping list — prevents duplicates.

### Get per-store shopping route as structured JSON

```bash
anylist-pp-cli lists by-store --name Groceries --agent --select name,storeName,category
```

Groups items by store with sort_index ordering; --select narrows the payload for agent context.

### Post-trip cleanup + next-week reset via cron

```bash
anylist-pp-cli lists reset --name Groceries --keep-unchecked && anylist-pp-cli sync status --stale-after 12h || anylist-pp-cli sync
```

Remove checked items, keep unchecked, then ensure cache is fresh — safe to run in a cron job.

## Auth Setup

AnyList uses email + password authentication returning a short-lived access_token and a refresh_token. The CLI stores these in ~/.config/anylist-pp-cli/config.toml and transparently refreshes on 401 responses. All requests require two additional headers: X-AnyLeaf-API-Version: 3 and a stable X-AnyLeaf-Client-Identifier (a 32-char hex UUID generated once per device and reused).

Run `anylist-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  anylist-pp-cli categories --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
anylist-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
anylist-pp-cli feedback --stdin < notes.txt
anylist-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.anylist-pp-cli/feedback.jsonl`. They are never POSTed unless `ANYLIST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ANYLIST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
anylist-pp-cli profile save briefing --json
anylist-pp-cli --profile briefing categories
anylist-pp-cli profile list --json
anylist-pp-cli profile show briefing
anylist-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `anylist-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add anylist-pp-mcp -- anylist-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which anylist-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   anylist-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `anylist-pp-cli <command> --help`.
