---
name: pp-walkerplus
description: "Find Japan events on Walkerplus for a trip window, inspect a known Walkerplus event, or require source-backed free admission or indoor venues. Use when asked to use Walkerplus or run walkerplus-pp-cli."
license: Apache-2.0
regions: [JP]
api_language: ja
metadata:
  openclaw:
    requires:
      bins: [walkerplus-pp-cli]
---

# Walkerplus event discovery

For a source checkout, use a locally built `walkerplus-pp-cli` binary with Go 1.26.6 or newer. Catalog installation requires a merged library release. Verify `walkerplus-pp-cli --version`; if missing, build with `go build -o walkerplus-pp-cli ./cmd/walkerplus-pp-cli` and make the binary available on PATH. Public Walkerplus HTML is the only runtime source. No credentials, browser, booking, or translation provider is involved. MCP is compiled for stdio only; no HTTP listener is included. When using MCP, inspect truncation metadata and narrow `--limit` or `--select` after a bounded result or capture-limit error. CLI JSON error codes remain available; auxiliary diagnostics are separate.

## Workflow

1. Resolve the travel location and event category through the catalogs. Accepted codes and source slugs map to Japanese source labels. City codes, source slugs and Japanese city names from `areas` are accepted with the matching prefecture; for example, `--prefecture tokyo --city ar0313104` or `--city shinjuku`.

   ```bash
   walkerplus-pp-cli areas --prefecture kyoto
   walkerplus-pp-cli categories
   ```

2. Discover bounded candidates cheaply. Search reads listings only. Use exact trip dates and inspect `coverage` before describing completeness.

   ```bash
   walkerplus-pp-cli search --prefecture kyoto --category festival --from 2026-10-01 --to 2026-10-31 --limit 5 --max-pages 3
   ```

3. Enrich a bounded shortlist when the traveler needs attendance confidence or strict constraints. Keep possible matches separate from confirmed days.

   ```bash
   walkerplus-pp-cli shortlist --prefecture kyoto --from 2026-10-10 --to 2026-10-12 --max-details 10 --agent --select id,title_ja,start_date,end_date,location,match,schedule,sources
   walkerplus-pp-cli shortlist --prefecture osaka --from 2026-10-11 --to 2026-10-11 --indoor --max-details 10
   ```

   `--free` requires explicit event admission evidence; `--indoor` requires unconditional indoor evidence. Both flags mean AND. Optional paid purchases do not make explicitly free admission paid; preserve those caveats. Unknown and conditional source attributes are excluded by strict constraints.

4. Read the event edition before recommending attendance. Carry schedule exceptions, weather, cancellation, admission and reservation facts into the answer with its source URL.

   ```bash
   walkerplus-pp-cli event ar0313e603640 --select id,title_ja,source_url,schedule,hours,admission,reservation_required,reservation_text,weather,cancellation,organizer_urls,sources
   ```

   Completion means every recommendation cites the edition, relevant source dates, match confidence, practical caveats, and coverage limits. If published evidence does not resolve attendance, describe it as possible and direct the traveler to the organizer.

## Evidence and bounds

Japanese titles are authoritative. Raw schedule, admission and reservation text, evidence, and source URLs are source facts. Normalized `edition_year`/`date_certainty`, parsed recurrence/exclusion lists, admission status, boolean classifications, and match/rank fields are derived conveniences. An overall date envelope is not proof of daily activity. Closure-only rules narrow possible days; explicit occurrences, daily activity or positive recurrence can confirm activity. Approximate seasons, unresolved holiday exceptions, and undisclosed next-year editions never become confirmed dates. `--timing starts|ends` checks published envelope boundaries.

JSON is compact by default; `--agent` and `--json` are compatible. `--select`/`--fields` project event fields while retaining query, coverage and provenance. Use `schema` to discover fields. Unknown scalars are null, collections are empty arrays, diagnostics are stderr.

Default caps are 10 returned events, 3 listing pages, and 10 detail candidates. Shortlist returns only detail-inspected candidates; reaching `--max-details` can leave fewer results than `--limit`. Detail selection checks listing-supported location/category matches before candidates with missing facts, using `--sort` within each group; returned results use `--sort`. Widen `--max-pages` and `--max-details` deliberately within hard limits; an empty bounded scan does not establish absence. Coverage gives sampled routes, counts, continuation, cache hits and incomplete reasons. Resolving an additional city performs at most one prefecture catalog lookup outside the `--max-pages` event-page budget; `catalog_requests`/`catalog_routes` record it, and `request_count` includes it. Native month/day routes have no year selector; exact local filtering cannot manufacture a future edition.

Use `--refresh` when a source recheck is needed. `sources` provide fetch time and cache age; publisher update text is separate. Every custom command accepts `--dry-run` for offline validation.

```bash
walkerplus-pp-cli doctor --dry-run --json
walkerplus-pp-cli event --dry-run --agent
```

Usage errors exit 2, missing event 3, fetch/parser failure 5, exhausted rate limiting 7. Source failure emits JSON error rather than empty success. Read `README.md` for build, cache, troubleshooting and live verification details.

## Catalog availability

Use the installer section below when Walkerplus is available in the Printing Press catalog. For a source checkout or when catalog installation is unavailable, build and verify locally using the instructions above.

## Prerequisites: Install the CLI

This skill drives the `walkerplus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install walkerplus --cli-only
   ```
2. Verify: `walkerplus-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/walkerplus/cmd/walkerplus-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Build a bounded shortlist from Walkerplus while preserving Japanese event titles and uncertainty. Inspect details before committing travel time.

## Unique Capabilities

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

## Auth Setup

Public Walkerplus HTML; no API key or login required.

Run `walkerplus-pp-cli doctor` to verify setup.
