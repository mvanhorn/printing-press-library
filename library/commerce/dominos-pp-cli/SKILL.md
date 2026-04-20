---
name: pp-dominos
description: "Order pizza from Domino's Pizza in the terminal. Find stores, browse menus, build carts, apply deals and rewards, validate pricing, place orders, and track deliveries. Use when the user wants to order Domino's, compare Domino's prices across stores, see current deals or member rewards, build a cart, check delivery status, or estimate nutrition for an order. Skip for Seattle-local pagliacci (use pp-pagliacci-pizza) or for anything other than Domino's."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["dominos-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/commerce/dominos-pp-cli/cmd/dominos-pp-cli@latest","bins":["dominos-pp-cli"],"label":"Install via go install"}]}}'
---

# Domino's Pizza - Printing Press CLI

Order pizza, browse menus, track deliveries, and manage Domino's rewards from the terminal. The CLI talks to Domino's public order API and caches menu + deal data locally so browsing stays snappy.

## When to Use This CLI

Reach for this when the user wants:

- place a real Domino's order (`checkout` or `orders`)
- find nearby stores or compare prices across them (`stores`, `compare-prices`)
- browse a store's menu or search for specific items (`menu`)
- see today's deals and member-only offers (`deals`, `rewards`)
- build and manage carts before placing an order (`cart`)
- save / reuse order templates for repeat orders (`template`)
- track an active delivery through to the door (`track`, `tracking`)
- estimate nutrition totals from cached menu data (`nutrition`)
- first-time onboarding (`quickstart` walks address + menu + first order)

Skip it when the user wants Seattle-local pagliacci — use `pp-pagliacci-pizza` instead. Skip entirely for non-Domino's chains.

## Financial Actions Caveat

The `checkout` and `orders` commands place real paid orders. The CLI requires `--yes` to proceed, but the gate is a confirmation-check, not a dollar-amount cap. Before invoking these from an agent context, ASK THE USER TO CONFIRM the cart total and delivery address. Agents should never place an order unprompted.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `dominos-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/dominos-pp-cli/cmd/dominos-pp-cli@latest
   ```

   If `@latest` installs a stale build (Go module proxy cache lag):
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/commerce/dominos-pp-cli/cmd/dominos-pp-cli@main
   ```
3. Verify: `dominos-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Identity setup (no API key; the service keys off delivery address + phone):
   ```bash
   dominos-pp-cli quickstart
   ```
   The quickstart walks adding a delivery address, finding the nearest store, and previewing the menu.
6. Verify: `dominos-pp-cli doctor` reports address, store, and cart status.

## MCP Server Installation

The CLI ships an MCP server at `dominos-mcp`:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/dominos-pp-cli/cmd/dominos-mcp@latest
claude mcp add dominos-mcp -- dominos-mcp
```

## Direct Use

1. Check installed: `which dominos-pp-cli`. If missing, offer CLI installation.
2. For a first-time user, run `dominos-pp-cli quickstart` to set up address + default store.
3. Discover commands: `dominos-pp-cli --help`; drill into `dominos-pp-cli <cmd> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   dominos-pp-cli <command> [args] --agent
   ```
5. For order flows: `address set` -> `menu` -> `cart add` -> `cart show` -> `checkout --yes`.

## Notable Commands

| Command | What it does |
|---------|--------------|
| `quickstart` | Guided setup + first order walkthrough |
| `address` | Manage saved delivery addresses (add/list/default) |
| `stores` | Find nearest store, show hours, verify delivery availability |
| `menu` | Browse a store's menu, search for items |
| `deals` | List store deals; sync to local for offline comparison |
| `compare-prices` | Compare the same item across nearby stores |
| `cart` | Add / remove / inspect items in the active cart |
| `rewards` | Loyalty points, member rewards, and member-only deals |
| `checkout` | Validate + price + place the active cart (requires `--yes`) |
| `orders` | Create / validate / price / place at a lower level than checkout |
| `track` | Watch an active order through tracker stages |
| `template` | Save and reuse common orders |
| `nutrition` | Nutrition totals for a cart or itemized list |

Run any command with `--help` for full flag documentation.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (store, item, order) |
| 4 | Authentication / identity required (address or phone missing) |
| 5 | API error (Domino's upstream, including store-closed or item-unavailable) |
| 7 | Rate limited |
