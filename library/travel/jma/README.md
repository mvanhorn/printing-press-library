# Japan Meteorological Agency CLI

**Resolve JMA areas and inspect official forecasts, warnings and uncertain typhoon tracks.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `jma-pp-cli` binary and the `pp-jma` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jma
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jma --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jma --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jma --agent claude-code
npx -y @mvanhorn/printing-press-library install jma --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jma/cmd/jma-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jma-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->

## Authentication

Public anonymous HTTPS; no credentials or fees.

## Quick Start

```bash
# Inspect configuration without network
jma-pp-cli doctor --dry-run

# Discover source IDs
jma-pp-cli areas search --query Kyoto --kind municipality

# Read Tokyo district forecasts
jma-pp-cli forecast get --area 130010 --days 3

# Read applicable municipality weather products
jma-pp-cli warnings get --area 1310100

# Discover current TC IDs before detail
jma-pp-cli typhoons list

```

## Agent Usage

Compact JSON is the default, including on a terminal. `--agent` and `--json` are accepted. Every weather result retains `meta` with source URLs, original retrieval timestamps, cache ages, request counts, elapsed time and coverage. Units appear in field names; all normalized timestamps use JST (`+09:00`). Missing source values are JSON null, including Japanese station names missing from source metadata.

```sh
./jma-pp-cli forecast get --area 130010 --agent --select results.series
./jma-pp-cli warnings get --area 1340100 --detail --select state,municipalities.events
./jma-pp-cli stations search --query 44132
./jma-pp-cli areas search --query Osaka --limit 5 --offset 0
```

Projection supports dotted paths through arrays and retains provenance and pagination. Unknown fields fail with exit 2 before stdout. Search uses source substrings; resolution requires an exact source name or ID and rejects ambiguity. Districts, subdivisions and municipalities walk JMA's parent hierarchy. Stations are forecast temperature reference stations, not observations or geocoded destinations. Office requests cover all their source districts; `--station` selects a source station within the resolved district. `page.next_offset` is null at the end.

## Forecast interpretation

`forecast get --period short|week|all --days 1..7` returns separate weather, probability and temperature series. Default: all, three days. Short days are counted from the short bulletin's issue date; weekly days from the following day. Weather is valid until JST midnight; short precipitation probabilities apply to six-hour intervals. Short temperature timeDefines are source markers for daily extrema, not instantaneous measurements. Weekly temperature ranges and A/B/C reliability codes are preserved from JMA. Temperature stations and weekly regions can cover a broader area than the selected municipality; the source IDs make that coverage explicit. No local forecast is invented for a missing weekly region.

## Warnings and coverage

`warnings get` uses the current **2026 r8** source, including JMA's urgent warning classification and separate landslide warnings. The old warning endpoint remains reachable but is stale and is not used. Each source product has its own issued time, headline, hazard code and lifecycle. `issued`, `continued`, `updated` (source downgrade) and `lifted` remain distinct. Headlines describe the issuing office's area, while the municipality rows describe the selected destination.

`state` is `active`, `none_reported` or `incomplete`. `none_reported` requires known complete source records for all selected municipalities across applicable weather products; lifted entries stay visible. Missing/unknown records never establish absence of warnings. JMA's no_wave_tide catalog explicitly identifies inland municipalities outside wave/surge applicability; those products are `not_applicable`, distinct from missing or no-warning records. Summary state covers all resolved municipalities before pagination. `has_active_hazards` can remain true when state is incomplete.

Coverage is explicitly partial: municipality weather warnings only. Joint river flood bulletins and coastal forecast-zone supplements are outside this CLI. JMA does not supply a warning expiry in this surface; `valid_until` is null, and issue age is not expiration. A no-warning result is not personal safety clearance or an evacuation instruction. Follow the source wording and current local authorities' instructions.

## Freshness and resource bounds

An embedded JMA area/station inventory supports offline discovery. Its capture timestamp is visible. Run `inventory refresh` to validate eight source catalogs and atomically replace the local inventory; reads never refresh it implicitly. The previous inventory survives a failed refresh.

HTTP cache is capped at 128 payloads / 32 MiB with oldest payload eviction; inventory persists separately. HTTP cache: forecast five minutes; warnings/typhoons 60 seconds. `--refresh` forces network retrieval. `--no-cache` bypasses HTTP reads and writes. Cache write/cleanup failures return valid live data with a `meta.notes` diagnostic and disable HTTP cache writes for the remaining command while retaining fresh cached reads; explicit inventory refresh still requires its snapshot write to succeed. `--offline` uses only inventory and still-fresh HTTP cache; expired cache is never a fallback. Default cache: OS user cache directory under `jma-cli`; override with `--cache-dir`. `--home` is an optional absolute path isolating retained Printing Press framework state.

