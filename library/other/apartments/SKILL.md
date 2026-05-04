---
name: pp-apartments
description: "The apartment-hunt CLI that actually works in 2026 — Surf-cleared bot protection plus a local SQLite store the website itself doesn't have. Trigger phrases: `find apartments in <city>`, `watch apartment listings for <area>`, `rank rentals by price per square foot`, `compare these apartments`, `use apartments-pp-cli`, `run apartments`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["apartments-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-cli@latest","bins":["apartments-pp-cli"],"label":"Install via go install"}]}}'
---

# Apartments.com — Printing Press CLI

Search every Apartments.com listing path-slug from the terminal, sync results to a local SQLite store, and run the workflows the website never built: diff a saved search week-over-week with `watch`, rank by $/sqft net of pet fees with `value`, compare a shortlist side-by-side with `compare`, and surface price drops or phantom listings with `drops`, `stale`, and `phantoms`. Every command is `--json`/`--select`-shaped so an agent can pipe the output without burning context.

## When to Use This CLI

Use this CLI when an agent or human needs structured Apartments.com data and needs the workflows the website itself doesn't expose: cross-search diffs over time, $/sqft and total-cost-of-occupancy rankings, multi-slug union queries, side-by-side comparison, and digest-style summaries. Reach for it for relocation tracking, value-per-dollar screens across shortlists, leasing-agent weekly digests, and any rental-search scenario that needs JSON + offline composition. Skip it for one-off browsing — the apartments.com website is fine for that.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Time-series intelligence
- **`watch`** — Re-run a stored search and surface what's NEW, REMOVED, or PRICE-CHANGED since the last sync.

  _Pick this when an agent is tracking a relocation over time and needs a reproducible 'what changed since last week' digest, not a fresh search._

  ```bash
  apartments-pp-cli watch austin-2br --json --since 7d
  ```
- **`drops`** — List listings whose max-rent dropped by ≥N% within a time window.

  _Pick this when timing the market or watching for distressed listings._

  ```bash
  apartments-pp-cli drops --since 14d --min-pct 5 --json
  ```
- **`stale`** — Flag listings whose price and availability haven't changed in N days — often phantom or stuck.

  _Pick this when a listing seems too good to be true; stale ones often are._

  ```bash
  apartments-pp-cli stale --days 30 --json --select url,maxrent,unchanged_days
  ```
- **`phantoms`** — Surface listings flagged by a three-signal join: 404 on re-fetch, dropped from saved-search results, or stale ≥45 days.

  _Pick this when prepping a shortlist for tour scheduling — phantoms waste tour slots._

  ```bash
  apartments-pp-cli phantoms --json
  ```
- **`history`** — Time-series of every observation of one listing — rent, availability, status.

  _Pick this when reasoning about a single listing's price trajectory._

  ```bash
  apartments-pp-cli history https://www.apartments.com/example-property-1234 --json
  ```

### Cross-market joins
- **`nearby`** — Fan out a search across multiple cities, zips, or neighborhoods and return one ranked, deduped list.

  _Pick this when an agent needs a single ranked feed across multiple search slugs without writing a fan-out loop._

  ```bash
  apartments-pp-cli nearby austin-tx round-rock-tx pflugerville-tx --beds 2 --price-max 2500 --rank sqft --agent
  ```

### Local-store math
- **`value`** — Rank synced listings by 12-month total cost (rent + pet rent + pet deposit + pet fee), filtered to your hard budget.

  _Pick this when budget is binding and pet fees might push a listing over the line._

  ```bash
  apartments-pp-cli value --budget 2800 --pet dog --months 12 --json --select rank,url,total_cost
  ```
- **`rank`** — Rank synced listings by ratio metrics — price per square foot or price per bedroom.

  _Pick this when value-per-dollar is the goal, not 'best match' or 'lowest price'._

  ```bash
  apartments-pp-cli rank --by sqft --beds 2 --price-max 2500 --json --limit 10
  ```
- **`floorplans`** — Rank per-floor-plan rent/sqft across synced listings — same building can yield 4 plans at different ratios.

  _Pick this when a building has multiple floor plans and you want the cheap one specifically._

  ```bash
  apartments-pp-cli floorplans --rank price-per-sqft --beds 2 --json --limit 10
  ```
- **`must-have`** — Filter synced listings to those whose amenities array contains ALL listed terms via FTS5.

  _Pick this when the must-haves are free-text, not in apartments.com's amenity dropdown._

  ```bash
  apartments-pp-cli must-have "in-unit washer" "covered parking" "dishwasher" --json
  ```

### Shortlist workflows
- **`compare`** — Pivot 2–8 listings into a wide table — one column per listing — with computed $/sqft and amenity overlap.

  _Pick this when narrowing a shortlist; the wide table makes amenity-overlap deltas obvious._

  ```bash
  apartments-pp-cli compare austin-arboretum-1 austin-arboretum-2 austin-arboretum-3 --json
  ```
- **`digest`** — Single-shot composer: new + removed + price-drops + top-5 by $/sqft + stale + phantom flags for one saved search over N days.

  _Pick this when an agent needs a Monday-morning summary in one call._

  ```bash
  apartments-pp-cli digest --saved-search austin-2br --since 7d --format md
  ```
- **`shortlist`** — Tag-based local shortlist table; add/show/remove listings with notes and tags.

  _Pick this when an agent or user is curating a shortlist; downstream commands like `compare` read from it._

  ```bash
  apartments-pp-cli shortlist add https://www.apartments.com/example-1234 --tag austin --note "liked the kitchen"
  ```

