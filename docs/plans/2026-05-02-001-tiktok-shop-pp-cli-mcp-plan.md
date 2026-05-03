# TikTok Shop Printing Press CLI + MCP Plan

Date: 2026-05-02

## Research Summary

Repo conventions were taken from:

- `library/commerce/shopify`: commerce CLI shape, `manifest.json`, `SKILL.md`, `cmd/*-pp-cli`, `cmd/*-pp-mcp`, `internal/cli`, `internal/client`, `internal/config`, `internal/mcp`, `.printing-press.json`, `spec.yaml`, agent flags, `doctor`, `profile`, `which`, and `agent-context` patterns.
- `library/marketing/klaviyo`: authenticated API config, MCP binary packaging, manifest user config, generated command layout, and explicit rate-limit documentation when known.
- `library/media-and-entertainment/hackernews`: lightweight read-only ergonomics, `tools-manifest.json`, MCP context tool, and no-mutation safety language.

Official TikTok Shop docs were fetched from the Partner Center document API behind `partner.tiktokshop.com/docv2`. No endpoint/auth claim below uses unofficial sources.

## Official Source Table

| Source | URL | Notes | Status |
|---|---|---|---|
| Seller API overview | https://partner.tiktokshop.com/docv2/page/650b1f2ff1fd3102b93c6d3d | Seller API concepts and seller/shop model. | confirmed |
| Authorization overview | https://partner.tiktokshop.com/docv2/page/678e3a3292b0f40314a92d75 | Auth links, auth code, token exchange, refresh token, `x-tts-access-token`, scopes. | confirmed |
| Authorization guide | https://partner.tiktokshop.com/docv2/page/678e3a2dbd083702fd17455c | 202309 seller authorization flow, auth code expiry, token/refresh endpoints. | confirmed |
| Signing algorithm | https://partner.tiktokshop.com/docv2/page/678e3a3d4ddec3030b238faf | HMAC-SHA256, app secret, sorted query params excluding `sign`/`access_token`, path/body canonicalization. | confirmed |
| Get Authorized Shops | https://partner.tiktokshop.com/docv2/page/6507ead7b99d5302be949ba9 | `GET /authorization/202309/shops`; returns shop cipher. | confirmed |
| Get Active Shops | https://partner.tiktokshop.com/docv2/page/650a69e24a0bb702c067291c | `GET /seller/202309/shops`. | confirmed |
| Get Order List | https://partner.tiktokshop.com/docv2/page/650aa8094a0bb702c06df242 | `POST /order/202309/orders/search`; page size 1-100; PII-bearing. | confirmed |
| Get Order Detail | https://partner.tiktokshop.com/docv2/page/650aa8ccc16ffe02b8f167a0 | `GET /order/202309/orders`; `ids` max 50; PII-bearing. | confirmed |
| Search Products | https://partner.tiktokshop.com/docv2/page/6503081a56e2bb0289dd6d7d | `POST /product/202309/products/search`; page size 1-100. | confirmed |
| Get Product | https://partner.tiktokshop.com/docv2/page/6509d85b4a0bb702c057fdda | `GET /product/202309/products/{product_id}`. | confirmed |
| Inventory Search | https://partner.tiktokshop.com/docv2/page/650a9191c16ffe02b8eec161 | `POST /product/202309/inventory/search`; product IDs or SKU IDs. | confirmed |
| Update Inventory | https://partner.tiktokshop.com/docv2/page/6503068fc20ad60284b38858 | `POST /product/202309/products/{product_id}/inventory/update`; confirmed but deferred because stock mutation idempotency is unresolved. | confirmed-deferred |
| Get Warehouse List | https://partner.tiktokshop.com/docv2/page/650aa418defece02be6e66b6 | `GET /logistics/202309/warehouses`. | confirmed |
| Search Package | https://partner.tiktokshop.com/docv2/page/650aa592bace3e02b75db748 | `POST /fulfillment/202309/packages/search`; page size 1-50. | confirmed |
| Get Package Detail | https://partner.tiktokshop.com/docv2/page/650aa39fbace3e02b75d8617 | `GET /fulfillment/202309/packages/{package_id}`; fulfillment PII risk. | confirmed |
| Search Returns | https://partner.tiktokshop.com/docv2/page/69c3070c441217049711fdea | `POST /return_refund/202602/returns/search`; newer/high-risk return/refund area. | deferred-v2 |

