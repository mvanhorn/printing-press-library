# Phase 4.95 Local Code + SKILL/README Review

Review path: dedicated review subagent (correctness + security + docs) over the 8 hand-written novel command files, scrapecreators_novel_helpers.go, scrapecreators_migrations.go, README.md, SKILL.md.

## Autofixed in-place
- 6 README/SKILL narrative overclaims corrected by reviewer: creator find "28 platforms"→"12"; removed nonexistent "posting cadence" from creator compare; trends "rising fastest"→"biggest" (snapshot, not velocity); creator track "every synced platform"→"a chosen platform"; account budget "local command log fusion"→"API credit balance + daily usage".
- mcp:read-only annotation removed from creator track + ads monitor (they write snapshots to the local store → store update, not read-only per AGENTS.md tool-safety contract; missing hint = permission prompt, false read-only on a writer = real bug). 6 pure-read novel commands retain read-only.
- LatestAdSnapshot ordering: ORDER BY batch_id (RFC3339Nano string, drops trailing-zero fractional secs) → ORDER BY rowid (monotonic insertion order). Fixes sub-second batch mis-ordering.

## Verified clean
- Concurrency: 4 fan-out commands use pre-indexed-slot pattern, per-goroutine subRequestCtx bound, wg.Wait before read, no shared-map writes, no goroutine leak on cancel. `go test -race ./internal/cli/ ./internal/store/` PASS.
- SQLite: parameterized queries throughout; drain-first; NOT NULL columns (no NULL-scan hazard); BeginTx/prepared-stmt/Commit; no store.Upsert inside open tx.
- Input validation: help-probe → dryRunOK → usageErr → boundCtx ordering correct; os.Stat after dryRunOK.
- Secret safety: no key value logged/written; sanitizeFetchErr relies on client key masking.

## Accepted-as-is (by design, low severity)
- Fan-out commands surface per-fetch failures via fetch_failures[] + stderr but exit 0 even on uniform failure (aggregator partial-failure semantics; single-fetch commands track/spikes/budget do use classifyAPIError for typed exit codes).

## Retro candidates
- None in generated/out-of-scope files.
- Doc tension noted: AGENTS.md "store updates → skip mcp:read-only" vs printing-press SKILL.md "reading from the local store" phrasing. The precise reading ("only effect is reading") resolves it — store WRITES are not read-only. Candidate: tighten the SKILL.md wording to say "reads from (not writes to) the local store."
