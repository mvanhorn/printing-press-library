---
name: pp-tablecheck
description: "Plan Japan dining through TableCheck: discover venues, inspect course prices and conditions, check party/date availability or a bounded shortlist, and return booking URLs."
allowed-tools: "Read Bash"
---

# TableCheck planning

## Local setup

Use this checkout's binary. From its directory, build and verify it, then expose it only to the current shell:

```bash
go build -o tablecheck-pp-cli ./cmd/tablecheck-pp-cli
export PATH="$PWD:$PATH"
tablecheck-pp-cli version
```

Completion: the local binary runs. No authentication is needed for the verified consumer read endpoints.

## Workflow

1. Resolve the trip location to explicit coordinates, cuisine key, budget meaning, dates and party. Use `cuisines list --query sushi` when a cuisine key is uncertain.
2. Discover a bounded shortlist with `venues search`. Retain stable IDs, slugs and Japanese names. Completion: each candidate matches the requested filters; search availability remains a discovery hint.
3. Fetch `venues get` and `courses list` only for shortlisted venues. Use `courses get` for a selected course's exact fine print and conditions. Completion: currency, quoted amount, price basis, charges and relevant conditions are either sourced or explicitly unknown.
4. Use `availability check` for one venue or `availability scan` for at most five venues and fourteen days. Completion: every requested venue/date has an observation or explicit failure, with party, local time zone and freshness retained.
5. Recheck the chosen option with `--refresh`, then return `booking-url` and the material conditions. Completion: the user has the canonical link and understands that only the booking page can confirm a named course and complete the reservation.

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

## Availability examples

Replace these example dates with the trip dates. Times are local to the venue.

```bash
tablecheck-pp-cli availability check sushi-tokyo81 --date 2026-09-30 --party 2 --time 18:00 --refresh
tablecheck-pp-cli availability scan sushi-tokyo81 sushi-shiono --from 2026-09-30 --to 2026-10-02 --party 2
tablecheck-pp-cli booking-url sushi-tokyo81 --date 2026-09-30 --time 17:30 --party 2
```

JSON is compact by default; diagnostics use stderr. Use dotted `--select` fields to reduce live output, retaining freshness when presenting availability. A dry run returns a request plan with different fields. If MCP reports its 60,000-byte result limit, narrow the scan or field selection and retry; use the CLI directly when the complete large result is needed. The error contains no partial availability result.

The source calendar covers a time window around an anchor, not the full day. Without `--time`, the CLI anchors at 18:00; with `--time`, it also checks that exact time. Inspect `query_anchor_time` and `coverage` for the observed time range; use another time for lunch or other periods.

## Interpretation

- Venue calendar slots and course catalog eligibility are separate. Neither a search result nor course status proves a named-course booking.
- Explicit slot false means unavailable. Closed, unknown, failed, sold-out, unpublished, request and waitlist states require their own source evidence. Missing data and static policy text never supply that evidence.
- Preserve decimal price strings, currency, Japanese names and exact conditions. Unknown amounts, price basis or charges stay null; tax included is not tax exempt. Fine print can qualify the headline price. Empty cancellation rules do not prove free cancellation; confirm final terms on the booking page.
- Cached observations keep their original fetch time. `--refresh` refreshes the local read, not the upstream cache. Treat every result as an observation rather than a guarantee.
- On partial failure, report the failed venue/date alongside successful observations. Refrain from replacing failed results with empty availability.

For access limitations, cache bounds and verification evidence, read [README.md](README.md). Booking mutations, background monitoring and Tabelog belong outside this skill.

## Prerequisites: Install the CLI

This skill drives the `tablecheck-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tablecheck --cli-only
   ```
2. Verify: `tablecheck-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/cmd/tablecheck-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Preserves source prices, conditions and party-specific availability observations in compact JSON. Complete reservations on the canonical TableCheck booking page.

## Unique Capabilities

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

## Auth Setup

Tested public consumer reads need no credentials. This CLI uses undocumented website endpoints; the separately documented partner API requires approval, with pricing supplied during application.

Run `tablecheck-pp-cli doctor` to verify setup.
