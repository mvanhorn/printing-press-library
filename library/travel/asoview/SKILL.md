---
name: pp-asoview
description: Discover and compare Asoview Japan leisure tickets, inspect public dates and price bands, or return booking handoff URLs. Use for Asoview tickets, Asoview availability, or Japan attraction comparisons anchored to Asoview.
license: Apache-2.0
---

# Asoview

## Local build (current release)

Use the local build from this project or its promoted local library copy. It has not been published to a registry. From the project:

```bash
go build -o asoview-pp-cli ./cmd/asoview-pp-cli
./asoview-pp-cli --version
```

Resolve `asoview-pp-cli` from the local build path before running examples below. Requires Go 1.26.6 or newer to build; no credentials or paid account for the verified public surface.

## When to Use This CLI

Find Asoview leisure tickets and experiences in Japan, compare terms, inspect dated public stock and fee bands, and hand off a canonical booking URL. Japanese content is the verified first-party language surface.

## Anti-triggers

Purchases, reservations, ticket holds, account/history access, other providers and guaranteed checkout quotes belong to other workflows.

## Operating steps

1. Resolve source IDs with `inventory --kind regions` or `inventory --kind categories`. Exact Japanese names or IDs work in filters.
2. Run `discover` with native region/category/date/party filters. Use `--query` only as a local substring over bounded scanned cards. Inspect `coverage.partial` and `coverage.next_cursor` before interpreting an empty result.
3. Fetch only shortlisted `product` details. Undated prices are advertised bands; lowest ticket price may be a child band. Use `--full` only when descriptions/manuals are needed.
4. Use `availability` for one date or calendar month. Preserve unknown/request-only/closed/sold-out distinctions, source units and timestamps. Date validity and opening hours do not create a reserved slot.
5. Use dated `options` when exact bands matter; select `--slot` from returned source slots when required. `--party` calculates an unconfirmed subtotal with exclusions, not a checkout quote. Preserve product and dated band age labels when they differ.
6. Finish with `handoff`. It derives the canonical URL; current existence is verified by `product`, not handoff alone.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Japan leisure planning
- **`compare`** — Compare a bounded shortlist and preserve missing values.

  _Compare a bounded shortlist and preserve missing values._

  ```bash
  asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent
  ```
- **`discover`** — Filter Japanese names without claiming a global keyword index.

  _Filter Japanese names without claiming a global keyword index._

  ```bash
  asoview-pp-cli discover --region prf130000 --category 192 --query 葛西 --agent
  ```
- **`availability`** — Inspect public stock for a date and quantity.

  _Inspect public stock for a date and quantity._

  ```bash
  asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --agent
  ```
- **`options`** — Read exact date-specific price units and age labels.

  _Read exact date-specific price units and age labels._

  ```bash
  asoview-pp-cli options pln3000044589 --date 2026-10-03 --agent
  ```
- **`inventory`** — Resolve region/category IDs with explicit refresh control.

  _Resolve region/category IDs with explicit refresh control._

  ```bash
  asoview-pp-cli inventory --kind categories --query 水族館 --agent
  ```

## Worked examples

```bash
asoview-pp-cli discover --region prf130000 --category 192 --agent --select meta,results.id,results.name_ja,results.booking_url
asoview-pp-cli product ticket0000049223 --agent
asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --quantity 2 --agent
asoview-pp-cli options ticket0000049223 --date 2026-10-02 --party 2411665:2,2411667:1 --agent
asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent
asoview-pp-cli inventory --kind categories --query 水族館 --refresh-inventory --agent
```

Replace dates with the intended Japan local date. Units, fees, cancellation conditions and limits come from source data; missing facts remain null. `meta.sources` carries freshness and `meta.metrics` carries actual network/cache work. Explicit projection can omit metadata.

## Health and failures

Use `doctor --json`, command help or `agent-context --pretty` for runtime truth. Business stdout is compact JSON and diagnostics are stderr. Input/schema/source failures exit nonzero; empty discovery is a successful bounded scan, not a network failure.

Use `--refresh` for a fresh requested response, `--no-cache` to bypass cache, or `--data-source local` for fresh cache only. `--data-source live` bypasses business caching. Inventory refresh requires `--refresh-inventory`; default inventory reads never fetch the complete taxonomy.

Read README.md for exact limits, price/entry semantics and canonical evidence paths. Completion requires a relevant shortlist, verified requested public facts or explicit unknown coverage, and a canonical URL; no booking action is performed.

# Future published-release reference

The following generator-owned reference applies only after a public registry release exists. This build is unpublished: use the local build instructions above; registry install commands are currently unavailable.

## Prerequisites: Install the CLI

This skill drives the `asoview-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install asoview --cli-only
   ```
2. Verify: `asoview-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/asoview/cmd/asoview-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Bounded first-party discovery, lazy detail and honest date/party coverage. Human handoff only.

## Recipes

### Small shortlist

```bash
asoview-pp-cli discover --region prf130000 --category 192 --agent --select results.id,results.name_ja,results.booking_url
```

Project only handoff fields.

### Dated stock

```bash
asoview-pp-cli availability ticket0000049223 --date 2026-10-02 --agent
```

Stock is a snapshot, no booking.

### Terms comparison

```bash
asoview-pp-cli compare ticket0000049223 ticket0000012233 --agent
```

Compare advertised bands and ticket terms.

## Auth Setup

Selected public GETs need no credentials or paid account.

Run `asoview-pp-cli doctor` to verify setup.
