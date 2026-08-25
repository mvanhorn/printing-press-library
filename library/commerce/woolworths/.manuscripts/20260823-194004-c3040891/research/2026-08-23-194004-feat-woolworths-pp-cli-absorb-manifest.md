# Woolworths (AU) — Absorb Manifest

Binary: `woolworths-pp-cli`. First print. Slug: `woolworths`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Product search | elijah-g/Woolworths-mcp `woolworths_search_products` | (generated endpoint) products search | Offline FTS mirror, `--json`/`--select`, typed exits, no Puppeteer |
| 2 | Product detail by stockcode | elijah-g `woolworths_get_product_details` | (generated endpoint) products detail | 115 fields incl. Nutrition/Variants, cached locally |
| 3 | Batch product fetch | verified live | (generated endpoint) products batch | Comma-separated stockcodes in one call |
| 4 | schema.org JSON-LD product | tjhowse/aus_grocery_price_database | (generated endpoint) products schemaorg | Independent detail path; survives blocks differently |
| 5 | Search result count | documented by elijah-g, implemented by NOBODY | (generated endpoint) search count | ~514 B vs 501 KB full search; free specials-density signal |
| 6 | Search autocomplete | documented by elijah-g, implemented by NOBODY | (generated endpoint) search suggestions | Ranked suggestions + autocorrect |
| 7 | Category tree | elijah-g `woolworths_get_categories` | (generated endpoint) categories tree | 25 departments + specials taxonomy, cached |
| 8 | Category browse + paging | tjhowse, MattTimms/coles_vs_woolies | (generated endpoint) categories browse | camelCase body handled; `TotalRecordCount` paging |
| 9 | Specials listing | elijah-g `woolworths_get_specials` | (behavior in woolworths-pp-cli categories browse) `--special` flag targets `specialsgroup.*` nodes | Per-group targeting across all five specials groups |
| 10 | Store locator | verified live | (generated endpoint) stores list | TradingHours + Facilities + lat/long |
| 11 | Trolley read | elijah-g `woolworths_get_cart` | (generated endpoint) trolley get | Works anonymously via guest cart |
| 12 | Trolley add | elijah-g `woolworths_add_to_cart` | (generated endpoint) trolley add | `--dry-run`, typed exits |
| 13 | Trolley remove | elijah-g `woolworths_remove_from_cart` | (generated endpoint) trolley remove | `--dry-run` |
| 14 | Trolley update quantity | elijah-g `woolworths_update_cart_quantity` | (generated endpoint) trolley update | `--dry-run` |
| 15 | Session/cookie establishment | elijah-g (Puppeteer), tjhowse (GET /shop seeding) | (behavior in woolworths-pp-cli doctor) cookie-warm transport primes `/shop` then replays; auto re-primes on hang | Replaces a whole headless browser with 4 headers + a jar |
| 16 | Unit/cup price surfacing | coles_vs_woolies, Grocermatic | (behavior in woolworths-pp-cli products search) `CupPrice`/`CupMeasure`/`CupString` retained and indexed | Persisted for historical unit-price comparison |
| 17 | Member-price vs multibuy vs standard | MattTimms/coles_vs_woolies ONLY | (behavior in woolworths-pp-cli products search) parse `CentreTag.MemberPriceData` / `MultibuyData` / `ImageTag.FallbackText` | Persisted per observation, not just displayed |
| 18 | Nutrition panel | tazeek/nutrition-scraper (page scrape) | (behavior in woolworths-pp-cli products detail) native `Nutrition` block surfaced | API-native, no HTML scraping |
| 19 | Rate limiting | tjhowse RLHTTPClient.go — only tool with one | (behavior in woolworths-pp-cli doctor) adaptive limiter on every outbound call | Protects against the documented IP-ban-by-breadth failure |
| 20 | Local price history | tjhowse/aus_grocery_price_database (InfluxDB, no agent interface) | (behavior in woolworths-pp-cli sync) `price_observation` table per sync | Agent-queryable; powers all six novel features |
| 21 | Full-catalogue export | wulfftech/Australia_GroceriesScraper | (behavior in woolworths-pp-cli products search) `--csv` on any command | Bounded by tracked set, not a full crawl |
| 22 | MCP packaging | elijah-g (12 tools), hung-ngm (1 WW tool) | (behavior in woolworths-pp-cli mcp) Cobra-tree mirror | Every command is an MCP tool automatically |
| 23 | Saved shopping lists | NOBODY — discovered by our browser-sniff | (generated endpoint) savedlists list | `/api/v3/ui/savedlists`; no existing tool has this |
| 24 | Saved list detail | NOBODY — discovered by our browser-sniff | (generated endpoint) savedlists get | `products[]` + `freeTexts[]` |
| 25 | Past shops / order history | NOBODY — discovered by our browser-sniff | (generated endpoint) pastshops list | `/api/v3/ui/pastshops` |
| 26 | Browser cookie import | elijah-g (Puppeteer session) | (behavior in woolworths-pp-cli auth login) `--chrome` imports the Auth0 session cookies | Auth0+MFA means this is the only honest auth shape |

