# Shopify Printed CLI Republish Brief

## API Identity

This CLI wraps the Shopify Admin GraphQL API using the curated internal wrapper spec from `cli-printing-press`:

- API slug: `shopify`
- CLI binary: `shopify-pp-cli`
- MCP binary: `shopify-pp-mcp`
- Category: `commerce`
- API version: `2026-04`

The published library entry intentionally does not include the raw Shopify GraphQL SDL. The raw SDL remains pinned in the generator repo and is ignored for public-library packaging.

## Curated Surface

The generated command surface focuses on high-value commerce read paths:

- `customers`
- `fulfillment-orders`
- `inventory-items`
- `orders`
- `products`

The hand-finished command `bulk-operations` adds:

- `bulk-operations current`
- `bulk-operations run-query`

Resource commands are read-only. `bulk-operations run-query` is intentionally read-mostly, not read-only: it starts a Shopify bulk export job only when explicitly invoked.

## Auth And Endpoint Shape

The wrapper spec models Shopify's per-shop endpoint shape:

- Base URL: `https://{shop}`
- GraphQL path: `/admin/api/{api_version}/graphql.json`
- Required env vars: `SHOPIFY_SHOP`, `SHOPIFY_API_VERSION`, `SHOPIFY_ACCESS_TOKEN`
- Auth header: `X-Shopify-Access-Token`

The published plugin manifest exposes these env vars as user config and marks `SHOPIFY_ACCESS_TOKEN` as sensitive.

## Safety Notes

The branch was republished after removing the raw SDL from the generator PR. This library entry includes the curated wrapper spec only, not the 104k-line raw SDL.

No live Shopify response bodies, store hostname, or access token values are included in this manuscript.
