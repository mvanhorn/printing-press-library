# Phase 4.85 — agentic output review (run against the real, signed-in site via auth login --chrome)

Reviewer: operator inspection of live command output vs the site UI (no sub-skill available in this harness).

| Command | Check | Result |
|---|---|---|
| movers --kind gainers | TECHM +8.48%, WIPRO +5.95%, TCS +4.89% match /live "Top movers"; RS joined from screener | OK |
| screen 'close > sma50 and sma50 > sma200 and rs >= 70' --sector IT | 3 matches, all satisfy the predicate, sorted by rs desc | OK |
| sectors --window 1m | IT ranks first; share above 50/200 DMA computed from synced rows | OK |
| breadth | 96 up / 104 down, 35.2% above 200 DMA, 23.5% above 50 DMA — match /market ("24%", "35%", A/D) | OK |
| brief | Nifty 50/100/200 levels match site (23,154.46); breadth, sector leaders/laggards, signals present; watchlist empty on this account | OK |
| quote RELIANCE | price 1,211.20, 52w 1,152.20–1,611.80 match the chart page; sector percentiles present | OK |
| history RELIANCE / M&M | 1,236 adjusted bars; bonus factor applied (first close 1,353.8); special-character symbol works | OK |
| changes / breadth / sectors --trend | one snapshot stored -> honest empty result + note, exit 0 | OK (by design) |

## Findings
1. WARNING (fixed): `history NOPE` blamed an expired session for an unknown symbol. Now returns a not-found error (exit 3) naming both causes.
2. INFO: `watchlists` is omitted from `quote` when empty (omitempty) — intended.

No blocking findings.
