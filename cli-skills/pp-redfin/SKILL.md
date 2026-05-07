---
name: pp-redfin
description: "Stingray-backed Redfin CLI with the workflows the website can't do — saved-search diff, sold-price trends, $/sqft ranking, and offline SQL. Trigger phrases: `find homes for sale in <city>`, `watch redfin listings for <area>`, `rank houses by price per square foot`, `pull sold comps for <address>`, `compare these redfin listings`, `use redfin-pp-cli`, `run redfin`."
author: "rderwin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["redfin-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-cli@latest","bins":["redfin-pp-cli"],"label":"Install via go install"}]}}'
---

# Redfin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `redfin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install redfin --cli-only
   ```
2. Verify: `redfin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search homes for sale via Redfin's internal Stingray endpoints from the terminal, sync results to a local SQLite store, and run the workflows the website never built: diff a saved search week-over-week with `watch`, rank by $/sqft net of HOA with `rank`, pull sold comps for a subject property with `comps`, surface price drops or stale listings with `drops`, and overlay market trends across multiple cities with `trends`. Every command is `--json` / `--select`-shaped so an agent can pipe the output without burning context.

## When to Use This CLI

Use this CLI when an agent or human needs structured Redfin data and needs the workflows the website itself doesn't expose: cross-region diffs over time, $/sqft rankings net of HOA, sold-comp recipes for a subject property, market-trends overlays across cities, and digest-style summaries. Reach for it for buyer comparison-shopping, investor screening across multiple zips, relocator city comparisons, and buyer's-agent comp pulls. Skip it for one-off browsing — the redfin.com website is fine for that.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Time-series intelligence
- **`watch`** — Re-run a saved gis search and surface what's NEW, REMOVED, PRICE-CHANGED, or STATUS-CHANGED since the last sync.

  _Pick this when an agent is tracking a buyer's shortlist over time and needs a reproducible 'what changed' digest._

  ```bash
  redfin-pp-cli watch austin-3br --since 7d --json
  ```
- **`drops`** — List active listings whose price dropped by N% in a window, OR whose days-on-market exceed a threshold.

  _Pick this when timing the market or surfacing lowball candidates before tour scheduling._

  ```bash
  redfin-pp-cli drops --region-id 30772 --region-type 6 --since 7d --min-pct 3 --dom-min 30 --json
  ```

### Local-store math
- **`rank`** — Rank synced listings by price-per-sqft, with optional HOA-fee subtraction over a 5-year horizon.

  _Pick this when value-per-dollar is the goal and HOA-heavy condos must compete fairly against single-family._

  ```bash
  redfin-pp-cli rank --by price-per-sqft --net-hoa --region-id 30772 --region-type 6 --json --limit 25
  ```

### Shortlist workflows
- **`compare`** — Pull 2-8 listings through the combined Stingray detail endpoint and emit aligned columnar output (price, $/sqft, beds, baths, lot, year, schools, AVM delta, last sale, taxes).

  _Pick this when narrowing a shortlist; the wide table makes school-rating and AVM-delta differences obvious._

  ```bash
  redfin-pp-cli compare <your-listing-url> <another-listing-url> --json
  ```
- **`comps`** — For a subject listing, derive a circular polygon from --radius, run a sold-status search, filter by --sqft-tol and --bed-match, return the ranked comp set.

  _Pick this when an agent needs to pull comparable sales for a buyer offer; collapses 20 minutes of polygon-clicking into one command._

  ```bash
  redfin-pp-cli comps <your-listing-url> --radius 0.5 --sqft-tol 15 --months 6 --bed-match --json
  ```

### Cross-market joins
- **`rank`** — Union synced listings across multiple region slugs and rank across the entire set, deduped by listing URL.

  _Pick this when an agent needs a single ranked feed across multiple metros without writing a fan-out loop._

  ```bash
  redfin-pp-cli rank --regions 30772,30773,30774 --by price-per-sqft --beds-min 3 --price-max 600000 --json --limit 25
  ```
- **`trends`** — Pull aggregate-trends for N regions and emit one tidy long table (region × month × metric) over a window.

  _Pick this when a relocator is comparing cities and needs the medians overlaid on the same axis._

  ```bash
  redfin-pp-cli trends --regions 30743,18028,30739 --metric median-sale --period 24 --json
  ```

### Bulk extraction
- **`export`** — Slice the price space into bands, page-walk gis-csv per band until each returns under 350 rows, dedupe on listing URL, emit one CSV/JSON.

  _Pick this when you need every comp for a year, not the first 350 sorted by relevance._

  ```bash
  redfin-pp-cli export --region-slug "city/30772/TX/Austin" --status sold --year 2024 --csv > austin-sold-2024.csv
  ```

### Aggregations
- **`summary`** — Single command: active count, pending count, sold-90d count, median list, median sold, median DOM, median $/sqft, % with price drops, plus a trends snapshot.

  _Pick this when an agent needs the one-shot snapshot of a market for a buyer brief._

  ```bash
  redfin-pp-cli summary 30772:6 --json
  ```
