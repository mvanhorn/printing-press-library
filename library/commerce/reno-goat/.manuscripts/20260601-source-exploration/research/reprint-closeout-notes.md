# Reno Goat reprint closeout notes

Date: 2026-06-04

## Summary

Reno Goat is reprint-ready from the printed-CLI sourceability and model-intel side. The work moved the CLI from the original 11-retailer product-search surface to a 33-active-source renovation-middle CLI with:

- Category-routed fan-out across appliances, plumbing, electrical, HVAC, flooring, hardware, materials, furniture, and decor.
- `model-intel` for installed-selection decisions that need sourceable model/SKU rows, pricing, linked spec/install documents, and bounded model-page probes.
- `source-probe` for measuring WAF, challenge, 403, timeout, readable-route status, and large-body truncation before promoting showroom and big-box sources.
- Reprint-facing metadata and skill docs that describe the current 33 active sources plus 5 tracked stubs.

The implementation deliberately focuses on the squishy middle between commodity builder supply and pure home decor: homeowner-visible selections that commonly need a trade or GC to install.

## Promoted Sources

Active source breadth now includes:

- Appliances: GE Appliances, Bray & Scarff, PC Richard, Appliance Factory, Best Buy, Abt, Homewise Appliance, IKEA, Ferguson.
- Plumbing and bath showroom: Floor & Decor, PlumbersStock, FaucetDepot, FaucetList, PlumbTile, Modern Bathroom, KBAuthority, Vintage Tub, Signature Hardware, QualityBath, Ferguson.
- Electrical and lighting: Super Bright LEDs, PROLIGHTING, 1000Bulbs, Bees Lighting, Lighting New York, Lightology.
- HVAC and comfort: Pioneer Mini Split, Sylvane, IWAe, The Hardware Hut.
- Hardware, flooring, materials, furniture, and decor: The Hardware Hut, Rejuvenation, Floor & Decor, IKEA, West Elm, Article, Shopify DTC, Lighting New York, Lightology, KBAuthority, Vintage Tub.

PC Richard moved out of the deferred appliance bucket through readable Demandware product-tile extraction. Appliance Factory moved out of the adjacent appliance-showroom candidate bucket through AVB/LINQ REST replay over HTTP/1.1. Homewise Appliance moved out through API Gateway/Bloomreach replay exposed by the public Next.js app chunks. Vintage Tub moved out of the deferred bath-showroom bucket through Searchspring JSON extraction. QualityBath moved out through React Query hydration extraction from the corrected `/search/<query>` route.

## Verified Model-Intel Matrix

The final representative matrix was run with search-result offer fallback disabled:

```bash
./reno-goat-pp-cli model-intel "<query>" --sources auto --search-offers=false --json
```

All matrix queries returned priced live source rows with no inspected semantic false positives after fixing `moen shower valve` and `medicine cabinet` filters:

- HVAC/electrical: `thermostat`, `floor register`, `ceiling fan`, `vanity light`, `picture light`.
- Plumbing/bath systems: `linear drain`, `shower valve`, `shower door`, `shower head`, `medicine cabinet`.
- Hardware/bath finish: `cabinet pull`, `door hinge`, `grab bar`, `soap dispenser`.

Evidence:

- `research/final-representative-matrix.md`
- `final-matrix/model-intel-*-response.json`
- `research/model-intel-coverage-audit.md`
- `research/source-viability-matrix.md`

## Fixed Before Reprint

- `moen shower valve`: rejected the prior Floor & Decor tile false positive while preserving valve trim, M-CORE valve-only, transfer-valve trim, rough-in, tub/shower valve, and Vintage Tub shower-set rows.
- `medicine cabinet`: rejected West Elm medicine-cabinet riser/organizer-style accessories while preserving cabinet and mirror-cabinet rows.
- Reprint metadata: refreshed top-level help, `agent-context`, `SKILL.md`, `manifest.json`, `tools-manifest.json`, `.printing-press.json`, product-search help, and patch metadata to describe the current 33-source surface.
- Source-probe diagnostics: widened the response-body window to 256KB and added `body_truncated` so hydrated search payloads and large SSR pages are not misclassified from a small early body slice.
- MCP safety: marked `sources` as `mcp:read-only`; `model-intel` and `source-probe` already expose read-only annotations.

