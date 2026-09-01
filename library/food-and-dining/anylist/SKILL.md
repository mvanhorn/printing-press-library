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

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install anylist --cli-only
   ```
2. Verify: `anylist-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/cmd/anylist-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

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
- **`recipes missing`** — Show which recipe ingredients are not already on a shopping list.

  _Use this as a read-only audit before adding ingredients — it reports what is missing so the AI can make the right reuse/create/ask decisions._

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
- **`recipes ingredients`** — Return raw recipe ingredient rows plus a fresh read of target-list candidates and stored-price facts. This is the read-only fact packet for the AI meal-planning workflow; it preserves source wording and does not choose or write an item.

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

Use the global `--db <path>` option for an isolated per-run SQLite cache. AnyList returns one complete user-data envelope, so `sync --resources <hint>` is an automation hint rather than a server-side filter; `--full` explicitly requests the normal complete refresh.

## AnyList Meal Planning Workflow

Use this skill as the coordinator and the press as its factual execution layer. The workflow is intentionally approval-gated: recipe selection and ingredient identity are AI decisions; AnyList writes are explicit stable-ID operations.

### Preferences

On the first run, conduct the complete interview and ask which answers should become defaults: purpose and cadence; number of meals, dates, servings and leftovers; budget and nutrition goals; dietary restrictions, allergies and disliked foods; preferred, limited and excluded proteins; cuisine, difficulty, time and equipment; freezer and pantry/fridge inventory; target AnyList list, store, ZIP code and aisle/category preferences; review/debug mode; and whether calendar events should be scheduled after approval. Save only non-secret preferences.

On later runs, print the saved profile and ask whether to use it unchanged, customize this run only, or edit and save the defaults. One-run overrides do not silently change the saved profile. The companion skill stores the profile at `~/.config/anylist-meal-planning/preferences.json` with mode `0600`.

### Curate, approve, then resolve

Read recipes, collections, servings, preparation time, nutrition and available item metadata. Prefer economical recipes that share staples, avoid recent repeats, and provide variety. Return the proposed recipes and a raw combined ingredient list for approval; do not write recipes, calendar events or shopping-list items before approval. After approval, ask what is already in the pantry/fridge.

The `recipes ingredients` packet preserves each source ingredient separately, including wording, quantity, note, recipe and form context, and returns fresh candidate facts such as stable IDs, checked state, quantity, notes, UPC, package size, photos, stores, categories and stored prices. The AI then chooses `reuse`, `recycle`, `create`, `skip` (pantry), or `ask` (ambiguous). It must not infer block, sliced or shredded cheese when the recipe does not specify the form.

### Review table before writes

When review/debug mode is enabled, show the following table before any shopping-list mutation:

| Ingredient needed | Form/package | Proposed AnyList item | Source | Metadata comparison | UPC action | Stored price | Proposed action | Confidence/notes |
|---|---|---|---|---|---|---:|---|---|
| ½ cup cheddar | unresolved | — | — | block/sliced/shredded differ | ask | — | ask | recipe does not specify form |
| 1 gallon milk | gallon | Milk - Gallon | historical | package and barcode agree | reuse | $3.48 | recycle | previous item has best metadata |
| 8 oz shredded cheese | shredded, 8 oz | Shredded Sharp Cheddar Cheese | current | strongest metadata among candidates | keep | $2.97 | reuse | selected stable ID |
| jarred garlic | jar | — | new | no suitable candidate | attempt lookup | — | create | UPC pending |

The table distinguishes current exact items, historical items, metadata mismatches, new items and unresolved ambiguity. With review/debug off, the same AI resolution is still required; the table is simply omitted.

### Pricing and UPC enrichment

For now, report only prices already stored on AnyList, with a known-price subtotal and missing-price count. Always disclose: **Prices come from stored AnyList item data. They may be outdated, incomplete, or missing; this is not a live Walmart price or availability quote.** ZIP-aware live store pricing and aisle lookup require a future Walmart press.

When creating a new item, the AI may propose a UPC only when product form and package are specific. Use `items lookup --barcode` or barcode-first creation, then read the item back and compare the returned name, brand, package size, photo, price, store and category. Never attach a guessed UPC to a generic ingredient; ask when multiple products are plausible. If AnyList does not enrich programmatic writes as the app does, leave uncertain metadata unset and report that limitation.

