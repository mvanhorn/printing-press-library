# Jimmy John's CLI

CLI for the Jimmy John's ordering API. Browse stores, menus, and product
modifiers; manage your cart; place orders; view rewards and saved payments.
Backed by Jimmy John's proprietary API at www.jimmyjohns.com/api and
authenticated via cookies imported from a logged-in Chrome session
(PerimeterX clearance + JJ session cookies).

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

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. **Jimmy John's runs PerimeterX bot protection** that aggressively fingerprints automation; the canonical way in is to capture a fresh session from a real Chrome browser, then import the cookies.

**Recommended (works against PerimeterX):**

```bash
# 1. Open Chrome, navigate to jimmyjohns.com, solve any PerimeterX challenge,
#    and browse naturally — find a store, view the menu, sign in if you want
#    rewards/order endpoints. Do NOT let any automation touch this session.
# 2. Export cookies from that exact Chrome session:
browser-use -b real --profile "Default" cookies export ~/jj-cookies.json

# 3. Import into the CLI:
jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json
```

**Legacy path (often fails — Chrome locks its cookie DB while running):**

```bash
jimmy-johns-pp-cli auth login --chrome
```

Requires `pycookiecheat`, `cookies`, or `cookie-scoop-cli` on PATH. Chrome must NOT be running for these tools to read the encrypted cookie DB.

When your session expires, repeat the recommended flow above. See **Known Gaps** below for the PerimeterX caveat.

### 3. Verify Setup

```bash
jimmy-johns-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
jimmy-johns-pp-cli stores list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local cart composition
- **`menu unwich-convert`** — Convert a sandwich's modifier set to an Unwich (lettuce wrap) variant — pure-local computation, no live API call.

  _Reach for this when an agent is building a JJ cart for a user with a no-bread preference — it gives you the exact modifier delta with no API round-trip._

  ```bash
  jimmy-johns-pp-cli menu product-modifiers 33328641 --json | jimmy-johns-pp-cli menu unwich-convert --product-id 33328641 --json
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

## Health Check

```bash
jimmy-johns-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/jimmy-johns-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Known Gaps

This CLI is `ship-with-gaps`. The structural surface is complete (16 endpoints typed, MCP server bundled, dry-run works for every command), but the live API has real limitations:

- **PerimeterX bot protection.** Jimmy John's runs a `browser_required`-class anti-automation stack. Even with valid session cookies + Surf's Chrome TLS impersonation, requests can return HTTP 403 if PerimeterX has flagged the session. The reliable workaround is the **fresh session** capture documented in [Authenticate](#2-authenticate) — do NOT touch the source Chrome window with automation between login and cookie export. Sessions that get fingerprinted as bots stay flagged for ~1 hour.
- **No transcendence features yet.** Planned but not yet built: `unwich-mode` (auto-convert a cart to lettuce wraps), `office-lunch --people N` (sized cart suggestion for groups), `freaky-fast` (per-store delivery ETA predictor based on local order history). All are pure local computations on synced data; they'd ship in the next iteration.
- **Cookie auth is hand-wired.** `auth import-cookies` was added by hand because the spec's `auth.type: cookie` doesn't map cleanly onto the press's v4.x templates. Future press versions may emit this natively.

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `jimmy-johns-pp-cli doctor` to check credentials
- Re-import a fresh cookie export from your real Chrome session (see [Authenticate](#2-authenticate)); the cookie may have expired

**HTTP 403 from `stores list` / `menu products` / etc.**
- PerimeterX flagged the session. Capture a fresh cookie set from a Chrome window that has NEVER been driven by automation, then re-run `auth import-cookies`.

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Surf with Chrome TLS impersonation for the underlying HTTP transport. It does not keep a resident browser process — discovery is one-shot at auth time. See [Known Gaps](#known-gaps) for the PerimeterX caveat.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
