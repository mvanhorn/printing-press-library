# MUFAP CLI

**Pakistan's mutual fund industry as a dated local panel: two decades of daily NAVs, monthly PKR asset allocation, and a market-implied short rate no other tool derives.**

MUFAP publishes one HTML page per date and nothing else. This CLI walks those dates into a local SQLite panel with an observed_at stamp on every row, then derives series the site never shows: a daily short-rate proxy from money-market fund yields, industry equity exposure in rupees, and cross-sectional return dispersion. It records which dates returned zero rows, so a gap is never mistaken for a zero.

## Install

The recommended path installs both the `mufap-pp-cli` binary and the `pp-mufap` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install mufap
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install mufap --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install mufap --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install mufap --agent claude-code
npx -y @mvanhorn/printing-press-library install mufap --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mufap/cmd/mufap-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mufap-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install mufap --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-mufap --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-mufap --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install mufap --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/mufap-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mufap/cmd/mufap-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mufap": {
      "command": "mufap-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credentials. MUFAP sits behind Cloudflare, so the CLI ships a Chrome-fingerprint HTTP transport that clears the challenge without a browser, a clearance cookie, or any login.

## Quick Start

```bash
# Confirm the Chrome-fingerprint transport reaches MUFAP before fetching anything
mufap-pp-cli doctor --dry-run

# 27 asset managers; their GUIDs key every fund lookup
mufap-pp-cli amcs --json

# Walk four dates into the local panel, one request per date
mufap-pp-cli backfill daily --from 2026-09-03 --to 2026-09-04

# Confirm what was actually stored, including any zero-row dates
mufap-pp-cli coverage --resource daily-returns --from 2026-09-03 --to 2026-09-04

# Derive the market-implied short rate from the stored panel
mufap-pp-cli rates --from 2026-09-03 --to 2026-09-04 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Series only a local panel can produce
- **`rates`** — Derive a daily short-term interest rate series from the cross-section of money-market fund yields.

  _This is the only daily, backfillable PKR short-rate proxy obtainable without a blocked government source._

  ```bash
  mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`exposure`** — Total PKR the mutual fund industry holds in listed equities, by month, from per-fund asset allocation.

  _It is the asset-side counterpart to a mutual-fund net-flow series, enabling a cross-source join no single publisher offers._

  ```bash
  mufap-pp-cli exposure --from 2026-07 --to 2026-07 --max-amcs 1 --agent
  ```
- **`dispersion`** — Cross-sectional spread of fund returns within a category on each date.

  _Dispersion is a daily breadth measure that a single-fund page cannot express._

  ```bash
  mufap-pp-cli dispersion --category Equity --from 2026-09-01 --to 2026-09-04 --agent
  ```

### Local panel construction
- **`backfill`** — Fetch the daily fund panel or monthly allocation across a date range into the local store.

  _Publication timing is what makes a variable admissible as ex-ante, so every row records when it was actually observed._

  ```bash
  mufap-pp-cli backfill daily --from 2026-09-03 --to 2026-09-03
  ```
- **`panel`** — Query the stored daily NAV and return panel by date, fund, category or sector.

  _Turns 21 years of single-date HTML pages into one queryable table._

  ```bash
  mufap-pp-cli panel --category Equity --from 2026-09-01 --to 2026-09-04 --agent --select date,fund,nav
  ```