### Aggregations
- **`market`** — Median, p10, p90 of rent and rent/sqft, pet-friendly share, by city/state and bed count.

  _Pick this when an agent needs to anchor 'is this a fair price' against the local distribution._

  ```bash
  apartments-pp-cli market austin-tx --beds 2 --json
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 0 API entries from 0 total network entries
- Protocols: html-ssr (95% confidence)
- Generation hints: use Surf with Chrome TLS fingerprint at runtime (UsesBrowserHTTPTransport), all responses are HTML/SSR — extract via html_extract mode: page, no clearance cookie capture; no resident browser sidecar, schema.org microdata (meta itemprop=streetAddress|addressLocality|addressRegion|postalCode) plus data-beds / data-baths / data-maxrent attributes are the primary extraction targets
- Candidate command ideas: rentals — Path-slug search is the primary entry point at apartments.com; listing — Listing detail page extracts schema.org microdata
- Caveats: protection-active: Apartments.com (CoStar) employs Akamai-style bot detection. stdlib HTTP returns 403; Surf with Chrome TLS fingerprint clears it. Watch for protection escalation that might require Chrome-clearance cookie import or full-browser fallback in future versions.

## Command Reference

**listing** — Fetch a single Apartments.com listing detail page by URL or property ID, parsing rent, beds/baths, address, amenities, and pet policy.

- `apartments-pp-cli listing <property_id>` — Fetch one Apartments.com listing detail page and parse schema.org microdata.

**rentals** — Search Apartments.com rental listings by city, beds, baths, price, and pet policy. Returns parsed listing placards.

- `apartments-pp-cli rentals` — Run a path-slug search at apartments.com and return listing placards parsed from the HTML response.


**Hand-written commands**

- `apartments-pp-cli sync-search <saved-search>` — Run a saved search against apartments.com and snapshot placards into the local store.
- `apartments-pp-cli watch <saved-search>` — Diff the latest sync of a saved search against the previous; emit NEW / REMOVED / PRICE-CHANGED listings.
- `apartments-pp-cli nearby <slug...>` — Fan out a search across multiple city/zip/neighborhood slugs and return one ranked, deduped list.
- `apartments-pp-cli value` — Rank synced listings by 12-month total cost of occupancy (rent + pet rent + pet deposit + pet fee).
- `apartments-pp-cli rank` — Rank synced listings by ratio metrics — $/sqft or $/bed.
- `apartments-pp-cli compare <url-or-id...>` — Pivot 2–8 listings into a wide table — one column per listing — with computed $/sqft and amenity overlap.
- `apartments-pp-cli drops` — List listings whose max-rent dropped by ≥N% within a time window.
- `apartments-pp-cli stale` — Flag listings whose price and availability have not changed in N days — phantom or stuck signal.
- `apartments-pp-cli phantoms` — Surface listings flagged by a three-signal union: 404 on re-fetch, dropped from saved-search results, or stale ≥45...
- `apartments-pp-cli market <city-state>` — Aggregate synced listings: median, p10, p90 of rent and rent/sqft, plus pet-friendly share.
- `apartments-pp-cli history <url-or-id>` — Time-series of every observation of one listing — rent, availability, status.
- `apartments-pp-cli digest` — Weekly digest composer: new + removed + price-drops + top-by-sqft + stale + phantoms in one structured output.
- `apartments-pp-cli floorplans` — Rank per-floor-plan rent/sqft across synced listings.
- `apartments-pp-cli must-have <term...>` — Filter synced listings to those whose amenities array contains ALL listed terms via FTS5.
- `apartments-pp-cli shortlist` — Local shortlist table — add / show / remove listings with notes and tags.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `APARTMENTS_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
apartments-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Three-neighborhood relocation hunt

```bash
apartments-pp-cli nearby austin-tx round-rock-tx pflugerville-tx --beds 2 --price-max 2500 --pets dog --rank sqft --json --select url,addressLocality,maxrent,sqft,price_per_sqft
```

Single ranked feed across three target neighborhoods, deduped by listing URL, with `--select` keeping only the columns the agent needs.

### Weekly leasing-agent digest

```bash
apartments-pp-cli digest --saved-search client-rachel --since 7d --format md
```

Composes new / removed / price-drops / top-5-by-sqft / stale / phantom flags for a single client search into a markdown report ready to paste into email.

### Total-cost screen with hard budget

```bash
apartments-pp-cli value --budget 2800 --pet dog --months 12 --json --select rank,url,maxrent,pet_rent,total_cost,price_per_sqft
```

Ranks the synced listings by 12-month total cost (rent + pet rent + pet deposit + pet fee), filtered to under-budget rows only.

### Phantom-detection sweep before tour scheduling

```bash
apartments-pp-cli phantoms --json --select url,reason,unchanged_days
```

Three-signal union — 404, dropped from saved search, stale ≥45 days. Filter your shortlist before booking tours so you don't drive across town to a leased unit.

### Offline FTS amenity intersect

```bash
apartments-pp-cli must-have "in-unit washer" "covered parking" "dishwasher" --json --select url,addressLocality,maxrent,amenities
```

Path filter for amenities apartments.com's filter dropdown doesn't expose. FTS5 AND-join over the synced amenities array.

## Auth Setup

No authentication required.

Run `apartments-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  apartments-pp-cli listing example-property --agent --select id,name,status
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
apartments-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
apartments-pp-cli feedback --stdin < notes.txt
apartments-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.apartments-pp-cli/feedback.jsonl`. They are never POSTed unless `APARTMENTS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `APARTMENTS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
apartments-pp-cli profile save briefing --json
apartments-pp-cli --profile briefing listing example-property
apartments-pp-cli profile list --json
apartments-pp-cli profile show briefing
apartments-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `apartments-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.25+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-cli@latest
   ```
3. Verify: `apartments-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add apartments-pp-mcp -- apartments-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which apartments-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   apartments-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `apartments-pp-cli <command> --help`.
