---
name: pp-carstay
description: "Plan Japan designated overnight stops through Carstay: discover Japanese listings, compare facilities, check parking-space constraints, audit evidence and produce booking handoffs. Use for Carstay overnight spots, facilities, Japanese coverage, or carstay-pp-cli."
---

# Carstay overnight planning

## Prerequisites for this source build

This CLI is proposed for the public Printing Press library. Public installer commands remain unavailable until the PR is merged and a release is published. Before then, build the source checkout from this PR using Go 1.26.6 or newer:

```bash
cd library/travel/carstay
go build -trimpath -o carstay-pp-cli ./cmd/carstay-pp-cli
./carstay-pp-cli --version
```

Verify `carstay-pp-cli --version` in the runtime that will use this skill. Place the built binary on that runtime's `$PATH`, or invoke `./carstay-pp-cli` from its directory. Untagged source builds report `0.0.0-dev`.

The inactive template below records the post-merge installation contract for structural verification; it becomes usable after merge and release.

<!-- INACTIVE POST-MERGE TEMPLATE — unavailable until merge and release.
## Prerequisites: Install the CLI

This skill drives the `carstay-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install carstay --cli-only
   ```
2. Verify: `carstay-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/carstay/cmd/carstay-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.
-->

Search designated overnight spots using the fuller Japanese source, compare facilities and parking-space dimensions, and preserve unknowns. Dated results are provider-filtered candidates; complete quotes and availability require provider confirmation.

## When to Use This CLI

Use for designated Carstay overnight stations around a Japan campervan itinerary, Japanese listing coverage, facility/rule evidence, small shortlist comparisons, explicit parking-space constraints and canonical booking handoff. Public reads need no credentials.

## Anti-triggers

Use the provider website/human booking flow for reservations, payments, accounts, exact vacancy and complete dated quotes. Use a routing tool for road distance, driving time or tolls. Preserve original Japanese source text; this CLI does not generate translations or certify that any vehicle/pitch is accepted.

## Unique Capabilities

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

## Command Reference

Start with `carstay-pp-cli spots --help` and the relevant leaf help. `find` supports text, prefecture, language and dates. `show` keeps original source facts. Use `compare` to align a 2–5 ID shortlist, `fit` to test explicit requirements, and `audit` to inspect unresolved evidence. `near` ranks by straight-line kilometers, `coverage` counts locale gaps, and `handoff` prints the canonical link plus JST date query.

## Freshness Contract

`spots` commands read current public HTTP data and reject `--data-source local`. A dated result is `provider_date_filtered_candidate`, with `availability=unknown`. Its absence inside a scan cap is not unavailability. Starting prices are JPY references/night; option fee units and complete totals remain unknown. Parking dimensions are meters and represent parking-area size, not vehicle acceptance.

Preserve `source_present` true/false/null and facility notifications. Missing fields are unknown; false plus notification requires confirmation. On-site `restroom` differs from nearby `toilet`. Source option price zero with a fee note is not a free-service promise. Nearby numeric distances have an unverified source unit. Japanese is the fuller discovery baseline; English approval and actual translated-name presence are independent.

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

## Auth Setup

Public discovery needs no credentials. Complete bookings in Carstay’s canonical website flow.

Run `carstay-pp-cli doctor` to verify setup.

## Agent Mode

Use `--agent`; one `{meta, results}` envelope preserves source/observation/date/scan meaning, and empty collections are `[]`. Use `--select results.id,results.name_ja,results.source_url` for decision rows; selected agent results retain provenance. `--json` and `--dry-run` are also supported.

Bound work independently of output: `--limit` default 10/max 50; directory `--max-scan-records` default 500/max 5000; dated `--max-scan-pages` default 2/max 10; compare/fit/audit at most 5 IDs. `--text-limit` bounds source text per field (default 2000/max 12000 characters); truncation links back to the original page. Inspect `scan_complete`, `matched_within_scan`, `source_text_truncated` and `meta.fetch_failures` before making a complete-coverage claim.

Metadata-only agent selection keeps `{meta, results}` with `results=[]` and `meta.results_omitted_by_select=true`. Compare/fit/audit return nonzero when any requested detail read fails; partial rows retain `detail_fetch_complete=false`, requested/successful counts and `fetch_failures`.

HTTP 429 is a source error, never zero inventory. Every public client call honors root `--timeout`; read pacing is capped at 2 requests/second. Source-contract errors remain failures rather than empty results.

MCP companion inputs are scalar slots: compare requires `first` and `second`; fit/audit require `id`; additional IDs use `second`/`third`/`fourth`/`fifth` where applicable. Show/handoff require `id`; near requires `lat`/`lon`. Do not pass a space-joined variadic ID string.

Generated learning read tools can create/migrate their local store despite a read-only hint; recall can record local events. Treat those framework tools as local writes. `--no-learn` suppresses optional journaling/recall events but does not turn every learning store open into a read-only operation.

## Direct Use

A separate generated reference mirror supports `sync --resources directory` and offline `search --type directory`. It can contain activity-only rows and stale reference data; `spots` handles current overnight evidence. Read README.md for local installation, mirror examples and troubleshooting.

The MCP companion preserves bounded partial CLI evidence in the first content block when a tool fails, keeps `isError=true`, and separates diagnostics. Inspect both the failed-tool flag and `meta.fetch_failures`/coverage; partial rows never certify a complete shortlist.

The optional local learning engine requires a unique literal prefix match before treating a cached resource ID as verified. Teaching undo atomically invalidates affected inferred query/resource/venue families while retaining explicit taught and unrelated patterns. Native `spots` reads use current provider evidence independently of these local learnings.
