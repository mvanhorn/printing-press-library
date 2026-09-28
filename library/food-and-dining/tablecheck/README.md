# TableCheck CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Find Japan restaurants, inspect courses, and check a bounded shortlist before booking.**

Preserves source prices, conditions and party-specific availability observations in compact JSON. Complete reservations on the canonical TableCheck booking page.

## Authentication

Tested public consumer reads need no credentials. This CLI uses undocumented website endpoints; the separately documented partner API requires approval, with pricing supplied during application.

## Local setup

Requires Go 1.26.6 or newer. From this checkout:

```bash
go build -o tablecheck-pp-cli ./cmd/tablecheck-pp-cli
export PATH="$PWD:$PATH"
```

Read [SKILL.md](SKILL.md) for the agent workflow. The CLI creates no reservations; complete bookings on TableCheck.

## Quick Start

```bash
# Check the local command surface without network.
tablecheck-pp-cli doctor --dry-run

# Discover a bounded Tokyo shortlist.
tablecheck-pp-cli venues search --lat 35.681236 --lon 139.767125 --radius 3000 --cuisine sushi --limit 3

# Inspect course summaries lazily.
tablecheck-pp-cli courses list sushi-tokyo81 --limit 3

```

## Commands

| Command | Use |
| --- | --- |
| `venues search` | Geographic discovery with cuisine, dinner-average budget, date and party filters; cursor pagination |
| `venues get SLUG` | Venue identity, Japanese name, location, policy and booking URL |
| `cuisines list` | Find stable cuisine keys and translated labels |
| `courses list SLUG` | Bounded course summaries |
| `courses get SLUG COURSE_ID` | Exact prices, fine print and structured conditions |
| `availability check SLUG` | Party-specific venue slots on a date |
| `availability scan SLUG...` | Compare a bounded shortlist across dates |
| `booking-url SLUG` | Canonical booking page with optional date, time and party prefill |

Use a leaf command's `--help` for its current arguments. Search requires explicit coordinates so the caller's IP location cannot move a Japan search elsewhere. Search budget filters refer to venue dinner averages, not course prices. Search time is a meal-time preference; use `availability check --time` for an exact-time observation.

## Agent Usage

Planning commands emit compact JSON by default. Diagnostics go to stderr. IDs, Japanese names, source URLs and explicit missing values are retained. Use `--select` for dotted field selection and `--limit` for bounded output:

```bash
./tablecheck-pp-cli venues search --lat 35.681236 --lon 139.767125 --radius 3000 --cuisine sushi --limit 3 --agent --select items.id,items.name_ja,items.booking_url
./tablecheck-pp-cli courses get sushi-tokyo81 68da546fcde865308c33e7f9
```

CSV/plain output contains one row per primary record; availability uses venue/date checks, including failed checks. Select row fields with paths such as `checks.slug,checks.date,checks.status`. For planning lists and details, selected envelope fields such as `meta.requests` appear as named context columns on each row; an envelope-only selection produces one summary row. Structured values and nulls in record or context cells use JSON text, so conditions and categories remain parseable.

For availability, `--quiet` emits one slug per check, so multi-day scans repeat slugs; an explicit selection must retain `checks.slug` or all of `checks`. Venue/course detail `--quiet` output is the venue/course ID; an explicit selection must retain `venue.id`/`course.id` or its full parent. Use the default JSON for complete status, slots and freshness.

Availability and course eligibility are separate evidence. A listed restaurant or course does not prove it can be booked for the requested party and time. A venue slot does not identify which course can be booked. Static request/waitlist policy text is not a live request or waitlist result.

Explicit available, unavailable, closed, unknown and failed observations retain their source evidence. Sold-out, unpublished, request and waitlist interpretations require explicit source evidence; missing inventory is not sold out. Unrecognized source statuses remain unknown. Scans retain failed and empty days rather than silently dropping them.

Money amounts remain decimal strings with source currency. Price basis and charges remain unknown when absent. Detail commands preserve tax/service flags, fine print, date/time/quantity restrictions, payment requirements and cancellation rules. For example, the observed ¥19,800 headline course also had a ¥19,800–22,000 fine-print range and a separate 10% service charge; the CLI does not invent an all-in total. Empty cancellation rules do not establish free cancellation; the booking page may expose additional venue-specific conditions.

The source calendar covers a time window around an anchor, not the full day. Without `--time`, the CLI anchors at 18:00; with `--time`, it also checks that exact time. Inspect `query_anchor_time` and `coverage` for the observed time range; use another time for lunch or other periods.

## Freshness and limits

Every availability result is a timestamped observation, not a reservation guarantee. Cached results retain the original fetch time and show cache age. `--refresh` bypasses local cache reads; it cannot force TableCheck's own cache to update.

Default result limit is 10, maximum 50. Scans allow at most five venues and fourteen inclusive dates. Each command allows at most twenty HTTP attempts, one retry per request, and concurrency two. Requests and responses are bounded; expensive course and venue details are fetched only when needed. The returned calendar can cover more dates than requested; output stays inside the requested range. Scans fetch additional windows for uncovered dates within the shared request budget. Later responses can recover earlier failed or unknown dates when they explicitly cover them. Dates still omitted after their own check stay unknown; unrecovered request failures and exhausted budgets remain explicit failures.

