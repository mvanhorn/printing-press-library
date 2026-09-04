# Creative Fabrica CLI Absorb Manifest

No competing CLI, MCP server, or SDK wrapper exists for Creative Fabrica
(verified in Phase 1.5a search). The "competitor" is Creative Fabrica's own
web UI. The absorbed surface matches the web UI's catalog capabilities; the
transcendence surface adds what only a local SQLite store + agent-native
output can do. v1 is the anonymous, search-only Algolia catalog surface
(20.5M products); the authenticated personal library is a documented follow-up.

## Absorbed (match the web UI)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Keyword search | CF web search | `creativefabrica-pp-cli find <query>` | offline cache, --json/--csv, scriptable, deterministic |
| 2 | Filter by type | CF facets | `(behavior in creativefabrica-pp-cli find) --type Fonts` | combine every filter in one call |
| 3 | Filter by category | CF facets | `(behavior in creativefabrica-pp-cli find) --category Icons` | 724 categories, scriptable |
| 4 | Browse a designer | CF designer pages | `creativefabrica-pp-cli designer <id\|name>` | full designer catalog, agent-native |
| 5 | Sort results | CF web sort | `(behavior in creativefabrica-pp-cli find) --sort newest` | relevance/newest, deterministic |
| 6 | Free items | CF freebies/free | `creativefabrica-pp-cli free` | isFree filter + newest sort |
| 7 | List categories | CF mega-menu | `creativefabrica-pp-cli categories` | all 724 with counts, offline |
| 8 | List types | CF nav | `creativefabrica-pp-cli types` | type facet with counts |
| 9 | Product detail | CF product page | `creativefabrica-pp-cli product <objectID>` | metadata from Algolia, agent-native |
| 10 | POD/commercial filter | CF POD section | `creativefabrica-pp-cli pod` + `(behavior in creativefabrica-pp-cli find) --pod` | first-class commercial-use filter |
| 11 | On-sale filter | CF discounts | `(behavior in creativefabrica-pp-cli find) --on-sale` | hasPromotions filter |
| 12 | Price filter | CF price facet | `(behavior in creativefabrica-pp-cli find) --max-price` | numeric filtering |
| 13 | Pagination | CF infinite scroll | `(behavior in creativefabrica-pp-cli find) --page --limit` | bounded, scriptable |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|-----------------|
| 1 | File-format filter | find --format svg,dxf,png,eps,pes | hand-code | Format is NOT an Algolia facet; requires local FTS over tags/name/description | Use to narrow craft results to a cut/print file format. Do NOT use `--type` for this; type is the broad product class, not the file format. |
| 2 | Real-discount ranking | deals | hand-code | Requires per-hit local (regularPrice-price)/regularPrice computation + sort; the API only filters on hasPromotions | Use to find the deepest genuine discounts. Differs from `search --on-sale`, which only filters without ranking by discount magnitude. |
| 3 | Designer profile stats | designer-stats | hand-code | Requires local aggregation across a designer's synced catalog (type mix, price band, free/POD counts, newest) | Use for a one-shot profile of a single designer. For the raw list use `designer`; to compare two use `designer-compare`. |
| 4 | Designer comparison | designer-compare | hand-code | Requires a cross-designer SQLite join no single API call provides | Use to compare two creators head-to-head. For a single designer use `designer-stats`. |
| 5 | New-since-snapshot diff | new-since | hand-code | Requires diffing objectIDs against a prior local snapshot; no API "what's new" exists | Use to surface catalog additions since your last sync for a tracked query or designer. Tracks the public catalog, not a personal library. |
| 6 | Subscription-free filter | find --no-subscription | hand-code | outsideSubscription is a hit field but NOT a server facet; requires local post-filter | Use to keep only assets usable without a subscription. Distinct from `--pod` (commercial license) and `free` (zero price). |
| 7 | Tag/facet explorer | tags | hand-code | Requires local tag/category frequency rollup across a result set | Use to discover refinement terms for a query. Counts existing tags/categories; does not classify or invent styles. |

## Deferred (documented follow-up, NOT v1 scope)
- Authenticated personal library / favorites / downloads (cookie-auth GraphQL gateway). User is logged in; cookies captured; can be added via /printing-press-amend.
- Curated daily-gifts (24h commercial-license freebies) — distinct from the 89k permanent free items; needs the daily-gifts page surface.
- Content index (prod_items): blog/tutorials/classes search.
