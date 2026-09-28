# Publication guardrail verification

PR #2070 corrects the publishing account to `zjsng` and resolves three additional review findings: client-controlled MCP cache destinations, explicit room-bathroom negation, and nested comparison field selection. The prior partial-result repairs remain intact.

Full Go tests, vet, build, publication validation and reachable-vulnerability checks pass. The fresh full publication gate passes 101/101 executed checks with 78 documented skips. Nine deterministic witness tests pass; additional Execute-level and parser tests cover source references, overlapping selection, empty inventory, partial failures, explicit bath absence and independently evidenced outdoor baths.

Live source acceptance covers 37 distinct scenarios across a full run (36 passing) and a targeted pagination rerun (passing), recorded in publication-source-acceptance.json. Earlier failed witnesses are retained. Independent search requests returned different native property-card sets; a later request cannot validate the first request's exact pagination snapshot. Pagination now checks the fresh cached HTTP body with an independent parser and binds its URL, timestamp and body hash to the response. Other witnesses remain separate HTTP requests with matching anonymous headers. Price checks stay scoped to the matching native property card. No raw source pages are published.

| Comparison | Cold bytes / latency / requests / peak KiB | Warm full bytes / latency / requests / peak KiB | Warm selected bytes / latency / requests / peak KiB |
|---|---|---|---|
| Two dates | 15,599 / 1,894 ms / 2 / 32,240 | 15,600 / 62.58 ms / 0 / 27,568 | 8,066 / 59.72 ms / 0 / 27,280 |
| Two exact plans | 22,341 / 888.54 ms / 2 / 27,696 | 22,342 / 38.05 ms / 0 / 23,312 | 7,345 / 32.01 ms / 0 / 23,136 |

Selection retains identical offer IDs and whole price objects, order/counts, query, coverage, source observations and original timestamps. These are observed runs, not latency guarantees. The added Japanese negation variants and blocked filesystem attempts are deterministic cases, not claims of finding those exact phrases on live source pages.
