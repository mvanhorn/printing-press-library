# Moneycontrol CLI

**Indian market news and quotes without the scraping — offline-searchable and agent-native.**

Moneycontrol has no official API. This CLI wraps the news, index, and stock-quote surfaces that matter for following Indian markets into one tool with a local SQLite store, JSON output, and compound views no single web page offers.

Created by [@dev-abhirup-sc](https://github.com/dev-abhirup-sc) (dev-abhirup-sc).
Contributors: [@abhirup-dev](https://github.com/abhirup-dev) (Abhirup Das).

## Install

The recommended path installs both the `moneycontrol-pp-cli` binary and the `pp-moneycontrol` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol --agent claude-code
npx -y @mvanhorn/printing-press-library install moneycontrol --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/moneycontrol/cmd/moneycontrol-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/moneycontrol-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-moneycontrol --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-moneycontrol --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install moneycontrol --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/moneycontrol-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/moneycontrol/cmd/moneycontrol-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "moneycontrol": {
      "command": "moneycontrol-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check — no auth needed, moneycontrol is fully public.
moneycontrol-pp-cli doctor --dry-run


# Today's market snapshot: indices + gainers + losers + headlines.
moneycontrol-pp-cli market-wrap


# Live Reliance quote plus recent Reliance-tagged news.
moneycontrol-pp-cli stock-watch --sc-id RI


# Five deduped market headlines.
moneycontrol-pp-cli news digest --limit 5


# Raw latest-news listing as JSON for piping.
moneycontrol-pp-cli articles latest --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compound views

- **`market-wrap`** — One-command end-of-day view: indices, top gainers, top losers, and the latest market headlines in a single read.

  _Use this when you want the day's market snapshot without running four commands or loading a browser._

  ```bash
  moneycontrol-pp-cli market-wrap --json
  ```
- **`stock-watch`** — Full picture for one ticker: live quote plus the most recent news tagged to that stock.

  _Reach for this instead of the raw quote or news commands when you want context alongside price._

  ```bash
  moneycontrol-pp-cli stock-watch --sc-id RI --json
  ```
- **`news digest`** — Top market headlines plus brief recaps, deduplicated, ready to skim.

  _The fastest path to 'what moved today' for the Indian market._

  ```bash
  moneycontrol-pp-cli news digest --limit 10 --json
  ```
- **`news-for`** — Latest tagged news for several stocks at once, fetched in parallel.

  _Use this for a quick portfolio-news sweep across your watchlist in one call._

  ```bash
  moneycontrol-pp-cli news-for --sc-ids RI,INF,ITC --json
  ```

### Local state that compounds

- **`since`** — Articles synced to the local store newer than N hours, with stock-symbol filter.

  _Use this after a sync to answer 'what did I miss about my holdings since I last looked'._

  ```bash
  moneycontrol-pp-cli since --duration 2h --symbol RI
  ```

## Recipes


### End-of-day market wrap

```bash
moneycontrol-pp-cli market-wrap --json
```

Single JSON envelope with indices, top gainers, top losers, and the latest market headlines for the day.

### Drill into one stock

```bash
moneycontrol-pp-cli stock-watch --sc-id RI --select quote.HP,quote["52H"],news[].title --agent
```

Narrow the stock-watch output to just current price, 52-week high, and the news titles.

### Latest headlines, JSON for piping

```bash
moneycontrol-pp-cli news digest --limit 20 --json
```

Twenty deduped market headlines as clean JSON, ready to pipe into another tool.

### Portfolio news sweep

```bash
moneycontrol-pp-cli news-for --sc-ids RI,INF,ITC --per-stock-limit 5 --json
```

Latest headlines tagged to Reliance, Infosys, and ITC in one parallel call.

### Quote one ticker

```bash
moneycontrol-pp-cli stocks quote --sc-id RI --json
```

Full Reliance pricefeed: price, 52H/L, 1d-10y %chg, CAGR, volume.

## Usage

Run `moneycontrol-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MONEYCONTROL_CONFIG_DIR`, `MONEYCONTROL_DATA_DIR`, `MONEYCONTROL_STATE_DIR`, or `MONEYCONTROL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MONEYCONTROL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MONEYCONTROL_HOME=/srv/moneycontrol
moneycontrol-pp-cli doctor
```

Under `MONEYCONTROL_HOME=/srv/moneycontrol`, the four dirs resolve to `/srv/moneycontrol/config`, `/srv/moneycontrol/data`, `/srv/moneycontrol/state`, and `/srv/moneycontrol/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "moneycontrol": {
      "command": "moneycontrol-pp-mcp",
      "env": {
        "MONEYCONTROL_HOME": "/srv/moneycontrol"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MONEYCONTROL_DATA_DIR` overrides an explicit `--home` for that kind. Use `MONEYCONTROL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MONEYCONTROL_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `moneycontrol-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### articles

News articles across categories and per-stock tags.

- **`moneycontrol-pp-cli articles by-category`** - News headlines within a category (e.g. business/markets, business/stocks, india, world).
- **`moneycontrol-pp-cli articles by-tag`** - News headlines tagged to a stock or topic (e.g. reliance-industries, infosys, nifty).
- **`moneycontrol-pp-cli articles get`** - Full article page (title, description, and raw body HTML for goquery extraction).
- **`moneycontrol-pp-cli articles latest`** - Latest news headlines across all categories.

### indices

Indian index quotes (SENSEX, NIFTY 50, NIFTY BANK, NIFTY IT).

- **`moneycontrol-pp-cli indices`** - Live quote for an Indian index.

### stocks

Per-stock quote (full pricefeed: price, 52H/L, 1d-10y %chg, CAGR, volume, sector)

- **`moneycontrol-pp-cli stocks`** - Full NSE equity quote for a stock by its moneycontrol sc_id.

### trending

Trending stocks widget (most-searched tickers right now).

- **`moneycontrol-pp-cli trending`** - Currently trending stocks on moneycontrol.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`moneycontrol-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`moneycontrol-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`moneycontrol-pp-cli learnings list`** - Inspect taught rows
- **`moneycontrol-pp-cli learnings forget <query>`** - Undo a teach
- **`moneycontrol-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`moneycontrol-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`moneycontrol-pp-cli teach-pattern`** - Install a query/resource template up front
- **`moneycontrol-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MONEYCONTROL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `moneycontrol-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
moneycontrol-pp-cli articles get --slug india/tribunals-reforms-bill-2026-

# JSON for scripting and agents
moneycontrol-pp-cli articles get --slug india/tribunals-reforms-bill-2026- --json

# Filter to specific fields
moneycontrol-pp-cli articles get --slug india/tribunals-reforms-bill-2026- --json --select id,name,status

# Dry run — show the request without sending
moneycontrol-pp-cli articles get --slug india/tribunals-reforms-bill-2026- --dry-run

# Agent mode — JSON + compact + no prompts in one flag
moneycontrol-pp-cli articles get --slug india/tribunals-reforms-bill-2026- --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `MONEYCONTROL_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `moneycontrol-pp-cli trending`
- `moneycontrol-pp-cli trending get`
- `moneycontrol-pp-cli trending list`
- `moneycontrol-pp-cli trending search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
moneycontrol-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `moneycontrol-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `MONEYCONTROL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **priceapi returns code 201 'No data found for given key'** — The SC_ID or index key was wrong. Indices use in;SEN / in;NSX; stocks use the moneycontrol sc_id (e.g. RI for Reliance) with path /pricefeed/nse/equitycash/<SC_ID>.
- **news listing returns 0 articles** — Confirm the category slug is one moneycontrol actually serves: latest-news, business/markets, business/stocks, india, world.
- **article body is empty** — Some articles are live-blogs or video pages whose body uses a different container. The extractor targets #contentdata; report the URL for an extractor extension.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
