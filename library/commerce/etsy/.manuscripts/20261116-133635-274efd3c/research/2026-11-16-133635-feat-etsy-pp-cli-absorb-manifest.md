# etsy-pp-cli — Absorb Manifest

Run 20261116-133635-274efd3c · press v4.35.6 · spec: research/etsy-openapi-v3.json (107 ops) · evidence: research/ecosystem-catalog.md, research/2026-11-16-133635-novel-features-brainstorm.md

## Absorb Manifest

### Absorbed (match or beat everything that exists)
Sources: HUB = Bl0ck154/etsy-seller-mcp-hub, ASE = aserper/etsy-mcp, MOF = mofiaboss/etsy-mcp, PP = profplum700 etsy-cli/SDK, ADM = administrativetrick/etsy-mcp-server. Full evidence: research/ecosystem-catalog.md.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Correct `x-api-key: keystring:shared_secret` header | HUB/ASE (source) | (behavior in etsy-pp-cli doctor) composed from `ETSY_API_KEY` + `ETSY_SHARED_SECRET`, also accepts `ETSY_API_KEY` already in `k:s` form | Two of five MCPs still send a bare keystring and 403; `doctor` names the exact failure |
| 2 | OAuth 2.0 PKCE login (S256) with https redirect, two-step headless start/complete | HUB `oauth start/complete` (source) | etsy-pp-cli auth login | Least-privilege read-only default scopes; `--scopes` opt-in for writes; works over SSH |
| 3 | Token storage 0600 + proactive and on-401 refresh with refresh-token rotation | HUB/ASE/MOF (source) | (behavior in etsy-pp-cli auth status) | Never loses the 90-day refresh token |
| 4 | Token scope introspection | none (gap) | (generated endpoint) application tokenScopes | Shows granted scopes; `doctor` uses it to explain 403-on-scope |
| 5 | Health / ping | HUB `etsy_openapi_ping` | etsy-pp-cli doctor | Distinguishes bad format vs inactive key vs missing scope vs 429 |
| 6 | Read-only default, write gating, confirm on destructive | HUB annotations, PP `--yes` | (behavior in etsy-pp-cli listings update) mutating commands require `--yes`/confirm; agent `_execute` previews until confirm=true | Uniform across every write command |
| 7 | Pre-write snapshot of listing/inventory/personalization | HUB (source) | etsy-pp-cli listings snapshot | Snapshot stored in SQLite with history, not loose files |
| 8 | Rate-limit awareness (headers surfaced, Retry-After, backoff, daily budget guard) | MOF token bucket + HUB rateMeta (source) | (behavior in etsy-pp-cli doctor) client honors `retry-after`, reports `x-remaining-today`, refuses bulk ops near daily budget | Protects against the app bans seen in community reports |
| 9 | Shop get / update / search by name / by owner | HUB/ASE/ADM | (generated endpoint) shops getShop/updateShop/findShops/getShopByOwnerUserId | Offline after sync |
| 10 | Shop sections CRUD + listings in a section | HUB/ASE/MOF | (generated endpoint) shop-sections getShopSections/createShopSection/updateShopSection/deleteShopSection/getListingsByShopSectionId | none beyond agent-native output |
| 11 | Holiday preferences, production partners | HUB/MOF | (generated endpoint) shops getHolidayPreferences/updateHolidayPreferences/getShopProductionPartners | none |
| 12 | Economic operators (GPSR) CRUD | none (gap) | (generated endpoint) economic-operators create/get/list/update | Only tool covering GPSR before 2027-02-01 enforcement |
| 13 | Get listing / batch get by ids (with includes) | HUB/ASE/MOF | (generated endpoint) listings getListing/getListingsByListingIds | Local cache of every fetched listing |
| 14 | List shop listings by state (active/inactive/draft/expired) | HUB/ASE/PP | (generated endpoint) listings getListingsByShop | Synced into SQLite per state |
| 15 | Search active listings / shop active / featured / trending (sort_on=score) | ASE/ADM/MOF | (generated endpoint) listings findAllListingsActive/findAllActiveListingsByShop/getFeaturedListingsByShop | `--select` narrowing of huge listing payloads |
| 16 | Batch inventory / batch shipping for many listings | HUB (source) | (generated endpoint) listings getListingsInventoryByListingIds/getListingsShippingByListingIds | none |
| 17 | Create draft / update / delete listing, activate/deactivate | HUB/ASE/MOF/PP | (generated endpoint) listings createDraftListing/updateListing/deleteListing | Dry-run preview + confirm; snapshot before write |
| 18 | Copy/duplicate listing | MOF `copy` (source) | etsy-pp-cli listings copy | Dry-run shows the draft payload before creating |
| 19 | Bulk create/update from template | MOF (source) | etsy-pp-cli listings bulk-edit | Diff + dry-run + throttled, resumable apply (nobody has dry-run) |
| 20 | Client-side listing validation (title/tags/personalization limits) | HUB etsy-schema.mjs (source) | (behavior in etsy-pp-cli listings bulk-edit) validates against Etsy limits before any write | Catches errors without spending quota |
| 21 | Listings by receipt / by return policy | MOF | (generated endpoint) listings getListingsByShopReceipt/getListingsByShopReturnPolicy | none |
| 22 | Inventory get/set (incl. `enable_scale_validation`), product get | HUB/ASE/MOF | (generated endpoint) listing-inventory getListingInventory/updateListingInventory/getListingProduct | Exposes the Oct 2026 scale-validation flag nobody else does |
| 23 | Offering quantity adjust | MOF (source) | etsy-pp-cli inventory set-qty | Read-modify-write over full inventory PUT with dry-run diff |
| 24 | Low-stock report | HUB `report low-stock` (source) | etsy-pp-cli inventory low-stock | From local store: zero API calls after sync (HUB does N+1 live calls) |
| 25 | Images list/get/upload/delete (rank, alt text) | HUB/ASE/PP | (generated endpoint) listing-images getListingImages/getListingImage/uploadListingImage/deleteListingImage | none |
| 26 | Shop-wide image alt-text bulk update | MOF (source) | (behavior in etsy-pp-cli listings bulk-edit) `--field alt_text` | Dry-run + throttle |
| 27 | Videos list/get/upload/delete (multi-video) | HUB/ASE/MOF | (generated endpoint) listing-videos getListingVideos/getListingVideo/uploadListingVideo/deleteListingVideo | none |
| 28 | Digital files list/get/upload/delete | HUB/MOF | (generated endpoint) listing-files getAllListingFiles/getListingFile/uploadListingFile/deleteListingFile | none |
| 29 | Personalization get/set/delete with schema validation | HUB (source) | (generated endpoint) listing-personalization getListingPersonalization/updateListingPersonalization/deleteListingPersonalization | none |
| 30 | Properties get/set/delete | HUB/MOF | (generated endpoint) listing-properties getListingProperties/getListingProperty/updateListingProperty/deleteListingProperty | none |
| 31 | Translations get/create/update | HUB/MOF | (generated endpoint) listing-translations getListingTranslation/createListingTranslation/updateListingTranslation | none |
| 32 | Variation images get/set | HUB/MOF | (generated endpoint) listing-variation-images getListingVariationImages/updateVariationImages | none |
| 33 | Receipts list (paid/shipped/created filters) / get / update | HUB/ASE/PP | (generated endpoint) receipts getShopReceipts/getShopReceipt/updateShopReceipt | Synced; offline filters |
| 34 | Create shipment / add tracking | HUB/ASE/MOF | (generated endpoint) receipts createReceiptShipment | Dry-run + confirm |
| 35 | Unshipped-orders report, PII-minimized | HUB `report unshipped` (source) | etsy-pp-cli orders unshipped | From local store; age-sorted; PII-minimized by default, `--full` to include buyer details |
| 36 | Receipts by buyer | MOF | (behavior in etsy-pp-cli search) FTS over synced receipts | Offline |
| 37 | Transactions by shop / by receipt / by listing / one | HUB/ASE/MOF | (generated endpoint) transactions getShopReceiptTransactionsByShop/ByReceipt/ByListing/getShopReceiptTransaction | Synced |
| 38 | Payments by ids / by receipt; refunds derived from payments | HUB/ASE/MOF | (generated endpoint) payments getPayments/getShopPaymentByReceiptId | Refund view computed locally |
| 39 | Ledger entries list/get + payments for ledger entries | HUB/ASE/MOF | (generated endpoint) ledger getShopPaymentAccountLedgerEntries/getShopPaymentAccountLedgerEntry/getPaymentAccountLedgerEntryPayments | Synced with created-time windows |
| 40 | Payment account summary | MOF (source) | etsy-pp-cli money summary | Period totals from local ledger, no live calls |
| 41 | Shipping profiles + destinations + upgrades CRUD; carriers | HUB/ASE/MOF | (generated endpoint) shipping-profiles (14 ops) + getShippingCarriers | none |
| 42 | Processing profiles (readiness state definitions) CRUD | HUB (source) | (generated endpoint) processing-profiles (5 ops) | none |
| 43 | Return policies CRUD + consolidate | HUB/ASE | (generated endpoint) return-policies (6 ops) | none |
| 44 | Reviews by listing / by shop | HUB/ASE/ADM | (generated endpoint) reviews getReviewsByListing/getReviewsByShop | Synced + FTS over review text |
| 45 | Seller + buyer taxonomy nodes/properties; node text search | HUB/MOF | (generated endpoint) taxonomy getSellerTaxonomyNodes/getPropertiesByTaxonomyId/getBuyerTaxonomyNodes/getPropertiesByBuyerTaxonomyId | Cached tree; node search via `search --type` |
| 46 | Current user / user by id / user addresses | HUB/ASE | (generated endpoint) users getMe/getUser/getUserAddresses/getUserAddress | Correct `/user/addresses` path (ASE has it wrong) |
| 47 | Generic request passthrough | HUB `etsyctl request` (source) | (behavior in etsy-pp-cli api) generated raw-request escape hatch, if the generator emits one; otherwise covered by full typed surface | none |
| 48 | JSON + table output, pagination envelope | PP/ASE | (behavior in etsy-pp-cli listings) global `--json/--agent/--select/--csv`, auto-paging | CSV export (sellers ask for it, discussion #1766) |
| 49 | Multi-shop config / default shop | HUB | (behavior in etsy-pp-cli sync) `--shop`/default shop_id from config, resolved via getMe + getShopByOwnerUserId | none |

Not absorbed (non-spec/dead): v2 favorites/carts/feedback (MOF), `/users/{id}/addresses` (ASE bug), the `/listings/{id}/products/{pid}/offerings/{oid}` GET (HUB, not in spec), `DELETE /user/addresses/{id}` (not in spec; would need a live probe with address_w, which we can't do without a shop), local HTTP gateway / GPT adapter (not a CLI surface).

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Listing hygiene audit (+ GPSR rule set) | listings audit | hand-code | 9/10 | Scans every synced listing + image against Etsy limits (tags/materials/title/alt text/photo count) and, with `--rules gpsr`, flags listings missing `gpsr_manufacturer_*`/`gpsr_responsible_person_*` or referencing an economic operator not in the synced `economic_operators` table — a cross-entity local join; emits a plan file consumable by `listings bulk-edit` | Use this command to find hygiene and GPSR gaps across listings. Do NOT use it for expiry/renewal timing; use 'listings expiring' instead. |
| 2 | Fee breakdown | money fees | hand-code | 8/10 | Groups synced ledger entries by `ledger_type` and period locally; zero API calls after sync | Use this command for fees by type over a period. Do NOT use it for payout/receipt matching; use 'money reconcile' instead. |
| 3 | Ledger reconcile | money reconcile | hand-code | 8/10 | Joins synced receipts ↔ payments ↔ ledger entries (`reference_type`/`reference_id`) and checks running `balance` continuity; reports orphans PII-free | Use this command to find mismatches between receipts, payments and ledger. Do NOT use it for fee totals; use 'money fees' instead. |
| 4 | Renewal planner | listings expiring | hand-code | 7/10 | Filters synced active listings by `ending_timestamp` window with `should_auto_renew`, soonest first | Use this command for expiry/renewal timing. Do NOT use it for tag/photo/GPSR gaps; use 'listings audit' instead. |
| 5 | Inventory structural audit | inventory audit | hand-code | 7/10 | Walks synced products/offerings for zero quantity, empty/duplicate SKU, price spread across variations | Use this command for structural gaps (SKUs, price spread, zero-quantity offerings). Do NOT use it for a simple low-stock list; use 'inventory low-stock' instead. |
| 6 | Dead-weight listings | listings dead-weight | hand-code | 6/10 | LEFT JOIN synced active listings to transactions over a window → zero-sale listings with `num_favorers` (spec has no views field) | none |
| 7 | Shop attention feed | shop attention | hand-code | 6/10 | One compact JSON from the local store: unshipped count, expiring, audit counts, reconcile mismatches, sync age; no live calls | Use this command for a single cross-domain "what needs attention" summary. Do NOT use it for item-level detail; use 'listings audit', 'listings expiring', 'inventory audit' or 'money reconcile' instead. |
| 8 | Review digest | reviews digest | hand-code | 5/10 | Aggregates synced reviews into a rating histogram, low-rated list and per-listing average (mechanical, no sentiment) | none |

Absorbed rows that are also hand-code (composites over the spec, not novel): listings copy (#18), listings bulk-edit (#19/#20/#26), listings snapshot (#7), inventory set-qty (#23), inventory low-stock (#24), orders unshipped (#35), money summary (#40).

Claim-vs-reality corrections (verified against research/etsy-openapi-v3.json):
- Brainstorm killed `compliance gpsr` claiming "no listing-to-economic-operator field". FALSE: `ShopListing` has `gpsr_manufacturer_id/_type`, `gpsr_responsible_person_id/_type`, `gpsr_safety_warnings`, `gpsr_serial_number` (economic_operator_id references). Reinstated as the `--rules gpsr` rule set of `listings audit` (no new command).
- Brainstorm's `listings dead-weight` cited `views`; `ShopListing` has no `views` property → uses `num_favorers` + transactions only.

Stubs: none.

### Transcendence — market research (user-added at Gate 1.5, 2026-11-16)
Source: user asked to cover the competitive/market-research use cases of a peer project (method only; that project's data, set names and search terms are PRIVATE and must never appear in this CLI's code, tests, fixtures, README or examples — use synthetic shops/keywords). All rows below use API-key-only public endpoints (no shop needed → live-testable once the key is active). All live calls go through the rate-limit client with a per-run call budget (`--max-calls`, refuse near `x-remaining-today`); no polling/scheduling daemon.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 9 | Market snapshot | market snapshot | hand-code | 8/10 | Runs a term list × N pages of `findAllListingsActive` (sort_on=score) and stores a dated snapshot (term, page, position, listing, shop, price, num_favorers, tags, original_creation_timestamp, result `count`) plus the shops seen (`transaction_sold_count`, `listing_active_count`, `create_date`, rating); budgeted and resumable | Use this command to capture a dated market snapshot for a list of search terms. Do NOT use it to compare snapshots; use 'market trend' instead. |
| 10 | Market trend | market trend | hand-code | 8/10 | Diffs two stored snapshots: new/exited listings and shops, position moves, price changes, favorites and review-velocity deltas — the trend question a one-off capture can't answer | Use this command to compare two snapshots over time. Do NOT use it for a single snapshot's demand; use 'market demand' instead. |
| 11 | Demand proxy | market demand | hand-code | 7/10 | Per listing/term: reviews in a window (default 12 months, from `getReviewsByListing` with min_created, dates+ratings only, no review text/buyer ids stored) plus shop `transaction_sold_count`; three-state output (shown/capped/absent; absent never 0) and a "can't tell" floor (`--min-reviews`, default 10) | Use this command for which listings/terms show the most buyer demand. Do NOT use it for where competition is missing; use 'market gaps' instead. |
| 12 | Competitor shops + crowd-out | market shops | hand-code | 7/10 | Joins snapshot listings to shops: sales total, active listing count, age, rating; flags new shops (`--new-shop` default <1 year or <100 sales) and reports the share of top positions held by big vs new shops | none |
| 13 | Gap finder | market gaps | hand-code | 7/10 | Classifies snapshot titles/tags with a user-supplied rule file (`--rules classes.yaml`: target → keyword patterns) and reports targets with no on-target listing as "no listing found in P pages of T terms" (never "no competition"); `--control ids.txt` reports the share of known competitor listings the search reached, and downgrades gap claims when coverage is incomplete | Use this command to find targets with no or little competition. Do NOT use it for demand ranking; use 'market demand' instead. |
| 14 | Tag & title-word contrast | market tags | hand-code | 6/10 | Compares tag and title-word frequency in the top-demand quartile against the rest of the snapshot (tags are in the API, not in web capture) | none |
| 15 | Price bands | market price | hand-code | 6/10 | Price range (min/median/max) by taxonomy or rule-file class, and a Spearman rank correlation of price vs demand per group with a minimum group size (`--min-group`, default 15) | none |

Market-research API coverage (verified against spec): covered — titles, descriptions, tags, price, digital flag, variations (public inventory), taxonomy, original_creation_timestamp (true age, not renewal date), num_favorers, shop transaction_sold_count / listing_active_count / create_date / review_average / review_count, reviews with listing_id+rating+timestamp, search `count`. Partial — keyword position via sort_on=score approximates web search (no ads/personalization/spell-correction). NOT in API — ad/organic flag, "in N carts", badges, autocomplete, search volume, per-listing sales, listing views (peer claimed `views`; spec has none). Statistical extras from the peer method (bootstrap rank bands, shuffle baselines, family-wise correction) are not built in; `--csv` export of snapshots supports doing them externally.

Claim-vs-reality correction #3 (2026-11-16, live probe `GET /listings/active?limit=1`): live ShopListing responses DO include `views` (and `tags`, `original_creation_timestamp`), although the OpenAPI `ShopListing` schema omits `views`. Live API wins over spec: `listings dead-weight` and `market` features MAY use `views` (treat as optional/three-state since it is undocumented). Supersedes correction #2's "no views" note.
