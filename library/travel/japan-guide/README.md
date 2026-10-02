# Japan Guide CLI

**Source-linked Japan sightseeing facts in compact JSON.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).

Discover destinations and attractions, inspect scoped visit facts, and follow source itinerary links. Preserve seasonal qualifiers and source freshness.

## Install

Install from the catalog:

```bash
npx -y @mvanhorn/printing-press-library install japan-guide
```

To build from a source checkout, use the Go version declared in `go.mod`:

```bash
go build -o japan-guide-pp-cli ./cmd/japan-guide-pp-cli
go build -o japan-guide-pp-mcp ./cmd/japan-guide-pp-mcp
./japan-guide-pp-cli doctor --json
```

Place the binaries on your PATH for the examples below. An MCP host can run `japan-guide-pp-mcp` over stdio; its public tools mirror the live Cobra command tree.

Inspection's MCP hint permits a local write because `--cache` can replace an extracted-facts snapshot. MCP callers use the server's cache root; `cache-dir` overrides are rejected before CLI execution. Direct CLI operators can use `--cache-dir`. The other six guide tools read source pages only. Guide facts are not SQL rows; this CLI has no sync command. Use guide inspection/comparison for page facts and the generated SQL tool only for the local framework store.

Inspection's `--dry-run --json` or `--dry-run --agent` returns one simulation object with `dry_run`, `action` and `would`; it makes no source request or snapshot write.

## Authentication

Supported public guide pages require no account or API key.

## Quick Start

```bash
# Check local setup
japan-guide-pp-cli doctor --dry-run

# Find source destinations
japan-guide-pp-cli guide destinations --region kanto --limit 5 --agent

# Inspect Sensoji visit facts
japan-guide-pp-cli guide inspect e3001 --agent

```

## Commands

| Command | Source coverage |
|---|---|
| `guide destinations` | Directory, region/name filters and editorial dots |
| `guide interests` | Interest directory and stable source links |
| `guide attractions e2164` | One destination page; exact interest tags and name filters |
| `guide inspect e3002` | Separate facility hours, closed days, admission, access and notices |
| `guide compare e3001 e3002` | Up to five pages; successful items and per-item failures |
| `guide itineraries` | Regional index, or one destination with `--destination` |
| `guide itinerary e2400_kanto` | Source day/stop labels, local visit durations and seasonal closure qualifiers |

A source page ID, such as `e3001`, or its canonical HTTPS `www.japan-guide.com/e/` URL identifies a detail page. Arbitrary domains, nested paths and account pages are rejected. Destination directories may include side trips (`kind: destination`) and events (`kind: event`) alongside attractions.

## Recipes

### Tokyo temples

```bash
japan-guide-pp-cli guide attractions e2164 --interest temples --limit 5 --agent --select items
```

Filter source attraction tags.

### Compare facts

```bash
japan-guide-pp-cli guide compare e3001 e3002 --agent
```

Inspect a short list with per-item errors.

### Source itinerary

```bash
japan-guide-pp-cli guide itineraries --destination e2164 --agent
```

Return source plans and canonical links.

## Output and freshness

`--agent` returns one compact JSON document with `meta` and `results`. Ordinary `--json` returns the payload directly. Lists default to 10 rows, allow at most 50, and expose `total_matches`, `scanned_records`, `next_offset` and `max_source_pages: 1`. Continue with `--offset`; the same source page is fetched again. `--select items` narrows to a row array under `results`; other selections apply to payload fields before the agent envelope. Unknown field selections fail explicitly.

Each row carries its canonical URL and source URL. Editorial recommendations have 0–3 dots and an explicit label; they are separate from visitor ratings. Dates retain the source year. Schedules use Japan local time (JST); stated yen fees use JPY. Missing facts are `null` or an empty fact list. `open_now` is always `null`: source schedules do not confirm current operations.