- **`coverage`** — Show which dates were fetched, which returned zero rows, and which were never attempted.

  _Distinguishes fetched-and-empty from never-fetched, so a gap is never mistaken for a zero._

  ```bash
  mufap-pp-cli coverage --resource daily-returns --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`dump`** — Dump the stored panel, allocation or coverage tables as JSONL for piping, a single JSON array for agents, or CSV.

  _Lets an analysis pipeline consume the panel directly instead of scraping._

  ```bash
  mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --format json --agent
  ```

### Trust the numbers
- **`verify allocation`** — Check that fund asset-class percentages net to 100 and flag months whose percent columns are unpopulated.

  _A month that fails the invariant is corrupt input, not a weak signal, and must be excluded before modelling._

  ```bash
  mufap-pp-cli verify allocation --month 2026-07 --max-amcs 1 --agent
  ```
- **`universe`** — Report how many funds reported on each date, by sector and category.

  _A silently narrowing universe fakes verdicts, so width is printed alongside every cross-sectional result._

  ```bash
  mufap-pp-cli universe --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`freshness`** — Detect funds whose published NAV validity date lags the requested date.

  _Differencing the live view without this check manufactures returns that never happened._

  ```bash
  mufap-pp-cli freshness --agent
  ```

## Recipes

### Build the short-rate series

```bash
mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent --select date,median_yield,fund_count
```

Returns one row per date with the cross-sectional median money-market yield and the number of funds behind it, so a thin date is visible rather than silently averaged.

### Industry equity exposure by month

```bash
mufap-pp-cli exposure --from 2026-07 --to 2026-07 --max-amcs 1 --agent
```

Sums the PKR stocks-and-equities column across every fund, giving the asset-side series to pair against mutual-fund net flows.

### Audit a month before trusting it

```bash
mufap-pp-cli verify allocation --month 2026-07 --max-amcs 1 --agent
```

Flags funds whose asset-class percentages do not net to 100 after subtracting liabilities, which the site's own 100% label hides.

### Check universe width before any cross-section

```bash
mufap-pp-cli universe --from 2026-09-01 --to 2026-09-04 --agent --select date,fund_count
```

Prints how many funds reported per date so a narrowing universe cannot fake a result.

### Export the panel for external analysis

```bash
mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --format jsonl
```

Streams the stored panel as newline-delimited JSON for loading into a research database without re-fetching.

## Data model gotchas

Properties of MUFAP's published tables, each measured during discovery rather than assumed.

- **Negatives are in accounting notation.** On 2026-09-04, tab=returns, 96 of 388 rows (24.7%) carried a parenthesised YTD and **zero** rows carried a leading minus. A parser that skips unparseable cells removes exactly the left tail: before this was decoded, equity YTD dispersion on 2026-09-03 read median **+3.15** over 17 funds instead of **-3.70** over 91 — the sign of the market was inverted.
- **The row key is `Sector | Category | Fund Name`.** Fund names are not unique within a date: 49 of 388 rows on 2026-09-04 (12.6%) collide on name alone, mostly because VPS pension funds legitimately repeat one name across their Money Market, Debt and Equity sub-fund series. `dump` emits the composite as `row_key`.
- **Columns differ per tab.** The fund-name column is `Fund Name` on `--tab returns` and `Fund` on all four other tabs. `--tab payout` has **no** `Validity Date` column — its date column is `Payout Date`, so `--from`/`--to` select on a different field there. `--tab pricing` and `--tab ter` are current reference data rather than a dated panel: they return 551 rows regardless of the date requested, so their row counts are not a universe width.
- **Percent columns are 0.0 before roughly 2024** while the PKR amount columns stay correct. Derive percentages as amount/Total, which is what `exposure` does.
- **`TotalPercentage` is the literal string `"100%"`**, not a computed check. `verify allocation` computes it.
- **`message: "No data found"` appears even when `data` is populated.** Gate on the parsed row count, never on that field.
- **The net-sales Total row is an invariant, not data.** `netsales monthly` returns SectorId 100 / Sector `Total` separately as `total` and excludes it from `rows`; summing it alongside the others double-counts the whole month.
- **The same VPS figures appear under two pension sector labels.** MUFAP renders identical voluntary-pension rows under both `Pension Funds (Open-End Funds)` and `Employer Pension Funds`. Summing both overstates pension flow by about 13% — 4,680 rows before de-duplication versus 4,072 after. `netsales monthly` drops the duplicates and counts them in `vps_duplicates_dropped`.
- **On the net-sales pages a dash means MISSING, not zero — the opposite of the daily tables' convention and of CDC's.** A category showing `-` did not report; it is not a category that reported zero. Those figures come back as `null`, never `0`. For 2026-05, 33 of 40 rows are non-reporting while the month's total flow is 9,458m.
- **Reconciling net sales needs a rounding-aware tolerance, not a fixed one.** MUFAP renders whole PKR millions, so summing n rows against a rounded total carries up to ±0.5·(n+1) of rounding error. Across the 25 months that carry a Total row every residual is ≤ 2.0 while the bound ranges 3.0–7.5, so **all 25 reconcile**; a fixed ±1.5 tolerance falsely fails three of them (2025-12, 2026-01, 2026-04). Both the sales and the redemption residual must be checked — 2026-04 is exact on sales and off by 2.0 on redemptions.
- **The investor-class feed is a partial slice.** It carries no net column (derive net as sales − redemptions, only where both are present) and its class totals failed to reconcile against the headline month total in 19 of 25 measured months. Use it as a coverage-matched comparison; never to apportion the industry total.
- **Most net-sales months are legitimately empty, and a challenge is not an empty month.** MUFAP serves a rendered page for every (Month, Year) whether or not it published data — 79 of the 104 months from 2018-01 to 2026-08 are empty this way, with a Total row whose cells are all dashes. Cloudflare also challenges these two paths intermittently (3 of 8 sequential requests when measured); that is retried with backoff and then reported as a challenge, never as an empty month.
- **Four date encodings.** `YYYY-MM-DD` for the daily and monthly range flags, `M-YYYY` (not zero-padded, not ISO) for `allocation --month`, `YYYY` for `unitholders --year`, and `Mon DD, YYYY` as displayed in the table. A wrong encoding returns HTTP 200 with an empty table, or HTTP 500 — never an informative error.

