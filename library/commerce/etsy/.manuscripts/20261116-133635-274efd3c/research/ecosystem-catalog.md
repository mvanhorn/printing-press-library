# Etsy Open API v3 — ecosystem catalog (2026-11-16)

Method: GitHub/npm/PyPI searches via `gh search` and the npm registry; tool definitions read from source (`gh api` contents) for Bl0ck154/etsy-seller-mcp-hub, aserper/etsy-mcp, mofiaboss/etsy-mcp (managers + core client), profplum700/etsy-cli (commands + client), administrativetrick/etsy-mcp-server (build/index.js). Rows marked `(source)` were read from code, not a README. Spec mapping uses `etsy-openapi-v3.json` (paths below omit the `/v3/application` prefix). No live Etsy calls were made. Search 6: `anthropics/claude-plugins-official/external_plugins` has NO Etsy plugin. The `gh search repos` queries for "etsy claude plugin", "etsy skill SKILL.md" and "etsy bulk listing automation" returned zero hits: there is no Etsy Claude Code plugin or Claude skill in the ecosystem. Directory sites (lobehub, mcpmarket, fastmcp.me) were not separately crawled. The npm and GitHub MCP hits cover the same servers they list.

## Tools found

| Name | URL | Type | Lang | Stars | Last activity | v3? | Features cataloged |
|---|---|---|---|---|---|---|---|
| etsy-seller-mcp-hub (+ `etsyctl`, local HTTP gateway) | https://github.com/Bl0ck154/etsy-seller-mcp-hub (npm `etsy-seller-mcp-hub` 0.5.0) | MCP + CLI | JS (Node 22+) | 3 | push 2026-11-09 | yes (sends `keystring:secret`) | 79 MCP tools + 24 CLI commands (source) |
| aserper/etsy-mcp | https://github.com/aserper/etsy-mcp | MCP | TS | 13 | 2026-04-02 | yes (sends `keystring:secret`) | 37 tools (source) |
| mofiaboss/etsy-mcp (+ forks jaysadatay, peesaderd; PyPI `etsy-mcp`) | https://github.com/mofiaboss/etsy-mcp | MCP + auth CLI | Python 3.13 | 2 | 2026-04-15 | yes, but sends bare keystring (see Auth) | ~90 tools (README says 90, a repo description says 104) (source: managers) |
| @profplum700/etsy-v3-api-client + @profplum700/etsy-cli | https://github.com/profplum700/etsy-v3-api-client | SDK + CLI | TS | 1 | push 2026-11-16 (SDK 4.1.0; CLI 2.5.4 2026-03-30) | yes | SDK full client; CLI 12 commands (source) |
| @profplum700/etsy-mcp-server | npm 1.0.0 (2026-09-25) | MCP (read-only) | TS | n/a | 2026-09-25 | yes | not read (README claim: read-only, 10 tools per mofiaboss comparison table) |
| administrativetrick/etsy-mcp-server | https://github.com/administrativetrick/etsy-mcp-server | MCP | JS | 21 | 2025-12-12 | v3 but sends BARE keystring; broken since Feb 9 2026 | 7 tools (source) |
| @cprice70/etsy-cli | npm 0.1.3; github cprice70/etsy-cli | CLI | JS | 0 | 2026-03-21 | yes | `listings list --state ...`, JSON output (README only) |
| etspi (PyPI 1.0.4) | https://github.com/sov2000/etspi-cli | CLI | Python | n/a | n/a | not verified (shop/listing/variant JSON mgmt) | not examined |
| cabewaldrop/etsy-cli ("Peach Mountain") | https://github.com/cabewaldrop/etsy-cli | CLI (Rust) | Rust | 0 | 2026-08-06 | yes | not examined (learning port) |
| BusyBee3333/etsy-mcp-2026-complete (@mcpengine/etsy) | https://github.com/BusyBee3333/etsy-mcp-2026-complete | MCP | TS | 4 | 2026-03-13 | yes | "50+ tool modules" (README only; names `etsy_listings_list` etc.) |
| DColl/etsy-mcp-server | https://github.com/DColl/etsy-mcp-server | MCP | n/a | 2 | 2026-07-01 | yes | not examined (listings/inventory/images/orders) |
| georgejeffers/etsy-mcp-server | https://github.com/georgejeffers/etsy-mcp-server | MCP | n/a | 14 | 2025-05-07 | pre-enforcement | not examined (upload/edit listings) |
| @nasdigitaluk/etsy-mcp, etsy-api (npm), @mrfentmen/etsy-mcp, @pipeworx/mcp-etsy, @iflow-mcp/dynamicendpoints-etsy-mcp | npm | MCP | TS | n/a | 2026-04..11 | yes | not examined; `etsy-api` claims all 103 operations generated from the spec; `@pipeworx/mcp-etsy` is public listing search only (API-key only) |
| etsy-ts 8.0.1, etsy-seller 0.1.0-alpha.2, node-etsy-client 3.0.0 | npm | SDK | TS/JS | n/a | 2026-08..11 | yes | not examined |
| rhysnhall/etsy-php-sdk | https://github.com/rhysnhall/etsy-php-sdk | SDK | PHP | 52 | 2026-03-31 | yes | not examined (OAuth with client_id + shared_secret) |
| codetreats/etsy-kotlin, damhuman/Etsy3Py | github | SDK | Kotlin/Python | 0-5 | 2023-2026 | yes | not examined |
| fredfrom/n8n-nodes-etsy | https://github.com/fredfrom/n8n-nodes-etsy | n8n node | TS | 1 | 2026-03-23 | yes | not examined (shops, listings, orders, inventory, images) |
| martinsgirss/etsy-demand-radar | https://github.com/martinsgirss/etsy-demand-radar | script/workflow | Python | 0 | 2026-09-27 | yes | 8 analytic modules (README) — workflow ideas, listed in the Reports section |
| anassk01/etsy-shop-monitor-pro | https://github.com/anassk01/etsy-shop-monitor-pro | script (Postgres+Redis app) | Python | 2 | 2025-09-26 | yes | monitoring/alerts (README) |
| jeffkimble/etsy-mcp-server, TassoKarkanis/etsy-mcp, kumarsumit2000/Etsy-MCP, sagejtn/etsy_mcp | github | MCP | n/a | 0-1 | stale/empty | n/a | skipped (no description, stale or empty) |
| Etsy "Dev MCP Server" | etsy docs | MCP | n/a | n/a | n/a | docs only | skipped (endpoint schemas, no shop access) |

