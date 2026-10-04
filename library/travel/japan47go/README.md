# JAPAN47GO CLI

**Find local guides and experiences with published request, fee and participant evidence.**

Discover a bounded Japanese shortlist, inspect local association conditions, and compare notice, party and fee rules before choosing a date. Saved observations retain their source and retrieval times; availability remains unknown.

## Install

Before a merged catalog release, build the source with Go 1.27.1 or newer:

```sh
go build -o japan47go-pp-cli ./cmd/japan47go-pp-cli
go build -o japan47go-pp-mcp ./cmd/japan47go-pp-mcp
./japan47go-pp-cli --version
```

After catalog publication:

```sh
npx -y @mvanhorn/printing-press-library install japan47go --cli-only
```

The default installer directory must be on PATH. A Go fallback installs into the Go bin directory:

```sh
go install github.com/mvanhorn/printing-press-library/library/travel/japan47go/cmd/japan47go-pp-cli@latest
```

For MCP, build matched peers together or install the matched MCPB bundle. The included native bundle targets Darwin arm64 only; other platforms require their own builds. The server uses stdio only and resolves the companion CLI beside itself. Four domain tools mirror the actual Cobra commands; `services_compare` accepts `ids` as a comma-separated string, and its `as-of`/`require-free` parameter names retain hyphens. Inspect `tools/list` for the exact schema. The normalized domain handlers avoid raw provider payloads.

## Authentication

Public anonymous JAPAN47GO SSR pages; no API key or browser required. Provider reads only.

## Quick Start

```bash
# Check the installed runtime.
japan47go-pp-cli doctor --dry-run

# Find Japanese guide and experience candidates with honest page coverage.
japan47go-pp-cli services discover --query 妻籠 --kind guides --max-pages 1 --limit 3 --agent

# Inspect bounded Japanese request, duration, fee and schedule facts.
japan47go-pp-cli services inspect 2980022e-ef99-4115-95e5-be5227cdc74e --agent

# Check a requested date and party against explicit notice and participant rules.
japan47go-pp-cli services compare --ids 2980022e-ef99-4115-95e5-be5227cdc74e,0ad62a4e-2987-4e83-af63-7a6dd69e0d98 --on 2026-11-01 --as-of 2026-10-25 --party 1 --agent

```

## Commands

| Workflow | Purpose | Bounds |
|---|---|---|
| `services discover` | Native keyword/category candidates with page coverage | Default 1/hard 5 pages; default 10/hard 50 outputs |
| `services inspect` | Exact UUID or canonical detail URL; curated evidence | One record; HTTP body 4MiB maximum |
| `services compare` | Calendar notice, minimum party and optional zero-cost checks | 2..5 distinct records; partial failures explicit |
| `services saved` | Literal search of recorded facts without network access | 200 saved observations; maximum 50 outputs |

Guide tag 62 and experience tag 69 are native search menu categories. Detail guide category 617 is a different taxonomy. `source_matching_total` is the query's `pageInfo.totalCnt`; it excludes the global site inventory count. `scanned_records`, routes, continuation and `complete` describe the inspected window. Discovery never inspects details. Output truncation makes coverage incomplete even when every listing page was fetched. An empty scan does not prove absence.

## Evidence and comparison

Japanese names and bounded original terms are authoritative. The normalized request values, minute durations, fee statuses and comparison states are derived conveniences. Records omit staffing counts, guide ages, profile identities, personal contacts and images before cache/output. Only organization website URLs are retained.

