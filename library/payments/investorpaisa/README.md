# InvestorPaisa CLI

**The Nifty 200 momentum screener, movers and market breadth in your terminal, with a local history that builds from your first sync.**

InvestorPaisa shows a clean Nifty 200 dashboard in the browser only, behind a Google sign-in. This CLI reuses your browser session, runs the site's screener expression language from scripts, and stores daily snapshots in SQLite so you can diff what changed since the last sync, track breadth over time, and hand an agent a one-call morning brief. Every live data command needs a signed-in session; --data-source local replays stored snapshots offline.

Learn more at [InvestorPaisa](https://investorpaisa.com).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `investorpaisa-pp-cli` binary and the `pp-investorpaisa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa --agent claude-code
npx -y @mvanhorn/printing-press-library install investorpaisa --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/cmd/investorpaisa-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/investorpaisa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-investorpaisa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-investorpaisa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install investorpaisa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
investorpaisa-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/investorpaisa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/cmd/investorpaisa-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "investorpaisa": {
      "command": "investorpaisa-pp-mcp"
    }
  }
}
```

</details>

## Authentication

InvestorPaisa is sign-in only (Google). Sign in to investorpaisa.com in Chrome, then run auth login --chrome (needs the pycookiecheat, cookies or cookie-scoop-cli helper; pip install pycookiecheat). Alternatives: auth login --cookies-file, piping the __Secure-authjs.session-token cookie value into auth set-token, or exporting INVESTORPAISA_SESSION_TOKEN as a cookie string (the cookie name, an equals sign, then its value). No password is ever handled.

## Quick Start

```bash
# Verify-safe first step: prints what doctor would run without touching the network or credentials. Run doctor without --dry-run for the real health report.
investorpaisa-pp-cli doctor --dry-run

# Pull the universe, screener metrics, live snapshot and overview into the local store.
investorpaisa-pp-cli sync

# See what is moving in the Nifty 200 right now.
investorpaisa-pp-cli movers --kind gainers --top 5

# Run the site's trend-strength screen locally.
investorpaisa-pp-cli screen "close > sma50 and sma50 > sma200 and rs >= 70" --sort rs

# One-call deep dive on a single stock.
investorpaisa-pp-cli quote RELIANCE