## Usage

Run `mufap-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `data.db` (the mirror every derived command reads), `feedback.jsonl`, learning-loop tables |
| `state` | Runtime state such as persisted queries and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

This CLI stores no secrets: there is no `credentials.toml`, no cookie jar on disk and no auth sidecar, so relocation never has to move a credential. It does move the mirror — a fresh root starts with an empty `data.db`, and the mirror-only commands return an empty result at exit 0 until `backfill` runs against it.

Each kind resolves independently. The ladder is:

1. Per-kind env var: `MUFAP_CONFIG_DIR`, `MUFAP_DATA_DIR`, `MUFAP_STATE_DIR`, or `MUFAP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `MUFAP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export MUFAP_HOME=/srv/mufap
mufap-pp-cli doctor
```

Under `MUFAP_HOME=/srv/mufap`, the four dirs resolve to `/srv/mufap/config`, `/srv/mufap/data`, `/srv/mufap/state`, and `/srv/mufap/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "mufap": {
      "command": "mufap-pp-mcp",
      "env": {
        "MUFAP_HOME": "/srv/mufap"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `MUFAP_DATA_DIR` overrides an explicit `--home` for that kind. Use `MUFAP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `MUFAP_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `mufap-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### allocation

Per-fund monthly asset allocation in PKR millions and percent

- **`mufap-pp-cli allocation --fund-code <int> --month <M-YYYY>`** - Asset allocation for one fund in one month. `--fund-code` takes the **integer** `fund` field from `funds by-amc`, never the FundID GUID (a GUID returns HTTP 500); `--month` is required and is M-YYYY, not zero-padded and not ISO.

  ```bash
  mufap-pp-cli allocation --fund-code 12766 --month 7-2026 --json
  ```

### amcs

Asset management companies (AMCs) registered with MUFAP

- **`mufap-pp-cli amcs`** - List the 27 asset management companies MUFAP tracks. Returns each AMC's GUID, which every fund lookup keys on, plus its display name. Start here: `funds by-amc` needs the GUID, not the name.

### dates

