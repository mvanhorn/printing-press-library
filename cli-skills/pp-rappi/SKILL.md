---
name: pp-rappi
description: "The first agent-native CLI for Rappi Mexico — read-only catalog browsing with offline SQLite snapshots, cross-city... Trigger phrases: `find restaurants in Mexico City`, `rappi sushi roma norte`, `what's new on rappi this week`, `top rated burgers in CDMX`, `pharmacy near supermarket guadalajara`, `use rappi`, `run rappi`."
author: "bobe"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - rappi-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/food-and-dining/rappi/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Rappi (Mexico) — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `rappi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install rappi --cli-only
   ```
2. Verify: `rappi-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/cmd/rappi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for rappi-pp-cli when a user or agent needs Rappi Mexico catalog data in a structured, repeatable, proxy-free shape: discovering restaurants by city or category, comparing store coverage across CDMX/GDL/MTY, snapshotting a neighborhood's catalog to track newcomers and closures, or finding restaurants open at a non-current time. Do not use this CLI for ordering, cart, payment, or account flows — it is read-only by design.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`restaurants diff`** — See newcomers and closures in any city + cuisine between two snapshots — answers "what's new in Roma Norte sushi this week" in one command.

  _Pick this over a raw list when the user asks 'what's new' — the diff is the answer; you don't have to compute it._

  ```bash
  rappi-pp-cli restaurants diff --city ciudad-de-mexico --category sushi --since 2026-04-01 --agent
  ```
- **`restaurants top`** — Top-rated restaurants with both a minimum rating AND a minimum review-count floor — the listicle-grade filter Rappi UI hides.

  _Reach for this when a user asks 'best burgers in Polanco' — it weeds out new restaurants with three perfect ratings._

  ```bash
  rappi-pp-cli restaurants top --city ciudad-de-mexico --category hamburguesas --min-rating 4.5 --min-reviews 100 --limit 10 --agent
  ```
- **`stores coverage`** — Cross-city, cross-store-type coverage matrix (CDMX × markets × pharmacies × liquor × express, all in one table) for the cities you sync.

  _Best command for retail analysts asking 'where is Rappi expanding'; one query gives the whole MX picture._

  ```bash
  rappi-pp-cli stores coverage --cities ciudad-de-mexico,guadalajara,monterrey --agent
  ```
- **`stores coverage-diff`** — Delta-vs-last-snapshot of the (city, store_type) coverage matrix — see store openings and store-type expansions over time.

  _Pair with weekly sync to spot expansion trends; agents can flag 'new pharmacy zone added in GDL'._

  ```bash
  rappi-pp-cli stores coverage-diff --since 2026-04-01 --agent
  ```
- **`restaurants by-neighborhood`** — Group restaurants by neighborhood within a city (Polanco vs Condesa vs Roma Norte) and rank by count or top-rated per neighborhood.

  _When the question is 'which neighborhood has the most sushi options' or 'top-rated pizza per neighborhood' — this is the answer._

  ```bash
  rappi-pp-cli restaurants by-neighborhood --city ciudad-de-mexico --category pizza --agent
  ```
- **`restaurants multi-category`** — Restaurants listed under two or more cuisine categories — surfaces fusion places and mis-categorized spots in one query.

  _Pick this when a user wants 'fusion sushi-mexicana' or to disambiguate a chain with multiple cuisine listings._

  ```bash
  rappi-pp-cli restaurants multi-category --city ciudad-de-mexico --agent
  ```
- **`restaurants brand`** — Find every city × category where a restaurant brand (e.g., "Sushi Itto") appears in the synced catalog.

  _Reach for this on chain-coverage questions and multi-city expansion analysis._

  ```bash
  rappi-pp-cli restaurants brand --name "Sushi Itto" --agent
  ```

### Agent-native plumbing
- **`restaurants open`** — Restaurants open at an arbitrary local time (e.g., "23:30 on Sunday") parsed from schema.org openingHours — beyond Rappi's "open now" view.

  _Use this for late-night-eat queries and Sunday-morning planning where the live Rappi view is misleading._

  ```bash
  rappi-pp-cli restaurants open --city ciudad-de-mexico --at "23:30" --category sushi --agent
  ```
- **`restaurants near`** — Restaurants within a Haversine radius of a lat/lng with optional category filter — sorted by distance.

  _Best for proximity questions when the user has coordinates (address geocoded externally) and needs a precise radius._

  ```bash
  rappi-pp-cli restaurants near --lat 19.4216 --lng -99.1700 --radius-km 2 --category tacos --agent
  ```
- **`stores adjacency`** — Stores of type A within a Haversine radius of stores of type B (e.g., pharmacies within 1km of supermarkets) — for concierge-style "one-stop trip" planning.

  _Concierge agents picking a single trip route should reach for this over two independent radius queries. Requires `--fetch-detail` because list pages do not include store coordinates._

  ```bash
  rappi-pp-cli stores adjacency --type farmatodo --within-km 1 --of-type market --city ciudad-de-mexico --fetch-detail --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 10 API entries from 11 total network entries

