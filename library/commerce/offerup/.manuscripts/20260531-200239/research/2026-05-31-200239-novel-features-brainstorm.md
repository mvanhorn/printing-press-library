# OfferUp Novel-Features Brainstorm (subagent audit trail)

Run: 20260531-200239 · First print · subagent_type: general-purpose

## Customer model

**Renata — the weekend flipper/reseller (Phoenix, AZ).** Today: buys underpriced furniture/tools/electronics on OfferUp, resells on FB Marketplace/eBay. Runs 6-8 keyword searches by hand every morning, eyeballs prices, keeps a messy Notes file of "what a Herman Miller Aeron actually goes for in Phoenix." Weekly ritual: re-runs sourcing searches several times a week, scrolls for below-rate items, DMs sellers before competitors. Frustration: no memory between sessions — can't tell new-today vs week-old, no hard "going rate" number, misses fresh underpriced drops.

**Marcus — the patient local bargain hunter (Seattle, WA).** Today: wants one specific thing at the right price, willing to wait. Opens app every couple days, repeats search, tries to remember what he saw. Weekly ritual: checks standing wish-list searches near his ZIP within a tight radius; passes on firm/far-pickup. Frustration: no clean "what's new since I last looked," can't tell if a seller dropped the price on a watched item — feed just reshuffles.

**Dev / Priya — the agent-builder automating deal alerts.** Today: wires APIs into an assistant watching local marketplaces. OfferUp has no public API → stuck maintaining a brittle Apify scraper returning one-shot JSON, no persistence/history. Weekly ritual: runs scraper on cron, diffs by hand, bolts on own "is this cheap?" logic. Frustration: every scraper is stateless and not agent-native — no MCP, no stable --json/--select, no local store.

## Candidates (pre-cut)

1. price-check — KEEP (aggregation over local store, legit)
2. deals/underpriced — KEEP
3. new-since/watch — KEEP
4. price-drops — KEEP
5. seller-scan — KEEP
6. price-trend — KEEP→later killed
7. firm-vs-negotiable — CUT (thin wrapper over `analytics --group-by`; folded into price-check)
8. markdown-watch — MERGE into deals as `--markdowns`
9. vehicle-deals — SOFT-KEEP→killed (niche; covered by deals)
10. nearby/closest — CUT (thin rename of search --zip/--radius/--sort)
11. export-watchlist — CUT (--csv/--json are global flags)
12. digest — KEEP (descoped to one composite read)

## Survivors (≥5/10, all hand-code)

| # | Feature | Command | Score | Buildability | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Local going-rate stats | price-check "<query>" --zip <z> | 10/10 | hand-code | Renata |
| 2 | Below-median deal flagging | deals "<query>" --zip <z> --below 20 [--markdowns] | 10/10 | hand-code | Renata |
| 3 | New-listing diff per query | new-since "<query>" --since 24h | 10/10 | hand-code | Marcus/Renata |
| 4 | Cross-sync price-drop detection | price-drops "<query>" --since 7d | 8/10 | hand-code | Marcus |
| 5 | Offline seller inventory + reputation | seller-scan <sellerId> | 8/10 | hand-code | Renata |
| 6 | One-call daily deal report | digest "<query>" --since 24h | 8/10 | hand-code | Renata/Priya |

## Killed candidates

| Feature | Kill reason | Sibling |
|---------|-------------|---------|
| firm-vs-negotiable | Thin wrapper over framework `analytics`; ratio folded into price-check | price-check |
| markdown-watch | Overlaps deals; merged as `--markdowns` mode | deals |
| price-trend | Needs weeks of snapshots, won't verify in dogfood, weakest of price trio | price-drops |
| vehicle-deals | Niche auto sub-segment; covered by deals; price-per-mile is a flag | deals |
| nearby/closest | Thin rename of search --zip/--radius/--sort distance | new-since |
| export-watchlist | --csv/--json are global output flags, not a feature | digest |