## Endpoint Classification

| Area | Endpoint status | Reason |
|---|---|---|
| Auth code exchange | confirmed | Official `GET https://auth.tiktok-shops.com/api/v2/token/get`, `grant_type=authorized_code`. |
| Token refresh | confirmed | Official `GET https://auth.tiktok-shops.com/api/v2/token/refresh`, `grant_type=refresh_token`. |
| Signature generation | confirmed | Official HMAC-SHA256 algorithm using app secret. |
| Shops/account info | confirmed | Authorized and active shop endpoints confirmed. |
| Orders list/get | confirmed | 202309 endpoints confirmed; high PII risk. |
| Products/listings list/get | confirmed | 202309 product search/detail endpoints confirmed. |
| Inventory list/get | confirmed | 202309 inventory search confirmed. |
| Inventory update | confirmed-deferred | Endpoint confirmed, but v1 does not execute stock mutations until idempotency/no-retry policy is productionized. |
| Logistics/fulfillment list/get | confirmed | Warehouse and package endpoints confirmed; fulfillment data may contain PII. |
| Returns/refunds | deferred-v2 | 202602 endpoint found, but high-risk and outside strict v1. |
| Finance/settlements/affiliate | partner-gated or unclear | Out of v1. |
| Webhooks | unclear | Out of v1 until event/auth verification docs are scoped. |
| Rate limits | unclear | No general official numeric limits were confirmed; implementation handles 429 and `Retry-After` without encoding made-up quotas. |

## Required Scaffold Files

- `manifest.json` for MCP metadata and `user_config` env mapping.
- `SKILL.md` for agent-facing usage, source boundaries, auth setup, command reference, and install instructions.
- `cmd/tiktok-shop-pp-cli/main.go` as the Cobra CLI entry point.
- `cmd/tiktok-shop-pp-mcp/main.go` as the MCP stdio entry point.
- `internal/cli/root.go` with persistent flags, `--agent`, command registration, exit codes, `which`, and `agent-context`.
- `internal/cli/doctor.go` for config/env/auth readiness checks.
- `internal/cli/orders_list.go` for PII-conscious order list/get commands.
- `internal/config/config.go` for env/config loading without hardcoded secrets.
- `internal/client/client.go` for signing, auth token calls, raw JSON passthrough, dry-run redaction, and error taxonomy.
- `internal/mcp/tools.go` for MCP tools mirroring the safe read-only CLI surface.
- `spec.yaml` with only confirmed endpoints and explicit deferred mutation tagging.
- `tools-manifest.json` with safe v1 tool metadata.
- `.printing-press.json` provenance metadata.
- `go.mod` and generated `go.sum`.

## Strict MVP Command Matrix

| Command | User intent | API dependency | Risk | Output shape |
|---|---|---|---|---|
| `doctor` | Verify local setup and readiness | Config only; no PII endpoint probe | Low | JSON/object readiness report |
| `auth status` | Check whether credentials/tokens are configured | Config only | Low | JSON/object booleans, no secret values |
| `auth exchange --auth-code` | Exchange auth code for tokens | `GET /api/v2/token/get` on auth host | Medium | Redacted token summary |
| `auth refresh` | Refresh access token | `GET /api/v2/token/refresh` on auth host | Medium | Redacted token summary |
| `shops info` | Get authorized shops and shop ciphers | `GET /authorization/202309/shops` | Medium | Raw TikTok Shop JSON |
| `orders list` | List/search orders | `POST /order/202309/orders/search` | High PII | Raw TikTok Shop JSON |
| `orders get` | Inspect one order | `GET /order/202309/orders` | High PII | Raw TikTok Shop JSON |
| `products list` | Search products/listings | `POST /product/202309/products/search` | Medium | Raw TikTok Shop JSON |
| `products get` | Inspect product/listing | `GET /product/202309/products/{product_id}` | Medium | Raw TikTok Shop JSON |
| `inventory list` | Search inventory by product/SKU IDs | `POST /product/202309/inventory/search` | Medium | Raw TikTok Shop JSON |
| `inventory get` | Inspect one SKU inventory | `POST /product/202309/inventory/search` | Medium | Raw TikTok Shop JSON |
| `fulfillment list` | Search packages | `POST /fulfillment/202309/packages/search` | High PII | Raw TikTok Shop JSON |
| `fulfillment get` | Inspect package detail | `GET /fulfillment/202309/packages/{package_id}` | High PII | Raw TikTok Shop JSON |
| `fulfillment warehouses` | List warehouses | `GET /logistics/202309/warehouses` | Medium | Raw TikTok Shop JSON |
| `inventory update` | Update stock | Confirmed update endpoint | High mutation | Deferred placeholder |

