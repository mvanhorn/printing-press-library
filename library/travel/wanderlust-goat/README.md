# Wanderlust GOAT CLI

**What a knowledgeable local with great taste would tell you to walk to from here.**

Two-stage funnel: seed candidates from Google Places, then deep-research each against locale-aware sources (Tabelog/Naver/Le Fooding for the country you're in), trust-weight by source authority, kill-gate anything that's permanently closed, and return the 3-5 amazing things — not the comprehensive 40-row dump.

## Install

The recommended path installs both the `wanderlust-goat-pp-cli` binary and the `pp-wanderlust-goat` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install wanderlust-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install wanderlust-goat --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wanderlust-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wanderlust-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wanderlust-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-wanderlust-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-wanderlust-goat. The skill defines how its required CLI can be installed.
```

## Authentication

GOOGLE_PLACES_API_KEY is required for live Stage-1 seeding. Get a key at https://developers.google.com/maps/documentation/places/web-service/get-api-key (free $200/mo credit). ANTHROPIC_API_KEY is optional and enables sharper criteria judgment via --llm; without it, the free heuristic path runs.

## Quick Start

```bash
# verify Nominatim/OSRM reachable and required env vars set
wanderlust-goat-pp-cli doctor


# the headline workflow: identity + criteria + walking radius
wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten with no tourists" --identity "coffee snob into 70s Japanese kissaten culture" --minutes 15


# no-LLM path; same shape, free, deterministic
wanderlust-goat-pp-cli goat 35.6895,139.6917 --criteria "high-end seafood with counter seating" --minutes 12


# audit a ranking — every source, weight, and contribution
wanderlust-goat-pp-cli why "Bear Pond Espresso" --json


# is this place still open? cross-source verdict
wanderlust-goat-pp-cli status "Sushi Saito" --json


# pre-cache before the trip — offline-ready
wanderlust-goat-pp-cli sync-city "Tokyo" --country JP

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Persona-shaped fanout
- **`near`** — Find the 3-5 amazing things within walking distance that match your stated identity and criteria — not the 40 closest things.

  _When an agent needs the curated picks for a persona at a location, this is the single command that fuses ~12 sources into one ranked, sourced answer._

  ```bash
  wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten, no tourists, great pour-over" --identity "coffee snob, into 70s Japanese kissaten culture" --minutes 15 --agent
  ```
- **`goat`** — Same fanout as `near` but with no LLM in the runtime path — criteria-to-source mapping uses static lookup tables so the CLI works standalone.

  _Agents and humans both need a GOAT mode that works without an LLM caller — useful for shell pipelines, cron, and offline runs._

  ```bash
  wanderlust-goat-pp-cli goat "35.6895,139.6917" --criteria "vintage clothing, vinyl, hidden" --minutes 20 --agent
  ```

### Agent-orchestration plumbing
- **`research-plan`** — Output a JSON query plan agents execute in a loop — typed, country-aware, ordered by trust, ready to fan out.

  _Drop this into an agent loop to let the agent run multi-source travel research without re-deriving the fanout plan every call._

  ```bash
  wanderlust-goat-pp-cli research-plan "hand-pulled noodles, locals only" --anchor "Bukchon Hanok Village, Seoul" --country KR --json
  ```

### Cross-source walks
- **`crossover`** — Find pairs where a high-trust restaurant sits within 200m of a Wikipedia-notable historic site or Atlas Obscura entry — food + culture in one walk.

  _When the persona wants 'a great meal next to something interesting', this is the spatial query that compounds two layers._

  ```bash
  wanderlust-goat-pp-cli crossover --anchor "Marais, Paris" --radius 800m --pair food+culture --agent
  ```
- **`golden-hour`** — Compute sunrise/sunset/blue-hour locally (pure Go, no API) and pair with viewpoints photographers know about within walking distance.

  _When an agent needs to brief Felix the photographer for tonight's shoot, this is the one call that fuses the math and the spots._

  ```bash
  wanderlust-goat-pp-cli golden-hour "Eiffel Tower" --date 2026-06-15 --minutes 20 --agent
  ```
- **`route-view`** — Walking polyline from A to B, then everything interesting along the path — not just at the endpoints.

  _For walks where the journey IS the point, the agent needs everything along the path — not the closest thing to either end._

  ```bash
  wanderlust-goat-pp-cli route-view "Shibuya Station, Tokyo" "Yoyogi Park, Tokyo" --buffer 150m --agent
  ```
- **`quiet-hour`** — Places that locals describe as quiet at the requested time, intersected with OSM opening hours and walking radius.

  _Agents helping someone find the un-crowded version of a popular cafe need the Reddit-quiet-signal layer the persona always asks for but never gets._

  ```bash
  wanderlust-goat-pp-cli quiet-hour "Yurakucho, Tokyo" --minutes 15 --day mon --time 14:00 --agent
  ```

### Local store + sync
- **`sync-city`** — Pre-cache editorial best-of, Reddit threads, Wikipedia, Wikivoyage, OSM POIs, Atlas Obscura, and regional-language sources for offline use.

  _Agents working offline or with flaky connectivity need a synced local store; this populates it._

  ```bash
  wanderlust-goat-pp-cli sync-city "Tokyo" --country JP --json
  ```
- **`why`** — Print every source that mentioned a place, the trust weight, country boost, walking time, criteria match, and the final goat-score breakdown.

  _When the agent's pick surprises the user, this command answers 'why was this ranked #1?' in one call._

  ```bash
  wanderlust-goat-pp-cli why "珈琲 美美" --json
  ```
- **`reddit-quotes`** — Surface the highest-scored Reddit comment snippets that mention a place — verbatim quotes, no LLM summarization.

  _Agents giving travel advice need the actual local quotes, not a summary that can hallucinate. This returns the raw text with provenance._

  ```bash
  wanderlust-goat-pp-cli reddit-quotes "Kohi Bibi" --json
  ```
- **`coverage`** — Per-tier row counts, last-sync ages, country-match boost, and which v1 sources are missing for a synced city.

  _Before an agent trusts a `near` answer, it should check whether the local store actually has the layers it claims to fuse._

  ```bash
  wanderlust-goat-pp-cli coverage tokyo --json
  ```

## Usage

Run `wanderlust-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### places

Geocode addresses and look up canonical place coordinates via Nominatim (anchor-resolution layer for the two-stage GOAT funnel).

- **`wanderlust-goat-pp-cli places reverse`** - Reverse geocode lat/lng to a structured address.
- **`wanderlust-goat-pp-cli places search`** - Forward geocode an address, place name, or business to lat/lng candidates.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wanderlust-goat-pp-cli places search --query example-value

# JSON for scripting and agents
wanderlust-goat-pp-cli places search --query example-value --json

# Filter to specific fields
wanderlust-goat-pp-cli places search --query example-value --json --select id,name,status

# Dry run — show the request without sending
wanderlust-goat-pp-cli places search --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wanderlust-goat-pp-cli places search --query example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-wanderlust-goat -g
```

Then invoke `/pp-wanderlust-goat <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-mcp@latest
```

Then register it:

```bash
claude mcp add wanderlust-goat wanderlust-goat-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wanderlust-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/cmd/wanderlust-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wanderlust-goat": {
      "command": "wanderlust-goat-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
wanderlust-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/wanderlust-goat-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **exit code 4: Google Places auth missing** — export GOOGLE_PLACES_API_KEY=<your-key>; get one at https://developers.google.com/maps/documentation/places/web-service/get-api-key
- **near returns 0 results in country X** — country X may not have a Stage-2 region row yet — run `coverage <city> --json` to see the active region; add a row to internal/regions/regions.go to extend
- **Atlas Obscura returns nothing for my city** — Atlas Obscura's slug index doesn't cover every city; this is expected, the dispatcher falls through silently
- **Tabelog/Naver/Le Fooding returns nothing for a place that exists** — Stage-2 lookup is by name; the source's anti-bot mitigation may have intercepted. Run with --verbose to see per-source HTTP status; rate-limit backoff is per-source

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
