## Customer model

### Maya, solo handmade-goods seller (one shop, a few hundred listings)
- **Today (without this CLI):** Keeps the Shop Manager dashboard open next to a spreadsheet. Downloads the order CSV and the payment-account CSV by hand each week, and scrolls the Listings tab to eyeball which items look thin. Cannot answer "which listings have fewer than 13 tags or no alt text" or "which active listings never sold" without opening them one by one.
- **Weekly ritual:** Monday: clear the unshipped queue and add tracking. Midweek: fix listings that are about to expire or lapse. Friday: check stock on variations before a promo.
- **Frustration:** Catalog hygiene is invisible. Etsy's dashboard gives no cross-listing view of missing tags/alt text/photos or of expiring items, so problems are found by accident, one listing at a time.

### Priya, seller of mixed physical and digital goods who does her own books
- **Today (without this CLI):** Exports ledger and payment CSVs and reconciles them against receipts in a spreadsheet at month end. Etsy fees (listing, transaction, processing, offsite ads) arrive as ledger lines, so she pivots by hand to see what each fee type cost.
- **Weekly ritual:** Weekly: glance at fees and payouts. Monthly: close the month, match payouts to the bank, chase refunds.
- **Frustration:** She cannot quickly prove that every paid receipt has a matching payment/ledger entry and that the ledger balance runs without gaps. Month-end reconciliation is manual and error-prone.

### Ravi, developer/agent-builder automating shop operations for a seller
- **Today (without this CLI):** Wraps thin MCP servers or raw v3 calls. Every report costs live quota (N+1 calls), and he has read of apps banned for polling and many writes, so he rations calls. Has no local copy to query.
- **Weekly ritual:** Runs scheduled audits and hands an agent a "what needs attention in this shop" brief, then applies approved bulk edits in throttled batches.
- **Frustration:** There is no safe, quota-free, structured "what is wrong in this shop" feed. An agent either burns the daily budget or works from stale guesses.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description |
|---|------|---------|-------------|---------|--------|------------------|
| 1 | Fee breakdown | `money fees` | Etsy fees by ledger type per period, from the local ledger | Priya | (a) persona, (c) cross-entity | Use this command for fees by type over a period. Do NOT use it for payout/receipt matching; use 'money reconcile' instead. |
| 2 | Ledger reconcile | `money reconcile` | Paid receipts without payment/ledger entries, refunds without a matching entry, ledger balance-continuity gaps | Priya | (a), (c) | Use this command to find mismatches between receipts, payments and ledger. Do NOT use it for fee totals; use 'money fees' instead. |
| 3 | Per-listing P&L | `money listing-pnl` | Revenue minus allocated fees per listing via transactions and ledger | Priya | (c) | none |
| 4 | Listing hygiene audit | `listings audit` | Listings with <13 tags, no materials, missing alt text, too few photos, over-long titles; emits a bulk-edit plan file | Maya, Ravi | (a), (b) | Use this command to find hygiene gaps across listings. Do NOT use it for expiry/renewal timing; use 'listings expiring' instead. |
| 5 | Renewal planner | `listings expiring` | Active listings ending within N days, with should_auto_renew state, sorted by ending_timestamp | Maya | (b), (c) | Use this command for expiry/renewal timing. Do NOT use it for tag/photo gaps; use 'listings audit' instead. |
| 6 | Dead-weight listings | `listings dead-weight` | Active listings with zero transactions in N days (LEFT JOIN listings to transactions), with views/favorers | Maya | (c) | none |
| 7 | Inventory audit | `inventory audit` | Out-of-stock variations, missing/duplicate SKUs, price spread across variations | Maya, Ravi | (b), (c) | Use this command for structural gaps (SKUs, price spread, zero-quantity offerings). Do NOT use it for a simple low-stock list; use 'inventory low-stock' instead. |
| 8 | Review digest | `reviews digest` | Reviews since N days: rating histogram, listings with low-rated reviews, per-listing rating trend | Maya | (b), (c) | none |
| 9 | GPSR readiness | `compliance gpsr` | Check listings against economic operators for GPSR coverage | Maya | (b) | none |
| 10 | Shop attention feed | `shop attention` | One JSON feed: unshipped count, expiring count, audit counts, reconcile mismatches, stale-sync flag | Ravi | (a) | none |
| 11 | Repeat buyers | `orders repeat-buyers` | Buyer ids with 2+ orders and their lifetime totals | Maya | (c) | none |
| 12 | Late-ship risk | `orders late-risk` | Unshipped receipts past or near transaction expected_ship_date | Maya | (a) | none |
| 13 | Shipping profile audit | `shipping audit` | Listings on profiles with missing destinations or zero cost | Maya | (b) | none |
| 14 | Near-duplicate listings | `listings overlap` | Tag/title overlap between listings (self-competition) | Maya | (c) | none |
| 15 | Tag suggestions | `listings suggest-tags` | Suggest tags from taxonomy and competitor listings | Maya | (b) | none |
| 16 | Tax-year export | `money export` | Annual sales/fees CSV for taxes | Priya | (a) | none |

