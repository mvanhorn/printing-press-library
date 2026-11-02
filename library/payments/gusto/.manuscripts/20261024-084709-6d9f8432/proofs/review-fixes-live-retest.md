# Review fixes: live re-test (gusto-pp-cli)

Covers the fixes for PR review findings (summed same-description reimbursements, unique full-name matching, fixed-schedule-only next period, debit-component review settling, surfaced save rejections).

- Date: 2026-10-26, live Gusto account, fresh signed-in session (`doctor`: credentials valid).
- Sandbox: the open regular draft payroll for the next pay period, empty before the test (0 hours, no reimbursements). One hourly employee ("the test employee").
- No payroll was submitted or approved.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `payrolls due --agent` | 0 | weekly schedule inferred; period_end 2026-11-01 |
| 2 | `payrolls show --agent` | 0 | 0 hours, reimbursements [] |
| 3 | `payrolls prep --stdin --agent` (stdin: two entries `{"amount":0.01,"description":"PP test parking"}` for the test employee, hours 0) | 0 | one saved line of 0.02; mismatches []; calculated review total_debit 0.02, debit_breakdown_total 0.02 |
| 4 | `payrolls verify --stdin --agent` (same stdin) | 0 | mismatches [] (expected 0.02, saved 0.02) |
| 5 | `payrolls review --agent` | 0 | settled: total_debit 0.02 = debit_breakdown_total 0.02 |
| 6 | `payrolls reimburse remove --employee <test employee> --description 'PP test parking' --agent` | 0 | removed 1 |
| 7 | `payrolls show --agent` | 0 | 0 hours, reimbursements [] |

Cleanup: step 6 removed the test line; step 7 confirms the draft is back to its starting state.
Unit tests added for each finding: TestSameDescriptionReimbursementsAreSummed, TestDuplicateFullNameNeedsUUID, TestOnlyFixedSchedulesPredictNextPeriod, TestReviewConsistent (check-paid cases), TestUpdateDraftPayrollErr.
Exit codes as captured: 7 of 7 commands exited 0.
