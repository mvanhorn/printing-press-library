# Instacart history backfill — DevTools fallback

Use this flow when you do not have `claude-in-chrome` MCP tools in your Claude Code session, or when you just want to drive the backfill by hand.

Same result as the skill-driven flow: a JSONL file that `instacart history import` ingests. Just you in the driver seat instead of the skill.

Works only in Chrome or Chromium. Firefox/Safari lack the `window.__APOLLO_CLIENT__` global the extractor depends on.

## Prerequisites

- `instacart-pp-cli` installed and authenticated.
- Chrome logged in at https://www.instacart.com with the correct profile selected.

## Step 1 — Open the orders page + DevTools

1. Open https://www.instacart.com/store/account/orders in Chrome.
2. If redirected to `/store/profiles`, pick a profile first.
3. Open DevTools (Cmd+Option+I on macOS, Ctrl+Shift+I on Linux/Windows).
4. Switch to the Console tab.

## Step 2 — Collect your order IDs (dumper)

Paste the entire contents of [`dumper.js`](dumper.js) into the console and press Enter. Wait for the return value. It looks like:

```json
{
  "total_ids": 183,
  "new_ids_this_run": 183,
  "already_dumped": 0,
  "pending_extract": 183,
  "resumed": false,
  "state_key": "__ic_backfill_state",
  "pending_sample": ["123...", "456..."]
}
```

Note `total_ids` (how many orders the dumper found) and `pending_extract` (how many still need per-order extraction). If you see `"profile_picker": true`, pick a profile in the tab and paste the dumper again.

## Step 3 — Pull each order's detail (extractor)

For every order ID in `pending_extract`:

1. Navigate the tab to `https://www.instacart.com/store/orders/<order-id>`.
2. Paste the contents of [`extract-one.js`](extract-one.js) into the console. Press Enter. The extractor polls for the Apollo cache and normalizes the record; return value names the retailer + item count.

Do this for every pending order. The accumulator persists in `localStorage.__ic_dumped` so navigation between orders does not lose progress.

If you have 50+ orders, this is tedious by hand. Consider:

- A DevTools snippet (Sources -> Snippets) so you can double-click to run extract-one instead of pasting.
- A short `fetch` loop that navigates via `window.location.href = "/store/orders/" + id; await new Promise(r => setTimeout(r, 4500));` and then runs the extractor. Not included here because navigation tears down the JS context, so the loop has to be two snippets cooperating via localStorage.
- Or just use the skill-driven flow — `claude-in-chrome` MCP is free and handles the navigation loop for you. See [`backfill-walkthrough.md`](backfill-walkthrough.md).

## Step 4 — Export JSONL

Paste the contents of [`export-jsonl.js`](export-jsonl.js) into the console. Chrome downloads `instacart-orders.jsonl` to your default Downloads folder. The return value summarizes what was exported:

```json
{
  "downloaded": true,
  "filename": "instacart-orders.jsonl",
  "bytes": 48213,
  "orders": 183,
  "skipped_count": 0,
  "next_command": "instacart history import ~/Downloads/instacart-orders.jsonl"
}
```

If `skipped_count > 0`, some orders failed to extract. You can re-navigate to those (check `localStorage.__ic_dumped` for `skipped: true` entries) and re-run the extractor. Re-running the exporter overwrites the download with the new set.

## Step 5 — Import

In your terminal:

```bash
instacart history import ~/Downloads/instacart-orders.jsonl
```

Idempotent by primary key — safe to re-run. Output names how many orders and items landed.

Verify:

```bash
instacart history stats
instacart add <retailer> "<something you buy>" --dry-run --json
```

Look for `"resolved_via": "history"` in the dry-run output.

## Top-up later

Re-running `dumper.js` skips orders already in `__ic_backfill_state.seen_ids`. Re-running `extract-one.js` on an order already in `__ic_dumped` dedupes (the push is guarded). Re-running the exporter produces a superset. Re-running `history import` is idempotent. So the full loop is safe to repeat monthly for top-ups.

If you want to start over from zero, clear the browser-side state in DevTools:

```js
localStorage.removeItem('__ic_dumped');
localStorage.removeItem('__ic_backfill_state');
```

Then delete your local history tables with `instacart history import --dry-run` on a corrupt file (there is no first-class wipe command yet; open an issue if you need one).