## Deferred Source Gaps

These are documented residual gaps, not active sources and not blockers for reprint. They are the source-expansion work that remains worth cycles; avoid drifting into commodity builder supplies or isolated accessory examples unless they unlock one of these source classes.

- Local showroom and appliance breadth behind WAF, challenge, 403, failed plain-client capture, or missing replayable API extraction: ABW, ADU catalog, AJ Madison, Costco, Spencer's, Grand Appliance, Warners' Stellian. PC Richard, Appliance Factory, Best Buy, Abt, and Homewise Appliance are promoted and should remain in the priority ledger as regression/breadth checks. The final ADU pass found a readable AVB storefront root and public chunks exposing `/api/rest`, `/search/<query>?limit=4&embed=products`, `/catalog/<key>`, and `/products/<url_key>`, but dishwasher product-bearing routes still returned HTTP 403/PerimeterX, so the product gap remains ADU dishwasher/category and exact appliance rows pending GHST-293 transport. The final AJ Madison pass found public static assets exposing Algolia client code plus `/papi/search/*` and `/webservices/api/catadmin/*` contracts, but exact-model, dishwasher category/search, sitemap, and catadmin product/category routes still returned HTTP 403/PerimeterX; the product gap remains AJ Madison dishwasher/category and exact appliance model rows with prices, not an active source. The final Spencer's/Grand Appliance pass tested dishwasher category/search and guessed AVB-style REST routes; all returned HTTP 403, so the product gap is still local-showroom dishwasher/category and exact appliance rows rather than a known parsing task.
- Bath-showroom depth behind timeout/522 behavior, HTTP 403, redirect-to-Ferguson AccessDenied, or unproven product extraction: HomePerfect, DecorPlanet, Build.com. The product gap is shower valves/controls, shower systems, and bathroom vanities from those showroom catalogs. The final HomePerfect pass found reachable Magento Luma/Smile ElasticSuite static assets, but catalogsearch pages remained Cloudflare-challenged and inferred suggest/category/API routes returned 404, so HomePerfect is a GHST-293/browser-resource target rather than an active source. The final DecorPlanet pass found site-wide Cloudflare HTTP 522 under plain replay, including root, sitemap, search/category, GraphQL, API/search, JSON, and static guesses, so there is no parser task until browser/resource capture proves a product surface. The final Build.com pass found that root/category/search routes either return AccessDenied or redirect into Ferguson Home; API/suggest guesses expose a Ferguson Home React/Apollo shell, and the GraphQL endpoint is Ferguson Home GraphQL, which Reno Goat already covers as `ferguson`. Build.com remains a distinct product gap for Build.com-branded shower valves/controls, shower systems, and bathroom vanities rather than a source to promote by aliasing Ferguson. QualityBath, Vintage Tub, and Signature Hardware are promoted and should remain in the priority probe as regression/breadth checks.
- Big-box breadth: Home Depot, Lowe's, Menards, and Ace-style coverage remain incomplete or not cleanly replayable for broad sourceability.
- Door levers/locksets: covered for current model-intel purposes. `interior door lever`, `passage door lever`, and `privacy door lever` now route to hardware and return priced Hardware Hut door-entry, passage, privacy, and lever-set rows with product URLs, prices, and selected spec/install documents.
- Designer Appliances: strong transport candidate because curl-visible Magento catalogsearch/product pages expose useful appliance rows, but not active because the printed Go client still receives Cloudflare challenge HTML.

Priority ledger: `research/priority-source-gaps.md`.

## Follow-Up Issue

Keep GHST-293 open. The remaining source gaps need shared browser-like/impersonating transport, curl/browser parity, or source-specific replayable API extraction. Do not blend that transport work into this reprint.

## Verification

Latest verification:

- `go test ./...`
- `go build -o ./reno-goat-pp-cli ./cmd/reno-goat-pp-cli`
- `go build -o ./reno-goat-pp-mcp ./cmd/reno-goat-pp-mcp`
- Runtime `agent-context --json` check: expanded 33-source description and read-only annotations for `model-intel`, `source-probe`, and `sources`.
- Runtime `sources --json` check: 33 active sources and 5 tracked stubs.

Reprint recommendation: proceed with documented residual gaps and keep GHST-293 as the transport follow-up.
