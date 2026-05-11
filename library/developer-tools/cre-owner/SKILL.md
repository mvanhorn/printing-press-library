---
name: pp-cre-owner
description: "Find who owns any commercial building, pierce the LLC veil, and surface motivated-seller signals — all from free... Trigger phrases: `who owns this building`, `find the owner of`, `pierce the LLC`, `motivated sellers in`, `cold outreach list for`, `commercial property owner`, `tax delinquent properties`, `entity chain for`, `use cre-owner`, `run cre-owner`."
author: "Danilo Ojeda"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cre-owner-pp-cli
---

# CRE Owner — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cre-owner-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cre-owner --cli-only
   ```
2. Verify: `cre-owner-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

CRE Owner aggregates county assessor records, State Secretary of State filings, SEC EDGAR, OpenCorporates, and Crexi listings into a single local database. Compound queries across this mirror surface hidden portfolios, distress signals, and outreach targets that no individual source can produce alone.

## When to Use This CLI

Use CRE Owner when you need to find who owns a commercial building, understand their full portfolio across multiple LLCs, identify motivated sellers in a market, or build cold-outreach lists with contact info. It's the right tool for CRE investors doing off-market deal sourcing, wholesalers building mailing campaigns, and brokers researching ownership for prospecting. The local SQLite mirror means compound queries work offline and don't burn API quotas.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**brokers** — CRE broker profiles and contact info from Crexi

- `cre-owner-pp-cli brokers get` — Get broker profile with contact info
- `cre-owner-pp-cli brokers search` — Search brokers by name

**comps** — Sold comparables from Crexi and county recorder

- `cre-owner-pp-cli comps` — Search sold comparables by market, type, and price range

**edgar** — REIT filings and beneficial ownership from SEC EDGAR

- `cre-owner-pp-cli edgar search` — Full-text search of SEC filings
- `cre-owner-pp-cli edgar submissions` — Get filing history for a company by CIK

**entities** — Business entities (LLCs, corporations) from State SoS and OpenCorporates

- `cre-owner-pp-cli entities get` — Get entity details including officers, registered agent, filing history
- `cre-owner-pp-cli entities officers` — List officers and registered agent for an entity
- `cre-owner-pp-cli entities search` — Search business entities by name

**listings** — Active and sold CRE listings from Crexi

- `cre-owner-pp-cli listings brokers` — Get broker contacts for a listing
- `cre-owner-pp-cli listings get` — Get full listing details from Crexi
- `cre-owner-pp-cli listings search` — Search active CRE listings on Crexi

**owners** — Property owners — individuals and entities

- `cre-owner-pp-cli owners chain` — Full entity chain — LLC to officers to beneficial owner
- `cre-owner-pp-cli owners lookup` — Look up owner of record for an address or parcel

**parcels** — Property parcels from county assessor records

- `cre-owner-pp-cli parcels get` — Get full parcel details including owner, assessed value, tax history
- `cre-owner-pp-cli parcels search` — Search parcels by address, owner name, or parcel ID

**tax_records** — Tax assessment and delinquency records from county assessor

- `cre-owner-pp-cli tax_records get` — Get tax assessment history for a parcel
- `cre-owner-pp-cli tax_records search` — Search tax records by parcel, owner, or delinquency status


**Hand-written commands**

- `cre-owner-pp-cli search` — Search listings, parcels, and owners across all sources
- `cre-owner-pp-cli owners lookup` — Owner of record for an address or parcel
- `cre-owner-pp-cli portfolio` — All buildings owned by an entity or same beneficial owner
- `cre-owner-pp-cli motivated` — Ranked deal-sourcing list with motivated-seller signals
- `cre-owner-pp-cli outreach` — Ranked cold-outreach list with mailing/registered-agent/contact
- `cre-owner-pp-cli sync` — Refresh local SQLite mirror from all sources
- `cre-owner-pp-cli enrich` — Hand off to contact-goat-pp-cli for phone/email enrichment
- `cre-owner-pp-cli comps` — Sold comparables and transaction history
- `cre-owner-pp-cli market` — Market-level aggregation and heat maps
- `cre-owner-pp-cli entities search` — Business entity lookup via State SoS and OpenCorporates
- `cre-owner-pp-cli edgar` — SEC EDGAR REIT filings and beneficial ownership
- `cre-owner-pp-cli churn` — Track ownership changes — surfaces flippers and hot-potato properties
- `cre-owner-pp-cli dormant` — Properties held by dissolved or lapsed business entities
- `cre-owner-pp-cli tax-countdown` — Properties approaching tax sale deadline, ranked by urgency
- `cre-owner-pp-cli comp-gap` — Compare assessed value against recent comparable sales
- `cre-owner-pp-cli network` — Discover hidden partnerships via shared officers or agents
- `cre-owner-pp-cli package` — Generate a portfolio dossier for a target owner


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cre-owner-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find hidden portfolios in a market

```bash
cre-owner-pp-cli portfolio 'Lakefront Holdings LLC' --depth 3 --json --select entity_name,parcel_count,total_value
```

Uses the SQL interface to find multi-property owners ranked by total assessed value

### Motivated sellers with portfolio distress

```bash
cre-owner-pp-cli motivated --market lake-county-in --signal portfolio-distress --min-score 70 --json --select parcel_id,address,owner_name,signal_score,evidence
```

Tax-delinquent parcels owned by entities with 3+ buildings, filtered by distress score with key fields selected

### Owner chain for an address

```bash
cre-owner-pp-cli owners chain --owner-id 550e8400 --json --select name,entity_type,officers
```

Pierce the LLC veil and show the entity chain with key fields for outreach

### Export outreach list for direct mail

```bash
cre-owner-pp-cli outreach --market lake-county-in --type industrial --min-sqft 10000 --csv
```

Build a CSV mailing list of industrial building owners with contacts and registered agent addresses

### Tax sale countdown for urgent leads

```bash
cre-owner-pp-cli tax-countdown --market lake-county-in --within 6mo --json --select address,owner_name,days_remaining,amount_owed
```

Properties approaching tax sale deadline with key outreach fields selected

## Auth Setup

Foundation-tier commands (owner lookup, entity search, motivated scoring) work without any API keys — they query free public records. Crexi enrichment (listings, broker contacts) requires a Chrome session cookie: log in to Crexi in Chrome, then run `cre-owner-pp-cli auth login --chrome` to import the session. Optional paid hooks (Regrid, ATTOM, Trepp) are configured via environment variables when you have accounts.

Run `cre-owner-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cre-owner-pp-cli brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
cre-owner-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cre-owner-pp-cli feedback --stdin < notes.txt
cre-owner-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cre-owner-pp-cli/feedback.jsonl`. They are never POSTed unless `CRE_OWNER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CRE_OWNER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
cre-owner-pp-cli profile save briefing --json
cre-owner-pp-cli --profile briefing brokers get --broker-id 550e8400-e29b-41d4-a716-446655440000
cre-owner-pp-cli profile list --json
cre-owner-pp-cli profile show briefing
cre-owner-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `cre-owner-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cre-owner-pp-mcp -- cre-owner-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cre-owner-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cre-owner-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cre-owner-pp-cli <command> --help`.
