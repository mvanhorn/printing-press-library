# Wanderlust GOAT CLI

**What a knowledgeable local with great taste would tell you to walk to from here — fused across editorial, local-language, and crowd layers no single tool ranks together.**

Most 'near me' tools return the 40 closest results. Wanderlust GOAT returns the 3 results that match your stated identity and criteria. It fuses Nominatim, OSRM walking time, OSM Overpass, Wikipedia, Wikivoyage, Atlas Obscura, Reddit, editorial scrapes (Eater, Time Out, NYT 36 Hours, Michelin), and language-aware regional sources (Tabelog, Naver, Le Fooding) through one trust-weighted score, with local-language names preserved alongside transliterations. Free, no API keys, with an offline SQLite store and a JSON `research-plan` surface for agent orchestration.

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

wanderlust-goat is intentionally key-less. Every v1 source is a free public API or a politely-rate-limited public scrape. OSRM uses the project-osrm.org public demo; the README documents how to point at a self-hosted OSRM. Set a contact-bearing User-Agent via `WANDERLUST_GOAT_UA` env var (Nominatim policy requires a real contact URL or email).

## Quick Start

```bash
# Pre-cache Tokyo's editorial + Reddit + Wikipedia + OSM + Tabelog layers into the local SQLite store.
wanderlust-goat-pp-cli sync-city tokyo --layers all


# Persona-shaped 15-minute walk: 3-5 ranked picks with local-language names and one-line 'why it's special'.
wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten, no tourists, great pour-over" --identity "coffee snob, into 70s Japanese kissaten culture" --minutes 15


# Emit the typed JSON query plan an agent should execute — narrowed to client and source list.
wanderlust-goat-pp-cli research-plan "Bukchon Hanok Village" --criteria "hand-pulled noodles, locals only" --json --select sources,calls.client


# Tonight's blue-hour windows + the photographer-known viewpoints inside a 20-minute walking radius.
wanderlust-goat-pp-cli golden-hour "Eiffel Tower" --date 2026-06-15 --minutes 20


# Audit the goat-score breakdown for any place the persona is curious about.
wanderlust-goat-pp-cli why "珈琲 美美"

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
  wanderlust-goat-pp-cli research-plan "Bukchon Hanok Village, Seoul" --criteria "hand-pulled noodles, locals only" --identity "food traveler" --json
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
  wanderlust-goat-pp-cli sync-city tokyo --layers all --agent
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

Geocode addresses and look up canonical place coordinates via Nominatim (foundation layer for the multi-source GOAT stack).

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

- **Nominatim returns 403** — Set WANDERLUST_GOAT_UA to a string with your real contact URL or email — Nominatim blocks placeholder UAs (no `example.com`).
- **Tabelog or Michelin returns 202/AWS-WAF challenge** — These sources use Surf (Chrome TLS fingerprint) by default; if a corporate proxy strips TLS extensions, run `wanderlust-goat-pp-cli doctor` to confirm Surf transport.
- **Atlas Obscura entries empty** — AO sometimes adds Cloudflare gates regionally; rerun with `--data-source local` to use cached entries from the last `sync-city` pass.
- **OSRM public demo is slow or 5xx** — Set WANDERLUST_GOAT_OSRM_BASE_URL to a self-hosted OSRM endpoint; instructions in README under 'Self-host OSRM'.
- **near returns 40 results instead of 3-5** — Add a stronger --criteria — narrow phrasing forces persona-shaped scoring. Add --identity for a +0.05 trust boost on local-language sources.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Mapbox MCP Server**](https://github.com/mapbox/mapbox-mcp-server) — TypeScript
- [**AWS Location MCP Server**](https://github.com/awslabs/mcp) — Python
- [**google-maps-mcp**](https://github.com/modelcontextprotocol/servers) — TypeScript
- [**atlas-obscura-api**](https://github.com/bartholomej/atlas-obscura-api) — JavaScript
- [**gurume**](https://github.com/narumiruna/gurume) — Python
- [**Naver-Place-scraper**](https://github.com/seolhalee/Naver-Place-scraper) — Python
- [**trip-planner**](https://github.com/adl1995/trip-planner) — Python
- [**query-overpass**](https://github.com/perliedman/query-overpass) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
