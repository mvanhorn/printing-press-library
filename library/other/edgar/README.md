# SEC EDGAR CLI

**An agent-native CLI for SEC EDGAR — six sanctioned endpoints, SQLite-cached filings with FTS5, and LODESTAR-shaped compound bundles (primary-sources, insider-summary) that no other EDGAR tool emits.**

Built for a Claude Code agent doing conviction research on public-company filings. Compound commands collapse a ticker's full primary-source pull into one structured response; insider-summary distinguishes discretionary code-S sales from RSU-tax code-F noise and flags senior officers separately. Local SQLite cache with tiered TTLs makes re-reads near-free; FTS5 over cached bodies eliminates re-hitting EDGAR for the same query.

## Install

The recommended path installs both the `edgar-pp-cli` binary and the `pp-edgar` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install edgar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install edgar --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/edgar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-edgar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-edgar --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-edgar skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-edgar. The skill defines how its required CLI can be installed.
```

## Authentication

No API key — SEC EDGAR is publicly accessible. Identity is the User-Agent: set COMPANY_PP_CONTACT_EMAIL once and every request goes out as `lodestar-edgar-pp-cli <email>`. The CLI refuses to run if the env var is unset, with a clear error pointing at setup. Rate-limited to ≤2 req/sec sustained (well under SEC's 10 req/sec ceiling) with adaptive backoff on 429.

## Quick Start

```bash
# SEC fair-access requires a real contact email in the User-Agent. Set once.
export COMPANY_PP_CONTACT_EMAIL=user@example.com


# Verifies UA env var, runs a reachability probe against data.sec.gov, opens the SQLite store, confirms FTS5.
edgar-pp-cli doctor


# LODESTAR primary-source bundle: latest 10-K, four 10-Qs, 90-day 8-Ks, 12-month Form 4s with senior-officer flag, latest DEF 14A — all in one structured response.
edgar-pp-cli primary-sources AAPL --json


# Form 4 aggregator with code-S (discretionary) vs code-F (RSU tax) separated and senior officers flagged.
edgar-pp-cli insider-summary AAPL --senior-only --since 12mo --json


# Recheck delta — only filings filed after the supplied timestamp, using the local cursor.
edgar-pp-cli since AAPL --as-of 2026-05-08 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`since`** — Return only the filings filed since a given timestamp, using a per-CIK local cursor — the entire LODESTAR /$recheck loop in one call.

  _For quarterly thesis rechecks, reach for this before fetching the full submissions index — it eliminates re-paying token cost on filings already seen._

  ```bash
  edgar-pp-cli since AAPL --as-of 2026-05-08 --json
  ```
- **`fts`** — Full-text search over locally-cached filing bodies via FTS5 — ticker- and form-scoped — with snippet windows and byte offsets for precise re-read.

  _Reach for fts during deep-dives where you re-query the same filing body multiple times; reach for the absorbed `efts` command for the first lookup or for cross-issuer queries._

  ```bash
  edgar-pp-cli fts "going concern" --ticker AAPL --form 10-Q --json
  ```

### Service-specific content patterns
- **`eightk-items`** — Enumerate 8-Ks with parsed Item numbers and a --material-only flag that excludes exhibits-only (Item 9.01 alone) refilings.

  _Use this instead of pulling 8-K bodies when you need 'has anything material happened?' — answers in one call without reading filing text._

  ```bash
  edgar-pp-cli eightk-items AAPL --since 2026-05-08 --material-only --json
  ```
- **`ownership-crosses`** — Enumerate 13D and 13G filings against an issuer (when someone else crosses 5% of the ticker), with filer name, percent owned, and filed-at.

  _Use in LODESTAR Gate 3 asymmetric-structure checks to spot activist or institutional concentration without scrolling submissions._

  ```bash
  edgar-pp-cli ownership-crosses AAPL --json
  ```
- **`governance-flags`** — Compose three independent service-specific signals into one call: 8-K Item 4.01 auditor changes, Item 4.02 non-reliance restatements, and NT-10-K late-filing notices (Form 12b-25).

  _Use as an early disqualifier check — if any flag fires, surface to LODESTAR before spending tokens on the full thesis._

  ```bash
  edgar-pp-cli governance-flags AAPL --since 2y --json
  ```

### Cross-entity joins
- **`insider-followthrough`** — For every senior-officer code-S sale of ≥$1M, scan the next 90 days of 8-Ks for material items and emit (sale, subsequent material 8-K, days-between) pairs.

  _Reach for this in LODESTAR Gate 2 execution-validation when an insider sale precedes material disclosures — surface management exits before bad news._

  ```bash
  edgar-pp-cli insider-followthrough AAPL --json
  ```

  **Form 4 ingest cap.** `insider-summary`, `insider-followthrough`, and `primary-sources` ingest Form 4 filings through a shared `--max-form4 N` cap (default `200`) that bounds DB/API pressure on high-volume filers (large biotechs during offerings, late-stage IPOs). When the cap clips older filings, the JSON output surfaces `form4_truncated: true` and `form4_total_in_window: <N>` under `form4_skipped`, and a stderr WARN fires so the truncation is never silent. Pass `--max-form4 0` to disable the cap entirely; pass a larger value to widen it.
