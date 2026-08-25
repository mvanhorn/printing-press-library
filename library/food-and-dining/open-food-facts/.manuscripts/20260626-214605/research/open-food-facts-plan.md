# Open Food Facts Research Plan

## Goal

Build a read-only Printing Press CLI for Open Food Facts product lookup, structured search, nutrition summaries, allergen/ingredient inspection, product comparison, category browsing, source posture, and local readiness checks.

## Sources

Open Food Facts documents the API at `https://openfoodfacts.github.io/openfoodfacts-server/api/`. The service describes Open Food Facts as an open food product database that can be reused under open-data terms, with product records contributed voluntarily by users. The documentation warns that data is not guaranteed to be accurate, complete, or reliable, so the CLI must surface source and data-quality caveats in agent output.

The current recommended API line is v3 for new integrations, with v3.6 noted as the current/latest v3 sub-version. Product read operations are available in both v2 and v3; structured search remains available in v2 through `/api/v2/search`; `/api/v3/search` is not implemented. Full-text search is not available in v2/v3 server-side APIs. The legacy `/cgi/search.pl` route supports text search but is not the preferred new-integration path.

Useful documented endpoints and patterns for this first print:

- `GET https://world.openfoodfacts.org/api/v3/product/<barcode>.json`
- `GET https://world.openfoodfacts.org/api/v2/product/<barcode>.json?fields=...`
- `GET https://world.openfoodfacts.org/api/v2/search?...`
- `GET https://world.openfoodfacts.org/api/v2/taxonomy?tagtype=categories&tags=...&fields=...`
- `GET https://world.openfoodfacts.org/api/v3/taxonomy_suggestions?tagtype=categories&string=...`

The API documentation asks clients to always use a custom `User-Agent` in the form `AppName/Version (ContactEmail)` so the app is not mistaken for a bot. Read operations do not require authentication beyond that custom User-Agent. Write operations require authentication, but writes are intentionally out of scope for this CLI.

Documented read-side rate limits include 15 requests per minute per IP address for product read queries and 10 requests per minute per IP address for search queries. The docs ask clients needing more than a few hundred products to download CSV or JSONL exports instead of using the live API as a bulk backend.

## Command Surface

The first print keeps the surface intentionally small and agent-native:

- `product <barcode>` fetches one product by barcode and returns compact product identity, brands, categories, labels, countries, quantity/serving size, image/source URLs, Nutri-Score/NOVA/Eco-Score fields when present, data-quality tags, and caveats.
- `search` performs bounded structured search through `/api/v2/search` using filters such as `--category`, `--brand`, `--country`, `--label`, `--nutrition-grade`, `--page-size`, and `--page`, returning compact product candidates. Plain free-text search should be documented as limited because v2/v3 do not offer recommended server-side full-text search.
- `nutrition <barcode>` fetches one product and returns nutriments, serving basis, Nutri-Score/NOVA/Eco-Score fields, energy/fat/sugar/salt summaries, and caveats about voluntary data.
- `allergens <barcode>` fetches one product and returns allergens, traces, ingredients text, ingredient analysis tags, additives, vegan/vegetarian status tags when available, and source caveats.
- `compare <barcode1> <barcode2> [...barcodeN]` fetches a small bounded set of product records and compares name, brand, categories, Nutri-Score/NOVA/Eco-Score, key nutriments, allergens/traces, additives, data-quality tags, and source URLs.
- `category <category>` uses taxonomy suggestions or partial taxonomy lookup to explain a category and optionally fetch a small product sample through structured search.
- `sources` reports API deployments, endpoint coverage, auth mode, custom User-Agent expectation, documented rate limits, freshness/data-quality caveats, licenses, and non-goals.
- `doctor` reports local configuration: active base URL, `OPEN_FOOD_FACTS_USER_AGENT`, optional `OPEN_FOOD_FACTS_CONTACT_EMAIL`, request timeout, read-only mode, and fair-use posture.

## Live Research Findings

- Open Food Facts v3 is current and recommended for new integrations, but structured product search is still v2-only.
- Product lookup is keyless for read operations but should include a custom User-Agent.
- Search is rate-limited more tightly than product lookup; no command should make unbounded request loops.
- API docs explicitly warn that product data is user-contributed and not guaranteed accurate/complete/reliable.
- Bulk use should point to CSV/JSONL exports instead of live API fanout.
- Write-side product edits, image uploads, and account sessions are documented but out of scope.

## Authentication

No API key or login is required for the read-only command set.

Optional environment variables:

- `OPEN_FOOD_FACTS_USER_AGENT`: explicit `User-Agent` header value. Recommended format: `AppName/Version (ContactEmail)`.
- `OPEN_FOOD_FACTS_CONTACT_EMAIL`: contact email used when constructing the default identifiable User-Agent.
- `OPEN_FOOD_FACTS_BASE_URL`: optional override for testing or staging, defaulting to `https://world.openfoodfacts.org`.

If no custom identity is configured, the CLI must still send a clear generated User-Agent that identifies the CLI and points to the package, and `doctor` must explain how to configure a contact-bearing identity for regular use.

## Output Contract

All commands are read-only. `--agent` and `--json` output must be compact JSON with recurring fields where applicable:

- `source`
- `configured`
- `barcode`
- `query`
- `request`
- `product`
- `results`
- `nutrition`
- `ingredients`
- `allergens`
- `comparison`
- `category`
- `freshness`
- `data_quality`
- `caveats`
- `source_url`

Output should favor stable product facts, barcode/source URLs, and explicit caveats over prose. Commands must avoid unbounded fanout. Comparison should cap the number of barcodes and return per-product fetch errors without dropping successful products.

## Non-Goals

- No product edits.
- No image uploads.
- No account login/session workflows.
- No write-side contribution flows.
- No search-as-you-type loops.
- No bulk harvesting or local database sync in the first print.
- No medical advice or dietary recommendation claims.
- No claim that Open Food Facts data is complete, accurate, or authoritative.
- No raw endpoint dump.
