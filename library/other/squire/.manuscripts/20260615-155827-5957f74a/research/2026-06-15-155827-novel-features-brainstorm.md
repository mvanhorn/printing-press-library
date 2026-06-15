# Squire Novel-Features Brainstorm (audit trail)

## Customer model
- **Marcus — soonest-cut hunter**: opens 4-5 shop pages, eyeballs each barber's availability; wants the earliest open slot across saved neighborhood shops; site shows one shop at a time, no cross-shop availability.
- **Priya — price-conscious comparison shopper**: compares "Haircut" prices across the neighborhood page-by-page; no sort/filter/compare; prices change quietly.
- **Dwayne — shop owner auditing own listing (B2B)**: weekly competitive glance on price/rating/staff vs rivals; no side-by-side, no change detection.
- **Renata — relocator**: new city, opens shop after shop cold; wants top shops ranked by rating × volume with AI summary in one view.

## Survivors (transcendence)
1. soonest (8/10, hand-code, live) — soonest-available barber across shops; sorts live nextAvailableTime ISO ascending. Killed sibling: availability-grid (single-shop subset).
2. compare (8/10, hand-code, local) — cross-shop price/rating/staff stacked table; SQLite join across services/reviews/shops. Killed sibling: contact (field projection).
3. cheapest (7/10, hand-code, local) — cheapest service-by-category across an area; FTS + sort on cost cents; low end of costRange flagged. Killed sibling: menu (absorbed endpoint+sort).
4. watch (7/10, hand-code, local) — price/staff drift snapshot diff for one shop; cents-level price moves, barber add/remove, rating change. Killed sibling: reviews-trend.
5. roster (6/10, hand-code, local) — city shop ranking by rating × log(numberOfRatings), AI summary passed through verbatim. Killed sibling: summary (multi-shop AI summary).

## Internal helper (not a headline feature)
- resolve (slug→UUID) — foundational plumbing every command uses; details response carries id; ships internal, not a novel feature.

## Killed candidates
- menu → absorbed by generated get_service + sort (→ cheapest)
- availability-grid → single-shop subset (→ soonest)
- reviews-trend → only current rating exposed; folds into watch
- summary → single-shop absorbed; multi-shop = roster column
- contact → field projection via --select (→ compare)
- value (rating-per-dollar) → 5/10 line cut; blended axis duplicates cheapest+roster, distrusted weighting
- resolve → demoted to internal helper
