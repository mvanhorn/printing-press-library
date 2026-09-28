---
name: pp-jalan
description: "Discover Japan accommodation on Jalan, inspect ryokan room and plan terms, or compare a few stay dates. Use for Jalan hotel search, onsen/bath evidence, meals, prices, cancellation terms, and booking handoff URLs."
allowed-tools: "Read Bash"
---

# Jalan accommodation

## Source checkout setup

When running from a source checkout, build locally. Build with `go build -o bin/jalan-pp-cli ./cmd/jalan-pp-cli`, then invoke `./bin/jalan-pp-cli` from that directory. Examples below use `jalan-pp-cli` when the local binary directory is already on PATH. No global installation or configuration is required.

## When to Use This CLI

Use Jalan accommodation data to shortlist stays, inspect exact room/plan evidence, or compare bounded alternatives. Start with `stay capabilities` and command help for current bounds and flags.

## Anti-triggers

Use another workflow for bookings, payments, reservation changes/cancellations, coupon claims, account rewards, other providers, or arbitrary Japanese-to-English translation.

## Workflow

1. Resolve a destination using `stay locations`. Preserve its exact source area and Japanese name; aliases may describe a region larger than one town.
2. Run `stay search` with dates and per-room party. Keep elementary children separate from infant meal/bedding categories. Use identical occupancy per room.
3. Fetch `stay property` for access, amenities and review categories, then `stay offers` for dated room/plan combinations.
4. Fetch `stay plan` for each serious candidate. Keep property ID, plan ID and room ID distinct; leading zeroes matter.
5. Compare equal-party alternatives with `stay compare`. Return canonical Jalan URLs for booking handoff.

Done when candidates have source-linked identity, dated occupancy, known terms and explicit unknowns. A list of property names alone does not establish room or plan suitability.

## Recipes

### Bounded shortlist

```bash
jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 3 --agent --select id,name_ja,url,price
```

Find a small dated shortlist while retaining source and coverage metadata.

### Exact offer terms

```bash
jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2
```

Inspect this plan/room pair with Japanese source evidence and explicit price scope.

### Date alternatives

```bash
jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3
```

Compare only the bounded offers retrieved for the same party. Use `--select check_in,results.property_id,results.plan_id,results.room_id,results.price` to retain nested offer identities and complete price units in a compact comparison. Alternative query, freshness, coverage and failures remain available.

## Agent Mode

Accommodation commands emit compact JSON by default. `--agent --select` can narrow result fields while retaining shared metadata. Read `.meta` freshness/query, `.pagination` coverage, and `.fetch_failures` before using `.results`. Evidence `source_ref` values resolve through `.meta.sources`.

```bash
jalan-pp-cli stay search --destination Hakone --check-in 2026-11-10 --adults 2 --limit 3 --agent --select id,name_ja,url,price
jalan-pp-cli stay property 371898
jalan-pp-cli stay offers 385995 --check-in 2026-11-10 --adults 2 --limit 3
jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2
jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 3
```

Use future dates. Prices and inventory in examples are never guaranteed. Inspect current help for child-category flags and supported preference filters.

## Interpretation

- **Baths:** property facilities and room facilities have separate scopes. Private use, advance reservability, outdoor location and hot-spring water are independent. Preserve negative evidence such as room baths that are explicitly not hot springs.
- **Prices:** preserve source units. Base quotes, conditional coupon reductions and earned points are different facts. Extra fee text can leave the final payable amount unknown.
- **Terms:** retain Japanese meals, check-in restrictions, cancellation bands and child policies with evidence. Missing facts are unknown.
- **Coverage:** no matches, unavailable offers, unsupported inputs, partial results and access errors require different conclusions. Comparisons cover only the reported fetched subset.
- **Freshness:** dates/times use Asia/Tokyo. Inventory is live by default; `--max-age` opts into observation reuse and `--refresh` fetches again. Cache timestamps describe the original observation. Explicit cache reuse gives repeatable slices within one source page; fresh calls and separate native pages are not an atomic snapshot.

## Unique Capabilities

### Accommodation decisions
- **`stay plan`** — Inspect separate bath facts without inferring room facilities from property amenities.


  ```bash
  jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2 --agent
  ```
- **`stay plan`** — Separate quoted cash price, conditional coupons, earned points and extra fees.


  ```bash
  jalan-pp-cli stay plan 385995 --plan-id 03912759 --room-id 0576806 --check-in 2026-11-10 --adults 2 --agent
  ```
- **`stay compare`** — Compare a few dates or exact plans under equal party conditions.


  ```bash
  jalan-pp-cli stay compare 385995 --dates 2026-11-10,2026-11-11 --adults 2 --limit 2 --agent
  ```
- **`stay property`** — Preserve Japanese facts, explicit unknowns and source evidence.


  ```bash
  jalan-pp-cli stay property 371898 --agent
  ```
- **`stay locations`** — Resolve supported Japanese and English destination aliases explicitly.


  ```bash
  jalan-pp-cli stay locations --query Hakone --agent
  ```

## Auth Setup

Public accommodation commands need no credentials or paid service. Legacy API registration is closed and the credential-gated API is excluded.

Run `jalan-pp-cli doctor` to verify setup.

## Direct Use

Use `jalan-pp-cli stay --help` for command discovery. Diagnostics are on stderr; stdout is predictable structured data. For verification status and source limitations, read [docs/verification.md](docs/verification.md). For build and worked examples, read [README.md](README.md).

## Catalog installation

Use the installer below when Jalan appears in the published library catalog. Before the publication PR merges, use the source checkout setup above.

## Prerequisites: Install the CLI

This skill drives the `jalan-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install jalan --cli-only
   ```
2. Verify: `jalan-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jalan/cmd/jalan-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only public accommodation discovery with source evidence, explicit price units and canonical booking links.
