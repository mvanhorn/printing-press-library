# Novel-features brainstorm (subagent output, 2026-11-17)

## Customer model
- Katrin Maier, 38, Ulm, two kids, part-time: orders Thursday evening for Saturday pickup by WhatsApp voice note; 70% of the basket repeats weekly. Frustration: cannot see what is on offer without browsing ~1,200 offer articles; re-adds the same 25 items; found an unavailable item only at the final step.
- Herbert Brandt, 72, Neu-Ulm, pickup only, cash: "the same as last week, but no sausage". Frustration: no real buy-again; weight articles and transport box deposit surprise him.
- Mehmet Kaya, 29, student, offer hunter with two EDEKA markets nearby: Sunday offer scan, compares staples across markets. Frustration: markets price the same article differently; cannot tell whether an offer beats the shelf price from two weeks ago; cannot see what changed since last week.
- OneDone shopping agent (WhatsApp, ChatGPT/Claude via MCP): voice note -> basket operations, offers for usual items, slot proposal, yes before order place --confirm. Frustration: needs small deterministic JSON answers, one call each, no surprises at the order step.

## Candidates (pre-cut)
1 wishlist add/list/check (keep) - 2 deals since (keep) - 3 prices history (keep) - 4 basket audit (keep) - 5 order reorder (keep) - 6 order usuals (heavy) - 7 compare markets (keep) - 8 deals cheapest (fragile unit normalisation) - 9 slots wait (background poll, scope creep) - 10 weights list (field of basket audit) - 11 delivery check (market finder already has delivery postal codes) - 12 payback estimate (no data, unverifiable) - 13 basket fit (needs LLM judgement) - 14 plan meals (LLM).

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Wishlist offer check | wishlist check | 8/10 | hand-code | Katrin, Agent |
| 2 | New-offers diff | deals since | 7/10 | hand-code | Mehmet, Agent |
| 3 | Price history | prices history | 8/10 | hand-code | Mehmet |
| 4 | Basket audit | basket audit | 7/10 | hand-code | Herbert, Katrin, Agent |
| 5 | Reorder a past order | order reorder | 8/10 | hand-code | Herbert, Katrin, Agent |
| 6 | Cross-market price compare | compare markets | 6/10 | hand-code | Mehmet |
### Killed candidates
order usuals (heavy, one order show per past order) - deals cheapest (fragile units) - slots wait (recurring job) - weights list (field of audit) - delivery check (absorbed in markets search) - payback estimate (no data) - basket fit (LLM) - plan meals (LLM).

## Build owner's decisions after the brainstorm
- Kept: price history (as `price history`), compare markets.
- Reframed: wishlist check -> `deals check <terms...>` (no local wishlist store; offer-badged search hits per term).
- Dropped: deals since (one snapshot needs ~40 offer pages of 2.7 MB each), basket audit (one 2 MB product page per basket line), order reorder (no order on the test account, cannot be verified without placing an order, which is not authorized).
