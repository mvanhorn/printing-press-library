# Phase 4.95 — local code review (immoweb-pp-cli)

Review path: direct reviewer-subagent dispatch (correctness, security, maintainability), all three re-run every round.
In-scope: internal/immo/, internal/store/immoweb_store.go, hand-written internal/cli/ commands. Generated-code defects that affect this CLI's safety were patched locally and recorded in .printing-press-patches/.

## Autofix summary
All findings autofixed in place across 3 rounds (no git repo; the working tree and .printing-press-patches/ are the record). Round 1: ~25 findings. Round 2: 21 (correctness 6, security 4, maintainability 11). Round 3: 7 (correctness 2, security 1, maintainability 4).

## Surface-to-user findings
None. No finding required a scope, dependency or research tradeoff.

## Deferred low items (documented, not fixed)
- scopeHarvest/ensureArea resolve communes twice (one extra cached autocomplete call).
- "postcode or commune → Criteria" branch repeated 4 times; triage RunE ~240 lines; db!=nil guards in harvest; appendUniq duplicates immo.appendUnique.
- Offline commune resolution outside Brussels uses stored locality text (a stray locality can widen the postal-code set vs the live resolution).
- photos --dir is an MCP parameter (fixed NN.ext names, CDN-only content; tool is not annotated read-only).

## Retro candidates (Printing Press, out of scope here)
- cobratree/shellout.go cliArgsFromMCP emitted "--flag value" as two tokens: a string sent for a bool flag smuggled hidden/blocked flags (--db, --base-url, --config). Patched locally (mcp-flag-values-joined-with-equals); should be fixed in the generator.
- store.OpenWithContext / hardenSQLiteFiles chmods an existing path before checking it is SQLite; hardenSQLiteFiles opened/closed DB files and released POSIX locks (patched locally, sqlite-harden-must-not-open-db-files).
- Generated sync/search also expose --db to MCP.
- boundCtx applies --timeout as a whole-command deadline (harvesting commands need a longer overall budget); RateLimitError always says 429; 2 req/s auto rate is slow for count-heavy harvests.

## Convergence outcome
Stopped at the round-3 cap: round-3 findings were all fixed in place and verified (go test ./..., live checks, shipcheck PASS 7/7, 89/100); no fourth review round was run.