### Partial-Quantity Accounting Rule

Every partial amount — regardless of whether it comes from a bag, jar, box, bottle, or loose unit — is compared against each scaled recipe requirement and resolved as **skip** (pantry), **buy-shortfall/remainder** (recycle an existing item or create a new one with the right stable ID), or **ask** (ambiguous). This rule applies per-run and must hold on repeated runs: if a previous run recycled a gallon of milk, the next run compares against the remaining gallon quantity, not the original full quantity.

### Apply and verify

Remain read-only until recipe approval, pantry adjustments, ingredient decisions and the review gate are complete. Preview by default; pass `--apply` for live mutations on commands that support writes. **Bulk paths fail closed:** `meal add-to-list` and `recipes add-to-list`/`recipes batch-add` do not support `--apply` — they preview or fail closed. Live writes use explicit stable-ID operations (`items add` / `items update --item-id`) with `--apply`, followed by fresh read-after-write verification. Reuse or recycle only the selected stable ID, preserve quantity and package size, protect checked items, perform fresh reads, and report partial failures without silently retrying as new items. Calendar create/update/delete is supported behind the same explicit `--apply` and fresh read-back gate; a disposable live round-trip has been verified.

## Command Reference

**categories** — View item categories, and create, rename, delete, or reorder custom categories

- `anylist-pp-cli categories` — List all item categories (read-only)
- `anylist-pp-cli categories create --list <list> --name <name>` — Preview or create a custom category with explicit `--apply`; the list and category group are resolved by a fresh live read (the list must have exactly one group unless `--category-group` selects one), the new category gets a non-conflicting stable ID and a valid sort index, and the create is verified by stable ID before success
- `anylist-pp-cli categories rename --list <list> --category <id-or-name> --new-name <name>` — Preview or rename one custom category by stable ID or exact name with explicit `--apply`; ambiguous or missing names fail closed, the stable identifier/group/list/sort index are preserved, and the new name is verified by stable ID before success
- `anylist-pp-cli categories delete --list <list> --category <id-or-name>` — Preview or delete one custom category by stable ID or exact name with explicit `--apply`; ambiguous or missing names fail closed, system categories cannot be deleted, and the deletion is verified by confirming the stable ID is absent from a fresh read before success
- `anylist-pp-cli categories reorder --list <list> --order <id-or-name,id-or-name,…>` — Preview or reorder a category group's categories with explicit `--apply`; the list must have exactly one group unless `--category-group` selects one, `--order` must name every category in the group exactly once (duplicates, unknown entries, and silently appended or dropped categories fail closed), and the exact stable-ID order is verified from a fresh read before success
- Custom category writes use the verified multipart `/data/shopping-lists/update-v2` wire contract (the non-persistent v1 route and the old non-persistent `remove-category` handler are never used). Category-group create, rename, and delete (group CRUD) remain unsupported.

**collections** — List recipe collections and preview collection writes. Create/add/remove/delete previews are available, but live writes currently fail closed because a disposable AnyList create acknowledged HTTP success without persisting on fresh read-back. Do not use `--apply` until a future live probe proves the exact collection contract.

- `anylist-pp-cli collections add` — Preview an add; live mutation is fail-closed pending persistence proof
- `anylist-pp-cli collections create` — Preview a create; live mutation is fail-closed pending persistence proof
- `anylist-pp-cli collections delete` — Preview a delete; live mutation is fail-closed pending persistence proof
- `anylist-pp-cli collections list` — List all recipe collections
- `anylist-pp-cli collections remove` — Preview a removal; live mutation is fail-closed pending persistence proof

**favorites** — Manage favorite items with explicit apply gates and fresh read-after-write verification. Photo and price mutation remain separate capabilities.

- `anylist-pp-cli favorites` — List favorite items across all lists
- `anylist-pp-cli favorites add --list LIST --name NAME` — Preview or add an item to a favorite list; pass `--apply` to write
- `anylist-pp-cli favorites remove --list LIST --item ITEM` — Preview or remove an item from a favorite list; pass `--apply` to write

