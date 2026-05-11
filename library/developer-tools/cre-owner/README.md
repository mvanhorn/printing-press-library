# CRE Owner CLI

**Find who owns any commercial building, pierce the LLC veil, and surface motivated-seller signals — all from free public records stitched into a local SQLite mirror**

CRE Owner aggregates county assessor records, State Secretary of State filings, SEC EDGAR, OpenCorporates, and Crexi listings into a single local database. Compound queries across this mirror surface hidden portfolios, distress signals, and outreach targets that no individual source can produce alone.

## Install

The recommended path installs both the `cre-owner-pp-cli` binary and the `pp-cre-owner` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cre-owner
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cre-owner --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cre-owner-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cre-owner --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cre-owner --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cre-owner skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cre-owner. The skill defines how its required CLI can be installed.
```

## Authentication

Foundation-tier commands (owner lookup, entity search, motivated scoring) work without any API keys — they query free public records. Crexi enrichment (listings, broker contacts) requires a Chrome session cookie: log in to Crexi in Chrome, then run `cre-owner-pp-cli auth login --chrome` to import the session. Optional paid hooks (Regrid, ATTOM, Trepp) are configured via environment variables when you have accounts.

## Quick Start

```bash
# Pull Lake County IN parcels, owners, tax records, and entities into local SQLite
cre-owner-pp-cli sync --market lake-county-in


# Look up owner, pierce the LLC, show the full entity chain to beneficial owner
cre-owner-pp-cli owner '123 Broadway, Merrillville, IN' --chain


# Surface motivated sellers ranked by distress signals
cre-owner-pp-cli motivated --market lake-county-in --min-score 60


# Build a cold-outreach list with contacts and mailing addresses
cre-owner-pp-cli outreach --market lake-county-in --property-type industrial --export csv


# Find every building this entity controls across all its LLCs
cre-owner-pp-cli portfolio 'Lakefront Holdings LLC'

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Entity intelligence
- **`portfolio`** — Find all buildings owned by the same beneficial owner across multiple LLCs

  _When an agent needs to understand the full scope of a CRE owner's holdings hidden behind multiple shell LLCs_

  ```bash
  cre-owner-pp-cli portfolio 'Lakefront Holdings LLC' --depth 3 --json
  ```
- **`network`** — Discover hidden partnerships by finding LLCs that share officers, registered agents, or mailing addresses with a target entity

  _When an agent needs to map the full network of related entities and co-investors behind a commercial property portfolio_

  ```bash
  cre-owner-pp-cli network 'Midwest Realty Group LLC' --depth 2 --json
  ```
- **`owners chain`** — Visual tree showing LLC to officers to other LLCs to beneficial owner chain from multi-source traversal

  _When an agent needs to understand the corporate structure behind a commercial property to find the actual decision-maker_

  ```bash
  cre-owner-pp-cli owners chain --owner-id 550e8400 --json
  ```

### Deal sourcing signals
- **`motivated`** — Ranked deal-sourcing list combining tax delinquency, hold duration, out-of-state ownership, and code violations into a single distress score

  _When an agent is sourcing off-market CRE deals and needs to rank properties by likelihood of motivated seller_

  ```bash
  cre-owner-pp-cli motivated --market lake-county-in --min-score 70 --json
  ```
- **`tax-countdown`** — Ranked list of properties approaching tax sale deadlines, giving investors a precise window to approach desperate owners

  _When an agent is identifying the highest-urgency motivated sellers — owners about to lose their property to tax sale_

  ```bash
  cre-owner-pp-cli tax-countdown --market lake-county-in --within 6mo --json
  ```
- **`comp-gap`** — Compare a property's assessed value against recent comparable sales to find value arbitrage opportunities

  _When an agent is evaluating whether a property is undervalued relative to market and worth pursuing as an off-market deal_

  ```bash
  cre-owner-pp-cli comp-gap --address '123 Broadway, Merrillville, IN' --radius 0.5mi --json
  ```

### Outreach automation
- **`outreach`** — Ranked cold-outreach list with mailing addresses, registered agent info, and cross-source contact confidence scores

  _When an agent is building a targeted mailing or calling campaign for CRE building owners_

  ```bash
  cre-owner-pp-cli outreach --market lake-county-in --type industrial --csv
  ```
- **`package`** — Generate a one-page dossier for a target owner's entire portfolio with property count, values, tax exposure, entity status, and contacts

  _When an agent is preparing outreach materials or meeting prep for a specific CRE portfolio owner_

  ```bash
  cre-owner-pp-cli package 'Midwest Realty Group LLC' --json
  ```

## Usage

Run `cre-owner-pp-cli --help` for the full command reference and flag list.

## Commands

### brokers

CRE broker profiles and contact info from Crexi

- **`cre-owner-pp-cli brokers get`** - Get broker profile with contact info
- **`cre-owner-pp-cli brokers search`** - Search brokers by name

### comps

Sold comparables from Crexi and county recorder

- **`cre-owner-pp-cli comps search`** - Search sold comparables by market, type, and price range

### edgar

REIT filings and beneficial ownership from SEC EDGAR

