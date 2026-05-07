# TikTok Shop Printing Press CLI/MCP

Safe v1 CLI and MCP server for confirmed TikTok Shop Seller APIs.

This package intentionally exposes a conservative surface based only on official TikTok Shop Partner Center docs: auth readiness, token exchange and refresh, shop discovery, read-only orders, products, inventory search, fulfillment packages, and warehouses. Inventory update is documented as confirmed but remains deferred until idempotency, retry, and operator-confirmation safety are designed.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tiktok-shop --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tiktok-shop --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tiktok-shop skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tiktok-shop. The skill defines how its required CLI can be installed.
```

## Install

The recommended path installs both the `tiktok-shop-pp-cli` binary and the `pp-tiktok-shop` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install tiktok-shop
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tiktok-shop --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/cmd/tiktok-shop-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tiktok-shop-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Configure

Use credentials obtained through official TikTok Shop Partner Center flows.

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

## Examples

```bash
tiktok-shop-pp-cli doctor --json
tiktok-shop-pp-cli auth status --json
tiktok-shop-pp-cli shops info --dry-run --json
tiktok-shop-pp-cli orders list --limit 20 --json
tiktok-shop-pp-cli products get <product-id> --json
tiktok-shop-pp-cli inventory get <sku-id> --json
tiktok-shop-pp-cli fulfillment warehouses --json
tiktok-shop-pp-cli inventory update --json
```

`--dry-run` builds the request shape without sending it and redacts app key, app secret, access token, refresh token, shop cipher, auth code, and generated signature material from output.

## Safety Boundaries

- Orders, fulfillment data, buyer IDs, and returns/refunds data may contain PII.
- Safe v1 does not implement inventory mutation, product mutation, returns/refunds, shipping label mutation, finance, or webhook registration.
- Do not retry mutations. The only implemented network calls are read operations and official token exchange/refresh calls.
- Do not add endpoints from SDK guesses, blog posts, copied Postman collections, or unofficial examples.

## MCP

Register the MCP server with Claude Code:

```bash
claude mcp add tiktok-shop-pp-mcp -- tiktok-shop-pp-mcp
```

The MCP server exposes the same safe v1 read surface plus `inventory_update_status`, which explains why stock mutation is deferred.
