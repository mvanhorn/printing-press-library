---
name: pp-weathernews
description: "Japan weather and seasonal travel evidence with explicit dates, freshness and coverage. Trigger phrases: `Japan weather evidence`, `Kyoto foliage dates`, `cherry blossom reports`, `use weathernews`, `run weathernews`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "places|weather|season <command> [flags]"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - weathernews-pp-cli
---

# Weathernews — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `weathernews-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install weathernews --cli-only
   ```
2. Verify: `weathernews-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/weathernews/cmd/weathernews-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Resolve Japanese places, inspect bounded forecasts and seasonal evidence, and compare caller criteria. Public first-party access without a paid account.

## When to Use This CLI

Resolve Japanese weather places, check bounded forecasts, inspect cherry blossom or autumn foliage evidence, or compare explicit caller thresholds. All data comes from Weathernews public first-party pages.

## Anti-triggers

Use another workflow for bookings, purchases, universal destination ratings, other providers, commercial WxTech, or member-only radar. See README coverage before interpreting mountain-place coordinates as summit evidence.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Travel evidence
- **`season search`** — Bounded literal Japanese seasonal search with lazy details

  _Bounded literal Japanese seasonal search with lazy details_

  ```bash
  weathernews-pp-cli season search --product koyo --area kyoto --query 嵐山 --agent
  ```
- **`season show`** — Separate observation, forecast, norm, season year and ended state

  _Separate observation, forecast, norm, season year and ended state_

  ```bash
  weathernews-pp-cli season show --product koyo --id 26102 --agent
  ```
- **`weather compare`** — Compare caller daily thresholds across bounded exact coordinates

  _Compare caller daily thresholds across bounded exact coordinates_

  ```bash
  weathernews-pp-cli weather compare --points 'Kyoto=35.01167,135.76806|Tokyo=35.681,139.767' --date 2026-10-01 --max-pop 40 --agent
  ```
- **`season compare`** — Compare caller date proximity to published peak predictions

  _Compare caller date proximity to published peak predictions_

  ```bash
  weathernews-pp-cli season compare --product koyo --ids 26101,26111 --date 2026-11-26 --max-days-from-peak 3 --agent
  ```
- **`places resolve`** — Preserve source names, URLs and coordinates before forecast selection

  _Preserve source names, URLs and coordinates before forecast selection_

  ```bash
  weathernews-pp-cli places resolve --query 高尾山 --agent
  ```

## Command Reference

Read `weathernews-pp-cli places --help`, `weathernews-pp-cli weather --help` and `weathernews-pp-cli season --help` for live flag truth. Resolve names before choosing coordinates; search each seasonal product before selecting its spot IDs.

## Recipes

### Compact seasonal search

```bash
weathernews-pp-cli season search --product koyo --area kyoto --agent --select items.id,items.name_ja,season
```

Find source IDs before lazy details

## Auth Setup

Selected public products need no credentials. Member-only products and commercial WxTech are outside scope.

Run `weathernews-pp-cli doctor` to verify setup.

## Agent Mode

Focused commands emit compact JSON. `--agent` adds meta/results; `--select` projects dotted paths through arrays. Retain season/source/horizon when answering freshness questions. Metrics/errors are stderr. Keep issue time null when missing; never substitute fetch time or first valid time. Separate observation, forecast and historical norm; infer date year only from the source season title. Elevation stays null if absent.

## Paths and state

Use `--home /absolute/path` for isolated config/state/cache, and `--no-learn` for deterministic read-only tasks. Focused caches have bounded TTL and no stale fallback. `--refresh` explicitly refreshes inventory; `--data-source local` serves only fresh cached source snapshots. Paginate with `next_offset` before refreshing; spot details are lazy.

## Health Check

Run `weathernews-pp-cli doctor --dry-run` locally; `places resolve --query 京都 --metrics` checks live public access.

## Troubleshooting

Check stderr and exit code: 2 usage/projection, 3 not-found/cache miss, 4 access restriction, 5 upstream/schema/partial comparison, 7 rate limit, 10 local path/config. An empty search is valid. Partial comparisons emit candidate errors plus exit 5. See README and evidence/final-report.md for current seasonal coverage and live verification.
