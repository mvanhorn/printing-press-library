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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

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

# Use an isolated cache for tests or automation; this does not change credentials
anylist-pp-cli sync --db /tmp/anylist-test.db --full

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
- **`items lookup`** — Resolve a UPC/EAN barcode through AnyList's product catalog and return the canonical name, structured package size, UPC, prices, and thumbnail URL.

  _Use this before adding a packaged product. Passing the returned barcode to `items add` lets AnyList receive the identifier first and enrich the item when its catalog has a match._

  ```bash
  anylist-pp-cli items lookup --barcode 049000028904 --json
  anylist-pp-cli items add --list Groceries --item "Coca-Cola" --barcode 049000028904 --package-size "12 count, 12 fl oz" --apply
  ```

- **`items update`** — Rename an item or update its quantity, notes, verified barcode, package size, category match, store assignments, or price. Preview is the default and `--apply` is required for every live write; fresh list metadata resolves names, browser-confirmed protobuf handlers perform the write, the live item is verified by ID, and the cache is synced only after verification. Use `--clear-category`, `--store`, `--remove-store`, `--price`, or `--clear-price` for metadata CRUD. Aisle/category-assignment subrecords beyond the captured path and recipe photos remain unsupported. Stdin accepts `category`, `clear_category`, `store`, `remove_store`, `price`, `price_store`, `price_details`, `clear_price`, and `apply` in addition to the scalar fields.

  ```bash
anylist-pp-cli items update --list Groceries --item "Milk" --package-size "12 oz carton" --apply --json
printf '%s\n' '{"list":"Groceries","item":"Milk","package_size":"12 oz carton","apply":true}' | anylist-pp-cli items update --stdin --json
anylist-pp-cli items update --list Groceries --item "Milk" --name "Whole Milk" --apply --json
printf '%s\n' '{"list":"Groceries","item":"Whole Milk","name":"Milk","apply":true}' | anylist-pp-cli items update --stdin --json
anylist-pp-cli items update --list Groceries --item "Milk" --category Produce --apply --json
anylist-pp-cli items update --list Groceries --item "Milk" --store "Paris Walmart" --apply --json
anylist-pp-cli items update --list Groceries --item "Milk" --remove-store "Paris Walmart" --apply --json
anylist-pp-cli items update --list Groceries --item "Milk" --price 3.49 --apply --json
anylist-pp-cli items update --list Groceries --item "Milk" --clear-price --apply --json
  ```

`--clear-price` without `--price-store` clears every store-specific price returned for the live item and verifies that no positive price remains. Use `--price-store <id>` to clear only one store's price while preserving prices for other stores.

- **`items photo attach`** — Upload and attach a photo to a shopping-list item. Local dry-run validates the file without network access; `--apply` is required for the live upload and typed attachment. The command fresh-reads the item, verifies the returned 32-hex photo ID, and persists it in the local cache before reporting success (`cache_updated:true`).

  ```bash
  anylist-pp-cli items photo attach --list Groceries --item "Milk" --file milk.jpg --dry-run --json
  anylist-pp-cli items photo attach --list Groceries --item "Milk" --file milk.jpg --apply --json
  ```

### Agent-native plumbing

- **`meal add-to-list`** — Preview meal-plan ingredients only; live bulk writes are disabled. Use the raw ingredient packet and explicit item operations.

  _Use this in a cron job or agent workflow to pre-populate the shopping list before a weekly grocery trip._

  ```bash
  anylist-pp-cli meal add-to-list --week --list Groceries --dry-run
  ```
- **`recipes ingredients`** — Return raw recipe ingredients plus fresh target-list candidate facts and stored-price data for an AI decision layer. It is read-only, preserves source wording, and does not choose or write an item.

  ```bash
  anylist-pp-cli recipes ingredients --name "Pasta Bake" --list Groceries --scale 4 --agent
  ```
- **`recipes add-to-list`** — Deprecated bulk path; it fails closed. Use `recipes ingredients`, then explicit `items add`/`items update --item-id` operations selected by the AI.
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

Use the global `--db <path>` option to select a per-invocation SQLite cache without changing the saved config. AnyList returns one complete user-data envelope, so `sync --resources <hint>` is accepted for automation compatibility but still refreshes the complete cache; `--full` makes that behavior explicit.

## AnyList Meal Planning Workflow

The companion meal-planning skill is the coordinator; this press supplies recipe facts, raw ingredient rows, current and historical item metadata, barcode lookup, and explicit writes.

### Preference lifecycle

