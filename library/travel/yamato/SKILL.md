---
name: pp-yamato
description: "Plan Japan luggage forwarding using Yamato public route tariffs, estimated dates, airport shipping deadlines, parcel sizing, counter evidence and limited same-day schedules. Use for TA-Q-BIN planning or when asked to run yamato."
author: zjsng
license: Apache-2.0
argument-hint: "<command> [flags]"
allowed-tools: "Read Bash"
---

# Yamato luggage planning

Use the local `yamato-pp-cli` for public Japan luggage planning. Verify `yamato-pp-cli --version`; if unavailable, build/install from the provided source directory following `README.md`. Public planning uses HTTP and needs no login, API key or browser.

## Local build status

This CLI is currently local and has not been published to the catalog. Build/install from the supplied source directory using README.md. The canonical catalog install instructions below apply after a separately authorized publication.

## Prerequisites: Install the CLI

This skill drives the `yamato-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install yamato --cli-only
   ```
2. Verify: `yamato-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/yamato/cmd/yamato-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Measure luggage, compare distinct TA-Q-BIN products, and query current public rates and estimated delivery dates. Airport terminal and limited same-day evidence preserve cutoff and acceptance uncertainty.

## When to use

Answer route-rate, source calendar deadline, measured luggage category, airport terminal, counter or product-eligibility questions. Real shipments, payment, private tracking and account changes are outside this CLI's scope.

## Workflow

1. Measure packed dimensions in cm and weight in kg. Run `parcel`; retain both categories, the greater chargeable size, rejection reasons and unknown acceptance.
2. Choose the distinct product. Standard `takkyubin` uses postal origin/destination. `airport` and `airport-roundtrip` require an exact live terminal ID from `airports`. Lodging `roundtrip` uses postal origin/destination.
3. Use a real current/future JST date and its correct meaning: `ship`, standard `delivery`, airport `boarding`, or lodging `use`. Query `quote` with the chargeable size; use `--detail` only when a full tariff comparison is needed.
4. Inspect `products`, `counters` or `same-day` for eligibility/cutoff evidence. Retain source timestamps and URLs, rate basis/fees and explicit unknowns in the answer.
5. Stop when the required records and source evidence are present. If the source fails or a PDF/image changes, use the stated public handoff URL and preserve the gap.

## Recipes

### parcel

```bash
yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23 --agent
```

Classify both size and weight and preserve acceptance unknowns.

### quote

```bash
yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --size 160 --agent
```

Use a current/future JST date and correct date-kind. Read current source tariffs and an estimated arrival or shipping deadline; acceptance remains unknown.

### airports

```bash
yamato-pp-cli airports --query 成田 --agent --select results
```

Use live upstream airport codes and Japanese terminal names.

### counters

```bash
yamato-pp-cli counters --query Narita --agent
```

Inspect public send/pickup/floor evidence and fingerprint-checked image hours.

### same-day

```bash
yamato-pp-cli same-day --query Narita --agent
```

Inspect limited-area cutoff schedules and separate example prices.

### products

```bash
yamato-pp-cli products --service airport --agent
```

Review live service policy evidence for standard, airport, roundtrip and same-day products.

## Output and decisions

Use `--agent` or `--json`; planning results are `{meta,results}` with source timestamps, URLs, response bytes, SHA-256, request count and latency. `--select results.items` narrows a list, and `--limit`/`--offset` page airport/counter matches. Empty matches are `[]`. `--timeout` bounds the whole command. Optional-source errors are explicit in `meta.fetch_failures`; throttles exit 7. Use `--dry-run --json` for a source-free preview.

Rates are tax-inclusive list tariffs per parcel. Relevant source tables already include outbound airport 660 JPY and roundtrip reductions; do not deduct them again. Conditional member/dropoff/digital discounts, packaging and optional fees require their source conditions. Confirm Kansai return airport charges.

Dates are estimated, not guaranteed. A shipping deadline date has an unknown counter dispatch time. Closing hours do not establish dispatch cutoff. Confirm hotel reception, parcel contents/packaging, counter acceptance, holidays, disruptions and islands before treating a plan as feasible.

The same-day command covers nine selected Narita/Haneda airport-to-hotel rows; price examples are separate, and all other routes/directions use the full source PDF/directory. Current PDF/image hashes must match verified snapshots for schedules or Narita Terminal 2 hours to be returned. Other image-only hours remain unknown.

`parcel` is a dated rule snapshot; live planning commands reject `--data-source local`. `--no-learn` disables optional local learning for deterministic workflows. Consult `README.md` for build, MCP, source limits and evidence.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Luggage planning
- **`parcel`** — Classify both size and weight and preserve acceptance unknowns.

  _Classify both size and weight and preserve acceptance unknowns._

  ```bash
  yamato-pp-cli parcel --length 70 --width 45 --height 30 --weight 23 --agent
  ```
- **`quote`** — Fetch current source rates and date-specific arrival or shipping deadline.

  _Fetch current source rates and date-specific arrival or shipping deadline._

  ```bash
  yamato-pp-cli quote --origin 1000005 --destination 6008216 --date 2026-10-08 --size 160 --agent
  ```
- **`airports`** — Use live upstream airport codes and Japanese terminal names.

  _Use live upstream airport codes and Japanese terminal names._

  ```bash
  yamato-pp-cli airports --query 成田 --agent
  ```
- **`counters`** — Inspect public send/pickup/floor evidence and fingerprint-checked image hours.

  _Inspect public send/pickup/floor evidence and fingerprint-checked image hours._

  ```bash
  yamato-pp-cli counters --query Narita --agent
  ```
- **`same-day`** — Inspect limited-area cutoff schedules and separate example prices.

  _Inspect limited-area cutoff schedules and separate example prices._

  ```bash
  yamato-pp-cli same-day --query Narita --agent
  ```
- **`products`** — Review live service policy evidence for standard, airport, roundtrip and same-day products.

  _Review live service policy evidence for standard, airport, roundtrip and same-day products._

  ```bash
  yamato-pp-cli products --service airport --agent
  ```

## Auth Setup

No login or API key is required for supported public planning commands.

Run `yamato-pp-cli doctor` to verify setup.
