# TenderNed CLI

**Every Dutch public tender, with the sub-threshold long tail that EU TED never sees, in a local-first CLI you can pipe.**

TenderNed is the central Dutch national procurement platform; all 143,000+ live notices are CC-0 licensed and freely accessible. This CLI puts every publication, every contracting authority, and every attached bestek into a local SQLite store you can grep, join, and aggregate offline. The novel commands — buyer dossier, sub-threshold leads, closing-deadline view, CPV drift over time, tender-thread reconcile — are workflows the official web UI cannot do.

Printed by [@markvandeven](https://github.com/markvandeven) (markvandeven).

## Install

The recommended path installs both the `tenderned-pp-cli` binary and the `pp-tenderned` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install tenderned
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tenderned --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install tenderned --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install tenderned --agent claude-code
npx -y @mvanhorn/printing-press install tenderned --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tenderned-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tenderned --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tenderned --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tenderned skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tenderned. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tenderned-current).
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
    "tenderned": {
      "command": "tenderned-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Most of TenderNed (search, list, document download, RSS) is unauthenticated and works out of the box. Only the full eForms XML endpoint requires a free username and password — request via functioneelbeheer@tenderned.nl, then export TENDERNED_USERNAME and TENDERNED_PASSWORD.

## Quick Start

```bash
# List the past two weeks of public-works tenders as JSON
tenderned-pp-cli notices list --agent


# Populate the local SQLite store so offline queries and novel commands work
tenderned-pp-cli sync


# Compute a full procurement profile for one contracting authority
tenderned-pp-cli buyer dossier "Gemeente Rotterdam" --since 2025-01-01 --agent


# Find sub-threshold construction tenders that never reach EU TED
tenderned-pp-cli leads --national --max-value 200000 --cpv 45000000-7 --since 2026-04-01 --agent


# Triage what's closing in the next two weeks
tenderned-pp-cli deadline --within 14 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local aggregations that beat the API
- **`buyer dossier`** — Full profile for one contracting authority: spending cadence, top CPVs, top procedures, active vs awarded vs cancelled counts over a window.

  _Reach for this when a user wants the full procurement picture of one buyer in one call instead of paginating through their notice list._

  ```bash
  tenderned-pp-cli buyer dossier Rotterdam --since 2025-01-01 --agent
  ```
- **`leads`** — Filter notices to national-only AND below an estimated-value cap AND inside CPV stripes — the long-tail tenders that EU TED never sees.

  _Reach for this when serving a Dutch SME or municipality-focused supplier whose target contracts are below EU thresholds._

  ```bash
  tenderned-pp-cli leads --national --max-value 200000 --cpv 45000000-7,71000000-8 --since 2026-04-01 --agent
  ```
- **`deadline`** — Lists notices closing within N days, grouped by day, with countdown plus buyer plus procedure filters.

  _Reach for this when triaging which open tenders need a response this week. Mirrors eu-tenders deadline._

  ```bash
  tenderned-pp-cli deadline --within 14 --cpv 45000000-7 --agent
  ```
- **`buyers top`** — Top contracting authorities by tender count for a CPV plus NUTS plus date slice.

  _Reach for this when the user asks 'which buyers are most active in this slice?'_

  ```bash
  tenderned-pp-cli buyers top --cpv 72000000-5 --nuts NL33 --since 2025-01-01 --agent
  ```
- **`concentration`** — Herfindahl-Hirschman index of award concentration for a CPV plus NUTS slice; tells you whether a market is dominated by one buyer or fragmented.

  _Reach for this when assessing whether to bid in a market dominated by a few incumbents. Mirrors eu-tenders concentration._

  ```bash
  tenderned-pp-cli concentration --cpv 72000000-5 --nuts NL --since 2024-01-01 --agent
  ```
- **`deadline-heat`** — Ranked calendar of expiring notices weighted by urgency times estimated value; daily prioritized view of what needs attention.

  _Reach for this when sequencing bid effort across many open deadlines. Mirrors eu-tenders deadline-heat._

  ```bash
  tenderned-pp-cli deadline-heat --days 14 --cpv 45000000-7 --agent
  ```

### Workflow primitives
- **`watch`** — Persist a filter set; watch run returns only NEW notices since the cursor's last advance.

  _Reach for this when a user wants a recurring filter without an external alerting system._

  ```bash
  tenderned-pp-cli watch run civils-watch --agent
  ```

### Time-series intelligence
- **`cpv-drift`** — Year-by-year top-CPV table for one buyer or NUTS region; shows whether a buyer's purchasing mix is shifting.

  _Reach for this when answering 'is this buyer's spending mix changing?' for a market-trend analysis. Mirrors eu-tenders cpv-drift._

  ```bash
  tenderned-pp-cli cpv-drift --buyer Amsterdam --years 3 --agent
  ```
- **`velocity`** — Weekly publication-count trend over the local snapshot; spot heating or cooling markets per CPV or buyer.

  _Reach for this when sizing market momentum for one CPV or buyer. Mirrors eu-tenders velocity._

  ```bash
  tenderned-pp-cli velocity --cpv 72000000-5 --weeks 12 --agent
  ```

### Cross-system bridges
- **`ted-link`** — Extracts the canonical TED publication number from the eForms XML, prints it plus the TED URL.

  _Reach for this when the user wants to pivot from a TenderNed notice to the EU-wide TED record (e.g., for use with the sibling eu-tenders CLI)._

  ```bash
  tenderned-pp-cli ted-link 425283 --agent
  ```
- **`thread reconcile`** — Walks PIN to CN to CAN to Modification chains for one buyer; flags orphan PINs, orphan CNs, and unresolved modifications.

  _Reach for this when a compliance officer is auditing whether announced tenders were actually awarded or cancelled._

  ```bash
  tenderned-pp-cli thread reconcile --buyer Eindhoven --since 2024-01-01 --agent
  ```

### Document corpus tools
- **`docs grep`** — Streams documents from matching notices, runs a regex against PDF and Word text, prints pub_id, doc_id, line hits.

  _Reach for this when the user wants to find every notice mentioning a clause, vendor, or technical requirement across many bestek documents._

  ```bash
  tenderned-pp-cli docs grep "clausule.*aansprakelijkheid" --cpv 45000000-7 --limit 50 --agent
  ```

## Usage

Run `tenderned-pp-cli --help` for the full command reference and flag list.

## Commands

### buyers

Browse contracting authorities (aanbestedende diensten) — Dutch public buyers

- **`tenderned-pp-cli buyers get`** - Fetch one contracting authority by ID
- **`tenderned-pp-cli buyers list`** - List Dutch contracting authorities (paginated)

### docs

List and download tender documents (bestek, PvE, evaluation criteria, Q&A)

- **`tenderned-pp-cli docs download`** - Download all documents for one publication as a zip archive
- **`tenderned-pp-cli docs get`** - Download a single document's binary content (PDF/Word/etc.)
- **`tenderned-pp-cli docs list`** - List attached documents for one publication

### notices

Search, list and fetch tender notices (aankondigingen) from TenderNed — mirrors 'eu-tenders notices' for the Dutch market

- **`tenderned-pp-cli notices get`** - Fetch full structured metadata for one publication
- **`tenderned-pp-cli notices list`** - Search and list tender publications with rich filters (CPV, dates, buyer, procedure, scope)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tenderned-pp-cli buyers list

# JSON for scripting and agents
tenderned-pp-cli buyers list --json

# Filter to specific fields
tenderned-pp-cli buyers list --json --select id,name,status

# Dry run — show the request without sending
tenderned-pp-cli buyers list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tenderned-pp-cli buyers list --agent
```

## Cookbook

Recipes for common Dutch-procurement workflows. All commands below use verified
flag names — run `tenderned-pp-cli <cmd> --help` for the full flag list.

```bash
# 1. Bootstrap a local snapshot for offline aggregations
tenderned-pp-cli sync
tenderned-pp-cli doctor   # verify cache freshness and connectivity

# 2. Search the live API for open construction tenders, JSON for scripting
tenderned-pp-cli notices list \
  --cpv-codes 45000000-7 \
  --publicatie-type AAO \
  --since 2026-01-01 \
  --json

# 3. Find every tender mentioning a liability clause in its attached PDFs
tenderned-pp-cli docs grep "aansprakelijkheidsclausule" \
  --cpv 45000000-7 \
  --limit 25 \
  --agent

# 4. Build a full procurement profile for one contracting authority
tenderned-pp-cli buyer dossier "Gemeente Rotterdam" \
  --since 2025-01-01 \
  --agent

# 5. Surface only the sub-threshold long tail TED never sees
tenderned-pp-cli leads \
  --national \
  --max-value 200000 \
  --cpv 45000000-7,71000000-8 \
  --since 2026-04-01 \
  --agent

# 6. Triage tenders closing in the next two weeks, ranked by urgency × value
tenderned-pp-cli deadline-heat --days 14 --cpv 45000000-7 --agent

# 7. Track how a buyer's CPV mix has shifted over the last three years
tenderned-pp-cli cpv-drift --buyer Amsterdam --years 3 --agent

# 8. Measure market concentration (HHI) for an IT-services slice
tenderned-pp-cli concentration --cpv 72000000-5 --nuts NL --since 2024-01-01 --agent

# 9. Spot heating or cooling markets via weekly publication velocity
tenderned-pp-cli velocity --cpv 72000000-5 --weeks 12 --agent

# 10. Persist a saved query and re-run later to get only NEW notices
tenderned-pp-cli watch add civils '{"cpvCodes":"45000000-7","nationaalOfEuropees":"NA"}'
tenderned-pp-cli watch run civils --agent

# 11. Pivot from a TenderNed publication ID to the TED record
tenderned-pp-cli ted-link 425283 --agent

# 12. Reconcile a buyer's PIN → CN → CAN tender lifecycle chains
tenderned-pp-cli thread reconcile --buyer Eindhoven --since 2024-01-01 --agent

# 13. Full-text search across the local snapshot
tenderned-pp-cli search "spoedeisende hulp" --limit 20 --json

# 14. Pipe results into jq for custom filtering
tenderned-pp-cli notices list --since 2026-01-01 --agent \
  | jq '[.[] | select(.estimated_value > 1000000)] | length'
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
tenderned-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/tenderned-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on `xml fetch`** — The eForms XML endpoint needs Basic auth. Set TENDERNED_USERNAME and TENDERNED_PASSWORD; request credentials from functioneelbeheer@tenderned.nl if you don't have them.
- **Empty results for valid-looking CPV filter** — TenderNed expects the full 8-digit-plus-check-digit CPV form (e.g. `45000000-7`, not `45000000`). Pass it with the dash and check digit.
- **Novel commands (dossier, leads, drift) return no data** — Run `tenderned-pp-cli sync` first — these commands query the local SQLite store, not the live API.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tenderned-analyse/code-example-tn-xml-api**](https://github.com/tenderned-analyse/code-example-tn-xml-api) — Python
- [**TheGabeMan/tenderned**](https://github.com/TheGabeMan/tenderned) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