- A single notice rule can produce a calendar request deadline. Weeks mean seven-day intervals. Months use the same day in the earlier month, clamped to its final day. The inclusive date calculation does not establish a cutoff hour or request acceptance. Multiple alternatives stay `ambiguous` and produce `unknown` compatibility.
- `--on` is the requested visit date; `--as-of` is the intended request date and defaults to the current Japan date. `--party` tests an explicitly stated minimum count. Missing minimum information remains unknown.
- `--require-free` requires explicit zero-cost evidence. Paid guide fees and required expenses are excluded. Volunteer labels never imply free. Contradictory fee narratives stay ambiguous. `total_jpy` is always null; source amounts retain qualifiers and units, including null when unstated.
- `source_closed=true` excludes the record. `source_closed=false` leaves `open_now` null and availability unknown. Start/end dates describe a published envelope; inside that envelope does not prove a session. Old winter dates never recur automatically.
- `supported_by_published_rules` means only that the specified notice/party/fee checks pass. It is not an availability or suitability guarantee.

## Agent Usage

Use `--agent` for JSON plus provenance. `--select` narrows service fields while query/coverage/source-boundary metadata stays visible. All custom commands support offline `--dry-run`; errors use typed exits and JSON envelopes, with warnings on stderr.

```sh
japan47go-pp-cli services inspect 2980022e-ef99-4115-95e5-be5227cdc74e --agent --select id,name_ja,source_url,request,price,durations_minutes,observed_at
```

For travel recommendations, cite `source_url`, the practical terms and `observed_at`, then explain unresolved facts. The generated local learning commands are separate from service observations. Their recall/teach workflow is documented in SKILL.md; `--no-learn` or `JAPAN47GO_NO_LEARN=true` makes test and deterministic flows explicit. No generic provider sync or full-text inventory search is offered.

Generic framework live export does not support JAPAN47GO SSR HTML. It fails with source/parser exit 5 and emits no service records; this does not indicate closed, sold out or absent inventory. For reusable source data, use normalized `services discover`, `services inspect`, `services compare` or `services saved` output with `--json`.

## Cache and paths

Successful live inspect/compare calls save normalized observations with normal SQLite transactions. At most 200 latest records are stored, each at most 64KiB. An integer instant timestamp prevents a delayed older save, varying offset or fraction-width ordering from replacing a newer observation. Saved reads use SQLite read-only mode, create no files or tables, and query at most 200 records. Missing storage produces an explicit empty local window.

`--data-source auto` prefers live reads and only falls back for transport/network failure, preserving the failure and original observation time. HTTP errors and parser/schema changes remain explicit errors. `--data-source live` never falls back; `local` reads only recorded facts. `--refresh` forces live rechecks and conflicts with `local`. `--no-cache` avoids saves and fallback. Local age and stale flags are explicit; source update dates never replace fetch dates.

Runtime settings use config/config.json under the selected home. Source reads require no credentials; there is no TOML/secret migration. `--home` or `JAPAN47GO_HOME` relocates config/data/state/cache together. MCP rejects arbitrary filesystem destination flags; configure its environment before starting. `agent-context --json` reports actual resolved paths. The observation database is `cache/japan47go-observations-v1.sqlite` under the selected home. Native defaults follow platform directories.

## Health Check

```sh
japan47go-pp-cli doctor --json
```

Doctor's source reachability is a health probe. It does not prove current guide operation or request acceptance.

## Troubleshooting

Usage errors exit 2; absent exact/local records exit 3; source/parser failure exit 5; exhausted throttling exits 7. Rate-limit failures never become empty source successes. Compare retains each failed read and excludes it from successful records; all failed reads error. A failed local save is a warning beside valid fresh facts.

If a bounded search is empty, inspect query coverage and deliberately widen `--max-pages` or the keyword. If a saved record is absent or stale, inspect its known UUID with `--refresh`. If source structure changes, report the parser error rather than treating it as zero matches. Unsupported article types explicitly error; this CLI accepts location/event detail records and does not turn stories or routes into service records.

### API-specific
- **No candidates in a bounded page window** — Inspect coverage and broaden --query or deliberately increase --max-pages.
- **Saved observation is absent or old** — Use services inspect UUID --refresh before relying on source conditions.

## Cookbook

