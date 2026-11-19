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
