# Ticketmaster CLI

**Every Discovery v2 endpoint plus offline search, multi-venue watchlists, residency dedup, and on-sale tracking no API call exposes.**

This CLI is the first single-binary tool for the Ticketmaster Discovery API. It absorbs the full read-only surface (events, venues, attractions, classifications, suggest) and adds a local SQLite store with FTS search, named watchlists, residency collapse, tour-view with on-sale flags, and markdown briefs — the workflows real users built scripts to handle.

Learn more at [Ticketmaster](http://developer.ticketmaster.com/support/contact-us/).

Printed by [@omarshahine](https://github.com/omarshahine) (Omar Shahine).

## Install

The recommended path installs both the `ticketmaster-pp-cli` binary and the `pp-ticketmaster` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ticketmaster
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ticketmaster --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketmaster-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ticketmaster --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ticketmaster --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ticketmaster skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ticketmaster. The skill defines how its required CLI can be installed.
```

## Authentication

Authentication is a single Ticketmaster Discovery API consumer key, passed as the `apikey` query parameter on every request. Register at https://developer-acct.ticketmaster.com and copy the Consumer Key from your My Apps dashboard. Set TICKETMASTER_API_KEY in your shell environment. The free tier allows 5000 requests/day at 5/second.

## Quick Start

```bash
# Verifies the API key is set, the Discovery API is reachable, and the local store is initialized.
ticketmaster-pp-cli doctor


# Sync events for a metro (here Seattle-Tacoma DMA 383) into the local SQLite store.
ticketmaster-pp-cli sync --resource events --dma 383 --days 60


# Fan out across multiple venues and return one merged event list.
ticketmaster-pp-cli events upcoming --venues KovZ917Ahkk,KovZpZAFkvEA --days 60 --json


# Collapse Broadway/opera residencies into single rows with first/last date and night count.
ticketmaster-pp-cli events residency --window 28 --json


# Render this week's events as a paste-ready markdown brief.
ticketmaster-pp-cli events brief --dma 383 --window 7

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local-store aggregations
- **`events upcoming`** — Fan out across a venue ID file or list and return one merged, deduplicated, date-sorted event list — the watchlist primitive behind any curated 'what's on at my venues' workflow.

  _When the user has a curated list of venues they care about and wants one merged feed; replaces hand-rolled per-venue fan-out scripts._

  ```bash
  ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json
  ```
- **`events residency`** — Collapse runs of same-name + same-venue events into one row per residency with first_date, last_date, night_count, and id_list — so a 16-night opera season shows as one entry, not 16.

  _When listing upcoming events would otherwise show many near-duplicate rows for Broadway tours, opera seasons, or comedy residencies._

  ```bash
  ticketmaster-pp-cli events residency --window 28 --json
  ```
- **`events by-classification`** — Local join of events × classifications, grouped by segment and genre, with event count and three example events per leaf — the bucketed view newsletter authors and local-scene trackers reach for.

  _When summarizing 'what's on this month' broken down by music vs theatre vs comedy vs sports._

  ```bash
  ticketmaster-pp-cli events by-classification --dma 383 --window 60 --json
  ```
- **`events watchlist`** — Save, list, run, and remove named filter sets (venue IDs, attraction IDs, segments, DMA IDs) that persist across runs in the local SQLite store — the generic primitive any curated 'my venues' workflow composes from.

  _When the same curated venue/artist/genre filter recurs across many queries._

  ```bash
  ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA
  ```
- **`events price-bands`** — Bucket events by priceRanges.min into <$50 / $50-100 / $100-200 / $200+ bands and report count + sample events per band, grouped by classification.

  _When the user wants to know where the affordable shows are this month, or how a venue's pricing skews._

  ```bash
  ticketmaster-pp-cli events price-bands --dma 383 --window 30 --json
  ```

### Tour & on-sale tracking
- **`events tour`** — For a given attraction (artist/team/touring show), return every upcoming event sorted by date, with city, venue, on-sale status, and a flag for events going on-sale within 7 days.

  _When tracking an artist across cities or watching for presale windows._

  ```bash
  ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json
  ```
- **`events on-sale-soon`** — Local query for events whose public on-sale falls in the next N days, sorted ascending — the canonical 'presale watch' view that no API endpoint provides.

  _When the user wants to be alerted to upcoming on-sale dates without polling each artist manually._

  ```bash
  ticketmaster-pp-cli events on-sale-soon --window 7 --classification rock --json
  ```

### Agent-native plumbing
- **`events dedup`** — Read an event JSON array from stdin or the local store, apply a deduplication strategy (name+venue+date, or tour-leg), and write the deduped stream to stdout — composes with any upstream command.

  _When merging results from multiple queries or sources and the duplicates need to be removed before agent processing._

  ```bash
  ticketmaster-pp-cli events list --keyword phish --json | ticketmaster-pp-cli events dedup --strategy tour-leg
  ```
- **`events brief`** — Render a markdown 'what's on' report grouped by night → venue → events with classification labels and price bands, suitable for newsletter, Obsidian, iMessage, or agent context.

  _When the user needs a paste-ready event summary for a chat thread, newsletter, or LLM context._

  ```bash
  ticketmaster-pp-cli events brief --dma 383 --window 7
  ```

## Usage

Run `ticketmaster-pp-cli --help` for the full command reference and flag list.

## Commands

### attractions

Manage attractions

- **`ticketmaster-pp-cli attractions find`** - Find attractions (artists, sports, packages, plays and so on) and filter your search by name, and much more.
- **`ticketmaster-pp-cli attractions get`** - Get details for a specific attraction using the unique identifier for the attraction.

### classifications

Manage classifications

- **`ticketmaster-pp-cli classifications get`** - Get details for a specific segment, genre, or sub-genre using its unique identifier.
- **`ticketmaster-pp-cli classifications get-genre`** - Get details for a specific genre using its unique identifier.
- **`ticketmaster-pp-cli classifications get-segment`** - Get details for a specific segment using its unique identifier.
- **`ticketmaster-pp-cli classifications get-subgenre`** - Get details for a specific sub-genre using its unique identifier.
- **`ticketmaster-pp-cli classifications list`** - Find classifications and filter your search by name, and much more. Classifications help define the nature of attractions and events.

### events

Manage events

- **`ticketmaster-pp-cli events get`** - Get details for a specific event using the unique identifier for the event. This includes the venue and location, the attraction(s), and the Ticketmaster Website URL for purchasing tickets for the event.
- **`ticketmaster-pp-cli events list`** - Find events and filter your search by location, date, availability, and much more.

### suggest

Manage suggest

- **`ticketmaster-pp-cli suggest list`** - Find search suggestions and filter your suggestions by location, source, etc.

### venues

Manage venues

- **`ticketmaster-pp-cli venues get`** - Get details for a specific venue using the unique identifier for the venue.
- **`ticketmaster-pp-cli venues list`** - Find venues and filter your search by name, and much more.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ticketmaster-pp-cli attractions get mock-value

# JSON for scripting and agents
ticketmaster-pp-cli attractions get mock-value --json

# Filter to specific fields
ticketmaster-pp-cli attractions get mock-value --json --select id,name,status

# Dry run — show the request without sending
ticketmaster-pp-cli attractions get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ticketmaster-pp-cli attractions get mock-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ticketmaster -g
```

Then invoke `/pp-ticketmaster <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add ticketmaster ticketmaster-pp-mcp -e TICKETMASTER_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketmaster-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TICKETMASTER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ticketmaster": {
      "command": "ticketmaster-pp-mcp",
      "env": {
        "TICKETMASTER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
ticketmaster-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ticketmaster-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TICKETMASTER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ticketmaster-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TICKETMASTER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 401 / 'Invalid API Key'** — Confirm TICKETMASTER_API_KEY matches the Consumer Key (not Consumer Secret) at developer-acct.ticketmaster.com/products-and-docs/user/me. Re-source your shell.
- **HTTP 429 'Quota exceeded'** — Free tier is 5000 req/day, 5/sec. Run `ticketmaster-pp-cli doctor` to see today's call count, or upgrade the plan.
- **Empty results for a venue you know exists** — Venue may not be Ticketmaster-primary; try a DMA query with classification filter instead. Use `suggest <venue-name>` to discover the canonical venue ID.
- **priceRanges missing on event get** — Resale and dynamic-priced events often omit priceRanges; this is upstream behavior, not a CLI bug.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**delorenj/mcp-server-ticketmaster**](https://github.com/delorenj/mcp-server-ticketmaster) — TypeScript
- [**mochow13/ticketmaster-mcp-server**](https://github.com/mochow13/ticketmaster-mcp-server) — TypeScript
- [**arcward/ticketpy**](https://github.com/arcward/ticketpy) — Python
- [**arcward/picketer**](https://github.com/arcward/picketer) — Go
- [**npm:ticketmaster**](https://www.npmjs.com/package/ticketmaster) — JavaScript
- [**konfig-sdks/ticketmaster-python-sdk**](https://github.com/konfig-sdks/ticketmaster-python-sdk) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
