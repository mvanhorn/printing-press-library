# Robinhood Brokerage Route Map

This package combines the official Robinhood Crypto Trading API with a sanitized authenticated browser route map captured from Robinhood stock, options, crypto, account, settings, transfer, document, and market pages on 2026-05-27. The route-map commands intentionally mix Robinhood's own Crypto OpenAPI with the browser-backed brokerage/account map so agents can inspect one consolidated inventory.

## Sources

- Official Crypto Trading API documentation: `https://docs.robinhood.com/crypto/trading/`
- Official Crypto API help article: `https://robinhood.com/us/en/support/articles/crypto-api/`
- Public/community brokerage API reference: `https://github.com/jmfernandes/robin_stocks`
- Public/community route docs: `https://robin-stocks.readthedocs.io/`
- Personal companion map: `zaydiscold/robinhood-cli` generated `275` unified route entries, including `16` Robinhood-published Crypto entries, `259` brokerage/account templates, and `217` latest authenticated CDP templates.

## PP Write Gate

Read commands and route-map inspection do not require a write gate.

Live writes require both:

1. An explicit command path that can write, such as `crypto trading-orders-post` or `brokerage execute` against a write route.
2. `ROBINHOOD_PP_ALLOW_WRITES=1` in the environment.

The CLI defaults write routes to `--dry-run` unless `--live-write` is passed. The HTTP client also blocks mutating Crypto API verbs unless `ROBINHOOD_PP_ALLOW_WRITES=1` is present. MCP write tools expose `mcp:read-only=false` and `mcp:risk=<level>` metadata.

## Commands

```bash
robinhood-pp-cli brokerage summary --json
robinhood-pp-cli brokerage all-routes --host trading.robinhood.com --json
robinhood-pp-cli brokerage routes --host api.robinhood.com --risk sensitive-read --json
robinhood-pp-cli brokerage browser-routes --host bonfire.robinhood.com --json
robinhood-pp-cli brokerage plan https://api.robinhood.com/goku/lcm --json
robinhood-pp-cli brokerage execute https://api.robinhood.com/goku/lcm --json
```

The last command previews by default because `goku/lcm` is a `write-safe` route in the map. For real brokerage/account reads, provide `ROBINHOOD_BROKERAGE_TOKEN` or `ROBINHOOD_COOKIE`. For real writes, also provide `ROBINHOOD_PP_ALLOW_WRITES=1`.

## Current Map Counts

- Unified official Crypto plus brokerage/account entries: `275`
- Official Crypto entries from Robinhood OpenAPI: `16`
- Brokerage/account templates: `259`
- Latest authenticated CDP templates: `217`
- Hosts: `api`, `bonfire`, `cashier`, `dora`, `identi`, `minerva`, `nummus`, `phoenix`
- Risk classes: `read`, `sensitive-read`, `write-safe`, `write-mutate`, `write-or-sensitive`, `destructive`

## Mutation Field

Per-endpoint markdown generated in the personal companion map starts with `Mutation: yes|no`. For this PP package, `brokerage all-routes --json`, `brokerage routes --json`, and `brokerage browser-routes --json` expose the same mutation signal through each route's `risk` field:

- `read` and `sensitive-read`: `Mutation: no`
- `write-safe`, `write-mutate`, `write-or-sensitive`, `destructive`: `Mutation: yes`
