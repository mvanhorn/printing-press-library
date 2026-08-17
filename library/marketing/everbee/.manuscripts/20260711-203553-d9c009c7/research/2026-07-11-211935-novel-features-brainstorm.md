# EverBee Novel-Features Brainstorm (Phase 1.5c.5 audit trail)

Subagent run 2026-07-11. Prior research: none (first-print semantics; Pass 2(d) did not fire).
Binding user decision carried into the brainstorm: evidence layer is **annotate-only, never drop**
(`--min-relevance` is a caller-side filter).

## Customer model

**Maya — POD apparel seller** (2-person shop, Printify-fulfilled tees/sweatshirts).
- Today: pays for EverBee Growth, click-drives Keyword Research + Product Analytics, screenshots tables into Notion; types "dad shirt" variants one at a time and eyeballs whether rows match.
- Weekly ritual: Sunday-night niche hunt — 10-15 seed searches, sort by est_mo_revenue, manually discard digital/SVG rows polluting apparel research, pick two designs.
- Frustration: the UI's optimism. "Low competition" has no documented threshold, results mix physical and digital, and there's no way to run 15 seeds as one comparable batch.

**Derek — digital-download seller** (SVG/PNG bundles, clipart).
- Today: EverBee + eRank free tier, exports tables, hand-filters `listing_type` in spreadsheets.
- Weekly ritual: before designing the next bundle, checks whether sub-niches are saturating and whether demand is seasonal (Father's Day spike) or evergreen — by squinting at trend sparklines.
- Frustration: no product-type constraint anywhere, no mechanical seasonal-vs-durable split, so he ships bundles into dead post-holiday niches.

**Priya — niche researcher for multiple seller clients.**
- Today: re-runs the same searches monthly per client, hand-diffs spreadsheets; evidence trail is screenshots.
- Weekly ritual: Monday batch across every client's niche portfolio — re-pull, compare to last run, flag movers, write the memo.
- Frustration: zero persistence, zero provenance. When a client asks "why low competition?", she has a screenshot, not thresholds, evidence counts, or a fetch timestamp.

**Scout — AI research agent** (the #1492 batch runner).
- Today: drives the published CLI and gets confidence 1.0 on default-feed garbage ("????gift", African masks) because transport success was mistaken for evidence.
- Ritual: daily batch sub-niche scans (`--parent dad --product t-shirt`), JSON consumed downstream.
- Frustration: no machine-readable way to know the data path is semantically valid; confidence untethered from evidence; plan-cap errors surfacing as empty results.

## Candidates (pre-cut) — 13 generated

C1 research niche · C2 research subniches · C3 research competitors · C4 research tags ·
C5 research drift · C6 research listing · C7 selftest · C8 discover (raw safe-GET) ·
C9 quota · C10 research priceband · C11 shops listings · C12 keywords lowcomp · C13 research seasonal

Kill/keep checks applied inline: no candidate needs an LLM (relevance is token-overlap math), an
external service, absent auth, or reimplementation of an upstream aggregation (EverBee ships none —
all synthesis here is genuinely client-side, exactly as the UI's own Tag Analytics tab does it).

## Survivors (7) — see absorb manifest transcendence table

All 7 tagged `hand-code`. Scores: niche 10, subniches 10, competitors 9, tags 9, drift 8, selftest 8, listing 6.

## Killed candidates (6)

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C8 discover (raw safe-GET) | Run at most once per reprint by a maintainer, not weekly by any persona — press tooling, not a shipped command. | `selftest` |
| C9 quota | Duplicate of absorbed row #9 (`doctor`/account already reports plan + usage). | `selftest` |
| C10 research priceband | Redundant: F8 makes price bands a field of every niche/sub-niche result, not a destination command. | `research niche` |
| C11 shops listings | Coverage of a competitor's listings in the synced slice is accidental, so results silently mislead — fails the honesty bar the reprint exists to raise. | `research competitors` |
| C12 keywords lowcomp | A drop-rows command contradicts the binding annotate-only decision; correct shape is documented-threshold annotation + caller-side filter flags. | `research niche` |
| C13 research seasonal | Single-metric command too thin alone; shipped as the seasonality flag inside tag consensus. | `research tags` |
