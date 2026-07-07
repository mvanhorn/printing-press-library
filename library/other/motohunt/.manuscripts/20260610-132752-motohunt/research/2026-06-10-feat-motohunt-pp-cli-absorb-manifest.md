# MotoHunt CLI — Absorb Manifest

No existing motohunt/atvhunt CLI, MCP, or SDK exists (no-API site). "Absorbed" = table-stakes
marketplace capabilities a buyer expects from the website UI; "Transcendence" = capabilities the
website UI does NOT give you (offline rank, watch/diff, agent-native price-research access).

## Absorbed (match the website's own search UX, agent-native)

| # | Feature | Source (website UI) | Our Implementation | Added Value |
|---|---------|---------------------|--------------------|-------------|
| 1 | Free-text + location search | motohunt.com search box | `motohunt-pp-cli search --q --location` | JSON out, `--select`, scriptable |
| 2 | Filter by make / style / model / state | path-segment facets | `(behavior in motohunt-pp-cli search) --make --style --model --state` | combinable, validated slugs |
| 3 | Sort (recent / price / best-deal) | `?sort=` dropdown | `(behavior in motohunt-pp-cli search) --sort t\|p\|a\|c` | explicit, agent-readable |
| 4 | Paginate full result set | infinite-ish `?start=` | `(behavior in motohunt-pp-cli search) --limit --start --max-pages` | auto-pages past the 24/page wall |
| 5 | Parsed listing cards | rendered card HTML | `motohunt-pp-cli search` (goquery: title/price/mileage/badges/location/dealer/id) | structured rows, not a web page |
| 6 | Listing detail + specs | `/l/{id}` page | `motohunt-pp-cli get <id>` (goquery: VIN/mileage/color/condition/stock/dealer) | one struct, JSON |
| 7 | Enumerate makes | "Browse by Make" block | `motohunt-pp-cli makes` | drives precise searches without guessing slugs |
| 8 | Enumerate models | `/model-selector` cascade | `motohunt-pp-cli models --make <X>` | same, per make |
| 9 | ATV/UTV marketplace | atvhunt.com (sister site) | `(behavior in motohunt-pp-cli search/get) --site atv` | one CLI, both marketplaces |

## Transcendence (the website UI can't do this)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Price-research surfacing | `get <id>` (priceResearch block) | hand-code | The site buries MSRP/ALP/deal-rating in prose; we expose `base_msrp`, `alp`, `deal_rating` as typed fields an agent can compare | none |
| 2 | Offline deal ranking | `deal --make <X> --location <zip>` | hand-code | Requires syncing many listings into SQLite then ranking by ask-vs-ALP gap — the site only deal-rates one listing at a time | Use this to find the biggest under-ALP deals across a synced search. Not a live single-listing check; use `get` for that. |
| 3 | Saved-search watch + diff | `watch add\|run\|list\|rm` | hand-code | Requires snapshot history in the local store; reports NEW listings and PRICE DROPS between runs — the site has no alerts for anonymous users | none |
| 4 | Local SQLite mirror + SQL/search | `sync`, `search --local`, `sql` | spec-emits + hand-code | Offline full-text search over synced inventory; no website equivalent | none |
| 5 | Dual-marketplace parity | `--site moto\|atv` global flag | hand-code | atvhunt is the same engine; one binary covers motorcycles + ATV/UTV/SxS | none |

## Stubs
None. All rows above are shipping scope (per Richard's "Full" scope choice).

## Notes
- Generated html-extract endpoints (`listings search` links-mode, `listings get` page-mode) are the thin
  baseline; the rich parsing in rows 5/6 and all transcendence rows are hand-written goquery (the kayak
  html-scrape pattern). Selector brittleness is inherent to scrape — `doctor` includes a selector-health probe.
