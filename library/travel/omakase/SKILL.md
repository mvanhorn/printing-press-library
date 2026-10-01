---
name: pp-omakase
description: Inspect OMAKASE restaurant discovery, course prices, cancellation terms and source reservation releases. Use for OMAKASE planning or public membership boundaries; exact date/party seats remain unknown without accessible source inventory.
author: zjsng
license: Apache-2.0
allowed-tools: Read Bash
---

# OMAKASE planning

## Prerequisites: Install the CLI

This skill drives the `omakase-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install omakase --cli-only
   ```
2. Verify: `omakase-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/omakase/cmd/omakase-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Find OMAKASE restaurant names, read public restaurant/course details, inspect releases and compare source terms. Integrates only OMAKASE English and Japanese documents.

## Anti-triggers

Use the canonical website for booking or payment. Other providers, account eligibility, Premium calendars and exact seat queries require a different authorized workflow.

## Workflow

1. Run `omakase-pp-cli restaurants find --query Sugita --limit 5`. Results are literal name matches on a bounded source page; inspect coverage and follow `next_offset`, then `next_page` as needed.
2. Read a selected source ID using `omakase-pp-cli restaurants show hc778124`. Japanese name costs one extra document; `--japanese=false` avoids it.
3. Inspect `courses`, `release` or `compare` for the decision. Preserve source price floors, service/reservation fees and cancellation caveats.
4. Hand off the result's canonical restaurant URL. Exact seats remain unknown when the public source does not expose them.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Restaurant planning
- **`courses`** — Inspect public course price floors, units, service charges, reservation fee and cancellation caveats.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli courses hc778124 --agent
  ```
- **`release`** — Read source release timing in JST; distinguish scheduled, irregular and undetermined releases from seats.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli release jv742052 --agent
  ```
- **`availability`** — Inspect a planning date and party with unknown exact seats and query_evaluated false when public access is gated.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli availability hc778124 --date 2026-11-01 --party 2 --agent
  ```
- **`compare`** — Compare public terms for two to five restaurant IDs with explicit per-ID failures.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli compare hc778124 qt951856 --agent
  ```
- **`inventory`** — Explicitly refresh bounded summary pages and inspect or search local inventory coverage.

  _Inspect source terms before booking by canonical link._

  ```bash
  omakase-pp-cli inventory --agent
  ```

## Recipes

### Compare courses

```bash
omakase-pp-cli compare hc778124 qt951856 --agent --select results.id,results.name,results.courses
```

Bounded public comparison.

### Inspect release

```bash
omakase-pp-cli release jv742052 --agent
```

Release timing is separate from seats.

### Inspect availability boundary

```bash
omakase-pp-cli availability hc778124 --date 2026-11-01 --party 2 --agent
```

Report unknown when login is required.

## Agent Mode

Planning JSON is compact by default. Use `--select results.id,results.name` to narrow it. `null` means missing/unknown; `[]` means an empty collection. Read `meta.partial`, request/cache metrics and per-item `evidence.fetched_at`; retrieval time is not provider update time.

## Auth Setup

Public discovery and detail need no account. Exact seats require login; Premium search and calendars remain gated. No paid account required by this CLI.

Run `omakase-pp-cli doctor` to verify setup.

## Paths and state

`--home /absolute/path` isolates cache/data/config. `inventory refresh` is explicit and atomic. Detail is lazy; document cache is bounded at 128 entries. `--refresh` forces current reads; `--offline` uses cached evidence and marks staleness. Online failures do not silently fall back. `--no-learn` disables optional generated local learning.

## Exit Codes

0 success/valid empty/observed unknown; 2 usage; 3 missing source; 4 access blocked; 5 source/network/parse or partial comparison failure; 7 rate limit; 10 local configuration. Compare permits explicit `--allow-partial`. Diagnostics use stderr.
