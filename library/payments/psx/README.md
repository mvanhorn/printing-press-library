# Pakistan Stock Exchange CLI

**Every PSX data-portal surface in one Go binary, plus the local price history the exchange itself throws away.**

The PSX data portal renders only what is true right now and keeps nothing for you. This CLI mirrors the instrument master, screener metrics, sector aggregates and market snapshots into local SQLite, so longitudinal questions the portal structurally cannot answer become one command: what changed since last week (diff), which names are abnormal against their own history (unusual), and when a valuation multiple compressed (drift). Announcements, payouts and OHLCV history are read live rather than mirrored. No API key, no account, no browser.

Learn more at [Pakistan Stock Exchange](https://dps.psx.com.pk).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `psx-pp-cli` binary and the `pp-psx` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install psx
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install psx --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install psx --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install psx --agent claude-code
npx -y @mvanhorn/printing-press-library install psx --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/psx/cmd/psx-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/psx-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install psx --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-psx --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-psx --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install psx --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/psx-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/psx/cmd/psx-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "psx": {
      "command": "psx-pp-mcp"
    }
  }
}
```

</details>

## Authentication

PSX publishes no API and requires no credentials. Every endpoint this CLI uses is unauthenticated plain HTTP, so there is nothing to configure before your first command. The CLI paces itself to stay within the portal's observed politeness ceiling.

## Quick Start

```bash
# confirm the portal is reachable before anything else
psx-pp-cli doctor

# pull the instrument master once so symbol lookups work offline
psx-pp-cli sync --resources symbols

# price a single name to confirm live data flows
psx-pp-cli quote OGDC

# save the names you actually follow
psx-pp-cli watchlist track OGDC LUCK ENGRO

# record a point-in-time snapshot; diff, drift, unusual and rotation all read what this stores, so run it on a schedule
psx-pp-cli snapshot take

# the payoff: every corporate action across your names in one call
psx-pp-cli actions --watchlist --since 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-surface joins
- **`actions`** — See every announcement, payout, AGM date and circuit-breaker event for the symbols you track, in one place.

  _Reach for this when the question is 'did anything happen to my names', instead of running four searches and merging them by hand._

  ```bash
  psx-pp-cli actions --watchlist --since 7d --agent
  ```
- **`basis`** — Compare futures-board prices against the regular spot board to see premium or discount per symbol.

  _Use this for carry and roll questions that require crossing PSX's separate market boards._

  ```bash
  psx-pp-cli basis --market DFC --top 20 --agent
  ```
- **`docs search`** — Search PSX regulatory documents, listing guides and notices by keyword, with last-updated dates.

  _Use this to locate the governing PSX rule, guide or notice for a question instead of browsing the corporate site by hand._

  ```bash
  psx-pp-cli docs search "rule book" --agent
  ```

### Local state that compounds
- **`diff`** — Show what changed in price, volume and valuation metrics between two synced snapshots.

  _Use this when the user asks what moved since last week rather than what the price is right now._

  ```bash
  psx-pp-cli diff --since 7d --agent
  ```
- **`watchlist`** — Keep a saved symbol set and price it on demand, with change measured from the day you added each name.

  _Use this to give an otherwise stateless market portal a memory of which names the user actually cares about._

  ```bash
  psx-pp-cli watchlist prices --agent
  ```
- **`drift`** — Trace one symbol's PE, dividend yield, market cap or free float across time.

  _Use this when the question is about the trajectory of a multiple, not its current value._

  ```bash
  psx-pp-cli drift OGDC --metric pe --since 90d --agent
  ```

### Baselines the portal never keeps
- **`unusual`** — Find names trading abnormally against their own trailing history, not just the day's biggest movers.

  _Use this to separate a genuine volume spike from a name that always trades that way._

  ```bash
  psx-pp-cli unusual --baseline 30d --top 20 --agent
  ```
- **`rotation`** — Rank sectors by movement over a window and name the constituents that drove each move.

  _Use this for 'what is leading and lagging, and why' instead of a bare current-volume ranking._

  ```bash
  psx-pp-cli rotation --window 30d --top 5 --agent
  ```

## Recipes

### Weekly corporate-action sweep for your holdings

```bash
psx-pp-cli actions --watchlist --since 7d --agent --select symbol,kind,date,headline
```

Joins announcements, payouts, AGM dates and breaker events for saved symbols, narrowed with --select so an agent reads four fields instead of a large nested payload.

### Find genuinely abnormal volume, not just big movers

```bash
psx-pp-cli unusual --baseline 30d --top 20 --agent
```

Ranks each name against its own trailing median and dispersion, so a thin scrip doubling on tiny size does not crowd out a real institutional print.

### Trace a valuation multiple over a quarter

```bash
psx-pp-cli drift OGDC --metric pe --since 90d --agent
```

Reads accumulated screener snapshots to show when the multiple moved, which the portal cannot answer at all.

