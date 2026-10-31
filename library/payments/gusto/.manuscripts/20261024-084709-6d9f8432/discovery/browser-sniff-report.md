# Gusto browser-sniff report (run 20261024-084709-6d9f8432)

## 1. User Goal Flow
- Goal: add a one-time (mileage) reimbursement and regular hours to a Draft payroll, then read back the review totals. Never submit.
- Steps completed (Draft payroll Oct 26 - Nov 1, 2026, which was empty before and restored to empty after):
  1. Open payroll inputs page -> PayrollInputStep, PayrollInputStepDraftEmployeePayrollsMetadata, PayrollInputStepDraftEmployeePayrollsByIds, BackgroundNetPayPreviewByIds.
  2. Open employee reimbursements drawer -> ReimbursementDescriptionSuggestions.
  3. Add one-time reimbursement ($1.23, test description), save -> AutoSaveReimbursements (mutation updateDraftPayroll).
  4. Set Regular hours to 1 -> AutoSaveWorkedHours, MarkCompanyTouchedHours.
  5. Review summary -> CalculateDraftPayroll, PollPayrollSubmissionStatus, PayrollReviewStep (gross, reimbursements, taxes, net, debit).
  6. Go back, clear hours -> AutoSaveWorkedHours; delete reimbursement row, save -> AutoSaveReimbursements (empty itemized list).
  7. Pay dashboard -> PayPageAdmin; Pay history -> PayrollHistoryContainerPayrollSummaries.
- Steps skipped: Submit payroll (out of scope by design).
- Coverage: 7 of 7 planned steps.

## 2. Pages & Interactions
- /<company>/payroll_admin/pay (dashboard) -> nothing due.
- /<company>/payroll_admin/pay/payrolls/scheduled/regular.<company>.<schedule>.2026-11-01/inputs -> clicked Reimbursements cell, "Add a one-time reimbursement", typed description + 1.23, Save; edited Regular hrs cell to 1, Tab.
- .../review -> read totals; "Go back".
- .../inputs -> cleared hours; reimbursements drawer, Delete, Save.
- Breadcrumb "Pay" -> dashboard; "View pay history" -> /pay/history/payrolls.

## 3. Browser-Sniff Configuration
- Backend: chrome-MCP (user's running Chrome, fresh capture tab) after agent-browser auto-connect failed (no remote-debugging port). In-page fetch+XHR interceptor; export via a user-approved Blob download, then identifier scrub.
- Pacing: user-paced UI interactions only; no agent-initiated replays during capture.
- Proxy pattern: not a proxy envelope. Single GraphQL endpoint https://graphql.app.gusto.com/?operationName=<Op>.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| POST | graphql.app.gusto.com /?operationName=PayrollInputStep | 200 | application/json | auth-required |
| POST | ...=PayrollInputStepDraftEmployeePayrollsMetadata | 200 | application/json | auth-required |
| POST | ...=PayrollInputStepDraftEmployeePayrollsByIds | 200 | application/json | auth-required |
| POST | ...=BackgroundNetPayPreviewByIds | 200 | application/json | auth-required |
| POST | ...=ReimbursementDescriptionSuggestions | 200 | application/json | auth-required |
| POST | ...=AutoSaveReimbursements | 200 | application/json | auth-required |
| POST | ...=AutoSaveWorkedHours | 200 | application/json | auth-required |
| POST | ...=MarkCompanyTouchedHours | 200 | application/json | auth-required |
| POST | ...=CalculateDraftPayroll | 200 | application/json | auth-required |
| POST | ...=PollPayrollSubmissionStatus | 200 | application/json | auth-required |
| POST | ...=PayrollReviewStep | 200 | application/json | auth-required |
| POST | ...=PayPageAdmin | 200 | application/json | auth-required |
| POST | ...=PayrollHistoryContainerPayrollSummaries | 200 | application/json | auth-required |

## 5. Traffic Analysis
- Protocols: graphql (0.92).
- Auth signals (names only): session cookie on .app.gusto.com (httpOnly, session, random hex name); request headers x-csrf-token and x-role-id; apollographql-client-name. CSRF comes from <meta name="csrf-token"> and role id from "role_id": in the HTML of an authenticated app.gusto.com page.
- Protection: Cloudflare challenge on stdlib HTTP (cf-mitigated: challenge). probe-reachability: mode browser_http (Surf Chrome TLS clears it). The analyzer's standard_http guess comes from in-browser traffic only and is overridden by the probe.
- Warnings: none from the analyzer. Mutations require a client-generated payrollActivitySessionId (UUID) and, for itemized reimbursements, a client-generated employeeReimbursementId (UUID).
- Candidate commands: payroll list/show, reimbursements list/set/remove, hours set, payroll calculate + review.

## 6. Coverage Analysis
- Exercised: draft payroll inputs, reimbursements, hours, calculate/review, pay history, pay dashboard.
- Missed: bonus/fixed compensation mutation (not exercised), off-cycle payrolls, employees list outside payroll context, recurring reimbursements, submit (deliberately excluded).

## 7. Response Samples
- Stored scrubbed in browser-sniff-capture.json (names, company/employee UUIDs, bank last-4 replaced with deterministic fakes).

## 8. Rate Limiting Events
- None observed. No 429s.

## 9. Authentication Context
- Authenticated session used (the operator's signed-in Chrome via chrome-MCP).
- Cookie replay (Step 2d), Surf Chrome transport, run 2026-10-24:
  - On-disk Chrome cookies (pycookiecheat): FAIL, identical with and without cookies; the app session cookie is memory-only.
  - press-auth capture under domain gusto.com: FAIL, the .app.gusto.com session cookie does not domain-match gusto.com and was dropped; replay redirected to login.gusto.com Keycloak.
  - Cookies from the live controlled window filtered to app.gusto.com scope (.gusto.com + .app.gusto.com): PASS. GET /pay 200 with csrf + role_id; GraphQL PayrollHistoryContainerPayrollSummaries 200, no errors, 5982 bytes.
  - Without cookies: redirect to login.gusto.com (unauthenticated).
- Verdict: cookie auth works when captured for domain app.gusto.com. Generated CLI auth: press-auth login app.gusto.com.
- Session state excluded from manuscripts: no cookie values were written to disk by the agent; the controlled-window cookies were piped to the validator only.