Skipped as v2-only/dead: miniatureape/etsy-cli (2014), plentysmart/EtsyClient, georgicodes/etsy-client and similar 2014–2018 repos. Magento/Medusa/Laravel sync plugins (`magefast/magento2-EtsySync`, `@jytextiles/medusa-plugin-etsy-sync`) are platform integrations out of scope.

Key observations:
- No tool is a SQLite mirror or offline analytics CLI. All are thin per-endpoint wrappers; the hub adds guard rails and two reports.
- No Claude plugin or skill exists for Etsy (anthropics/claude-plugins-official checked).
- Two of the five MCPs read in source send a bare keystring and are likely broken since Feb 2026. Verified from source: administrativetrick (`"x-api-key": API_KEY`) and mofiaboss (`client.py:195 "x-api-key": self.auth.get_keystring()` returns the keystring only). aserper and the hub send `keystring:secret`.

## Feature catalog

Legend: operationIds are from the spec. `(source)` = read from code. "raw" = covered only by a generic passthrough (hub `etsy_request_*`). Tool names: HUB = etsy-seller-mcp-hub, ASE = aserper, MOF = mofiaboss, PP = profplum700 CLI/SDK, ADM = administrativetrick.

### Auth / session

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| OAuth 2.0 PKCE login with local loopback callback server (random port, opens browser) | ASE `startLocalAuthFlow` (source); MOF `etsy-mcp auth login` on localhost:3456 (source) | `GET https://www.etsy.com/oauth/connect`; `POST /v3/public/oauth/token` | Automates PKCE verifier/challenge + state. Note: Etsy requires an https redirect URI, so loopback `http://localhost` may be rejected (aserper open issue about server.address bug) |
| Two-step headless OAuth (`oauth start` prints URL, `oauth complete --state --code`) with state records on disk, TTL, and expected-shop check | HUB `etsyctl oauth start/complete` (source: oauth.mjs) | token endpoint | Works over SSH/remote and with https redirect URI; verifies the returned user's shop matches expected `shop_id` (calls `GET /users/me` + `GET /users/{id}/shops`) |
| Token storage in a local file, mode 0600, atomic write | MOF `~/.config/etsy-mcp/tokens.json` (source); ASE `~/.etsy-mcp/tokens.json`; PP `~/.etsy-cli`; HUB token-store.mjs | n/a | Local persistence |
| Auto refresh: proactive (60 s before expiry) and reactive on 401 then retry once | HUB token-store.mjs + client.mjs (source); ASE auth-manager (source); PP SDK | `POST /v3/public/oauth/token` grant_type=refresh_token | Transparent; refresh token rotates (store new one) |
| Refresh-token rotation handling documented as a hazard | MOF README (tokens "rotate on every refresh"); PP SDK "Refresh Token Rotation" | token endpoint | Prevents losing the 90-day refresh token |
| Multi-shop aliases + per-shop credential files + default shop | HUB config.example.json / `--shop alias` (source) | n/a | Manage several shops from one config |
| `doctor` / `init` setup wizard / `etsy auth configure` / `auth status` | HUB `etsyctl init`, `doctor` (source); PP `etsy auth configure/status` (source) | n/a (doctor likely pings) | Guided config + health check |
| Scope selection (`--scopes`), validation of scope names | HUB (source: regex `^[a-z][a-z0-9_]*$`; default `shops_r listings_r transactions_r`); PP CLI default `listings_r,shops_r`; ASE requests ALL_SCOPES including dead v2 scopes (billing_r, cart_*, favorites_*, feedback_r, recommend_*) (source) | `https://www.etsy.com/oauth/connect` | HUB's least-privilege default is a design to match; ASE's dead scopes are a bug to avoid |
| Token scope introspection | none (no tool wraps it) | `POST /scopes` tokenScopes | Gap |
| Read-only by default mode (`ETSY_MCP_READ_ONLY=1`), write tools gated | HUB (source: tool `readOnly/destructive/idempotent` annotations) | n/a | Safety for agent use |
| Policy gates / confirmation modes for writes | MOF docs/permissions.md (README) | n/a | Confirm-before-write modes |
| Audit log of every API call (shop, method, path, status, operation) | HUB hub.mjs `audit()` (source) | n/a | Local audit trail |
| Redaction layer that scrubs OAuth tokens from logs/output | MOF `etsy_core/redaction.py` (source file exists) | n/a | Secret hygiene |
| Pre-write snapshot (opt-out `--no-backup`) | HUB: every high-level write snapshots the listing (+ inventory, personalization) first unless `--no-backup`; returns `backup` in result (source) | GETs before PUT/PATCH/DELETE | Undo safety net |
| `doctor`-style ping | HUB `etsy_openapi_ping`; ASE integration ping test | `GET /openapi-ping` (ping) | Verifies key + secret + status |