- **`xbrl-pivot`** — Multi-ticker XBRL pivot that resolves concept aliases (Revenues ↔ RevenueFromContractWithCustomerExcludingAssessedTax ↔ SalesRevenueNet) into a flat ticker×quarter×concept table.

  _For cross-sectional quality screens — pivot before parsing 50 companyfacts JSON blobs by hand._

  ```bash
  edgar-pp-cli xbrl-pivot --tickers AAPL,MSFT,GOOGL --concepts Revenues,NetIncomeLoss --quarters 8 --csv
  ```

### Token-efficient extraction
- **`sections`** — Extract requested Items from a 10-K or 10-Q with byte-offset boundaries; emits ONLY the requested items in compact JSON instead of the full 100KB-10MB HTML body.

  _Use this instead of fetching the raw 10-K body — saves an order of magnitude in tokens when you only need Risk Factors and MD&A._

  ```bash
  edgar-pp-cli sections AAPL --form 10-K --items 1A,7,7A --json
  ```

## Usage

Run `edgar-pp-cli --help` for the full command reference and flag list.

## Commands

### companies

Company identifiers (ticker → CIK) and per-issuer submissions index

- **`edgar-pp-cli companies lookup`** - Resolve ticker → CIK (and company name + SIC) from SEC's nightly index. Cache 24h in local SQLite.
- **`edgar-pp-cli companies submissions`** - Structured submissions index for a company (filing history with accession numbers, form types, filed-at).

### companyfacts

XBRL company facts (financial concepts: revenue, net income, assets, cash flow, etc.) for a CIK

- **`edgar-pp-cli companyfacts get`** - All XBRL company facts for a CIK. Optionally filter to a single concept client-side.

### efts

EDGAR full-text search across all filings (efts.sec.gov). For offline FTS5 over cached filing bodies, use the offline `fts` command instead.

- **`edgar-pp-cli efts query`** - Online full-text search across EDGAR filings. For offline FTS5 over cached bodies, use `edgar-pp-cli fts`.

### filings

Per-form-type filing retrieval and individual filing document fetch

- **`edgar-pp-cli filings browse`** - EDGAR filing index for a CIK + form type. Returns HTML/Atom; the generator wraps it but parsing happens in the compound commands.
- **`edgar-pp-cli filings get`** - Fetch raw filing index page or document for a specific accession. Accession must be the no-dashes form (e.g., 000032019322000049).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
edgar-pp-cli companyfacts mock-value

# JSON for scripting and agents
edgar-pp-cli companyfacts mock-value --json

# Filter to specific fields
edgar-pp-cli companyfacts mock-value --json --select id,name,status

# Dry run — show the request without sending
edgar-pp-cli companyfacts mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
edgar-pp-cli companyfacts mock-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-edgar -g
```

Then invoke `/pp-edgar <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add edgar edgar-pp-mcp -e COMPANY_PP_CONTACT_EMAIL=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/edgar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COMPANY_PP_CONTACT_EMAIL` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "edgar": {
      "command": "edgar-pp-mcp",
      "env": {
        "COMPANY_PP_CONTACT_EMAIL": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
edgar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/edgar-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COMPANY_PP_CONTACT_EMAIL` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `edgar-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COMPANY_PP_CONTACT_EMAIL`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Exit code 4 with 'COMPANY_PP_CONTACT_EMAIL not set'** — export COMPANY_PP_CONTACT_EMAIL=user@example.com — SEC fair-access requires identification in the User-Agent.
- **Exit code 3 with 'rate limited (HTTP 429)'** — The 2 req/sec limiter is conservative but adaptive backoff is in play; if persistent, your IP may be in a 10-minute SEC penalty box — wait, then retry.
- **Exit code 7 with 'cache miss and --offline'** — Drop --offline or run edgar-pp-cli sync <TICKER> first to warm the local SQLite store.
- **Wrong ticker → CIK after corporate action** — edgar-pp-cli sync-tickers — refreshes company_tickers.json (24h TTL).
- **Form 4 amounts look wrong** — Use --senior-only and verify the column is is_discretionary; code F (RSU tax withholding) is non-directional and should be excluded from net sells.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**edgartools**](https://github.com/dgunning/edgartools) — Python (2100 stars)
- [**secedgar**](https://github.com/sec-edgar/sec-edgar) — Python (1400 stars)
- [**sec-edgar-mcp**](https://github.com/stefanoamorelli/sec-edgar-mcp) — Python (265 stars)
- [**sec-edgar-toolkit**](https://github.com/stefanoamorelli/sec-edgar-toolkit) — Python
- [**jadchaar/sec-edgar-downloader**](https://github.com/jadchaar/sec-edgar-downloader) — Python
- [**palafrank/edgar**](https://github.com/palafrank/edgar) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
