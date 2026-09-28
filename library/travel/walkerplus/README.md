# Walkerplus CLI

**Discover Japan events for a trip with source-backed dates and schedule confidence.**

Build a bounded shortlist from Walkerplus while preserving Japanese event titles and uncertainty. Inspect details before committing travel time.

## Build locally

For a source checkout, build locally with Go 1.26.6 or newer. Catalog installation requires a merged library release:

```bash
go build -o walkerplus-pp-cli ./cmd/walkerplus-pp-cli
./walkerplus-pp-cli --help
```

## Authentication

Public Walkerplus HTML; no API key or login required.

## Quick Start

```bash
# Check the local command setup.
walkerplus-pp-cli doctor --dry-run

# Discover supported event filters.
walkerplus-pp-cli categories

# Find bounded trip candidates.
walkerplus-pp-cli search --prefecture kyoto --from 2026-10-10 --to 2026-10-12 --limit 5

```

## Agent Usage

Commands emit one compact JSON document by default. `--json` and `--agent` are compatible with that default. Search and shortlist return `query`, `events`, `coverage`, and `meta`; event returns `event`, `coverage`, and `meta`. Missing source facts are `null`; collections are `[]`. Diagnostic warnings go to stderr. Raw schedule, admission and reservation text, evidence, and source URLs are source facts. Normalized `edition_year`/`date_certainty`, parsed recurrence/exclusion lists, admission status, boolean classifications, and match/rank fields are derived conveniences.

```bash
./walkerplus-pp-cli shortlist --prefecture tokyo --from 2026-09-27 --to 2026-09-30 --select id,title_ja,start_date,end_date,location,match,sources
./walkerplus-pp-cli event ar0313e603640 --fields id,title_ja,admission,reservation_required,schedule,sources
./walkerplus-pp-cli schema
./walkerplus-pp-cli search --dry-run --agent
```

`--select` and its alias `--fields` project event fields only; query, coverage, and provenance remain present. Nested paths such as `location.venue` work. `schema` lists accepted paths. Every custom command supports `--dry-run` before required-input validation or network/cache access.

| Command | Source work |
| --- | --- |
| `search` | Listing pages only; cheap candidates, no detail enrichment |
| `shortlist` | Listing pages plus a bounded candidate set of event/data/price pages |
| `event ID\|URL` | One source edition's event/data/price pages |
| `areas` | Source-derived prefecture catalog; `--prefecture` fetches current city routes |
| `categories` | Source-derived category catalog with Japanese labels and aliases |

Filters use `--prefecture`, `--city`, `--category`, `--from`, and `--to`. Discover accepted codes, source slugs and Japanese city names with the catalogs. Supply the matching `--prefecture` with city codes, source slugs or Japanese city names. City resolution performs at most one extra prefecture catalog lookup, recorded in `coverage.catalog_requests` and `catalog_routes`. Its HTTP attempts are included in `request_count`, while the lookup is separate from the event listing pages bounded by `--max-pages`. Dates are exact ISO calendar dates in Asia/Tokyo. `--timing overlap|starts|ends` selects envelope overlap or a published opening/closing boundary. `--sort relevance|start|end|source` gives stable ordering.

`shortlist --free` requires explicit free event admission; `--indoor` requires unconditional indoor evidence. Both flags apply together. Unknown, mixed, child-only free, free parking, and indoor-only-on-rain evidence do not pass these strict filters. Free admission can coexist with optional paid booth purchases or activities; their source caveats remain present. Admission text is preserved even when a numeric price is unavailable.

## Bounds and Freshness

Defaults are 10 results, 3 listing pages, and 10 detail candidates. Hard limits are 100 results, 20 listing pages, 30 detail candidates, 4 concurrent requests, and 3 retries. `--page` chooses the first listing page. `--limit` bounds returned events; `--max-pages` bounds discovery work independently; `--max-details` bounds shortlist enrichment. Shortlist returns only candidates whose detail was inspected, so it can return fewer than `--limit` when the detail cap is reached. Detail selection prioritizes listing-supported location/category matches, then candidates with missing facts, preserving `--sort` within each group. Remaining budget can resolve missing facts; known conflicts are skipped and final filters remain strict. Returned events use `--sort`. A trip window can span at most 366 days.

