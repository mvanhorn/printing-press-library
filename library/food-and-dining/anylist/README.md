# AnyList CLI

**Every AnyList feature in your terminal — plus offline search, store routing, and cron-safe automations no mobile app can match.**

The CLI syncs your shopping lists, recipes, and meal plan to a local SQLite database, then lets you query and automate everything from the shell. Search recipes by ingredient, split shopping lists by store, and build this week's grocery list from your meal plan — all scriptable, JSON-outputting, and safe to run on a schedule.

Created by [@jeeves](https://github.com/jeeves) (Jeeves).

## Install

The recommended path installs both the `anylist-pp-cli` binary and the `pp-anylist` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install anylist
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install anylist --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install anylist --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install anylist --agent claude-code
npx -y @mvanhorn/printing-press-library install anylist --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/cmd/anylist-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/anylist-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install anylist --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-anylist --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-anylist --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install anylist --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/anylist-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ANYLIST_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "anylist": {
      "command": "anylist-pp-mcp",
      "env": {
        "ANYLIST_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

AnyList uses email + password authentication returning a short-lived access_token and a refresh_token. The CLI stores these in ~/.config/anylist-pp-cli/config.toml and transparently refreshes on 401 responses. All requests require two additional headers: X-AnyLeaf-API-Version: 3 and a stable X-AnyLeaf-Client-Identifier (a 32-char hex UUID generated once per device and reused).

## Quick Start

```bash
# Authenticate with your AnyList email and password
anylist-pp-cli auth login

# Pull all your lists, recipes, and meal plan into the local SQLite cache
anylist-pp-cli sync

# See all your shopping lists
anylist-pp-cli lists list

# Pipe unchecked items to jq for scripting
anylist-pp-cli items list --list Groceries --unchecked --json | jq '.[].name'

# View this week's meal plan as a Mon-Sun grid
anylist-pp-cli meal summary --week

# Preview adding this week's recipe ingredients to your grocery list
anylist-pp-cli meal add-to-list --week --list Groceries --dry-run

```

## Unique Features

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

## Usage

Run `anylist-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

View item categories

- **`anylist-pp-cli categories`** - List all item categories

### collections

Manage recipe collections

- **`anylist-pp-cli collections add`** - Add a recipe to a collection
- **`anylist-pp-cli collections create`** - Create a new recipe collection
- **`anylist-pp-cli collections delete`** - Delete a recipe collection
- **`anylist-pp-cli collections list`** - List all recipe collections
- **`anylist-pp-cli collections remove`** - Remove a recipe from a collection

### favorites

View favorite items

- **`anylist-pp-cli favorites`** - List favorite items across all lists

### folders

Organize shopping lists into folders

- **`anylist-pp-cli folders create`** - Create a new list folder
- **`anylist-pp-cli folders delete`** - Delete a list folder
- **`anylist-pp-cli folders list`** - List all list folders

### items

Manage items within a shopping list

- **`anylist-pp-cli items add`** - Add an item to a shopping list
- **`anylist-pp-cli items check`** - Mark one or more items as checked (bought)
- **`anylist-pp-cli items list`** - List items in a shopping list
- **`anylist-pp-cli items recent`** - Show recently added items across all lists
- **`anylist-pp-cli items remove`** - Remove an item from a shopping list
- **`anylist-pp-cli items search`** - Search for items by name across all shopping lists
- **`anylist-pp-cli items uncheck`** - Mark one or more items as unchecked
- **`anylist-pp-cli items update`** - Update an existing item's quantity or notes

### lists

Manage shopping lists

- **`anylist-pp-cli lists by-store`** - Display a shopping list grouped by store with aisle ordering
- **`anylist-pp-cli lists create`** - Create a new shopping list
- **`anylist-pp-cli lists delete`** - Delete a shopping list
- **`anylist-pp-cli lists list`** - List all shopping lists
- **`anylist-pp-cli lists reset`** - Clear all checked items from a list to prepare for the next shopping trip
- **`anylist-pp-cli lists settings`** - View or update settings for a shopping list

### meal

Manage the meal planning calendar

- **`anylist-pp-cli meal add`** - Add an event to the meal planning calendar
- **`anylist-pp-cli meal add-to-list`** - Add all recipe ingredients from the meal plan to a shopping list
- **`anylist-pp-cli meal delete`** - Delete a meal plan event
- **`anylist-pp-cli meal labels`** - List meal plan labels (Breakfast, Lunch, Dinner, Snack, etc.)
- **`anylist-pp-cli meal show`** - Show meal plan events for a date range (defaults to current week)
- **`anylist-pp-cli meal summary`** - Display the meal plan as a Mon–Sun grid with Breakfast/Lunch/Dinner labels

### recipes

Manage recipes — import, organize, and add to shopping lists

- **`anylist-pp-cli recipes add-to-list`** - Add recipe ingredients to a shopping list, avoiding duplicate unchecked items
- **`anylist-pp-cli recipes batch-add`** - Add ingredients from multiple recipes to a list at once
- **`anylist-pp-cli recipes create`** - Create a new recipe
- **`anylist-pp-cli recipes delete`** - Delete a recipe
- **`anylist-pp-cli recipes filter`** - Filter recipes by prep time, rating, servings, and collection
- **`anylist-pp-cli recipes import`** - Import a recipe from a URL
- **`anylist-pp-cli recipes list`** - List all recipes
- **`anylist-pp-cli recipes missing`** - Show which recipe ingredients are not already on a shopping list
- **`anylist-pp-cli recipes scale`** - Scale a recipe's ingredient quantities to a target serving count
- **`anylist-pp-cli recipes search`** - Search recipes by name or ingredient (uses local SQLite cache)
- **`anylist-pp-cli recipes show`** - Show full recipe details including ingredients and preparation steps

### starters

Manage starter list items (template items for new lists)

- **`anylist-pp-cli starters`** - List starter list items

### stores

View and manage stores and store filters

- **`anylist-pp-cli stores`** - List all stores and store filters

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
anylist-pp-cli categories

# JSON for scripting and agents
anylist-pp-cli categories --json

# Filter to specific fields
anylist-pp-cli categories --json --select id,name,status

# Dry run — show the request without sending
anylist-pp-cli categories --dry-run

# Agent mode — JSON + compact + no prompts in one flag
anylist-pp-cli categories --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
anylist-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/anylist-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ANYLIST_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Cookbook

Common patterns and recipes for everyday use.

**Build this week's grocery list from your meal plan:**

```bash
anylist-pp-cli meal add-to-list --week --list Groceries --dry-run   # preview
anylist-pp-cli meal add-to-list --week --list Groceries             # apply
```

**Find recipes by ingredient and scale servings:**

```bash
anylist-pp-cli recipes search --ingredient chicken --json | jq '.[0].name'
anylist-pp-cli recipes scale --recipe "Chicken Tikka" --servings 8
```

**Sync, filter, and pipe items to another tool:**

```bash
anylist-pp-cli sync && anylist-pp-cli items list --list Groceries --unchecked --json \
  | jq -r '.[].name' | sort
```

**Check authentication and connectivity:**

```bash
anylist-pp-cli doctor
anylist-pp-cli doctor --json | jq .credentials
```

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `anylist-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ANYLIST_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every request** — Run `anylist-pp-cli auth refresh` to force a token refresh, or `auth login` to re-authenticate
- **Commands return stale data (old item names, missing lists)** — Run `anylist-pp-cli sync` to pull the latest state from the server
- **recipes search --ingredient returns no results** — Run `anylist-pp-cli sync` first — ingredient search queries the local cache which must be populated
- **X-AnyLeaf-Client-Identifier errors** — The client identifier in ~/.config/anylist-pp-cli/config.toml must be a stable 32-char hex string; delete the config and re-run `auth login` to regenerate

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bobby060/anylist-mcp**](https://github.com/bobby060/anylist-mcp) — TypeScript
- [**davidashman/anylist-mcp**](https://github.com/davidashman/anylist-mcp) — TypeScript
- [**kevdliu/hacs-anylist**](https://github.com/kevdliu/hacs-anylist) — Python
- [**codetheweb/anylist**](https://github.com/codetheweb/anylist) — JavaScript
- [**phildenhoff/anylist_rs**](https://github.com/phildenhoff/anylist_rs) — Rust
- [**bcspragu/anylist**](https://github.com/bcspragu/anylist) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
