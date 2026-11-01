## Customer model

**Persona 1: Small-business owner-operator (one hourly employee)**
- **Today (without this CLI):** Each week she opens the employee's timesheet and the trip log, then opens app.gusto.com on the Pay page. An agent with the hosted Gusto MCP can set the regular hours. It cannot add reimbursements, so the mileage gets worked out by hand (miles x rate) and typed into the draft payroll. The brief records this happening on 2026-10-22 and 2026-10-24. She cannot tell whether the draft now matches the timesheet without going through the review screen line by line.
- **Weekly ritual:** Find the open regular payroll, enter that week's hours and any mileage or trip reimbursements, calculate, read back gross, net and debit, then click Submit herself.
- **Frustration:** Mileage reimbursements are the one input an agent cannot enter, so the most error-prone step (arithmetic plus a description) is still done by hand.

**Persona 2: Small-business bookkeeper running weekly or biweekly payroll**
- **Today (without this CLI):** They work from the client's timesheet export, with the Gusto pay dashboard open in one tab and payroll history in another. They retype hours and reimbursements, calculate, and compare the debit to last period from memory or a spreadsheet. They cannot quickly answer "why is this week's debit $180 higher than last week's?"
- **Weekly ritual:** Prepare the draft from the client's inputs, check it against the source numbers and the previous period, then hand it to the owner to submit before the deadline.
- **Frustration:** The read-back check is all manual. Gusto has no screen that says "the draft disagrees with what you meant to enter" or "this period moved this much against the last one."

**Persona 3: The agent acting for the owner (Claude Code session driving gusto-pp-cli)**
- **Today (without this CLI):** It calls the hosted MCP's `update_payroll` for hours. The MCP documentation says "reimbursements cannot be edited here", so the agent has to stop and ask the human to type mileage. It has no structured read-back to show the human before they submit.
- **Weekly ritual:** Take a timesheet and trip log from the human, prepare the draft end to end, and report totals in a form the human can approve in one look.
- **Frustration:** It cannot finish the prep in one pass, and it cannot prove that what it entered matches the source inputs.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description |
|---|------|---------|-------------|---------|--------|------------------|
| C1 | Mileage reimbursement | `payrolls mileage --employee <name> --miles 5.2 --rate 0.60 [--note "Client visit"]` | Works out miles x rate and writes one reimbursement line with the description "Mileage 5.2 mi x $0.60" through AutoSaveReimbursements. A client UUID keyed on the line makes re-runs idempotent. | P1, P3 | (a), (e) | Use this command to turn a mileage figure into a reimbursement. Do NOT use it for a non-mileage reimbursement with a fixed amount; use 'payrolls reimburse add' instead. Do NOT use it to load a whole week's inputs; use 'payrolls prep' instead. |
| C2 | One-shot weekly prep | `payrolls prep --from week.csv [--rate 0.60] [--dry-run]` | Reads a timesheet/trip file (employee, hours, miles, other reimbursements). Then it finds the open draft (PayPageAdmin), sets hours (AutoSaveWorkedHours + MarkCompanyTouchedHours), writes reimbursements (AutoSaveReimbursements), runs CalculateDraftPayroll + PollPayrollSubmissionStatus, and prints the PayrollReviewStep table. It never submits. `--dry-run` prints the planned writes only. | P1, P2, P3 | (e), (a) | Use this command to load a full week of hours and reimbursements onto the open draft and calculate it. Do NOT use it to check a draft that is already prepared; use 'payrolls verify' instead. Do NOT use it for a single mileage line; use 'payrolls mileage' instead. |
| C3 | Draft read-back verify | `payrolls verify --from week.csv` | Re-reads the draft (PayrollInputStepDraftEmployeePayrollsByIds + PayrollReviewStep) and diffs it against the source file: hours per employee, the reimbursement lines and their sum, and duplicate reimbursement descriptions. It also flags a draft that is uncalculated or stale. Exits non-zero on a mismatch so an agent can gate on it. | P2, P3, P1 | (a), (b) | Use this command to prove the current draft matches the source timesheet before a human submits. Do NOT use it to write changes; use 'payrolls prep' instead. Do NOT use it to compare against a previous period; use 'payrolls compare' instead. |
| C4 | Period-over-period compare | `payrolls compare [--against last]` | Puts the calculated draft's review totals (gross, reimbursements, employee taxes, company taxes, net, debit) next to the most recent processed payroll from history summaries and shows the deltas. | P2, P1 | (c) | Use this command to see how this draft's totals moved against the last processed payroll. Do NOT use it to check the draft against the source timesheet; use 'payrolls verify' instead. |
| C5 | Reimbursement history by employee | `payrolls reimbursements-total --employee <name> --since 90d` | Answers "what did we reimburse X this quarter" from synced payroll history in local SQLite. | P2 | (c) | none |
| C6 | Local mileage ledger | `mileage log add` / `mileage bill` | Keeps trips locally, then moves unbilled trips onto the draft as reimbursements and marks them billed. | P1 | (e) | none |
| C7 | Submit-handoff note | `payrolls handoff` | A markdown summary for the human submitter: hours, reimbursements, net, debit, funding account, deadline. | P3, P1 | (b) | none |
| C8 | Hours sanity check | `payrolls hours check` | Flags employees with zero hours, more than 40 hours, or hours outside the expected range on the draft. | P2 | (b) | none |
| C9 | Net-pay what-if | `payrolls what-if --employee X --hours 32` | Previews net pay at different hours using BackgroundNetPayPreviewByIds. | P1 | (b) | none |
| C10 | Deadline countdown | `payrolls deadline` | Time left until the open payroll's deadline. | P1, P2 | (b) | none |
| C11 | Bonus entry | `payrolls bonus set --employee X --amount 50` | Sets a one-time bonus on the draft. | P1 | (e) | none |
| C12 | Description normalizer | `reimbursements describe` | Picks a canonical reimbursement description using the suggestions endpoint. | P3 | (b) | none |

