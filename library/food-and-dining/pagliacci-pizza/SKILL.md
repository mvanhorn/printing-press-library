---
name: pp-pagliacci-pizza
description: "Order pizza from Pagliacci Pizza (Seattle-area chain) in the terminal. Browse the menu, build an order, apply stored coupons and rewards, and send the order through Pagliacci's web ordering API. Use when the user wants Pagliacci specifically (Seattle / Pacific Northwest delivery area), wants to check Pagliacci rewards, reuse a past order, or manage a Pagliacci account. Use pp-dominos for national Domino's; use pp-pagliacci-pizza only for Pagliacci."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["pagliacci-pizza-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@latest","bins":["pagliacci-pizza-pp-cli"],"label":"Install via go install"}]}}'
---

# Pagliacci Pizza - Printing Press CLI

Order from Pagliacci Pizza, a Seattle-area pizza chain, from the terminal. The CLI wraps Pagliacci's web ordering API (sniffed) so browsing menus, redeeming rewards, and placing orders all happen without opening the browser.

## When to Use This CLI

Reach for this when the user wants:

- order Pagliacci specifically (if they're in the Seattle / Puget Sound area)
- browse the Pagliacci menu by slice, topping, or category (`menu-top`, `menu-slices`)
- build an order with suggestions from past orders (`order-suggestion`)
- apply stored coupons, credit, or gift balance (`stored-coupons`, `stored-credit`, `stored-gift`)
- view or redeem Pagliacci reward-card balance (`reward-card`)
- look up a past order or reorder from history (`order-list`)
- manage a Pagliacci account (register, login, reset password)

Skip it when the user is outside Pagliacci's delivery area; they'll get no-store-nearby errors. Use `pp-dominos` for national-chain pizza instead.

## Financial Actions Caveat

`order-send` places a real paid order against the user's Pagliacci account. Agents must ask the user to confirm the cart total and delivery address before invoking it. `--yes` suppresses the confirmation prompt but is not a dollar-amount gate.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `pagliacci-pizza-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation (binary may not ship; check); otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@latest
   ```

   If `@latest` installs a stale build (Go module proxy cache lag):
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@main
   ```
3. Verify: `pagliacci-pizza-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup (cookie-session based):
   ```bash
   pagliacci-pizza-pp-cli login --email you@example.com --password "..."
   # or export the captured session cookie directly:
   export PAGLIACCI_PIZZA_PAGLIACCI_AUTH="<cookie-value>"
   ```
6. Verify: `pagliacci-pizza-pp-cli doctor` reports session status.

## Direct Use

1. Check installed: `which pagliacci-pizza-pp-cli`. If missing, offer CLI installation.
2. If not logged in, run `login` first. Session persists via cookie.
3. Discover commands: `pagliacci-pizza-pp-cli --help`; drill into `pagliacci-pizza-pp-cli <cmd> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   pagliacci-pizza-pp-cli <command> [args] --agent
   ```

## Notable Commands

| Command | What it does |
|---------|--------------|
| `login` / `logout` | Session management (email + password -> cookie) |
| `register` | Create a new Pagliacci account |
| `menu-top` | Top-level menu (categories, featured items) |
| `menu-slices` | Browse available slices at a store |
| `store` | Find store by address or ID, list hours |
| `order-list` | Past orders on the account |
| `order-suggestion` | Build a new order from a past one |
| `quote-building` / `quote-store` | Price the current cart |
| `order-send` | Place the order (real money) |
| `reward-card` | View and redeem reward balance |
| `stored-coupons` / `stored-credit` / `stored-gift` | Applied credits on the account |
| `password-forgot` / `password-reset` | Account recovery |

Run any command with `--help` for full flag documentation.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (store outside delivery area, order not found) |
| 4 | Authentication required (not logged in or session expired) |
| 5 | API error (Pagliacci upstream) |
| 7 | Rate limited |
