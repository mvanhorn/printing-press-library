# StackAdapt — Phase 5 Live Dogfood Acceptance

Level: Full Dogfood (binary-owned live matrix, read-only CLI)
Auth: bearer_token (real GraphQL token via env; read-only queries + local sync only)

Result: PASS
- matrix_size: 77
- tests_passed: 77
- tests_failed: 0
- tests_skipped: 36 (write/fixture-only commands not applicable to a read-only CLI)

Live evidence highlights (from resumed-run validation):
- account → id <account-id> USD
- sync --full → 427 real objects (advertisers 27, campaigns 100, campaign_groups 100, ads 100, segments 100)
- sql GROUP BY over the synced store → real per-type counts
- search "acme" → matched advertiser + campaign offline
- advertisers list --data-source local → served from store
- novel features sampled live: pacing, bottleneck, stale-campaigns, delivery-drift (4/4 in scorecard live probe)

Fixes applied this phase: none (all matrix commands passed first run).
Printing Press issues for retro: framework-fit — verify's data-pipeline gate and the
generator assume a syncable REST store; a GraphQL live-query CLI needs a hand-authored
sync/store/search/sql layer to satisfy it. Candidate: graphql_sync template wiring from a
GraphQL spec, or a verify accommodation for live-query CLIs.

Gate: PASS → promote.