## Command Reference

**catalog** — Alphabetized product catalog index

- `rappi-pp-cli catalog` — Browse the product catalog indexed by initial letter and page number

**promotions** — Public promotions and active campaigns

- `rappi-pp-cli promotions` — Public promotions landing page

**restaurants** — Restaurant catalog browsing via SSR list and detail pages

- `rappi-pp-cli restaurants get` — Fetch the restaurant detail page (name, cuisine, address, hours, geo, rating)
- `rappi-pp-cli restaurants list-category` — List restaurants in a city filtered by cuisine category (hamburguesas, pizza, sushi, tacos, etc.)
- `rappi-pp-cli restaurants list-city` — List restaurants in a Mexican city (e.g. ciudad-de-mexico, guadalajara, monterrey)

**stores** — Supermarket, pharmacy, liquor, and convenience store catalog

- `rappi-pp-cli stores get` — Fetch a store detail page (name, type, address, branding)
- `rappi-pp-cli stores list-by-type` — List stores by type (market for supermarkets, farmatodo for pharmacy, liquor, express, rappimall-parent)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
rappi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly newcomers in a neighborhood

```bash
rappi-pp-cli restaurants diff --city ciudad-de-mexico --category sushi --since 2026-04-01 --agent --select added,removed
```

Pulls the added and removed restaurant rows between the requested baseline and the latest snapshot — the answer to 'what's new this week.'

### Late-night options on a Sunday

```bash
rappi-pp-cli restaurants open --city ciudad-de-mexico --at "23:30" --day sunday --category tacos --agent
```

Walks synced restaurant openingHours and emits places open at the requested time + weekday.

### Cross-city retail coverage

```bash
rappi-pp-cli stores coverage --cities ciudad-de-mexico,guadalajara,monterrey,puebla --agent
```

One markdown/CSV table of store counts per (city, store_type) cell across four cities.

### Trim a big response with --select

```bash
rappi-pp-cli restaurants list-city --city ciudad-de-mexico --agent --select name,rating,review_count,url
```

Restaurant lists default to ~30 fields per row; --select narrows to just the four that matter for ranking.

### Brand expansion across MX

```bash
rappi-pp-cli restaurants brand --name "Sushi Itto" --agent
```

Fuzzy-matches the brand name across every synced (city, category) snapshot and emits a presence matrix.

## Auth Setup

No authentication required.

Run `rappi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  rappi-pp-cli promotions --agent --select id,name,status
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
rappi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
rappi-pp-cli feedback --stdin < notes.txt
rappi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.rappi-pp-cli/feedback.jsonl`. They are never POSTed unless `RAPPI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RAPPI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
rappi-pp-cli profile save briefing --json
rappi-pp-cli --profile briefing promotions
rappi-pp-cli profile list --json
rappi-pp-cli profile show briefing
rappi-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `rappi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add rappi-pp-mcp -- rappi-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which rappi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   rappi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `rappi-pp-cli <command> --help`.