```

## Unique Features

### Local analytics over synced data
- **`screen`** — Run the site's screener expression language (close > sma50 and rs >= 70) from the terminal over the Nifty 200 metrics (live, falling back to your last stored snapshot).

  _Reach for this when an agent needs a custom multi-metric filter of the Nifty 200 instead of paging raw rows._

  ```bash
  investorpaisa-pp-cli screen "close > sma50 and sma50 > sma200 and rs >= 70" --sector IT --agent
  ```
- **`quote`** — Price, range, RSI/SMA/EMA, returns, sector percentile, signals, headlines and watchlist membership for one or more stocks in a single call.

  _Use this first when asked about a specific stock._

  ```bash
  investorpaisa-pp-cli quote RELIANCE TCS --agent
  ```

### Market pulse
- **`movers`** — Ranked gainers, losers, unusual volume, gappers and 52-week highs/lows, filtered by RS and sector.

  _Answers what is moving today and is also strong, in one call._

  ```bash
  investorpaisa-pp-cli movers --kind volume --min-rs 70 --top 10 --agent
  ```
- **`sectors`** — Equal-weight sector returns over 1d-1y, average RS and % above 50/200 DMA, with rank change versus an earlier stored snapshot (--trend needs two stored days).

  _Use for sector rotation questions across the sectors._

  ```bash
  investorpaisa-pp-cli sectors --window 1m --trend --agent
  ```
- **`brief`** — Index levels, breadth, your watchlist rows with change, RS and signals, and fresh headlines in one JSON document.

  _One call gives a scheduled agent the whole market-morning note._

  ```bash
  investorpaisa-pp-cli brief --agent
  ```

### Local state that compounds
- **`changes`** — Which stocks entered or left conditions (new 52-week high, RS crossing 70, 50/200 DMA crossings, 2x volume) since an earlier stored snapshot; needs two stored trading days.

  _Gives a morning agent the what-changed primitive without a scrape-and-diff script._

  ```bash
  investorpaisa-pp-cli changes --since 1d --agent
  ```
- **`breadth`** — Trend of advance/decline, % above 50/200 DMA, new 52-week highs and stocks at 52-week lows across your stored daily snapshots.

  _Answers whether participation is widening or narrowing._

  ```bash
  investorpaisa-pp-cli breadth --days 30 --agent
  ```
- **`history`** — Five years of daily OHLCV, split/bonus adjustment factors and corporate events for a stock, read from its chart page and cached locally (--raw, --years, --tail).

  _Use for long-horizon price context and backtests on adjusted prices._

  ```bash
  investorpaisa-pp-cli history TCS --years 5 --agent
  ```

## Recipes

### Morning note for an agent

```bash
investorpaisa-pp-cli brief --agent --select index,breadth,watchlists.rows.symbol,watchlists.rows.chg_pct
```

Narrow the brief to the few fields a scheduled agent needs.

### Strong stocks with unusual volume

```bash
investorpaisa-pp-cli movers --kind volume --min-rs 70 --top 10 --agent
```

Volume spikes only among relative-strength leaders.

### What entered the 52-week-high list

```bash
investorpaisa-pp-cli changes --since 1d --agent --select conditions.new_52w_high
```

Compares against the nearest stored earlier snapshot (see baseline_as_of); needs two stored days.

### Sector leaders over a month

```bash
investorpaisa-pp-cli sectors --window 1m --trend
```

Rank the sectors by equal-weight 1-month return and see rank movement versus the earlier snapshot.

## Usage

Run `investorpaisa-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `INVESTORPAISA_CONFIG_DIR`, `INVESTORPAISA_DATA_DIR`, `INVESTORPAISA_STATE_DIR`, or `INVESTORPAISA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `INVESTORPAISA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export INVESTORPAISA_HOME=/srv/investorpaisa
investorpaisa-pp-cli doctor
```

Under `INVESTORPAISA_HOME=/srv/investorpaisa`, the four dirs resolve to `/srv/investorpaisa/config`, `/srv/investorpaisa/data`, `/srv/investorpaisa/state`, and `/srv/investorpaisa/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "investorpaisa": {
      "command": "investorpaisa-pp-mcp",
      "env": {
        "INVESTORPAISA_HOME": "/srv/investorpaisa"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `INVESTORPAISA_DATA_DIR` overrides an explicit `--home` for that kind. Use `INVESTORPAISA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `INVESTORPAISA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `investorpaisa-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### bookmarks

Bookmarked news and video cards

- **`investorpaisa-pp-cli bookmarks`** - List bookmarked cards

### health

Service health (public, no auth)

- **`investorpaisa-pp-cli health`** - Check that investorpaisa.com is up

### live

Live Nifty 200 snapshot: index levels, breadth, per-stock LTP rows, signals, intraday curve

- **`investorpaisa-pp-cli live`** - Current intraday snapshot (server cache 15s). Includes NIFTY50/100/200 levels, advance/decline, sector rollups, 200 stock rows and signals

### market

Market overview: breadth, 52-week signals, equal-weight index

- **`investorpaisa-pp-cli market`** - End-of-day market overview with the ranked signal list (52-week highs/lows, gaps, volume spikes)

### me

Your profile and saved chart/screener layout

- **`investorpaisa-pp-cli me get`** - Show the signed-in profile (name, image)
- **`investorpaisa-pp-cli me layout`** - Show your saved chart layout (symbol, range, indicators, screener expression, theme)

### news

Headlines (Google News RSS aggregation) tagged to Nifty 200 symbols

- **`investorpaisa-pp-cli news by-symbol`** - Latest news for one stock symbol, newest first
- **`investorpaisa-pp-cli news feed`** - Market-wide news and video card feed with cursor pagination

### screener

Daily screener metrics for every Nifty 200 stock (48 fields: RS, RSI, SMAs, returns, beta, drawdown, volume pace)

- **`investorpaisa-pp-cli screener`** - Fetch the full 200-row metrics table (the site filters client-side; the sector query param is ignored server-side)

### screens

Your saved screens

- **`investorpaisa-pp-cli screens`** - List saved screener screens

### universe

The Nifty 200 stock universe (symbol, name, sector, last close, day change)

- **`investorpaisa-pp-cli universe`** - List the 200 stocks tracked by InvestorPaisa with last close and 1-day change

### videos

Finance videos and shorts (CNBC-TV18, NDTV Profit, Zee Business) from YouTube

- **`investorpaisa-pp-cli videos`** - List recent finance videos or shorts

### watchlists

Your watchlists (up to 20, up to 200 symbols each)

**These three watchlist commands change your InvestorPaisa account (`delete` is irreversible); every other command is read-only.**

- **`investorpaisa-pp-cli watchlists create`** - Create a watchlist
- **`investorpaisa-pp-cli watchlists delete`** - Delete a watchlist
- **`investorpaisa-pp-cli watchlists list`** - List watchlists with their symbols
- **`investorpaisa-pp-cli watchlists set-items`** - Replace the symbols in a watchlist


### breadth


- **`investorpaisa-pp-cli breadth`** - Trend of advance/decline, % above 50/200 DMA, new 52-week highs and stocks at 52-week lows across your stored daily snapshots.


### brief


- **`investorpaisa-pp-cli brief`** - Index levels, breadth, your watchlist rows with change, RS and signals, and fresh headlines in one JSON document.


### changes


- **`investorpaisa-pp-cli changes`** - Which stocks entered or left conditions (new 52-week high, RS crossing 70, 50/200 DMA crossings, 2x volume) since an earlier stored snapshot; needs two stored trading days.


### history


- **`investorpaisa-pp-cli history`** - Five years of daily OHLCV, split/bonus adjustment factors and corporate events for a stock, read from its chart page and cached locally (--raw, --years, --tail).


### movers


- **`investorpaisa-pp-cli movers`** - Ranked gainers, losers, unusual volume, gappers and 52-week highs/lows, filtered by RS and sector.


### quote


- **`investorpaisa-pp-cli quote`** - Price, range, RSI/SMA/EMA, returns, sector percentile, signals, headlines and watchlist membership for one or more stocks in a single call.


### screen


- **`investorpaisa-pp-cli screen`** - Run the site's screener expression language (close > sma50 and rs >= 70) from the terminal over the Nifty 200 metrics (live, falling back to your last stored snapshot).


### sectors


- **`investorpaisa-pp-cli sectors`** - Equal-weight sector returns over 1d-1y, average RS and % above 50/200 DMA, with rank change versus an earlier stored snapshot (--trend needs two stored days).




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`investorpaisa-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`investorpaisa-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`investorpaisa-pp-cli learnings list`** - Inspect taught rows
- **`investorpaisa-pp-cli learnings forget <query>`** - Undo a teach
- **`investorpaisa-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`investorpaisa-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`investorpaisa-pp-cli teach-pattern`** - Install a query/resource template up front
- **`investorpaisa-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `INVESTORPAISA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `investorpaisa-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
investorpaisa-pp-cli bookmarks

# JSON for scripting and agents
investorpaisa-pp-cli bookmarks --json
# Filter to specific fields
investorpaisa-pp-cli bookmarks --json --select kind,id,savedAt

# Dry run — show the request without sending
investorpaisa-pp-cli bookmarks --dry-run

# Agent mode — JSON + compact + no prompts in one flag
investorpaisa-pp-cli bookmarks --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to `watchlists create` retries and `--ignore-missing` to `watchlists delete` retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
investorpaisa-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `investorpaisa-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/investorpaisa-pp-cli/config.toml`; `--home`, `INVESTORPAISA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INVESTORPAISA_SESSION_TOKEN` | per_call | No | Session cookie string: the `__Secure-authjs.session-token` cookie name, an equals sign, then its value. Optional: `auth login --chrome` stores the cookie for you. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `investorpaisa-pp-cli doctor` reports `agentcookie: detected` and `auth status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `investorpaisa-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INVESTORPAISA_SESSION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `investorpaisa-pp-cli universe list` to see the stocks InvestorPaisa tracks

### API-specific
- **HTTP 401 or redirect to login** — Sign in to investorpaisa.com in Chrome, then run investorpaisa-pp-cli auth login --chrome (or refresh INVESTORPAISA_SESSION_TOKEN).
- **auth login --chrome says no cookie tool found** — Run pip install pycookiecheat, or use investorpaisa-pp-cli auth login --cookies-file <file> / pipe the cookie into auth set-token.
- **changes, breadth or sectors --trend return an empty result with a note** — They need at least two stored trading days; run investorpaisa-pp-cli sync on separate days.
- **history says no chart data for a symbol** — Check the symbol is in the Nifty 200 (investorpaisa-pp-cli universe list) and that your session is still signed in.
- **Prices look stale after market hours** — Live prices are last-traded values; the official close replaces them after the market shuts. Re-run sync after 16:00 IST.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**nse-mcp**](https://pypi.org/project/nse-mcp/) — Python
- [**Indian-Stock-Exchange-MCP**](https://github.com/anuragkrishna/Indian-Stock-Exchange-MCP) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