```sh
japan47go-pp-cli services discover --query 博多 --kind guides --max-pages 1 --limit 3 --agent
japan47go-pp-cli services compare --ids 0ad62a4e-2987-4e83-af63-7a6dd69e0d98,c98eaa8d-a854-4494-88db-a04b6de17461 --require-free --agent
japan47go-pp-cli services saved --query 妻籠 --limit 3 --agent
```

The captured source-selection matrix spans Hokkaido, Nagano and Fukuoka and demonstrates Tsumago request lead time/four durations, Hakata lead time/expense categories, and Tanushimaru minimum party/notice evidence beyond the inspected tool contracts. Existing empty bounded scans are not evidence of global absence.

## Validation and source

Run `go test -count=1 ./...`, `go vet ./...` and matched CLI/MCP builds. Focused tests cover request ambiguity, month boundaries, qualified fees, private fields, HTTP throttling, pagination, missing read-only storage, cache bounds and delayed observations. The manuscript proofs record actual source/MCP calls and full shipcheck/live acceptance, with explicit skipped framework probes.

Source: [JAPAN47GO public Japanese pages](https://www.japan47go.travel/ja). Research and normalized evidence are archived in .manuscripts; raw captures remain private. Runtime stays 0.0.0-dev until the public library's post-merge release workflow stamps it. No release ledger is invented locally.

Printed with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) by [zjsng](https://github.com/zjsng). Apache-2.0.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local service decisions
- **`services discover`** — Find Japanese guide and experience candidates with honest page coverage.

  _Find Japanese guide and experience candidates with honest page coverage._

  ```bash
  japan47go-pp-cli services discover --query 妻籠 --kind guides --max-pages 1 --limit 3 --agent
  ```
- **`services inspect`** — Inspect bounded Japanese request, duration, fee and schedule facts.

  _Inspect bounded Japanese request, duration, fee and schedule facts._

  ```bash
  japan47go-pp-cli services inspect 2980022e-ef99-4115-95e5-be5227cdc74e --agent
  ```
- **`services compare`** — Check a requested date and party against explicit notice and participant rules.

  _Check a requested date and party against explicit notice and participant rules._

  ```bash
  japan47go-pp-cli services compare --ids 2980022e-ef99-4115-95e5-be5227cdc74e,0ad62a4e-2987-4e83-af63-7a6dd69e0d98 --on 2026-11-01 --as-of 2026-10-25 --party 1 --agent
  ```
- **`services compare`** — Compare fees without turning expense-based volunteer services into free tours.

  _Compare fees without turning expense-based volunteer services into free tours._

  ```bash
  japan47go-pp-cli services compare --ids 0ad62a4e-2987-4e83-af63-7a6dd69e0d98,c98eaa8d-a854-4494-88db-a04b6de17461 --require-free --agent
  ```

### Saved evidence
- **`services saved`** — Revisit saved normalized facts offline with original observation times.

  _Revisit saved normalized facts offline with original observation times._

  ```bash
  japan47go-pp-cli services saved --query 妻籠 --limit 3 --agent
  ```

## Recipes

### Guide candidates

```bash
japan47go-pp-cli services discover --query 妻籠 --kind guides --max-pages 1 --limit 3 --agent
```

Carry source query and page coverage into the answer.

### Compact source conditions

```bash
japan47go-pp-cli services inspect 2980022e-ef99-4115-95e5-be5227cdc74e --agent --select id,name_ja,source_url,request,price,durations_minutes,observed_at
```

Keep decision facts and provenance visible.

### Short notice and single traveler

```bash
japan47go-pp-cli services compare --ids 2980022e-ef99-4115-95e5-be5227cdc74e,0ad62a4e-2987-4e83-af63-7a6dd69e0d98 --on 2026-11-01 --as-of 2026-10-25 --party 1 --agent
```

Published-rule compatibility does not imply an available guide.

### Saved evidence

```bash
japan47go-pp-cli services saved --query 妻籠 --limit 3 --agent
```

Use recorded fetch times rather than implying current facts.