Detail output preserves separate shrine, museum and garden scopes. `source_updated` is the source's page update date; `retrieved_at` is the fetch time. `facts_truncated` marks bounded extraction. Offline reads use `freshness: offline_snapshot`, `meta.source: local`, and the original source and retrieval dates. Cache files contain extracted facts rather than full articles. `--cache-dir` overrides the facts directory; otherwise it follows the framework cache root (`JAPAN_GUIDE_HOME` can relocate the CLI).

Event identity comes from the source's current-page category. `event_calendar` distinguishes `annual_recurrence` with no invented event year from `dated_notice` with an explicit source year. Narrative seating fees retain their named scope and dated release notes; they do not establish current inventory. Unrecognized Hours and Fees layouts are disclosed in `extraction_limitations`.

`guide inspect` defaults to `--data-source auto`: an eligible live failure falls back to the selected/default saved snapshot, retaining local provenance and timestamps and reporting `live_failure`. It saves fresh facts only with `--cache`. `--data-source live` disables fallback; `--data-source local` selects offline facts. A 429 remains a typed error. `--no-cache` disables automatic fallback, and `--rate-limit 0` disables pacing.

A failed explicit snapshot save is an error even after a successful source read; it does not replace fresh acquisition with an older snapshot.

`guide compare` preserves non-throttle partial failures in `fetch_failures` and emits a stderr warning. If every source fails, the command fails. A 429 exits through the framework's rate-limit error with retry guidance; it is never an empty successful result.

The MCP `guide_compare` field accepts a space-separated string of at most five source IDs. The generated raw source endpoint is hidden from the public MCP catalog.

## Limits and verification

Runtime uses standard HTTPS requests with a per-source adaptive limiter, one page per list/detail request, a 4 MiB download cap, up to five comparison requests and a command-wide `--timeout`. List metrics report upstream request count, downloaded bytes and elapsed milliseconds. The CLI extracts concise planning facts; it does not reproduce guide articles, estimate routes, confirm ticket inventory, make bookings, or compute live opening status.

Source HTML layout changes cause explicit extraction errors where required records disappear. Construction and event notices retain dates even when they are historical. Access text and long fact fields are bounded; follow the canonical citation for full details. Public pages were checked first in the native browser and then through the HTTP runtime. Verification and resource measurements are in `.manuscripts/20261002-001811-227018a4/proofs/`; deterministic parser tests complement live source checks.

For source errors, open the returned canonical page, check network access and retry later. A source 429 includes `Retry-After` guidance. Usage errors exit 2; framework rate-limit errors exit 7. Use `doctor --json` for local setup and `agent-context --pretty` for the current command contract. Public source reads require no authentication setup.

## Unique Features

These capabilities extend the generated source-link foundation.

### Planning facts
- **`guide compare`** — Compare scoped visit facts for up to five source pages and retain per-item failures.

  _Compare a small shortlist using source facts._

  ```bash
  japan-guide-pp-cli guide compare e3001 e3002 --agent
  ```
- **`guide inspect`** — Keep shrine, museum and garden schedules attached to their own facilities.

  _Use facility-specific admission and closure facts._

  ```bash
  japan-guide-pp-cli guide inspect e3002 --agent
  ```
- **`guide inspect`** — Return source schedules with open_now explicitly unknown.

  _Avoid interpreting editorial schedules as live operating status._

  ```bash
  japan-guide-pp-cli guide inspect e3001 --agent
  ```
- **`guide inspect`** — Read saved compact facts while retaining their original source and retrieval dates.

  _Save a source snapshot before an offline planning session._

  ```bash
  japan-guide-pp-cli guide inspect e3001 --cache --agent
  ```
- **`guide destinations`** — Return a bounded destination page with request, byte and elapsed-time metrics.

  _Limit output and inspect single-page source coverage._

  ```bash
  japan-guide-pp-cli guide destinations --region kanto --limit 5 --agent
  ```
