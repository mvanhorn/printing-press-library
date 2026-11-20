# Round 3 mutation evidence

The builder authored both implementation and regression tests. Proofs ran through
`mutation-verify.sh` in a `git archive HEAD` scratch copy (no git inside the copy, copies deleted).
Each accepted proof used `--expect-fail-output`, one exact `-run` row plus an identical `-skip`
knock-out with `--knockout-delta-line '=== RUN '`, and `go build ./internal/cli` as the syntax check.

| Item | Hazard mutation | Matched failure text | Control mutation (own failure text) |
|---|---|---|---|
| 1 guard | drop `&& !indeterminate` from the all-miss check | `MARKET_SELECT_EMPTY_REJECTED` (row `shops_empty_store_row`) | `m["notes"]` emptied; fails `MARKET_SELECT_EMPTY_HINT` |
| 1 case fold | `strings.ToLower` removed from select paths | `MARKET_SELECT_CASE: --select matched no fields` (row `Results_shop_id`) | `m["results"]` forced empty; fails `MARKET_SELECT_CASE_ROWS` |
| 1 warning | stderr `valid fields:` warning removed | `MARKET_SELECT_NO_VALID_FIELDS` (row `shops`) | `usageErr` replaced by a plain error; fails `MARKET_SELECT_TYPO_EXIT` |
| 2 skip "; " | `add` skips any note containing "; " | `EMPTY_HINT_NOT_IN_NOTES` (row `tags`) | hint placeholder back to `<SEARCH TERM>`; fails `HINT_ESCAPING_BROKEN` |
| 2 union | `else if !composite { add(note) }` emptied | `NOTES_RESUME_UNION` | `mktEmitReport` returns an error; fails `CTRL_EMIT_BROKEN` |
| 3 resume | resume clause made unconditional (`if true`) | `--resume 0` (row `Partial_false`) | "take another snapshot" reworded; fails `take a further snapshot` |
| 4 conflict | `ON CONFLICT ... WHERE` clause removed | `CROSS_PAGE_OWNERSHIP: page=2` | page DELETE made a no-op; fails `owner removal count=` |
| 5 eligibility | per-term review-data check made always true | `has review data; use --snapshot 1, or re-run market snapshot` (row `DoneOnly_false`) | hint reworded `--SNAPSHOT`; fails `use --SNAPSHOT` |

All eight hazard mutations were CAUGHT; every knock-out exited 0 on the mutant and removed
exactly one `=== RUN ` line and the hazard text; every control went RED for its own reason.

Refused configurations (recorded, then corrected):

- Item 1 case fold: expected text `MARKET_SELECT_CASE_NOTES` was absent (the mutant fails earlier,
  at the exit check, with `MARKET_SELECT_CASE:`); expected text changed to the real failure.
- Item 3: control `need at least TWO ...` was INERT (the row does not assert that prefix);
  control moved to the hint tail.
- Item 4: control `result_count + 1` was INERT (no assertion reads it); control moved to the page DELETE.
- Item 5: first expected text `TERM_ALT_ELIGIBILITY: "no review data ...; snapshot` was REFUSED as not
  specific to the hazard (the control printed it too); expected text narrowed.
