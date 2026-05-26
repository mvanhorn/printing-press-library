# defillama-pp-cli — print brief (run 20260526-024500)

## API

- **Source:** DefiLlama free API at `api.llama.fi` (plus `coins.llama.fi`,
  `stablecoins.llama.fi`, `yields.llama.fi`), pro endpoints at
  `pro-api.llama.fi/{key}`.
- **Spec:** https://api-docs.defillama.com/defillama-openapi-free.json
- **Auth:** none on free tier. Optional `DEFILLAMA_PRO_KEY` env var unlocks
  bridges, emissions, hacks, raises, treasuries, ETFs, RWA, narratives, and
  derivatives.
- **Endpoint count:** 31 free + 38 pro across TVL, fees/revenue, yields,
  stablecoins, DEX volume, options, OI, prices.

## The agent problem

DefiLlama publishes the cleanest cross-chain DeFi data set there is, but the
useful questions are never single-endpoint queries. "Top Arbitrum protocols
by fee revenue" needs `/protocols` (5,000+ entries), filtered to Arbitrum,
then `/summary/fees/{protocol}` for each match — N+1 calls and megabytes
of JSON to hold in context. The existing MCP servers proxy raw endpoints
1:1; the SDK wrappers are similarly thin.

## Decision

Ship a Printing Press CLI that:

1. **Mirrors the free tier into local SQLite** so compound queries become
   one shell call (typical: 50ms against the mirror vs. 5–10 seconds and
   hundreds of KB of JSON via the API).
2. **Surfaces compound commands as the primary interface** — `top`,
   `compare`, `yields top`, `stables flow`, `profile`, `chains` — backed by
   a SQL escape hatch when the question doesn't fit a named command.
3. **Auto-syncs on staleness** so agents never see stale data, but never
   pay the sync cost twice within the window.
4. **Emits typed JSON** (TVL is a real number, not "$13.70B") so
   downstream parsing is trivial.
5. **Ships an MCP server** that delegates each tool call to a sibling CLI
   binary.

## Schema decisions worth noting

- `protocol_chain_tvl` is required to make `top --chain X` correct.
  Without it, every protocol's global TVL leaks into every chain view.
  Same fix applied to `dex_chain_volume` for `dexs --chain X`.
- `fees_overview` has both `total_24h_fees` and `total_24h_rev`. Revenue
  columns end in `_rev`, not `_revenue` — this trips agents and skills.
- Historical tables (`protocol_tvl_hist`, `fees_hist`, `dex_hist`,
  `chain_tvl_hist`, `pool_hist`, `stablecoin_hist`) are populated lazily
  via `sync --protocol <slug>` and `sync --chain <name>`; not part of the
  default overview sync.

## Non-goals

- Wallet-level analysis, on-chain transaction tracing, governance voting.
  These aren't in DefiLlama's API; the skill points to Nansen / Arkham /
  Tally instead.