Pre-cut kill/keep notes: #15 requires NLP/semantic judgement and an external competitor crawl (LLM dependency, external service) and is cut now. #3 gets a verifiability flag because per-listing fee allocation is a modelling choice.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Fee breakdown | `money fees --since 30d --group-by ledger_type` | 8/10 | Priya | hand-code | Groups synced `ledger_entries` (PaymentAccountLedgerEntry: `amount`, `ledger_type`, `created_timestamp`) by type and period in local SQLite, zero API calls after sync. | Brief Top Workflow 3 (fees by type, P&L); Etsy dashboard/CSV "do these poorly" (brief thesis); `money summary` absorbed only gives totals. Fixture-testable offline; live check needs a shop. | Use this command for fees by type over a period. Do NOT use it for payout/receipt matching; use 'money reconcile' instead. |
| 2 | Ledger reconcile | `money reconcile --since 30d` | 8/10 | Priya | hand-code | Joins local `receipts` (paid) to `payments` and `ledger_entries` (`reference_type`/`reference_id`) and checks running `balance` continuity across ordered entries; reports orphans only, PII-free (ids and amounts). | Brief Workflow 3 (fees, refunds, payouts reconciliation); ledger schema carries `balance`, `reference_id`; no competitor has it (ecosystem catalog: thin request mirrors). Fixture-testable. | Use this command to find mismatches between receipts, payments and ledger. Do NOT use it for fee totals; use 'money fees' instead. |
| 3 | Listing hygiene audit | `listings audit` | 9/10 | Maya, Ravi | hand-code | Scans synced `listings` (tags, materials, title) and `listing_images` (alt text, count) against Etsy limits and emits findings plus an optional bulk-edit plan file consumable by `listings bulk-edit`. | Brief Workflow 4 and Build Priority 3 (audit for missing tags/materials/alt text/photos); HUB does N+1 live calls; MOF alt-text bulk update exists but nothing finds the gaps. Offline fixtures. | Use this command to find hygiene gaps across listings. Do NOT use it for expiry/renewal timing; use 'listings expiring' instead. |
| 4 | Renewal planner | `listings expiring --within 30d` | 7/10 | Maya | hand-code | Filters synced `listings` on `ending_timestamp` within the window and `state=active`, shows `should_auto_renew`, sorted soonest first, from the local store. | Brief Workflow 4 and Build Priority 3 name "expiring/expired renewal planner"; ShopListing has `ending_timestamp` and `should_auto_renew`. Offline fixtures. | Use this command for expiry/renewal timing. Do NOT use it for tag/photo gaps; use 'listings audit' instead. |
| 5 | Dead-weight listings | `listings dead-weight --since 90d` | 6/10 | Maya | hand-code | LEFT JOINs synced active `listings` to `transactions` by `listing_id` over the window and returns those with zero sales, plus `views` and `num_favorers`. | Brief Workflow 1 (query offline without quota) and catalog scale (hundreds to tens of thousands of listings); no API aggregate exists. Offline fixtures. | none |
| 6 | Inventory audit | `inventory audit` | 7/10 | Maya, Ravi | hand-code | Walks synced `listing_inventory` products/offerings for `quantity=0`, empty or duplicate `sku`, and price spread across variations in local SQLite. | Brief Workflow 5 (out-of-stock variations, SKU gaps, price spread) and Build Priority 3; HUB `low-stock` covers only a threshold list. Offline fixtures. | Use this command for structural gaps (SKUs, price spread, zero-quantity offerings). Do NOT use it for a simple low-stock list; use 'inventory low-stock' instead. |
| 7 | Review digest | `reviews digest --since 30d` | 5/10 | Maya | hand-code | Aggregates synced `reviews` (`rating`, `created_timestamp`, listing id) into a histogram, low-rated review list and per-listing average, mechanically (no sentiment analysis). | Brief data layer syncs reviews with `min_created`; reviews are public (works for key-only users); thin demand evidence beyond the brief. Offline fixtures. | none |
| 8 | Shop attention feed | `shop attention --agent` | 6/10 | Ravi | hand-code | Composes counts from the local store (unshipped receipts, expiring listings, audit finding counts, reconcile mismatches, last-sync age via `hintIfStale`) into one compact JSON object; no live calls. | Brief thesis (safe, read-first, quota-free agent surface) and Reachability Risk (no polling, ban risk); competitors expose only live N+1 calls. Offline fixtures. | Use this command for a single cross-domain "what needs attention" summary. Do NOT use it for item-level detail; use 'listings audit', 'listings expiring', 'inventory audit' or 'money reconcile' instead. |

