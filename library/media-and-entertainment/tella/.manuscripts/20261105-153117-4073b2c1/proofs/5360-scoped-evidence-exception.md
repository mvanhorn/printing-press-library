# Scoped evidence and known verifier gap

The generic live verifier is intentionally not run because upstream [cli-printing-press #5360](https://github.com/mvanhorn/cli-printing-press/issues/5360) can fall back to unsafe unscoped sync. Evidence uses a scoped live matrix and mock verification. No phase5 acceptance marker was fabricated. Binary publish validation exits 5 for the missing generic marker; it has not passed.

The final scoped matrix contains 361 rows: 120 PASS, 0 FAIL, 241 SKIP, covering 288 CLI paths and 73 runtime MCP tools. Read-only MCP discovery has 30 tools. Mock verification passed 210/210. Skips remain skips, including unavailable positive-retake fixtures and output review. No full live coverage is claimed.

Exactly one paid chat consumed 41.38815 credits. Three owned test copies were created and deleted; cleanup confirmed GET 404 and a complete 45-row listing with no next cursor. Original source timelines remained unchanged. No additional paid chat, generic live verifier/dogfood, unscoped sync, or publication call was run for public preparation.