**folders** — View and safely organize shopping-list folders. Preview is the default and `--apply` is required for live mutations; enabled mutations require fresh read-back before cache sync.

- `anylist-pp-cli folders create` — Create a folder with `--name`
- `anylist-pp-cli folders delete` — Delete an empty folder with `--name`; non-empty folders are refused
- `anylist-pp-cli folders list` — List all list folders
- `anylist-pp-cli folders update` — Rename with `--new-name`, move with `--parent`, or reorder children with `--order`; pass `--apply` for live mutation and fresh verification. An explicit disposable ordering probe is available with `ANYLIST_FOLDER_ORDERING_PROBE=1`.

Create/delete, rename, parent movement, and child ordering use handlers verified against live fresh read-back; the APK-equivalent `set-ordered-folder-items` payload is recorded in `.printing-press-patches/review-folder-ordering-live-probe.json`.

**items** — Manage items within a shopping list

- `anylist-pp-cli items add` — Add an explicitly selected item to a shopping list. The AI must choose the stable item ID or explicitly request creation. Preview is the default and `--apply` is required for every live write; fresh live and cache read-back verification must succeed before the command reports success. Use `--store` for a store assignment and `--price`, `--price-store`, or `--price-details` for price metadata. Store removal remains an `items update --remove-store` operation.
- `anylist-pp-cli items check` — Mark one or more items as checked (bought)
- `anylist-pp-cli items list` — List items in a shopping list; JSON output includes cached package size, photo IDs, prices, store IDs, category match ID, and category assignments when available
- `anylist-pp-cli items lookup` — Look up product details by UPC/EAN barcode
- `anylist-pp-cli items recent` — Show recently added items across all lists
- `anylist-pp-cli items recent remove` — Preview or remove an entry from the Recent Items store (the sharded autocomplete backing store). The selector must be an exact item ID or an exact name (case-insensitive); a name present in several chunks is rejected until disambiguated with the exact item ID or `--chunk`. Preview resolves against a fresh live read; `--apply` is required to write, and the removal is verified by reading back every chunk before the local cache is updated.
- `anylist-pp-cli items remove` — Remove an item from a shopping list
- `anylist-pp-cli items search` — Search for items by name across all shopping lists
- `anylist-pp-cli items uncheck` — Mark one or more items as unchecked
- `anylist-pp-cli items update` — Preview or update an item by stable ID; every live write requires `--apply` and fresh read-after-write verification
- `anylist-pp-cli items photo attach` — Upload and attach an item photo; `--apply` is required and the live item is verified before success. Verified photo IDs are persisted in the local cache (`cache_updated:true`).

**lists** — Manage shopping lists. Create, rename, delete, item reset, cached settings reads, initial settings creation, settings clear, the live-verified item sort-order and eleven boolean setting updates, notification-location changes, and invite-add are available with explicit `--apply` gates and fresh read-back checks. Sharing removal, stop-sharing, household-user writes, and other settings remain fail-closed.

- `anylist-pp-cli lists by-store` — Display a shopping list grouped by store with aisle ordering
- `anylist-pp-cli lists create` — Create a shopping list via the verified `new-shopping-list` protobuf operation, then confirm it is readable
- `anylist-pp-cli lists delete` — Delete a shopping list by name via the verified `remove-shopping-list` protobuf operation, then confirm it is absent
- `anylist-pp-cli lists list` — List all shopping lists
- `anylist-pp-cli lists rename` — Preview or rename a shopping list with explicit `--apply`; the list is read back by stable ID before cache sync
- `anylist-pp-cli lists reset` — Clear all checked items from a list to prepare for the next shopping trip
- `anylist-pp-cli lists settings` — View cached settings for a shopping list
- `anylist-pp-cli lists settings update-sort-order` — Update a live-verified item sort order with `--apply`; existing settings are cloned and fresh read-back is required
- `anylist-pp-cli lists settings update-flags` — Update one of eleven live-verified boolean settings with `--setting`, `--value true|false`, and `--apply`; preview is the default
- `anylist-pp-cli lists settings save` — Create an initial settings record with `--apply`; existing records are refused and fresh read-back is required
- `anylist-pp-cli lists settings clear` — Remove a settings record with `--apply`; fresh absence read-back is required
- `anylist-pp-cli lists notifications add/remove` — Add or remove a notification location with `--apply`; fresh presence/absence read-back is required
- `anylist-pp-cli lists sharing list` — Show accepted shared users from fresh user data
- `anylist-pp-cli lists sharing add LIST --email ADDRESS` — Preview an invite by exact list ID or unique name; pass `--apply` to send it. A fresh read reports an unregistered invite as pending rather than claiming it is accepted. Removal and household-user writes remain unsupported.

