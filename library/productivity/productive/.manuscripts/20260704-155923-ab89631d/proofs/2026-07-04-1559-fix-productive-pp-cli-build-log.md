# Productive CLI — Phase 3 build log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built
- **Generation (Phase 2):** 38 resources (list + get) + 11 report types under `reports`, 85 endpoints. All gates passed (go vet, govulncheck, build, doctor). MCP Cloudflare pattern auto-applied (search+execute).
- **Auth enrichment (post-gen, durable edits):**
  - `internal/config/config.go`: added `ProductiveOrganizationId` field, `PRODUCTIVE_ORGANIZATION_ID` env load, `applyOrgHeader()` → injects `X-Organization-Id` into `Config.Headers` so every generated + hand-built request carries both required headers.
  - `internal/cli/doctor.go`: added org-id presence check.
  - `internal/client/client.go`: dry-run preview now renders `Config.Headers` (so `X-Organization-Id` shows in `--dry-run`).
- **Novel commands (hand-authored, implemented bodies):**
  1. `recognized-revenue` — financial_item_reports grouped budget×period, date range, `--financial-item-type` (default service).
  2. `invoiced` — same report, `financial_item_type=invoice_attribution`, `total_invoiced`.
  3. `reconcile` — joins recognized + invoiced series by budget×period, signed delta, `--threshold` flagging. **Headline command.**
  4. `aging` — scans unpaid invoices, buckets `amount_unpaid` by age past `pay_on`.
  5. `sync` (+`sql`/`search`) — framework-provided (SQLite mirror + FTS). Novel stub skipped by generator (collision), which is correct.
  6. `export` — framework-provided (auto-paginated JSONL/JSON). Novel stub skipped (collision); research.json description aligned to JSONL/JSON reality.
  - Shared helper `internal/cli/productive_reports.go` (JSON:API parse, report pagination with 3s report-rate throttle, budget-name resolution via include).
- **Generic write surface (hand-authored):** `create` / `update` (PATCH) / `delete` in `internal/cli/productive_write.go` — build the JSON:API `{data:{type,attributes,relationships}}` envelope from `--set`/`--set-json`/`--rel`/`--data`, send `application/vnd.api+json`. Fills the generator's PATCH + envelope gap; works across all resource types.
- **Tests:** `internal/cli/productive_reports_test.go` — table tests for relID, rowMoneyCents, metaInt, flattenReportRow, buildJSONAPIBody.

## Deferred / notes
- Exact `group=budget,date:month` raw-wire syntax and report row field placement (attributes vs relationships) to be confirmed on first live call (Phase 5); helpers tolerate missing keys and are trivial to tune.
- Money kept as integer cents throughout; `--json` preserves cents, `--csv`/human output can divide by currency subunit downstream.
- No generator limitations blocked the build beyond the known PATCH/two-header gaps, both handled by hand-code.