Reporting periods MUFAP has published

- **`mufap-pp-cli dates`** - List the Year/Month periods MUFAP has published industry statistics for. Use it to plan a monthly backfill window instead of guessing which months exist. Note the envelope's message field reads 'No data found' even when periods are returned; trust the row count.

### funds

Funds managed by an AMC, with category and pricing mechanism

- **`mufap-pp-cli funds`** - List funds for one AMC (AMCId is the GUID from `amcs list`)

### payouts

Announced fund payouts and distributions

- **`mufap-pp-cli payouts`** - List announced fund payouts and distributions, with the per-unit amount and the ex-NAV the payout is struck against. Use it to separate a NAV fall caused by a distribution from a genuine loss.

### unitholders

Unit-holder pattern by investor type and sector

- **`mufap-pp-cli unitholders --year <YYYY>`** - Unit-holder pattern for one calendar year

### vps

Voluntary Pension Scheme breakdowns

- **`mufap-pp-cli vps age-wise`** - VPS allocation broken down by contributor age band
- **`mufap-pp-cli vps retired-cash`** - Break down Voluntary Pension Scheme assets held as retired cash. Shows how much pension money has been moved out of market exposure into cash on retirement, which the headline VPS allocation does not separate.
- **`mufap-pp-cli vps withdrawals`** - Report cash withdrawn from Voluntary Pension Scheme funds. Pairs with `vps retired-cash` to show whether pension outflows are being drawn down or merely reallocated to cash.

### netsales

Monthly industry flow: sales, redemptions and net sales, and the same month split across nine investor classes. Both are HTML tables, parsed and reconciled rather than returned raw.

- **`mufap-pp-cli netsales monthly --month <1-12> --year <YYYY>`** - Sales, redemptions and net sales by sector and category, in PKR millions, reconciled against the sheet's own Total row.
- **`mufap-pp-cli netsales investor --month <1-12> --year <YYYY>`** - The same month across Individuals, Banking & Financial Institutions, Provident fund, Gratuity fund, Pension fund, Public Limited Companies, Associated Companies, Fund of funds and Other. **A partial slice — cannot apportion the industry total.**

### import (do not use)

`mufap-pp-cli --help` lists `import` as "Import data from JSONL file via API create/upsert calls". MUFAP publishes no write surface, so there is no endpoint behind it: it is inert generator scaffolding whose own examples are placeholders (`import <resource> --input data.jsonl`). Nothing in this CLI mutates remote state.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`mufap-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`mufap-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`mufap-pp-cli learnings list`** - Inspect taught rows
- **`mufap-pp-cli learnings forget <query>`** - Undo a teach
- **`mufap-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`mufap-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`mufap-pp-cli teach-pattern`** - Install a query/resource template up front
- **`mufap-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `MUFAP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `mufap-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
mufap-pp-cli allocation --fund-code 12766 --month 7-2026

# JSON for scripting and agents
mufap-pp-cli allocation --fund-code 12766 --month 7-2026 --json

# Filter to specific fields by name
mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --json --select date,median_yield,fund_count

# Dry run — show the request without sending
mufap-pp-cli allocation --fund-code 12766 --month 7-2026 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
mufap-pp-cli allocation --fund-code 12766 --month 7-2026 --agent
```

`allocation` takes the **integer** fund code from `funds by-amc` (the `fund` field), never the
FundID GUID — a GUID returns HTTP 500 — and `--month` is required in M-YYYY form.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select` returns only the fields you need, e.g. `--select date,median_yield,fund_count` on `rates` or `--select date,fund,NAV` on `panel`
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only** - MUFAP exposes no write surface, so this CLI does not create, update, delete, publish, send, or mutate remote resources. `--help` still lists a generated `import` command ("Import data from JSONL file via API create/upsert calls"): it is inert scaffolding with placeholder examples and no endpoint behind it — do not use it.
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
mufap-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `mufap-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/mufap-pp-cli/config.toml`; `--home`, `MUFAP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Two further environment variables are read directly by the config loader, in
addition to the path variables in **Paths & environment variables** above:

| Variable | Effect |
|----------|--------|
| `MUFAP_CONFIG` | Absolute path to the config file to load instead of the resolved `config.toml`. The `--config` flag wins over it; it in turn wins over the per-kind and `MUFAP_HOME` path ladder. |
| `MUFAP_BASE_URL` | Overrides the API base URL (default `https://www.mufap.com.pk`) for every request, including the URL shown by `--dry-run`. Intended for pointing the CLI at a mock, proxy or archive rather than at MUFAP. |

There is no API key and no `MUFAP_API_KEY`: MUFAP publishes these endpoints
without authentication, so `doctor` checks reachability rather than credentials.

## Known Gaps

1. **Six commands read the mirror, not MUFAP.** `rates`, `panel`, `dispersion`, `universe`, `coverage` and `dump` never call the site. Until `backfill` has stored the range they return an empty result at exit 0, with the reason on stderr only. This is by design — a fetch is a separate, dated, resumable act — but it means an agent that skips `backfill` gets silence rather than an error.
2. **`import` does nothing useful.** MUFAP has no create/upsert endpoint; the command is generator scaffolding that survived into the build. Its own examples are placeholders.
3. **`coverage` cannot separate "never attempted" from "every attempt failed".** The ledger records successful saves only and has no outcome column. Both appear as `never-fetched`.
4. **A `--max-amcs` run is a sample, not an industry total.** `exposure` and `verify allocation` both accept it, and both report `amcs_used`/`amcs_listed` so the sample is visible. Read those fields before quoting any figure.
5. **`which` has no confidence floor.** Exit 0 means at least one index entry shared a term with the query, not that the CLI can serve it: `which "book a flight to paris"` exits 0 with `exposure`. Check the score and description.
6. **Intraday and stock-level data do not exist here.** MUFAP publishes fund NAVs once per validity date; there are no PSX quotes, no index levels and no daily AUM (net assets are monthly).
7. **The three `vps` endpoint mirrors return no usable panel.** MEASURED 2026-09-07: `vps retired-cash` and `vps withdrawals` answer with an empty `Table` on every call, and `vps age-wise` answers with what is plainly MUFAP placeholder data -- 10 rows all dated `2023-09-01`, `Conventional` only (no Shariah scheme), and values that are almost all the literal `32`. All three also return `data: {}` or `data: null` intermittently, so a consumer must parse `data` defensively rather than assume the documented `Table`. Nothing in the CLI can fix this: these are faithful passthroughs of what the endpoints serve. Treat the `vps` group as a discovered surface, not a data source. The real pension panel is in the daily mirror instead: MUFAP files those funds under their own sectors, so `mufap-pp-cli panel --sector "Voluntary Pension Scheme (VPS)"` (6 VPS categories) and `--sector "Employer Pension Funds"` (2 more) return them with full NAVs and returns, and `allocation --fund-code` resolves any one of them.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every command returns an empty result and doctor reports a challenge page** — MUFAP requires the Chrome-fingerprint transport; confirm http_transport is browser-http and retry, as plain HTTP always returns HTTP 403.
- **A historical month of allocation data shows every percentage as zero** — Percent columns are unpopulated before roughly 2024; use the PKR amount columns and divide by Total, which is what `exposure` does.
- **allocation get returns HTTP 500** — Pass the integer fund code from `funds by-amc` to --fund-code, not the FundID GUID, and format --month as M-YYYY such as 7-2026.
- **A fetched date holds fewer funds than expected** — Run `universe` for that date; weekends and holidays legitimately return a small or empty panel, and `coverage` distinguishes that from a failed fetch.
- **Today's panel mixes several different validity dates** — The current-day view is a latest-available-per-fund fallback; run `freshness` to see the lag, and prefer an explicit historical date, which returns a clean single-date panel.