## Deferred To V2

- Inventory mutation execution.
- Returns/refunds reads and mutations.
- Finance, settlements, affiliate APIs.
- Webhook registration, validation, and event subscriptions.
- Product create/update/delete.
- Shipping-label purchase, RTS, carrier mutations.
- Batch jobs, exports, reporting, and local sync database.
- Any partner-gated API package requiring explicit approval beyond basic seller management.

## Architecture And Security Decisions

Config/env vars:

- `TIKTOK_SHOP_CONFIG`: optional config file path.
- `TIKTOK_SHOP_APP_KEY`: required for live signed Open API calls.
- `TIKTOK_SHOP_APP_SECRET`: required for signing; sensitive.
- `TIKTOK_SHOP_ACCESS_TOKEN`: required for live Open API calls; sensitive; sent as `x-tts-access-token`.
- `TIKTOK_SHOP_REFRESH_TOKEN`: required for `auth refresh`; sensitive.
- `TIKTOK_SHOP_SHOP_ID`: optional local identifier.
- `TIKTOK_SHOP_SHOP_CIPHER`: required for shop-scoped order/product/inventory/logistics calls.
- `TIKTOK_SHOP_BASE_URL`: optional override; defaults to `https://open-api.tiktokglobalshop.com`.
- `TIKTOK_SHOP_AUTH_BASE_URL`: optional override; defaults to `https://auth.tiktok-shops.com`.

Token storage:

- Env vars override config for CI/agent use.
- Config file path is `~/.config/tiktok-shop-pp-cli/config.toml` with `0600` permissions on writes.
- Auth exchange/refresh never prints token values.
- Token persistence requires explicit `--save`.

Signing:

- Add `app_key`, `timestamp`, and `sign` query params.
- Sign path + sorted query key/value pairs excluding `sign` and `access_token`.
- Append JSON body for non-multipart requests.
- Wrap with app secret and HMAC-SHA256 using app secret as key.

Retry/backoff/rate limits:

- No invented numeric limits.
- Client surfaces 429 as rate-limit exit code `7` and honors `Retry-After` when present.
- Future read retries should be bounded exponential backoff with jitter for 429/5xx.
- Mutations must not be retried until idempotency semantics are explicit.

Error taxonomy:

- `2`: usage error.
- `4`: authentication/config material missing.
- `5`: upstream API error.
- `7`: rate limited.
- `10`: config error.
- Messages include action hints where possible and never include token/app secret values.

Idempotency:

- Read commands are safe to retry by operators, but implementation does not currently loop retries automatically.
- Mutating commands require a separate design for confirmation, dry-run diff, idempotency key or no-retry policy, and rollback/verification before enabling.

Compliance/risk:

- Treat orders, packages, returns, buyer IDs, addresses, and messages as PII.
- Least privilege: enable only seller scopes needed for the v1 command set.
- No scraping or unofficial endpoints.
- No endpoint implementation based on SDK snippets, blogs, Postman collections, or package README examples.
- Partner-gated and high-risk APIs remain disabled until explicit approval and official docs are available.
