---
name: pp-navitime
description: Check Japan itinerary travel windows, resolve NAVITIME station/place ambiguity, compare alternatives, or inspect overnight dates, fares and rail-pass constraints.
license: Apache-2.0
argument-hint: "<command> [args]"
allowed-tools: "Read Bash"
---

# NAVITIME

## Prerequisites: Install the CLI

This skill drives the `navitime-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install navitime --cli-only
   ```
2. Verify: `navitime-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/navitime/cmd/navitime-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use NAVITIME source evidence to decide whether a Japan journey fits an itinerary. Return candidate locations when names are ambiguous; booking and broader itinerary optimization belong to another workflow.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Itinerary evidence
- **`places search`** — Return bilingual candidates without silently choosing a station.

  _Return bilingual candidates without silently choosing a station._

  ```bash
  navitime-pp-cli places search 大久保 --type station --agent
  ```
- **`routes search`** — Check dated departures, deadlines and first/last services.

  _Check dated departures, deadlines and first/last services._

  ```bash
  navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --agent
  ```
- **`routes show`** — Expand a stored route with full dates and fare assumptions.

  _Inspect overnight boundaries and optional fare supplements before accepting an itinerary._

  ```bash
  navitime-pp-cli routes show --latest --agent
  ```
- **`routes compare`** — Compare the returned journeys by time, fare, walking and transfers.

  _Compare the returned journeys by time, fare, walking and transfers._

  ```bash
  navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --agent
  ```
- **`passes list`** — Discover advertised passes and their verification status.

  _Discover advertised passes and their verification status._

  ```bash
  navitime-pp-cli passes list --agent
  ```

## Command Reference

Use `capabilities` for verified, advertised, credential-gated and unavailable features; use each command's `--help` for flags. Read [data-contract.md](docs/data-contract.md) for overnight, fare, timing-basis or pass interpretation. Read [access.md](docs/access.md) when comparing website and API access.

## Recipes

### Resolve a station

```bash
navitime-pp-cli places search 大久保 --type station --agent
```

Choose a source station ref from the candidates.

### Compact route summaries

```bash
navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --agent --select routes.id,routes.departure_at,routes.arrival_at,routes.fare
```

Keep output focused on the travel window and displayed price.

### Compare a deadline

```bash
navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --agent
```

Compare returned alternatives before an appointment.

### Discover passes

```bash
navitime-pp-cli passes list --agent
```

Use an advertised ID with routes search --pass.

## Auth Setup

The verified Japan Travel website surface requires no API key or subscription. Runtime uses Firefox-compatible HTTP; no running browser or imported cookies.

Run `navitime-pp-cli doctor` to verify setup.

## Agent Mode

Default output is compact JSON. `--agent` preserves the same field paths and adds noninteractive defaults. Use `--select` or `--fields` for projection; `--pretty` is for human inspection. Diagnostics and `--metrics` go to stderr. Retain source URLs and freshness with conclusions; `null` means unknown.

## Paths and state

Use `NAVITIME_CACHE_DIR` or absolute `--cache-dir` to isolate public data. `--refresh` bypasses cache reads; `--no-cache` skips reads and writes. `routes show` reads a historical snapshot with its original fetch time. Temporary source challenges are errors: pause before retrying; do not infer no route from failed access.

## Exit Codes

Zero means completed, two invalid usage, three missing data. Other nonzero exits signal source, throttle or runtime failures. A valid empty result can succeed; inspect its notes and bounds.

## Argument Parsing

Offset-free date-times mean Asia/Tokyo; output timestamps carry an offset. First/last take a date. Calendar spans may be more precise than displayed leg clocks. Compare fares with their stated basis, including any taxi estimates.

## Anti-triggers

A schedule does not establish live operation or seat inventory. This CLI does not book, purchase passes, compute pass-holder out-of-pocket cost or optimize a multi-service itinerary.

## MCP Server Installation

Optional local build: `go build -o navitime-pp-mcp ./cmd/navitime-pp-mcp`. It serves stdio. Register only when the user requests configuration changes; CLI use needs no registration.

## Direct Use

Finish with selected source references, dated travel window, practical tradeoffs, fare/pass assumptions, fetch time and canonical NAVITIME URL.
