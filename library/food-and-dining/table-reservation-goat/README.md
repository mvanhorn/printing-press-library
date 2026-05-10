# Table Reservation GOAT CLI

**One reservation CLI for OpenTable and Tock — search both networks at once, watch for cancellations, book, and track changes from a local store agents can query.**

OpenTable and Tock split the US fine-dining world between them and share zero data. This CLI unifies them: `goat` searches both at once, `watch` polls both for cancellations, `earliest` composes availability across both, and `drift` surfaces what changed at a venue since your last look. Auth is one `auth login --chrome` import — your real Chrome cookies for both sites, no partner keys.

## Install

The recommended path installs both the `table-reservation-goat-pp-cli` binary and the `pp-table-reservation-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install table-reservation-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install table-reservation-goat --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/table-reservation-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-table-reservation-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-table-reservation-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-table-reservation-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-table-reservation-goat. The skill defines how its required CLI can be installed.
```

## Authentication

Anonymous reads (search, restaurant detail, availability) work out of the box via Surf with a Chrome TLS fingerprint that clears Akamai (OpenTable) and Cloudflare (Tock). For richer data — anything that requires being signed in to OpenTable or Tock — run `auth login --chrome` once to import your already-logged-in cookies from your local Chrome profile. There are no API keys to manage; the OpenTable Partner API is out of scope.

## Quick Start

```bash
# import cookies from your Chrome profile so authenticated commands work
table-reservation-goat-pp-cli auth login --chrome


# populate the local SQLite store from both networks (restaurants, availability)
table-reservation-goat-pp-cli sync --full


# headline command — single ranked list across both networks
table-reservation-goat-pp-cli goat 'omakase manhattan' --party 2 --when 'fri 7-9pm' --agent


# set up a cancellation watch and let the printer poll both networks adaptively
table-reservation-goat-pp-cli watch add 'alinea' --party 2 --window 'sat 7-9pm' --notify local


# soonest open slot per venue across both networks
table-reservation-goat-pp-cli earliest 'le-bernardin,atomix,smyth,alinea' --party 2 --within 14d --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-network ground truth
- **`goat`** — One query across OpenTable and Tock simultaneously, ranked by relevance, earliest availability, and price band.

  _When a user asks an agent to find a table, this is the single command that searches both reservation networks and returns structured ranked results — agents do not need to know which network covers which restaurant._

  ```bash
  table-reservation-goat-pp-cli goat 'tasting menu chicago' --party 2 --when 'this weekend' --agent --select results.name,results.network,results.earliest_slot,results.price_band
  ```
- **`earliest`** — Across a list of restaurants from either network, return the earliest open slot per venue within a time horizon.

  _When a user gives an agent a shortlist of venues and wants the soonest opportunity, this is the right shape — one structured response with one row per venue across both networks._

  ```bash
  table-reservation-goat-pp-cli earliest 'alinea,le-bernardin,smyth,atomix' --party 4 --within 21d --agent --select earliest.venue,earliest.network,earliest.slot_at,earliest.attributes
  ```

### Local state that compounds
- **`watch`** — Persistent local watcher that polls both networks for openings on your target venues and party size, with notifications and optional auto-book.

  _Resy's Notify covers Resy only; tockstalk covers Tock only; restaurant-mcp's snipe covers Resy+OT only. None covers both networks; none persists state. Use this when an agent or user needs a hot reservation that isn't currently available._

  ```bash
  table-reservation-goat-pp-cli watch add 'le-bernardin' --party 2 --window 'Fri 7-9pm' --notify slack
  ```
- **`drift`** — Show what changed at a specific venue since the last sync — new experiences, slot price moves, hours changes.

  _Hot-target deep-watch: when an agent or user is hunting one venue, drift surfaces every meaningful change since the last look._

  ```bash
  table-reservation-goat-pp-cli drift alinea --since '2026-04-01' --agent
  ```

- **`book`** — Place a reservation on OpenTable or Tock (v0.2). Free reservations only; payment-required venues return a typed `payment_required` error pointing at v0.3. Two safety layers: `PRINTING_PRESS_VERIFY=1` short-circuits to dry-run regardless (verifier mock-mode floor), and live commit fires only when `TRG_ALLOW_BOOK=1` is set. Without the env var, returns a dry-run envelope with a hint. Idempotency pre-flight via `ListUpcomingReservations` + normalized matching prevents double-book on retry; filesystem advisory lock keyed on `(network, slug, date, time, party)` prevents concurrent double-book across processes.

  ```bash
  TRG_ALLOW_BOOK=1 table-reservation-goat-pp-cli book opentable:water-grill-bellevue --date 2026-05-13 --time 19:00 --party 2 --agent
  ```

  Tock book uses chromedp-attach (drives a real Chrome session) since Tock's book flow uses traditional form-submit + Braintree CSRF. Card-required venues (most non-prepay Tock restaurants) prompt for CVC on stderr; the value flows through to the browser at confirm time. Free venues skip the CVC prompt. Requires Chrome running with `--remote-debugging-port=9222`, OR the CLI spawns a stealth headless Chrome as fallback. CVC can also be set via `TRG_TOCK_CVC` env var for non-interactive usage (MCP tool calls).

- **`cancel`** — Cancel a reservation. NOT gated by `TRG_ALLOW_BOOK` (recovery action) but still respects the `PRINTING_PRESS_VERIFY` floor. Compound argument shape:
  - OpenTable: `cancel opentable:<restaurantId>:<confirmationNumber>:<securityToken>`
  - Tock:      `cancel tock:<venueSlug>:<purchaseId>`

  All compound parts are returned by the corresponding `book` command's JSON output.

  ```bash
  table-reservation-goat-pp-cli cancel opentable:1255093:114309:01Ozsdas9H1Yx --agent
  ```

## Usage

Run `table-reservation-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### availability

