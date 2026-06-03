# Perfumenz Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Browse all perfumes | Website collection page + /products.json | perfumenz list [--brand] [--gender] [--max-price] --json --select title,price,vendor | Works offline after sync, composable filters, SQL on notes too |
| 2 | View perfume details + notes | Product page (body_html with explicit Top/Heart/Base) | perfumenz get <handle> --json | Parsed note fields in the store + raw description; agent can query "has lemon in top" |
| 3 | Search by text/brand | Site search + vendor filter | perfumenz search "vanilla" --brand Rayhaan | Full FTS across title+notes+desc + structured note filters |
| 4 | Filter by price / stock / gender | Faceted UI on site | perfumenz list --max-price 100 --in-stock --gender unisex | Exact, fast, offline, and can be combined with note predicates |
| 5 | Brand list + stats | Implied by vendor in products | perfumenz brands --json | Count per brand, price range, note diversity — website doesn't aggregate this |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Note profile search & overlap | search --notes "lemon,cedar" --exclude "patchouli" | hand-code | Requires local parsed note sets + set operations across the whole catalog | Use for "perfumes that match my favorite notes without the ones I hate". Do NOT use for simple brand search; use 'search --brand'. |
| 2 | Similarity to another perfume | similar <handle> | hand-code | Local Jaccard or overlap on the three note lists + brand signals | "Find things like Wolf by Rayhaan but cheaper or with more citrus". Website has no "more like this" beyond crude tags. |
| 3 | Price per ml + value ranking | value --sort ppm | hand-code (after sync) | Needs price + size (grams or variant title) normalized across items | "Best value right now in woody scents". Pure website can't compute this dynamically for arbitrary filters. |
| 4 | What notes are trending in current stock | stats notes --limit 10 | hand-code | Aggregate parsed notes over the live synced set | Shows rising accords in the authentic NZ catalog — data only exists together locally. |
| 5 | Build a discovery set (e.g. 5 perfumes covering these notes under budget) | recommend --notes "vanilla,oud,citrus" --budget 400 --count 5 | hand-code | Knapsack-like over local store with note coverage + price | The killer workflow for enthusiasts and agents. No web UI offers this. |

**Hand-code commitment:** ~5 transcendence features will require hand-written Go after generate (store queries + simple set math + nice output). The generator will give us the typed resource commands + sync skeleton + FTS for free.

## Source Priority
- Primary: the Perfumenz Shopify site itself (public JSON + HTML descriptions with explicit notes). No official public "API" beyond the store feeds.
- Economics: completely free, no key required.
- No inversion risk.

## User Vision (from interaction)
User invoked /printing-press on the site URL and, when asked to choose, said "Other" + "what do you recommend?". We recommended (and they are proceeding with) the website-itself path because this is a retail fragrance catalog whose value is in the products + note data, not a backend REST API.

## Build Priorities (summary for Phase 3)
- P0: store + sync from the captured public JSON shape (parse the Fragrance Notes: Top/Heart/Base spans in body_html).
- P1: list, get, search, brands (absorbed).
- P2: the 5 transcendence rows above (the real reason to have the CLI).
- Polish: excellent --help examples with real handles and note queries, doctor that pings the .json, --agent friendly output.
