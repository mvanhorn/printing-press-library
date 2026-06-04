# Printify Crowd-Sniff Report

## npm Packages Analyzed
- Crowd-sniff returned 5 endpoints from community-sdk tier.
- Scanner warning: downloads API returned status 400. Provenance JSON still reports endpoints and wrote a candidate spec.

## GitHub Repos Searched
- Automated scanner output did not include per-repo provenance beyond community-sdk tier.
- Manual Phase 1 research inspected:
  - `TSavo/printify-mcp` — agent/MCP wrapper around Printify product, catalog, upload, shop tooling.
  - `lawrencemq/printipy` — Python wrapper for Printify shops, products, catalog, orders, webhooks.

## Endpoints Discovered
| Method | Path | Source Tier | Source Count | Merge Decision |
|---|---|---:|---:|---|
| POST | `/admin/printify/shops` | community-sdk | 1 | reject: third-party admin route, not Printify public API |
| GET | `/admin/printify/products/{id}` | community-sdk | 1 | reject: third-party admin route, not Printify public API |
| GET | `/admin/printify/orders/{id}/submit` | community-sdk | 1 | reject: third-party admin route, not Printify public API |
| GET | `/admin/printify/products` | community-sdk | 1 | reject: third-party admin route, not Printify public API |
| GET | `/admin/printify/shops` | community-sdk | 1 | reject: third-party admin route, not Printify public API |

## Base URL Resolution
- Official selected base URL remains `https://api.printify.com`.
- Crowd-sniff routes are `/admin/printify/...`, which look like a third-party application proxy/admin API. They do not match Printify's documented `/v1/...` and `/v2/...` public API shape.

## Auth Patterns Detected
- Official OpenAPI: HTTP bearer token, `Authorization: Bearer <token>`.
- Manual source/code research found env var conventions:
  - Official docs examples: `PRINTIFY_API_TOKEN`.
  - `TSavo/printify-mcp`: `PRINTIFY_API_KEY`, optional `PRINTIFY_SHOP_ID`.
  - Crowd candidate spec inferred `PRINTIFY_TOKEN`, but this is not used because endpoints are rejected.

## Parameter Name Evidence
- No useful parameter-name evidence discovered by crowd-sniff.
- Official spec and Help Center remain the source of truth for product creation fields, print areas, placeholder image/text fields, and uploads.

## Coverage Summary
- Total crowd endpoints found: 5.
- Endpoints accepted into generation: 0.
- Rationale: all discovered paths are non-public third-party admin/proxy routes and would make the generated CLI less correct.
- Phase 2 input: use official OpenAPI spec only, with auth/category/narrative enrichment. Do not pass `printify-crowd-spec.yaml` to `generate`.