**meal** — Manage the meal planning calendar

- `anylist-pp-cli meal add` — Preview by default or create a typed meal-calendar event with explicit `--apply`; supports a positive finite `--scale-factor` for recipe servings, resolves calendar/recipe/label IDs, and verifies fresh read-back before cache sync (live round-trip verified)
- `anylist-pp-cli meal add-to-list` — Preview meal-plan ingredients; live bulk writes fail closed
- `anylist-pp-cli meal delete` — Preview by default or delete a typed meal-calendar event with explicit `--apply`; deletion is verified absent before cache sync (live round-trip verified)
- `anylist-pp-cli meal labels` — List meal plan labels (Breakfast, Lunch, Dinner, Snack, etc.)
- `anylist-pp-cli meal show` — Show meal plan events for a date range (defaults to current week)
- `anylist-pp-cli meal summary` — Display the meal plan as a Mon–Sun grid with Breakfast/Lunch/Dinner labels
- `anylist-pp-cli meal update` — Preview by default or update a typed meal-calendar event with explicit `--apply`; supports `--scale-factor`, preserves omitted fields, and requires fresh read-back (live round-trip verified)

**recipes** — Manage recipes — import, organize, add to shopping lists, and manage recipe-sharing links. Sharing writes are preview-only by default and require explicit `--apply` plus fresh read-back verification; email/print sending is not exposed.

- `anylist-pp-cli recipes add-to-list` — Deprecated bulk path; fails closed and directs the AI to `recipes ingredients` plus explicit item operations
- `anylist-pp-cli recipes batch-add` — Deprecated bulk path; fails closed for the same reason
- `anylist-pp-cli recipes create` — Create a new recipe
- `anylist-pp-cli recipes delete` — Delete a recipe
- `anylist-pp-cli recipes filter` — Filter recipes by prep time, rating, servings, and collection
- `anylist-pp-cli recipes import` — Import a recipe from a URL; exact-name duplicates are skipped by default, with `--on-duplicate update|allow` available for explicit handling
- `anylist-pp-cli recipes import-paprika` — Preview or explicitly apply a validated Paprika archive/file import; duplicate candidates are reported and photo/category/collection writes are not attempted
- `anylist-pp-cli recipes duplicates` — Report duplicate recipe names from the local cache without changing anything
- `anylist-pp-cli recipes link` — Show a cached recipe's source URL (read-only local operation)
- `anylist-pp-cli recipes sharing list` — Show pending requests, requests awaiting confirmation, and linked users from fresh user data
- `anylist-pp-cli recipes sharing request` — Preview or explicitly request recipe sharing with an exact email address
- `anylist-pp-cli recipes sharing cancel` — Preview or explicitly cancel a pending request by exact ID or confirming email
- `anylist-pp-cli recipes sharing accept` — Preview or explicitly accept an incoming request by exact ID
- `anylist-pp-cli recipes sharing unlink` — Preview or explicitly unlink a linked user by exact user ID or email
- `anylist-pp-cli recipes list` — List all recipes
- `anylist-pp-cli recipes ingredients` — Return raw recipe ingredients and fresh shopping-list candidate facts
- `anylist-pp-cli recipes missing` — Show which recipe ingredients are not already on a shopping list
- `anylist-pp-cli recipes scale` — Scale a recipe's ingredient quantities to a target serving count
- `anylist-pp-cli recipes search` — Search recipes by name or ingredient (uses local SQLite cache)
- `anylist-pp-cli recipes show` — Show full recipe details including ingredients and preparation steps
- `anylist-pp-cli recipes photo attach` — Preview a validated image offline, or upload and attach it to a recipe with explicit `--apply`; the recipe is read back by ID before success and verified photo IDs are persisted in the local cache (`cache_updated:true`)
- `anylist-pp-cli recipes photo clear` — Preview or remove one recipe photo with `--photo-id`, or all recipe photos when omitted; explicit `--apply` and fresh read-back verification are required (`cache_updated:true`)
- `anylist-pp-cli recipes update` — Preview selected recipe changes by default; `--apply` enables the verified `save-recipe` write and fresh read-after-write/cache verification. Omitted fields are preserved (`--new-name`, `--source-name`, `--source-url`, `--note`, `--nutrition`, `--servings`, `--prep-time`, `--cook-time`, `--rating`); stdin also accepts ingredient and preparation-step arrays plus `apply: true`.

