# Phase 4.95 — Local Code Review findings

## Review path chosen
Direct reviewer-subagent dispatch via the Agent tool: `correctness` + `security/maintainability` lenses in parallel, scoped to hand-authored files (umami_*.go, watch/seo/coverage/movers/new_referrers/pace.go, store/umami_migrations.go, root.go hand-edit block).

## Autofix summary
14 findings autofixed in-place across round 1 (3 errors, 11 warnings): export binary-envelope decode (`_pp_binary`), peak-hours/seo/pages/new-referrers limit guards (slice-panic class), new-referrers missing window end bound, auth login 401→exit 4, --qualified non-JSON warning, --period vs explicit --end-at conflict error, movers `new` marker, pace `new` verdict, seo Google suffix-match + direct excluded from referral denominator, snapshot upsert error surfacing, snapshot metric-day timezone consistency, dead `UmamiHistoryPresent` removed, new-referrers site resolution shared with name matching. All verified live post-fix (export downloads a valid ZIP; --top 0 → usage error; --period+--end-at → usage error).

## Out-of-scope / generator retro candidates (full detail)
1. **internal/mcp/intents.go lost by regen merge** — `generate --force` "novel-only preservation" merge dropped the generated `intents.go` (and `recipe_intents_test.go`) while `tools.go` kept its `RegisterIntents` call → `go build ./...` broke. The generator's own quality gates passed because they run against the staging tree before the merge. Restored by hand as a documented no-op (spec declares no intents). File against regen-merge.
2. **`--quiet` help text vs behavior** (root.go template): says "bare output, one value per line" but suppresses all payload output.
3. **Freshness Contract rendered lists** (README/SKILL template): registration-map keys presented as invocable commands (admin-users, me-websites, per-resource `search` variants don't resolve).
4. **Env-var table** marks alternative credentials (UMAMI_TOKEN / UMAMI_API_KEY) both "Required: Yes".
5. (From Phase 3, already logged in build log): filters object-default not expressible in spec; auth set-token generated but never registered; hand AddCommand wiring lost on regen.

## Surface-to-user findings
None — no scope shrinkage, no competing-fix tradeoffs; everything was mechanical.

## Convergence outcome
Round 1: 14 in-scope findings, all fixed. Round 2 (combined-lens re-review): validated all round-1 fixes sound, found 3 residuals — (1) error: new-referrers windowEnd over-extended one day on midnight-aligned periods (fixed: exclusive end from `EndMs-1`); (2) warning: isGoogleDomain still prefix-matched lookalikes (fixed: TLD-bounded regex + .google.com suffix); (3) warning: watch/new-referrers day keys were UTC while snapshot buckets in --timezone (fixed: matching --timezone flags on both readers, default UTC). All three verified by build + tests + live runs. Findings cleared at round 2.
