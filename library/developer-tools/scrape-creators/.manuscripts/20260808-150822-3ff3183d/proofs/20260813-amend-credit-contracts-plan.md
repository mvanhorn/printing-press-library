---
date: 2026-08-13
target_cli: scrape-creators-pp-cli
amend_run_id: amend-2026-08-13T1200
scope_tier: all
findings_count: 3
mode: direct
base: upstream/main after #1680 merge (b20261cf27), branch amend/scrape-creators-20260813
---

# Amend plan: scrape-creators-pp-cli (F1+F2+F3)

## F1 — bug: unreachable pre-first-fetch guard in `comments sweep` + misleading test

- **Target files:** `internal/cli/comments_sweep.go`, `internal/cli/amend_sweep_budget_test.go`
- **Fact:** `newSweepBudget` starts `charged=0, maxCost=1`, so `allows()` is true for
  every possible `--max-credits` before the first charge. The special-cased guard at
  `comments_sweep.go:97-100` can never fire; `TestSweepBudget_GatesTheFirstFetch`
  asserts only constructor defaults and its own comment ("admits nothing") contradicts
  its assertion (asserts -1 IS admitted).
- **Fix:** remove the unreachable branch; document at the first fetch site and on the
  `sweepBudget` doc comment that a fresh budget admits the first fetch by construction
  (floor estimate, integer budgets) and enforcement starts at `charge()`. Replace the
  misleading test with one named for the true contract
  (`TestSweepBudget_FreshBudgetAdmitsTheFirstFetch`) plus integration tests (via the F2
  seam) proving the *reachable* gates: comments-fetch gate, posts-page gate, breach halt.
- **Expected behavior change:** none at runtime (branch was dead); test suite now pins
  the real contract.

## F2 — feature: injectable client seam + behavioral tests for the credit-spend contracts

- **Target files:** new `internal/cli/amend_credit_seam.go`, refactors in
  `comments_sweep.go`, `comments_thread.go`, `account_estimate.go`; new
  `internal/cli/amend_credit_contracts_test.go`
- **Fix:** introduce `apiGetter` (the `Get(ctx, path, params)` subset of
  `*client.Client`); extract each RunE's fetch/decision logic into functions taking
  `apiGetter`:
  - `runCommentsSweep(ctx, c, db *sql.DB, opts)` — sweep loop (budget gating, cutoff,
    max-posts, pagination, persistence);
  - `fetchCommentThread(ctx, c, opts)` — probe, route decision, flat/per-comment fetch,
    traversal (F3), returns envelope + store rows;
  - `runAccountEstimate(ctx, c, plan)` — projection + live-balance parse
    (`parseCreditBalance`), returns envelope; RunE maps `OverBudget` to exit 7.
- **Tests (offline, scripted fake getter + temp sqlite):** estimate exit-7 boundary
  (projected > balance over, == not over; creditCount number and string; zero balance →
  error, not exit 7); thread auto-routing thresholds (16→flat, 15→flat, 14→per-comment)
  and forced flat = exactly one fetch; sweep budget stop / max-posts stop / breach halt.
- **Expected behavior change:** none; pure refactor + tests.

## F3 — feature: budgeted traversal of further top-level comment pages in `comments thread`

- **Target files:** `internal/cli/comments_thread.go`, README.md, SKILL.md
- **Direction (user-confirmed):** `--max-credits` on `comments thread`, traversal via
  `sweepBudget`, `truncated:true` retained when stopping on budget.
- **Design:** `--max-credits 0` (default) keeps today's behavior — single page,
  `truncated:true` + note (note now points at `--max-credits`). `--max-credits N>0`
  enables cursor traversal (`cursor` request param / `cursor` response field, gated by
  `envelopeHasMore`) of further top-level pages, on both routes (flat pages keep
  `include_replies=true`; per-comment pages fetch replies per new comment, each fetch
  gated). Every fetch, including the probe and reply calls, is charged to one
  `sweepBudget`; on a pre-fetch refusal or a post-charge breach the command stops,
  keeps what it fetched, sets `truncated:true` when pages remain, and explains in
  `note`. A 50-page safety cap backs the budget. `max_credits` and true
  `credits_charged` (from the budget) land in the envelope.
  Default asymmetry with sweep (`0` = no budget there, `0` = no traversal here) is
  deliberate — traversal is new spend, so it must be opt-in — and documented on the flag.
- **Docs:** README/SKILL `comments thread` unique-feature bullet + examples gain the
  `--max-credits` recipe; flag help states the 0-default semantics.

## Risks / dependencies
- F3 depends on F2's extraction (traversal is implemented inside `fetchCommentThread`).
- Per-comment traversal multiplies paid calls; mitigated by per-fetch gating and opt-in default.
- `parseCommentsPayload` returns only the first array it finds; traversal appends page
  by page so no cross-page merge logic is needed.

## Validation plan
`go build ./... && go vet ./...`, focused `go test ./internal/cli/`, then
`cli-printing-press publish validate --dir $CLI_DIR --json`, then the library's
`verify_skill.py` against the updated docs.