Check open reservation slots across OpenTable and Tock

- **`table-reservation-goat-pp-cli availability check`** - Check open slots for a restaurant on a specific date and party size
- **`table-reservation-goat-pp-cli availability multi-day`** - Multi-day availability for a single restaurant — Mon-Sun matrix

### restaurants

Search and inspect restaurants across OpenTable and Tock

- **`table-reservation-goat-pp-cli restaurants get`** - Get a restaurant's full detail — hours, address, cuisine, price band, photos, accolades
- **`table-reservation-goat-pp-cli restaurants list`** - List restaurants across OpenTable and Tock; filter by location, cuisine, price band, accolades, and party size

### watch

Persistent local cancellation watcher across both networks

- **`table-reservation-goat-pp-cli watch add`** - Register a watch for a venue, party size, and time window
- **`table-reservation-goat-pp-cli watch list`** - List active watches
- **`table-reservation-goat-pp-cli watch cancel`** - Cancel a watch by id
- **`table-reservation-goat-pp-cli watch tick`** - Run one polling tick across all active watches (for cron / agents)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
table-reservation-goat-pp-cli restaurants list

# JSON for scripting and agents
table-reservation-goat-pp-cli restaurants list --json

# Filter to specific fields
table-reservation-goat-pp-cli restaurants list --json --select id,name,neighborhood

# Dry run — show the request without sending
table-reservation-goat-pp-cli restaurants list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
table-reservation-goat-pp-cli restaurants list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-table-reservation-goat -g
```

Then invoke `/pp-table-reservation-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add table-reservation-goat table-reservation-goat-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/table-reservation-goat-current).
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
    "table-reservation-goat": {
      "command": "table-reservation-goat-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
table-reservation-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/table-reservation-goat-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

### Power-user knobs (env vars)

OpenTable's WAF can rate-limit aggressive scans. The CLI ships with a disk cache, singleflight dedupe, and an AdaptiveLimiter so typical use never hits the limit. These env vars override the defaults:

| Env var | Default | Effect |
|---|---|---|
| `TRG_OT_CACHE_TTL` | `3m` | How long a cached availability response stays fresh. Range `[1m, 24h]`; out-of-range falls back to default with a stderr warning. |
| `TRG_OT_THROTTLE_RATE` | `0.5` | Initial calls/second for the OT AdaptiveLimiter. Lower values pace harder (`0.1` = 10s spacing); higher values are appropriate when routing through a personal proxy. Range `[0.01, 5.0]`. |
| `TRG_OT_NO_CACHE` | unset | Set to `1` to bypass the cache by default. The `--no-cache` flag on `earliest` and `watch tick` does the same per-call. |
| `TRG_ALLOW_BOOK` | unset | Live commit gate for `book`. Without it, `book` returns a dry-run envelope. `cancel` is NOT gated by this — it's a recovery action. |
| `PRINTING_PRESS_VERIFY` | unset | Verifier-mode floor. When `=1`, both `book` and `cancel` short-circuit to dry-run regardless of `TRG_ALLOW_BOOK`. Set automatically by `printing-press verify` mock-mode subprocesses. |
| `TRG_TOCK_CVC` | unset | When set, used as the CVC for Tock card-required bookings instead of prompting on stderr. Useful for MCP tool calls and other non-interactive contexts. |
| `TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL` | `http://localhost:9222` | Override for the Chrome DevTools endpoint used by the Tock chromedp-attach book flow. |
| `HTTPS_PROXY` / `HTTP_PROXY` | unset | Standard Go-honored proxy URLs. Useful for routing OT traffic through a personal proxy or Tor SOCKS5 (`socks5://localhost:9050`). |

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`PersistedQueryNotFound` 400 from OpenTable on first run** — the persisted-query hash drifted; run `table-reservation-goat-pp-cli doctor --refresh-hashes` to bootstrap the current hash from a fresh homepage fetch
- **Cloudflare challenge from exploretock.com** — Surf transport with Chrome TLS clears this automatically; if you see a 403, run `table-reservation-goat-pp-cli doctor` to verify the Surf fingerprint is loaded
- **`Authentication required` on a venue or detail call that needs sign-in** — run `table-reservation-goat-pp-cli auth login --chrome` to import cookies, then `auth status` to confirm both networks are signed in
- **Empty availability results for a venue you know has openings** — check `--party` and `--time` (Tock returns empty when no slot matches the seating area filter); also try `goat <venue> --debug` to see the per-network response
- **Watch never fires even though slots opened on the website** — verify `watch list --json` shows your watch `state: active` and `last_polled_at` recent; if the limiter is throttled the typed `RateLimitError` will be in the recent log — increase `--cadence` to back off

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**azoff/tockstalk**](https://github.com/azoff/tockstalk) — TypeScript (43 stars)
- [**jrklein343-svg/restaurant-mcp**](https://github.com/jrklein343-svg/restaurant-mcp) — TypeScript
- [**21Bruce/resolved-bot**](https://github.com/21Bruce/resolved-bot) — Go
- [**duaragha/opentable-mcp**](https://github.com/duaragha/opentable-mcp) — TypeScript
- [**bedheadprogrammer/reservationserver**](https://github.com/bedheadprogrammer/reservationserver) — TypeScript
- [**singlepatient/tablehog**](https://github.com/singlepatient/tablehog) — Rust
- [**spudtrooper/opentable**](https://github.com/spudtrooper/opentable) — Go
- [**Henrymarks1/Open-Table-Bot**](https://github.com/Henrymarks1/Open-Table-Bot) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