### Shops

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Get shop by id | HUB `etsy_shop_get`; ASE `get_shop`; PP `etsy shops get`; MOF | `GET /shops/{shop_id}` getShop | none |
| Update shop (title, announcement, sale message, etc.) | HUB `etsy_shop_update`; ASE `update_shop` (source) | `PUT /shops/{shop_id}` updateShop | none |
| Search shops by name | HUB `etsy_shop_search`; ASE `find_shops`; ADM `search_shops`, `get_shop_by_name` (source) | `GET /shops?shop_name=` findShops | none |
| Get shop by owner user id | HUB `etsy_shop_by_owner`; MOF | `GET /users/{user_id}/shops` getShopByOwnerUserId | none |
| List configured shops (aliases) | HUB `etsy_shops` (source) | local config | Multi-shop |
| Shop sections: list/get/create/update/delete | HUB `etsy_sections_list/get/write`; ASE `get_shop_sections` + `manage_shop_sections`; PP `etsy shops sections` (source) | `GET/POST /shops/{id}/sections`, `GET/PUT/DELETE /shops/{id}/sections/{section_id}` getShopSections, createShopSection, getShopSection, updateShopSection, deleteShopSection | ASE folds create/update/delete into one `action` enum tool with required-field validation |
| Listings in a shop section | MOF `list_by_shop_section` (source) | `GET /shops/{id}/shop-sections/listings` getListingsByShopSectionId | none |
| Holiday preferences list/update | HUB `etsy_holiday_preferences_list/update` (source) | `GET /shops/{id}/holiday-preferences`, `PUT .../{holiday_id}` getHolidayPreferences, updateHolidayPreferences | none |
| Production partners list | HUB `etsy_production_partners`; MOF `production_partners_list` (source) | `GET /shops/{id}/production-partners` getShopProductionPartners | none |
| Economic operators (GPSR) create/get/list/update | none found (HUB raw only) | `POST/GET /shops/{id}/economic-operators`, `GET/PATCH .../{id}` createShopEconomicOperator, getShopEconomicOperators, getShopEconomicOperator, updateShopEconomicOperator | Gap; spec-new (GPSR enforced 2027-02-01) |
| Shop overview / health snapshot (active listings, favorites, vacation) | anassk01/etsy-shop-monitor-pro (README) | derived from getShop + listings | Alerts + history in Postgres |

