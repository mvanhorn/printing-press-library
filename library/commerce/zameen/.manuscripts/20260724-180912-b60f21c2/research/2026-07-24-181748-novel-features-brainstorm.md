# Zameen Novel-Features Brainstorm (audit trail)

## Customer model

**Persona A — "The DHA/Bahria plot-file flipper" (Lahore/Islamabad investor).**
- Today: Refreshes `Lahore_DHA_Defence` and `Bahria` Plots pages every morning in a browser, eyeballing price-per-Marla to spot a file listed under the society's going rate before another dealer grabs it.
- Weekly ritual: Manually re-scans the same 4-5 society URLs daily; keeps a mental/Excel note of "what a 10-Marla in DHA Phase 6 should cost."
- Frustration: No alerts, no way to know which listing is *new* today vs yesterday, and no way to rank files by price-per-Marla against the area median.

**Persona B — "The Karachi 3-bed family renter on a budget."**
- Today: Runs the same Rentals search for 3-bed apartments under a rent ceiling, sorts by newest, checks whether anything dropped into budget.
- Weekly ritual: Re-runs one saved filter set every few days; wants to catch price drops on units she already saw.
- Frustration: Filters aren't sticky across sessions; can't tell which listings are genuinely new or which dropped price.

**Persona C — "The agent doing weekly market comps for a client" (Karachi/Lahore broker).**
- Today: Assembles a comps sheet by hand — median asking price and inventory count for an area — eyeballs which agencies dominate a society's supply.
- Weekly ritual: Pulls area-level price stats and agency inventory before a client pitch; exports to CSV.
- Frustration: Community scrapers dump CSV but none give area medians, price-per-Marla, or agency rollups; days-on-market is invisible.

**Persona D — "The PK-property data analyst."**
- Weekly ritual: Bulk-exports result sets, joins listings to the location hierarchy for area rollups.
- Frustration: Marla/Kanal units and crore/lakh idioms make raw fields hard to normalize; no local store to diff runs over time.

## Survivors (transcendence)

| # | Feature | Command | Persona | Score | Buildability |
|---|---------|---------|---------|-------|--------------|
| 1 | Saved-search cross-run diff (new + price drops) | `watch <name>` | A,B | 10/10 | hand-code |
| 2 | Area price research (median, price-per-Marla, inventory) | `comps --city --area` | C,D | 9/10 | hand-code |
| 3 | Below-market plot-file detector | `deals --city --area --type Plots` | A | 8/10 | hand-code |
| 4 | Days-on-market / stale inventory | `stale --city --days` | A,C | 7/10 | hand-code |
| 5 | Agency inventory leaderboard | `agencies --city` | C | 6/10 | hand-code |

## Killed candidates
| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| save | Plumbing subsumed by `watch` snapshot store | watch |
| new | Strict subset of `watch` (added-only) | watch |
| drops | Strict subset of `watch` (price-drop-only) | watch |
| relisted | Harder-to-verify days-on-market sibling of `stale` | stale |
| verified | Filter flag over absorbed search (wrapper) | search |
| hot | Wrapper over ad-tier field; surfaces paid boosting | search |
| files | Installments filter flag over search (wrapper) | deals |
| newprojects | Separate secondary surface; scope creep | agencies |
| convert | Pure Marla/Kanal calculator; folded into deals/comps | comps |
