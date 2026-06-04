# Harris Teeter CLI

Harris Teeter grocery shopping API discovered from the logged-in web app

Learn more at [Harris Teeter](https://www.harristeeter.com).

Created by [@jwmoss](https://github.com/jwmoss) (Jonathan Moss).

## Install

The recommended path installs both the `harris-teeter-pp-cli` binary and the `pp-harris-teeter` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install harris-teeter
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install harris-teeter --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install harris-teeter --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install harris-teeter --agent claude-code
npx -y @mvanhorn/printing-press-library install harris-teeter --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/harris-teeter/cmd/harris-teeter-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/harris-teeter-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-harris-teeter --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-harris-teeter --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install harris-teeter --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
harris-teeter-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/harris-teeter-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/harris-teeter/cmd/harris-teeter-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "harris-teeter": {
      "command": "harris-teeter-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Authenticate

This CLI uses your browser session for authentication. Log in to .harristeeter.com in Chrome, then:

```bash
harris-teeter-pp-cli auth login --chrome
```

The CLI uses `pycookiecheat`, `cookies`, or `cookie-scoop-cli` when available. On this machine it can also fall back to the live Chrome session through `agent-browser`/`browser-use`, which matches the browser-sniff workflow used to create the CLI. If live browser fallback is unavailable, install one:

```bash
pip install pycookiecheat          # Python (recommended)
brew install barnardb/cookies/cookies  # Homebrew
```

When your session expires, run `auth login --chrome` again.

Product and coupon endpoints also require Harris Teeter's location/availability/fulfillment headers. The CLI defaults to the captured Beau Rivage Marketplace context (`location-id=09700096`, `facility-id=12956`). Override with `HARRIS_TEETER_LOCATION_ID`, `HARRIS_TEETER_FACILITY_ID`, `HARRIS_TEETER_MODALITY_TYPE`, or a full `HARRIS_TEETER_LAF_OBJECT` JSON value when using a different store.

### 3. Verify Setup

```bash
harris-teeter-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
harris-teeter-pp-cli cart
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Browser-backed reachability
- **`auth login --chrome`** — Read Harris Teeter session cookies from the already logged-in Chrome session, falling back through agent-browser/browser-use when standalone cookie tools are missing.

  _Use this first when the CLI reports auth errors or a browser session has changed._

  ```bash
  harris-teeter-pp-cli auth login --chrome
  ```

### Store-context grocery reads
- **`products search`** — Search Harris Teeter products using the browser-observed Atlas search endpoint with location, fulfillment method, and LAF/modality headers.

  _Use this to inspect current store-specific grocery results from a terminal or agent workflow._

  ```bash
  harris-teeter-pp-cli products search --query milk --location-id 09700096 --page-size 5 --json --no-input
  ```
- **`products get`** — Fetch full product, offer, nutrition, inventory, and variant projections by UPC/GTIN through the logged-in Atlas product endpoint.

  _Use this when an agent needs exact item metadata without scraping a rendered product page._

  ```bash
  harris-teeter-pp-cli products get --upc 0007203673813 --json --no-input
  ```

### Account-aware savings
- **`coupons`** — List Harris Teeter digital coupons from the authenticated web endpoint, including optional UPC filtering.

  _Use this to check available savings before building a grocery list._

  ```bash
  harris-teeter-pp-cli coupons --page-size 5 --json --no-input
  ```
- **`cart`** — Inspect the authenticated Harris Teeter cart and shopping-list surfaces without exposing mutating checkout or order actions.

  _Use this when an agent needs to understand the current cart state without changing it._

  ```bash
  harris-teeter-pp-cli cart --json --no-input
  ```

## Usage

Run `harris-teeter-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Inspect logged-in customer preferences and membership state.

- **`harris-teeter-pp-cli account enrollments`** - List membership enrollments and benefits for the current account.
- **`harris-teeter-pp-cli account preferences`** - List customer preferences for the logged-in account.

### cart

Inspect the logged-in account cart.

- **`harris-teeter-pp-cli cart list`** - List carts for the current logged-in Harris Teeter account.

### coupons

List available digital coupons and coupon-product links.

- **`harris-teeter-pp-cli coupons list`** - List available digital coupons, optionally filtered by UPC.

### lists

Inspect Harris Teeter shopping lists.

- **`harris-teeter-pp-cli lists get`** - Get a shopping list by ID.
- **`harris-teeter-pp-cli lists list`** - List shopping lists for the current logged-in account.

### products

Search Harris Teeter products, look up item details, and inspect search facets.

- **`harris-teeter-pp-cli products get`** - Get full product, offer, nutrition, inventory, and variant details by UPC/GTIN.
- **`harris-teeter-pp-cli products related-tags`** - Get related search tags for a query and location.
- **`harris-teeter-pp-cli products search`** - Search products for a store location and fulfillment method.
- **`harris-teeter-pp-cli products suggestions`** - Get search suggestions for a query and location.
- **`harris-teeter-pp-cli products visual-navigations`** - Get visual navigation categories shown on search pages.

### recommendations

Inspect personalized grocery recommendations from the web app.

- **`harris-teeter-pp-cli recommendations better-for-you`** - Get better-for-you product recommendations.
- **`harris-teeter-pp-cli recommendations purchase-history-homepage`** - Get homepage purchase-history shortcuts for the logged-in account.
- **`harris-teeter-pp-cli recommendations start-my-cart`** - Get the Start My Cart product recommendations shown on the homepage.

### stores

Find Harris Teeter stores and store metadata.

- **`harris-teeter-pp-cli stores`** - Find stores by ZIP code, city, state, or address text.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
harris-teeter-pp-cli cart

# JSON for scripting and agents
harris-teeter-pp-cli cart --json

# Filter to specific fields
harris-teeter-pp-cli cart --json --select id,name,status

# Dry run — show the request without sending
harris-teeter-pp-cli cart --dry-run

# Agent mode — JSON + compact + no prompts in one flag
harris-teeter-pp-cli cart --agent
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
harris-teeter-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/harris-teeter-pp-cli/config.toml`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `harris-teeter-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.harristeeter.com/ruxitagentjs_ICA15789NPQRTUVXfhqrtux_10337260504112723.js
- Capture coverage: 166 API entries from 889 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: bearer_token — headers: Authorization; api_key — query: filter.keyword, key
- Protection signals: cloudflare (90% confidence), akamai (75% confidence)
- Generation hints: browser_http_transport, requires_protected_client, weak_schema_confidence
- Candidate command ideas: create_EdClXAWAB — Derived from observed POST /qB1py7FxiRxje/xBaRro/nGmSVUVg/N3irhr9NJ5N5cQ/dW1ccAkbBg/G3J/EdClXAWAB traffic.; create_dont_forget_usual_products — Derived from observed POST /atlas/v1/recommendations/v1/dont-forget-usual-products traffic.; create_echoData — Derived from observed POST /clickstream/v1/echoData traffic.; create_events — Derived from observed POST /clickstream/v1/events traffic.; create_preferences — Derived from observed POST /atlas/v1/modality/preferences traffic.; create_prioritized_carousels — Derived from observed POST /atlas/v1/search/v1/prioritized-carousels traffic.; create_qESoJeGYu — Derived from observed POST /qB1py7FxiRxje/xBaRro/nGmSVUVg/p8irhr9N/Ph5ncAkbBg/V3l/qESoJeGYu traffic.; create_realtimeconversion — Derived from observed POST /track/realtimeconversion traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