### Listings

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Get one listing | HUB `etsy_listing_get`; ASE `get_listing` (supports `includes`); PP `etsy listings get` | `GET /listings/{listing_id}` getListing | HUB normalizes `listing_type`<->`type` and derives `is_digital` (source: etsy-schema.mjs) |
| List shop's listings by state (active/inactive/draft/expired), limit/offset | HUB `etsy_listings_list` + `etsyctl listing list [STATE]`; ASE `get_listings_by_shop`; PP `etsy listings list --state --limit` (source) | `GET /shops/{id}/listings` getListingsByShop | `--json`, table output (PP) |
| Search active listings (keywords, sort_on, filters) | ASE `search_listings`; ADM `search_listings`, `get_trending_listings` (sort_on=score); MOF `search_active` | `GET /listings/active` findAllListingsActive | none |
| Shop's active listings | MOF `list_by_shop` | `GET /shops/{id}/listings/active` findAllActiveListingsByShop | none |
| Featured listings | MOF `get_featured` | `GET /shops/{id}/listings/featured` getFeaturedListingsByShop | none |
| Batch get listings by ids (with `includes`) | HUB `etsy_listings_batch_get`; MOF `list_by_ids` (source) | `GET /listings/batch` getListingsByListingIds | none |
| Batch inventory / batch shipping for many listings | HUB `etsy_listings_batch_inventory`, `etsy_listings_batch_shipping` (source) | `GET /listings/batch/inventory`, `GET /listings/batch/shipping` getListingsInventoryByListingIds, getListingsShippingByListingIds | none |
| Create draft listing | HUB `etsy_listing_create`; ASE `create_draft_listing`; MOF `create_draft` (source) | `POST /shops/{id}/listings` createDraftListing | none |
| Update listing | HUB `etsy_listing_update` + `etsyctl listing update` + `listing state`; ASE `update_listing`; MOF `update` (source) | `PATCH /shops/{id}/listings/{id}` updateListing | HUB: snapshot-before-write; MOF: `_poll_verify` re-reads after write to confirm; MOF: fetch-merge-put |
| Activate / deactivate listing (state change) | MOF `activate`, `deactivate`; HUB `etsyctl listing state LISTING_ID STATE` (source) | `PATCH /shops/{id}/listings/{id}` state=... | Convenience verbs |
| Delete listing | HUB `etsy_listing_delete`; ASE `delete_listing`; PP `etsy listings delete --yes` (source) | `DELETE /listings/{listing_id}` deleteListing | PP confirm prompt; HUB snapshot first |
| Copy/duplicate a listing | MOF `copy` (source) | client-side: GET listing + POST createDraftListing | Value: client-side composite |
| Bulk create listings from a template | MOF `bulk_create_from_template` (source) | createDraftListing xN | Rate-limit-friendly loop with per-item failure isolation |
| Bulk update listings from a template | MOF `bulk_update_from_template` (source: per-listing fetch-merge-put) | updateListing xN | Per-item failure isolation |
| Listing payload validation (title/tags/personalization limits) before sending | HUB etsy-schema.mjs `validatePersonalizationQuestions` etc. (source) | n/a | Client-side validation |
| Listing templates (model files) | MOF `models/listing_template.py` (source) | n/a | Reusable defaults |
| Listings by shop receipt | MOF `list_by_shop_receipt` (source) | `GET /shops/{id}/receipts/{rid}/listings` getListingsByShopReceipt | none |
| Listings by return policy | MOF `list_by_shop_return_policy` (source) | `GET /shops/{id}/policies/return/{pid}/listings` getListingsByShopReturnPolicy | none |
| Snapshot a listing (listing + inventory + personalization) to disk | HUB `etsyctl snapshot LISTING_ID` (source) | getListing, getListingInventory, getListingPersonalization | Backup/restore |

