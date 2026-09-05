# NCCPL CLI

**Pakistan's clearing-layer data as a local research panel with a coverage audit and an arithmetic self-check, not another dashboard.**

NCCPL publishes the only per-symbol leverage, short-interest and free-float data in the Pakistani market - once, behind Cloudflare, one date at a time, with no export. Every tool built on it renders a chart and throws the history away. This one keeps it: sync backfills a local SQLite panel, panel emits it in the shape a regression consumes, coverage tells you exactly which sessions are missing, and verify proves each date against NCCPL's own arithmetic identities before you trust it.

Learn more at [NCCPL](https://www.nccpl.com.pk).

Created by [@qazmataz](https://github.com/qazmataz) (qazmataz).

## Install

The recommended path installs both the `nccpl-pp-cli` binary and the `pp-nccpl` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install nccpl
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install nccpl --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install nccpl --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install nccpl --agent claude-code
npx -y @mvanhorn/printing-press-library install nccpl --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/nccpl/cmd/nccpl-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nccpl-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install nccpl --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-nccpl --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-nccpl --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install nccpl --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
nccpl-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/nccpl-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/nccpl/cmd/nccpl-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "nccpl": {
      "command": "nccpl-pp-mcp"
    }
  }
}
```

</details>

## Authentication

NCCPL sits behind Cloudflare and a Laravel session, so there is no API key to set. Run 'nccpl-pp-cli auth login --chrome' to import the clearance and session cookies from a Chrome profile that has visited the site; the CLI then performs the CSRF handshake itself on each call. The Laravel session lasts about two hours and is renewed automatically, but the Cloudflare clearance cookie comes only from a real browser - if calls start returning challenge pages, re-run auth login.

## Quick Start

```bash
# confirm the binary and config resolve before touching the network
nccpl-pp-cli doctor --dry-run

# backfill investor-class flows; no browser or credentials needed
nccpl-pp-cli flows --from 2026-08-01 --to 2026-09-04

# preview the capture plan; add --launch to open the browser and fetch
nccpl-pp-cli capture --resources var-margins --latest-only

# prove each stored date against NCCPL's own arithmetic identities
nccpl-pp-cli verify

# see exactly which sessions landed and which are missing
nccpl-pp-cli coverage

# emit the panel in the shape a regression consumes
nccpl-pp-cli panel --resource var-margins --metrics free_float --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Research-grade integrity
- **`verify`** — Prove a date's flow numbers are internally consistent before they reach a regression.

  _Reach for this before feeding any NCCPL date into a model; a date that fails an invariant is corrupt input, not a weak signal._

  ```bash
  nccpl-pp-cli verify --from 2026-08-01 --to 2026-09-04 --agent
  ```
- **`coverage`** — List which sessions are missing per resource, how stale each one is, and how wide each date's data actually is.

  _Use this as a pipeline pre-flight - it exits non-zero on a gap, so a scheduled research job fails loudly instead of silently treating a missing session as a zero. Re-syncing a date replaces it rather than adding to it: a stored settlement date always mirrors the last snapshot fetched for it, so if NCCPL revises a date and drops symbols the next sync removes them and the store never serves a row the source stopped publishing. coverage row_count therefore always equals the number of observations actually stored, and the audit can never disagree with the data it audits. One deliberate exception: a fetch returning zero rows for a date that already holds rows does NOT delete them, because an empty response and a transient failure look identical from here; the next non-empty fetch mirrors the date correctly._

  ```bash
  nccpl-pp-cli coverage --resources fipi,lipi,var-margins --exit-code --agent
  ```
- **`contract-check`** — Assert every endpoint family still answers correctly for a date it just reported as its own latest.

  _Run this when results look empty - it separates an expired session or a changed request contract from a genuine no-data day._

  ```bash
  nccpl-pp-cli contract-check --agent
  ```

### Local history that compounds
- **`panel`** — Emit any synced resource as a tidy long-format panel, with gaps marked rather than filled.

  _This is the handoff from CLI to research store - it is the shape a regression consumes, and the observed_at column is what makes a flow number admissible as an ex-ante input._

  ```bash
  nccpl-pp-cli panel --resource fipi --from 2015-12-09 --to 2026-09-04 --agent
  ```
