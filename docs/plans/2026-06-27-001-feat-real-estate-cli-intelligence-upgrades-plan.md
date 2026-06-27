---
title: "feat: Upgrade real-estate CLIs from listing search to durable property intelligence"
type: feat
status: proposed
date: 2026-06-27
target_repo: mvanhorn/printing-press-library
target_clis:
  - library/other/redfin
  - library/commerce/loopnet
  - library/other/apartments
---

# feat: Upgrade real-estate CLIs from listing search to durable property intelligence

## Summary

`redfin`, `loopnet`, and `apartments` are useful seeds, but the current catalog shape still reads like listing search plus local snapshots. The next useful version should make each CLI remember what the site forgets: price cuts, days-on-market drift, removed listings, recurring brokers, recurring owners, neighborhood supply, and normalized comp packs.

This plan proposes an upgrade pass for the existing real-estate CLIs before adding more narrowly scoped property CLIs. The north star is an agent-friendly property intelligence layer: every search produces a durable local store, every rerun produces a diff, and every candidate property can be explained with comparable listings, ownership hints, and source caveats.

## Problem Frame

The library already has:

- `redfin`: homes-for-sale search, saved-search diff, $/sqft net-HOA ranking, sold comps, multi-region trends.
- `loopnet`: commercial listing snapshots, price-cut, days-on-market, supply trends.
- `apartments`: Apartments.com listing search, saved-search diff, $/sqft ranking, price drops, phantom listings.

Those are valuable, but they remain site-shaped. A user evaluating property wants entity-shaped outputs:

- What changed since the last run?
- Which assets are genuinely comparable?
- Which listings disappeared or came back?
- Which broker/owner keeps showing up?
- Which markets have supply/demand drift?
- What fields are observed facts versus inferred or missing?

## Requirements Trace

- R1. Each CLI should support `sync`, `diff`, and `watch` over saved searches with stable listing IDs and source URLs.
- R2. Each CLI should maintain a local SQLite store with listing snapshots, price history, status history, location, broker/contact fields when available, and normalized facts.
- R3. Each CLI should expose a `comps` command that produces a bounded comparable set with transparent criteria: geography, asset type, size, price, recency, and source.
- R4. Each CLI should expose a `supply` or `market` command that summarizes active count, new listings, removals, price cuts, median asking price, and days-on-market by geography.
- R5. Each CLI should emit caveats for source limitations, bot blocks, missing fields, inferred facts, and stale snapshots.
- R6. Shared fields should converge across the three CLIs so downstream tools can join results without bespoke adapters.
- R7. The upgrade should preserve current command names where possible and add new workflow commands rather than breaking existing invocations.

## Scope Boundaries

In scope:

- Local stores and diffable snapshots for `redfin`, `loopnet`, and `apartments`.
- Shared listing schema guidance across the three CLIs.
- CLI-level commands for saved-search diffs, comps, market summaries, and watchlists.
- README and SKILL updates documenting caveats and source limits.

Out of scope:

- Paid-data integrations such as CoStar, Reonomy, Trepp, CompStak, or Crexi unless separately generated as their own CLIs.
- Claiming source data is authoritative when it is only scraped or reverse-engineered.
- Automated outreach to brokers or listing contacts.
- Cross-site entity resolution beyond conservative heuristics; ambiguous matches should remain unresolved.

## Proposed CLI Shape

### `redfin`

```bash
redfin-pp-cli sync --search saved:austin-mf
redfin-pp-cli diff --search saved:austin-mf --since 7d
redfin-pp-cli comps --listing <listing-id> --radius 2mi --sold-since 18mo
redfin-pp-cli market --geo "Austin, TX" --asset residential --window 30d
```

### `loopnet`

```bash
loopnet-pp-cli sync --search saved:phoenix-industrial
loopnet-pp-cli diff --search saved:phoenix-industrial --since 14d
loopnet-pp-cli comps --listing <listing-id> --asset industrial --radius 5mi
loopnet-pp-cli market --geo "Phoenix, AZ" --asset industrial --window 90d
loopnet-pp-cli brokers repeat --geo "Phoenix, AZ" --asset industrial
```

### `apartments`

```bash
apartments-pp-cli sync --search saved:brooklyn-rentals
apartments-pp-cli diff --search saved:brooklyn-rentals --since 7d
apartments-pp-cli comps --listing <listing-id> --radius 1mi --beds 2
apartments-pp-cli market --geo "Brooklyn, NY" --beds 2 --window 30d
```

## Shared Schema Direction

Each CLI should normalize toward these core tables:

- `listings`: source, source_listing_id, canonical_url, title, asset_type, status, geography, address fields when available.
- `listing_snapshots`: observed_at, asking_price, rent, square_feet, beds, baths, lot_size, cap_rate, NOI fields when available.
- `listing_contacts`: broker, brokerage, phone/email when public, contact role.
- `listing_events`: first_seen, price_cut, removed, relisted, status_change.
- `saved_searches`: source query, normalized geography, filters, created_at.
- `source_caveats`: missing fields, auth/cookie requirement, bot challenge, stale result, inferred value.

## Implementation Units

- [ ] Unit 1: Audit current schemas and commands across `redfin`, `loopnet`, and `apartments`.
- [ ] Unit 2: Define a shared real-estate listing schema and migration pattern.
- [ ] Unit 3: Add or repair `sync` and `diff` behavior for each CLI.
- [ ] Unit 4: Add `comps` workflows with explicit criteria and caveat output.
- [ ] Unit 5: Add `market`/`supply` summaries by geography and asset type.
- [ ] Unit 6: Add repeat-broker/contact summaries where the source exposes public contact fields.
- [ ] Unit 7: Update README/SKILL docs with examples, caveats, and source limitations.
- [ ] Unit 8: Run live dogfood against at least one saved search per CLI and archive proof artifacts.

## Validation

- `go test ./...` from each touched CLI root.
- `go vet ./...` from each touched CLI root.
- `go build ./...` from each touched CLI root.
- Live dogfood proof for at least one saved search per CLI.
- `git diff --check`.

## Why This Belongs In The Library

The catalog already has three real-estate CLIs. Improving them into durable property intelligence is higher leverage than adding another shallow listing scraper. Once the shared schema and diff/comps pattern exist, new CRE-specific CLIs can plug into the same downstream shape.