### Listing sub-resources

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Inventory get | HUB `etsy_inventory_get` + `etsyctl inventory get`; ASE `get_listing_inventory` | `GET /listings/{id}/inventory` getListingInventory | none |
| Inventory set (replace products/offerings) | HUB `etsy_inventory_set`; ASE `update_listing_inventory` (source); MOF `update` | `PUT /listings/{id}/inventory` updateListingInventory | HUB: snapshot first. Spec adds `enable_scale_validation` (Oct 2026) — nobody uses it |
| Get one product / one offering | HUB `etsy_listing_product_get`, `etsy_listing_offering_get`; MOF `get_product`, `get_offering` | `GET /listings/{id}/inventory/products/{product_id}` getListingProduct. Offering path used by HUB is `/listings/{id}/products/{pid}/offerings/{oid}`, NOT in spec | Offering endpoint is non-spec (see coverage check) |
| Update an offering quantity directly | MOF `update_offering_quantity` (source) | composite over PUT inventory | Stock-adjust verb |
| Low-stock report (threshold, paged, read-only, skips deleted/disabled offerings, counts no-inventory listings) | HUB `etsyctl report low-stock` / `etsy_report_low_stock` (source: seller-reports.mjs) | getListingsByShop + getListingInventory per listing | Aggregation over N+1 calls; paging via `next_offset` |
| Images: list/get | HUB `etsy_images_list/get`; ASE `get_listing_images`; MOF | `GET /listings/{id}/images`, `.../{image_id}` getListingImages, getListingImage | none |
| Image upload (path, base64, or by existing id; rank, alt text, overwrite) | HUB `etsy_image_upload`, `etsyctl images upload FILE --rank --alt`; ASE `upload_listing_image`; PP `etsy images upload --rank --overwrite` (source) | `POST /shops/{id}/listings/{id}/images` uploadListingImage | HUB streams multipart from file path with size cap (`maxUploadBytes`); mime sniffing |
| Image delete | HUB; ASE `delete_listing_image`; PP `etsy images delete --yes` (source) | `DELETE /shops/{id}/listings/{id}/images/{image_id}` deleteListingImage | confirm / snapshot |
| Reorder images | MOF `reorder` (source) | re-upload with rank | Client-side composite |
| Bulk update image alt text across an entire shop | MOF `update_alt_text` / README (source) | uploadListingImage per image | Shop-wide bulk op |
| Videos list/get/upload/delete (multi-video since Sept 2026) | HUB `etsy_videos_*`; ASE `upload_listing_video`; MOF (source) | `GET /listings/{id}/videos`, `.../{video_id}`, `POST/DELETE /shops/{id}/listings/{id}/videos[/{video_id}]` getListingVideos, getListingVideo, uploadListingVideo, deleteListingVideo | Upload by bytes or by existing video_id |
| Digital files list/get/upload/delete | HUB `etsy_files_*`; MOF `listing_digital_files` (source) | `GET/POST /shops/{id}/listings/{id}/files`, `GET/DELETE .../{file_id}` getAllListingFiles, uploadListingFile, getListingFile, deleteListingFile | none |
| Personalization get / set / delete | HUB `etsy_personalization_*` + `etsyctl personalization get/set/delete` (source) | `GET /listings/{id}/personalization` getListingPersonalization; `POST/DELETE /shops/{id}/listings/{id}/personalization` updateListingPersonalization, deleteListingPersonalization | HUB validates question schema client-side (1-5 questions; text<=45; dropdown/upload types; max chars 1-1024; add_on_price rules) and snapshots first |
| Properties get/set/delete | HUB `etsy_listing_properties_get`, `etsy_listing_property_write`; MOF `listing_properties` (source) | `GET /shops/{id}/listings/{id}/properties`; `PUT/DELETE .../properties/{property_id}`; `GET /listings/{id}/properties/{property_id}` getListingProperties, updateListingProperty, deleteListingProperty, getListingProperty | none |
| Translations get/create/update | HUB `etsy_listing_translation_get/write`; MOF `listing_translations` (source) | `GET/POST/PUT /shops/{id}/listings/{id}/translations/{language}` getListingTranslation, createListingTranslation, updateListingTranslation | none |
| List a listing's available translations | MOF `list_translations` | loops per language | composite |
| Variation images get/set | HUB `etsy_variation_images_get/set`; MOF `update_variation_images` (source) | `GET/POST /shops/{id}/listings/{id}/variation-images` getListingVariationImages, updateVariationImages | none |
| Transactions for a listing | MOF `transactions_list_by_listing` | `GET /shops/{id}/listings/{id}/transactions` getShopReceiptTransactionsByListing | none |

