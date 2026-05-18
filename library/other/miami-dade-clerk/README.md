# Miami-Dade Clerk Official Records CLI

**Pull the full lien chain on any Miami-Dade property in 30 seconds — every deed, mortgage, satisfaction, lis pendens, and federal tax lien ever recorded.**

The Miami-Dade Clerk portal indexes deeds by property and everything else (mortgages, satisfactions, liens, lis pendens, judgments) by name. This CLI is the only tool that pivots property → owners → bounded name searches → filtered-back-to-property timeline, producing the chain-of-title and surviving-liens analysis that title companies charge $150-300 for.

Printed by [@alexkleis](https://github.com/alexkleis) (Alex Kleis).

## Install

The recommended path installs both the `miami-dade-clerk-pp-cli` binary and the `pp-miami-dade-clerk` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install miami-dade-clerk
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install miami-dade-clerk --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install miami-dade-clerk --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install miami-dade-clerk --agent claude-code
npx -y @mvanhorn/printing-press install miami-dade-clerk --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/miami-dade-clerk-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-miami-dade-clerk --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-miami-dade-clerk --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-miami-dade-clerk skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-miami-dade-clerk. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/miami-dade-clerk-current).
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
    "miami-dade-clerk": {
      "command": "miami-dade-clerk-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public records, no login required. The clerk portal is gated by reCAPTCHA Enterprise v3 (invisible scoring); the CLI mints a fresh token per call from a headless Chromium driver, so search commands work out of the box. `auth login --chrome` is only useful if you want to reuse an existing browser session cookie for the rare endpoints that benefit from sticky NetScaler affinity.

## Quick Start

```bash
# Confirm portal reachability + recording-cutoff date.
miami-dade-clerk-pp-cli doctor


# Smoke test against a known Hialeah SFR (returns 14 deeds, 1995-2022).
miami-dade-clerk-pp-cli search-property --address-no-unit '5600 W 13 AVE' --json


# The killer feature — full encumbrance timeline for one property.
miami-dade-clerk-pp-cli lien-chain --folio 30-2232-066-1610 --agent


# Bulk auction-prep: 30+ folios in one pass, Supabase-ingest-ready.
miami-dade-clerk-pp-cli enrich --folio-list auction-tuesday.csv --out tuesday-summary.json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`lien-chain`** — See every recording ever filed against a property in one chronological timeline — deeds, mortgages, satisfactions, lis pendens, federal tax liens, assignments — without manually deed-walking owner-by-owner.

  _When an agent is asked for the full encumbrance history on a Miami-Dade property (foreclosure underwriting, title research), this is the single command that produces it — bypassing the portal's split index._

  ```bash
  miami-dade-clerk-pp-cli lien-chain --folio 30-2232-066-1610 --agent
  ```
- **`surviving-liens`** — Tells you which liens on a property will survive a foreclosure or tax-deed sale, with totals in cents — Federal Tax Liens always survive tax deed; junior mortgages get wiped on senior foreclosure; HOA liens cap at FL Statute 720/718 safe-harbor amounts.

  _Computes the real Max Safe Bid for a foreclosure or tax-deed auction property. Pre-CLI, this number required a $150-300 O&E report from a title company._

  ```bash
  miami-dade-clerk-pp-cli surviving-liens --folio 30-2232-066-1610 --agent
  ```
- **`chain-of-title`** — Returns the ordered list of every deed conveying a property, with grantor, grantee, consideration, and recording date — and detects gaps where a grantee doesn't match the next deed's grantor.

  _The title-industry-standard 60-year chain-of-title deliverable, in one command. Agents can verify clean title before recommending a bid._

  ```bash
  miami-dade-clerk-pp-cli chain-of-title --folio 30-2232-066-1610 --since 1990-01-01 --agent
  ```

### Cross-entity insights
- **`owner-portfolio`** — Give a person or LLC name and see everything Miami-Dade has on them: every folio they appear on, every active mortgage or lien they've taken, every lis pendens or judgment against them.

  _Lead-gen for off-market deals and risk signal for already-distressed owners. Answers 'has this LLC defaulted elsewhere?' in one call._

  ```bash
  miami-dade-clerk-pp-cli owner-portfolio --last-name 'DEOD INVESTMENT LLC' --agent
  ```
- **`case-arc`** — Walks a single court case across all its recorded documents in chronological order: lis pendens → final judgment → certificate of title → satisfaction, with a status classifier (open / judgment-entered / sale-complete / dismissed).

  _Complements case-docket sources by adding the recording-layer view — when a judgment was actually recorded, when the certificate of title issued, whether subsequent satisfactions cleared the lien._

  ```bash
  miami-dade-clerk-pp-cli case-arc --case 2024-020991-CA-01 --agent
  ```

### Agent-native plumbing
- **`enrich`** — For a list of folios, compute lien-chain + surviving-liens + chain-of-title for each, and emit a single JSON file with totals_cents, surviving_lien_count, oldest_deed_date, last_deed_date, current_owner, FTL_count — designed for downstream ingest pipelines.

  _Lets a Tuesday-morning auction-prep workflow process 30+ properties in one invocation. The output JSON drops directly into a Supabase upsert._

  ```bash
  miami-dade-clerk-pp-cli enrich --folio-list folios.csv --out lien-summary.json
  ```
- **`ftl-scan`** — Find every Federal Tax Lien recorded after a given date, optionally filtered to a folio watchlist. FTLs survive tax-deed foreclosure, so any FTL on a TD-bound property is a deal-breaker red flag.

  _The single highest-signal alert for tax-deed investors. Today the v3 underwriter's `uw_td_govt_liens` column has no upstream feed._

  ```bash
  miami-dade-clerk-pp-cli ftl-scan --since 2024-01-01 --folio-list tuesday-auction.csv --agent
  ```

## Usage

Run `miami-dade-clerk-pp-cli --help` for the full command reference and flag list.

## Commands

### doc-types

Enumerate the 80+ recordable document types (3-letter codes + display labels)

- **`miami-dade-clerk-pp-cli doc-types`** - List all document types: codes (DEE, MOR, LIS, FTL, etc.) and display labels

### environment

Portal environment metadata (status, maintenance windows, current server date)

- **`miami-dade-clerk-pp-cli environment date`** - Get current server date
- **`miami-dade-clerk-pp-cli environment status`** - Get portal status (open / maintenance / outage) and last recording-cutoff date

### search-name

Search by party name (last + first) + optional document type. Returns name-indexed records: mortgages, satisfactions, assignments, lis pendens, judgments, federal tax liens, liens, court papers.

- **`miami-dade-clerk-pp-cli search-name`** - Submit a Name/Document search. Returns an encrypted qs token used to fetch the full result set.

### search-property

Search by property address. Returns property-indexed records (deeds + quit claim deeds only — mortgages and other liens are name-indexed, use search-name).

- **`miami-dade-clerk-pp-cli search-property`** - Submit a Property/Condo search. Returns an encrypted qs token used to fetch the full result set.

### search-results

Fetch the full record list for a previously-submitted search (using the qs token from search-name or search-property).

- **`miami-dade-clerk-pp-cli search-results`** - Get up to 500 recording records for a search. No native pagination; narrow by doc_type or date range to exceed cap.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
miami-dade-clerk-pp-cli doc-types

# JSON for scripting and agents
miami-dade-clerk-pp-cli doc-types --json

# Filter to specific fields
miami-dade-clerk-pp-cli doc-types --json --select id,name,status

# Dry run — show the request without sending
miami-dade-clerk-pp-cli doc-types --dry-run

# Agent mode — JSON + compact + no prompts in one flag
miami-dade-clerk-pp-cli doc-types --agent
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
miami-dade-clerk-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/miami-dade-clerk-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `miami-dade-clerk-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search returns `{isValidSearch:false,qs:null}`** — The address is not in the clerk's property index (folio renumbering, recent subdivision). Use `lien-chain` instead — its 4-layer fallback recovers the chain via owner-name search.
- **reCAPTCHA token rejected (HTTP 403 on search)** — Single-use token replayed or page-script blocked. Re-run the command (the helper mints a fresh token per call). If the page-load step itself fails, check that Google Chrome is installed locally — chromedp launches it headless to acquire the token.
- **Search returns exactly 500 records** — Hit the portal's 500-row cap. Narrow by `--doc-type <code>` (use `doc-types` to list) or shorter `--since`/`--until` window. The CLI's auto-pagination handles this when `--auto-narrow` is set.
- **Condo unit search fails with unit number from address line** — Use the unit from PA's legal_description (e.g. 'UNIT B312'), not the '#312' display form. The CLI does this automatically when given `--folio` instead of `--address`.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
