# Screener.in CLI

**Every Screener.in feature, plus offline search, screen intersections, and a local fundamentals mirror no other tool has.**

Search any Indian company and pull its full fundamental profile — key metrics, machine-generated pros/cons, quarterly results, P&L, balance sheet, cash flow, ratios, shareholding, peers, and price charts — from the terminal. Sync company pages, screens, and insider trades into a local SQLite mirror, then run cross-company comparisons, quarterly trend flags, screen overlaps, and insider-flow rankings that no single page or API call provides.

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `screener-pp-cli` binary and the `pp-screener` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install screener
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install screener --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install screener --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install screener --agent claude-code
npx -y @mvanhorn/printing-press-library install screener --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/screener/cmd/screener-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/screener-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install screener --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-screener --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-screener --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install screener --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
screener-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/screener-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/screener/cmd/screener-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "screener": {
      "command": "screener-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Screener.in's public pages (company profiles, screens, sectors, IPO) need no login. Market-pulse pages — latest quarterly results, insider trades, filings — require a free Screener.in account. Run 'screener-pp-cli auth login --chrome' to import your logged-in Chrome session cookie once; the CLI replays it for gated pages.

## Quick Start

```bash
# Verify connectivity to screener.in
screener-pp-cli doctor


# Find the company ID and symbol
screener-pp-cli company search --q 'Tata Steel' --json


# Pull the full fundamental profile
screener-pp-cli company profile INFY --json


# Run the Bull Cartel screen
screener-pp-cli screens run 1 the-bull-cartel --json


# Spot earnings acceleration or deterioration
screener-pp-cli qtrend INFY --quarters 8 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`compare`** — See two to four companies' fundamentals side by side (valuation, margins, growth, insider activity) without tab-switching.

  _Pick this when an agent needs to decide between candidate stocks on comparable fundamentals instead of fetching pages one at a time._

  ```bash
  screener-pp-cli compare TCS HDFCBANK --agent
  ```
- **`qtrend`** — Spot whether a company's quarterly profit/sales growth is accelerating or deteriorating, with YOY change and margin drift computed automatically.

  _Pick this when an agent needs the trend shape of a company's earnings, not just the latest quarter's raw numbers._

  ```bash
  screener-pp-cli qtrend INFY --quarters 8 --agent
  ```
- **`insider-flow`** — Answer 'who is net buying the most this month?' with per-company insider buy/sell aggregation.

  _Pick this when an agent needs insider conviction ranked by net value, not a raw list of individual trades._

  ```bash
  screener-pp-cli insider-flow --since 30d --top 10 --agent
  ```

### Cross-entity local queries

- **`overlap`** — Find companies that appear in two or more stock screens in one command, replacing spreadsheet dedup.

  _Pick this when an agent needs the intersection of multiple screening strategies to find high-conviction candidates._

  ```bash
  screener-pp-cli overlap 1 the-bull-cartel 59 magic-formula --agent
  ```
- **`rank`** — Re-score a screen's companies with a composite of fundamentals (P/E, ROCE, growth), sorted by what matters to you.

  _Pick this when an agent should prioritize within a screen by fundamentals or insider conviction rather than the default table order._

  ```bash
  screener-pp-cli rank 1 the-bull-cartel --by roce --agent
  ```

## Recipes


### Full company research brief

```bash
screener-pp-cli company profile RELIANCE --agent --select top_ratios,analysis,quarterly_results
```

One command returns the key metrics, pros/cons, and quarterly results as agent-ready JSON

### High-conviction screen intersection

```bash
screener-pp-cli overlap 1 the-bull-cartel 59 magic-formula --agent
```

Companies passing both a value formula and a growth screen

### Insider conviction ranking

```bash
screener-pp-cli insider-flow --since 30d --top 10 --agent
```

Who's net buying the most this month, ranked by value

### Earnings momentum check

```bash
screener-pp-cli qtrend INFY --quarters 8 --agent
```

Is profit growth accelerating or deteriorating over 8 quarters?

### Candidate shortlist comparison

```bash
screener-pp-cli compare TCS WIPRO HCLTECH --agent --select name,pe,roce,profit_growth
```

Side-by-side fundamentals for a shortlist, narrowed with --select

## Usage

Run `screener-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SCREENER_CONFIG_DIR`, `SCREENER_DATA_DIR`, `SCREENER_STATE_DIR`, or `SCREENER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SCREENER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SCREENER_HOME=/srv/screener
screener-pp-cli doctor
```

Under `SCREENER_HOME=/srv/screener`, the four dirs resolve to `/srv/screener/config`, `/srv/screener/data`, `/srv/screener/state`, and `/srv/screener/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "screener": {
      "command": "screener-pp-mcp",
      "env": {
        "SCREENER_HOME": "/srv/screener"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SCREENER_DATA_DIR` overrides an explicit `--home` for that kind. Use `SCREENER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SCREENER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `screener-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### company

Search companies and fetch fundamental profiles

- **`screener-pp-cli company by-id`** - Fetch a company page by numeric ID (redirects to symbol page)
- **`screener-pp-cli company chart`** - Fetch price/technical chart data for a company
- **`screener-pp-cli company peers`** - Fetch peer comparison table for a company
- **`screener-pp-cli company profile`** - Fetch the full company profile page (key metrics, analysis, quarterly results, P&L, balance sheet, cash flow, ratios, shareholding, documents)
- **`screener-pp-cli company profile-standalone`** - Fetch standalone (parent-only) company profile page
- **`screener-pp-cli company search`** - Search for companies by name or ticker (live autocomplete API)

### explore

Browse popular screens and sector categories

- **`screener-pp-cli explore`** - Browse popular themes, formulas, and sector categories

### filings

Market pulse filings hub (requires login)

- **`screener-pp-cli filings`** - Market Pulse hub: bulk deals, block deals, SAST trades, insider trades

### full_text_search

Full-text search across companies and filings (requires login)

- **`screener-pp-cli full-text-search`** - Full-text search for companies

### ipo

Upcoming and recent IPO listings

- **`screener-pp-cli ipo`** - List upcoming IPOs with subscription status

### market

Browse market sectors and industry groups

- **`screener-pp-cli market <sector_path>`** - List companies in a market sector (e.g. IN08/IN0801/IN080101 = IT - Software)

### results

Latest quarterly results (Market Pulse, requires login)

- **`screener-pp-cli results`** - Latest quarterly results with YOY growth (Sales, EBIDT, Net profit, EPS) and filters

### screens

Browse and run stock screening screens

- **`screener-pp-cli screens list`** - List all stock screening screens
- **`screener-pp-cli screens run`** - Run a stock screen and get ranked results (CMP, P/E, Mar Cap, Div Yld, NP Qtr, Qtr Profit Var, Sales Qtr, Qtr Sales Var, ROCE)

### trades

Market pulse trade activity (requires login)

- **`screener-pp-cli trades`** - Insider trades (bought/sold/pledge/ESOP) with filters


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`screener-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`screener-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`screener-pp-cli learnings list`** - Inspect taught rows
- **`screener-pp-cli learnings forget <query>`** - Undo a teach
- **`screener-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`screener-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`screener-pp-cli teach-pattern`** - Install a query/resource template up front
- **`screener-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SCREENER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `screener-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
screener-pp-cli company search --q example-value

# JSON for scripting and agents
screener-pp-cli company search --q example-value --json
# Filter to specific fields
screener-pp-cli company search --q example-value --json --select id,name,url

# Dry run — show the request without sending
screener-pp-cli company search --q example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
screener-pp-cli company search --q example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SCREENER_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `screener-pp-cli explore`
- `screener-pp-cli explore get`
- `screener-pp-cli explore list`
- `screener-pp-cli explore search`
- `screener-pp-cli filings`
- `screener-pp-cli filings get`
- `screener-pp-cli filings list`
- `screener-pp-cli filings search`
- `screener-pp-cli full_text_search`
- `screener-pp-cli full_text_search get`
- `screener-pp-cli full_text_search list`
- `screener-pp-cli full_text_search search`
- `screener-pp-cli ipo`
- `screener-pp-cli ipo get`
- `screener-pp-cli ipo list`
- `screener-pp-cli ipo search`
- `screener-pp-cli screens`
- `screener-pp-cli screens get`
- `screener-pp-cli screens list`
- `screener-pp-cli screens search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
screener-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `screener-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/screener-pp-cli/config.toml`; `--home`, `SCREENER_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SCREENER_SESSION_COOKIE` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `screener-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `screener-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SCREENER_SESSION_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **results/trades commands redirect to a register page** — You are not logged in. Run the 'auth login' command with the Chrome option with your Screener.in session, or 'screener-pp-cli auth status' to check.
- **HTTP 429 rate limited** — Screener.in throttles bulk access. Wait a few minutes and retry, or use 'sync --since' to cache data locally and query offline.
- **HTML table parse produces empty rows** — Screener.in occasionally changes table markup. Re-sync with 'screener-pp-cli sync --full' and report the page URL if parsing still fails.
- **Commands fail with unexpected errors or empty output** — Run 'screener-pp-cli doctor' to verify connectivity and auth, then retry. For auth-gated pages, run the 'auth login' command with the Chrome option to re-import your session.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**screenercli**](https://github.com/mayur1064/screenercli) — Python (1 stars)
- [**screener-ai-tool**](https://pypi.org/project/screener-ai-tool/) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
