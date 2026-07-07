# Grubhub CLI

**Every Grubhub restaurant search, plus a sortable delivery-fee comparison board, a cross-restaurant deal radar, and offline menu-item search no Grubhub tool has.**

grubhub-pp-cli browses Grubhub's marketplace from the command line: search restaurants by street address, browse full menus, and compare delivery fees, minimums, and ETAs across the whole neighborhood at once with `compare`. It caches restaurants and menus in a local SQLite store so `dish` can full-text-search menu items across nearby restaurants and `deals` can rank every active offer in one sweep. No API key required — it mints an anonymous Grubhub token for you.

## Install

The recommended path installs both the `grubhub-pp-cli` binary and the `pp-grubhub` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install grubhub
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install grubhub --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install grubhub --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install grubhub --agent claude-code
npx -y @mvanhorn/printing-press-library install grubhub --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/grubhub/cmd/grubhub-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/grubhub-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install grubhub --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-grubhub --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-grubhub --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install grubhub --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/grubhub-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GRUBHUB_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/grubhub/cmd/grubhub-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "grubhub": {
      "command": "grubhub-pp-mcp",
      "env": {
        "GRUBHUB_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Grubhub has no public API key. grubhub-pp-cli authenticates exactly like the website: it scrapes a fresh anonymous client id from grubhub.com and mints a short-lived bearer token automatically on first use, caching it locally. You don't set anything. For the raw `restaurants` endpoint commands you can optionally run `grubhub-pp-cli auth login` (still credential-free) or set GRUBHUB_TOKEN. Logged-in features like order history are not supported in this version.

## Quick Start

```bash
# Confirm Grubhub is reachable before searching.
grubhub-pp-cli doctor

# Search restaurants near an address, cheapest delivery first.
grubhub-pp-cli near "350 5th Ave, New York, NY" --sort fee --limit 10

# Side-by-side fee/minimum/ETA board for the whole neighborhood.
grubhub-pp-cli compare "350 5th Ave, New York, NY" --sort eta

# Find which nearby restaurants carry a dish under a price.
grubhub-pp-cli dish "350 5th Ave, New York, NY" "poke bowl" --max-price 15

# Rank nearby restaurants by active offers and coupons.
grubhub-pp-cli deals "350 5th Ave, New York, NY"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`compare`** — See delivery fee, minimum, ETA, rating, and distance for every nearby restaurant side by side, sorted however you want.

  _Reach for this when a user wants the cheapest or fastest option across the whole neighborhood, not one restaurant at a time._

  ```bash
  grubhub-pp-cli compare "350 5th Ave, New York, NY" --sort eta --agent
  ```
- **`dish`** — Find which nearby restaurants carry a specific dish, with price, by full-text searching cached menus across the neighborhood.

  _Reach for this when the user names a dish, not a restaurant: 'who near me has a poke bowl under $15'._

  ```bash
  grubhub-pp-cli dish "350 5th Ave, New York, NY" "poke bowl" --max-price 15 --agent
  ```

### Deal hunting
- **`deals`** — Rank every nearby restaurant currently running an offer, coupon, or promo code in one sweep.

  _Reach for this when the deal should pick the restaurant, not the other way around._

  ```bash
  grubhub-pp-cli deals "350 5th Ave, New York, NY" --agent
  ```
- **`pick`** — Get one recommended restaurant from a transparent score over fee, rating, active deals, and ETA.

  _Reach for this when the user wants a single 'just pick one' answer with a visible score breakdown._

  ```bash
  grubhub-pp-cli pick "350 5th Ave, New York, NY" --weight-deal 2 --agent
  ```

## Recipes


### Cheapest fast delivery near an address

```bash
grubhub-pp-cli compare "350 5th Ave, New York, NY" --sort fee --eta-under 30 --agent
```

Comparison board filtered to sub-30-minute ETAs, cheapest delivery first, as agent JSON.

### Find a dish across the neighborhood

```bash
grubhub-pp-cli dish "1 Infinite Loop, Cupertino, CA" "burrito" --max-price 12
```

Full-text search of cached menus for burritos under $12 at any nearby restaurant.

### Pull a full menu as compact JSON for an agent

```bash
grubhub-pp-cli menu 1414955 --agent --select menu_category_list.menu_item_list.name,menu_category_list.menu_item_list.price
```

Restaurant menus are deeply nested; --select narrows the payload to item names and prices so agents don't burn context on the full blob.

### Let the deal pick the restaurant

```bash
grubhub-pp-cli deals "350 5th Ave, New York, NY" --sort value
```

Ranks every nearby restaurant running an offer by value in one sweep.

## Usage

Run `grubhub-pp-cli --help` for the full command reference and flag list.

## Commands

These are the primary commands. They take a street **address** (auto-geocoded) and
mint an anonymous token for you — no API key or setup.

- **`grubhub-pp-cli near <address>`** - Search restaurants near an address (`--cuisine`, `--pickup`, `--sort`, `--open-now`, `--limit`)
- **`grubhub-pp-cli compare <address>`** - Sortable delivery fee / minimum / ETA / rating board (`--sort`, `--max-fee`, `--max-min`, `--eta-under`)
- **`grubhub-pp-cli dish <address> <query>`** - Find which nearby restaurants carry a dish (`--max-price`, `--diet`, `--max-scan-restaurants`; `--data-source local` for cached)
- **`grubhub-pp-cli deals <address>`** - Rank nearby restaurants by active offers and coupons (`--sort value|count`)
- **`grubhub-pp-cli pick <address>`** - One recommended restaurant from a transparent score (`--weight-fee/-eta/-rating/-deal`)
- **`grubhub-pp-cli menu <restaurant-id>`** - Browse a restaurant's full menu (`--category`, `--popular`, `--limit`)
- **`grubhub-pp-cli item <restaurant-id> <item-id>`** - Show a menu item's modifiers and prices
- **`grubhub-pp-cli geocode <address>`** - Resolve a street address to coordinates

### restaurants (raw API surface)

Hidden, typed endpoint commands for power users. They take a `POINT(lng lat)` location
and need a token (`grubhub-pp-cli auth login`, still credential-free, or `GRUBHUB_TOKEN`).

- **`grubhub-pp-cli restaurants get <id>`** - Get a restaurant's details and full menu by id
- **`grubhub-pp-cli restaurants menu-item <id> <item-id>`** - Get a menu item's full detail including modifier/choice categories
- **`grubhub-pp-cli restaurants search`** - Search restaurants near a location (raw; use `near` for address-based search)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
grubhub-pp-cli restaurants get mock-value

# JSON for scripting and agents
grubhub-pp-cli restaurants get mock-value --json

# Filter to specific fields
grubhub-pp-cli restaurants get mock-value --json --select id,name,status

# Dry run — show the request without sending
grubhub-pp-cli restaurants get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
grubhub-pp-cli restaurants get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
grubhub-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/grubhub-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GRUBHUB_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `grubhub-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `grubhub-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GRUBHUB_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 403 or 'access denied' on search** — Grubhub uses PerimeterX bot protection; retry, slow your request rate, or run from a residential network. The CLI already sends a realistic browser User-Agent.
- **Empty results for a valid address** — Confirm the address geocodes: run `grubhub-pp-cli geocode "<address>"`. If it returns no coordinates, add city/state/zip.
- **'no local mirror' from compare/dish/deals** — Run `grubhub-pp-cli sync "<address>"` first to cache restaurants and menus for that location.
- **Token expired after a while** — Tokens are minted automatically; just re-run the command. For the raw `restaurants` surface, re-run `grubhub-pp-cli auth login`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**n0shake/dash**](https://github.com/n0shake/dash) — Python (30 stars)
- [**jlumbroso/grubhub**](https://github.com/jlumbroso/grubhub) — Python (6 stars)
- [**patilanup246/grubhubScraper**](https://github.com/patilanup246/grubhubScraper) — Python (2 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
