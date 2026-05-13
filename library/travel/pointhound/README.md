# Pointhound CLI

**Every Pointhound flight search, plus a local SQLite of every deal you've ever seen, balance-aware reachability, and drift detection no other award-search tool has.**

Pointhound's web search is great for one-off lookups but doesn't compound. This CLI lets you batch search 20 routes overnight, ask 'where can I go with the points I actually have' in one call, watch a route and exit-code-2 when a new deal appears, and pivot every snapshot through agent-native --json output.

Learn more at [Pointhound](https://www.pointhound.com).

## Install

The recommended path installs both the `pointhound-pp-cli` binary and the `pp-pointhound` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install pointhound
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install pointhound --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pointhound-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pointhound --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pointhound --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pointhound skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pointhound. The skill defines how its required CLI can be installed.
```

## Authentication

Most of the CLI works anonymously — flight offer reads, filter facets, airport autocomplete, credit-card catalog. Only the `top-deals-matrix` and `search` commands need authentication, which happens via `pointhound-pp-cli auth login --chrome` — the CLI reads your existing Pointhound login cookies from Chrome (cf_clearance + ph_session) so no separate token is needed.

## Quick Start

```bash
# Find airport codes with Pointhound's deal-aware autocomplete (no auth).
pointhound-pp-cli airports SFO --agent


# Read the offers for a search session you obtained from the website (anonymous).
pointhound-pp-cli offers list --search-id ofs_xxx --cabins business --sort-by points --json --select id,pricePoints,airlinesList,totalDuration,totalStops


# Optional: import Chrome cookies to unlock search creation and top-deals matrix.
pointhound-pp-cli doctor


# The headline novel command: where can I go in business this October?
pointhound-pp-cli from-home SFO --balance "ur:250000,mr:80000" --search-ids ofs_xxx --cabin business --month 2026-10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`from-home`** — Tell me where I can fly with the points I actually hold — feed in your Chase UR, Amex MR, Bilt, Capital One, and Citi TY balances and get back every destination reachable in the requested cabin within those balances, ranked by lowest effective spend.

  _When an agent has a goal like 'plan a fall trip' and a balance fact like 'user has 450k transferable points', this command answers the multi-step optimization in one call instead of N searches._

  ```bash
  pointhound-pp-cli from-home SFO --balance "ur:250000,mr:80000,bilt:120000" --cabin business --month 2026-10 --agent
  ```
- **`compare-transfer`** — Given a transferable points program (Chase UR, Amex MR, etc.) and a route, list every redemption ranked by source-program points spent — multiplying each offer's price by the real transfer ratio (1:1 instant for UR→United vs 0.333:1 up_to_72 for Marriott→United).

  _Agents reasoning about 'cheapest redemption' should ask in user-input units, not airline-output units. This command does the math._

  ```bash
  pointhound-pp-cli compare-transfer chase-ultimate-rewards --search-id ofs_xxx --json
  ```
- **`batch`** — Issue N route+date searches in parallel from a CSV file (or repeated --route flags); all results are snapshotted to the local SQLite store with throttling.

  _Multi-search is the common case for travel planning; web UIs only do one search at a time._

  ```bash
  pointhound-pp-cli batch --search-ids-file ~/routes.txt --throttle 1s --cabin business --json
  ```
- **`top-deals-matrix`** — Submit a multi-origin × multi-destination × month-range matrix search (e.g. SFO,LAX → LIS,FCO,LHR across Oct-Dec) and snapshot every result. Mirrors Pointhound's Premium Top Deals product but with offline access and cabin filtering.

  _Travel agents and trip planners can ask 'best Europe deal this fall?' in one shot._

  ```bash
  pointhound-pp-cli top-deals-matrix --origins SFO,LAX --dests LIS,FCO,LHR --months 2026-10,2026-11,2026-12 --cabin business
  ```
- **`drift`** — For a watched route, diff the latest snapshot against the previous and show per-offer status: new, cheaper, disappeared, unchanged. Includes the points delta and timestamp gap.

  _Answers 'did anything change?' in one terse output, which is what an agent or human re-checker actually wants._

  ```bash
  pointhound-pp-cli drift SFO LIS 2026-08-15 --since yesterday --json
  ```
- **`calendar`** — For a route + cabin, batch-search every month over a 12-month window and produce a month-grid showing min points cost per month (and the offer that achieved it).

  _Trip-planning agents need a month picker, not a date picker, when the user says 'sometime next year'._

  ```bash
  pointhound-pp-cli calendar --search-ids ofs_a,ofs_b,ofs_c --cabin business --json
  ```

### Agent-native plumbing
- **`watch`** — Register a route as a saved watch; subsequent runs poll Pointhound and exit with code 2 only when a new or cheaper deal appears since the last snapshot. Perfect for cron.

  _The agent-native equivalent of 'tell me when something changes' — exit-code-driven, suitable for any scheduler._

  ```bash
  pointhound-pp-cli watch SFO LIS 2026-08-15 --cabin business --quiet && say 'new deal'
  ```

### Service-specific patterns
- **`explore-deal-rating`** — Use Pointhound's `scout.pointhound.com/places/search` `dealRating` and `isTracked` fields to discover airports near a metro that historically have high-frequency deals, optionally chaining into `batch` to fetch live offers for them.

  _Lets an agent narrow the search space before fan-out: 'find me cheap deals from somewhere near NYC' becomes one command, not three._

  ```bash
  pointhound-pp-cli explore-deal-rating --metro NYC --min-rating high --limit 5 --agent
  ```
- **`transferable-sources`** — Given an airline redeem program (e.g. United MileagePlus), list every transferable earn program that feeds it with the ratio and transfer time (instant vs up_to_72).

  _Quick lookup for 'can I get to United via Capital One?' without remembering the table._

  ```bash
  pointhound-pp-cli transferable-sources united-mileageplus --json
  ```

## Usage

Run `pointhound-pp-cli --help` for the full command reference and flag list.

## Commands

### offers

Flight offers returned for a search session

- **`pointhound-pp-cli offers filter_options`** - Get the filterable facets (card programs, airline programs, airlines) available for a given search session.
- **`pointhound-pp-cli offers list`** - List flight offers for an existing search session, with optional filters and sort.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pointhound-pp-cli offers list --search-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-pointhound -g
```

Then invoke `/pp-pointhound <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
pointhound-pp-cli doctor

claude mcp add pointhound pointhound-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
pointhound-pp-cli doctor
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pointhound-current).
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
    "pointhound": {
      "command": "pointhound-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
pointhound-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pointhound-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pointhound-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`pointhound-pp-cli search` returns 'cookie required'** — Run `pointhound-pp-cli auth login --chrome` to import Pointhound cookies from your Chrome profile.
- **`offers list` returns empty or 400** — The search session expired. Get a fresh `ofs_*` from the Pointhound website URL, or run `pointhound-pp-cli search` after `auth login --chrome`.
- **`from-home` returns no results** — Run `pointhound-pp-cli sync` to populate the local store with transferOptions; without it, balance math has no ratios to use.
- **Rate limiting / 429 on batch** — Increase `--throttle` (default 1s) and re-run; batch uses adaptive backoff but will surface a typed rate-limit error if exhausted.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.pointhound.com
- Capture coverage: 30 API entries from 134 total network entries
- Reachability: standard_http (90% confidence)
- Protocols: rest_json (95% confidence)
- Auth signals: none; cookie — cookies: cf_clearance, ph_session
- Protection signals: cloudflare (95% confidence)
- Generation hints: primary_base_url=https://www.pointhound.com, auth_type=cookie, anonymous_read_endpoints, cross_domain_novel_commands=scout.pointhound.com,db.pointhound.com, search_create_blocked_by_cloudflare_requires_cookie_replay
- Candidate command ideas: — Primary read endpoint; verified replayable anonymously.; — Filter facets for a given search session; verified replayable anonymously.; — Airport/city autocomplete with deal-aware ranking (cross-domain — hand-written novel command).

Warnings from discovery:
- :
- :

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
