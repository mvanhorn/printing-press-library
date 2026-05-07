---
name: pp-instacart
description: "Printing Press CLI for Instacart. Natural-language Instacart CLI that talks directly to the web GraphQL API. Add items to your cart, search products, and manage carts across retailers without browser automation. Also caches your purchase history locally so 'add' resolves items you have bought before instead of guessing from live search. Trigger phrases: 'install instacart', 'use instacart', 'run instacart', 'add X to my Safeway cart', 'what did I buy last time', 'order the usual', 'add my regulars to Costco', 'backfill my instacart history', 'sync my instacart orders', 'download my order history', 'save my instacart history locally'."
version: "1.2.1"
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp | backfill"
allowed-tools: "Read Bash WebFetch"
metadata:
  openclaw:
    requires:
      bins:
        - instacart-pp-cli
    install:
      - kind: go
        bins: [instacart-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/instacart/cmd/instacart-pp-cli
---

# Instacart - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `instacart-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install instacart --cli-only
   ```
2. Verify: `instacart-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/instacart/cmd/instacart-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this when a user wants:

- Add a product to an Instacart cart by natural language ("add lemon sorbet to QFC")
- Add something they have bought before ("add my usual milk to Safeway")
- Show, search, or compare their active carts across retailers
- List or search their own Instacart order history
- Run an Instacart flow from a script, cron job, or agent loop

Do not reach for this if the user wants to actually check out. This CLI adds items to your cart; you still complete checkout in the Instacart app or web UI.

## Unique Capabilities

### History-first `add`

`add` checks your local purchase history FIRST and, when a confident match exists at the target retailer, skips the three-call live GraphQL chain entirely. Drops the cost of "add the lemon sorbet pops I usually get" from ~1.2s to ~200ms AND makes it resolve to the right SKU (the one you actually buy) instead of whatever live search ranks highest today.

Confidence rules:
- FTS5 match in your local purchased_items at that retailer
- Purchased within the last 365 days
- Was in stock on the last purchase

Falls through to today's live-search behavior when any condition fails. Pass `--no-history` to force live search.

Every successful `add` (history-resolved or live-resolved) writes back to `purchased_items` so the signal gets warmer without a full re-sync.

### One-command history backfill

Typing "backfill my instacart orders" (or similar, see Argument Parsing) kicks off a Chrome-MCP-driven flow that walks the user's logged-in Instacart tab, extracts their order history into JSONL, and imports it into the local DB. After backfill, `add` resolves from real purchase history instead of live-search guesses.

Primary path: Chrome MCP. Fallback: paste three JS files into DevTools by hand.

Full walkthrough below under "Backfill Flow". Reference docs with more detail:

- [`docs/backfill-walkthrough.md`](https://github.com/mvanhorn/printing-press-library/blob/main/library/commerce/instacart/docs/backfill-walkthrough.md) — Chrome MCP flow
- [`docs/backfill-devtools-fallback.md`](https://github.com/mvanhorn/printing-press-library/blob/main/library/commerce/instacart/docs/backfill-devtools-fallback.md) — manual DevTools flow

`history list` / `history search` / `history stats` inspect whatever has been loaded.

Instacart does not expose a clean order-history GraphQL op, so the legacy `history sync` command cannot work. See [`docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md`](https://github.com/mvanhorn/printing-press-library/blob/main/docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md) for why.

### Natural-language `add`

Resolves a product from free-text via Instacart's own three-call GraphQL chain (ShopCollectionScoped -> Autosuggestions -> Items) and fires `UpdateCartItemsMutation`. No browser automation.

When Instacart rejects a candidate with `notFoundBasketProduct` (autosuggest occasionally surfaces a product that is not addable at your active cart's shop), `add` automatically retries up to 3 ranked candidates before giving up. In `--json` output a successful retry sets `retry_count > 0` and includes an `attempts` array listing the rejected item ids. When history-first resolution hits the same error, `add` falls through to live search and reports `resolved_via: "history->live"`.

### Multi-retailer `carts`

`carts list` shows every active cart across retailers at once. Useful for agents that need to know where items live before adding to the right one.

## Command Reference

Authentication:

- `instacart auth login` - extract session cookies from Chrome
- `instacart auth status` - show current session state
- `instacart auth logout` - clear saved cookies
- `instacart auth paste` - paste cookie JSON manually (fallback for newer macOS Chrome)
- `instacart auth import-file <path>` - load cookies from a browser-use export JSON

Cart operations:

- `instacart add <retailer> <query...>` - add a product by natural language
- `instacart add <retailer> <query...> --no-history` - skip the history-first resolver
- `instacart add --item-id <id> <retailer>` - add by exact Instacart item id
- `instacart cart show <retailer>` - show current cart contents at a retailer
- `instacart cart remove <item-id> <retailer>` - remove an item from a cart
- `instacart carts list` - list every active cart across retailers

Discovery:

- `instacart search <query> --store <retailer>` - search products at a retailer
- `instacart retailers list` - list retailers available at your address
- `instacart retailers show <slug>` - cache one retailer locally

Purchase history:

- `instacart history import <path>` - load a JSONL order dump into the local DB (the working path)
- `instacart history import - --json` - read from stdin, JSON output for agent pipelines
- `instacart history import <path> --dry-run` - preview counts without writing
- `instacart history list` - top purchased items by count + recency
- `instacart history list --store <retailer> --limit 20` - filter + paginate
- `instacart history search <query>` - FTS search your purchase history
- `instacart history search <query> --store <retailer>` - scoped FTS search
- `instacart history stats` - counts + per-retailer state

Maintenance:

- `instacart doctor` - health check: config, store, ops, history, session, live ping
- `instacart capture` - refresh the GraphQL operation hash cache
- `instacart capture --remote` - merge fresh hashes from the community registry
- `instacart ops list` - show the operation-hash cache state

## Recipes

### First-time setup

```bash
instacart auth login                # extract cookies from Chrome
instacart doctor                    # verify auth + live ping
instacart capture                   # seed built-in op hashes
```

Then backfill history (optional but recommended; unlocks history-first `add`):

> Tell the agent: "backfill my instacart orders"

The skill drives the rest. See the "Backfill Flow" section below.

### Add something you buy all the time

```bash
instacart add safeway "oat milk"    # resolves via local history if you have bought it before
```

Look for `via history` in the output. If you see `via live`, the FTS match did not pass the confidence check; check `instacart history search "oat milk" --store safeway` to see what is actually in your history.

### Force a fresh live search

```bash
instacart add safeway "oat milk" --no-history --dry-run --json
```

`--dry-run --json` is useful when debugging - the output includes `resolved_via` so you can see which path would have fired.

### Daily top-up from recent history

```bash
instacart history list --store safeway --limit 20 --json | jq -r '.[].name' \
  | while read item; do instacart add safeway "$item" --yes --json; done
```

## Auth Setup

Requires a logged-in Instacart session in Chrome. The CLI extracts cookies via kooky (no credential handling on our side). If Chrome is locked or you are on a system kooky cannot read:

```bash
instacart auth paste         # paste the full cookie JSON manually
instacart auth import-file <path>
```

Session lives at `~/.config/instacart/session.json` (0600).

## Agent Mode

The CLI is agent-native by default. Pass `--json` on any command for machine-readable output. `--dry-run` previews `add` without firing the mutation and surfaces which resolver (`history`, `live`, or `item-id`) would have fired.

`add` JSON envelope fields worth knowing:

- `resolved_via`: one of `history`, `live`, `history->live` (history pick was rejected, live retry succeeded), or `item-id`.
- `retry_count`: how many candidates were rejected before the winner. `0` when the first pick landed.
- `attempts`: present only when `retry_count > 0`, array of `{item_id, name, error_type}` for each rejected candidate.
- On exhaustion (exit 5): JSON envelope with `error`, `retailer`, `query`, `attempts`, and a `hint` naming the concrete next step (`search` then `add --item-id`, or retry with `--no-history`).

### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
instacart-pp-cli <command> --agent --select id,name
instacart-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error |
| 3 | Auth missing or rejected |
| 4 | Resource not found |
| 5 | API error / conflict |
| 7 | Rate limited or transient network |

## Argument Parsing

Given a free-form natural-language request:

1. Empty, `help`, or `--help` -> run `instacart --help`
2. Starts with `install` -> CLI install; ends with `mcp` -> MCP install
3. Matches a **backfill intent** -> run the Backfill Flow below. Trigger phrases include: "backfill my orders", "backfill my history", "sync my instacart history", "sync my instacart orders", "download my order history", "save my instacart history locally", "pull in my past orders", "import my recent orders".
4. Anything else -> map to the best subcommand and run with `--json` when invoked from an agent

## Backfill Flow

Drive this when the user hits a backfill intent. Read [`docs/backfill-walkthrough.md`](https://github.com/mvanhorn/printing-press-library/blob/main/library/commerce/instacart/docs/backfill-walkthrough.md) via WebFetch for the full procedure; summary below.

Setup check:

1. Confirm `instacart-pp-cli` is on PATH. If not, install: `go install github.com/mvanhorn/printing-press-library/library/commerce/instacart/cmd/instacart-pp-cli@latest`.
2. Probe `mcp__claude-in-chrome__tabs_context_mcp`. If the tool is unavailable, route to the DevTools fallback: fetch [`docs/backfill-devtools-fallback.md`](https://github.com/mvanhorn/printing-press-library/blob/main/library/commerce/instacart/docs/backfill-devtools-fallback.md) and walk the user through it. Stop.
3. Run `instacart-pp-cli history stats --agent`. If orders > 0, this is a top-up run; the resume state will skip already-dumped orders automatically.

Chrome MCP loop:

1. WebFetch the three JS files (cache each in the current session):
   - `https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/library/commerce/instacart/docs/dumper.js`
   - `https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/library/commerce/instacart/docs/extract-one.js`
   - `https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/library/commerce/instacart/docs/export-jsonl.js`
2. Open or reuse a tab at `https://www.instacart.com/store/account/orders`. If the dumper returns `profile_picker: true`, ask the user to pick a profile in the tab, then re-run.
3. Inject `dumper.js` via `mcp__claude-in-chrome__javascript_tool`. Read back `total_ids` and `pending_extract`.
4. For each order ID in the pending set, navigate to `/store/orders/<id>` then inject `extract-one.js`. Report progress to the user every 10 orders.
5. When pending reaches 0, inject `export-jsonl.js`. It downloads `instacart-orders.jsonl` to the user's default Downloads folder.
6. Run `instacart-pp-cli history import ~/Downloads/instacart-orders.jsonl --agent` in a Bash tool. Show the summary JSON to the user.
7. Verify: `instacart-pp-cli history stats --agent`. Offer a follow-up sanity check: `instacart-pp-cli add <retailer> "<something they've bought>" --dry-run --json` and flag `resolved_via: "history"` when it appears.

Error surfaces worth translating for the user:

- Extractor `cache_key_missing` on every order -> Instacart rotated their web bundle. Report the observed cache keys and point at the rotation-recovery section of the walkthrough doc.
- Dumper reports fewer IDs than the user expected -> probably on a multi-profile account; ensure the selected profile is the one with purchase history.
- `history import` shows 0 orders imported -> the JSONL is empty (only skip records). Re-run the extractor loop with fresh tabs.
## Direct Use

1. Check installed: `which instacart-pp-cli`
2. Check auth: `instacart doctor`
3. Capture GraphQL hashes: `instacart capture`
4. (Optional but recommended) Backfill history — run the Backfill Flow above. Unlocks history-first `add` resolution.
5. Run your command with `--json` if invoked from an agent