- **`cre-owner-pp-cli edgar search`** - Full-text search of SEC filings
- **`cre-owner-pp-cli edgar submissions`** - Get filing history for a company by CIK

### entities

Business entities (LLCs, corporations) from State SoS and OpenCorporates

- **`cre-owner-pp-cli entities get`** - Get entity details including officers, registered agent, filing history
- **`cre-owner-pp-cli entities officers`** - List officers and registered agent for an entity
- **`cre-owner-pp-cli entities search`** - Search business entities by name

### listings

Active and sold CRE listings from Crexi

- **`cre-owner-pp-cli listings brokers`** - Get broker contacts for a listing
- **`cre-owner-pp-cli listings get`** - Get full listing details from Crexi
- **`cre-owner-pp-cli listings search`** - Search active CRE listings on Crexi

### owners

Property owners — individuals and entities

- **`cre-owner-pp-cli owners chain`** - Full entity chain — LLC to officers to beneficial owner
- **`cre-owner-pp-cli owners lookup`** - Look up owner of record for an address or parcel

### parcels

Property parcels from county assessor records

- **`cre-owner-pp-cli parcels get`** - Get full parcel details including owner, assessed value, tax history
- **`cre-owner-pp-cli parcels search`** - Search parcels by address, owner name, or parcel ID

### tax_records

Tax assessment and delinquency records from county assessor

- **`cre-owner-pp-cli tax_records get`** - Get tax assessment history for a parcel
- **`cre-owner-pp-cli tax_records search`** - Search tax records by parcel, owner, or delinquency status


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cre-owner -g
```

Then invoke `/pp-cre-owner <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
cre-owner-pp-cli auth login --chrome

claude mcp add cre-owner cre-owner-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
cre-owner-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cre-owner-current).
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
    "cre-owner": {
      "command": "cre-owner-pp-mcp"
    }
  }
}
```

</details>

## Cookbook

```bash
# Who owns that building? Pierce the LLC and find the human behind it.
cre-owner-pp-cli owners lookup --address '456 State St, Hammond, IN' --json

# Full ownership chain: LLC -> officers -> other LLCs -> beneficial owner
cre-owner-pp-cli owners chain --owner-id 550e8400 --depth 3 --json

# Every building this entity controls across all its shell LLCs
cre-owner-pp-cli portfolio 'Lakefront Holdings LLC' --depth 3 --json

# Top 20 most motivated sellers in a market, ranked by distress score
cre-owner-pp-cli motivated --market lake-county-in --min-score 60 --limit 20 --json

# Properties about to hit tax sale — highest urgency motivated sellers
cre-owner-pp-cli tax-countdown --market lake-county-in --within 6mo --json

# Is this building undervalued vs. comparable recent sales?
cre-owner-pp-cli comp-gap --address '123 Broadway, Merrillville, IN' --radius 0.5 --json

# Build a cold-outreach list for industrial owners in a market
cre-owner-pp-cli outreach --market lake-county-in --type industrial --csv

# Find hidden partnerships — LLCs sharing officers or registered agents
cre-owner-pp-cli network 'Midwest Realty Group LLC' --depth 2 --json

# Hot-potato properties — parcels that changed hands 3+ times in 2 years
cre-owner-pp-cli churn --market "Chicago" --min-turnover 3 --months 24 --json

# Market health snapshot: listing volume, median values, concentration
cre-owner-pp-cli market --market "Dallas" --json

# Search all synced data for an owner name across every resource type
cre-owner-pp-cli search "Smith Holdings" --json

# Properties held by dissolved LLCs — abandoned or distressed assets
cre-owner-pp-cli dormant --market lake-county-in --inactive-years 3 --json

# Owner dossier: portfolio count, values, tax exposure, contacts
cre-owner-pp-cli package 'Midwest Realty Group LLC' --json

# SQL query against the local mirror
cre-owner-pp-cli search "SELECT * FROM resources WHERE resource_type='parcels' LIMIT 5" --json
```

## Health Check

```bash
cre-owner-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cre-owner-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CREXI_SESSION_COOKIE` | per_call | No | Crexi session cookie for listings and broker data. Not needed for public-records commands. |
| `CRE_OWNER_BASE_URL` | config | No | Override the default API base URL (default: `https://api.crexi.com`). |
| `CRE_OWNER_CONFIG` | config | No | Override the config file path (default: `~/.config/cre-owner-pp-cli/config.json`). |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cre-owner-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CREXI_SESSION_COOKIE`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **County assessor returns empty results** — Run `sync --market lake-county-in --force` to refresh the local mirror. The assessor portal may have changed.
- **Crexi returns 403 or empty** — Your Crexi session cookie expired. Log in to Crexi in Chrome and re-run `auth login --chrome`.
- **OpenCorporates rate limited (200 req/mo)** — Entity lookups are cached locally. Run `sync` first, then queries hit the local mirror instead of the API.
- **Entity chain shows 'unknown' for officers** — Some states don't publish officer data publicly. Try `entity <name> --source inbiz` for Indiana-specific SoS lookup.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**sec-edgar-mcp**](https://github.com/stefanoamorelli/sec-edgar-mcp) — Python
- [**find-property-owner**](https://github.com/vitworks/find-property-owner) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
