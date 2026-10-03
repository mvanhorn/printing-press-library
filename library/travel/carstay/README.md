# Carstay CLI

**Discover Japan overnight spots with Japanese coverage, honest facility evidence and booking links.**

Created by [@zjsng](https://github.com/zjsng) (zjsng).

Search designated overnight spots using the fuller Japanese source, compare facilities and parking-space dimensions, and preserve unknowns. Dated results are provider-filtered candidates; complete quotes and availability require provider confirmation.

## Install from this source checkout

Carstay is published in the Printing Press library. Build this source checkout to use the amendment fixes before they merge; the public catalog installer follows the current published release. Use Go 1.26.6 or newer:

```bash
cd library/travel/carstay
go build -trimpath -o carstay-pp-cli ./cmd/carstay-pp-cli
./carstay-pp-cli --version
./carstay-pp-cli spots coverage --agent
```

Run the commands below with `./carstay-pp-cli`, or place the binary in a directory on your `$PATH`. This checkout retains the current published version stamp (`2026.10.1`); the library release process assigns the next version after merge.

Build the optional stdio MCP companion from the same directory with `go build -trimpath -o carstay-pp-mcp ./cmd/carstay-pp-mcp`. Start it with `./carstay-pp-mcp` in an MCP host that supports stdio. Building it does not change agent configuration.

## Authentication

Public discovery needs no credentials. Complete bookings in Carstay’s canonical website flow.

## Quick Start

```bash
# Check the command without credentials or network.
carstay-pp-cli doctor --dry-run

# Start with Japanese coverage.
carstay-pp-cli spots find --prefecture Yamanashi --limit 5 --agent

# Inspect a known overnight station.
carstay-pp-cli spots show 632c59b82b614b99a252d1b2 --agent

# Evaluate explicit constraints, preserving unknown acceptance.
carstay-pp-cli spots fit 632c59b82b614b99a252d1b2 --length-m 6 --width-m 2.1 --require electricity,restroom --agent

```

## Source meaning

Japanese is the default and fuller source. `--lang en` on find/near uses only English-approved records; approval and actual translated-name presence are separate facts. Every station keeps its original Japanese name, stable ID, canonical URL and observation timestamp. `spots coverage` reports current corpus counts and language gaps. Activity-only records are excluded from overnight workflows.

A dated search returns **provider date-filtered candidates**. `availability` remains `unknown`; a result or its absence in a capped scan does not establish vacancy or unavailability. Check-in/out strings are JST calendar dates. The wire page number starts at 1. The public calendar has recurring rules and orders; this CLI does not reproduce the provider's final inventory calculation.

Prices are JPY starting references per night, never complete dated quotes. Facility numeric option prices retain an unknown unit/basis until the provider confirms them. A zero source price alongside a false flag and a fee note is not a free-service promise. Nearby distances retain their unverified source unit; only calculated proximity is labeled straight-line kilometers.

Facilities preserve `source_present` as true/false/null plus `notification`. Missing flags are unknown. A false flag with a note becomes `requires_confirmation`, because a note can qualify access or describe a separately paid service. On-site `restroom` and nearby `toilet` are different fields. Parking-space length/width/height are meters; satisfying dimensions and facility flags does not certify vehicle or pitch acceptance.

## Unique Features

These commands compute bounded evidence views from the public Carstay station source.

### Overnight itinerary evidence
- **`spots compare`** — Contrast a small shortlist with facility, price and parking-space evidence.

  _Use when itinerary decisions require comparable source evidence instead of a raw record._

  ```bash
  carstay-pp-cli spots compare 632c59b82b614b99a252d1b2 5cff4813839680041631c452 --agent
  ```
- **`spots fit`** — Check explicit space dimensions and facility requirements while preserving unknowns.

  _Use when itinerary decisions require comparable source evidence instead of a raw record._

  ```bash
  carstay-pp-cli spots fit 632c59b82b614b99a252d1b2 --length-m 6 --width-m 2.1 --require electricity,restroom --agent
  ```
- **`spots near`** — Rank designated overnight spots around an itinerary coordinate by straight-line distance.

  _Use when itinerary decisions require comparable source evidence instead of a raw record._

  ```bash
  carstay-pp-cli spots near --lat 35.5 --lon 138.75 --radius-km 60 --limit 5 --agent
  ```
- **`spots audit`** — Expose unresolved price, availability and qualified facility evidence before booking.

  _Use when itinerary decisions require comparable source evidence instead of a raw record._

  ```bash
  carstay-pp-cli spots audit 632c59b82b614b99a252d1b2 --check-in 2026-10-10 --check-out 2026-10-11 --agent
  ```

### Japanese source coverage
- **`spots coverage`** — See Japanese options and English publication or translation gaps.

  _Use when itinerary decisions require comparable source evidence instead of a raw record._

  ```bash
  carstay-pp-cli spots coverage --prefecture Yamanashi --agent
  ```

### Optional learning evidence

Manual pattern teaching uses its current resource type, venue, entity kind and examples. Inference and undo use only positive `boost` teachings from eligible sources; `hide` and `alias_of` rows stay explicit local rules and never support a synthesized boost. Undo retains an inferred rule only when its remaining distinct positive bindings still support it. Recall keeps a known conflicting cached resource in the mismatch evidence. Synthesized patterns validate only the entity actually substituted; another entity mentioned by the query cannot validate that target. A pattern tries later query bindings after a cached identity conflict and contributes one valid binding. Recall keeps the best identity evidence for each resource type and ID, then applies its result limit; final returned targets do not also appear as rejected alternatives. Ordinary recall ranks pattern metadata by its final confidence order and stops identity reads once enough exact targets are settled. Direct teachings remain scanned; matching pattern metadata is scanned when pattern evaluation is needed, and `--debug-mismatches` may validate remaining patterns to collect rejected evidence. Standalone pattern application retains its first ID-verified binding, score ordering and caps. An unavailable cached identity retains identifier-only pattern fallback and proves no provider facts. Only a genuinely missing row uses the warned missing-resource fallback; cancellation and unexpected SQLite payload-read errors fail explicitly.

```bash
carstay-pp-cli recall 'Yamanashi overnight stop' --debug-mismatches --json
```

This optional helper may migrate or record local learning state; native `spots` provider evidence remains independent. The MCP mirror advertises these local effects. For a failed native MCP shortlist call, inspect `isError` and its first bounded evidence block before reading separate diagnostics.

## Commands

| Command | Decision supported |
|---|---|
| `spots find` | Text/prefecture discovery and optional dated candidacy |
| `spots show` | Whitelisted original facilities, rules and parking-space evidence |
| `spots compare` | A consistent matrix for 2–5 overnight station IDs |
| `spots fit` | Explicit length/width/height and facility constraints for 1–5 IDs |
| `spots near` | Radial ranking around an explicit latitude/longitude |
| `spots coverage` | Japanese baseline versus English approval and translated names |
| `spots audit` | Unknown dimensions, fees, qualified facilities and dated observation scope |
| `spots handoff` | Print a provider link; performs no booking or browser launch |
| `directory` | Bounded reference summaries including activity markers |

Use each command's help for current flags. The three similar evidence tools have distinct purposes: compare aligns facts, fit checks supplied requirements, and audit lists unresolved evidence.

## Agent Usage

```bash
carstay-pp-cli spots find --prefecture Yamanashi --limit 5 --agent --select results.id,results.name_ja,results.source_url
carstay-pp-cli spots coverage --agent --select results.japanese_overnight,results.english_approved_overnight
carstay-pp-cli spots find carstay-impossible-query-9f04 --json
```

With `--agent --select meta`, `results` is `[]` and `meta.results_omitted_by_select=true` distinguishes omitted rows from empty inventory. Shortlist commands (`compare`, `fit`, `audit`) preserve bounded partial evidence and return nonzero when requested detail reads are incomplete; ordinary partial failures use API exit 5. Rate limits and cancellation abort through their classified failure path. `detail_fetch_complete`, requested/successful counts and `fetch_failures` show the partial scope.

Agent output has one `{meta, results}` envelope. `meta` records source, JST date meaning, scan scope and fetch failures. Empty results are `[]`. `--select` narrows fields; selected agent results retain provenance metadata. `--json` also works independently. Dry runs return JSON without source reads. The domain commands use public live reads; `--data-source local` returns a usage error instead of inventing cached inventory.

Output and scan bounds are separate: `--limit` defaults to 10 (maximum 50); undated scans use `--max-scan-records` default 500 (maximum 5000). Dated scans use `--max-scan-pages` default 2 (maximum 10), with provider total and actual scanned rows reported. Shortlist commands cap IDs at 5. Source text defaults to 2000 Unicode characters per field; `--text-limit` can raise it to 12000, with truncation flagged and the canonical page linked. The HTTP client caps each response at 8 MiB, paces calls at at most 2 requests/second, and honors the root timeout. HTTP 429 is a typed failure, never an empty result.

The optional `carstay-pp-mcp` companion uses stdio. Its domain tools expose required scalar station IDs: compare takes `first`/`second` and optional `third`/`fourth`/`fifth`; fit/audit take `id` and up to four further scalar IDs; show/handoff take `id`; near requires numeric `lat`/`lon`. The package is rebuilt with the CLI; no agent configuration is changed.

Cached-resource alias recall requires one canonical shared by the query, teaching and actual cached resource; a conflicting target remains a mismatch. Missing-resource fallback keeps its explicit warning.

Optional learning reads (`recall`, `learnings list/candidates/stats`, `playbook list`) are advertised as local writes in MCP: they keep their current store migration, event or pruning behavior and return current data or an explicit store-unavailable error while another SQLite writer holds WAL mode; after that writer closes, recall sees the committed data. Native read-only MCP mirrors enforce a child context that suppresses implicit journals, flag derivation, refresh/cache writes and receipts. Normal CLI optional journaling remains enabled unless `--no-learn` is selected; that switch does not make every learning-store open read-only. Carstay station requests are public GETs.

## Freshness

Every spots invocation fetches live public observations; it does not cache booking availability. The separate generated directory mirror supports local reference search and SQL:

```bash
carstay-pp-cli sync --resources directory --db /tmp/carstay-directory.sqlite --max-pages 1
carstay-pp-cli search '鈴鹿' --type directory --db /tmp/carstay-directory.sqlite --limit 5 --agent
```

That mirror is a reference snapshot and may include activity-only records. Its generated cache freshness policy is 24h. Use the `spots` workflows for current overnight decisions and explicit unknowns. `--no-learn` disables the framework's optional local learning journal when deterministic reference tooling is wanted. No credentials or cookies are needed.

## Health Check

```bash
carstay-pp-cli doctor --json
```

Observed during the original installed-build live gate: `api="reachable (HTML body at /ja/stations/)"`, `auth="not required"`, `config="ok"`, `version="0.1.0"`. The isolated reference cache initially had status unknown before sync.

A health response confirms transport/runtime setup; it does not verify dated availability. Inspect current source coverage with `spots coverage`.

## Troubleshooting

| Symptom | Action |
|---|---|
| English discovery misses options | Use `spots find --lang ja` and inspect `spots coverage`. |
| Date scan does not include an ID | Inspect `scan_complete` and raise `--max-scan-pages`; absence within a cap is not unavailability. |
| False facility flag has an access or fee note | Read `spots show`/`spots audit` and confirm with the provider. |
| Fit says requires confirmation | Inspect missing dimensions and qualified facilities; acceptance stays unknown. |
| HTTP 429 | Respect Retry-After/backoff and retry later; the command reports a source failure. |
| Source JSON changes or returns HTML | Keep the reported error; verify the canonical provider page instead of interpreting it as zero inventory. |

### API-specific
- **English search misses options** — Use --lang ja and spots coverage to inspect publication gaps.
- **Facility false flag has a fee or access note** — Read spots show or spots audit and confirm the qualified evidence with the provider.

## Cookbook

```bash
carstay-pp-cli spots compare 632c59b82b614b99a252d1b2 5cff4813839680041631c452 --agent
carstay-pp-cli spots near --lat 35.5 --lon 138.75 --radius-km 60 --limit 5 --agent
carstay-pp-cli spots audit 632c59b82b614b99a252d1b2 --check-in 2026-10-10 --check-out 2026-10-11 --agent
```

`near` gives straight-line proximity, not driving routes, tolls or travel time. `audit` keeps a dated candidate's availability and complete quote unknown. The CLI does not translate rules, certify broad vehicle acceptance, monitor vacancies, log in, reserve or pay.

## Validation

The project includes deterministic parser, facility-unknown, dimension-boundary, geodesic, date, pagination, whitelist, HTTP 429/context and flat-agent-envelope tests. A sanitized source-contract brief and the fresh binary-owned full live acceptance marker are included under `.manuscripts/`. Raw live responses, local paths and receipt ledgers are excluded from this public package.

## Recipes

### Compare overnight options

```bash
carstay-pp-cli spots compare 632c59b82b614b99a252d1b2 5cff4813839680041631c452 --agent
```

Read a bounded evidence matrix, with starting prices.

### Find stops near an itinerary point

```bash
carstay-pp-cli spots near --lat 35.5 --lon 138.75 --radius-km 60 --limit 5 --agent
```

Distance is straight-line kilometers, not driving distance.

### Project Japanese coverage

```bash
carstay-pp-cli spots coverage --prefecture Yamanashi --agent --select results.japanese_overnight,results.english_approved_overnight
```

Keep only decision fields in agent output.

### Audit a dated candidate

```bash
carstay-pp-cli spots audit 632c59b82b614b99a252d1b2 --check-in 2026-10-10 --check-out 2026-10-11 --agent
```

Check missing or qualified evidence without claiming vacancy.

The MCP companion preserves bounded partial CLI evidence in the first content block when a tool fails, keeps `isError=true`, and separates diagnostics. Inspect both the failed-tool flag and `meta.fetch_failures`/coverage; partial rows never certify a complete shortlist.

The optional local learning engine requires a unique literal prefix match before treating a cached resource ID as verified. Explicit pattern teaching records its current resource type, venue, entity kind and examples; later inference preserves that manual payload. Teaching undo atomically reconciles affected inferred query/resource/venue families: rules with at least two distinct compatible retained bindings survive with examples from those supporters; stale or incompatible rows do not veto the compatible cohort, unsupported rules are removed, and explicit taught or unrelated patterns remain. Native `spots` reads use current provider evidence independently of these local learnings.

The generated directory client refuses foreign effective-origin redirects before any inherited configured/per-call headers or cookies are sent. Same-origin normalized default ports, fresh auth signing and the ten-hop limit remain; policy refusals return directly. Existing binary body/MIME and stream timeout rules stay separately scoped. Native `spots` requests do not use configured credential headers; their source URLs are canonical provider references and do not attest the final redirect origin.
