# AutoTempest Novel-Features Brainstorm (subagent audit)

## Customer model
- Dana (cross-site comparer): re-runs same searches; no memory of what's new/cheaper between sessions.
- Marcus (deal hunter): sorts by price but lacks a market baseline ("is $18k good?").
- Priya (flipper/reseller): same VIN on multiple sources at different prices; dedupe by hand.
- Atlas (the agent): wants stable JSON with VIN keys; existing tools are HTML/Excel/paid.

## Survivors (>=5/10)
1. drops 8/10 hand-code — price-drop watch (cross-snapshot delta)
2. dedupe 8/10 hand-code — cross-source VIN collapse (GROUP BY vin, per-source price array)
3. deal 7/10 hand-code — mechanical median-delta vs comparable (model+year+mileage band)
4. spread 7/10 hand-code — per-source min/median/max price
5. watch 7/10 hand-code — saved-search registry + sync replay (enables drops/diff)
6. auctions 6/10 spec-emits — eBay auction slice (currentBid/bids)

## Killed
diff (fold into snapshot machinery), mileage-curve (inside deal), nearby (search --sort distance), history (subset of drops), get (thin wrapper → search --vin), gone (--gone flag), stale-price (framework stale), markets (sql one-liner).