- **`appreciation`** — For all child neighborhoods under a parent metro, call aggregate-trends and rank by YoY median-sale % change.

  _Pick this when a relocator or investor needs the 'where in this metro is hottest' answer._

  ```bash
  redfin-pp-cli appreciation --parent "city/30772/TX/Austin" --period 12 --json --limit 10
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Protocols: stingray-json-api (95% confidence)
- Generation hints: Strip the {}&& CSRF prefix from every Stingray JSON response before decoding, Use Surf with Chrome TLS fingerprint at runtime (UsesBrowserHTTPTransport), Conservative rate limit: 1 req/s default with adaptive backoff on 429, Stingray is geo-restricted to US IPs; doctor command should warn non-US users, Region IDs are visible in redfin.com URL paths (e.g., /city/30772/TX/Austin); region type 6=city, 1=zip, 11=neighborhood
- Candidate command ideas: homes — Stingray gis search is the primary entry point; listing — Listing detail composes initialInfo + aboveTheFold + belowTheFold; market — aggregate-trends endpoint exposes neighborhood medians
- Caveats: csrf-prefix: Stingray JSON responses are prefixed with the literal bytes '{}&&' as CSRF prevention. Generated client must strip them before json.Unmarshal.; geo-restricted: Stingray endpoints are US-only. Non-US callers will get 403 regardless of TLS fingerprint.

## Command Reference

**homes** — Search Redfin homes for sale via the internal Stingray /api/gis JSON endpoint.

- `redfin-pp-cli homes` — Run a Stingray gis search and return parsed listing rows from the JSON map payload. Strip the {}&& CSRF prefix...

**listing** — Fetch full listing detail by combining initialInfo, aboveTheFold, and belowTheFold Stingray calls.

- `redfin-pp-cli listing` — First Stingray call for a listing — returns the canonical listingId and propertyId from the URL path.

**market** — Aggregate market trends for a region (median sale price, days on market, supply, list-to-sale ratio) over a window.

- `redfin-pp-cli market` — Fetch aggregate-trends JSON for one region and period (months).


**Hand-written commands**

- `redfin-pp-cli homes` — Search Redfin for-sale homes by city, beds, baths, price, etc.
- `redfin-pp-cli sold` — Search Redfin sold homes by city + filters (delegates to homes with status=sold).
- `redfin-pp-cli listing <url-or-path>` — Fetch full listing detail (initialInfo + aboveTheFold + belowTheFold combined).
- `redfin-pp-cli region` — Region tools: resolve a city/zip to a Redfin region_id.
- `redfin-pp-cli feed` — Tail Redfin's RSS feeds: new listings, updated listings.
- `redfin-pp-cli sync-search <saved-search>` — Run a saved search and snapshot results into the local store.
- `redfin-pp-cli watch <saved-search>` — Diff the latest sync against the previous: NEW / REMOVED / PRICE-CHANGED / STATUS-CHANGED listings.
- `redfin-pp-cli rank` — Rank synced listings by ratio metrics — $/sqft (optionally net of HOA), $/bed.
- `redfin-pp-cli compare <url-or-id...>` — Pivot 2-8 listings into a wide table — one column per listing — with $/sqft, AVM delta, school ratings.
- `redfin-pp-cli comps <subject-url>` — Pull sold comps for a subject listing within a radius, sqft tolerance, and recency window.
- `redfin-pp-cli drops` — List active listings whose price dropped by N% in a window OR whose DOM exceeds a threshold.
- `redfin-pp-cli market <region-slug>` — Single-region snapshot: counts, medians, DOM, % drops, plus trends call.
- `redfin-pp-cli summary <region-slug>` — Neighborhood market summary: active/pending/sold-90d counts, medians, DOM, % drops.
- `redfin-pp-cli trends` — Multi-region aggregate-trends overlay; emits tidy table (region × month × metric).
- `redfin-pp-cli appreciation` — Rank child neighborhoods under a metro by YoY median-sale % change.
- `redfin-pp-cli export` — Bulk export past Redfin's 350-row gis-csv cap by slicing price bands and deduping on URL.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `REDFIN_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `redfin-pp-cli homes`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
redfin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Three-metro relocator overlay

```bash
redfin-pp-cli trends --regions 30743,18028,30739 --metric median-sale --period 24 --json --select rows.region,rows.month,rows.value
```

Pulls aggregate-trends for all three metros over the same 24-month window, joins them into one tidy long table for export — the overlay Redfin's UI never produces.

### Buyer's-agent comp pull in one command

```bash
redfin-pp-cli comps <your-listing-url> --radius 0.5 --sqft-tol 15 --months 6 --bed-match --json
```

Resolves the subject listing, derives a 0.5-mile circle, pulls sold homes in the last 6 months matching beds and within 15% sqft, returns the ranked comp set ready for a buyer offer brief.

### Investor screen across 5 zips

```bash
redfin-pp-cli rank --regions 30772,30773,30774,30775,30776 --by price-per-sqft --net-hoa --beds-min 3 --price-max 600000 --status for-sale --json --limit 25
```

Unions synced searches across 5 Austin zips and ranks the full union by net-HOA $/sqft. Filter narrows to investor screen criteria; result is a single ranked feed Redfin cannot produce.

### Weekly diff for a saved search

```bash
redfin-pp-cli watch austin-3br --since 7d --json --select new_listings.url,new_listings.price,price_changed.url,price_changed.delta_pct
```

Composes the watch result with --select to emit only the columns the agent needs — agents can drop straight into a weekly buyer email without extra parsing.

### Bulk pull sold homes for 2024

```bash
redfin-pp-cli export --region-slug "city/30772/TX/Austin" --status sold --year 2024 --csv > austin-sold-2024.csv
```

Slices the price space into bands, page-walks gis-csv per band, dedupes on listing URL across bands, emits a single CSV that exceeds Redfin's 350-row cap.

## Auth Setup

No authentication required.

Run `redfin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  redfin-pp-cli homes --agent --select id,name,status
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
redfin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
redfin-pp-cli feedback --stdin < notes.txt
redfin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.redfin-pp-cli/feedback.jsonl`. They are never POSTed unless `REDFIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `REDFIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
redfin-pp-cli profile save briefing --json
redfin-pp-cli --profile briefing homes
redfin-pp-cli profile list --json
redfin-pp-cli profile show briefing
redfin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `redfin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add redfin-pp-mcp -- redfin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which redfin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   redfin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `redfin-pp-cli <command> --help`.
