# Gusto CLI Brief

## API Identity
- Domain: US small-business payroll, HR and benefits (Gusto, formerly ZenPayroll).
- Users: small-business owners and bookkeepers who run payroll weekly or biweekly; agents acting for them.
- Data profile: companies -> employees -> jobs/compensations; pay schedules -> payrolls -> employee_compensations (hourly_compensations, fixed_compensations, reimbursements, deductions, taxes); recurring reimbursements; contractors and contractor payments.
- Spec: official OpenAPI 3.1, "Gusto Embedded API" version 2025-06-15, 211 paths, servers https://api.gusto.com (prod) and https://api.gusto-demo.com (demo). Source used: Gusto/gusto-typescript-client `.speakeasy/logs/changes/new.openapi.yaml` (HTTP 200, 1,629,773 bytes). Upstream input Gusto/Gusto-Partner-API `generated/embedded/api.v2026-06-15.embedded.yaml` returned HTTP 404 (private repo). Copy saved as `gusto-embedded-openapi-2025-06-15.yaml` in this research dir.

## Reachability Risk
- Medium (access, not breakage). Auth is OAuth2 bearer (`CompanyAccessAuth`, `SystemAccessAuth`); `/oauth/token` issues access_token + refresh_token with `expires_in`.
- Docs (docs.gusto.com/app-integrations/docs/authentication, HTTP 200): "Due to the sensitive nature of payroll, all potential integrations must be vetted and approved by Gusto." Production credentials only after a Production Pre-Approval application and QA. "Gusto is unable to provide production test accounts. All testing ... must take place in our demo environment."
- docs.gusto.com/app-integrations/docs/production-access: HTTP 404.
- No 403/blocked/deprecated issues found on Gusto/gusto-typescript-client.

## Top Workflows
1. Weekly hourly payroll prep: find the open (unprocessed) regular payroll, prepare it, set Regular Hours per employee, add one-time reimbursements (mileage), add a bonus, then calculate and read back gross/net/debit before a human submits.
2. Mileage reimbursement from a log: miles x IRS or company rate -> one reimbursement line with a description ("Mileage 5.2 mi x $0.60").
3. Payroll review: list payrolls (processed/unprocessed) with totals; per-employee breakdown of hours, bonus, reimbursements, taxes, net, check amount.
4. Recurring reimbursements: list/create/update/delete an employee's recurring reimbursement (phone stipend, etc.).
5. Blockers and deadlines: list payroll blockers and the payroll_deadline for each open period.

## Table Stakes
- Companies, employees, jobs/compensations, pay schedules, payrolls list/get/prepare/update/calculate, payroll blockers, earning types, recurring reimbursements, contractors and contractor payments (read).
- Competitors: Gusto's hosted MCP connector (claude.ai "Gusto"): list/get/update payrolls, but update_payroll states "Deductions and reimbursements cannot be edited here", and run_payroll is blocked for hourly workers. Community MCPs: BusyBee3333/gusto-mcp-2026-complete (100+ tools), agenticledger/gusto-mcp-http, Savinda96/gusto-mcp. SDKs: @gusto/embedded-api 0.16.3 (npm), Gusto/gusto-python-client, Gusto/gusto-java-client. No Gusto CLI exists in printing-press-library (registry 595 entries, no match; blocked-apis.json no match).

## Data Layer
- Primary entities: payrolls (with totals and employee_compensations), employees, recurring reimbursements.
- Sync cursor: payroll pay_period start/end dates; processed vs unprocessed.
- FTS/search: employee name search; payroll history by period for "what did we pay X in reimbursements this quarter".

## User Vision
- From the operator session (a small business with one hourly employee): enter weekly hours and trip/mileage reimbursements onto a Draft payroll from a timesheet, read back totals, and stop at review. A human always clicks Submit. Real need surfaced 2026-10-22 and 2026-10-24: the hosted Gusto MCP could set hours and bonus but not reimbursements, so mileage was typed into app.gusto.com by hand.

## Product Thesis
- Name: gusto-pp-cli.
- Why it should exist: the one thing agents cannot do with the hosted Gusto MCP is edit reimbursements on a draft payroll. The official API supports it (`employee_compensations[].reimbursements[] {amount, description, uuid}` on PUT /v1/companies/{company_id}/payrolls/{payroll_id}). A CLI that does payroll prep end to end, with mileage math and a read-back, and that never submits by default, closes that gap.

## Build Priorities
1. `payrolls list/get/prepare/update/calculate` with reimbursements, hours and fixed compensations.
2. Novel: `payroll reimburse` (one-time reimbursement add/replace with description) and `payroll mileage --miles --rate` math.
3. Novel: `payroll prep` = prepare + hours + reimbursements + calculate + review table, guarded so submit needs an explicit flag.
4. Recurring reimbursements CRUD, payroll blockers, employees list.
5. OAuth (demo and prod) token handling with refresh.

## Runtime Decision (added after Phase 1.6/1.7, 2026-10-24)
- User chose the signed-in web session runtime over the official OAuth API, because the official API needs Gusto's production approval before it can touch a real company.
- Runtime: Chrome-compatible HTTP (Surf; Cloudflare blocks stdlib) to https://graphql.app.gusto.com/?operationName=<Op>, with the app.gusto.com session from press-auth, plus x-csrf-token and x-role-id read from an authenticated app.gusto.com page.
- Captured operations: PayrollInputStep, PayrollInputStepDraftEmployeePayrollsMetadata, PayrollInputStepDraftEmployeePayrollsByIds, BackgroundNetPayPreviewByIds, ReimbursementDescriptionSuggestions, AutoSaveReimbursements, AutoSaveWorkedHours, MarkCompanyTouchedHours, CalculateDraftPayroll, PollPayrollSubmissionStatus, PayrollReviewStep, PayPageAdmin, PayrollHistoryContainerPayrollSummaries.
- Not captured yet: bonus/fixed-compensation edits, employee directory, off-cycle payrolls, submit (submit is deliberately out of scope).

## Reachability Gate
- Decision: PASS (browser-sniff exception, mode browser_http)
- Evidence: probe-reachability app.gusto.com: stdlib 403 (cf-mitigated challenge), surf-chrome 200. graphql.app.gusto.com GET: 400 on both transports (GraphQL rejects GET; not a challenge page). Surf POST PayrollHistoryContainerPayrollSummaries with the app.gusto.com session: 200, no errors, 5982 bytes (2026-10-24).
- Generate will pass --traffic-analysis so the CLI ships Chrome-compatible transport.
