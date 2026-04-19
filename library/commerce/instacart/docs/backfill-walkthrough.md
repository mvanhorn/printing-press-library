# Instacart history backfill — the one-command walkthrough

This is the canonical end-to-end backfill flow. A user types "backfill my instacart orders" (or a near-variant) into a Claude Code session and the `/pp-instacart` skill drives the rest.

The short version: Chrome MCP walks your logged-in Instacart tab, extracts each order's Apollo cache entry, downloads `instacart-orders.jsonl`, and then `instacart history import` upserts it into your local SQLite store. After that, `instacart add <retailer> "<thing you've bought>"` resolves via history instead of live search.

## Before you start

You need:

- `instacart-pp-cli` installed and authenticated (`instacart doctor` should show session OK, at least one op-hash cached, history warn is fine).
- A Chrome tab signed in to https://www.instacart.com with the correct profile selected (multi-profile accounts redirect through `/store/profiles` on a fresh session).
- The `claude-in-chrome` MCP tools available in Claude Code. Run `/pp-instacart backfill my orders` from a session where you have those tools.

If you do not have Chrome MCP available, follow [`backfill-devtools-fallback.md`](backfill-devtools-fallback.md) instead. Same JS files, same import command, different driver.

## The loop

The skill drives Chrome MCP to run three JS files in order. You can read them on GitHub:

- Dumper: [`library/commerce/instacart/docs/dumper.js`](dumper.js)
- Per-order extractor: [`library/commerce/instacart/docs/extract-one.js`](extract-one.js)
- JSONL exporter: [`library/commerce/instacart/docs/export-jsonl.js`](export-jsonl.js)

The flow:

1. Navigate to `https://www.instacart.com/store/account/orders`. If the tab redirects to `/store/profiles`, pick a profile and try again.
2. Inject `dumper.js` into the page via `mcp__claude-in-chrome__javascript_tool`. It scrolls + clicks "Load more" until no new order IDs appear, then writes progress into `localStorage.__ic_backfill_state`.
3. Read the dumper output. `total_ids` is how many orders were seen. `pending_extract` is how many still need per-order extraction.
4. For each pending order ID, navigate to `/store/orders/<id>` then inject `extract-one.js`. The extractor polls for Apollo cache hydration (up to 10s), pulls the `OrderManagerOrderDelivery` entry, and appends the record to `localStorage.__ic_dumped`.
5. After the per-order loop finishes, inject `export-jsonl.js`. It filters skip records, writes the rest as JSONL, and triggers a browser download of `instacart-orders.jsonl` into your Downloads folder.
6. Run `instacart history import ~/Downloads/instacart-orders.jsonl` (use `--json` for agent output). Upsert is idempotent so re-runs are safe.
7. Verify with `instacart history stats` and a sanity-check `instacart add <retailer> "<something you buy>" --dry-run --json`. Look for `"resolved_via": "history"`.

## Resume and top-up

Both `dumper.js` and `extract-one.js` are designed to resume. Interrupted sessions (tab closed, MFA kicked in, computer slept) do not lose progress:

- `localStorage.__ic_backfill_state.seen_ids` — every order ID the dumper has seen on the orders page.
- `localStorage.__ic_dumped` — every order record the extractor has produced (valid records and structured skip records).

Re-running the skill reads both, cross-checks, and only navigates to orders that are still pending. If no orders are pending, it jumps straight to the exporter so `history import` can merge any newer orders.

For monthly top-ups (you have backfilled before and just want to catch new orders since then), the skill detects the populated store via `instacart history stats` and takes the top-up path: it walks the orders page to find any new IDs, extracts only those, and imports. Typically completes in under 30 seconds when nothing is new.

## Troubleshooting

- **`instacart doctor` warns "session: fail"** — cookies are stale. Run `instacart auth login` to re-extract from Chrome.
- **Dumper reports `profile_picker: true`** — the tab is at `/store/profiles`. Click a profile in your browser, then re-run.
- **Extractor reports `cache_not_hydrated` or `cache_key_missing` for every order** — Instacart probably rotated their web bundle. The extractor's fallback scan looks for any cache entry with `"includeOrderItems":true` in its key; if even that fails, the cache shape has changed materially. File an issue with the first few `Object.keys(window.__APOLLO_CLIENT__.cache.extract())` output from one of your order detail pages.
- **`history import` succeeds but `add` still falls through to live search** — check `instacart history search "<query>" --store <retailer>` to see whether the item is actually in your local store. The FTS match has to clear a confidence threshold (recent purchase, in stock on last purchase). Pass `--no-history` to force live search and compare.
- **Exporter says `only_skip_records`** — every extract failed. Usually means the tab got signed out mid-loop. Check `instacart doctor`, re-auth, and re-run.

## Why this flow exists

Instacart does not expose a clean GraphQL operation for order history. The orders index page is server-rendered HTML; per-order data lives in the client-side Apollo cache after the order detail page hydrates. See [`docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md`](../../../docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md) for the full architecture finding and [`docs/patterns/authenticated-session-scraping.md`](../../../docs/patterns/authenticated-session-scraping.md) for the tiered decision tree on when Chrome-MCP-driven scraping is the right tool for a printed CLI.