`recipes import-paprika --input <path>` parses the complete `.paprika`/`.paprikarecipe` input and previews planned creates, skips, and duplicate candidates without network access. `--apply` is required for AnyList writes; `--update-existing` opts into updates, ambiguous matches are rejected, and live ingredient/step content is verified before cache refresh. Paprika categories are report-only; photo, collection, and Walmart operations are not guessed.

`recipes import --url <url>` skips an exact-name duplicate by default. Use `--on-duplicate update` to replace a unique matching recipe, or `--on-duplicate allow` to create another copy. Ambiguous updates fail closed. `recipes duplicates` reports the local exact-name groups that need cleanup.

Recipe photo commands use the captured `/data/photos/upload` plus `save-recipe` protobuf flow. Attach and clear are preview-only by default, require `--apply` for writes, verify the complete photo-ID result with a fresh live read, and sync the photo IDs into the local cache. Recipe sharing uses the separate typed link-request endpoints; it does not send email or print invitations.

**starters** — Manage user starter-list items with explicit apply gates and fresh read-after-write verification. Photo and price mutation remain separate capabilities.

- `anylist-pp-cli starters` — List starter list items
- `anylist-pp-cli starters add --list LIST --name NAME` — Preview or add an item to a starter list; pass `--apply` to write
- `anylist-pp-cli starters remove --list LIST --item ITEM` — Preview or remove an item from a starter list; pass `--apply` to write

**stores** — View stores and store filters (write operations are not exposed). AnyList does not provide a ZIP-aware Walmart price/availability query; use a separate Walmart press for that capability.

- `anylist-pp-cli stores` — List all stores and store filters

**export** — Create a local JSON snapshot

- `anylist-pp-cli export --output anylist-backup.json` — Export all data returned by AnyList; `--gzip` requires `--output` and output files use mode 600


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
anylist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Sunday meal-prep: build grocery list from this week's plan

**Phase 1 — read raw ingredient facts:**

```bash
anylist-pp-cli recipes ingredients --name "Pasta Bake" --list Groceries --scale 4 --agent
```

**Phase 2 — AI resolves identity and pantry quantities, then shows the review table:**

Compare each partial amount (½ cup, 8 oz, 1 gallon, etc.) against the scaled recipe requirements. Resolve as: **skip** (pantry), **buy-shortfall/remainder** (recycle or create with stable ID), or **ask** (ambiguous). Show the review table with proposed actions.

**Phase 3 — explicit item operations (no bulk):**

```bash
anylist-pp-cli items update --item-id <id> --quantity 4 --apply --json
anylist-pp-cli items add --list Groceries --item "Shredded Sharp Cheddar Cheese" --package-size "8 oz bag" --apply --json
```

**Phase 4 — read-after-write verification:**

```bash
anylist-pp-cli items list --list Groceries --agent
```

`meal add-to-list --week --dry-run` previews ingredients only. Live bulk writes on this command are disabled (fail closed).

### Find quick weeknight recipes with available chicken

```bash
anylist-pp-cli recipes search --query chicken --ingredient --agent | anylist-pp-cli recipes filter --max-prep 30 --min-rating 4
```

Search by ingredient then filter by metadata — pure offline SQLite, no API calls.

### Check what you still need before adding a recipe

```bash
anylist-pp-cli recipes missing --recipe "Thai Green Curry" --list Groceries --agent
```

Shows only the ingredients not already on your shopping list — use this as a read-only audit before the AI decides whether to reuse, create, or ask.

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