- **`universe`** — Reconstruct which symbols were listed and clearing-eligible on any past date.

  _Use this to state a screen's universe width in genuinely live names, and as a second opinion that can disagree with a price-staleness filter for reasons that filter cannot move for._

  ```bash
  nccpl-pp-cli universe --on 2019-03-15 --agent
  ```
- **`risk-changes`** — Date every step change in a symbol's free float, VAR margin and haircut.

  _Free float is the input a cap-weighted cross-section needs and no other public Pakistani source publishes it daily._

  ```bash
  nccpl-pp-cli risk-changes --since 30d --field free_float --agent
  ```

### Cross-market joins
- **`leverage`** — Join MTS, MFS and MSF open positions with SLB net open position into one per-symbol cross-section.

  _SLB net open position is the closest thing this market has to published short interest; pair it with MTS and MFS open interest to see leverage building in a name before a forced release._

  ```bash
  nccpl-pp-cli leverage --from 2026-08-01 --to 2026-09-04 --agent
  ```

### Reachability mitigation
- **`flows`** — Fetch daily FIPI/LIPI sector flows into the local store without a browser.

  _This is the one NCCPL dataset obtainable unattended, so it is what a scheduled daily job can actually collect._

  ```bash
  nccpl-pp-cli flows --from 2026-08-01 --to 2026-09-04 --agent
  ```
- **`ingest`** — Load NCCPL responses captured through your own browser into the same local store.

  _Reach for this when a dataset is gated: export a HAR from a normal browsing session and every other command works on it unchanged. Bodies can also be piped in directly with --stdin (pbpaste | nccpl-pp-cli ingest --stdin --resource var-margins --date 2026-09-04), so a single copied response never needs a temp file. Nothing unidentifiable is stored: a HAR entry is only ingested when it was served by nccpl.com.pk or a subdomain over https, and a response body must carry either the documented envelope key for the resource or exactly one of the API's own other envelope names. An all-traffic capture routinely holds other origins -- a proxy, a mock, a local dev server on the same path -- and those are refused and listed under skipped with a reason, never filed under a guess._

  ```bash
  nccpl-pp-cli ingest capture.har --agent
  ```
- **`capture`** — Fetch the gated NCCPL datasets through a controlled browser and store them locally; opt in with --launch, and add --headless to run with no window.

  _This is the only route to per-symbol free float, VAR margins, leverage positions and settlement data; every other command then reads them from the local store with no browser involved. With --headless no window appears and the run is unattended, so a scheduled job can keep the gated datasets current; Chrome must be installed because the clearance cannot be replayed by any non-browser HTTP client._

  ```bash
  nccpl-pp-cli capture --resources var-margins --latest-only --agent
  ```

## Recipes

### Backfill the foreign-flow archive and check it landed

```bash
nccpl-pp-cli sync --resources fipi,lipi --full && nccpl-pp-cli coverage --resources fipi,lipi --exit-code
```

Fills the local store as far back as NCCPL serves, then fails non-zero if any session is missing so a scheduled job cannot silently proceed on a hole.

### Emit a regression-ready flow panel with only the fields a model needs

```bash
nccpl-pp-cli panel --resource fipi --from 2015-12-09 --to 2026-09-04 --agent --select date,metric,value,observed_at
```

Long-format rows with the vintage stamp that establishes ex-ante availability, narrowed to four columns so an agent does not parse the whole payload.

### Find every symbol whose free float stepped this quarter

```bash
nccpl-pp-cli risk-changes --since 90d --field free_float --agent
```

Dates each change and prints symbols scanned versus symbols changed, so an empty result is distinguishable from an unscanned universe.

### Scan for leverage building ahead of a forced release

```bash
nccpl-pp-cli leverage --from 2026-08-01 --to 2026-09-04 --agent
```

One cross-section joining MTS, MFS and MSF open interest with SLB net open position, with force-release events flagged against the open interest they hit.

### Audit a date before trusting it

```bash
nccpl-pp-cli verify --from 2026-09-01 --to 2026-09-04 --agent
```

Checks FIPI net against minus LIPI net and each sector row's netting, returning a quarantine list of dates that fail.

## Usage

