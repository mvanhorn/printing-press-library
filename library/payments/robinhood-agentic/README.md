# Robinhood Agentic Trading CLI

**Every tool on Robinhood's official Agentic Trading MCP as a typed, review-first CLI — sanctioned OAuth instead of scraped tokens, plus an offline portfolio journal no Robinhood tool ships.**

Every other Robinhood tool rides the reverse-engineered private API with browser-lifted tokens that Robinhood periodically breaks. This CLI speaks the official agentic MCP surface: OAuth login with automatic refresh, server-side order simulation as the default dry-run, the undocumented tools (Level-2 books, financials, tax lots, scanner specs) surfaced as typed commands, and a local SQLite store that turns one-shot MCP calls into portfolio history, order audit trails, and offline search.

## Install

The recommended path installs both the `robinhood-agentic-pp-cli` binary and the `pp-robinhood-agentic` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic --agent claude-code
npx -y @mvanhorn/printing-press-library install robinhood-agentic --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/cmd/robinhood-agentic-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/robinhood-agentic-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-robinhood-agentic --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-robinhood-agentic --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install robinhood-agentic --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
robinhood-agentic-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/robinhood-agentic-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ROBINHOOD_AGENTIC_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/cmd/robinhood-agentic-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "robinhood-agentic": {
      "command": "robinhood-agentic-pp-mcp",
      "env": {
        "ROBINHOOD_AGENTIC_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

First run: `auth login` self-registers a public OAuth client against Robinhood's dynamic-registration endpoint (no shipped secrets), opens your browser for the PKCE authorization on robinhood.com, catches the localhost callback, and stores access + refresh tokens in your config with automatic refresh (~4-day access tokens). `auth status` shows expiry; `ROBINHOOD_AGENTIC_TOKEN` overrides for CI. Reads and `review` (server-side order simulation) are always allowed. Order placement and cancellation, watchlist writes, and scan writes are blocked at the transport unless `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` is set — the hard floor that keeps read-only testing safe by construction. On top of that gate, the `guard` policy adds per-order and daily notional caps, a symbol allow/denylist, and a kill switch, all enforced locally before any order leaves the machine, and every mutation is recorded to the local write journal (`audit`). Recommended flow: `equities review` first, then set the write gate to place for real.

## Quick Start

```bash
# One-time OAuth: self-registers a client, opens the browser, stores refresh-capable tokens
robinhood-agentic-pp-cli auth login

# See your accounts and which one is agentic_allowed (the only one that can place agent orders)
robinhood-agentic-pp-cli accounts

# Real-time quotes with official prior close (max 20 symbols per call)
robinhood-agentic-pp-cli market quotes AAPL,MSFT,NVDA --compact

# Pull positions, orders, watchlists, and P&L into the local SQLite store
robinhood-agentic-pp-cli sync --full

# Server-side order simulation — Robinhood's own pre-trade warnings, nothing placed
robinhood-agentic-pp-cli equities review --account <account> --symbol AAPL --side buy --type limit --quantity 1 --limit-price 180

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`portfolio history`** — Answer 'what was my portfolio worth on any given day' from a local time series Robinhood doesn't keep.

  _Reach for this when a task needs portfolio value over time — no MCP tool can answer it._

  ```bash
  robinhood-agentic-pp-cli portfolio history --since 30d --sparkline
  ```
- **`portfolio winrate`** — Round-trip win rate, average win/loss, and per-symbol stats computed from your synced trade history.

  _Journaling and strategy review without exporting to a spreadsheet._

  ```bash
  robinhood-agentic-pp-cli portfolio winrate --account 5XX12345 --by-symbol
  ```
- **`wheel status`** — Per-symbol wheel-strategy stage (cash-secured put → assigned → covered call → called away) inferred automatically.

  _The Friday post-expiration answer to 'what got assigned and what stage is each position in'._

  ```bash
  robinhood-agentic-pp-cli wheel status --account RH123456 AAPL
  ```

### Agent safety rails
- **`guard`** — Set per-trade caps, daily caps, symbol allow/denylists, and a kill switch that the CLI enforces before any order leaves the machine.

  _Use this before letting any agent loop place orders — it is the only enforceable budget/kill-switch layer for the agentic account._

  ```bash
  robinhood-agentic-pp-cli guard set --max-order 500 --daily-cap 2000
  ```
- **`equities settle`** — Resolve an order to verified terminal truth — actual fill price and state — instead of trusting the placement echo.

  _Run after every place or cancel to get the real outcome, not the optimistic echo._

  ```bash
  robinhood-agentic-pp-cli equities settle 1a2b3c4d-5678-90ab-cdef-1234567890ab --account RH123456 --wait
  ```
- **`audit`** — See everything the CLI (or an agent driving it) reviewed, placed, canceled, or was denied — with idempotency keys and outcomes.

  _The weekly agent-accountability review: reconstruct exactly what an automation did._

  ```bash
  robinhood-agentic-pp-cli audit --since 7d --denied
  ```

### Agent-native rituals
- **`brief`** — The whole pre-open check — portfolio value, day-over-day delta, open orders, positions, top movers among your holdings, and upcoming earnings for held symbols — in one command.

  _The one-command replacement for the four-round-trip morning ritual._

  ```bash
  robinhood-agentic-pp-cli brief --account RH123456 --agent
  ```
- **`surface diff`** — Know when Robinhood adds, removes, or reshapes MCP tools — with dates — instead of discovering breakage in production.

  _Check after any unexplained failure: the tool surface is beta and moves without notice._

  ```bash
  robinhood-agentic-pp-cli surface diff
  ```

## Recipes

### Agent morning brief, trimmed

```bash
robinhood-agentic-pp-cli brief --account RH123456 --agent --select portfolio.total_value,delta,open_orders,movers
```

The whole pre-open check as compact JSON, narrowed with dotted --select paths so an agent doesn't parse kilobytes it won't use.

### Review an order without placing it

```bash
robinhood-agentic-pp-cli equities review --account RH123456 --symbol AAPL --side buy --type limit --quantity 1 --limit-price 180
```

Server-side simulation returns Robinhood's own pre-trade warnings; nothing is placed.

### Set guardrails before an agent session

```bash
robinhood-agentic-pp-cli guard set --max-order 500 --daily-cap 2000
```

Client-side caps the CLI enforces on every subsequent place command — the platform has no native equivalent.

### Watchlist quote board

```bash
robinhood-agentic-pp-cli watchlists quotes 11111111-2222-3333-4444-555555555555 --csv
```

Joins a watchlist's members to live quotes in one command (get the list id from `watchlists list` first) and emits CSV for spreadsheets.

### Portfolio sparkline

```bash
robinhood-agentic-pp-cli portfolio history --since 30d --sparkline
```

30 days of locally-snapshotted portfolio value — a series Robinhood's API cannot return.

## Usage

Run `robinhood-agentic-pp-cli <group> --help` (accounts, equities, market, options, portfolio, scans, watchlists, plus the transcendence commands brief/guard/audit/surface/wheel) for per-command flags; the Commands section below is the full reference.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ROBINHOOD_AGENTIC_CONFIG_DIR`, `ROBINHOOD_AGENTIC_DATA_DIR`, `ROBINHOOD_AGENTIC_STATE_DIR`, or `ROBINHOOD_AGENTIC_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ROBINHOOD_AGENTIC_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ROBINHOOD_AGENTIC_HOME=/srv/robinhood-agentic
robinhood-agentic-pp-cli doctor
```

Under `ROBINHOOD_AGENTIC_HOME=/srv/robinhood-agentic`, the four dirs resolve to `/srv/robinhood-agentic/config`, `/srv/robinhood-agentic/data`, `/srv/robinhood-agentic/state`, and `/srv/robinhood-agentic/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "robinhood-agentic": {
      "command": "robinhood-agentic-pp-mcp",
      "env": {
        "ROBINHOOD_AGENTIC_HOME": "/srv/robinhood-agentic"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ROBINHOOD_AGENTIC_DATA_DIR` overrides an explicit `--home` for that kind. Use `ROBINHOOD_AGENTIC_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ROBINHOOD_AGENTIC_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `robinhood-agentic-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

Brokerage accounts and the agentic-account boundary

- **`robinhood-agentic-pp-cli accounts`** - List all brokerage accounts; agentic_allowed marks the only account that can place agentic orders

### equities

Equity positions, orders, tax lots, and the review-first order lifecycle

- **`robinhood-agentic-pp-cli equities cancel`** - Request cancellation of an open equity order. accepted=true acknowledges the REQUEST — cancellation can race a fill, so re-read the order for terminal state
- **`robinhood-agentic-pp-cli equities orders`** - Equity order history and single-order lookup, newest first; executions[] carries fills
- **`robinhood-agentic-pp-cli equities place`** - Place a REAL equity order in the agentic account. Same parameters as review plus a ref_id idempotency key. Dry-run by default; live placement requires the write gate
- **`robinhood-agentic-pp-cli equities positions`** - Open equity positions with quantity, average cost, and hold breakdowns (pair with quotes for market value; use shares_available_for_sells for sellable shares)
- **`robinhood-agentic-pp-cli equities review`** - Server-side order simulation: returns Robinhood's pre-trade warnings WITHOUT placing anything. The canonical preflight before place
- **`robinhood-agentic-pp-cli equities taxlots`** - Open acquisition lots for ONE symbol: lot id, quantity, cost basis, acquisition date, holding period. Lot ids feed specified-lot sells (max 30 lots per order)

### market

Search, quotes, fundamentals, historicals, indicators, earnings, indexes

- **`robinhood-agentic-pp-cli market book`** - Level-2 bid/ask depth (max 4 symbols per call)
- **`robinhood-agentic-pp-cli market earnings-calendar`** - Market-wide earnings events in a window of up to 31 days
- **`robinhood-agentic-pp-cli market earnings-results`** - Up to 8 quarters of EPS history (estimate vs actual) plus the next report date for one symbol
- **`robinhood-agentic-pp-cli market financials`** - Reported quarterly or annual revenue, gross profit, net income, and net margin (max 20 symbols, 40 periods)
- **`robinhood-agentic-pp-cli market fundamentals`** - Valuation, market cap, session OHLCV, 52-week range, dividends, and company profile (max 10 symbols)
- **`robinhood-agentic-pp-cli market historicals`** - OHLCV bars for up to 10 symbols; intervals from 15second to 50year; bounds cover regular, extended, and 24/7 sessions
- **`robinhood-agentic-pp-cli market index-quotes`** - Real-time index levels by instrument UUID (from market indexes; match responses by instrument_id, not symbol)
- **`robinhood-agentic-pp-cli market indexes`** - Look up market indexes by symbol (comma-separated string; unmatched symbols are silently dropped)
- **`robinhood-agentic-pp-cli market indicators`** - Server-computed technical indicators (18 types) for one symbol
- **`robinhood-agentic-pp-cli market quotes`** - Real-time equity quotes with official prior close (max 20 symbols per call; beyond 20 the close blocks are omitted)
- **`robinhood-agentic-pp-cli market search`** - Search instruments, currency pairs, or market indexes by name or ticker
- **`robinhood-agentic-pp-cli market tradability`** - Per-account tradability: fractional, extended-hours, all-day, and short-selling flags (max 10 symbols)

### options

Option chains, contracts, quotes, positions, orders, and the option watchlist

- **`robinhood-agentic-pp-cli options cancel`** - Request cancellation of an open option order; accepted=true can race a fill — re-read for terminal state
- **`robinhood-agentic-pp-cli options chains`** - Option chains for an underlying: chain id, expiration dates, multiplier, underlying instruments
- **`robinhood-agentic-pp-cli options instruments`** - Option contract discovery with expiry/strike/type filters; returns contract UUIDs used by quotes and orders
- **`robinhood-agentic-pp-cli options orders`** - Option order history with state and chain filters
- **`robinhood-agentic-pp-cli options place`** - Place a REAL single-leg option order in the agentic account (multi-leg unsupported by the MCP). Dry-run by default; live placement requires the write gate
- **`robinhood-agentic-pp-cli options positions`** - Option positions with type/expiry/chain filters
- **`robinhood-agentic-pp-cli options quotes`** - Live option quotes and prior-session closes by contract UUID
- **`robinhood-agentic-pp-cli options review`** - Server-side option order simulation (single-leg): pre-trade warnings without placing
- **`robinhood-agentic-pp-cli options upgrade-info`** - Options-access level for the account and the URL to apply for an upgrade
- **`robinhood-agentic-pp-cli options watchlist`** - The separate options watchlist (errors if options trading is not enabled on the account)
- **`robinhood-agentic-pp-cli options watchlist-add`** - Add option contracts to the options watchlist by contract UUID
- **`robinhood-agentic-pp-cli options watchlist-remove`** - Remove option contracts from the options watchlist (position type must match how they were added)

### portfolio

Portfolio value, buying power, and P&L

- **`robinhood-agentic-pp-cli portfolio pnl-trades`** - Trade-by-trade realized P&L history, cursor-paginated, newest first (RHS account number)
- **`robinhood-agentic-pp-cli portfolio realized-pnl`** - Realized P&L buckets by span or date range. Uses the RHS account number, and asset_classes is required by the live service despite being documented optional
- **`robinhood-agentic-pp-cli portfolio show`** - Per-account portfolio value breakdown by asset class plus authoritative buying power (get_accounts buying power is unreliable; this is the source of truth)

### scans

Server-side market scanners and the runtime-discoverable filter DSL

- **`robinhood-agentic-pp-cli scans create`** - Create a saved scan (validate filters against scans specs first)
- **`robinhood-agentic-pp-cli scans list`** - Saved scans with their filters, columns, and sort configuration
- **`robinhood-agentic-pp-cli scans run`** - Run a saved scan and return matching instruments
- **`robinhood-agentic-pp-cli scans set-config`** - Update a saved scan's name, columns, or sort configuration
- **`robinhood-agentic-pp-cli scans set-filters`** - Replace a saved scan's filter set
- **`robinhood-agentic-pp-cli scans specs`** - The valid scanner filter types, predicates, and parameter shapes — the scan-filter DSL is discoverable at runtime, not statically documented

### watchlists

Custom and curated watchlists

- **`robinhood-agentic-pp-cli watchlists add`** - Add members to a custom watchlist — exactly one of symbols, currency pair ids, or index ids
- **`robinhood-agentic-pp-cli watchlists create`** - Create a custom watchlist (display name must be unique)
- **`robinhood-agentic-pp-cli watchlists follow`** - Follow a curated watchlist (cannot follow your own custom lists; a follow-limit error means unfollow another first)
- **`robinhood-agentic-pp-cli watchlists items`** - Members of one watchlist (instruments, currency pairs, indexes, futures — options live on the separate options watchlist)
- **`robinhood-agentic-pp-cli watchlists list`** - All watchlists: custom (user-writable) and robinhood-curated (manage via follow/unfollow)
- **`robinhood-agentic-pp-cli watchlists popular`** - Robinhood-curated popular watchlists available to follow
- **`robinhood-agentic-pp-cli watchlists remove`** - Remove members from a custom watchlist — exactly one of symbols, currency pair ids, or index ids
- **`robinhood-agentic-pp-cli watchlists unfollow`** - Unfollow a curated watchlist
- **`robinhood-agentic-pp-cli watchlists update`** - Update a CUSTOM watchlist's name, description, or emoji (curated lists are read-only)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`robinhood-agentic-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`robinhood-agentic-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`robinhood-agentic-pp-cli learnings list`** - Inspect taught rows
- **`robinhood-agentic-pp-cli learnings forget <query>`** - Undo a teach
- **`robinhood-agentic-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`robinhood-agentic-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`robinhood-agentic-pp-cli teach-pattern`** - Install a query/resource template up front
- **`robinhood-agentic-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ROBINHOOD_AGENTIC_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `robinhood-agentic-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
robinhood-agentic-pp-cli accounts

# JSON for scripting and agents
robinhood-agentic-pp-cli accounts --json

# Filter to specific fields
robinhood-agentic-pp-cli accounts --json --select id,name,status

# Dry run — show the request without sending
robinhood-agentic-pp-cli accounts --dry-run

# Agent mode — JSON + compact + no prompts in one flag
robinhood-agentic-pp-cli accounts --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
robinhood-agentic-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `robinhood-agentic-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/robinhood-agentic-pp-cli/config.toml`; `--home`, `ROBINHOOD_AGENTIC_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ROBINHOOD_AGENTIC_TOKEN` | per_call | No (optional override) | Bearer/access-token override for CI; normally unnecessary — `auth login` stores refresh-capable OAuth tokens. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `robinhood-agentic-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `robinhood-agentic-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ROBINHOOD_AGENTIC_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 authentication required on every command** — Run `auth login` (tokens expire after ~4 days without refresh; `auth status` shows expiry)
- **Order rejected with a time-in-force error** — Robinhood's MCP accepts only `--tif gfd` or `--tif gtc` — the classic `day` value is rejected server-side
- **portfolio realized-pnl fails with a missing-parameter error** — Pass `--asset-classes equity,option` — the live service requires it despite docs marking it optional, and use the RHS account number from `accounts list`
- **place returns an account-not-allowed rejection** — Only the dedicated Agentic account (agentic_allowed=true in `accounts list`) can place orders; reads work on every account
- **Exit code 7 / rate-limited responses** — Robinhood publishes no rate limits; the CLI backs off adaptively — re-run `doctor --json` to see limiter state

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**robin_stocks**](https://github.com/jmfernandes/robin_stocks) — Python (2106 stars)
- [**alpaca-mcp-server**](https://github.com/alpacahq/alpaca-mcp-server) — Python (880 stars)
- [**schwab-mcp**](https://github.com/jkoelker/schwab-mcp) — Python (57 stars)
- [**robinhood-for-agents**](https://github.com/kevin1chun/robinhood-for-agents) — TypeScript (48 stars)
- [**robinhood-mcp**](https://github.com/verygoodplugins/robinhood-mcp) — Python (33 stars)
- [**robinhood-cli-mcp-api**](https://github.com/zaydiscold/robinhood-cli-mcp-api) — TypeScript (8 stars)
- [**robin-python-SDK**](https://github.com/gordil/robin-python-SDK) — Python
- [**alpheus**](https://github.com/JackZhao98/alpheus) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
