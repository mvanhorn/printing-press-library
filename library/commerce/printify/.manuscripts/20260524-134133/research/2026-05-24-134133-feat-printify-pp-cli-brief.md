# Printify CLI Brief

## API Identity
- Domain: Print-on-demand commerce automation for shops, catalog selection, product creation, image uploads, publishing, orders, shipping, and webhooks.
- Users: Merchants, POD operators, growth teams, and agents creating or updating many product listings from generated artwork/templates.
- Data profile: Official REST/OpenAPI spec with 40 operations: Shops 2, Catalog 7, Products 10, Orders 7, Uploads 4, Webhooks 5, V2 Catalog Blueprints 5.
- Spec source: https://developers.printify.com/openapi.json
- Docs source: https://developers.printify.com/API-Doc-RREdits.html
- Auth: HTTP bearer token. Canonical env var for this run: `PRINTIFY_API_TOKEN` from the user's `.env`.

## Reachability Risk
- Low for official API reachability: official spec exists, base URL is `https://api.printify.com`, and the user has a PAT in `.env`.
- Medium for personalization completeness: the OpenAPI schema exposes product `print_areas`, placeholder images, text/font fields, and image upload, but the Help Center documents UI-only automated personalization setup with personalizable layers, buyer instructions, and buyer text limits. A Reddit report also shows a merchant could set `sales_channel_properties.personalisation` instructions/limits but still could not enable the personalization checkbox through the public API.
- Wrapper issues: `lawrencemq/printipy` has one open parsing issue around `send_order_to_production`; no strong 403/blocking pattern found. `TSavo/printify-mcp` issues are mostly MCP stdio/logging and missing order tools, not API reachability.

## Top Workflows
1. Catalog-to-product: choose blueprint, provider, variants, placeholders, and prices, then create a product.
2. Asset pipeline: upload artwork by URL or local/base64 content, then place images into print areas with scale, position, angle, and layer metadata.
3. Personalization setup: model reusable text/image personalization templates, including buyer-facing labels, input text, font attributes, instructions, and response limits where the API supports them.
4. Publish lifecycle: validate product readiness, publish to the connected sales channel, watch publish status, unpublish or mark publish success/failure.
5. Shop operations: sync shops, products, uploads, orders, shipping methods, webhooks, and local search/SQL for bulk QA.

## Table Stakes
- List/switch shops and keep a default shop context.
- Browse catalog blueprints, print providers, variants, placeholders, shipping, and V2 economy/express shipping.
- Upload images from URL and local file/base64.
- Create, update, list, retrieve, delete, publish, unpublish, and inspect GPSR product information.
- Submit orders, calculate shipping, send to production, and track order details.
- Manage webhooks and verify webhook signatures.
- Agent-native output: `--json`, `--select`, compact views, dry-run for write commands, typed exit codes.

## Data Layer
- Primary entities: shops, blueprints, print providers, variants, placeholders, uploads, products, product variants, print areas, orders, line items, shipments, webhooks, events.
- Sync cursor: page-based pagination for list endpoints; updated/create timestamps where present; upload/product/order IDs for incremental reconciliation.
- FTS/search: product titles/descriptions/SKUs/tags, blueprint/provider titles, upload filenames, order IDs and shipment statuses.
- Local joins worth keeping: product -> variants -> print areas -> uploaded image IDs -> publish status; blueprint -> provider -> variant options -> shipping profile.

## Codebase Intelligence
- `TSavo/printify-mcp` exposes Printify to agents with shop/product/catalog/image tools, optional Replicate image generation, and env vars `PRINTIFY_API_KEY`, `PRINTIFY_SHOP_ID`, `REPLICATE_API_TOKEN`, `IMGBB_API_KEY`.
- `printipy` is a Python wrapper focused on typed access to shops/products/orders/catalog/webhooks; useful as wrapper evidence, but not a CLI.
- Auth pattern from official spec and tools: bearer token in `Authorization`, merchant-level shop IDs commonly cached as default context.

## User Vision
- Use the Printify API spec with a PAT.
- Main point: create products with image uploads and personalization options.

## Product Thesis
- Name: Printify Studio CLI
- Why it should exist: A single CLI should turn Printify's multi-screen Product Creator workflow into an agent-safe product pipeline: choose a catalog item, upload artwork, compose print areas, apply personalization metadata, create/publish, and audit the resulting listing locally before selling.

## Build Priorities
1. Generate from the official OpenAPI spec with bearer auth enriched to `PRINTIFY_API_TOKEN`.
2. Preserve all endpoint coverage, especially products, uploads, catalog, publish lifecycle, and webhooks.
3. Hand-build product creation helpers around manifest-driven JSON bodies: `product draft`, `product validate`, `product create-from-manifest`, `image upload-file`, and template/personalization helpers if approved.
4. Add local store/search for products, uploads, variants, blueprints, and orders so agents can reuse IDs without re-querying the API.
5. Browser-sniff enrichment decision: offer temporary browser discovery for personalization/Product Creator fields because the official spec may not expose the full UI contract for automated personalization toggles and buyer input configuration.

## Sources
- Official API docs: https://developers.printify.com/API-Doc-RREdits.html
- Official OpenAPI spec: https://developers.printify.com/openapi.json
- Product creation Help Center: https://help.printify.com/hc/en-us/articles/4483644382865-How-can-I-create-a-product-with-Printify
- Personalization Help Center: https://help.printify.com/hc/en-us/articles/29856933892241-How-do-I-set-up-product-personalization
- MCP competitor: https://github.com/TSavo/printify-mcp
- Python wrapper: https://pypi.org/project/printipy/