Run `nccpl-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `NCCPL_CONFIG_DIR`, `NCCPL_DATA_DIR`, `NCCPL_STATE_DIR`, or `NCCPL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `NCCPL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export NCCPL_HOME=/srv/nccpl
nccpl-pp-cli doctor
```

Under `NCCPL_HOME=/srv/nccpl`, the four dirs resolve to `/srv/nccpl/config`, `/srv/nccpl/data`, `/srv/nccpl/state`, and `/srv/nccpl/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "nccpl": {
      "command": "nccpl-pp-mcp",
      "env": {
        "NCCPL_HOME": "/srv/nccpl"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `NCCPL_DATA_DIR` overrides an explicit `--home` for that kind. Use `NCCPL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `NCCPL_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `nccpl-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### fipi

Foreign Investors Portfolio Investment (FIPI) net flows by investor class and market segment.

- **`nccpl-pp-cli fipi data`** - Foreign Investors Portfolio Investment (FIPI) net flows by investor class and market segment. Date range. NOTE: this endpoint requires DD/MM/YYYY dates, unlike the sector-wise and single-date endpoints which use YYYY-MM-DD.
- **`nccpl-pp-cli fipi latest-date`** - Most recent publication date available for fipi. Needs Cloudflare clearance only; no CSRF or session.

### fipi-normal

FIPI buy/sell volume and value by client type and market type for one date.

- **`nccpl-pp-cli fipi-normal data`** - FIPI buy/sell volume and value by client type and market type for one date. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli fipi-normal latest-date`** - Most recent publication date available for fipi-normal. Needs Cloudflare clearance only; no CSRF or session.

### fipi-sector

FIPI net flows broken out by market sector.

- **`nccpl-pp-cli fipi-sector data`** - FIPI net flows broken out by market sector. Date range. NOTE: this endpoint requires YYYY-MM-DD dates, unlike the fipi/lipi endpoints which use DD/MM/YYYY.
- **`nccpl-pp-cli fipi-sector latest-date`** - Most recent publication date available for fipi-sector. Needs Cloudflare clearance only; no CSRF or session.

### lipi

Local Investors Portfolio Investment (LIPI) net flows by investor class and market segment.

- **`nccpl-pp-cli lipi data`** - Local Investors Portfolio Investment (LIPI) net flows by investor class and market segment. Date range. NOTE: this endpoint requires DD/MM/YYYY dates, unlike the sector-wise and single-date endpoints which use YYYY-MM-DD.
- **`nccpl-pp-cli lipi latest-date`** - Most recent publication date available for lipi. Needs Cloudflare clearance only; no CSRF or session.

### lipi-normal

LIPI buy/sell volume and value by client type and market type for one date.

- **`nccpl-pp-cli lipi-normal data`** - LIPI buy/sell volume and value by client type and market type for one date. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli lipi-normal latest-date`** - Most recent publication date available for lipi-normal. Needs Cloudflare clearance only; no CSRF or session.

### lipi-sector

LIPI net flows broken out by market sector.

- **`nccpl-pp-cli lipi-sector data`** - LIPI net flows broken out by market sector. Date range. NOTE: this endpoint requires YYYY-MM-DD dates, unlike the fipi/lipi endpoints which use DD/MM/YYYY.
- **`nccpl-pp-cli lipi-sector latest-date`** - Most recent publication date available for lipi-sector. Needs Cloudflare clearance only; no CSRF or session.

### market

Market-wide traded value and volume series.

- **`nccpl-pp-cli market latest`** - Most recent market-wide traded value or volume series. Needs Cloudflare clearance only.
- **`nccpl-pp-cli market range`** - Market-wide traded value or volume series over an explicit date range. Sent as multipart/form-data with YYYY-MM-DD dates.

### mfs

Murabaha Share Financing (MFS) open positions per symbol, with free-float percentages.

- **`nccpl-pp-cli mfs data`** - Murabaha Share Financing (MFS) open positions per symbol, with free-float percentages. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mfs latest-date`** - Most recent publication date available for mfs. Needs Cloudflare clearance only; no CSRF or session.

### mfs-top

Top 15 MFS financee / financier pairs.

- **`nccpl-pp-cli mfs-top data`** - Top 15 MFS financee / financier pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mfs-top latest-date`** - Most recent publication date available for mfs-top. Needs Cloudflare clearance only; no CSRF or session.

