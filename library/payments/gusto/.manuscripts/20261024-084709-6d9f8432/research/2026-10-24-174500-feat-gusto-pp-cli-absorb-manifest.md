# gusto-pp-cli Absorb Manifest

Runtime: Gusto web GraphQL (graphql.app.gusto.com) with the user's app.gusto.com session (press-auth), Chrome-compatible HTTP. Scope is limited to what the captured web operations can reach. Official-API-only features (employee CRUD, contractors, benefits, time off, company setup) are out of scope for this runtime; they are not stubs.

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List payrolls (processed) with totals | Gusto hosted MCP list_payrolls | gusto-pp-cli payrolls history | --json/--select, synced to SQLite for compare |
| 2 | Show one draft payroll: hours, reimbursements per employee | Gusto hosted MCP get_payroll | gusto-pp-cli payrolls show | Draft inputs in one table, agent-shaped |
| 3 | Set regular hours on a draft | Gusto hosted MCP update_payroll | gusto-pp-cli payrolls hours set | --dry-run, read-back of the saved value |
| 4 | Add one-time reimbursements with description | none (hosted MCP cannot; official API PUT reimbursements[]) | gusto-pp-cli payrolls reimburse add | Only tool that can do it on the web runtime; idempotent |
| 5 | List and remove reimbursements on a draft | official API GET payroll reimbursements | gusto-pp-cli payrolls reimburse list | remove via gusto-pp-cli payrolls reimburse remove |
| 6 | Calculate and read review totals (gross, reimbursements, taxes, net, debit) | hosted MCP get_payroll (calculated) | gusto-pp-cli payrolls review | Calculates, polls, prints debit and funding account; never submits |
| 7 | What is due and deadlines | hosted MCP list_payrolls(unprocessed) | gusto-pp-cli payrolls due | Company, schedule and deadline in one call |
| 8 | Reimbursement description suggestions | Gusto web app | gusto-pp-cli reimbursements suggestions | Sends the CSRF and role headers the web API needs |
| 9 | Net pay preview per employee | Gusto web app | gusto-pp-cli payrolls net-pay-preview | Resolves the draft and employees itself; no hand-written variables |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Mileage reimbursement | payrolls mileage | 10/10 | hand-code | miles x rate arithmetic, deterministic description and idempotent client UUID on top of the web reimbursement mutation the hosted MCP cannot call | Use this command to turn a mileage figure into a reimbursement. Do NOT use it for a non-mileage reimbursement with a fixed amount; use 'payrolls reimburse add' instead. Do NOT use it to load a whole week's inputs; use 'payrolls prep' instead. |
| 2 | One-shot weekly prep | payrolls prep | 9/10 | hand-code | chains due -> hours -> reimbursements -> calculate -> review from a local timesheet file, hard stop before submit | Use this command to load a full week of hours and reimbursements onto the open draft and calculate it. Do NOT use it to check a draft that is already prepared; use 'payrolls verify' instead. Do NOT use it for a single mileage line; use 'payrolls mileage' instead. |
| 3 | Draft read-back verify | payrolls verify | 8/10 | hand-code | joins the source file with the live draft and review; non-zero exit on mismatch so agents can gate the human handoff | Use this command to prove the current draft matches the source timesheet before a human submits. Do NOT use it to write changes; use 'payrolls prep' instead. Do NOT use it to compare against a previous period; use 'payrolls compare' instead. |
| 4 | Period-over-period compare | payrolls compare | 6/10 | hand-code | joins draft review totals with synced processed-payroll history | Use this command to see how this draft's totals moved against the last processed payroll. Do NOT use it to check the draft against the source timesheet; use 'payrolls verify' instead. |

### Not in scope / gaps
- Bonus edits: the web mutation was not captured (killed C11). Real need: a $100 bonus was entered on 2026-10-22. Can be added after capturing the bonus mutation.
- Submit: deliberately excluded. A human always submits in app.gusto.com.

### Revision 2026-10-24 (Phase 4)
- Rows 8-9 moved from generated endpoints to hand-built commands. The generated GraphQL endpoints could not send x-csrf-token/x-role-id and their graphql.app.gusto.com base_url bypassed the verify mock. Same features, same scope. The spec now carries one generated cookie-only endpoint: `session` (GET /api/user_authentication_events).
