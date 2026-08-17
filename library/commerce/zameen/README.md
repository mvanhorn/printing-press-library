# Zameen CLI

**Search Zameen.com property listings from your terminal — real filters, an offline SQLite mirror, saved-search monitoring, and price-drop alerts no scraper offers.**

Zameen.com has no API and no alerts, so investors and agents refresh the same search pages by hand every day. This CLI searches with real filters (price, beds, area in Marla/Kanal, city/area), mirrors results into a local SQLite store, and diffs runs to surface new listings and price drops. Built-in `comps`, `deals`, and `aging` turn that store into area medians, below-market plot-file detection, and days-on-market leverage.

Learn more at [Zameen](https://www.zameen.com).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `zameen-pp-cli` binary and the `pp-zameen` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zameen
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zameen --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zameen --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zameen --agent claude-code
npx -y @mvanhorn/printing-press-library install zameen --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/zameen/cmd/zameen-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zameen-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install zameen --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zameen --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zameen --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install zameen --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zameen-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/zameen/cmd/zameen-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zameen": {
      "command": "zameen-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key needed. Zameen's public search is served as standard HTTP; the CLI reads listing data straight from the server-rendered search pages.

## Quick Start

```bash
# Confirm Zameen is reachable and the local store is ready — no auth required.
zameen-pp-cli doctor


# Search 3+ bed homes for sale in Islamabad under 3 crore.
zameen-pp-cli find --city Islamabad --purpose buy --type Homes --min-beds 3 --max-price 30000000


# Mirror the first 5 pages into the local store so offline search, comps, and diffing work.
zameen-pp-cli pull --city Islamabad --purpose buy --type Homes --max-pages 5


# Area price research: median price and price-per-Marla from your synced listings.
zameen-pp-cli comps --city Islamabad --area DHA_Defence


# Save this search and, on each re-run, see new listings and price drops since last time.
zameen-pp-cli watch dha-homes --city Islamabad --purpose buy --type Homes --area DHA_Defence

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`watch`** — Re-run a saved search and see exactly which listings are new and which dropped in price since last time.

  _Reach for this to monitor a market over time instead of re-eyeballing search pages; it answers 'what changed since yesterday?' which no single fetch can._

  ```bash
  zameen-pp-cli watch dha-homes --city Islamabad --purpose buy --type Homes --area DHA_Defence --agent
  ```
- **`comps`** — Median price, price-per-Marla, and inventory count for an area or society, computed from your synced listings.

  _Use for area-level comps sheets and market rollups before a pitch or an offer._

  ```bash
  zameen-pp-cli comps --city Lahore --area DHA_Defence --agent --select area,median_price,median_price_per_marla,count
  ```
- **`agencies`** — Ranks agencies by listing count and median asking price in an area, from your synced listings.

  _Use to see who dominates supply in a society before choosing an agent or gauging market concentration._

  ```bash
  zameen-pp-cli agencies --city Karachi --agent
  ```

### Deal-finding

- **`deals`** — Ranks scanned listings by how far under the area's price-per-Marla they sit and flags below-market files.

  _Reach for this to surface underpriced plot files in Pakistan's plot-flipping market before another dealer grabs them._

  ```bash
  zameen-pp-cli deals --city Islamabad --area DHA_Defence --type Plots --agent
  ```
- **`aging`** — Lists the longest-standing inventory by days-on-market, derived from each listing's update timestamp.

  _Use to find sellers likely to negotiate — inventory that has sat on the market longest._

  ```bash
  zameen-pp-cli aging --city Islamabad --days 90 --agent
  ```

## Recipes


### Search homes for sale under budget

```bash
zameen-pp-cli find --city Islamabad --purpose buy --type Homes --min-beds 3 --max-price 30000000 --sort price-asc
```

Three-plus-bed homes for sale in Islamabad under 3 crore, cheapest first.

### Bulk-export a search to CSV

```bash
zameen-pp-cli find --city Lahore --purpose rent --type Homes --max-scan-pages 10 --csv > lahore-rentals.csv
```

Scan up to 10 pages of Lahore rentals and write a CSV for a spreadsheet.

### Find below-market plot files

```bash
zameen-pp-cli deals --city Islamabad --area DHA_Defence --type Plots --agent
```

Rank DHA Islamabad plot files by how far under the area price-per-Marla they sit, as structured output for an agent.

### Narrow a verbose listing payload for an agent

```bash
zameen-pp-cli find --city Karachi --purpose buy --type Homes --agent --select external_id,title,price,area_marla,beds,location
```

Return only the fields an agent needs from each listing instead of the full record, keeping context small.

### Monitor a saved search for changes

```bash
zameen-pp-cli watch dha-homes --city Islamabad --purpose buy --type Homes --area DHA_Defence
```

On each run, diff against the last snapshot to show new listings and price drops.

## Usage

Run `zameen-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ZAMEEN_CONFIG_DIR`, `ZAMEEN_DATA_DIR`, `ZAMEEN_STATE_DIR`, or `ZAMEEN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ZAMEEN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ZAMEEN_HOME=/srv/zameen
zameen-pp-cli doctor
```

Under `ZAMEEN_HOME=/srv/zameen`, the four dirs resolve to `/srv/zameen/config`, `/srv/zameen/data`, `/srv/zameen/state`, and `/srv/zameen/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "zameen": {
      "command": "zameen-pp-mcp",
      "env": {
        "ZAMEEN_HOME": "/srv/zameen"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ZAMEEN_DATA_DIR` overrides an explicit `--home` for that kind. Use `ZAMEEN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ZAMEEN_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `zameen-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### listings

Search and read Zameen property listings

- **`zameen-pp-cli listings <category> <location> <page>`** - Fetch one page of Zameen search results (server-rendered; listings parsed from the embedded page state)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`zameen-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`zameen-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`zameen-pp-cli learnings list`** - Inspect taught rows
- **`zameen-pp-cli learnings forget <query>`** - Undo a teach
- **`zameen-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`zameen-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`zameen-pp-cli teach-pattern`** - Install a query/resource template up front
- **`zameen-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ZAMEEN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `zameen-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zameen-pp-cli listings Homes Islamabad-3 1

# JSON for scripting and agents
zameen-pp-cli listings Homes Islamabad-3 1 --json

# Filter to specific fields
zameen-pp-cli listings Homes Islamabad-3 1 --json --select id,name,status

# Dry run — show the request without sending
zameen-pp-cli listings Homes Islamabad-3 1 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zameen-pp-cli listings Homes Islamabad-3 1 --agent
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
zameen-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `zameen-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/zameen-pp-cli/config.toml`; `--home`, `ZAMEEN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **find returns 0 results for a city name** — Use a Zameen location slug with its numeric id (e.g. Islamabad-3, Lahore_DHA_Defence-9), or one of the known cities: Islamabad, Lahore, Karachi, Rawalpindi, Faisalabad, Multan.
- **comps / deals / watch say the store is empty** — Run zameen-pp-cli pull --city <c> --purpose <buy|rent> --type <Homes|Plots|Commercial> --max-pages <n> first to populate the local mirror.
- **scan seems capped and misses matches** — Raise --max-scan-pages (find) or --max-pages (pull); Zameen filters are applied client-side, so more pages are scanned to find matches.
- **HTTP 403 or challenge page from Zameen** — The CLI already sends a browser User-Agent; if it persists you are likely rate-limited — wait and retry, or lower --max-pages.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**zameen-com-scrapper**](https://github.com/IIvexII/zameen-com-scrapper) — Python (6 stars)
- [**zameen.com-scraper**](https://github.com/osmanrkhan/zameen.com-scraper) — Python (3 stars)
- [**homehunt**](https://github.com/afspies/homehunt) — Python (1 stars)
- [**zameen-scraper**](https://github.com/aliraheel626/zameen-scraper) — Python (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
