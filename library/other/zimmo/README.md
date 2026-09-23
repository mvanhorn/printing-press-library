# Zimmo CLI

**Belgian real estate from Zimmo with exact addresses, sold comps, commune prices per m² and a local store for deal sourcing.**

Search Zimmo's listings, including sold and rented ones, from the terminal with no account. Listings you find are stored locally, which powers comps, underpriced, peb-trap, yield and motivated, and same-as matches them against the immoweb-pp-cli and immovlan-pp-cli stores.

Learn more at [Zimmo](https://www.zimmo.be).

Created by [@sambassio](https://github.com/sambassio) (sambassio).

## Install

The recommended path installs both the `zimmo-pp-cli` binary and the `pp-zimmo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zimmo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zimmo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zimmo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zimmo --agent claude-code
npx -y @mvanhorn/printing-press-library install zimmo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/zimmo/cmd/zimmo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zimmo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zimmo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zimmo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zimmo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zimmo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zimmo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/zimmo/cmd/zimmo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zimmo": {
      "command": "zimmo-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or key. The CLI mints Zimmo's anonymous web token automatically (valid 24 hours) and caches it.

## Quick Start

```bash
# Check connectivity; the anonymous token is minted on first use
zimmo-pp-cli doctor

# Resolve a postcode to Zimmo place ids
zimmo-pp-cli places --postal-code 1050

# Search and store matching listings
zimmo-pp-cli find --postcode 1050 --type apartment --max-price 400000

# Read one listing in full by its Zimmo code
zimmo-pp-cli show LAISZ

# Commune price per m² with its monthly history
zimmo-pp-cli prices --postcode 1050

```

## Unique Features

Commands built on the local store and on fields only Zimmo publishes (exact address, EPC kWh, rent, sold listings).

### Deal sourcing
- **`enrich`** — Refresh stored listings from Zimmo, resumably: status changes, price history, EPC, flood and planning flags, GPS.

  _Run it before peb-trap or motivated so their filters see detail fields._

  ```bash
  zimmo-pp-cli enrich --missing epc,flood --top 200
  ```
- **`peb-trap`** — Energy-poor stored listings (EPC F/G or high kWh/m²), rented ones first with their rent, showing Zimmo's renovation-obligation flag.

  _Surfaces owners under pressure to sell._

  ```bash
  zimmo-pp-cli peb-trap --postcode 1030 --max-price 450000
  ```
- **`motivated`** — Rank listings by seller pressure: days on market, total price cut, re-listing, EPC.

  _Finds negotiable deals first._

  ```bash
  zimmo-pp-cli motivated --postcode 1030 --min-days 120
  ```

### Cross-portal
- **`same-as`** — Find the Immoweb or Immovlan twin of a Zimmo listing and the price gap between portals.

  _Dedupe leads across Belgian portals without refetching anything._

  ```bash
  zimmo-pp-cli same-as --all --unmatched --agent
  ```

### Valuation
- **`comps`** — Sold or rented listings around one property with median €/m² and the subject's gap to it.

  _Median sold €/m² around the subject and its gap to it; Zimmo lists few sold properties, so the result widens to the postcode when thin._

  ```bash
  zimmo-pp-cli comps LAISZ --radius 800m --months 24 --agent
  ```
- **`underpriced`** — Rank stored for-sale houses and apartments by discount to their commune's €/m² (viager, whole buildings and service flats skipped).

  _Turns thousands of listings into a short list of apparent bargains._

  ```bash
  zimmo-pp-cli underpriced --postcode 1050 --type apartment --below 15
  ```
- **`yield`** — Gross rental yield of sale listings: the published rent when let, otherwise rentals of similar surface in the same postcode and type.

  _Buy-to-let screening without a spreadsheet._

  ```bash
  zimmo-pp-cli yield --all --postcode 1060 --min 5
  ```

## Recipes

### Cheap flats in Ixelles

```bash
zimmo-pp-cli find --postcode 1050 --type apartment --max-price 300000 --agent --select listings.zimmo_code,listings.price,listings.surface_m2,listings.epc,listings.address
```

Narrow a large listing payload to the fields that matter.

### Sold comps for a lead

```bash
zimmo-pp-cli comps LAISZ --radius 800m --months 24
```

Sold listings nearby with median price per m²; widens to the postcode when the radius is thin.

### Daily PEB hunt

```bash
zimmo-pp-cli peb-trap --postcode 1030 --max-price 450000 --json
```

Reads the local store: run `zimmo-pp-cli watch run --all` first; a one-off `zimmo-pp-cli find --epc F,G` also works. Energy-poor listings, rented ones first with their current rent.

### Daily alert

```bash
zimmo-pp-cli watch run --all --json
```

New, cheaper and gone listings for every saved search since the last run.

## Usage

Run `zimmo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZIMMO_CONFIG_DIR`, `ZIMMO_DATA_DIR`, `ZIMMO_STATE_DIR`, or `ZIMMO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZIMMO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZIMMO_HOME=/srv/zimmo
zimmo-pp-cli doctor
```

Under `ZIMMO_HOME=/srv/zimmo`, the four dirs resolve to `/srv/zimmo/config`, `/srv/zimmo/data`, `/srv/zimmo/state`, and `/srv/zimmo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zimmo": {
      "command": "zimmo-pp-mcp",
      "env": {
        "ZIMMO_HOME": "/srv/zimmo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZIMMO_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZIMMO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZIMMO_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `zimmo-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### Listings, prices and alerts

- `zimmo-pp-cli find` — Search listings (sale, rent, sold, rented, take-over) by commune, postcode, type, price, bedrooms, surface, EPC and text; stores every result
- `zimmo-pp-cli show <code>` — One listing in full: exact address, GPS, EPC kWh, flood/planning flags, rent, price history, agency
- `zimmo-pp-cli photos <code>` — List or download a listing's photos
- `zimmo-pp-cli prices` — Commune price per m² by type with the monthly history
- `zimmo-pp-cli agency <uuid|code>` — Agency contact, VAT, review score, listings count
- `zimmo-pp-cli watch save|run|list|rm` — Saved searches: new, cheaper and gone listings since the last run
- `zimmo-pp-cli drops` — Price cuts on stored listings (Zimmo price history + your observations)
- `zimmo-pp-cli shortlist [code]` — Star listings with a note, or list starred ones
- `zimmo-pp-cli dump` — Export stored listings as JSON, CSV or GeoJSON

Listings reach the local store only through `find`, `show`, `watch run` and `enrich`; framework `sync` loads places only.


### dealers

Get and search real-estate agencies

- **`zimmo-pp-cli dealers get`** - Get one real-estate agency (dealer) by UUID
- **`zimmo-pp-cli dealers search`** - Search agencies (filter by category, placeId; sort by distance)

### geocode

Geocode a Belgian address

- **`zimmo-pp-cli geocode`** - Geocode a Belgian address to coordinates and place ids

### listings

Get and search listings (raw API shape)

- **`zimmo-pp-cli listings get`** - Get one listing by Zimmo code (e.g. LAISZ) or listing UUID
- **`zimmo-pp-cli listings property-count`** - Total number of active properties on Zimmo
- **`zimmo-pp-cli listings search`** - Search listings with a Zimmo filter object (status, placeId, postalCode, category, price, bedrooms, energyLabel, text)

### locality-price

Commune price per m² (raw API shape)

- **`zimmo-pp-cli locality-price <placeId>`** - Price per m² for a commune (by property type) with monthly history

### places

Resolve postcodes and commune names to place ids

- **`zimmo-pp-cli places`** - Resolve a postcode and/or commune name to Zimmo place ids

### sub-locality-price

Sub-locality price per m² (raw API shape)

- **`zimmo-pp-cli sub-locality-price <placeId>`** - Price per m² for a sub-locality with monthly history

### sublocalities

Search sub-localities by keyword

- **`zimmo-pp-cli sublocalities`** - Search sub-localities (deelgemeenten) by keyword


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zimmo-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zimmo-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zimmo-pp-cli learnings list`** - Inspect taught rows
- **`zimmo-pp-cli learnings forget <query>`** - Undo a teach
- **`zimmo-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zimmo-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zimmo-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zimmo-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZIMMO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zimmo-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zimmo-pp-cli show LAISZ

# JSON for scripting and agents
zimmo-pp-cli show LAISZ --json
# Filter to specific fields
zimmo-pp-cli show LAISZ --json --select address,price,epc

# Dry run — show the request without sending
zimmo-pp-cli show LAISZ --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zimmo-pp-cli show LAISZ --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - commands that read listings (peb-trap, underpriced, motivated, drops, dump, same-as) use the local SQLite store filled by find/show/watch run; `search` queries Zimmo live by default and the stored listings with `--data-source local`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `1` other error (hand-written commands return Zimmo API errors as 1), `2` usage error, `3` not found, `4` access refused (generated commands on 401/403), `5` API error (generated commands), `7` rate limited (generated commands), `10` config error.

## Health Check

```bash
zimmo-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `zimmo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zimmo-pp-cli/config.toml`; `--home`, `ZIMMO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `find` or `places` to get a valid Zimmo code or place id

### API-specific
- **401 JWT Token not found** — Run the command again: the CLI re-mints Zimmo's anonymous token automatically; set ZIMMO_TOKEN only to force one
- **curl gets a Cloudflare 403 but the CLI works** — Expected: Cloudflare challenges curl's TLS fingerprint, not Go's
- **comps returns few sold listings** — Zimmo only shows listings agencies mark as sold; widen --radius or --months, or use prices for the commune €/m²
- **peb-trap, underpriced, motivated or same-as return nothing** — They read the local store: store listings first with `zimmo-pp-cli watch run --all`; a one-off `zimmo-pp-cli find --postcode 1050` also works. `sync` does not load listings
