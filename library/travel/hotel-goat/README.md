# Hotel Goat — multi-source cash hotel CLI

**Free hotel CLI — cash prices from Google Hotels + Trivago, deep booking links, agent-native JSON, and local SQLite wishlist. No API key needed.**

hotel-goat fans out across two cash-price sources by default:
- **Google Hotels** — scraped from the server-rendered page (same data the web UI shows)
- **Trivago** — called via Trivago's public MCP server (`https://mcp.trivago.com/mcp`), which exposes OTA-aggregated rates from Booking.com, Expedia, Agoda, Hotels.com, Priceline, etc.

Pick a single source with `--source google` or `--source trivago`; the default is `--source both`. When both sources see the same property (matched on lat/lng + name overlap), the OTA prices are merged into one `prices[]` array. Trivago-only properties are appended as standalone rows so the agent gets a wider candidate set.

Trivago is geolocated server-side and returns EUR regardless of client hints. When the headline currency differs (typically Google's USD vs Trivago's EUR), each Trivago price is converted via the Frankfurter ECB FX endpoint (free, no key, 24h on-disk cache) so the agent compares apples-to-apples. The source label records what happened:

- **`trivago/<OTA> [EUR 802 -> USD]`** — FX conversion succeeded. The numeric `price` and headline `price_per_night` are the converted (USD) values; the native EUR amount is preserved in the label so the agent can see both.
- **`trivago/<OTA> [EUR]`** — FX lookup failed (offline, Frankfurter outage). The numeric `price` is the native EUR value; the headline `price_per_night` is NOT overridden, so cross-source comparisons remain meaningful.
- **`trivago/<OTA>`** (no bracket suffix) — currencies already matched, no conversion needed.

v1 ships:
- `hotels <location> <ci> <co>` — multi-source search with rich filters (brand, hotel-class, max-price, min-rating, amenities, currency)
- `dates <location> --from --to --nights N` — sweep a date window for the cheapest pair per stay
- `near "<address>" --radius Nmi` — geo-radius search around any address (auto-geocoded via OpenStreetMap)
- `hotel show <token>` / `hotel reviews <token>` — single-property detail + review breakdown
- `wishlist add/list/remove <token>` — local SQLite saves

