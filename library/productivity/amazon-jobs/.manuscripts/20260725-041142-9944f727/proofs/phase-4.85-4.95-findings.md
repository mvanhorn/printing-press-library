# Phase 4.85 (output review) + 4.95 (local code review) findings

## Phase 4.85 Agentic Output Review — WARN, 1 finding (fixed)
- format-bugs (warning): find/get/new embedded raw HTML (<br/>, <a href>) in JSON/--agent output.
  FIX: always convert HTML->readable text via cleanJob() in the curated commands (find/get/new),
  regardless of flags. Raw `postings search` endpoint kept faithful. Verified: find --json has no <br/>.
- semantic relevance, ranking, aggregation-coverage: PASS.

## Phase 4.95 Local Code Review — 3 findings (all fixed)
1. MEDIUM (data loss): new.go advanced the cursor to the full current id set but counted new_count
   AFTER truncating to --limit, so with >limit new reqs the excess were marked seen and never
   surfaced again. FIX: capture trueNewCount before truncation, report it, emit a truncation note;
   cursor still advances (correct) but count is honest.
2. LOW: searches --delete ignored --json/--agent (plain text under machine mode). FIX: route the
   delete result through emitResult -> {"deleted": name} JSON in machine mode.
3. LOW/edge: baseline never transitioned for an always-empty saved search (len(LastSeen) test).
   FIX: hadBaseline now keys on saved.LastSynced != "" (set only by a prior `new`).

Verified clean by reviewer: no SQL injection (grouping exprs from whitelist maps, values via ?
placeholders), no drain-first violations, correct rows/db/context lifecycle, NULL-safe scans.

Out-of-scope (generator-reserved) — no findings routed to retro.