The first run conducts a complete interview and asks what to save as defaults: purpose/cadence; meals, dates, servings and leftovers; budget and nutrition; restrictions, allergies and dislikes; protein and cuisine preferences; difficulty, time, equipment and freezer use; pantry/fridge inventory; target list, store, ZIP and aisle/category preferences; review/debug mode; and calendar-write preference. Later runs print the saved profile and ask whether to use it unchanged, customize this run only, or edit and save the defaults. One-run overrides are not saved unless requested. Preferences are non-secret and stored at `~/.config/anylist-meal-planning/preferences.json` with mode `0600`.

### AI-led curation and approval

The AI reads recipes, collections, ingredients, nutrition, timing and available metadata, then proposes economical, varied recipes with a raw combined ingredient list. No shopping-list or calendar write occurs before recipe approval. After approval, the AI asks what is already in the pantry/fridge and uses `recipes ingredients` to collect facts without semantic merging. Each ingredient resolves to `reuse`, `recycle`, `create`, `skip` (already available), or `ask` (ambiguous). Generic “cheddar cheese” remains unresolved when the recipe does not say block, sliced or shredded.

### Review table before shopping-list writes

When review/debug mode is enabled, present this table before mutation:

| Ingredient needed | Form/package | Proposed AnyList item | Source | Metadata comparison | UPC action | Stored price | Proposed action | Confidence/notes |
|---|---|---|---|---|---|---:|---|---|
| ½ cup cheddar | unresolved | — | — | block/sliced/shredded differ | ask | — | ask | recipe lacks form |
| 1 gallon milk | gallon | Milk - Gallon | historical | package and barcode agree | reuse | $3.48 | recycle | best prior metadata |
| 8 oz shredded cheese | shredded, 8 oz | Shredded Sharp Cheddar Cheese | current | strongest metadata | keep | $2.97 | reuse | stable ID selected |
| jarred garlic | jar | — | new | no suitable candidate | attempt lookup | — | create | UPC pending |

The table distinguishes current exact items, historical items, metadata mismatches, new items and unresolved ambiguity. With review/debug disabled, the same AI resolution happens without displaying the table.

### Pricing, UPCs and safety

Pricing currently comes only from stored AnyList item data. Report a known-price subtotal and missing-price count, and always say: **Prices may be outdated, incomplete, or missing; this is not a live Walmart price or availability quote.** A future Walmart press will provide reliable ZIP-aware store and aisle pricing.

For a new item, the AI may propose a UPC only when the product form and package are specific. Run `items lookup --barcode` or barcode-first creation, then read the item back and compare name, brand, package size, photo, price, store and category. Never silently attach an uncertain UPC. All mutations are preview-first, require `--apply`, use selected stable IDs, protect checked items, and require fresh read-after-write verification. Calendar create/update/delete has also passed a disposable live round-trip and follows the same verification gate.

## Usage

