# Patch: use priceapi-specific client for indices/stocks generated commands

## Files
- internal/cli/promoted_indices.go
- internal/cli/promoted_stocks.go

## Change
Replaced `flags.newClient()` (default www.moneycontrol.com base) with
`newPriceAPIClient(flags)` (priceapi.moneycontrol.com base) in both generated
commands.

## Why
The default www client + resolver path sends request headers that priceapi's
Akamai edge blocks with HTTP 503, even though the same URL returns 200 via curl
and via the priceapi-pointed client used by the novel `market-wrap`/`stock-watch`
commands. Using the priceapi-specific client (which already works in the novel
commands) makes the generated `indices get` and `stocks quote` commands reliable.

The `newPriceAPIClient` helper lives in the hand-authored `mc_helpers.go` and is
preserved across reprints.