Buildability notes (all `hand-code`, roughly 50-150 LoC plus root.go wiring each): every command declares `// pp:data-source local`, calls `hintIfUnsynced(cmd, db, "<resource>")` then `hintIfStale` before returning rows, and follows drain-first (scan rows into structs, check `rows.Err()`, close, then follow-up queries). None writes to the store inside an open `BeginTx`. All default to PII-minimized output (ids, amounts, SKUs; no buyer names/emails/addresses). All run entirely from SQLite fixtures offline; none needs a shop to test. Live verification of the sync that feeds them (receipts, ledger, payments, transactions, private listing inventory) needs a shop and `ETSY_SHARED_SECRET`; reviews and public listings can be live-checked with the API key only. No survivor polls or makes high-volume live calls. #3 and #8 are siblings that share the planned scope-redirect Long Descriptions; every named sibling (`money reconcile`, `money fees`, `listings expiring`, `listings audit`, `inventory low-stock`, `inventory audit`) survives or is an absorbed command (`inventory low-stock`, row 24 of the absorbed table).

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `money listing-pnl` | Per-listing fee allocation is an arbitrary modelling choice (ledger entries reference receipts, not listings) and there is no cost-of-goods data, so output cannot be verified as correct. | `money fees` |
| `compliance gpsr` | The listing schema has no field linking a listing to an economic operator (only `ShopEconomicOperator` with `economic_operator_id`/`state`), so a per-listing coverage check cannot be computed; the shop-level remainder is covered by the absorbed economic-operators endpoints. | `listings audit` |
| `orders repeat-buyers` | Monthly at best, depends on buyer identity data (PII), and low research backing. | `shop attention` |
| `orders late-risk` | Duplicate of absorbed `orders unshipped` (age-sorted); a sort/filter flag, not a new capability. | `shop attention` |
| `shipping audit` | Monthly or occasional use; a thin filter over shipping-profile endpoints already absorbed as generated endpoints. | `inventory audit` |
| `listings overlap` | Occasional use and the value depends on semantic judgement of what counts as competing listings (LLM-adjacent); tag-set overlap alone is noisy. | `listings audit` |
| `listings suggest-tags` | Requires NLP and an external competitor crawl (LLM dependency plus external service). | `listings audit` |
| `money export` | Thin wrapper: global `--csv` on `money fees` already yields the export; used yearly, not weekly. | `money fees` |