### See which constituents drove a sector

```bash
psx-pp-cli rotation --window 30d --top 5 --agent
```

Ranks sectors by change over the window and attributes each move to its largest contributors.

### Check futures premium to spot

```bash
psx-pp-cli basis --market DFC --top 20 --agent
```

Fetches the futures and regular boards and computes the per-symbol spread and percent premium.

### Find the governing PSX rulebook or guide

```bash
psx-pp-cli docs search "shariah compliant" --agent --select title,url,updated
```

Full-text search across the corporate site's document map, returning just the fields an agent needs to cite a source.

## Usage

Run `psx-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PSX_CONFIG_DIR`, `PSX_DATA_DIR`, `PSX_STATE_DIR`, or `PSX_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PSX_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PSX_HOME=/srv/psx
psx-pp-cli doctor
```

Under `PSX_HOME=/srv/psx`, the four dirs resolve to `/srv/psx/config`, `/srv/psx/data`, `/srv/psx/state`, and `/srv/psx/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "psx": {
      "command": "psx-pp-mcp",
      "env": {
        "PSX_HOME": "/srv/psx"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PSX_DATA_DIR` overrides an explicit `--home` for that kind. Use `PSX_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PSX_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `psx-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### announcements

Corporate and regulatory announcement streams

- **`psx-pp-cli announcements`** - Search announcements across company, PSX, SECP, NCCPL and CDC streams

### board

Live trading boards with bid/offer depth across markets

- **`psx-pp-cli board <market> <board>`** - Trading board for one market and board combination

### calendar

AGM and EOGM meeting calendar

- **`psx-pp-cli calendar`** - AGM and EOGM events in a date window

### circuit-breakers

Securities that hit price circuit breakers

- **`psx-pp-cli circuit-breakers`** - Instruments halted by a price circuit breaker, with the limit band and halt state that triggered the suspension

### company

Per-company profile and financial report index

- **`psx-pp-cli company profile`** - Company profile and quote page for an instrument
- **`psx-pp-cli company reports`** - Financial report index for an instrument

### debt

Debt market instruments

- **`psx-pp-cli debt`** - TFCs, Sukuks and other debt instruments with coupon, maturity and yields

### eligible-scrips

Margin-trading eligible securities

- **`psx-pp-cli eligible-scrips`** - Securities eligible for margin trading and margin financing

### history

Full historical OHLCV series

- **`psx-pp-cli history`** - Full OHLCV history for a symbol. PSX ignores date parameters and always returns the complete series

### indices

Index listing and constituents

- **`psx-pp-cli indices`** - All PSX indices with current level and change

### listings

Listing status by board

- **`psx-pp-cli listings <board> <status>`** - Listed companies filtered by board and listing status

### market

Whole-market snapshots and movers

- **`psx-pp-cli market debt-performers`** - Most active debt instruments by turnover, covering TFCs, Sukuks and government paper on the bonds board
- **`psx-pp-cli market performers`** - Top active, gainer and loser tables for the equity market
- **`psx-pp-cli market watch`** - Whole-market snapshot table: LDCP, open, high, low, current, change, volume

### payouts

Dividend, bonus and right-issue payout history

- **`psx-pp-cli payouts company`** - Full payout history for one instrument
- **`psx-pp-cli payouts list`** - Payout history across all instruments

### screener

Fundamental screening metrics for the listed universe

- **`psx-pp-cli screener`** - Market cap, price, PE (TTM), dividend yield, free float, 30-day average volume and 1-year change

### sectors

Sector aggregates and rankings

- **`psx-pp-cli sectors summary`** - Per-sector aggregates: advances, declines, turnover and market cap
- **`psx-pp-cli sectors top`** - Top sectors ranked by traded volume

### symbols

Listed instrument master: equities, ETFs and debt securities

- **`psx-pp-cli symbols`** - List every listed instrument with sector, ETF and debt flags

### timeseries

Intraday tick and end-of-day price series

- **`psx-pp-cli timeseries eod`** - End-of-day series as [epoch, close, volume, open] tuples
- **`psx-pp-cli timeseries intraday`** - Intraday tick series as [epoch, price, volume] tuples (last active session)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`psx-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`psx-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`psx-pp-cli learnings list`** - Inspect taught rows
- **`psx-pp-cli learnings forget <query>`** - Undo a teach
- **`psx-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`psx-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`psx-pp-cli teach-pattern`** - Install a query/resource template up front
- **`psx-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PSX_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `psx-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
psx-pp-cli announcements

# JSON for scripting and agents
psx-pp-cli announcements --json
# Filter to specific fields by name
psx-pp-cli announcements --json --select <field>[,<field>...]

# Dry run — show the request without sending
psx-pp-cli announcements --dry-run

