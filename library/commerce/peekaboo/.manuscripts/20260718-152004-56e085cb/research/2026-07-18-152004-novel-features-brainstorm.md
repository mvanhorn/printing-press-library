# Peekaboo Novel Features Brainstorm (subagent audit trail)

## Customer model
- Faisal — Karachi weekend deal-hunter. Ritual: Thursday sweep of a city's Food deals for the biggest valid discount. Frustration: no way to rank a whole city's deals by % or see which expire soon.
- Ayesha — multi-card discount maximizer (HBL + Meezan). Ritual: which of my cards gives the best discount here; plan outings around a card. Frustration: site only goes merchant->cards, never card->merchants.
- Bilal — navigator routing to a chain's nearest branch. Ritual: find chain, nearest branch, get directions; bulk-collect coordinates. Frustration: must click each branch's "Direction" button; no bulk coordinate/Maps-URL export.

## Survivors (6, all >= 6/10)
1. directions (10) hand-code — Maps directions URL per branch
2. nearest (9) hand-code — closest branch by haversine to city/coords
3. wallet (10) hand-code — card->merchants reverse index
4. top-deals (10) hand-code — rank a city's deals by discount %
5. expiring (7) hand-code — deals whose endDate is within N days
6. open-now (6) hand-code — merchants open at current time via branch timings

## Killed
best-card (weaker dup of wallet), footprint (analyst-only), compare (occasional), new-deals (dup of expiring), deal-map (folded into directions + --csv), city-leaderboard (cross-city niche)