Requests are paced at a maximum of four per second and sequential, at most two attempts per URL, 2 MiB per response, ten seconds per attempt and 45 seconds for the whole command by default (`--timeout` >0 and <=60s). Discovery defaults to 20 rows; `--limit` 1..100 and `--offset` 0..100000. Typhoon list fetches one document; detail fetches index, geometry and specifications. Mismatched issue/valid times fail as incomplete and advise `--refresh`. Source issue ages are visible; old forecasts (>24h) or cyclone detail (>12h) are labeled stale.

## Health Check

```sh
./jma-pp-cli doctor --dry-run
./jma-pp-cli doctor
```

The live check validates inventory and the public cyclone index. There are no credentials to configure. The optional MCP companion (`make build-all`) mirrors the focused normalized commands and preserves incomplete warning JSON as an error result. It limits execution to two concurrent companion calls and rejects excess calls for later retry. Keep both binaries beside each other.

Additional generated Printing Press framework utilities are hidden advanced commands; the documented JMA commands use a bounded client and do not open the framework SQLite store.

## Troubleshooting

- Exit 2: invalid input, ambiguous/unknown area, unavailable current cyclone ID or bad field projection. Resolve an ID with `areas search` or `typhoons list`.
- Exit 4: HTTP/transport/timeout/cache or offline miss. Retry with `--refresh`, inspect JMA directly, or choose a writable `--cache-dir` / `--no-cache`.
- Exit 5: incompatible or incomplete source documents. Inspect stderr and any incomplete JSON result; retry with `--refresh`. Retrieval failure produces no no-warning claim.
- `incomplete` warning state (exit 5, with the incomplete JSON preserved): source records could not establish complete coverage. Inspect product IDs and municipality events; use JMA's canonical page.
- Missing Japanese station name: source metadata lacks it; the forecast response can still supply its source name.
- Broken inventory: run `inventory refresh`; malformed stored inventory is reported rather than silently replaced.

## Cookbook

```sh
./jma-pp-cli inventory refresh
./jma-pp-cli forecast get --area 014030 --period week --days 7
./jma-pp-cli forecast get --area 130010 --station 44132 --period short
./jma-pp-cli warnings get --area 130000 --limit 5 --offset 0
./jma-pp-cli typhoons list --select results.id,results.issued_at
```

## Build and verification

`go test -count=1 ./...` and `go vet ./...`. The source module is standalone; no go.work is needed. [Final verification](evidence/FINAL.md) records live source checks, request/size/latency/memory measurements, independent review and Printing Press acceptance. `scripts/live_verify.py` requires public network access and current cyclone IDs. Deterministic tests use labeled synthetic data only for consequential parsing/state logic.

Canonical project: `<source-project>`. Printing Press staging and library paths are recorded in [paths](evidence/PATHS.md). No publishing or PR was performed.

## Sources and reuse

[Forecast services](https://www.jma.go.jp/jma/en/Activities/forecast.html), [warning classification](https://www.jma.go.jp/jma/kishou/know/bosai/warning.html), [typhoon uncertainty](https://www.jma.go.jp/jma/kishou/know/typhoon/7-1.html), [JMA terms](https://www.jma.go.jp/jma/en/copyright.html). Runtime URLs are first-party website JSON observed in JMA frontend code, not a guaranteed stable API. This CLI processes JMA data; JMA does not endorse it. Printing Press generated the framework. Archived community endpoint documentation and JMA weather MCP projects informed the focused scope; source data and wording come from JMA.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Source correctness
- **`areas resolve`** — Resolve source IDs and reject ambiguous destination names.

  _Resolve source IDs and reject ambiguous destination names._

  ```bash
  jma-pp-cli areas resolve --area 1310100 --agent
  ```
- **`warnings get`** — Separate applicable no-warning, not-applicable, lifted and incomplete source records.

  _Separate applicable no-warning, not-applicable, lifted and incomplete source records._

  ```bash
  jma-pp-cli warnings get --area 1340100 --agent
  ```
- **`typhoons get`** — Join matching JMA issued/valid-time geometry and intensity documents.

  _Join matching JMA issued/valid-time geometry and intensity documents._

  ```bash
  jma-pp-cli typhoons get --id TC2633 --agent
  ```
- **`typhoons list`** — Discover current source cyclone IDs without detail requests.

  _Discover current source cyclone IDs without detail requests._

  ```bash
  jma-pp-cli typhoons list --agent
  ```
- **`inventory refresh`** — Validate first-party source catalogs before atomic local replacement.

  _Validate first-party source catalogs before atomic local replacement._

  ```bash
  jma-pp-cli inventory refresh --agent
  ```

## Recipes

### Tokyo warnings

```bash
jma-pp-cli warnings get --area 1310100 --agent --select results
```

Municipality-specific source status

### Typhoon discovery

```bash
jma-pp-cli typhoons list --agent
```

Lazy active cyclone inventory
