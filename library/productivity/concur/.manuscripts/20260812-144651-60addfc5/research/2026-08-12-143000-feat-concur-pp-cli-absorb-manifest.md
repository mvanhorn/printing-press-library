# Concur CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Get current user profile/policies/delegate list | concur-mcp `concur_whoami` (source) | (generated endpoint) profile whoami | Cached locally, offline re-read |
| 2 | List usable expense types for policy | concur-mcp `concur_list_expense_types` (source) | (generated endpoint) expense-types list | SQLite-cached, searchable |
| 3 | Get dynamic form fields for an expense type | concur-mcp `concur_get_expense_form_fields` (source) | (generated endpoint) expense-types fields | Per-field guidance surfaced in --help |
| 4 | Get report-header form fields for a policy | concur-mcp `concur_get_report_form_fields` (source) | (generated endpoint) reports form-fields | none |
| 5 | Get valid values for a list-type field | concur-mcp `concur_get_list_values` (source) | (generated endpoint) lists values | Cached, offline lookups by code |
| 6 | List payment types | concur-mcp `concur_list_payment_types` (source) | (generated endpoint) payment-types list | none |
| 7 | List attendee types | concur-mcp `concur_list_attendee_types` (source) | (generated endpoint) attendee-types list | none |
| 8 | Search locations catalog | concur-mcp `concur_search_locations` (source) | (generated endpoint) locations search | Local FTS over synced cache |
| 9 | Create expense report | concur-mcp `concur_create_report` (source); prior-art `ConcurClient.create_report` | `concur-pp-cli reports create --name --start --end --purpose` | `--dry-run`, `--json`, direct deep-link URL echoed |
| 10 | Update report name/business purpose | concur-mcp `concur_update_report` (source) | (generated endpoint) reports update | none |
| 11 | Create expense inside report | concur-mcp `concur_create_expense` (source) | `concur-pp-cli expenses create` | Returns filled-field manifest |
| 12 | Update/fill expense fields (core v3 PUT + custom v4 PATCH) | concur-mcp `concur_update_expense_fields` (source) | `concur-pp-cli expenses update` | Single command routes to correct verified write path |
| 13 | Add attendees to expense (merge-preserve) | concur-mcp `concur_add_attendees` (source) | `concur-pp-cli expenses attendees add` | none |
| 14 | Remove attendees (replace-POST semantics) | concur-mcp `concur_remove_attendees` (source) | `concur-pp-cli expenses attendees remove` | none |
| 15 | Attach receipt image/PDF to expense | concur-mcp `concur_attach_receipt` (source) | `concur-pp-cli expenses receipt attach` | none |
| 16 | List reports (filterable) | concur-mcp `concur_list_reports` (source); prior-art | `concur-pp-cli reports list` | SQLite sync, offline filter/search |
| 17 | Get report header + expenses + deep link | concur-mcp `concur_get_report` (source) | `concur-pp-cli reports get <id>` | none |
| 18 | Get single expense with field manifest | concur-mcp `concur_get_expense` (source) | (generated endpoint) expenses get | none |
| 19 | Get expense attendees | concur-mcp `concur_get_expense_attendees` (source) | (generated endpoint) expenses attendees list | none |
| 20 | Search attendee catalog (exact + fuzzy) | concur-mcp `concur_search_attendees` (source) | `concur-pp-cli attendees search` | Local fuzzy cache |
| 21 | List delegators + permission flags | concur-mcp `concur_list_delegators` (source) | (generated endpoint) delegates list | none |
| 22 | Act on behalf of a delegator | concur-mcp (source, applies to every tool) | global `--on-behalf-of` flag on reports/expenses commands | none |
| 23 | List "Available Expenses" queue (unfiled card charges/e-receipts) | prior-art `count_available_expenses`/`AvailableExpensesPage` (source, proven against live tenant) | `concur-pp-cli available-expenses list` | SQLite sync, count/search offline |
| 24 | Move available expenses onto a report | prior-art `add_available_expenses` (source, proven) | `concur-pp-cli available-expenses move --to-report` | Idempotent, safe to re-run |
| 25 | Per-expense-type business-purpose default + reimbursement-cap split | prior-art `apply_expense_type_rules`/`ExpenseTypeRule` (source, proven, config-driven) | `concur-pp-cli expenses apply-rules --config expense_types.json` | Ported verbatim from proven business logic; safe to re-run (skips already-set fields) |
| 26 | Submit report for approval | prior-art `submit_report` (source, proven); concur-mcp deliberately omits this | `concur-pp-cli reports submit <id>` | Guarded (`--confirm` required), unlike concur-mcp which never submits |
| 27 | List trips/itineraries | Official docs Travel v4 Itinerary, Trip v1.1; Tevasoft/sap-concur-connector | `concur-pp-cli trips list` | SQLite sync |
| 28 | Get itinerary/booking detail | Official docs Booking v1.1, Itinerary v4 | `concur-pp-cli trips get <id>` | none |
| 29 | Travel allowance / per-diem lookup | Official docs Travel Allowance v4 | `concur-pp-cli travel-allowance get` | none |
| 30 | Travel request (pre-trip authorization) list/get | Official docs Request v4 | `concur-pp-cli requests list` / `requests get <id>` | none |
| 31 | Traveler profile + loyalty programs | Official docs Travel Profile v2 | (generated endpoint) profile travel | none |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|-------------------|
| 1 | Pre-submit policy validator | `reports validate <id>` | hand-code | 9/10 | Cross-references the report's expenses against expense-type rules (business-purpose requirements, reimbursement caps) and form-field requirements *before* hitting Concur's own submit validation — catches the exact failure prior-art users hit ("Missing required field: Business Purpose") without a round-trip rejection. | Use this command to check a report before submitting. Do NOT use it to submit; use 'reports submit' instead. |
| 2 | Trip/expense reconciliation | `trips reconcile --trip <id>` | hand-code | 8/10 | Requires a local join between synced trip date ranges and the available-expenses queue — no single Concur API call correlates "which unfiled charges belong to which trip." | none |
| 3 | Auto-link available expenses to a trip's report | `available-expenses link-to-trip --trip <id>` | hand-code | 8/10 | Same join as #2, but mutating — moves the matched expenses onto the trip's report in one command instead of manual drag-and-drop triage. | Use this command to file a specific trip's charges. Do NOT use it for a blanket monthly sweep; use 'available-expenses move' instead. |
| 4 | Approver budget-flag digest | `approvals list --summary` | hand-code | 7/10 | Aggregates pending-approval reports with computed budget/exception flags (missing receipts, over-cap items, policy exceptions) in one scan — the web UI requires opening each report individually to see this. | none |
| 5 | Cross-report duplicate-charge detector | `expenses scan-duplicates` | hand-code | 7/10 | Requires a local SQLite scan across all of a user's synced expenses (vendor + amount + date proximity) — Concur's own UI has no cross-report duplicate view. | none |
| 6 | Bulk expense tagging | `expenses tag --category --project` | hand-code | 6/10 | Loops a category/project-code update across every matching unfiled expense in one invocation instead of one-at-a-time UI edits. | none |

