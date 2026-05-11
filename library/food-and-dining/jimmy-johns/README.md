# Jimmy John's CLI

**First terminal CLI for Jimmy John's ordering — local Unwich conversion, agent-native JSON, every endpoint typed.**

Browse stores and menus, build carts, view rewards, and one-shot reorders from the terminal. The Unwich converter computes lettuce-wrap modifier deltas locally so agents can build no-bread orders without an extra API round trip.

Learn more at [Jimmy John's](https://www.jimmyjohns.com).

## Install

The recommended path installs both the `jimmy-johns-pp-cli` binary and the `pp-jimmy-johns` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install jimmy-johns
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install jimmy-johns --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jimmy-johns-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jimmy-johns --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jimmy-johns --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-jimmy-johns skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-jimmy-johns. The skill defines how its required CLI can be installed.
```

## Authentication

Jimmy John's runs PerimeterX bot protection. Authenticate by capturing cookies from a fresh, hand-driven Chrome session via 'browser-use cookies export', then 'jimmy-johns-pp-cli auth import-cookies --from-file <path>'. Sessions that get fingerprinted by automation stay flagged for ~1 hour.

## Quick Start

```bash
# Import cookies captured from a clean Chrome session
jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json


# Find stores near a ZIP — returns hours, distance, delivery/pickup flags
jimmy-johns-pp-cli stores list --address 98112 --json


# Generate a sized cart for 6 people — sandwiches + sides + drinks with dietary filters
jimmy-johns-pp-cli order plan --people 6 --json


# Compute the modifier delta for converting a sandwich to a lettuce wrap
jimmy-johns-pp-cli menu unwich-convert --from-file mods.json --product-id 33328641 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local cart composition
- **`menu unwich-convert`** — Convert a sandwich's modifier set to an Unwich (lettuce wrap) variant — pure-local computation, no live API call.

  _Reach for this when an agent is building a JJ cart for a user with a no-bread preference — it gives you the exact modifier delta with no API round-trip._

  ```bash
  jimmy-johns-pp-cli menu product-modifiers 33328641 --json | jimmy-johns-pp-cli menu unwich-convert --product-id 33328641 --json
  ```
- **`order plan`** — Suggest a sized cart for a group order — sandwiches + sides + cookies + drinks scaled to N people with dietary filters.

  _Reach for this when an agent gets a 'lunch for the team' request — it returns a ready-to-submit cart structure with rationale per line._

  ```bash
  jimmy-johns-pp-cli order plan --people 8 --dietary vegetarian --json
  ```
- **`menu half-and-half`** — Compose a two-product share order with the agent-facing note that JJ doesn't natively support half-and-half slicing.

  _Reach for this when a user says 'half Vito, half Pepe' — the command outputs the actual cart and the in-store ask the user has to make._

  ```bash
  jimmy-johns-pp-cli menu half-and-half --left 33328641 --right 33328700 --json
  ```

## Usage

Run `jimmy-johns-pp-cli --help` for the full command reference and flag list.

## Commands

### account

User account, profile, addresses, and saved payments

- **`jimmy-johns-pp-cli account current`** - Get the authenticated user's profile (name, email, preferences).
- **`jimmy-johns-pp-cli account delivery_addresses`** - List the authenticated user's saved delivery addresses.
- **`jimmy-johns-pp-cli account login`** - Authenticate with email + password. Sets JJ session cookies.
- **`jimmy-johns-pp-cli account saved_payments`** - List the authenticated user's saved payment methods.
- **`jimmy-johns-pp-cli account web_token`** - Refresh the web session token (called internally by the SPA).

### menu

Menu products, filters, and modifier options

- **`jimmy-johns-pp-cli menu product_filters`** - List available menu filter dimensions (categories, dietary tags, allergens).
- **`jimmy-johns-pp-cli menu product_modifiers`** - List modifier groups (bread, toppings, add-ons) for a specific product.
- **`jimmy-johns-pp-cli menu products`** - List menu products for the current store (subs, sides, drinks, cookies, catering).

### order

Cart and order management

- **`jimmy-johns-pp-cli order add_items`** - Add one or more items to the current cart in a single call.
- **`jimmy-johns-pp-cli order current`** - Get the current in-progress order/cart.
- **`jimmy-johns-pp-cli order upsell`** - Get upsell suggestions for the current cart (sides, drinks, cookies).

### rewards

Freaky Fast Rewards points balance and catalog

- **`jimmy-johns-pp-cli rewards catalog`** - List available reward redemptions for the current points balance.
- **`jimmy-johns-pp-cli rewards summary`** - Get the authenticated user's rewards points balance and recent activity.

### stores

Jimmy John's store locations and operating info

- **`jimmy-johns-pp-cli stores get_disclaimers`** - Get store-specific disclaimers (delivery zone caveats, hours warnings).
- **`jimmy-johns-pp-cli stores list`** - List stores. Accepts an address search or filter; returns stores with hours, distance, pickup/delivery flags.

### system

System utilities (Google Maps signing for store finder)

- **`jimmy-johns-pp-cli system sign_map_url`** - Sign a Google Maps URL for client-side use (used internally by store finder)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jimmy-johns-pp-cli stores list

# JSON for scripting and agents
jimmy-johns-pp-cli stores list --json

# Filter to specific fields
jimmy-johns-pp-cli stores list --json --select id,name,status

# Dry run — show the request without sending
jimmy-johns-pp-cli stores list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jimmy-johns-pp-cli stores list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `JIMMY_JOHNS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `jimmy-johns-pp-cli menu`
- `jimmy-johns-pp-cli menu product_filters`
- `jimmy-johns-pp-cli menu product_modifiers`
- `jimmy-johns-pp-cli menu products`
- `jimmy-johns-pp-cli stores`
- `jimmy-johns-pp-cli stores get_disclaimers`
- `jimmy-johns-pp-cli stores list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-jimmy-johns -g
```

Then invoke `/pp-jimmy-johns <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
jimmy-johns-pp-cli auth login --chrome

claude mcp add jimmy-johns jimmy-johns-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
jimmy-johns-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jimmy-johns-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jimmy-johns": {
      "command": "jimmy-johns-pp-mcp"
    }
  }
}
```

</details>

## Doctor

```bash
jimmy-johns-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/jimmy-johns-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `jimmy-johns-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 403 from every API call** — PerimeterX flagged the session. Capture a fresh cookie set from an undriven Chrome window and re-import.
- **auth login --chrome can't read cookies** — Chrome must be closed for pycookiecheat/cookies to read the encrypted DB. Use 'auth import-cookies --from-file' with a browser-use export instead.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

## Cookbook

Three workflows worth bookmarking. All three are pure-local computations — no live API call required once you have synced menu data.

### Lunch for the team

```bash
# Pipe the synced menu into order plan with --people N
jimmy-johns-pp-cli menu products --json | \
  jimmy-johns-pp-cli order plan --people 8 --dietary vegetarian --json
```

Returns a quantity-tagged cart with sandwich variety, sides, drinks, and cookies sized to 8 people. Each line carries a `reason` field explaining the rationale. Add `--dietary vegetarian,no-pork` for combined filters.

### Convert a sandwich to an Unwich

```bash
# Pull the modifier set for a specific sandwich
jimmy-johns-pp-cli menu product-modifiers 33328641 --json > /tmp/mods.json

# Compute the modifier delta to swap bread for lettuce wrap
jimmy-johns-pp-cli menu unwich-convert --from-file /tmp/mods.json --product-id 33328641 --json
```

Outputs the exact modifier ID change needed to make any sandwich an Unwich. Splice the `diff` field into your `order add-items` payload.

### Share a sandwich two ways

```bash
jimmy-johns-pp-cli menu half-and-half --left 33328641 --right 33328700 \
  --left-label "Vito" --right-label "Pepe" --json
```

Returns a two-line cart structure with the agent-facing note that JJ doesn't natively support half-and-half slicing. The notes spell out the in-store ask the user needs to make at pickup.
