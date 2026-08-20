# Wolt CLI Shipcheck Report

## Verdict: ship

## Numbers
- Shipcheck: 6/6 legs PASS (verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard)
- Scorecard: 78/100 Grade B
- Phase 5 dogfood: quick-check PASS (5/5 commands tested live against Wolt API)
- Live smoke of 4 novel commands: all PASS against Helsinki/Wolt
- govulncheck: needed `go get golang.org/x/net@latest` to clear GO-2026-5026 (idna punycode); fixed in v0.55.0

## What ships
- 4 fully-working consumer endpoint families: cities list, restaurants-near (lat/lon), search (venues|items), venue dynamic (open status/ETA/delivery config)
- 4 novel hand-written commands: `venues-now`, `venues-compare`, `cuisine-bottleneck`, `track`
- SQLite store, FTS5 search, `--json`/`--csv`/`--select`/`--profile`/`--compact`/`--agent`
- MCP server via runtime Cobra-tree mirror

## Known Gaps (also in README)
- `venues <slug>` (menu items via documented /v4 path): returns HTTP 200 with empty body via CloudFront. Surfaces empty payload honestly; future browser-sniff should find the live path.
- `track <share-link>`: stub that extracts the order id; live JSON tracking endpoint not yet reverse-engineered.

## Scorecard gaps (not blockers)
- Insight 2/10 (README cookbook depth)
- Cache Freshness 5/10
- Workflows 6/10
- These are polish-tier improvements; CLI is functional and shippable.