Availability cache lifetime is 30 seconds; discovery and static details can live longer. Use `--cache-dir` to isolate a run. Result metadata reports requests, response bytes and latency; cached reads require no HTTP when all needed entries remain fresh.

MCP planning command results are limited to 60,000 bytes. Larger results return an explicit error without a partial result. Narrow the venue/date range, lower `--limit`, select required fields with `--select`, or run the CLI directly for complete output.

## Cookbook

Replace the example dates with your trip dates. Times are local to the venue.

```bash
./tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2 --time 18:00 --refresh
./tablecheck-pp-cli availability scan sushi-tokyo81 sushi-shiono --from 2026-09-30 --to 2026-10-02 --party 2
./tablecheck-pp-cli booking-url sushi-tokyo81 --date 2026-09-30 --time 17:30 --party 2
```

Open the returned URL to confirm the course, conditions and final price and complete the reservation yourself. These commands create no cart, hold, reservation, payment or waitlist entry.

## Health Check

```bash
./tablecheck-pp-cli doctor --json
./tablecheck-pp-cli venues search --dry-run --json
```

A dry run describes intended work without network or cache writes. Doctor is a connectivity/installation check, not proof of venue availability.

## Access and coverage

The tested consumer website endpoints work without login, cookies or API keys. They are undocumented interfaces and may change. There is no fixed published entitlement or SLA for this access path. The separate [official API](https://tablecheck.atlassian.net/wiki/spaces/API/pages/44729064/Request+API+Access) requires approval; fees are provided during application. This CLI does not use that partner API.

Coverage is TableCheck's Japan consumer listings and whichever public course/calendar data each venue exposes. It is not all restaurants in Japan. The current read surface cannot prove named-course slot availability or reliably distinguish every possible booking mode; ambiguous cases are reported conservatively. No Tabelog integration or background polling is included.

## Troubleshooting

- Empty search: verify coordinates, cuisine key, radius and budget. Use the returned cursor for the next page.
- No slots: inspect status, date coverage and party size. A failed check or missing date is not sold out.
- Stale observation: rerun with `--refresh` and check the timestamp; inventory can change immediately afterward.
- HTTP 429/network/schema errors: keep the failure result and retry later. The CLI will not convert errors into empty inventory.
- Unknown price or charge: inspect `courses get` and the booking page; do not replace null with zero.

## Development and evidence

```bash
go test -count=1 ./...
go vet ./...
go build ./...
```

[VERIFICATION.md](VERIFICATION.md) records live comparisons and cached/uncached output size, request count, latency and peak memory. Live tests are opt-in because they read current inventory. Source contracts and provenance are archived with the Printing Press run.

Generated with [Printing Press](https://github.com/mvanhorn/cli-printing-press). [tc-mcp](https://github.com/nbw/tc-mcp) informed the public search/calendar parameter research; current contracts were independently tested.

## Unique Features

Verified planning behaviors:

### Japan trip planning
- **`availability scan`** — Check a bounded restaurant shortlist across trip dates with partial results.

  ```bash
  tablecheck-pp-cli availability scan sushi-tokyo81 sushi-shiono --from 2026-09-30 --to 2026-10-01 --party 2 --limit 3 --cache-dir .tablecheck-cache --max-requests 8 --json
  ```
- **`availability check`** — Report party-specific venue slots without upgrading search summaries or course listings into booking guarantees.

  ```bash
  tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2 --time 18:00 --limit 3 --cache-dir .tablecheck-cache --max-requests 8 --json
  ```
- **`courses get`** — Preserve exact prices, Japanese names, fine print and unknown charges.

  ```bash
  tablecheck-pp-cli courses get sushi-tokyo81 68da546fcde865308c33e7f9 --cache-dir .tablecheck-cache --max-requests 8 --json
  ```
- **`availability check`** — Keep cache age attached to every availability observation and refresh explicitly.

  ```bash
  tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2 --limit 3 --cache-dir .tablecheck-cache --max-requests 8 --json
  ```
- **`booking-url`** — Return the correct TableCheck booking page with validated date, time and party.

  ```bash
  tablecheck-pp-cli booking-url sushi-tokyo81 --date 2026-09-30 --time 17:30 --party 2 --cache-dir .tablecheck-cache --max-requests 8 --json
  ```

## Recipes

### Compact shortlist

```bash
tablecheck-pp-cli venues search --lat 35.681236 --lon 139.767125 --radius 3000 --cuisine sushi --limit 3 --agent
```

Keep a bounded set of stable identities and handoff URLs.

### Course conditions

```bash
tablecheck-pp-cli courses get sushi-tokyo81 68da546fcde865308c33e7f9
```

Read exact price and fine print before comparing costs.

### Venue identity

```bash
tablecheck-pp-cli venues get sushi-tokyo81
```

Resolve source details and booking mode.
