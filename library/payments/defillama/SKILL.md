---
name: pp-defillama
description: DefiLlama Printing Press CLI -- cross-chain DeFi data (TVL, fees, yields, stablecoins, DEX volume) via a local SQLite mirror. Use for any question that involves comparing protocols, filtering by chain/category, or pulling historical economic data.
binary: defillama-pp-cli
---

# pp-defillama

`defillama-pp-cli` is a SQLite-backed command-line front end for the DefiLlama
API. The local mirror means compound queries (top-N with filters, cross-domain
joins, historical comparisons) are one shell call instead of N+1 HTTP requests
plus megabytes of JSON.

## When to use it

Reach for `defillama-pp-cli` when you need:

- Protocol economics: TVL, fees, revenue, market cap, chain breakdown
- Cross-chain comparisons: chain leaderboards, per-chain stablecoin supply
- Yield discovery: pool APYs, TVL floors, stablecoin-only filters
- DEX volume and market structure: per-DEX, per-chain
- Historical series: TVL/fees/volume over 7d, 30d, 90d, 180d, 1y, all
- Token prices by contract address (live pass-through)
- Stablecoin flow: which chains are gaining/losing supply

Don't use it for: wallet-level analysis, on-chain transaction tracing, or
governance voting -- DefiLlama doesn't track those. Suggest Nansen, Arkham, or
Tally instead.

## Common workflows

```bash
# Top protocols on a chain with fees + revenue
defillama-pp-cli top --chain arbitrum --with fees,revenue --limit 20

# Side-by-side comparison
defillama-pp-cli compare aave compound morpho-blue --metrics tvl,fees,revenue

# Time series for a single protocol
defillama-pp-cli sync --protocol aave   # populate history first
defillama-pp-cli profile aave --period 90d
defillama-pp-cli tvl aave --history --period 90d
defillama-pp-cli fees aave --history --period 30d

# Best stablecoin yields
defillama-pp-cli yields top --chain solana --stablecoin-only --min-tvl 5000000 --sort apy

# Stablecoin flow
defillama-pp-cli stables flow --period 30d --sort change

# Live token price (pass-through, no mirror)
defillama-pp-cli price ethereum:0xdac17f958d2ee523a2206206994597c13d831ec7

# Raw SQL when no named command fits. The escape hatch is read-only: only
# SELECT is allowed (DDL and DML are rejected). Tip: spaces around `<` or `>`
# are interpreted by the shell as redirects -- pipe the query in from a file
# or quote tightly (e.g. `WHERE x>0` without spaces) when comparisons are
# needed.
defillama-pp-cli sql "SELECT p.name, p.tvl, p.category FROM protocols p ORDER BY p.tvl DESC LIMIT 15"

# Export to file
defillama-pp-cli export --format csv --query "SELECT ..." --output data.csv
```

## Routing rules

- **For multi-protocol questions** use `top`, `compare`, or `yields top`. These
  are the highest-leverage commands.
- **For a deep dive on one protocol** use `profile <slug>` (add `--period 90d`
  for the historical section).
- **For chain-level questions** use `tvl --chain X`, `chains`, or
  `stables --chain X`.
- **For historical data** run `sync --protocol <slug>` first; only the current
  snapshot is in the default sync. Use `--backfill` for full history.
- **When the named commands don't fit** use `sql "<select>"` directly against
  the schema below. The CLI rejects anything other than `SELECT`.

## Data freshness

Overview tables (protocols, chains, pools, fees, dexs, stables) auto-sync if
older than the `stale-threshold` config (default 1h). Historical data uses
`stale-historical` (default 24h). Pass `--no-sync` to skip the check.

## Output

Default is a compact table. Add `--json` for raw numeric JSON (TVL values are
real numbers, not formatted strings), or `--csv`. Use `--limit N`.

## SQLite schema (free tier)

Overview tables, populated by `sync`:

- `protocols(slug, name, symbol, chain, category, tvl, mcap, change_1h, change_1d, change_7d, chains, url, description)` -- one row per protocol; `tvl` is global, `chains` is a JSON array.
- `protocol_chain_tvl(protocol_slug, chain, tvl)` -- per-chain TVL breakdown. **Use this, not `protocols.tvl`, when filtering or aggregating by chain.**
- `chains(name, tvl, token_symbol, cmc_id, gecko_id)`
- `pools(pool_id, chain, project, symbol, tvl_usd, apy, apy_base, apy_reward, il_risk, stablecoin, exposure, ...)` -- yields.
- `stablecoins(id, name, symbol, peg_type, peg_mechanism, circulating, price, mcap, chains)`
- `stablecoin_chains(stablecoin_id, chain, circulating)`
- `dex_overview(protocol, display_name, total_24h, total_7d, total_30d, change_1d, change_7d, change_30d, chains)`
- `fees_overview(protocol, display_name, total_24h_fees, total_24h_rev, total_7d_fees, total_7d_rev, total_30d_fees, total_30d_rev, category, chains)`
- `options_overview(protocol, display_name, total_24h, total_7d, chains)`
- `open_interest(protocol, display_name, total_oi, chains)`
- `sync_meta(domain, last_sync, row_count, note)`

Historical tables, populated by `sync --protocol`, `sync --chain`, `stables flow`:

- `protocol_tvl_hist(protocol_slug, date, tvl, chain)` -- `chain = ''` means total.
- `chain_tvl_hist(chain, date, tvl)`
- `fees_hist(protocol, date, fees, revenue)`
- `dex_hist(protocol, date, volume)`
- `pool_hist(pool_id, date, tvl, apy)`
- `stablecoin_hist(stablecoin_id, date, circulating, chain)` -- aggregate per chain stored with `stablecoin_id = '_total'`.

## Important column quirks

- Revenue columns end in `_rev` (`total_24h_rev`, `total_7d_rev`, `total_30d_rev`).
- TVL deltas are `change_1h`, `change_1d`, `change_7d`. There is no `change_1m`.
- `pools.tvl_usd` (not `tvl`); APY is on a 0-100 scale already, not 0-1.
- `protocols.chains` is a JSON array string; query with `json_each`.

## Pro tier

If `DEFILLAMA_PRO_KEY` (or `config set pro-key`) is configured, additional
commands unlock: `bridges`, `emissions`, `hacks`, `raises`, `treasuries`, `etfs`,
`rwa`, `narratives`, `derivatives`. Run `sync --pro` to populate.
