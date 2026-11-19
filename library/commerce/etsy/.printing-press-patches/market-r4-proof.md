# Round 4 mutation evidence

The builder authored implementation and regression tests (codex). Proofs ran through `mutation-verify.sh`
in a `git archive HEAD` scratch copy (no git inside the copy, copies deleted). Each proof used
`--expect-fail-output`, one exact `-run` row plus an identical `-skip` knock-out with
`--knockout-delta-line '=== RUN '`, `go build ./internal/cli` as syntax check, and a control mutation
with its own failure text.

| Item | Hazard mutation | Matched failure text (row) | Control mutation (own failure text) |
|---|---|---|---|
| 1 notes exemption | drop `&& !notesRequested` from the all-miss check | `MARKET_SELECT_NOTES_ONLY_REJECTED` (`TestMarketSelectEverySubcommand/shops_notes`) | select-path `m["notes"]` gets an extra entry; fails `MARKET_SELECT_NOTES_ONLY_VALUE` |
| 2 space join | summary note joined with `" "` instead of `"; "` | `SUMMARY_NOTE_SEPARATOR` (`TestMarketReportCompositeNoLeak`) | resume_hint reworded `--RESUME`; fails `COMPOSITE_NOTES_EXACT` |
| 2 ordering | composite computed after snapshot_note/resume_hint/error (r3 order) | `COMPOSITE_NOTES_EXACT` (`TestMarketReportCompositeNoLeak`) | final recompute joined with `", "`; fails `COMPOSITE_NOTE_JOIN` |
| 4 WHERE removed | upsert `WHERE` clause deleted | `CROSS_PAGE_OWNERSHIP_BEST_POSITION` (`TestMarketCrossPageOwnership/in_response_duplicate`) | page DELETE table misspelled; fails `CROSS_PAGE_OWNERSHIP_FETCH` |
| 4 two-arm WHERE | r3 form `excluded.page = market_results.page OR excluded.position < ...` | `CROSS_PAGE_OWNERSHIP_BEST_POSITION` (`in_response_duplicate`) | same control; fails `CROSS_PAGE_OWNERSHIP_FETCH` |
| 5 key list | `selectFieldKeys(nil)` in the all-miss warning | `MARKET_SELECT_WARNING_KEYS: stderr` (`TestMarketSelectTypoPopulatedValidFields/shops`) | `usageErr` replaced by plain error; fails `MARKET_SELECT_TYPO_EXIT` |
| 6 oldest wins | `if declared[term] && alternatives[term] == 0` -> `if declared[term]` | `REVIEW_ADVICE_NEWEST_PER_TERM` (`TestMarketReviewAdviceSupersetNamed`) | hint reworded `--SNAPSHOT`; fails `review_advice_top_term_coverage` |

All seven hazard mutations were CAUGHT; each knock-out removed the hazard text and exactly one `=== RUN ` line,
and every control went RED for its own reason. Observation: for the 4 WHERE-removed proof the knock-out
command itself exited 1 on the mutant (the verifier accepted it as attributed by the deselected row).

Refused configurations (recorded, then corrected):

- First runner script placed `-v` only on the knock-out command: refused (`--knockout-cmd must start with --test-cmd`); `-v` added to both.
- Item 1 first control (`delete(m, "notes")` before the final emit) went RED with the hazard's own text
  `MARKET_SELECT_NOTES_ONLY_REJECTED` ("control RED for an unrelated reason"); control moved to the select-path notes value.
- Item 5 first mutation (`selectFieldKeys(nil)` alone) left `envelopeRaw` unused: refused as MUTATED FILE INVALID
  (build break); mutation now keeps `_ = envelopeRaw`.
- Item 4 in-response-duplicate is the only row that fails for both WHERE mutants (the better-page row passes without the guard, since the better page wins either way).
