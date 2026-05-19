# Tennis Warehouse novel features brainstorm

## Customer model

**Persona 1 — Marco, the spec-driven club player (4.0 NTRP).**
- Today: keeps 4 browser tabs open on Tennis Warehouse (all-racquets index, Wilson catalog, Babolat catalog, used by brand), copies head size / strung weight / swingweight / string pattern into a Google Sheet. Cannot answer "every current racquet between 320-330 swingweight, 16x19, under 11.5oz" without dozens of detail-page clicks.
- Weekly ritual: Sunday-evening scan of new arrivals in used inventory across Wilson, Babolat, Yonex; cross-references specs against current racquet.
- Frustration: spec triangulation locked behind individual detail pages.

**Persona 2 — Lena, the bargain-hunting used-buyer.**
- Today: refreshes `/usedcatpage.html?ccode=WILSONRACS` etc., eyeballs prices. No price history, no concept of "new since last check," no alerts.
- Weekly ritual: multiple brand-scoped used-page refreshes hunting drops or new arrivals across a ~5-model watchlist.
- Frustration: every visit is from-scratch eyeball scan; good Grade A units sell in hours.

**Persona 3 — Kai, replacing a discontinued racquet.**
- Today: Wilson Blade 98 v7 cracking, Wilson now sells v10. Opens 5 tabs (v7 used, v10 new, Pro Staff 97, Yonex VCORE 98, Babolat Pure Strike 98), mentally diffs specs.
- Weekly ritual: once-per-cracked-frame intense multi-day spec-comparison ritual.
- Frustration: no substitute-finder — 5 racquets × 9 spec dimensions in browser tabs is what makes him put off replacement decisions.

## Candidates (pre-cut)

(See the agent's full Pass 2 list. Survivors and kills below.)

## Survivors and kills

### Survivors (transcendence rows for the manifest)

| # | Feature | Command | Score | Buildability |
|---|---------|---------|-------|--------------|
| 1 | Substitute finder by spec similarity | `racquets similar <sku>` | 9/10 | hand-code |
| 2 | Side-by-side spec compare | `racquets compare <sku> <sku> [<sku>...]` | 8/10 | hand-code |
| 3 | Used-vs-new value gap | `used deals --min-discount-pct 30 --grade A` | 8/10 | hand-code |
| 4 | Price drop tracking | `used drops --since 7d --min-drop-pct 10` | 8/10 | hand-code |
| 5 | New-arrival feed | `used new --since 7d` | 7/10 | hand-code |
| 6 | Inventory depth by model | `used depth --min-units 3 --grade A` | 7/10 | hand-code |
| 7 | Watchlist + drop integration | `used watch <pcode>` / `used watchlist [drops]` | 6/10 | hand-code |
| 8 | Grip-size availability | `used grip-availability --size 4_3/8` | 5/10 | hand-code |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Multi-dim spec filter on used | Overlaps absorbed list+filter; belongs as flags on `used list` | Used-vs-new value gap |
| Successor / generation chain | Lineage extraction depends on brittle model regex; persona pain better served by spec similarity | Substitute finder |
| Demo eligibility filter | Thin flag-level enhancement over `racquets list --demo-eligible`; not transcendent | Substitute finder |
| Closeout / clearance scan | Belongs as `--status closeout` flag on `racquets list` | Used-vs-new value gap |
| Spec band-pass query | Belongs as range syntax on `--head-size 97-99` flag of `list` | Side-by-side spec compare |
| Demand-signal trend | No persona; speculative; needs 14d of sync history | New-arrival feed |