### msf

Margin Sharia Financing (MSF) open positions per symbol.

- **`nccpl-pp-cli msf data`** - Margin Sharia Financing (MSF) open positions per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli msf latest-date`** - Most recent publication date available for msf. Needs Cloudflare clearance only; no CSRF or session.

### msf-top

Top 15 MSF buyer / seller pairs.

- **`nccpl-pp-cli msf-top data`** - Top 15 MSF buyer / seller pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli msf-top latest-date`** - Most recent publication date available for msf-top. Needs Cloudflare clearance only; no CSRF or session.

### mts

Margin Trading System (MTS) open positions per symbol.

- **`nccpl-pp-cli mts data`** - Margin Trading System (MTS) open positions per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mts latest-date`** - Most recent publication date available for mts. Needs Cloudflare clearance only; no CSRF or session.

### mts-financiers

Count of MTS financiers and financees per symbol.

- **`nccpl-pp-cli mts-financiers data`** - Count of MTS financiers and financees per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mts-financiers latest-date`** - Most recent publication date available for mts-financiers. Needs Cloudflare clearance only; no CSRF or session.

### mts-force-release

MTS force-release volume and value by date.

- **`nccpl-pp-cli mts-force-release data`** - MTS force-release volume and value by date. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mts-force-release latest-date`** - Most recent publication date available for mts-force-release. Needs Cloudflare clearance only; no CSRF or session.

### mts-refinanced

MTS amount released versus amount refinanced.

- **`nccpl-pp-cli mts-refinanced data`** - MTS amount released versus amount refinanced. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mts-refinanced latest-date`** - Most recent publication date available for mts-refinanced. Needs Cloudflare clearance only; no CSRF or session.

### mts-top-financiers

Top 15 MTS financier / financee pairs.

- **`nccpl-pp-cli mts-top-financiers data`** - Top 15 MTS financier / financee pairs. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli mts-top-financiers latest-date`** - Most recent publication date available for mts-top-financiers. Needs Cloudflare clearance only; no CSRF or session.

### settlement-cm

Trade versus settlement volume and value, clearing-member-wise, per symbol.

- **`nccpl-pp-cli settlement-cm data`** - Trade versus settlement volume and value, clearing-member-wise, per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli settlement-cm latest-date`** - Most recent publication date available for settlement-cm. Needs Cloudflare clearance only; no CSRF or session.

### settlement-uin

Trade versus settlement volume and value, UIN-wise, per symbol.

- **`nccpl-pp-cli settlement-uin data`** - Trade versus settlement volume and value, UIN-wise, per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli settlement-uin latest-date`** - Most recent publication date available for settlement-uin. Needs Cloudflare clearance only; no CSRF or session.

### slb

Securities Lending and Borrowing (SLB) open positions per symbol.

- **`nccpl-pp-cli slb data`** - Securities Lending and Borrowing (SLB) open positions per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli slb latest-date`** - Most recent publication date available for slb. Needs Cloudflare clearance only; no CSRF or session.

### tfc

Unlisted Term Finance Certificate (TFC) transactions.

- **`nccpl-pp-cli tfc data`** - Unlisted Term Finance Certificate (TFC) transactions. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli tfc latest-date`** - Most recent publication date available for tfc. Needs Cloudflare clearance only; no CSRF or session.

### var-margins

Value-at-Risk margin, haircut and free float per symbol.

- **`nccpl-pp-cli var-margins data`** - Value-at-Risk margin, haircut and free float per symbol. Single settlement date. The 'date' field must be YYYY-MM-DD.
- **`nccpl-pp-cli var-margins latest-date`** - Most recent publication date available for var-margins. Needs Cloudflare clearance only; no CSRF or session.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`nccpl-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`nccpl-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`nccpl-pp-cli learnings list`** - Inspect taught rows
- **`nccpl-pp-cli learnings forget <query>`** - Undo a teach
- **`nccpl-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`nccpl-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`nccpl-pp-cli teach-pattern`** - Install a query/resource template up front
- **`nccpl-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `NCCPL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `nccpl-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nccpl-pp-cli fipi data --from-date 01/09/2026

