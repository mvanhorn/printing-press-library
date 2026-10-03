# SnowJapan live acceptance

PASS — full Press-owned matrix: 135/135, zero failures. Source fingerprint: 24b9e7dd555da90847bcfb88c33509fed243738cdd1a40646fbcf0cd2fe8bc45.

The portable `scripts/verify-live.sh` prepares a disposable SQLite fixture by executing real public directory/season reads (478 resorts, 417 historical rows), captures one detail twice, supplies that declared fixture to all planner test annotations through the existing database flag, and deletes it on exit. The five planner happy paths have populated factual results, no dry-run and no uncaptured-season state. Changes reaches the available-baseline path. The current runner's special sync row is an explicit skip; real sync was independently executed by fixture setup. No hollow feature flags are present.

Unmatched free-text search remains an ordinary empty success, with a documented error-probe opt-out. Malformed exact IDs, duplicate scopes, unsupported dates, source/schema failures and actual network-only fallback remain distinct. The same fresh reviewer resolved nine findings and checked source, typed MCP, populated computations, docs and the real extracted bundle.

Ordinary JSON/agent collection parity checks inspect several resort, season, report and municipality rows. MCP collection parity is also verified on the packaged server. Decision fields and provenance are retained within output bounds; detail-only smoke tests are not treated as collection coverage.

Representative live search: 0.87 seconds, 31,604,736 bytes peak resident memory. Local town planning: 0.02 seconds, 26,460,160 bytes. Source-specific gosec findings: zero; emitted runtime findings are separately recorded as generator candidates. Tool quality and PII gates have no pending findings.

Scope: Japan resort facts and dated base/town observations, historical span evidence and five local computations. No live lift operation, forecasting, booking, popular-region membership filter or uninterrupted-operation inference. Source contradictions stay ambiguous/inconsistent rather than being corrected by guesswork.

Evidence: source-bound phase5-acceptance.json, actual matrix argv audit, reproducible script, collection and artifact proofs, source-contracts.json and REVIEW.md. Final publisher validation and a fresh publication-time live gate remain required.
