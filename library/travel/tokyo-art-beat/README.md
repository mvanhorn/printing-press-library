# Tokyo Art Beat CLI

Created by [@zjsng](https://github.com/zjsng) (zjsng).

**Discover Japanese exhibitions for trip dates with bilingual identities and explicit schedule uncertainty.**

## Install

The recommended path installs both the `tokyo-art-beat-pp-cli` binary and the `pp-tokyo-art-beat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tokyo-art-beat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tokyo-art-beat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tokyo-art-beat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tokyo-art-beat --agent claude-code
npx -y @mvanhorn/printing-press-library install tokyo-art-beat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/cmd/tokyo-art-beat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tokyo-art-beat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->

## Quick Start

```bash
# Check runtime prerequisites without network.
tokyo-art-beat-pp-cli doctor --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`events search`** — Filter inclusive date spans; closures require detail.

  _Filter inclusive date spans; closures require detail._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --limit 2 --agent
  ```
- **`events search`** — Filter exhibition starts or ends within trip dates.

  _Filter exhibition starts or ends within trip dates._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --relation starts --limit 2 --agent
  ```
- **`events detail`** — Assess a full-year date, preserving schedule uncertainty.

  _Assess a full-year date, preserving schedule uncertainty._

  ```bash
  tokyo-art-beat-pp-cli events detail 2ccd6619-6e5e-410c-851a-56bf5d4662dc --on 2027-05-03 --agent
  ```
- **`nearby`** — Rank bounded candidates by straight-line public venue distance.

  _Rank bounded candidates by straight-line public venue distance._

  ```bash
  tokyo-art-beat-pp-cli nearby --lat 35.6605 --lon 139.7292 --area Roppongi --candidates 100 --limit 2 --agent
  ```
- **`events search`** — Project fields and expose cache/source freshness.

  _Project fields and expose cache/source freshness._

  ```bash
  tokyo-art-beat-pp-cli events search --from 2026-10-01 --to 2026-10-07 --fields id,name,starts,ends --limit 2 --agent
  ```

## Recipes

### Trip exhibition search

```bash
tokyo-art-beat-pp-cli events search --area Roppongi --from 2026-10-01 --to 2026-10-07 --limit 2 --agent
```

Filter source date spans, preserving bilingual identities.

## Usage

Read-only public Japan exhibition, gallery and museum discovery. No credentials or browser runtime. Build: `go build -o tokyo-art-beat-pp-cli ./cmd/tokyo-art-beat-pp-cli`.

Search filters: `--query`, `--artist`, `--area`, `--category`, `--venue`. `venues search` adds `--type`. Resolve English/Japanese names or stable IDs using catalogs areas/categories/types. Search defaults to 10, maximum 50; `--offset` continues at meta.pagination.next_offset. Sorting uses one source key and an ID tiebreak inside each page; source updates can shift pagination.

Artist matching is local over bounded full-text candidates: default three pages, maximum five with `--max-scan-pages`. Nearby computes straight-line public-coordinate distance over up to 100 venue candidates; use `--area` to focus coverage. Source artist/coordinate operators can be silently ignored. Empty capped results do not establish absence of matching exhibitions. No walking times are inferred.

## Agent Usage

Compact JSON is `{meta,results,errors}` by default. Missing source values are null; EN/JA fields remain separate. Retain source URLs, Japanese names, freshness, partial/truncated flags and pagination. `--fields id,name,starts,ends` projects result fields while retaining metadata; `--select results.id,meta.stats` projects the envelope. `agent-context` describes the runtime tree. CSV/plain/quiet output contains results only.

## Health Check

`tokyo-art-beat-pp-cli doctor --json` checks prerequisites and public feed connectivity. `--dry-run` performs no network/cache operations.

## Cookbook

```sh
tokyo-art-beat-pp-cli venues detail import_venue_record__61183FDF --agent
tokyo-art-beat-pp-cli venues events import_venue_record__61183FDF --status all --limit 5 --agent
tokyo-art-beat-pp-cli nearby --lat 35.6605 --lon 139.7292 --area Roppongi --candidates 100 --limit 5 --agent
tokyo-art-beat-pp-cli compare 2ccd6619-6e5e-410c-851a-56bf5d4662dc import_event_record__2004_41B6 --agent
```

## Source interpretation

Full-year date windows are inclusive spans, not open-day calendars. Event hours and fees remain separate from venue defaults. `events detail --on` reports outside_span, weekly_closure, no_listed_weekly_closure or unknown. Exceptional/holiday notes, hidden/missing closures and unconfirmed dates require official confirmation; no_listed_weekly_closure never confirms opening. Publication status is not live opening status.

Archive timestamps normalize to YYYY-MM-DD while source values, start year and legacy archive edition year are retained separately. Fee categories and conditions remain bilingual source text. Last-admission phrases are preserved from notes; structured time remains null when unavailable. Official venue/exhibition links are source-provided. Ticket links are empty when absent. Listings never establish ticket inventory, reservation slots or sellout status. Public MuPon indicators require membership for redemption; member-only data and coupon use are excluded. No purchases, bookings or account changes.

## Freshness and bounds

The private response cache has a one-hour TTL, at most 128 files/25 MiB and seven-day eviction. `--cache-dir PATH` selects its location; `--fresh` refreshes, `--offline` reads exact cached queries (staleness explicit), `--no-cache` bypasses. Failed live requests do not silently use stale data. `--data-source auto|local|live` maps to TTL cache|offline|fresh.

Requests run sequentially at at most two per second, with one retry, 15-second per-request and 60-second default command deadlines (maximum 2m), a 4 MiB response cap and 20 logical/60 wire request budget. The feed is an undocumented website surface, not an official API.

## Troubleshooting

Errors emit structured JSON to stdout, diagnostics to stderr: 2 invalid input, 3 access denied, 4 missing identity, 5 network/schema/budget, 7 throttle. Partial joins/compares preserve successes and errors with meta.partial=true, exit 0. A cache miss requires the exact query online first. Unknown/ambiguous filter names include an actionable catalog hint. Correct the named flag or wait/retry the source failure.

Run `go test -count=1 ./...` and `go vet ./...`. Live and fixture checks are documented separately in the archived manuscripts.

## Distribution limits

Homebrew tap publishing is not configured: no verified tap destination is available. Use the documented library installer or Go install after maintainer merge. Dry runs validate local input and cache/output modes without cache or network access; source identities and filter names require a live lookup to resolve.
