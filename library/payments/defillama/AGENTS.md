# DefiLlama Printed CLI Agent Guide

This directory ships `defillama-pp-cli`, a Printing Press CLI for DefiLlama.
The local SQLite mirror is the load-bearing piece: compound queries that
would require N+1 API calls and megabytes of JSON become one shell command.

## Local Operating Contract

Start by asking the CLI for current runtime truth instead of trusting a
copied command list:

```bash
defillama-pp-cli --help
defillama-pp-cli <command> --help
```

Every command supports machine-readable output. `--json` emits raw numeric
values (TVL is a number, not `"$13.70B"`), `--csv` does the same without
scientific notation, and `--no-header` is available for downstream scripts.

The local mirror auto-syncs when overview tables are older than the
`stale-threshold` config (default 1h) or historical tables older than
`stale-historical` (default 24h). Pass `--no-sync` to skip the freshness
check.

```bash
defillama-pp-cli sync                                  # full sync
defillama-pp-cli sync --status                         # check freshness
defillama-pp-cli sync --protocol aave --backfill       # full history for a protocol
defillama-pp-cli sync --chain ethereum                 # chain TVL history
```

For arbitrary questions that don't map to a named command, use the
read-only SQL escape hatch — DDL and DML are rejected:

```bash
defillama-pp-cli sql "SELECT p.name, p.tvl, f.total_24h_rev, p.change_7d
  FROM protocols p JOIN fees_overview f ON p.slug = f.protocol
  WHERE f.total_24h_rev > 50000 AND p.change_7d < -5
  ORDER BY f.total_24h_rev DESC LIMIT 15"
```

The full schema is documented in `SKILL.md`. Key tables to know about:

- `protocols` and `protocol_chain_tvl` — global vs per-chain TVL
- `fees_overview` — fees and revenue (columns end in `_rev`, not `_revenue`)
- `pools`, `pool_hist` — yields
- `stablecoins`, `stablecoin_chains`, `stablecoin_hist` — circulating supply
- `dex_overview`, `dex_chain_volume`, `dex_hist` — DEX volume (global, per-chain, historical)

## MCP

`defillama-pp-mcp` is the stdio MCP server bundled alongside the CLI. It
delegates each tool call to a sibling `defillama-pp-cli` binary (found by
adjacency, `$DEFILLAMA_PP_CLI`, or `$PATH`). For direct shell debugging
the CLI also exposes the same logic via `defillama-pp-cli mcp`.

## Local Customizations

If you modify this CLI beyond what was published, record each customization
so it isn't lost on the next regen.

1. **Mark every changed site** in source with a comment:

    ```
    // PATCH: <one-line summary>
    ```

    Then `grep -rn 'PATCH' .` surfaces every customization from this
    directory.

2. **Catalog the change** in `.printing-press-patches.json` (parallel to
   `.printing-press.json`). Minimum shape:

    ```json
    {
      "schema_version": 1,
      "applied_at": "YYYY-MM-DD",
      "base_run_id": "<copy from .printing-press.json>",
      "base_printing_press_version": "<copy from .printing-press.json>",
      "patches": [
        {
          "id": "short-identifier",
          "summary": "what changed and why",
          "files": ["relative/path"]
        }
      ]
    }
    ```

For product docs, install notes, and full command examples, read `README.md`
and `SKILL.md`. This file intentionally stays small so repo-local agents
get invariant local guidance without duplicating the published docs.
