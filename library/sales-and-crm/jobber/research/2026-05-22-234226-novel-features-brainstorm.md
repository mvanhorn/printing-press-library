# Novel Features Brainstorm — jobber-pp-cli

## Customer model

**Persona 1 — Mark (Fractional CFO / M&A advisor at Bouvier Advisory Partners)**

*Today (without this CLI):* Mark logs into the client's Jobber web UI through a borrowed seat, exports CSVs one report at a time (AR aging, invoice list, payment list, jobs report), drops them into `\BAP LLC\...\Heritage\` and pivots them in Excel. Each CSV has different column shapes and date formats. He has Jobber + QBO + Outlook tabs open simultaneously and copy-pastes invoice numbers between them. He cannot answer "which invoices touched both a payment and a credit memo in the last 90 days" without a manual VLOOKUP marathon.

*Weekly ritual:* Wednesday morning, pull last week's AR delta for Heritage, identify newly aged-out invoices, cross-reference against QBO deposits, write a 1-page client memo. Repeat on Friday for the second client tenant.

*Frustration:* The Jobber UI reports are pre-formatted and unfilterable on the dimensions that matter (e.g., "open AR by tag + invoice age bucket"). Every analytical question requires a fresh export and a fresh pivot. He cannot get a stable snapshot to diff week-over-week.

**Persona 2 — Linda (Office manager / bookkeeper at a 25-tech home services co)**

*Today (without this CLI):* Linda runs Jobber daily for scheduling and invoicing. For accounting close she exports the Payment Records report and the Invoice report monthly and emails them to the outside accountant. When the accountant asks "why is this invoice still open when the client says they paid?" Linda searches the Jobber UI client-by-client, opens each invoice, checks the payments tab, and screenshots it.

*Weekly ritual:* Friday afternoon — reconcile the week's deposits against Jobber Payments payouts, flag mismatched amounts, queue questions for the accountant.

*Frustration:* The payout-to-invoice trace is buried three clicks deep per record and there's no bulk view that ties payout → payment record → invoice → client in one screen.

**Persona 3 — Sam (Operations manager at a multi-crew field-service company)**

*Today (without this CLI):* Sam wants to know which jobs lost money last quarter. He exports the Jobs report (revenue), the Timesheet report (labor hours), and the Expenses report separately, then joins them in Excel by job number — except job numbers don't appear consistently across all three exports, so the join breaks and he gives up halfway.

*Weekly ritual:* Monday standup — review last week's completed jobs, flag underwater ones, identify stuck/unscheduled work for the dispatcher.

*Frustration:* Cannot get job-level P&L without a 45-minute Excel session. Cannot easily list "jobs with no visits scheduled in the next 14 days" or "quotes approved but never converted to a job."

## Candidates (pre-cut)

C1 ar aging | C2 invoices trace | C3 payouts reconcile | C4 jobs pnl | C5 jobs stale | C6 visits unscheduled | C7 funnel | C8 quotes approved-unconverted | C9 snapshot diff | C10 clients rollup --tag | C11 invoices mismatched | C12 cost-report | C13 snapshot save | C14 clients 360 | C15 users utilization | C16 audit export

(Full Pass 2 table in subagent output above; pre-cut folded C6→C5, C8→C7, C11→C2, C12→sync flag, C13→C7 flag.)

## Survivors (8)

1. ar aging — 9/10, hand-code
2. invoices trace — 8/10, hand-code
3. payouts reconcile — 8/10, hand-code
4. jobs pnl — 9/10, hand-code
5. jobs stale — 7/10, hand-code
6. funnel — 7/10, hand-code
7. snapshot diff — 8/10, hand-code
8. clients 360 — 8/10, hand-code

## Killed candidates

- C6 unscheduled — folded into C5 `--no-future-visits`
- C8 approved-not-converted — folded into C7 `--stage stuck`
- C10 tag rollup — Tag reachability unconfirmed; risk for v1
- C11 invoice-payment mismatch — folded into C2 `--mismatched`
- C12 cost-report — sync flag, not novel
- C13 snapshot save — `snapshot diff --save` flag
- C15 labor utilization — user pay rate not confirmed in schema
- C16 audit export — monthly cadence; composable from sync + sql + shell
