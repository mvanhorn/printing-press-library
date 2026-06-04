# OfferUp CLI

**Search local OfferUp listings from your terminal, keep them in a local database, and spot underpriced deals before anyone else — no login and no paid key for the things that matter most.**

OfferUp's web scrapers all dump a one-shot listing array and forget it. This CLI persists every listing in a local SQLite store, then layers on price intelligence no marketplace tool offers: price-check tells you the going rate in your area, deals flags listings below the local median, new-since and price-drops track saved searches over time, and seller-scan profiles a seller's whole inventory. Built for agents too — every command speaks --json, --select, and an MCP surface.

Learn more at [OfferUp](https://offerup.com).

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `offerup-pp-cli` binary and the `pp-offerup` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install offerup
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install offerup --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install offerup --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install offerup --agent claude-code
npx -y @mvanhorn/printing-press-library install offerup --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/offerup/cmd/offerup-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/offerup-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-offerup --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-offerup --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install offerup --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/offerup-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/offerup/cmd/offerup-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "offerup": {
      "command": "offerup-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public commands need no login: search, item detail (listings get), seller lookup, category-scoped browse, and every price-intelligence command (price-check, deals, new-since, price-drops, digest, seller-scan). Set location with --zip (or --lat/--lon), not a credential. Account commands act on your own OfferUp account and require login: account (your profile), my-listings (plus archived, mark-sold, archive), saved, and messages (plus messages read). Auth is your OfferUp web session cookie. Capture it once with `auth login --chrome`, which extracts the cookie straight from your logged-in browser — no extra install needed. The optional `press-auth` companion adds a smoother one-click controlled-Chrome capture and encrypts the session at rest, but it is not required. You can also paste the cookie via the `OFFERUP_COOKIE` environment variable or `auth set-token`. Check with `auth status`, clear with `auth logout`. The mutating commands my-listings mark-sold and my-listings archive preview by default and apply only with --confirm. Creating a new listing is not supported — OfferUp's web app posts Jobs only, so listing creation is mobile-app-only.

## Quick Start

```bash
# Health check — confirms the CLI can reach OfferUp; needs no auth.
offerup-pp-cli doctor --dry-run

# Search live listings near a ZIP with no login.
offerup-pp-cli listings search "dewalt drill" --zip 98101 --limit 20

# See the local going rate; this fetches and stores listings itself, no separate sync needed.
offerup-pp-cli price-check "dewalt drill" --zip 98101

# Flag listings at least 25% under the local median.
offerup-pp-cli deals "dewalt drill" --zip 98101 --below 25

```

## Known Gaps

- **Cross-run intelligence is empty on the first run.** `price-drops`, `new-since`, and `digest` compare each search against the listing history stored locally for that query, so the first run for a new query returns no drops or new items — by design. Run the same query again later (the commands re-fetch and re-record on every run) and the second run onward will show changes within the `--since` window.
- **`seller-scan` reads only locally-known inventory.** OfferUp has no public endpoint that lists a seller's full catalog, so `seller-scan <seller-id>` returns an empty inventory until you run `offerup-pp-cli listings get <listing-id>` on that seller's items. The inventory grows as you fetch more of their listings.
- **Posting a new listing is not supported.** OfferUp's web app only allows posting Jobs; creating a marketplace listing is mobile-app-only, so there is no `listing create`. The CLI covers managing existing listings (`mark-sold`, `archive`), not creating them.
- **Account commands need a captured OfferUp session.** `account` / `my-listings` / `saved` / `messages` require login — run `auth login --chrome` (extracts the cookie from your browser, no extra install), set `OFFERUP_COOKIE`, or `auth set-token`. The optional `press-auth` companion gives a one-click capture but is not required. The public commands need none of this.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local price intelligence
- **`price-check`** — See the real going rate for an item in your area — median, 25th/75th percentile, min, max, and the firm-vs-negotiable split — computed across every listing you've pulled.

  _Reach for this before buying or pricing a resale — it answers "what is this actually worth here" with a number instead of a scroll._

  ```bash
  offerup-pp-cli price-check "herman miller aeron" --zip 85001 --agent
  ```
- **`deals`** — Surface listings priced a chosen percentage below the local median for that item — the underpriced finds, ranked by how far under they are.

  _The deal-sniping command: it tells an agent which listings are underpriced right now, not just what exists._

  ```bash
  offerup-pp-cli deals "dewalt drill" --zip 98101 --below 25 --agent
  ```
- **`price-drops`** — Detect listings whose price fell between syncs — the same item, now cheaper — sorted by the size of the drop.

  _Catches sellers who just cut a price on something you're tracking — the moment to make an offer._

  ```bash
  offerup-pp-cli price-drops "macbook pro" --since 7d --agent
  ```

### Track saved searches
- **`new-since`** — Show only the listings that appeared since a cutoff for a saved search, so you never re-scan items you already saw.

  _Use this for a recurring watch — it answers "what dropped since I last looked" in one call._

  ```bash
  offerup-pp-cli new-since "road bike" --since 24h --agent
  ```
- **`digest`** — A single composite report for a saved search combining what's new, what dropped in price, and what's underpriced.

  _The morning-ritual command and the natural single MCP tool call for an agent watching a market._

  ```bash
  offerup-pp-cli digest "snowboard" --since 24h --agent
  ```

### Seller intelligence
- **`seller-scan`** — Pull a seller's full synced inventory alongside their reputation badges (business/dealer/TruYou), join date, and the median asking price across their listings.

  _Vet a dealer before buying to flip, or watch a high-volume seller's pricing in one view._

  ```bash
  offerup-pp-cli seller-scan 161842229 --agent
  ```

## Recipes

### Narrow a verbose search for an agent

```bash
offerup-pp-cli listings search "macbook pro" --zip 98101 --agent --select listingId,title,price,locationName,conditionText
```

Listing payloads are wide; --agent with --select returns only the fields an agent needs, saving context.

### Find this week's underpriced finds

```bash
offerup-pp-cli deals "aeron chair" --zip 85001 --below 30 --agent
```

Lists Aeron chairs priced at least 30% below the local median around Phoenix.

### Daily watch on a saved search

```bash
offerup-pp-cli digest "road bike" --since 24h --agent
```

One call returns what's new, what dropped in price, and what's a deal for road bikes since yesterday.

### Vet a seller before buying to flip

```bash
offerup-pp-cli seller-scan 161842229 --agent
```

Shows the seller's badges, join date, full inventory, and median asking price in one view.

## Usage

Run `offerup-pp-cli --help` for the full command reference and flag list.

## Commands

### listings

Search and view OfferUp listings (public, no login)

- **`offerup-pp-cli listings get`** - Get the full detail for one listing
- **`offerup-pp-cli listings search`** - Search live OfferUp listings by keyword and location

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
offerup-pp-cli listings get mock-value

# JSON for scripting and agents
offerup-pp-cli listings get mock-value --json

# Filter to specific fields
offerup-pp-cli listings get mock-value --json --select id,name,status

# Dry run — show the request without sending
offerup-pp-cli listings get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
offerup-pp-cli listings get mock-value --agent
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
offerup-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/offerup-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results for a common item** — Widen the area with a larger --radius or a different --zip, or drop extra keywords from the query.
- **Requests slow down or start failing in bursts** — OfferUp throttles rapid requests; lower --limit/--max-pages and let the built-in pacing back off — the CLI rate-limits per request.
- **Results are for the wrong city** — Location defaults to IP geo; pass --zip (or --lat/--lon) explicitly to scope to your area.
- **price-check or deals returns nothing** — Run sync for that query first — the price commands read the local store, not the live feed.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**pyOfferUp**](https://github.com/oscar0812/pyOfferUp) — Python
- [**planetzero/offerup**](https://github.com/planetzero/offerup) — JavaScript
- [**OfferupUnofficalAPI**](https://github.com/everettperiman/OfferupUnofficalAPI) — Python
- [**unofficial-offerup-api**](https://github.com/everettcaldwell/unofficial-offerup-api) — Python
- [**gs-scraper**](https://github.com/jgdigitaljedi/gs-scraper) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
