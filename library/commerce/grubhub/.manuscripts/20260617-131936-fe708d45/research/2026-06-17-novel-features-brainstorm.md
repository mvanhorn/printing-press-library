# Grubhub Novel-Features Brainstorm (Phase 1.5c.5 subagent audit trail)

## Customer model

**Dana — the office lunch admin.** Coordinates team lunches for a 12-person downtown team.
- Today: taps restaurant-by-restaurant to check fee/minimum/budget, screenshots into Slack, fields dietary questions one at a time.
- Weekly ritual: picks Tue/Thu lunch spots that deliver to the office, low fee + reasonable minimum, at least one veg/GF option.
- Frustration: fees/minimums/ETAs buried one-tap-deep; no side-by-side; can't ask "which nearby places have a GF entrée under $15."

**Marco — the deal/coupon hunter.** Orders 3-4x/week, refuses full price.
- Today: scrolls promo carousel, opens each restaurant for banner coupons, cross-checks RetailMeNot/DealNews.
- Weekly ritual: sweeps nearby restaurants for the best stacked offer; lets the deal pick the restaurant.
- Frustration: offers scattered per-restaurant; app never answers "show every nearby deal ranked by value."

**Priya — the agent/integration developer.** Building a "what's for dinner" assistant + Slack bots.
- Today: reverse-engineers the GTM API by hand, re-derives anonymous-bearer + client_id dance, parses raw blobs.
- Weekly ritual: wires Grubhub into agents; needs deterministic JSON, stable IDs, cross-restaurant comparisons.
- Frustration: no official API; raw responses don't answer comparative/dish-level questions without a local join.

**Sam — the picky frequent orderer.** Dinner 4-5x/week, same few dishes.
- Today: remembers which 2-3 restaurants carry the dish, taps through menus to confirm + check price.
- Weekly ritual: "Who near me has a poke bowl tonight, under $15, delivering in <40 min?"
- Frustration: no dish-level search across restaurants; app searches restaurant/cuisine names, never menu item names.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | Persona | Buildability proof | Long Description |
|---|---------|---------|-------|--------------|---------|--------------------|------------------|
| 1 | Fee/ETA/minimum comparison board | `compare <address>` | 9/10 | hand-code | Dana | Joins cached restaurant cards in SQLite, multi-key client-side sort over delivery_fee/minimum/ETA/rating/distance — no per-card tapping | none |
| 2 | Offline dish search | `dish <address> <query>` | 8/10 | hand-code | Sam | SQLite FTS over cached menu_item_list names/descriptions across all nearby restaurants — cross-restaurant join no API call answers | Use this to find a specific menu item across all nearby restaurants. Do NOT use it to browse one known restaurant's full menu; use `menu <restaurant-id>` for that. `--diet` is a mechanical keyword match, not a verified dietary certification. |
| 3 | Deal radar | `deals <address>` | 7/10 | hand-code | Marco | Aggregates available_offers/coupons_available/available_promo_codes across cached nearby restaurants, ranks by value — cross-restaurant rollup, not a per-card field read | Use this for a ranked cross-restaurant view of who is running a deal right now. Do NOT use it to read offers on a single restaurant; absorbed `search` surfaces per-restaurant offers inline. |
| 4 | Best-value picker | `pick <address>` | 6/10 | hand-code | Marco/Dana | Transparent normalized score over cached fee/rating/active-offer/ETA; returns top pick with breakdown — deterministic arithmetic, no LLM | Use this for one recommended restaurant from a transparent fee/rating/deal/ETA score. Do NOT use it for the full ranked table; use `compare`. |

(`sync <address>` was a 5th survivor but maps to the framework sync command — spec-emits, not a hand-code novel row.)

## Killed candidates
| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| price-index | Redundant — `dish` already returns per-item prices; min/med/max is a flag | `dish` |
| standalone --diet command | Reframe leaves only low-precision keyword match; survives as a flag on `dish` | `dish --diet` |
| compare budget flags as own command | Filter flags, folded into `compare` | `compare` |
| promos | Thin projection of available_promo_codes that `deals` aggregates | `deals` |
| cuisines map | Low pain, not in any persona's weekly ritual | `search --cuisine` |
| chain-vs-independent | No reliable chain signal in spec; needs external dataset | `compare` |
| menu --popular | Flag on absorbed `menu`, not novel | `menu` |
| open-now/availability | Availability richness unconfirmed; filter flag at best | `compare` |
| compare --json | `--json` is a global framework flag | all survivors |