Inline kill/keep checks:
- **C11, auth/capture gap:** bonus and fixed-compensation edits were not captured. Cut.
- **C9, verifiability:** a preview probably needs the draft to be saved first, which would mutate the draft just to answer a question. Cut.
- **C12, thin wrapper:** this duplicates absorbed row #9 and leans toward an LLM-style choice. Cut.
- **C6, scope creep:** the user already keeps the trip log outside the CLI. The double-billing guard is better handled by C1 idempotency plus C3's duplicate check. Cut.
- **C10:** this is absorbed row #7 (`payrolls due` already shows deadlines). Cut.

## Survivors and kills

Pass 3 answers per survivor:

**C1 `payrolls mileage`**
1. **Weekly use:** Yes. P1 has trips most weeks, and this is the exact gap from 2026-10-22 and 2026-10-24.
2. **Wrapper vs leverage:** Not a thin rename. It adds the miles x rate arithmetic, a deterministic description, and client-UUID idempotency on top of AutoSaveReimbursements.
3. **Transcendence proof:** It is a service-specific content pattern (mileage at a per-mile rate) that the hosted MCP cannot write.
4. **Sibling kill:** C6, the local mileage ledger. It rebuilds a log the user already keeps.
5. **Buildability:** `hand-code`, with `// pp:data-source live`.
6. **Long description:** Valid. It points to `payrolls reimburse add` (absorbed #4) and `payrolls prep` (survivor).

**C2 `payrolls prep`**
1. **Weekly use:** Yes. This is Top Workflow 1 and the core of the User Vision.
2. **Wrapper vs leverage:** It chains six captured operations and parses a local input file.
3. **Transcendence proof:** It gives an agent-shaped end-to-end workflow with a hard stop at review, so a human always submits.
4. **Sibling kill:** C11, bonus entry. That operation was not captured. Bonus support can join prep once the operation is captured.
5. **Buildability:** `hand-code`, with `// pp:data-source live`.
6. **Long description:** Valid. It points to `payrolls verify` and `payrolls mileage`.

**C3 `payrolls verify`**
1. **Weekly use:** Yes. Every prep should end with it, and P3 gates its handoff on the exit code.
2. **Wrapper vs leverage:** It compares source-file intent with live draft state. No single endpoint does this.
3. **Transcendence proof:** It joins a local input file with the live draft and review, and its pass/fail exit code is agent-shaped.
4. **Sibling kill:** C8, the hours sanity check. Its useful part (hours that do not match the source) is covered here. Hard-coded thresholds would be speculative.
5. **Buildability:** `hand-code`, with `// pp:data-source live`.
6. **Long description:** Valid. It points to `payrolls prep` and `payrolls compare`.

**C4 `payrolls compare`**
1. **Weekly use:** Yes for P2, who checks every period. It is plausible for P1.
2. **Wrapper vs leverage:** It joins the draft review with processed history. Neither endpoint gives deltas.
3. **Transcendence proof:** It is a cross-source join (PayrollReviewStep with PayrollHistoryContainerPayrollSummaries), and it can read synced history from local SQLite. It calls `hintIfUnsynced`/`hintIfStale` on payroll history and uses the drain-first scan.
4. **Sibling kill:** C5, reimbursement history by employee. The captured history summaries may not carry per-employee reimbursement lines.
5. **Buildability:** `hand-code`, with `// pp:data-source auto`.
6. **Long description:** Valid. It points to `payrolls verify`.

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|---------|------------------|
| 1 | Mileage reimbursement | `payrolls mileage --employee <name> --miles 5.2 --rate 0.60` | 10/10 (D3 P3 F2 R2) | hand-code | Uses AutoSaveReimbursements (updateDraftPayroll with amount, description and client UUID) to write a reimbursement of miles x rate, described "Mileage N mi x $R", with no external dependencies. Re-runs are idempotent through a deterministic client UUID. | User Vision: mileage was typed into app.gusto.com by hand on 2026-10-22 and 2026-10-24. The hosted MCP's update_payroll says "Deductions and reimbursements cannot be edited here". Top Workflow 2. | P1 owner-operator, P3 agent | Use this command to turn a mileage figure into a reimbursement. Do NOT use it for a non-mileage reimbursement with a fixed amount; use 'payrolls reimburse add' instead. Do NOT use it to load a whole week's inputs; use 'payrolls prep' instead. |
| 2 | One-shot weekly prep | `payrolls prep --from week.csv [--rate 0.60] [--dry-run]` | 9/10 (D3 P3 F1 R2) | hand-code | Uses PayPageAdmin, AutoSaveWorkedHours, MarkCompanyTouchedHours, AutoSaveReimbursements, CalculateDraftPayroll, PollPayrollSubmissionStatus and PayrollReviewStep to load a local timesheet onto the open draft and print the review totals. It never calls submit and has no external dependencies. | Brief Build Priority 3 ("payroll prep ... guarded so submit needs an explicit flag"), Top Workflow 1, and User Vision "stop at review. A human always clicks Submit." | P1 owner-operator, P2 bookkeeper, P3 agent | Use this command to load a full week of hours and reimbursements onto the open draft and calculate it. Do NOT use it to check a draft that is already prepared; use 'payrolls verify' instead. Do NOT use it for a single mileage line; use 'payrolls mileage' instead. |
| 3 | Draft read-back verify | `payrolls verify --from week.csv` | 8/10 (D3 P2 F2 R1) | hand-code | Uses PayrollInputStepDraftEmployeePayrollsByIds and PayrollReviewStep to diff per-employee hours, reimbursement lines and sums against the source file. It flags duplicates and an uncalculated draft, and exits non-zero on a mismatch, with no external dependencies. | Top Workflow 1 ("read back gross/net/debit before a human submits"). The User Vision asks for read-back of totals. | P2 bookkeeper, P3 agent, P1 owner-operator | Use this command to prove the current draft matches the source timesheet before a human submits. Do NOT use it to write changes; use 'payrolls prep' instead. Do NOT use it to compare against a previous period; use 'payrolls compare' instead. |
| 4 | Period-over-period compare | `payrolls compare [--against last]` | 6/10 (D2 P2 F1 R1) | hand-code | Uses PayrollReviewStep totals for the draft and PayrollHistoryContainerPayrollSummaries (live, or synced to local SQLite) for the last processed payroll. It computes deltas in gross, reimbursements, taxes, net and debit with no external dependencies. | Top Workflow 3 (payroll review with totals across processed and unprocessed payrolls). The brief's Data Layer lists payroll history by period. | P2 bookkeeper, P1 owner-operator | Use this command to see how this draft's totals moved against the last processed payroll. Do NOT use it to check the draft against the source timesheet; use 'payrolls verify' instead. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C5 Reimbursement history by employee | The captured history summaries are not shown to carry per-employee reimbursement lines, so the query cannot be built from captured operations. | `payrolls compare` |
| C6 Local mileage ledger | It rebuilds a trip log the user already keeps elsewhere. Double-billing is covered by mileage idempotency and verify's duplicate check. | `payrolls mileage` |
| C7 Submit-handoff note | It repeats `payrolls review` output. `--json`/`--agent` on review already gives the agent a handoff payload. | `payrolls prep` |
| C8 Hours sanity check | Mismatches against the source are covered by verify, and fixed hour thresholds would be speculative policy. | `payrolls verify` |
| C9 Net-pay what-if | A preview probably requires saving hours to the draft first, which mutates real payroll state just to answer a question. It cannot be verified safely. | `payrolls prep --dry-run` |
| C10 Deadline countdown | Duplicates absorbed `payrolls due`, which already shows deadlines. | `payrolls prep` |
| C11 Bonus entry | The bonus and fixed-compensation edit operation was not captured, so this has no buildable write path. | `payrolls prep` |
| C12 Description normalizer | It is a thin rename of the absorbed reimbursement-suggestions endpoint and implies a judgment call an LLM would make. | `payrolls mileage` |