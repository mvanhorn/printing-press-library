# defillama-pp-cli

Agent-native CLI for DefiLlama. SQLite-backed, compound queries, token-efficient.

DefiLlama is the de facto source for cross-chain DeFi data: TVL, fees, revenue,
yields, stablecoin flows, DEX volume, bridge activity across 5,000+ protocols
and 350+ chains. The interesting questions are never single-endpoint calls —
"top protocols on Arbitrum by fee revenue" requires fetching all protocols,
filtering by chain, then hitting `/summary/fees/{protocol}` for each match.

This CLI mirrors everything into a local SQLite database, so compound queries
that would take 3–10 API calls and megabytes of JSON become one shell command.

## Install

```bash
go install github.com/kierandotai/defillama-pp-cli/cmd/defillama-pp-cli@latest
```

The free tier needs no auth. Set `DEFILLAMA_PRO_KEY` (or `defillama-pp-cli config set pro-key <key>`) to unlock bridges, emissions, hacks, raises, treasuries, ETFs, RWA, narratives, and derivatives.

## Quick start

```bash
# One-time sync of the free-tier overview tables (~30s).
defillama-pp-cli sync

# Top protocols on Arbitrum, with fees and revenue.
defillama-pp-cli top --chain arbitrum --with fees,revenue --limit 20

# Find value extraction: high revenue, declining TVL.
defillama-pp-cli sql "SELECT p.name, p.tvl, f.total_24h_rev, p.change_7d
  FROM protocols p JOIN fees_overview f ON p.slug = f.protocol
  WHERE f.total_24h_rev > 50000 AND p.change_7d < -5
  ORDER BY f.total_24h_rev DESC LIMIT 15"

# Best stablecoin yields on Solana, $5M minimum TVL.
defillama-pp-cli yields top --chain solana --stablecoin-only --min-tvl 5000000 --sort apy --limit 10

# Side-by-side comparison over 90 days.
defillama-pp-cli sync --protocol aave --backfill
defillama-pp-cli compare aave compound morpho --metrics tvl,fees --period 90d
```

Default output is a compact table. Add `--json` for machine-readable JSON (raw
numbers, not formatted strings), `--csv` for CSV, or `--limit N` to cap rows.

## Agent integration

This CLI ships with `SKILL.md` documenting the routing rules and SQLite schema
for use inside agent systems. See [SKILL.md](SKILL.md) for command-by-command
guidance, plus the full table schema agents need to write `sql` queries.

It also runs as an MCP stdio server:

```bash
defillama-pp-cli mcp
```

Ten JSON-RPC tools wrap the compound commands: `defillama_top`, `defillama_compare`,
`defillama_yields`, `defillama_stables`, `defillama_fees`, `defillama_dexs`,
`defillama_profile`, `defillama_price`, `defillama_sql`, `defillama_sync`.

## How it fits the Printing Press

This CLI follows the [Printing Press](https://printing-press.dev) conventions:
SQLite mirror, compound commands, structured table/JSON output, agent skill,
and an MCP server entry point.

## API source

[DefiLlama API docs](https://api-docs.defillama.com/) · free tier covers 31
endpoints. No auth required.

## License

MIT — see [LICENSE](LICENSE).
