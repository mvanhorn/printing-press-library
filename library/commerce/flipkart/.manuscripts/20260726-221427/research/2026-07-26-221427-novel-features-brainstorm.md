## Customer model

**Priya — a deal-hunter who waits for bank offers to stack with price drops before buying electronics.**

*Today (without this CLI):* Priya keeps 4-5 browser tabs open on Flipkart product pages plus a coupon-aggregator site and a third-party price-tracker site (buyhatke-style). She manually re-checks prices every couple of days in the run-up to a sale, screenshots bank-offer banners because they change without notice, and keeps a mental (or WhatsApp-note) list of "things I'm watching." She cannot answer "across everything I'm tracking, what changed since I last looked?" without opening every tab again.

*Weekly ritual:* Every few days — more often during sale windows — she re-visits her shortlist of 5-10 watched products to see if price dropped or a better card offer appeared.

*Frustration:* There is no single view of "here's what changed across my whole watchlist this week" — she re-derives that by re-opening every product page and comparing against memory.

**Arjun — a shopper who window-shops and shortlists gadgets weekly before a purchase.**

*Today (without this CLI):* When Arjun is in an active buying cycle (phone, headphones, appliance), he opens 3-4 shortlisted product pages side by side, manually copies price/rating/discount/specs into a notes app or spreadsheet so he can eyeball differences. He re-does this most weeks during festive-sale season as new options surface.

*Weekly ritual:* Re-comparing his current shortlist whenever a new candidate shows up or a price changes, redoing the copy-paste each time because nothing persists between sessions.

*Frustration:* No side-by-side view exists — comparison is manual transcription across tabs, redone from scratch every time a single product's price moves.

**Meena — an affiliate marketer running a deals blog, refreshing listings from category and delta feeds weekly.**

*Today (without this CLI):* Meena runs ad hoc affiliate-API scripts to pull a category feed and a delta feed, then eyeballs the raw JSON for products whose discount % jumped meaningfully, manually tracking which `fromVersion` she last used in a notes file.

*Weekly ritual:* Weekly pull of category + delta feed to find newly-discounted products worth writing about for her blog.

*Frustration:* The delta feed returns everything that changed, unranked and unclassified — she manually re-derives "which of these are actually noteworthy" and manually tracks the version cursor by hand.

## Candidates (pre-cut)

| # | Feature | Command | Persona | Source | Kill/keep verdict |
|---|---------|---------|---------|--------|-------------------|
| 1 | Multi-product side-by-side compare | `flipkart compare <url1> <url2> [url3...]` | Arjun | (a) persona | Keep — no LLM, no external service, calls existing product-detail path N times, mechanical diff table |
| 2 | Watchlist digest (what changed across everything I'm tracking) | `flipkart watch digest` | Priya | (a) persona / (c) cross-entity | Keep — local join of watch list + price_history, not a fake API call |
| 3 | Biggest price drops across everything ever fetched | `flipkart history biggest-drops --days N` | Priya | (c) cross-entity | Keep for now, flagged as near-duplicate of #2 |
| 4 | Saved-search diff (re-run a query, show new/removed/changed) | `flipkart search diff "<query>"` | Priya/Meena | (c) cross-entity | Keep — search_results table is explicitly timestamped for this per Data Layer |
| 5 | Category deal scanner | `flipkart deals category <cat> --min-discount N` | Priya/Meena | (b) service pattern | Keep with caution — risk of being a thin filter on the feed endpoint; must persist for reuse to transcend |
| 6 | Feed digest with auto version-cursor + ranked deltas | `flipkart feed digest <category>` | Meena | (a) persona / (c) cross-entity | Keep — real Delta Feed endpoint + local cursor state, distinct from raw feed pull |
| 7 | "Similar / frequently compared" product extraction | `flipkart product similar <url>` | Arjun | (b) service pattern | Flagged low-confidence — no Reachability probe evidence this HTML block is reliably present |
| 8 | Brand-store product pull | `flipkart brand <brand>` | Arjun/Meena | (b) service pattern | Flagged — likely a thin wrapper over search with a brand filter |
| 9 | Best-card arbitrage across a wishlist | `flipkart offers best-card <url1> <url2>...` | Priya | (a) persona | Keep with caution — close to absorbed feature 7's stated added value; must be scoped as aggregation, not comparison |
| 10 | Seller price/rating comparison for one product | `flipkart product sellers <url>` | Arjun | (c) cross-entity | Flagged — thin evidence Flipkart product pages expose multiple comparable sellers |
| 11 | Big Billion Days / sale-event tracker by name | `flipkart deals sale-event <name>` | Priya | (b) service pattern | **Cut now** — scope creep, no stable endpoint, requires hardcoding event names/dates |
| 12 | In-category discount/rating rank for one product | `flipkart product rank <url>` | Meena | (c) cross-entity | Flagged — depends on a fragile precondition (full category snapshot already fetched) |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|--------------|----------|
| 1 | Watchlist digest | `flipkart watch digest` | 10/10 | Priya | hand-code | Joins the local `watch` list against `price_history` to compute a "what changed since last check" summary — no single API call produces this because there is no server-side watchlist. | Brief Top Workflow #4 + Data Layer section names `price_history` explicitly for this purpose |
| 2 | Multi-product compare | `flipkart compare <url1> <url2> [url3...]` | 8/10 | Arjun | hand-code | Fetches N product-detail records (already-absorbed endpoint) and renders a unified diff table across price/rating/discount/specs. | Brief Top Workflow #3 |
| 3 | Feed digest with auto cursor | `flipkart feed digest <category>` | 8/10 | Meena | hand-code | Calls the official Delta Feed API but auto-resolves `fromVersion` from a locally persisted sync-cursor table, then ranks the returned deltas by discount % change. | Brief Top Workflow #6 + Reachability/Auth Intelligence section |
| 4 | Saved-search diff | `flipkart search diff "<query>"` | 7/10 | Priya/Meena | hand-code | Diffs two timestamped snapshots of the same query stored in the local `search_results` table, surfacing new/removed/price-changed products. | Brief Data Layer section |
| 5 | Category deal scanner | `flipkart deals category <cat> --min-discount N` | 7/10 | Priya/Meena | hand-code | Pulls a category snapshot, filters by discount threshold, and persists matches so the scan becomes offline-queryable later. | Brief Build Priorities #3 |
| 6 | Best-card arbitrage | `flipkart offers best-card <url1> <url2>...` | 7/10 | Priya | hand-code | Aggregates the local `offers` table (sum of stacked discount, grouped by card/bank) across a user-supplied product set to find the single card maximizing total savings. | Brief data profile + Priya persona |

### Killed candidates

| Feature | Kill reason | Closest-surviving sibling |
|---|---|---|
| Biggest price drops across everything ever fetched | Redundant transcendence source vs. watchlist digest; weaker signal-to-noise | Watchlist digest |
| "Similar / frequently compared" product extraction | Verifiability failure — no evidence this HTML block is reliably present | Multi-product compare |
| Brand-store product pull | Fails wrapper-vs-leverage test — thin rename of `search --brand=X` | Saved-search diff |
| Seller price/rating comparison for one product | Thin evidence base — no workflow establishes multiple comparable sellers per product | Best-card arbitrage |
| Big Billion Days / sale-event tracker by name | Scope creep — no stable endpoint, requires hardcoding event names/dates | Category deal scanner |
| In-category discount/rating rank for one product | Fragile precondition + thin output | Feed digest with auto cursor |
