---
title: "feat: Add CRE-native records CLI candidates for ownership, permits, zoning, and filings"
type: feat
status: proposed
date: 2026-06-27
target_repo: mvanhorn/printing-press-library
candidate_clis:
  - county-records
  - acris
  - building-permits
  - zoning-atlas
  - cre-filings
---

# feat: Add CRE-native records CLI candidates for ownership, permits, zoning, and filings

## Summary

The catalog has useful real-estate listing CLIs, but it is missing the records layer that makes property work serious: deeds, mortgages, owner history, permits, violations, zoning, assessments, tax bills, and public-company filing context.

This plan proposes a batch of CRE-native CLI candidates. They should be generated as separate CLIs through the Printing Press rather than bolted into `redfin`, `loopnet`, or `apartments`.

## Problem Frame

Listing sites answer "what is advertised right now?" They do not reliably answer:

- Who owns this property?
- When did it last trade?
- What debt appears to be attached?
- What permits, violations, certificates, or zoning constraints matter?
- Which public-company filings mention the asset, parent, tenant, REIT, or lender?
- What changed across public records since last month?

Those are different data domains with different source contracts. Treating them as separate CLIs keeps source limitations honest and lets downstream tools compose them.

## Proposed Candidate CLIs

### 1. `county-records`

**Purpose:** county recorder / assessor search for deeds, mortgages, liens, parcel IDs, owners, assessed values, tax history, and transfer history.

**Candidate commands:**

```bash
county-records-pp-cli search --address "123 Main St, Austin, TX"
county-records-pp-cli parcel get --apn <apn>
county-records-pp-cli owner history --apn <apn>
county-records-pp-cli transfers --owner "Example LLC" --since 2020-01-01
county-records-pp-cli tax history --apn <apn>
county-records-pp-cli diff --watchlist portfolio.csv --since 30d
```

**Notes:** start with one or two jurisdictions with accessible public portals, then add adapters. Avoid pretending all counties share one schema.

### 2. `acris`

**Purpose:** New York City ACRIS-focused CLI for deeds, mortgages, assignments, satisfactions, parties, borough/block/lot, and document images.

**Candidate commands:**

```bash
acris-pp-cli search --address "11 Madison Ave"
acris-pp-cli bbl resolve --address "11 Madison Ave"
acris-pp-cli documents --bbl <bbl> --since 2015-01-01
acris-pp-cli debt history --bbl <bbl>
acris-pp-cli parties search --name "Example LLC"
acris-pp-cli watch --bbl-file portfolio-bbls.txt
```

**Notes:** ACRIS is a good first CRE records target because the domain is narrow and high-value.

### 3. `building-permits`

**Purpose:** municipal permits, inspections, violations, certificate-of-occupancy records, construction status, and alteration history.

**Candidate commands:**

```bash
building-permits-pp-cli search --address "123 Main St"
building-permits-pp-cli violations --address "123 Main St"
building-permits-pp-cli permits --address "123 Main St" --since 2020-01-01
building-permits-pp-cli activity --owner "Example LLC"
building-permits-pp-cli diff --watchlist portfolio.csv --since 30d
```

**Notes:** generate jurisdiction-specific adapters rather than a fake universal API.

### 4. `zoning-atlas`

**Purpose:** zoning, overlays, allowed uses, FAR, height limits, parking requirements, parcel constraints, and rezoning events when public data exists.

**Candidate commands:**

```bash
zoning-atlas-pp-cli lookup --address "123 Main St"
zoning-atlas-pp-cli parcel constraints --apn <apn>
zoning-atlas-pp-cli compare --apn <a> --apn <b>
zoning-atlas-pp-cli changes --geo "Austin, TX" --since 2024-01-01
```

**Notes:** this should be caveat-heavy. Zoning data is legal-risky; the CLI should cite source URLs and warn when human/legal review is required.

### 5. `cre-filings`

**Purpose:** REIT, tenant, lender, and public-company filing context connected to properties and markets.

**Candidate commands:**

```bash
cre-filings-pp-cli asset mentions --query "11 Madison Ave"
cre-filings-pp-cli reit portfolio --ticker VNO
cre-filings-pp-cli tenant risk --tenant "Example Corp"
cre-filings-pp-cli market exposure --ticker <ticker> --market "NYC"
cre-filings-pp-cli debt maturities --ticker <ticker>
```

**Notes:** this should build on `sec-edgar` / `edgar`, not duplicate their raw filing retrieval. The value is CRE-specific extraction and joins.

## Shared Output Contract

All CRE-native CLIs should emit:

- Source URL and observed timestamp for every record.
- Jurisdiction/source caveats.
- Stable entity keys when available: APN, BBL, document ID, permit ID, CIK, ticker.
- Machine-readable JSON with confidence fields for address/entity resolution.
- `diff` commands for watchlists, because change detection is more valuable than one-off lookup.

## Implementation Units

- [ ] Unit 1: Pick first jurisdiction/source for `acris` and generate via `/printing-press`.
- [ ] Unit 2: Generate `building-permits` for one public city portal with clear caveats.
- [ ] Unit 3: Add `cre-filings` as an extraction layer over SEC/EDGAR records.
- [ ] Unit 4: Define shared JSON fields for CRE records and watchlist diffs.
- [ ] Unit 5: Dogfood each candidate against a small fixture watchlist.
- [ ] Unit 6: Publish each CLI independently through `/printing-press-publish`.

## Validation

For each generated CLI:

- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `--help` and `--version`
- Live or fixture-backed dogfood proof
- Manuscripts with source caveats and validation evidence

## Why This Belongs In The Library

The published catalog is strongest where it turns web/API surfaces into durable agent-native workflows. CRE records are exactly that: fragmented public sources, repetitive lookup, high-value diffs, and strong need for explicit provenance.