# JSON for scripting and agents
nccpl-pp-cli fipi data --from-date 01/09/2026 --json
# Filter to specific fields
nccpl-pp-cli fipi data --from-date 01/09/2026 --json --select client_type,segment,buy_value

# Dry run — show the request without sending
nccpl-pp-cli fipi data --from-date 01/09/2026 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nccpl-pp-cli fipi data --from-date 01/09/2026 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
nccpl-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `nccpl-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/nccpl-pp-cli/config.toml`; `--home`, `NCCPL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Known Gaps

**NCCPL's own API cannot be reached by any HTTP client.** This is measured, not assumed. A
byte-exact TLS fingerprint match to a real Chrome 149 (identical `ja4`, `ja4_r`,
`peetprint_hash` and HTTP/2 Akamai fingerprint), sending the exact browser header set and a
valid `cf_clearance` cookie, over both HTTP/2 and HTTP/3, receives the same Cloudflare
challenge as sending no cookies at all. Seventeen hypotheses were eliminated with evidence.

What this means in practice:

| Data | How to get it | Unattended? |
|---|---|---|
| FIPI/LIPI investor-class and sector flows | `flows` | yes |
| VAR margins, free float, MTS/MFS/MSF/SLB, settlement | `capture` (controlled Chrome; `--launch` to run, `--headless` for no window) | no |
| Anything already fetched | `panel`, `export`, `verify`, `coverage`, `universe`, `leverage`, `risk-changes` | yes, reads the local store only |

1. **The 21 generated NCCPL endpoint commands and `contract-check` will fail** with exit 4
   until a session exists. They are kept because they are correct and will work the day the
   gate changes.
2. **`flows` uses a different publisher.** scstrade republishes NCCPL's FIPI/LIPI numbers.
   `verify` checks NCCPL's own two arithmetic identities on every fetched date, so provenance
   drift surfaces as a failing invariant rather than a plausible wrong number -- but it is not
   NCCPL's own wire data.
3. **`capture` needs Chrome installed**, because NCCPL's Cloudflare clearance cannot be
   replayed by any non-browser HTTP client -- a byte-exact TLS fingerprint match with valid
   cookies receives the same challenge as sending none. `--headless` runs it with no window,
   which is what a scheduled job should use; without it a window appears. Either way it uses
   a throwaway profile, never your daily one, and deletes it afterwards. It is not the CLI's
   transport; every other command reads the local store.
4. **`free_float` is share count, not currency.** Multiply by a close price for free-float
   market cap. The dataset also contains futures contracts (`SYM-SEP`, `SYM-OCT`) that repeat
   their spot symbol's free float; filter to spot symbols before any cross-sectional work.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `nccpl-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Every NCCPL request returns a Cloudflare challenge.** — That is expected and permanent: the clearance cannot be replayed by an HTTP client. Use 'nccpl-pp-cli capture' for gated datasets and 'nccpl-pp-cli flows' for investor-class flows.
- **capture reports it could not clear the challenge.** — A Chrome window opens during capture; if it shows a checkbox, click it. Raise the wait with --solve-wait 120s if your connection is slow.
- **A command returns an empty result after a capture.** — Run 'nccpl-pp-cli coverage' - the resource may have no rows for that date, which is recorded distinctly from never having been fetched.
- **A resource looks frozen weeks behind the others.** — That is usually real. NCCPL resources publish on independent schedules; coverage reports each one's own latest date.
- **A date range returns nothing on the fipi or lipi commands.** — Those two endpoints require DD/MM/YYYY while the sector-wise and single-date endpoints require YYYY-MM-DD. Check the flag help for the format that command expects.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `NCCPL_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.nccpl.com.pk/
- Capture coverage: 51 API entries from 204 total network entries
- Reachability: browser_http (78% confidence)
- Protocols: rest_json (75% confidence), html_scrape (55% confidence)
- Protection signals: cloudflare (90% confidence)
- Generation hints: browser_http_transport, requires_protected_client
- Candidate command ideas: create_data_by_date_range — Derived from observed POST /api/graph-data/data-by-date-range traffic.; create_rum — Derived from observed POST /cdn-cgi/rum traffic.; list_latest_data — Derived from observed GET /api/graph-data/latest-data traffic.; list_latest_date — Derived from observed GET /api/financiers-financees/latest-date traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