### Orders / receipts / transactions / tracking

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| List receipts (min/max_created, was_paid, was_shipped, sort) | HUB `etsy_orders_list` + `etsyctl orders list --paid --shipped`; ASE `get_shop_receipts`; PP `etsy receipts list --paid/--unpaid/--shipped/--unshipped --limit` (source) | `GET /shops/{id}/receipts` getShopReceipts | PP flags map to `was_paid/was_shipped` |
| Get one receipt | HUB; ASE `get_receipt`; PP `etsy receipts get` | `GET /shops/{id}/receipts/{rid}` getShopReceipt | none |
| Update receipt (mark shipped/paid) | HUB `etsy_order_update`; ASE `update_receipt` | `PUT /shops/{id}/receipts/{rid}` updateShopReceipt | none |
| Create shipment / add tracking (carrier, tracking code, send_bcc) | HUB `etsy_order_shipment_create`; ASE `create_shipment`; MOF `create_shipment` (source) | `POST /shops/{id}/receipts/{rid}/tracking` createReceiptShipment | none |
| Unshipped-orders report: paid + unshipped, privacy-minimized (omits buyer name/email/address), paged | HUB `etsyctl report unshipped` / `etsy_report_unshipped` (source: seller-reports.mjs `summarizeUnshippedReceipts`) | getShopReceipts was_paid=true was_shipped=false | PII-stripping projection; `next_offset` paging |
| Receipts by buyer | MOF `list_by_buyer` (source) | client-side filter of getShopReceipts | composite |
| Refunds by receipt | MOF `refunds_list_by_receipt` (source: no dedicated refunds endpoint; derived from payments' `refunds` arrays) | `GET /shops/{id}/receipts/{rid}/payments` | Derived view |
| Transactions: by shop / by receipt / one | HUB `etsy_transactions_list`, `etsy_transaction_get`; ASE `get_transactions_by_shop`, `get_transaction`, `get_transactions_by_receipt`; MOF | `GET /shops/{id}/transactions`, `.../transactions/{tid}`, `GET /shops/{id}/receipts/{rid}/transactions` getShopReceiptTransactionsByShop, getShopReceiptTransaction, getShopReceiptTransactionsByReceipt | none |

### Money (payments / ledger)

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Payments by ids (`payment_ids`) | HUB `etsy_payments_get` (source) | `GET /shops/{id}/payments` getPayments | none |
| Payment by receipt | HUB `etsy_payment_by_receipt`; ASE `get_payments` (receipt_id optional) (source) | `GET /shops/{id}/receipts/{rid}/payments` getShopPaymentByReceiptId | none |
| Ledger entries list (min/max_created, limit/offset) | HUB `etsy_ledger_entries`; ASE `get_ledger_entries`; MOF `ledger_entries_list` (source) | `GET /shops/{id}/payment-account/ledger-entries` getShopPaymentAccountLedgerEntries | none |
| Ledger entry get | HUB `etsy_ledger_entry_get`; MOF `ledger_entry_get` | `GET .../ledger-entries/{id}` getShopPaymentAccountLedgerEntry | none |
| Payments for ledger entries (`ledger_entry_ids`) | HUB `etsy_payments_by_ledger_entries`; MOF `ledger_entry_payments_get` (source) | `GET .../ledger-entries/payments` getPaymentAccountLedgerEntryPayments | none |
| Payment account summary | MOF `payment_account_summary` (source) | aggregation over ledger entries | Aggregate view |

### Shipping / processing profiles

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Shipping profiles list/get/create/update/delete | HUB `etsy_shipping_profiles_list`, `_get`, `_write`; ASE `get_shipping_profiles`, `create_shipping_profile`, `update_shipping_profile`; MOF `profiles_*` (source) | `GET/POST /shops/{id}/shipping-profiles`, `GET/PUT/DELETE .../{pid}` getShopShippingProfiles, createShopShippingProfile, getShopShippingProfile, updateShopShippingProfile, deleteShopShippingProfile | none |
| Shipping destinations: list/create/update/delete | HUB `etsy_shipping_profile_write`; ASE `manage_shipping_destinations` (create/delete only); MOF `destinations_*` (source) | `.../destinations` and `.../destinations/{id}` getShopShippingProfileDestinationsByShippingProfile, createShopShippingProfileDestination, updateShopShippingProfileDestination, deleteShopShippingProfileDestination | ASE folds into one action-enum tool |
| Shipping upgrades: list/create/update/delete | HUB `etsy_shipping_profile_write`; MOF `upgrades_*` (source) | `.../upgrades`, `.../upgrades/{upgrade_id}` getShopShippingProfileUpgrades, createShopShippingProfileUpgrade, updateShopShippingProfileUpgrade, deleteShopShippingProfileUpgrade | none |
| Shipping carriers list (by origin country) | HUB `etsy_shipping_carriers`; ASE `get_shipping_carriers`; MOF `carriers_list`, `origin_countries_list` (source) | `GET /shipping-carriers` getShippingCarriers | MOF derives origin-country list |
| Processing profiles (readiness state definitions) list/get/create/update/delete | HUB `etsy_processing_profiles_list/get/write` (source: path `/readiness-state-definitions`); PP SDK tests `client-readiness-state` | `GET/POST /shops/{id}/readiness-state-definitions`, `GET/PUT/DELETE .../{id}` getShopReadinessStateDefinitions, createShopReadinessStateDefinition, getShopReadinessStateDefinition, updateShopReadinessStateDefinition, deleteShopReadinessStateDefinition | none |

### Return policies, sections, holiday prefs, partners, operators

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Return policies list/get | HUB `etsy_return_policies_list/get`; ASE `get_return_policies`; MOF `list_return_policies` | `GET /shops/{id}/policies/return`, `.../{pid}` getShopReturnPolicies, getShopReturnPolicy | none |
| Return policy create/update/delete | HUB `etsy_return_policy_write`; ASE `manage_return_policy` (action enum) (source) | `POST`, `PUT/DELETE .../{pid}` createShopReturnPolicy, updateShopReturnPolicy, deleteShopReturnPolicy | none |
| Consolidate return policies | HUB `etsy_return_policy_write` (consolidate action) (source) | `POST /shops/{id}/policies/return/consolidate` consolidateShopReturnPolicies | none |
| (sections, holiday prefs, production partners, economic operators) | see Shops domain | | |

### Reviews

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Reviews by listing | HUB `etsy_reviews_list`; ASE `get_reviews_by_listing`; MOF | `GET /listings/{id}/reviews` getReviewsByListing | none |
| Reviews by shop | HUB; ASE `get_reviews_by_shop`; ADM `get_shop_reviews` (source) | `GET /shops/{id}/reviews` getReviewsByShop | none |
| Review velocity as sales proxy | martinsgirss/etsy-demand-radar `radar_reviews` (README) | derived from getReviewsByShop | Analytic |

### Taxonomy

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Seller taxonomy nodes + properties | HUB `etsy_taxonomy_get`; MOF `seller_nodes_list`, `seller_node_properties_get` (source) | `GET /seller-taxonomy/nodes`, `.../{id}/properties` getSellerTaxonomyNodes, getPropertiesByTaxonomyId | none |
| Buyer taxonomy nodes + properties | ASE `get_buyer_taxonomy`, `get_taxonomy_properties` (uses BUYER taxonomy, not seller taxonomy, which is wrong for listing creation) (source); MOF | `GET /buyer-taxonomy/nodes`, `.../{id}/properties` getBuyerTaxonomyNodes, getPropertiesByBuyerTaxonomyId | none |
| Taxonomy node search (text search over the tree) | MOF `node_search` (source) | client-side over getSellerTaxonomyNodes | Local search; a SQLite/FTS mirror would do this natively |

### Users / addresses

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Get current user | HUB `etsy_user_me`; ASE `get_me`; MOF | `GET /users/me` getMe | none |
| Get user by id | HUB; ASE `get_user` | `GET /users/{user_id}` getUser | none |
| User addresses list/get | HUB `etsy_user_addresses_list/get` (path `/user/addresses`); ASE `get_user_addresses` (uses `/users/{id}/addresses`, NOT in spec) | `GET /user/addresses`, `.../{id}` getUserAddresses, getUserAddress | ASE path mismatch is a bug to avoid |
| Delete user address | HUB `etsy_user_address_delete`; MOF `addresses_delete` (source) | `DELETE /user/addresses/{id}` — NOT in spec (spec has GET only) | Non-spec endpoint (see coverage check) |

### Reports / analytics / bulk / export / generic

| Feature | Best Source | Endpoint(s) / operationId | Value beyond raw endpoint |
|---|---|---|---|
| Generic request passthrough: read any GET, write any POST/PUT/PATCH/DELETE (JSON/form), multipart any | HUB `etsy_request_read/write/multipart`, `etsyctl request METHOD /application/...` (source) | any | Escape hatch; path normalization, allowed-method list |
| Capabilities / tool discovery (`etsy_capabilities`) | HUB (source: spec snapshot "guidance 2026-07-08") | n/a | Agent self-discovery |
| Rate-limit awareness: token bucket (capacity 10, refill 10/s) + daily counter, warn at 80% / refuse at 95% budget | MOF `rate_limiter.py` (source) | n/a | Protects against app bans (our research notes: modest automation got apps banned) |
| 429 handling: honor Retry-After, exponential backoff with cap | HUB client.mjs (source: max 10 s, `max429Retries`); ASE etsy-client.ts (source: MAX_RETRIES, 500ms*2^n); PP SDK | n/a | none |
| Surface rate-limit headers in result meta | HUB client.mjs `rateMeta(response.headers)` (source) | n/a | `x-limit-*` / `x-remaining-*` surfaced to caller |
| Pagination helper: limit<=100, `has_more`, `showing`, `next_offset` | ASE utils/pagination.ts (source); HUB reports (source) | n/a | Consistent paging envelope |
| Response caching (optional) | PP SDK (README) | n/a | Fewer calls |
| Client-side request throttling | PP SDK (README) | n/a | none |
| Output formats: `--json` and table | PP CLI (source: chalk + table) | n/a | none |
| Confirm prompt on destructive ops (`--yes` to skip) | PP CLI (source) | n/a | none |
| Local HTTP gateway / GPT-adapter / MCP-over-HTTP with OAuth | HUB `server.mjs`, `gpt-adapter.mjs`, `mcp-oauth.mjs` (file list) | n/a | Non-CLI surfaces; outside scope of a CLI |
| Demand/trend research: weekly snapshots, delta, niche crowding, seasonality, concept mining | martinsgirss/etsy-demand-radar (README) | findAllListingsActive snapshots + reviews | Snapshot history is the value; a local store makes it cheap |
| Shop monitoring: low-stock alerts, vacation reminders, stagnant growth alerts, top products, revenue tracking | anassk01/etsy-shop-monitor-pro (README) | derived | Needs history/store |
| Trending listings | ADM `get_trending_listings` (source: search sorted by score) | `GET /listings/active` sort_on=score | none |
| Favorites / carts / buyer feedback | MOF `buyer_manager` (source: `/users/{id}/favorites/listings`, `/users/{id}/carts`, `/users/{id}/feedback`) | NOT in spec (v2-era, dead) | Do not implement |

## Auth patterns observed

- **Header**: the spec says `x-api-key` must be `keystring:shared_secret` (enforced since 2026-02-09 per research notes).
  - Sends `keystring:secret`: aserper (`etsy-client.ts:76`, `` `${apiKey}:${sharedSecret}` ``), the hub (`oauth.mjs`, `token-store.mjs apiKeyHeader()`), profplum700 SDK (client takes `keystring` + `sharedSecret`, CLI refuses to run without `sharedSecret`).
  - Sends BARE keystring: administrativetrick (`"x-api-key": API_KEY`, `build/index.js:17`) and mofiaboss (`client.py:195` uses `auth.get_keystring()`; `shared_secret` exists on the auth object but is not used for the header). Both are very likely returning 403 today. mofiaboss's README only asks for `ETSY_KEYSTRING`.
  - So headline: only two of five MCPs get the header right, and none document the Feb 2026 change. A CLI that gets this right on day one (`ETSY_API_KEY` as `keystring:secret`, or separate `ETSY_KEYSTRING` + `ETSY_SHARED_SECRET`) is already ahead.
- **Authorization**: `Bearer <access_token>` (token already has the `<user_id>.` prefix); all send it for private calls. The hub sends it on every call (even public ones). aserper only when a token exists.
- **Env vars**: aserper `.env.example` and `ServerConfig {apiKey, sharedSecret}`; mofiaboss `ETSY_KEYSTRING`; hub uses a credential JSON file (`credentials.example.json`) + `ETSY_HUB_CONFIG`, `ETSY_MCP_READ_ONLY`, `ETSY_MCP_MAX_BODY_BYTES`; profplum CLI stores `apiKey`, `sharedSecret`, `scopes` in `~/.etsy-cli/` after `etsy auth configure`.
- **Token storage**: JSON files under home dir (`~/.etsy-mcp/tokens.json`, `~/.config/etsy-mcp/tokens.json` mode 0600, `~/.etsy-cli`); hub uses atomic writes + a per-shop credential file; profplum SDK offers memory/localStorage/file/custom storage.
- **Refresh**: proactive 60 s before expiry (hub), reactive on 401 retry-once (ASE, hub), MOF refreshes when `is_expired`. Etsy rotates the refresh token on every refresh; the hub and MOF store the new one (MOF README calls it out explicitly).
- **PKCE**: all implement S256. Redirect handling differs: ASE and MOF run a loopback `http://localhost:<port>/callback` server (conflicts with Etsy's https-only redirect rule; ASE has an open bug on `server.address`), the hub requires an https redirect URI and does a two-step start/complete.
- **Default scopes**: hub `shops_r listings_r transactions_r` (least privilege); PP CLI `listings_r,shops_r`; ASE requests ALL 20 scopes including v2-dead ones (will likely fail or be rejected).
- **Base URL**: ASE and ADM `https://openapi.etsy.com/v3`; hub `https://api.etsy.com/v3`; both hosts are documented as equivalent.
- **Write-safety**: hub snapshots before writes and defaults read-only; MOF has policy gates; PP confirms deletes. Nobody implements a dry-run flag on bulk edits (MOF bulk ops have no `--dry-run`; grep found none).

## Spec coverage check

Based on source reading of the hub (typed tools), ASE, MOF (managers), PP (CLI) and ADM. Treat "no typed coverage" as meaning no tool dedicated to it; the hub's raw `etsy_request_*` can reach any of them.

Spec operationIds with NO typed tool in any examined tool:
- `tokenScopes` (`POST /scopes`) — nobody wraps token-scope introspection.
- Economic operators: `createShopEconomicOperator`, `getShopEconomicOperators`, `getShopEconomicOperator`, `updateShopEconomicOperator` (new, GPSR; enforced 2027-02-01).
- Likely uncovered (not confirmed in source, read the repos if it matters): `getShopShippingProfileDestinationsByShippingProfile` and the shipping-upgrade ops outside MOF, `getListingsByShopSectionId` outside MOF. The not-examined npm MCPs (`etsy-api` claims all 103 operations; `@nasdigitaluk/etsy-mcp`) may cover more.
- Newer spec features nobody exposes: multi-video upload semantics (Sept 2026), `enable_scale_validation` on inventory PUT (Oct 2026), EU ECGT listing fields (Sept 2026), `featured_rank` removal (nobody sets it; ok).

Tool features whose endpoint is NOT in the spec (undocumented or dead):
- `GET /listings/{listing_id}/products/{product_id}/offerings/{offering_id}` — hub `etsy_listing_offering_get` (spec has only `/listings/{id}/inventory/products/{product_id}`; offering is likely a made-up/legacy path or an undocumented endpoint).
- `DELETE /user/addresses/{user_address_id}` — hub `etsy_user_address_delete`, MOF `addresses_delete`; spec defines only GET for that path. The spec lists `address_w` scope but no op uses it, so this is a plausible undocumented endpoint (needs a live probe, not done).
- `GET /users/{user_id}/addresses` — ASE `get_user_addresses` (spec path is `/user/addresses`); likely a bug.
- `/users/{user_id}/favorites/listings` (GET/PUT/DELETE), `/users/{user_id}/carts`, `/users/{user_id}/feedback` — MOF `buyer_manager`; v2-era, absent from the spec.
- `refunds`: MOF derives from payments; there is no refunds endpoint.
- ASE `delete_listing` uses `DELETE /listings/{id}` (in spec). Fine.
- Client-side composites (not endpoints): listing copy, activate/deactivate, image reorder, bulk-from-template, payment account summary, taxonomy node search, receipts-by-buyer, origin-country list, offering quantity update.

Other spec drift: in ASE `get_buyer_taxonomy` is the buyer taxonomy, but `createDraftListing` needs seller taxonomy ids, so its taxonomy tools are misleading.

## Codebase Intelligence

DeepWiki step skipped: source was read directly for 5 repos and the 2-minute time box was better spent on source and spec mapping. No DeepWiki data obtained.