All 6 transcendence features are **hand-code** (~50-150 LoC each plus
`root.go` wiring) — see "Correction note" in the novel-features-brainstorm
audit trail for why 3 of these were reclassified from the subagent's
`spec-emits` tag.

## Killed candidates

| Candidate | Kill Reason |
|---|---|
| Receipt Inbox Sync (OCR) | External OCR dependency — too much maintenance overhead for a generated CLI. |
| Delegate Session Audit | High false-signal risk; Concur's own audit logs are authoritative, a local CLI log would be incomplete. |
| Report Dashboard | Scope creep — covered by `reports get` + `--select`/`--json`. |
| Rule-set Audit | Low utility relative to `reports validate`. |

## Crowd-Sniff / Ecosystem Sources Consulted

- `Tevasoft/sap-concur-connector` (GitHub, Python, OAuth2, 1 star, active) —
  Expense Reports v4, Travel, Users, Financial Integration coverage.
- `bharath2020/concur-mcp` (GitHub, TypeScript MCP, OAuth2, 0 stars, updated
  11 days before this run) — 21-tool catalog, read/write/discovery/delegate
  tools, deepest field-level ground truth found (form fields, list values,
  attendee replace-semantics, verified write paths). Primary source for the
  Absorbed table above.
- `CDataSoftware/sap-concur-mcp-server-by-cdata` (GitHub, Java, 2 stars) —
  read-only generic JDBC-driver bridge; not endpoint-specific, low marginal
  value beyond confirming OAuth2/read-only framing.
- `@browser-automation-hub/sap-concur-browser-automation` (npm) and
  `Nilesunknowing346/sap-concur-browser-automation` (GitHub) — two
  independent browser-automation projects targeting the same
  individual-user-can't-get-API-access gap this CLI addresses.
- Private prior art: `expense-report-filer` +
  `magnite-playwright-okta-auth` (local, not public) — proven end-to-end
  workflow and business-rule logic; primary source for the "Available
  Expenses" and rule-engine features above.
