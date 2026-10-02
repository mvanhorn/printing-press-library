---
name: pp-japan-guide
description: Find Japan Guide destinations and attractions, inspect scoped visit facts, compare a small shortlist, or read source itinerary labels. Use for Japan Guide sightseeing facts or `use japan-guide`.
author: zjsng
license: Apache-2.0
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins: [japan-guide-pp-cli]
---

# Japan Guide

Public source reads require no account or API key.

## Prerequisites: Install the CLI

This skill drives the `japan-guide-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install japan-guide --cli-only
   ```
2. Verify: `japan-guide-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-guide/cmd/japan-guide-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Discover destinations and attractions, inspect scoped visit facts, and follow source itinerary links. Preserve seasonal qualifiers and source freshness.

## When to Use This CLI

Use Japan Guide as a single source for destination and attraction discovery, scoped visit information, short comparisons, and published itinerary labels. Begin with a source directory and reuse returned page IDs. Source IDs and canonical URLs are the lookup keys.

## Anti-triggers

Use another tool for bookings, payments, ticket inventory, route calculation, cross-provider rankings or current opening confirmation. Source recommendations are editorial dots, separate from visitor reviews. Schedules and source plans are reference facts.

## Unique Capabilities

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

## Command Reference

- `guide destinations`: one directory; filter by region, name or editorial dots.
- `guide interests`: source interest labels and canonical topic links.
- `guide attractions e2164`: one destination; filter exact interest tags or name.
- `guide inspect e3002`: separate facility hours, closed days, admission, access and dated notices.
- `guide compare e3001 e3002`: at most five pages; preserve per-item failures.
- `guide itineraries`: regional source index; `--destination e2164` reads local plan links.
- `guide itinerary e2400_kanto`: bounded source day/stop labels, local visit durations and seasonal closure qualifiers.

Use `japan-guide-pp-cli which "temple visit facts" --json` and command help for current runtime discovery.

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

## Auth Setup

Supported public guide pages require no account or API key.

Run `japan-guide-pp-cli doctor` to verify setup.

## Agent Mode

Use `--agent` for one compact `{meta,results}` JSON document. Lists default to 10 rows, allow at most 50, and expose `scanned_records` and `next_offset`; continue with `--offset`. Each invocation scans one source page. An empty filtered result carries a coverage note. `--select` filters the payload before the envelope; `--select items` makes `results` a row array.

Cite returned canonical source URLs. Preserve facility identity, source interest tags and destination/event kinds. Hours use JST; yen admissions use JPY. Keep the exact source year on dated facts. `open_now: null` is explicit uncertainty. Unknown fields remain unknown; `facts_truncated` directs the agent to the source for omitted text.

`event_calendar` separates annual recurrence (`explicit_year: null`) from a dated source notice. Preserve named seating fees and their release-date qualifiers without treating them as current ticket inventory. `extraction_limitations` discloses unsupported narrative admission layouts. The MCP comparison field accepts space-separated page IDs, bounded to five; raw endpoint mirrors are hidden.

## Offline and failures

`guide inspect --cache` saves only extracted facts. `--offline` reads them with `freshness: offline_snapshot` and `meta.source: local`; `retrieved_at` and `source_updated` retain their original meanings. `--cache-dir` overrides the facts cache. Other guide commands require live source reads.

Inspect's default `--data-source auto` tries live facts, then an available saved snapshot after an eligible failure. `live_failure` and a stderr warning explain the local fallback; it retains original timestamps and does not save a new snapshot without `--cache`. Use `--data-source live` or `--no-cache` to disable fallback. A 429 is an error, not a cache fallback. `--rate-limit 0` disables pacing; throttle errors exit 7.

A failed explicit snapshot save propagates an error rather than returning older facts after a successful live read. Inspection's MCP tool conservatively permits local writes because it exposes `cache`; the other six guide tools are read-only. Guide snapshots are separate JSON files, not SQL resources. This CLI has no sync command; use guide commands for source facts and SQL only for the local framework store.

Inspection's machine-format dry-run returns one simulation object (`dry_run`, `action`, `would`) rather than fetched facts, without a source request or snapshot write.

Comparisons return successful items plus non-throttle `fetch_failures`, with a stderr warning. All-source failure is an error. Rate limits return a typed 429 error and retry guidance rather than empty data. Layout errors and network errors name the source; follow its canonical URL and retry or select another supported source page. `--timeout` bounds the whole command.

MCP snapshot caching uses the server-owned cache root. The MCP tool rejects caller-supplied `cache-dir`; direct CLI invocations retain `--cache-dir`.

## Automatic learning

Recall once before discovery; a cold store can be skipped for the session. Treat learning candidates as trials requiring verification. Teach structural queries with identifiers removed. For deterministic calls, add `--no-learn` or set `JAPAN_GUIDE_NO_LEARN=true`.

## Agent Feedback

Use `japan-guide-pp-cli feedback --help` for the local feedback format. Record reproducible source-layout or command-contract failures without credentials or personal data.