Brand-loyalty expansion, full month-window scans (`cheapest-window`), price drift tracking, multi-room family logic, and per-OTA breakdown are deferred to v0.2 — see [Known Gaps](#known-gaps) below.

## Known Gaps

This is a focused v1. The following features were scoped in the design but deferred:

- `cheapest-window` — cheapest N-night window across a whole month (in-window date sweep is handled by `dates` today)
- `drift` — price history per property from local snapshots
- `brand-loyal --program <hyatt|marriott|hilton|ihg|accor>` — search expanded to all sub-brands
- `compare-cities "<city1,city2,...>"` — same nights across multiple cities
- `family --rooms N --kids M` — multi-room search with kid-aware ranking
- `watch <property> --target-price N` — cron-friendly price-drop alert with typed exit codes
- `bundle <city> --nights N --budget X` — best stay under a total budget
- OTA per-source price breakdown — the `prices[]` field is populated when Google's per-result OTA data is present, but inconsistent across results today
- Chain brand auto-detection from hotel names — works for some chains, misses independent properties

Learn more at [Google Hotels](https://www.google.com).

Created by [@kothari-nikunj](https://github.com/kothari-nikunj) (kothari-nikunj).

## Install

The recommended path installs both the `hotel-goat-pp-cli` binary and the `pp-hotel-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install hotel-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/cmd/hotel-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotel-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hotel-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hotel-goat --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hotel-goat --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotel-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/cmd/hotel-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hotel-goat": {
      "command": "hotel-goat-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required. Google Hotels SSR is publicly accessible. Set `Accept-Language` via the system locale; pass `--currency EUR` to get pricing in another currency.

## Quick Start

```bash
# Headline search: location + dates + filters. Returns booking URLs per result.
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --sort cheapest --max-price 300 --min-rating 4.0

# Agent-friendly: brand filter + nested-field selection.
hotel-goat-pp-cli hotels "Paris" 2026-07-20 2026-07-23 --brand Hyatt,Marriott --agent --select results.name,results.price_per_night,results.booking_urls.primary

# Property detail by Google's property_token from a search result.
hotel-goat-pp-cli hotel show <property-token>

# Save properties locally for later reference.
hotel-goat-pp-cli wishlist add <property-token>

# Confirm network reachability to Google Hotels.
hotel-goat-pp-cli doctor

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`hotels`** — Dotted-path selection through nested results.prices[], results.booking_urls, results.images[]. Returns only the fields the agent needs.

  _Use when you need only a couple of fields per hotel rather than the full nested response._

  ```bash
  hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.rating,results.price_per_night,results.booking_urls.primary
  ```

## Recipes

### Cheapest 4-star+ stay in Paris under EUR 300 for a specific weekend

```bash
hotel-goat-pp-cli hotels "Paris" 2026-07-20 2026-07-23 --currency EUR --hotel-class 4,5 --max-price 300 --sort cheapest
```

The default hotels invocation with a price ceiling, star floor, and explicit currency — the most common single-trip query.

### Agent-native: only name + cheapest OTA + booking link, nothing else

```bash
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.rating,results.prices.source,results.prices.price,results.booking_urls.primary
```

Pairs --agent with --select dotted paths to extract only the fields an agent needs — cuts response size ~10x while keeping the per-OTA price breakdown intact.

### Property detail by token

```bash
hotel-goat-pp-cli hotel show <property-token>
```

Full property details (amenities, nearby places, image gallery) from a `property_token` returned by the `hotels` search. The token is the value of `results[*].property_token` in the JSON envelope.

### Save and recall properties

```bash
hotel-goat-pp-cli wishlist add <property-token>
hotel-goat-pp-cli wishlist list
```

Local-SQLite wishlist: save properties from searches, list them later. Survives across sessions; exportable to JSON via `--agent`.

## Usage

Run `hotel-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### hotels

Hotel search results from Google Hotels for a location + date range

- **`hotel-goat-pp-cli hotels`** - Search hotels by location and date range. Returns ~30-40 properties per query
with full OTA price breakdown and booking deep-links.

### properties

Property detail records cached from Google's property detail pages

- **`hotel-goat-pp-cli properties`** - Full property detail by Google's property_token

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17

# JSON for scripting and agents
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --json

# Filter to specific fields
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --json --select results.name,results.price_per_night,results.booking_urls.primary

# Dry run — show the request without sending
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
hotel-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results array on a query that returns hotels in the Google Hotels web UI.** — Set --currency to match your locale (e.g. --currency EUR for European cities) — Google Hotels region-shifts inventory by currency. If still empty, run with --no-cache.
- **HTTP 429 or 403 from Google after rapid back-to-back calls.** — Lower request rate with --rate-limit 0.5 (one request every two seconds), or run from a fresh IP. The scrape uses no anti-bot rotation by design.
- **Brand filter --brand Hyatt misses Andaz / Thompson / Park Hyatt properties.** — Use brand-loyal --program hyatt instead — it expands to all sub-brands via the local brand_aliases table.
- **drift returns 'no snapshots' for a property you've searched before.** — drift keys on property_token, not display name. Run hotel-goat-pp-cli resolve "<name>" to get the token, then drift <token>.
- **Total cost looks wrong for a multi-room family search.** — The hotels command shows per-room price by default. Use the family command instead — it sums across rooms and surfaces the true family budget.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**fast_hotels**](https://github.com/jongan69/hotels) — Python
- [**google-hotels-scraper**](https://github.com/MFori/google-hotels-scraper) — JavaScript
- [**hotels_mcp_server**](https://github.com/esakrissa/hotels_mcp_server) — Python
- [**mcp-booking**](https://github.com/markswendsen-code/mcp-booking) — TypeScript
- [**travel-hacking-toolkit**](https://github.com/borski/travel-hacking-toolkit) — TypeScript
- [**amadeus-node**](https://github.com/amadeus4dev/amadeus-node) — JavaScript
- [**mcp_travelassistant**](https://github.com/skarlekar/mcp_travelassistant) — Python
- [**mcp-server-airbnb**](https://github.com/openbnb-org/mcp-server-airbnb) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
