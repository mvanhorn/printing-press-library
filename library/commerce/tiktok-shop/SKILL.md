---
name: pp-tiktok-shop
description: "Printing Press CLI/MCP for confirmed TikTok Shop Seller APIs. Safe v1 supports auth readiness, token exchange/refresh, read-only shops/orders/products/inventory/package/warehouse commands, and defers risky mutations."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["tiktok-shop-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/cmd/tiktok-shop-pp-cli@latest","bins":["tiktok-shop-pp-cli"],"label":"Install via go install"}]}}'
---

# TikTok Shop - Printing Press Safe v1

Use this CLI for confirmed TikTok Shop Seller API reads and auth readiness. Safe v1 is intentionally conservative: inventory updates, returns/refunds, shipping label mutations, product create/update/delete, finance, and webhooks are deferred.

## Official Source Boundaries

Only use official TikTok Shop Partner Center docs for API, auth, signing, scope, endpoint, and rate-limit claims:

- Seller API overview: https://partner.tiktokshop.com/docv2/page/650b1f2ff1fd3102b93c6d3d
- Authorization overview: https://partner.tiktokshop.com/docv2/page/678e3a3292b0f40314a92d75
- Authorization guide: https://partner.tiktokshop.com/docv2/page/678e3a2dbd083702fd17455c
- Signing algorithm: https://partner.tiktokshop.com/docv2/page/678e3a3d4ddec3030b238faf
- Get Authorized Shops: https://partner.tiktokshop.com/docv2/page/6507ead7b99d5302be949ba9
- Get Active Shops: https://partner.tiktokshop.com/docv2/page/650a69e24a0bb702c067291c
- Get Order List: https://partner.tiktokshop.com/docv2/page/650aa8094a0bb702c06df242
- Get Order Detail: https://partner.tiktokshop.com/docv2/page/650aa8ccc16ffe02b8f167a0
- Search Products: https://partner.tiktokshop.com/docv2/page/6503081a56e2bb0289dd6d7d
- Get Product: https://partner.tiktokshop.com/docv2/page/6509d85b4a0bb702c057fdda
- Inventory Search: https://partner.tiktokshop.com/docv2/page/650a9191c16ffe02b8eec161
- Update Inventory, deferred: https://partner.tiktokshop.com/docv2/page/6503068fc20ad60284b38858
- Get Warehouse List: https://partner.tiktokshop.com/docv2/page/650aa418defece02be6e66b6
- Search Package: https://partner.tiktokshop.com/docv2/page/650aa592bace3e02b75db748
- Get Package Detail: https://partner.tiktokshop.com/docv2/page/650aa39fbace3e02b75d8617

Rate limits remain unclear in accessible official docs and are not encoded as numeric claims.

## Command Reference

Implemented:

- `tiktok-shop-pp-cli doctor` - Check config, env, auth readiness, and official-doc basis without sending a live probe.
- `tiktok-shop-pp-cli auth status` - Show whether app credentials, tokens, and shop selector are configured without revealing secrets.
- `tiktok-shop-pp-cli auth exchange --auth-code <code>` - Exchange a Partner Center authorization code for tokens; output redacts tokens.
- `tiktok-shop-pp-cli auth refresh` - Refresh an access token; output redacts tokens.
- `tiktok-shop-pp-cli shops info` - List shops authorized for this app/token and retrieve shop cipher.
- `tiktok-shop-pp-cli orders list` - Search orders; response contains buyer/order PII.
- `tiktok-shop-pp-cli orders get <order-id>` - Get order detail; response contains buyer/order PII.
- `tiktok-shop-pp-cli products list` - Search products/listings.
- `tiktok-shop-pp-cli products get <product-id>` - Get product detail.
- `tiktok-shop-pp-cli inventory list --product-id <id>` or `--sku-id <id>` - Search inventory.
- `tiktok-shop-pp-cli inventory get <sku-id>` - Get inventory for one SKU.
- `tiktok-shop-pp-cli fulfillment list` - Search fulfillment packages; response may contain fulfillment PII.
- `tiktok-shop-pp-cli fulfillment get <package-id>` - Get package detail.
- `tiktok-shop-pp-cli fulfillment warehouses` - List seller warehouses.
- `tiktok-shop-pp-cli which [query]` - Resolve capabilities to commands.
- `tiktok-shop-pp-cli agent-context` - Emit JSON context for agents.

Deferred:

- `tiktok-shop-pp-cli inventory update` - Official endpoint is confirmed, but safe v1 defers execution until idempotency and no-retry mutation behavior are designed.

## Auth Setup

Set values obtained through official TikTok Shop Partner Center flows. Never hardcode secrets.

```bash
export TIKTOK_SHOP_APP_KEY="<from Partner Center>"
export TIKTOK_SHOP_APP_SECRET="<from Partner Center>"
export TIKTOK_SHOP_ACCESS_TOKEN="<from official token exchange>"
export TIKTOK_SHOP_REFRESH_TOKEN="<from official token exchange>"
export TIKTOK_SHOP_SHOP_CIPHER="<from shops info>"
```

Optional overrides:

```bash
export TIKTOK_SHOP_CONFIG="$HOME/.config/tiktok-shop-pp-cli/config.toml"
export TIKTOK_SHOP_BASE_URL="https://open-api.tiktokglobalshop.com"
export TIKTOK_SHOP_AUTH_BASE_URL="https://auth.tiktok-shops.com"
```

Run:

```bash
tiktok-shop-pp-cli doctor --json
```

Token exchange/refresh do not print token values. Add `--save` only when you intentionally want the returned token bundle persisted to `~/.config/tiktok-shop-pp-cli/config.toml` with `0600` permissions.

## Agent Mode

Add `--agent` to any command. It expands to `--json --compact --no-input --no-color --yes`.

Agent-safe examples:

```bash
tiktok-shop-pp-cli doctor --agent
tiktok-shop-pp-cli shops info --agent --dry-run
tiktok-shop-pp-cli which --agent
tiktok-shop-pp-cli inventory update --agent
```

## Safety Rules

- Treat orders, fulfillment, returns, and buyer identifiers as PII.
- Use `shops info` to obtain `shop_cipher`; do not invent or decrypt shop ciphers.
- Use `--dry-run` before first live calls to inspect signed request shape without transmitting it.
- Do not retry mutations. Safe v1 only executes read commands and auth token calls.
- Do not use unofficial endpoints, SDK guesses, blog posts, or copied Postman collections for missing API behavior.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success, including intentional deferred placeholder output |
| 1 | Unclassified error |
| 2 | Usage error |
| 4 | Authentication/config material missing |
| 5 | Upstream API error |
| 7 | Rate limited |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. Empty, `help`, or `--help` means show `tiktok-shop-pp-cli --help`.
2. Starts with `install` and ends with `mcp` means MCP installation; otherwise CLI installation.
3. Anything else means direct use with `--agent`.

## CLI Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/cmd/tiktok-shop-pp-cli@latest
tiktok-shop-pp-cli --version
```

## MCP Server Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/cmd/tiktok-shop-pp-mcp@latest
claude mcp add tiktok-shop-pp-mcp -- tiktok-shop-pp-mcp
```

The MCP server exposes the same safe v1 read surface and an `inventory_update_status` explainer instead of a stock mutation tool.