Run `anylist-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

View item categories, and create, rename, delete, or reorder custom categories

- **`anylist-pp-cli categories`** - List all item categories (read-only)
- **`anylist-pp-cli categories create`** - Preview or create a custom category in a shopping list with explicit `--apply`; the list and category group are resolved by a fresh live read, the new category gets a non-conflicting stable ID and a valid sort index, and the create is verified by stable ID before success
- **`anylist-pp-cli categories rename`** - Preview or rename a custom category by stable ID or exact name with explicit `--apply`; ambiguous or missing names fail closed, the stable identifier/group/list/sort index are preserved, and the new name is verified by stable ID before success
- **`anylist-pp-cli categories delete`** - Preview or delete a custom category by stable ID or exact name with explicit `--apply`; ambiguous or missing names fail closed, system categories cannot be deleted, and the deletion is verified by confirming the stable ID is absent from a fresh read before success
- **`anylist-pp-cli categories reorder`** - Preview or reorder a category group's categories with explicit `--apply`; `--order` must name every category in the group exactly once by stable ID or exact name (duplicates, unknown entries, and silently appended or dropped categories fail closed), and the exact stable-ID order is verified from a fresh read before success

Custom category writes use the verified multipart `/data/shopping-lists/update-v2` wire contract; the non-persistent v1 route and the old non-persistent `remove-category` handler are never used for them. Category-group create, rename, and delete (group CRUD) remain unsupported.

### collections

Manage recipe collections and preview collection writes. Create/add/remove/delete previews are available, but live writes currently fail closed because a disposable AnyList create acknowledged HTTP success without persisting on fresh read-back. Do not use `--apply` until a future live probe proves the exact collection contract.

- **`anylist-pp-cli collections add`** - Preview an add; live mutation is fail-closed pending persistence proof
- **`anylist-pp-cli collections create`** - Preview a create; live mutation is fail-closed pending persistence proof
- **`anylist-pp-cli collections delete`** - Preview a delete; live mutation is fail-closed pending persistence proof
- **`anylist-pp-cli collections list`** - List all recipe collections
- **`anylist-pp-cli collections remove`** - Preview a removal; live mutation is fail-closed pending persistence proof

### favorites

Manage favorite items. Preview is the default; `--apply` is required and every mutation is verified by a fresh read-back.

- **`anylist-pp-cli favorites`** - List favorite items across all lists
- **`anylist-pp-cli favorites add --list LIST --name NAME`** - Add an item to the linked favorite list
- **`anylist-pp-cli favorites remove --list LIST --item ITEM`** - Remove an item from the linked favorite list

### folders

View and safely organize shopping-list folders. Preview is the default and `--apply` is required for live mutations; each enabled mutation is verified by a fresh read-back before cache sync.

- **`anylist-pp-cli folders create`** - Create a folder with `--name`
- **`anylist-pp-cli folders delete`** - Delete an empty folder with `--name`; non-empty folders are refused so stale child memberships cannot be created
- **`anylist-pp-cli folders list`** - List all list folders
- **`anylist-pp-cli folders update`** - Rename with `--new-name`, move with `--parent`, or reorder children with `--order`; `--apply` performs the live mutation and requires fresh read-back verification. The explicit disposable probe is `ANYLIST_FOLDER_ORDERING_PROBE=1 go test -run '^TestFolderOrderingLiveProbe$' ./internal/anylist`.

Folder create/delete, rename, parent moves, and child ordering use handlers verified against fresh AnyList read-back; the APK-equivalent `set-ordered-folder-items` payload and live proof are recorded in `.printing-press-patches/review-folder-ordering-live-probe.json`.

### items

Manage items within a shopping list

- **`anylist-pp-cli items add`** - Add an explicitly selected item or create a new item. Semantic ingredient equivalence and candidate selection belong to the AI workflow, not the press. Preview is the default, `--apply` is required, and the CLI performs fresh live and cache read-back verification before success.
- **`anylist-pp-cli items check`** - Mark one or more items as checked (bought)
- **`anylist-pp-cli items list`** - List items in a shopping list; JSON output includes cached package size, photo IDs, prices, store IDs, category match ID, and category assignments when available
- **`anylist-pp-cli items lookup`** - Look up product details by UPC/EAN barcode
- **`anylist-pp-cli items recent`** - Show recently added items across all lists
- **`anylist-pp-cli items recent remove`** - Remove an entry from the Recent Items autocomplete store (sharded across many starter-list chunks). Preview is the default and `--apply` is required; the selector must be an exact item ID or an exact name (case-insensitive), and a name present in several chunks is rejected until disambiguated with the ID or `--chunk`. Every removal is verified by a fresh read-back of all chunks before the local cache is updated.
- **`anylist-pp-cli items remove`** - Remove an item from a shopping list
- **`anylist-pp-cli items search`** - Search for items by name across all shopping lists
- **`anylist-pp-cli items uncheck`** - Mark one or more items as unchecked
- **`anylist-pp-cli items update`** - Preview or update an item by stable ID; every live write requires `--apply` and fresh read-after-write verification
- **`anylist-pp-cli items photo attach`** - Upload and attach an item photo; `--apply` is required and the live item is verified before success. Verified photo IDs are persisted in the local cache (`cache_updated:true`).

### lists

Manage shopping lists. Create, rename, delete, item reset, cached settings reads, initial settings creation, settings clear, the live-verified item sort-order and eleven boolean setting updates, notification-location changes, and invite-add are available with explicit `--apply` gates and fresh read-back checks. Sharing removal, stop-sharing, household-user writes, and other settings remain fail-closed.

- **`anylist-pp-cli lists by-store`** - Display a shopping list grouped by store with aisle ordering
- **`anylist-pp-cli lists create`** - Create a new shopping list using AnyList's protobuf operation; the CLI verifies the list is readable before updating its local cache
- **`anylist-pp-cli lists delete`** - Delete a shopping list by name using AnyList's protobuf operation; the CLI verifies the list is gone before updating its local cache
- **`anylist-pp-cli lists list`** - List all shopping lists
- **`anylist-pp-cli lists rename`** - Preview or rename a shopping list with explicit `--apply`; the list is read back by stable ID before updating its local cache
- **`anylist-pp-cli lists reset`** - Clear all checked items from a list to prepare for the next shopping trip
- **`anylist-pp-cli lists settings`** - View cached settings for a shopping list
- **`anylist-pp-cli lists settings update-sort-order`** - Update a live-verified item sort order with `--apply`; existing settings are cloned and fresh read-back is required
- **`anylist-pp-cli lists settings update-flags`** - Update one of eleven live-verified boolean settings with `--setting`, `--value true|false`, and `--apply`; preview is the default
- **`anylist-pp-cli lists settings save`** - Create a list's initial settings record with `--apply`; refuses to overwrite an existing record and verifies fresh read-back
- **`anylist-pp-cli lists settings clear`** - Remove a list's settings record with `--apply` and fresh absence verification
- **`anylist-pp-cli lists notifications add/remove`** - Add or remove a notification location with `--apply` and fresh presence/absence verification
- **`anylist-pp-cli lists sharing list`** - Show accepted shared users from fresh user data
- **`anylist-pp-cli lists sharing add LIST --email ADDRESS`** - Preview an invite by exact list ID or unique name; pass `--apply` to send it. A fresh read reports an unregistered invite as pending rather than claiming it is accepted. Removal and household-user writes remain unsupported.

### meal

Manage the meal planning calendar

- **`anylist-pp-cli meal add`** - Preview by default or create a typed meal-calendar event with explicit `--apply`; `--scale-factor` controls recipe servings, calendar/recipe/label IDs are resolved, and fresh read-back is required before cache sync (live round-trip verified)
- **`anylist-pp-cli meal add-to-list`** - Preview meal-plan ingredients; live bulk writes fail closed
- **`anylist-pp-cli meal delete`** - Preview by default or delete a typed meal-calendar event with explicit `--apply`; deletion is verified absent before cache sync (live round-trip verified)
- **`anylist-pp-cli meal labels`** - List meal plan labels (Breakfast, Lunch, Dinner, Snack, etc.)
- **`anylist-pp-cli meal show`** - Show meal plan events for a date range (defaults to current week)
- **`anylist-pp-cli meal summary`** - Display the meal plan as a Mon–Sun grid with Breakfast/Lunch/Dinner labels
- **`anylist-pp-cli meal update`** - Preview by default or update a typed meal-calendar event with explicit `--apply`; `--scale-factor` updates recipe servings, omitted fields are preserved, and fresh read-back is required (live round-trip verified)

### recipes

Manage recipes — import, organize, add to shopping lists, and manage recipe-sharing links. Sharing writes are preview-only by default and require explicit `--apply` plus fresh read-back verification; email/print sending is not exposed.

- **`anylist-pp-cli recipes add-to-list`** - Deprecated bulk path; fails closed and directs the AI to `recipes ingredients` plus explicit item operations
- **`anylist-pp-cli recipes batch-add`** - Deprecated bulk path; fails closed for the same reason
- **`anylist-pp-cli recipes create`** - Create a new recipe
- **`anylist-pp-cli recipes delete`** - Delete a recipe
- **`anylist-pp-cli recipes filter`** - Filter recipes by prep time, rating, servings, and collection
- **`anylist-pp-cli recipes import`** - Import a recipe from a URL; exact-name duplicates are skipped by default, with `--on-duplicate update|allow` available for explicit handling
- **`anylist-pp-cli recipes import-paprika`** - Preview or explicitly apply a validated Paprika archive/file import; duplicate candidates are reported and photo/category/collection writes are not attempted
- **`anylist-pp-cli recipes duplicates`** - Report duplicate recipe names from the local cache without changing anything
- **`anylist-pp-cli recipes link`** - Show a cached recipe's source URL (read-only local operation)
- **`anylist-pp-cli recipes sharing list`** - Show pending requests, requests awaiting confirmation, and linked users from fresh user data
- **`anylist-pp-cli recipes sharing request`** - Preview or explicitly request recipe sharing with an exact email address
- **`anylist-pp-cli recipes sharing cancel`** - Preview or explicitly cancel a pending request by exact ID or confirming email
- **`anylist-pp-cli recipes sharing accept`** - Preview or explicitly accept an incoming request by exact ID
- **`anylist-pp-cli recipes sharing unlink`** - Preview or explicitly unlink a linked user by exact user ID or email
- **`anylist-pp-cli recipes list`** - List all recipes
- **`anylist-pp-cli recipes ingredients`** - Return raw recipe ingredients and fresh shopping-list candidate facts
- **`anylist-pp-cli recipes missing`** - Show which recipe ingredients are not already on a shopping list
- **`anylist-pp-cli recipes scale`** - Scale a recipe's ingredient quantities to a target serving count
- **`anylist-pp-cli recipes search`** - Search recipes by name or ingredient (uses local SQLite cache)
- **`anylist-pp-cli recipes show`** - Show full recipe details including ingredients and preparation steps
- **`anylist-pp-cli recipes photo attach`** - Preview a validated image offline, or upload and attach it to a recipe with explicit `--apply`; the recipe is read back by ID before success and verified photo IDs are persisted in the local cache (`cache_updated:true`)
- **`anylist-pp-cli recipes photo clear`** - Preview or remove one recipe photo with `--photo-id`, or all recipe photos when omitted; explicit `--apply` and fresh read-back verification are required (`cache_updated:true`)
- **`anylist-pp-cli recipes update`** - Preview selected recipe changes by default; pass `--apply` to save them through the verified `save-recipe` path. Omitted fields are preserved (`--new-name`, `--source-name`, `--source-url`, `--note`, `--nutrition`, `--servings`, `--prep-time`, `--cook-time`, or `--rating`); `--stdin` also accepts Paprika-cleanup ingredients, preparation steps, and `apply: true`.

`recipes import-paprika --input <path>` is preview-only by default and parses the complete `.paprika`/`.paprikarecipe` input before reporting planned creates, skips, and duplicate candidates. Add `--apply` to permit AnyList writes; matching recipes are skipped unless `--update-existing` is supplied, ambiguous updates are rejected, and live ingredient/step content is verified before the cache is refreshed. Paprika categories are report-only, and no photo, collection, or Walmart operation is guessed.

Recipe photo commands use the captured `/data/photos/upload` plus `save-recipe` protobuf flow. Attach and clear are preview-only by default, require `--apply` for writes, verify the complete photo-ID result with a fresh live read, and sync the photo IDs into the local cache. Recipe sharing uses the separate typed link-request endpoints; it does not send email or print invitations.

### starters

Manage user starter-list items. Preview is the default; `--apply` is required and every mutation is verified by a fresh read-back.

- **`anylist-pp-cli starters`** - List starter list items
- **`anylist-pp-cli starters add --list LIST --name NAME`** - Add an item to a starter list
- **`anylist-pp-cli starters remove --list LIST --item ITEM`** - Remove an item from a starter list

### stores

View stores and store filters (write operations are not exposed). This metadata is not a ZIP-aware retailer-pricing feed; use a separate Walmart press for Walmart store selection, price, and availability queries.

- **`anylist-pp-cli stores`** - List all stores and store filters

### export

Export a complete protobuf snapshot without mutating AnyList.

- **`anylist-pp-cli export`** - Write JSON to stdout or `--output`; use `--gzip` only with an output path (files are created mode 600)

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
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

`lists delete --name <name>` (or `--stdin` with `{"name":"<name>"}`) resolves the current live list, sends the verified `remove-shopping-list` protobuf operation, and confirms the list is absent before reporting success. `--dry-run` remains offline and does not resolve or delete a list.

`lists create --name <name>` (or `--stdin` with `{"name":"<name>"}`) sends the verified `new-shopping-list` protobuf operation, reads the generated list back by ID, and updates the local cache only after that verification succeeds. `--dry-run` remains offline.

`recipes update --name <existing-name>` previews by default and preserves omitted recipe fields. Pass `--apply` to write; use `--new-name` for a rename, or `--stdin` with `{"recipe":"<existing-name>","new_name":"...","ingredients":[...],"preparation_steps":[...],"apply":true}` to replace only the supplied fields. The update is read back by recipe ID before the local cache is updated.

`recipes import --url <url>` skips an exact-name duplicate by default. Use `--on-duplicate update` to replace a unique matching recipe, or `--on-duplicate allow` to create another copy. Ambiguous updates fail closed. `recipes duplicates` reports the local exact-name groups that need cleanup.

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
```

**Find recipes by ingredient and scale servings:**

```bash
anylist-pp-cli recipes search --query chicken --ingredient --json | jq '.[0].name'
anylist-pp-cli recipes scale --name "Chicken Tikka" --servings 8
```

**Sync, filter, and pipe items to another tool:**

```bash
anylist-pp-cli sync && anylist-pp-cli items list --list Groceries --unchecked --json \
  | jq -r '.[].name' | sort
```

**Look up a packaged product before adding it:**

```bash
anylist-pp-cli items lookup --barcode 049000028904 --json
anylist-pp-cli items add --list Groceries --item "Coca-Cola" \
  --barcode 049000028904 --package-size "12 count, 12 fl oz" --apply
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
