# payrolls write lifecycle proof (gusto-pp-cli)

Covers: `payrolls mileage`, `payrolls prep`, `payrolls verify` (exit 0 needs a draft that matches the timesheet, so it is state-dependent), plus the reads `payrolls reimburse list` and `payrolls show`.

- Date: 2026-10-24, live Gusto account, operator-approved.
- Sandbox: the open regular draft payroll for the next pay period, which had 0 hours and no reimbursements before the test. One hourly employee ("the test employee"). Test data: a $0.01 mileage line (0.02 mi x $0.50).
- No payroll was submitted or approved; no command in this CLI can submit.

## Commands, verbatim, with exit codes

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `gusto-pp-cli payrolls mileage --employee <test employee> --miles 0.02 --rate 0.50 --dry-run --agent` | 0 | preview only |
| 2 | `gusto-pp-cli payrolls mileage --employee <test employee> --miles 0.02 --rate 0.50 --agent` | 0 | saved: true, reimbursement_total 0.01 |
| 3 | `gusto-pp-cli payrolls reimburse list --employee <test employee> --agent` | 0 | one line "Mileage 0.02 mi x $0.50" |
| 4 | `gusto-pp-cli payrolls verify --stdin --rate 0.50 --agent` (stdin `[{"employee":"<test employee>","hours":0,"miles":0.02}]`) | 0 | mismatches [] |
| 5 | `gusto-pp-cli payrolls prep --stdin --rate 0.50 --agent` (same stdin) | 0 | no changes needed, calculated review net_pay 0.01, submitted false |
| 6 | `gusto-pp-cli payrolls reimburse remove --employee <test employee> --description 'Mileage 0.02 mi x $0.50' --agent` | 0 | removed 1, saved true |
| 7 | `gusto-pp-cli payrolls show --agent` | 0 | 0 hours, reimbursements [] |
| 8 | `gusto-pp-cli payrolls verify --stdin --agent` (stdin `[{"employee":"<test employee>","hours":0}]`) | 0 | mismatches [] |

## Cleanup

Step 6 removed the test line. Steps 7 and 8 confirm the draft is back to its starting state (0 hours, no reimbursements). Earlier in the build, the same lifecycle also ran for `payrolls hours set` and for a mileage correction (one line kept), each restored the same way.

Exit codes as captured: 8 of 8 commands exited 0.