**Not absorbed, deliberately:** cross-store comparison (Coles/IGA — out of scope, WW only);
price-drop email alerting (needs an external mail service); Everyday Rewards points / boosters /
e-receipts (MFA-gated, ~30-min tokens, 401 on every probe); barcode lookup and mobile shopping-list
CRUD (mobile API key revoked, 401 `invalid_client`).

**Stubs:** none. No row in this manifest ships as a stub.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Genuine-special verdict | `real-special <term\|stockcode>` | hand-code | Joins the live tile (`Price`, `WasPrice`, `IsHalfPrice`, `SavingsAmount`) against local `price_observation` history to emit one decided verdict (GENUINE / RECYCLED / WAS-PRICE-INFLATED / NO-HISTORY). The API has no historical endpoint, so no wrapper can answer it. Score 10/10 | Use this command to judge whether a product's current special is genuine, by comparing today's price against its own recorded price history. Do NOT use this command to ask when the next half-price run will start; use 'cycle' instead. Do NOT use it to find a cheaper alternative product; use 'swap' instead. |
| 2 | Half-price cycle forecast | `cycle <term\|stockcode>` | hand-code | Segments local history into half-price episodes per stockcode; reports median inter-episode gap, run length, days since last run, next-window estimate with episode-count confidence. Requires a timeseries the API never exposes. Score 9/10 | Use this command to forecast when a product's next half-price window is likely to open, from recorded episode history. Do NOT use this command to judge whether today's advertised special is real; use 'real-special' instead. |
| 3 | Cheaper substitute finder | `swap <term\|stockcode>` | hand-code | Normalises `CupPrice` across differing `CupMeasure` bases (per 100 g / 1 kg / 1 L / ea) into one comparable figure, filters to in-stock, annotates each with distance from its own local low. Server `CUPAsc` only orders within a single basis, so it breaks the moment a category mixes bases. Score 9/10 | Use this command to find a cheaper equivalent product ranked by normalised unit price. Do NOT use this command to evaluate a multi-buy offer's effective per-unit price; use 'multibuy' instead. Do NOT use it to judge whether one product's special is genuine; use 'real-special' instead. |
| 4 | Basket costing from a list | `basket <list-file>` | hand-code | Resolves each free-text line to a stockcode by deterministic search-rank + brand/size token overlap, prices it live, and joins each line against local history for a per-line delta and basket-total-versus-last-run. Mirrors the `freeTexts[]` field Woolworths itself stores on saved lists. Score 8/10 | Use this command to price a whole shopping list and see which lines moved since last time. Do NOT use this command to add the resolved items to the cart; use 'trolley add' instead. Do NOT use it to find a cheaper alternative for a single line; use 'swap' instead. |
| 5 | Specials rollover diff | `specials-diff` | hand-code | Set-differences current specials-group membership against the previous local snapshot per `specialsgroup.*` node — entrants, departures, re-entrants, each with days-since-last-seen-in-group. The API returns a flat list with no notion of change. Score 8/10 | Use this command to see what entered or left the specials groups since the last sync. Do NOT use this command to ask about a single named product's special; use 'real-special' instead. |
| 6 | Multi-buy effective unit price | `multibuy <term\|category>` | hand-code | Reads `CentreTag.MultibuyData` alongside `CupPrice`/`CupMeasure` and computes effective per-unit cost *at the offer's required quantity*, versus buying singly and versus the cheapest larger single pack. The realised per-unit figure appears in no response field. Score 8/10 | Use this command to work out whether a multi-buy offer is actually cheaper per unit at the quantity it demands. Do NOT use this command for straight single-price comparisons; use 'swap' instead. |

**Hand-code commitment: 6 of 6 transcendence rows are `hand-code`** (~50-150 LoC each plus
`root.go` wiring). Zero are `spec-emits`. All 26 absorbed rows are generator-emitted or behaviour
within a generated command.