Coverage records sampled routes, native year labels, scanned pages, candidates, detail count, request count, cache hits, elapsed time, truncation reasons, and continuation when available. Empty results include a note about bounded source coverage. They do not imply there are no events in Japan.

Source HTML is cached for one hour by default. Use `--cache-dir PATH`, `--cache-ttl 1h`, `--refresh`, or `--no-cache`. Event `sources` record fetch timestamps and cache age separately from publisher update text. Requests default to 15 seconds each and 60 seconds overall; use `--request-timeout` at most 15s and `--timeout` at most 60s. Transient errors have bounded retries.

## Health Check

```bash
./walkerplus-pp-cli doctor --json
./walkerplus-pp-cli doctor --dry-run --json
```

The live check tests public source connectivity. It does not guarantee parser compatibility or exhaustive event coverage.

## Troubleshooting

- Invalid flags, dates, locations, categories, fields, or bounds exit 2 with an actionable stderr message.
- A missing event exits 3; a source fetch or parse failure exits 5; exhausted rate limiting exits 7. Runtime source failures also emit a JSON `error` document. Partial results explicitly mark `coverage.incomplete` and warn on stderr.
- If a result is missing, inspect coverage, widen `--max-pages` within the cap, or check the source link. `--refresh` rechecks stale source facts.
- The site is an undocumented HTML contract. Parser drift and network failures are errors, not fabricated empty results. Shared credential/configuration files are not needed for the five event workflows.

## Cookbook

Catch events opening during a trip:

```bash
./walkerplus-pp-cli shortlist --prefecture osaka --from 2026-10-10 --to 2026-10-12 --timing starts --sort start
```

Find source-backed indoor alternatives:

```bash
./walkerplus-pp-cli shortlist --prefecture osaka --from 2026-10-11 --to 2026-10-11 --indoor --max-details 10
```

Read the weather and reservation caveats before traveling:

```bash
./walkerplus-pp-cli event ar0101e66092 --select id,title_ja,weather,cancellation,schedule,sources
./walkerplus-pp-cli event ar0313e603640 --select id,title_ja,admission,reservation_required,reservation_text,sources
```

A source start/end range is an envelope, not proof that every day is active. Search marks activity as possible. Shortlist uses explicit occurrences, recurrence, and exclusions. Closure-only rules exclude those days without proving all other days active; unresolved holiday/monthly rules and approximate seasonal dates stay possible. Unpublished future editions are not inferred from the current edition. Native routes select a month or day without a year selector, so cross-year discovery can be incomplete. Recheck the source and organizer before attendance.

## Development

Use the local CLI or MCP over stdio. The source spec compiles only stdio MCP; the HTTP listener and its authentication/TLS flags are not included.

MCP compacts large event lists as JSON and preserves CLI error codes. Check truncation metadata and narrow `--limit` or `--select` when needed. Returned text is bounded to 60 KB; command output beyond the capture limit produces an explicit error.

```bash
go test ./...
go vet ./...
WALKERPLUS_LIVE=1 go test ./tests -run TestLive -v
python3 scripts/measure.py --binary ./walkerplus-pp-cli --output ./measurements
```

Live tests assert actual region, category, ISO date relevance and unique IDs across Tokyo, Kyoto, Hokkaido and Miyagi, plus known detail evidence and a wrong-year exclusion. The measurement harness records cold/warm stdout bytes, HTTP requests, cache hits, wall latency and peak RSS for search, shortlist and event.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Trip discovery
- **`shortlist`** — Find exact-edition trip candidates with schedule confidence, constraints and explainable ranking.

  _Use for a bounded event shortlist tailored to trip dates and practical constraints._

  ```bash
  walkerplus-pp-cli shortlist --prefecture kyoto --from 2026-10-10 --to 2026-10-12 --max-pages 1 --max-details 3 --limit 3
  ```

## Recipes

### Trip shortlist

```bash
walkerplus-pp-cli shortlist --prefecture kyoto --from 2026-10-10 --to 2026-10-12 --limit 5
```

Enrich bounded candidates and explain schedule confidence.

### Compact candidate scan

```bash
walkerplus-pp-cli search --prefecture tokyo --from 2026-10-01 --to 2026-10-07 --agent --select id,title_ja,source_url
```

Keep event output small while retaining coverage metadata.

### Event facts

```bash
walkerplus-pp-cli event ar0313e603640
```

Read source schedule, access, pricing and reservation facts.
