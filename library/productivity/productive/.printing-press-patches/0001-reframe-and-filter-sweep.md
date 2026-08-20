# Patch 0001 — Reframe positioning + broaden list/report filter coverage

**Date:** 2026-07-06
**Author:** Derick Ng
**Reprint-guard:** reapply the intent below after any `printing-press generate --force` /
reprint. All filter param names were **live-validated** against the Productive REST API
(unsupported candidates were pruned — see "Rejected" notes).

---

## Part 1 — Positioning copy (was misdescribed as finance-only)

The generated description framed the CLI as *"Fast, paginated Productive.io **financial**
pulls … built for a revenue-recognition pipeline"*. It is actually a **full Productive.io v2
API client** (list/get across ~48 resources, generic create/update/delete, 11 report types,
local sync/search/export/tail) with revenue-recognition commands as value-adds on top.

Canonical description (propagate to every metadata surface):

> Full Productive.io v2 API client — list/get across ~48 resource types, generic
> create/update/delete, 11 report types, and a local sync/search/export/tail data layer,
> plus first-class revenue-recognition commands (recognized-revenue, invoiced, reconcile,
> aging).

Surfaces updated (all carried the old string): `.printing-press.json` (`description`),
`internal/cli/root.go` (`Short`/`Long` + "Highlights"→"Value-add commands"),
`manifest.json`, `tools-manifest.json`, `spec.json` (top-level `description`), `README.md`,
`SKILL.md` (description + body), `internal/cli/agent_context.go`, `internal/mcp/tools.go`.
Also fixed `sync` copy "Mirror **financial** resources" → "Mirror **any** resource" in
README.md, SKILL.md, .printing-press.json, internal/mcp/tools.go, internal/cli/which.go.

## Part 2 — Additive filter sweep (70 new filter flags)

The generator emitted only a subset of the filters the API supports. Added the following as
`endpoints.list.params` (regular resources) / `endpoints.<subtype>.params` (reports) in
`spec.json`, and as cobra flags in the corresponding generated `*_list.go` /
`reports_*.go` files. Pattern per flag: `var flagFilterXxx string` → `if flagFilterXxx != ""
{ params["filter[…]"] = formatCLIParamValue(flagFilterXxx) }` → `cmd.Flags().StringVar(…)`.

| Resource | New flags (→ API filter) |
|---|---|
| bookings | budget→`budget_id`, project→`project_id`, company→`company_id`, event→`event_id`, task→`task_id`, approver→`approver_id`, booking-type→`booking_type`, approval-status→`approval_status`, billing-type→`billing_type_id`, is-draft→`draft`, is-canceled→`canceled` |
| companies | status, tag→`tags[contains]`, company-code, vat→`vat[contains]`, parent-company→`parent_company_id`, project→`project_id`, subscriber→`subscriber_id`, last-activity-after→`last_activity_at[gt_eq]` |
| people | title→`title[contains]`, team→`team`, manager→`manager_id`, subsidiary→`subsidiary_id`, custom-role→`custom_role_id` |
| expenses | budget→`deal_id`, company→`company_id`, project→`project_id`, vendor→`vendor_id`, status, invoicing-status, approval-status |
| todos | deal→`deal_id`, due-from/due-to→`due_date` |
| tasks | start-from/start-to→`start_date`, closed-after→`closed_at[gt_eq]`, task-number, subscriber→`subscriber_id` |
| projects | workflow→`workflow_id`, archived-after→`archived_at[gt_eq]` |
| holidays | calendar→`holiday_calendar_id`, date-from→`after[gt_eq]`, date-to→`before[lt_eq]` |
| memberships | person→`person_id`, project→`project_id`, deal→`deal_id` |
| contact_entries | company→`company_id`, person→`person_id`, invoice→`invoice_id`, contactable-type→`contactable_type` |
| activities | task→`task_id`, deal→`deal_id`, project→`project_id`, person→`person_id`, creator→`creator_id`, date-from→`after[gt_eq]`, date-to→`before[lt_eq]` |
| reports time-entry | project→`project_id`, company→`company_id`, service→`service_id`, billing-type→`billing_type_id`, status, billable-min→`billable_time[gt_eq]` |
| reports booking | person→`person_id`, budget→`budget_id`, project→`project_id`, booking-type→`booking_type`, approval-status→`approval_status`, billing-type→`billing_type_id` |

### Param-name gotchas (REST ≠ MCP abstraction field names — verified live)
- bookings: MCP exposes `is_draft`/`is_canceled` but the REST list endpoint rejects them;
  the accepted params are **`draft`** and **`canceled`**.
- people: `team_id` is rejected; the accepted param is **`team`**.
- tasks: `subscriber_ids` is rejected; the accepted param is **`subscriber_id`** (singular).
- Relationship filters use `filter[<rel>_id][eq]` (consistent with the prior
  "Fix relationship filters" commit).

### Rejected (API returned "Unsupported filter" — deliberately NOT added)
- memberships: `membership_type`, `access_type` (rejected on the list endpoint).
- Not attempted / left off: invoices (already 11 filters, not re-audited); a `--department`
  people filter (an org-specific custom-field id — belongs in the report group-by,
  not a hardcoded generic flag).

## Verification performed
`go build ./... && go test ./...` (pass), `gofmt -l` clean, `go vet` clean. Every added
filter exercised live (against a real `PRODUCTIVE_ORGANIZATION_ID`): returns HTTP 200 and demonstrably
narrows results (e.g. `bookings --booking-type event` → absences only; `companies --status 2`
→ archived only; `reports time-entry --billing-type 3` → non-billable only).
