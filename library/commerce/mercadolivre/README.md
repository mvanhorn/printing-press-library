# Mercado Livre CLI

**Search Mercado Livre, compare prices and technical specs across listings, and build cotações — offline-persisted and agent-native, for procurement automation.**

A buyer-side CLI for procurement/suprimentos: it extracts stable JSON-LD from Mercado Livre search and product pages (clearing the captcha wall with an imported Chrome session), persists prices and spec attributes to a local SQLite store, and adds comparison commands no scraper offers — an aligned spec matrix (compare), cheapest-meeting-spec (cheapest), cross-seller price dispersion (dispersion), and cotação bundles with decision-time provenance.

Learn more at [Mercado Livre](https://api.mercadolibre.com).

Created by [@wandreis](https://github.com/wandreis) (wandreis).

## Install

The recommended path installs both the `mercadolivre-pp-cli` binary and the `pp-mercadolivre` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre --agent claude-code
npx -y @mvanhorn/printing-press-library install mercadolivre --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercadolivre-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mercadolivre --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mercadolivre --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install mercadolivre --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
mercadolivre-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mercadolivre-current).
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
    "mercadolivre": {
      "command": "mercadolivre-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Mercado Livre serves search and product pages behind a captcha wall to plain HTTP clients. Run 'auth login --chrome' to import your logged-in Chrome cookies; the CLI replays requests with a Chrome fingerprint (no resident browser). The no-auth helper endpoints (category attributes, domain discovery, autosuggest) work without any login.

## Quick Start

```bash
# import Chrome cookies to clear the captcha wall
mercadolivre-pp-cli auth login --chrome

# search with prices; auto-persists listings + a price snapshot to the local store
mercadolivre-pp-cli listings "notebook gamer" --limit 20

# fetch full technical specs for a candidate; persists product + attributes
mercadolivre-pp-cli products get MLB51764304

# aligned spec matrix, only differing rows
mercadolivre-pp-cli compare MLB51764304 MLB40287816 --diff

# cheapest listing meeting the spec floor
mercadolivre-pp-cli cheapest --query furadeira --spec "voltagem=220V"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Procurement comparison
- **`compare`** — Compare N products side-by-side as a normalized attribute matrix with a price row, so 'which meets 220V and >=700W' is one command instead of six browser tabs.

  _Reach for this when an agent must justify a procurement choice across candidate listings with differing spec labels._

  ```bash
  mercadolivre-pp-cli compare MLB51764304 MLB40287816 --diff --agent
  ```
- **`cheapest`** — Find the lowest-priced listing that satisfies attribute floors like voltage and power, evaluated against locally synced spec data.

  _Reach for this to answer 'cheapest that meets spec' without eyeballing mismatched seller attribute sets._

  ```bash
  mercadolivre-pp-cli cheapest --query furadeira --spec "voltagem=220V" --spec "potencia>=700W" --agent
  ```
- **`cotacao`** — Assemble a purchase-request-shaped quotation across chosen products with price, seller, key specs, and captured_at provenance.

  _Reach for this to emit the final cotação document instead of a raw table dump._

  ```bash
  mercadolivre-pp-cli cotacao MLB51764304 MLB40287816 --format md
  ```

### Price intelligence
- **`dispersion`** — Report min/max/median/stddev of prices across the multiple sellers a single Mercado Livre catalog product aggregates.

  _Reach for this to gauge whether a quoted price is competitive against the full seller spread for the same catalog item._

  ```bash
  mercadolivre-pp-cli dispersion MLB51764304 --agent
  ```
- **`price-history`** — Show how a product's price changed across repeated local snapshots, with an optional --since window.

  _Reach for this to catch supplier price creep on recurring consumables before it hits a PO._

  ```bash
  mercadolivre-pp-cli price-history MLB51764304 --since 30d --agent
  ```
- **`stale`** — List products whose newest price snapshot is older than a threshold, so a cotacao is never built on cold prices.

  _Reach for this before finalizing a quote to confirm every price is fresh enough to trust._

  ```bash
  mercadolivre-pp-cli stale --older-than 7d --agent
  ```

## Recipes

### Discover and persist candidates

```bash
mercadolivre-pp-cli listings "furadeira de impacto" --limit 30
```

Search, extract JSON-LD, and auto-store listings + a price snapshot for offline comparison.

### Cheapest meeting a spec floor

```bash
mercadolivre-pp-cli cheapest --query "furadeira de impacto" --spec "voltagem=220V" --spec "potencia>=700W" --agent
```

Filter synced listings by attribute predicates, then sort by price for the agent.

### Spec matrix, narrowed fields

```bash
mercadolivre-pp-cli compare MLB51764304 MLB40287816 --agent --select products.name,products.price,attributes.voltagem,attributes.potencia
```

Emit only the columns the decision needs from a deeply nested comparison payload.

### Check price spread before quoting

```bash
mercadolivre-pp-cli dispersion MLB51764304
```

See min/max/median across sellers for the catalog product.

### Build the cotacao

```bash
mercadolivre-pp-cli cotacao MLB51764304 MLB40287816 --format md
```

Assemble the purchase-request document with prices, specs, and captured_at provenance.

## Usage

Run `mercadolivre-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MERCADOLIVRE_CONFIG_DIR`, `MERCADOLIVRE_DATA_DIR`, `MERCADOLIVRE_STATE_DIR`, or `MERCADOLIVRE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MERCADOLIVRE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MERCADOLIVRE_HOME=/srv/mercadolivre
mercadolivre-pp-cli doctor
```

Under `MERCADOLIVRE_HOME=/srv/mercadolivre`, the four dirs resolve to `/srv/mercadolivre/config`, `/srv/mercadolivre/data`, `/srv/mercadolivre/state`, and `/srv/mercadolivre/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "mercadolivre": {
      "command": "mercadolivre-pp-mcp",
      "env": {
        "MERCADOLIVRE_HOME": "/srv/mercadolivre"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MERCADOLIVRE_DATA_DIR` overrides an explicit `--home` for that kind. Use `MERCADOLIVRE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MERCADOLIVRE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `mercadolivre-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### autosuggest

Query autosuggest / expansion (no auth)

- **`mercadolivre-pp-cli autosuggest`** - Suggested queries for a search term

### categories

Mercado Livre category metadata and attribute schema (no auth)

- **`mercadolivre-pp-cli categories attributes`** - Get the attribute schema for a category (which specs exist, value types) to normalize comparison columns
- **`mercadolivre-pp-cli categories get`** - Get category metadata by id

### discovery

Map a free-text query to a Mercado Livre category/domain (no auth)

- **`mercadolivre-pp-cli discovery`** - Resolve a free-text query to domain_id/category_id

### listings

Search Mercado Livre listings (JSON-LD extraction from the search page)

- **`mercadolivre-pp-cli listings <query>`** - Search listings by term; extracts the search page JSON-LD @graph (name, brand, price BRL, availability, product URL, rating)

### products

Fetch a catalog product page (JSON-LD Product + technical spec table)

- **`mercadolivre-pp-cli products <catalog_id>`** - Fetch a catalog product's price, rating, shipping and full technical attribute table


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mercadolivre-pp-cli categories get mock-value

# JSON for scripting and agents
mercadolivre-pp-cli categories get mock-value --json

# Filter to specific fields
mercadolivre-pp-cli categories get mock-value --json --select id,name,status

# Dry run — show the request without sending
mercadolivre-pp-cli categories get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mercadolivre-pp-cli categories get mock-value --agent
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
mercadolivre-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `mercadolivre-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/mercadolivre-pp-cli/config.toml`; `--home`, `MERCADOLIVRE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `mercadolivre-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Requests return a captcha/security page** — Re-run 'auth login --chrome' to refresh imported cookies; ensure you are logged into mercadolivre.com.br in Chrome.
- **compare shows empty columns for some attributes** — Run 'products get <id>' on both products first; sellers expose different attribute sets, and fetching each product populates its spec attributes.
- **price-history shows only one point** — Price history builds from repeated captures; re-run 'listings' or 'products get' on a schedule to accumulate snapshots.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.mercadolivre.com.br
- Capture coverage: 3 API entries from 3 total network entries
- Reachability: browser_clearance_http (90% confidence)
- Generation hints: browser_clearance_required

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**linces/MercadoScraper**](https://github.com/linces/MercadoScraper) — Python (24 stars)
- [**newerton/mcp-mercado-livre**](https://github.com/newerton/mcp-mercado-livre) — TypeScript (8 stars)
- [**Menegueli/WebscrapingML**](https://github.com/Menegueli/WebscrapingML) — Python
- [**heitornolla/WebScraping-MercadoLivre**](https://github.com/heitornolla/WebScraping-MercadoLivre) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
