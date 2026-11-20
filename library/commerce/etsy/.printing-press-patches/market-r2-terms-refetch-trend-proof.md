# Round 2 items 6–8 mutation evidence

The builder authored both implementation and regression tests. Proofs ran through
`mutation-verify.sh` in a `git archive HEAD` scratch copy with only the current
diff overlaid; no git commands ran inside the copy. Each accepted proof used
`--expect-fail-output`, an exact single-test `-skip` knockout with
`--knockout-delta-line '=== RUN '`, and `go build ./internal/cli` as the syntax check.

| Area | Hazard mutation | Failure phrase | Control mutation and outcome |
|---|---|---|---|
| C-TERMS | Replace `not_fetched` state with `absent` | `NOT_FETCHED_BRANCH_BROKEN` | Replace `shown` term state with `absent`; shown-at-ten test fails with `, want shown`, hazard test passes. Accepted. |
| Re-fetch | Make the DELETE predicate always false | `STALE_RESULTS_REMAIN` | Store `result_count + 1`; existing page-outcome test fails with `result_count=`, hazard test passes. Accepted. |
| Trend | Treat every snapshot count as empty | `TREND_PARTIAL_HINT_MISSING` | Return an error on the empty-store branch; empty-store test fails with `empty store should exit 0:`, hazard test passes. Accepted. |

All three hazard mutations were caught, and each knockout removed exactly one
`=== RUN ` line and eliminated its hazard phrase. All three controls failed for
their own reason without printing the hazard phrase.

Two initial control configurations were refused by the proof runner:

- Demand: the selected control needle `10 reviews:` already appeared in the
  passing test's logging. Replaced with assertion-only `, want shown`.
- Re-fetch: the selected control needle `count=` appeared in the main hazard
  failure. Replaced with `result_count=` and a control that changes only the
  API count stored in the page-outcome row.

Final proof logs and the runner are retained in the scratch directory
`/var/folders/3q/ck_d52b55ldchx3kbgybdxyw0000gn/T/mktfix-proof.MIjbfk`.