# Agent mode — JSON + compact + no prompts in one flag
psx-pp-cli announcements --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `PSX_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `psx-pp-cli circuit-breakers`
- `psx-pp-cli circuit-breakers get`
- `psx-pp-cli circuit-breakers list`
- `psx-pp-cli circuit-breakers search`
- `psx-pp-cli debt`
- `psx-pp-cli debt get`
- `psx-pp-cli debt list`
- `psx-pp-cli debt search`
- `psx-pp-cli eligible-scrips`
- `psx-pp-cli eligible-scrips get`
- `psx-pp-cli eligible-scrips list`
- `psx-pp-cli eligible-scrips search`
- `psx-pp-cli indices`
- `psx-pp-cli indices get`
- `psx-pp-cli indices list`
- `psx-pp-cli indices search`
- `psx-pp-cli screener`
- `psx-pp-cli screener get`
- `psx-pp-cli screener list`
- `psx-pp-cli screener search`
- `psx-pp-cli sectors`
- `psx-pp-cli sectors get`
- `psx-pp-cli sectors list`
- `psx-pp-cli sectors search`
- `psx-pp-cli symbols`
- `psx-pp-cli symbols get`
- `psx-pp-cli symbols list`
- `psx-pp-cli symbols search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
psx-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `psx-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/psx-pp-cli/config.toml`; `--home`, `PSX_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Known Gaps

Honest limitations in this release. None block normal use; all are recorded so you are not
surprised by them.

- **`announcements` shows the link text, not the filing URL.** The last column renders as
  `View PDF` rather than the href. Capturing hrefs requires a change to the shared table extractor
  that would alter the row schema for every table-backed command and the synced store.
- **`indices` values can carry an embedded as-of timestamp**, e.g. `HBLTTI (18-08-2026 18:30:00)`.
- **`quote` and `market watch` report sector as a numeric code** (`0821`), not a name.
- **`diff`, `drift`, `unusual` and `rotation` need `snapshot take` first.** They read local snapshot
  history, not `sync`: `diff` and `rotation` need two captures, `unusual` needs three or more.
- **Announcements, payouts and OHLCV history are read live, not mirrored.** `sync` covers symbols,
  sectors, screener, indices, debt, eligible-scrips and circuit-breakers.
- **`payouts deadline` is derived, not official.** PSX publishes book-closure dates rather than
  ex-dates, so `buy_by` is the calendar day before book closure opens. Weekends and market holidays
  are not modelled — confirm against the exchange calendar before acting on it.
- **Rate limiting is self-imposed.** PSX publishes no documented limit; this CLI defaults to a
  conservative 2 requests per second. Tune with `--rate-limit`.


## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **history returns far more rows than the date range you asked for** — expected: PSX ignores date parameters server-side and always returns the full series, so the CLI filters locally after fetching
- **intraday returns ticks dated before today** — the symbol did not trade today; the portal serves the last active session, and 'market status' shows the age of the latest tick
- **diff, drift, unusual or rotation report no history** — these read snapshots, not sync. Run 'psx-pp-cli snapshot take' on a schedule: diff and rotation need 2 captures, unusual needs 3 or more
- **sync --resources announcements (or payouts, history) fails** — expected: those surfaces are read live and are not mirrored. Syncable resources are symbols, sectors, screener, indices, debt, eligible-scrips and circuit-breakers
- **requests start failing after a large fan-out** — lower --rate-limit; the portal publishes no limit and the CLI targets a conservative 2 requests per second by default
- **an HTML-backed command returns zero rows after a portal redesign** — run 'doctor' to confirm reachability, then report the table whose headers changed; extraction is keyed on header text, not column position

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://dps.psx.com.pk/
- Capture coverage: 12 API entries from 235 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Generation hints: weak_schema_confidence
- Candidate command ideas: create_calendar — Derived from observed POST /calendar traffic.; list_KSE100 — Derived from observed GET /timeseries/int/KSE100 traffic.; list_OGDC — Derived from observed GET /timeseries/int/OGDC traffic.; list_symbols — Derived from observed GET /symbols traffic.; list_top_10_sectors — Derived from observed GET /data/top-10-sectors traffic.

Warnings from discovery:
- empty_response_shapes: All endpoint clusters have empty response shapes; capture may have omitted response bodies. Re-fetch discovered endpoints with curl or another direct HTTP path before generating types.
- weak_schema_evidence: Binary or protobuf response cannot provide reliable JSON schema evidence.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**psxdata**](https://github.com/mtauha/psxdata) — Python (19 stars)
- [**PSX-Data-Api**](https://github.com/AbdulSami455/PSX-Data-Api) — Python (11 stars)
- [**PSX-MCP-Server**](https://github.com/ahad-raza24/PSX-MCP-Server) — Python (5 stars)
- [**psx-mcp**](https://github.com/revolutionarybukhari/psx-mcp) — Python (3 stars)
- [**psx-mcp-server**](https://github.com/ahmedraza-96/psx-mcp-server) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
