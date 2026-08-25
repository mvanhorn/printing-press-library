# Discogs CLI — Absorb Manifest

Sources catalogued: **cswkim/discogs-mcp-server** (115⭐, full-surface MCP — the breadth incumbent; its TOOLS.md is the exhaustive checklist), **jmfontaine/agent-discogs** (token-efficient read-only agent CLI — the ergonomics bar), **rianvdm/discogs-mcp** (derived-intelligence MCP), **JOJ0/discodos** (heavyweight collector/DJ CLI), **python3-discogs-client** (409⭐, de-facto library, has price suggestions), **@lionralfs/discogs-client** (TS, fee calc + price suggestions), **discogs-alert** (wantlist price-drop alerting — adjacent to our flagship).

## Absorbed (match or beat everything that exists)

Every row is emitted by the hand-authored internal spec (`discogs-spec.yaml`, 54 endpoints). Added value across the board: works offline against a local SQLite mirror after `sync`, `--json`/`--select`/`--csv`/`--compact`, typed exit codes, `--dry-run` on every write, an MCP server, and the learn loop (condition-grade aliases: "vg+" → "Very Good Plus (VG+)").

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search database (type/artist/label/genre/style/country/year/format/catno/barcode) | cswkim `search`, agent-discogs `search` | (generated endpoint) database search | Offline FTS after sync; condition/format alias resolution |
| 2 | Get release | cswkim `get_release` | (generated endpoint) database release | `--select` narrows deep payload |
| 3 | Get master | cswkim `get_master_release` | (generated endpoint) database master | offline mirror |
| 4 | Master versions (pressings) | cswkim `get_master_release_versions` | (generated endpoint) database master_versions | country/format/label filters |
| 5 | Get artist | cswkim `get_artist` | (generated endpoint) database artist | |
| 6 | Artist releases | cswkim `get_artist_releases` | (generated endpoint) database artist_releases | sortable |
| 7 | Get label | cswkim `get_label` | (generated endpoint) database label | |
| 8 | Label releases | cswkim `get_label_releases` | (generated endpoint) database label_releases | |
| 9 | Community rating | cswkim `get_release_community_rating` | (generated endpoint) database community_rating | |
| 10 | User rating (get) | cswkim `get_release_rating_by_user` | (generated endpoint) database user_rating | |
| 11 | Set release rating | cswkim `edit_release_rating` | (generated endpoint) database rate | `--dry-run` |
| 12 | Delete release rating | cswkim `delete_release_rating` | (generated endpoint) database unrate | `--dry-run` |
| 13 | Collection folders | cswkim `get_user_collection_folders` | (generated endpoint) collection folders | |
| 14 | Get folder | cswkim | (generated endpoint) collection folder | |
| 15 | Folder items | cswkim `get_user_collection_items` | (generated endpoint) collection items | offline mirror + sort |
| 16 | Add release to folder | cswkim `add_release_to_user_collection_folder` | (generated endpoint) collection add | `--dry-run` |
| 17 | Remove instance | cswkim `delete_release_from_user_collection_folder` | (generated endpoint) collection remove | `--dry-run` |
| 18 | Collection value (min/median/max) | cswkim `get_user_collection_value` | (generated endpoint) collection value | |
| 19 | Custom fields | cswkim `get_user_collection_custom_fields` | (generated endpoint) collection fields | |
| 20 | Find release in collection | cswkim `find_release_in_user_collection` | (generated endpoint) collection find | |
| 21 | Create folder | cswkim `create_user_collection_folder` | (generated endpoint) collection create_folder | `--dry-run` |
| 22 | Rename folder | cswkim `edit_user_collection_folder` | (generated endpoint) collection rename_folder | `--dry-run` |
| 23 | Delete folder | cswkim `delete_user_collection_folder` | (generated endpoint) collection delete_folder | `--dry-run` |
| 24 | Rate collection instance | cswkim `rate_release_in_user_collection` | (generated endpoint) collection rate_instance | `--dry-run` |
| 25 | List wantlist | cswkim `get_user_wantlist` | (generated endpoint) wantlist list | offline mirror |
| 26 | Add to wantlist | cswkim `add_to_wantlist` | (generated endpoint) wantlist add | `--dry-run` |
| 27 | Remove from wantlist | cswkim `delete_item_in_wantlist` | (generated endpoint) wantlist remove | `--dry-run` |
| 28 | Edit wantlist item | cswkim `edit_item_in_wantlist` | (generated endpoint) wantlist edit | `--dry-run` |
| 29 | Seller inventory | cswkim `get_user_inventory` | (generated endpoint) marketplace inventory | offline mirror |
| 30 | Get listing | cswkim `get_marketplace_listing` | (generated endpoint) marketplace listing | |
| 31 | Create listing | cswkim `create_marketplace_listing` | (generated endpoint) marketplace create_listing | `--dry-run`, condition-alias |
| 32 | Update listing | cswkim `update_marketplace_listing` | (generated endpoint) marketplace update_listing | `--dry-run` |
| 33 | Delete listing | cswkim `delete_marketplace_listing` | (generated endpoint) marketplace delete_listing | `--dry-run` |
| 34 | **Price suggestions (per condition)** | python3-discogs-client, lionralfs (**cswkim LACKS**) | (generated endpoint) marketplace price_suggestions | **Beats the leading MCP**; snapshotted to local history |
| 35 | Marketplace stats (lowest price, # for sale) | cswkim `get_marketplace_release_stats` | (generated endpoint) marketplace stats | snapshotted to local history |
| 36 | Fee calc | lionralfs `fee` | (generated endpoint) marketplace fee | |
| 37 | Fee w/ currency | lionralfs | (generated endpoint) marketplace fee_currency | |
| 38 | List orders | cswkim `get_marketplace_orders` | (generated endpoint) marketplace orders | |
| 39 | Get order | cswkim `get_marketplace_order` | (generated endpoint) marketplace order | |
| 40 | Order messages | cswkim `get_marketplace_order_messages` | (generated endpoint) marketplace order_messages | |
| 41 | Update order | cswkim `edit_marketplace_order` | (generated endpoint) marketplace update_order | `--dry-run` |
| 42 | Add order message | cswkim `create_marketplace_order_message` | (generated endpoint) marketplace add_order_message | `--dry-run` |
| 43 | Identity (whoami) | cswkim `get_user_identity`, agent-discogs `status` | (generated endpoint) identity whoami | |
| 44 | Profile | cswkim `get_user_profile` | (generated endpoint) identity profile | |
| 45 | Submissions | cswkim `get_user_submissions` | (generated endpoint) identity submissions | |
| 46 | Contributions | cswkim `get_user_contributions` | (generated endpoint) identity contributions | |
| 47 | User lists | cswkim `get_user_lists` | (generated endpoint) lists user | |
| 48 | Get list | cswkim `get_list` | (generated endpoint) lists get | |
| 49 | Request inventory export | cswkim `inventory_export` | (generated endpoint) inventory export | `--dry-run` |
| 50 | List exports | cswkim `get_inventory_exports` | (generated endpoint) inventory exports | |
| 51 | Get export status | cswkim `get_inventory_export` | (generated endpoint) inventory export_get | |
| 52 | Download export CSV | cswkim `download_inventory_export` | (generated endpoint) inventory export_download | |
| 53 | List inventory uploads | (**beats cswkim** — upload is their TODO) | (generated endpoint) inventory uploads | |
| 54 | Get upload status | (**beats cswkim**) | (generated endpoint) inventory upload_get | |

**Minor endpoints intentionally not modeled in v1** (niche, complex multipart or low-value; not claimed): move-instance-between-folders, edit-custom-field-value, edit-profile, multipart CSV upload-add/change (write side of upload). These are honest non-goals, not stubs.

## Transcendence (only possible with our approach)

All 7 are **hand-code** (top-level Cobra commands wired to root + SQLite joins on the local mirror, ~50–150 LoC each). All score ≥8/10. The moat is `price_snapshots` — the Discogs API keeps no price history, so every price-trend feature only works because the CLI persists snapshots locally.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Wantlist limit-order fills (FLAGSHIP) | fills | hand-code | Joins a local `wantlist_items.max_price` column against latest marketplace `stats` and diffs the prior `price_snapshots` row — no API gives price memory or a fill signal; `discogs-alert` is only an adjacent standalone daemon | Use for wantlist items whose lowest asking has reached the local max_price limit you set (a limit-order fill). Do NOT use it to find market-wide mispriced deals with no preset limit; use 'undervalued' instead. |
| 2 | Portfolio value + history | portfolio | hand-code | API returns one min/median/max value with zero history; summing local `price_snapshots` over time + cost basis is the only way to chart the trend and attribute per-record change | none |
| 3 | Undervalued detection | undervalued | hand-code | Compares live `price_suggestions`/`stats` against each release's trailing median in local `price_snapshots` — the baseline exists only locally | Use to find releases priced below their own trailing/suggested baseline (market mispricing). Do NOT use it for wantlist items hitting a limit you set; use 'fills' instead. |
| 4 | Condition-matched comps | comps | hand-code | Reads `price_suggestions` per condition + local snapshot history for one release; the leading MCP (cswkim) lacks price suggestions entirely | Use for the condition-by-condition price picture of a single release, with local snapshot history. Do NOT use it to compare different pressings of the same album; use 'pressings' instead. |
| 5 | Fee-aware sell router | sell-plan | hand-code | Joins inventory/collection with `fee` + `price_suggestions` + `stats`, ranking by net-after-fee proceeds × a local liquidity signal (num_for_sale + have/want); no single API call ranks this | Use to rank YOUR inventory or collection by net-after-fee proceeds and liquidity. Do NOT use it to price a single release; use 'comps' instead. |
| 6 | Catno/barcode identity spine | identify | hand-code | Resolves a catno/barcode via `database search`, then left-joins the match against local `collection_items` + `wantlist_items` + current `stats` in one answer | Use to resolve a physical record's catalog number or barcode and see whether you own or want it and its current value. Do NOT use it for open-ended text search; use 'search' instead. |
| 7 | Pressing value ranker | pressings | hand-code | Expands a master's versions, joins each to `stats`/`price_suggestions`/snapshot median, ranks by value+liquidity — exploits Discogs' defining master/version structure | Use to rank the versions/pressings of one master by value and liquidity. Do NOT use it for the condition breakdown of one specific release; use 'comps' instead. |

**Killed at the cut (audit trail in `2026-07-11-092456-novel-features-brainstorm.md`):** liquidity (folded into sell-plan), changed (served by fills+portfolio), gaps, reprice, dupes, gems, movers.

## Hand-code commitment
- **7 transcendence commands, all hand-code** (Phase 3 scope): `fills`, `portfolio`, `undervalued`, `comps`, `sell-plan`, `identify`, `pressings`.
- **54 parity endpoints, all generator-emitted** (`spec-emits`).
- New Phase-3 store work beyond generated tables: a `price_snapshots` table + a local `wantlist_items.max_price` column (the moat substrate).

## Publish tiering (public open-source core vs project/dependency layer)

**Tier A — Public open-source core (ships in the printing-press library PR).** Everything above. All 54 parity + all 7 novel are generalizable to any Discogs collector/seller/agent. Design rule that keeps them publishable: the binary is **dependency-free and config-driven** — no reference to eBay, Gixen, or the Railway web app anywhere in it.
- Generalizing design choices baked into Tier A:
  - `portfolio` cost basis reads from a **configurable collection custom-field name** (e.g. "Paid"); default when absent = first-snapshot price. Does NOT hardcode the web app's `cost_basis = market-price-at-sync` convention.
  - All price commands take `--currency`; `max_price` is a user-set local column, no external source.
  - `fills` / `identify` / `comps` are clean standalone features whose `--json`/`--agent` output **is** the integration seam — so they stay 100% public with zero coupling.
- Three of the seven are **dual-use** (universal AND load-bearing for the Record Record loop): `fills` (Discogs-side wantlist feed), `identify` (catno/barcode identity key), `comps` (condition-matched valuation backbone). They ship public; the coupling lives one layer up (Tier B), not in the binary.

**Tier B — Project + dependency integration (NOT in the public CLI; local/external follow-on).** These benefit Jesse's Record Record loop and its dependencies (`ebay-pp-cli`, the Gixen snipe CLI, `discogs-api-backend`). Deliberately kept out of the binary so Tier A stays clean and shippable. Mostly agent-orchestration and config, not new baked-in commands:
- **Gixen feeder** — agent glue that pipes `fills --agent` (limit-order fills) into the Gixen snipe CLI as snipe candidates. The wantlist→sniper bridge. Lives in the orchestration between CLIs.
- **eBay cross-market** — Discogs-vs-eBay fee/comp routing that extends `sell-plan`/`comps` with eBay data. Couples to `ebay-pp-cli`; external.
- **Web-app alignment** — matching `discogs-api-backend`'s cost-basis convention, if desired. A config choice, not code.
- Delivery: documented as follow-on; a thin project-local wrapper or a later `/printing-press-amend` — none of it blocks or enters the public build.

