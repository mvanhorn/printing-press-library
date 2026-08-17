# Productive.io CLI — Absorb Manifest

## Ecosystem scan (Step 1.5a)
Every tool that touches this API today:
- **berwickgeek/productive-mcp** (npm, MCP) — projects, tasks, time entries, people, deals. PM-oriented.
- **druellan/Productive-Simple-MCP** (MCP) — read-focused; projects/folders/tasks/time/people/pages; JSON+TOON output.
- **laurkee/productive-mcp** (Codeberg, MCP) — ticket/task management bridge.
- **BenEdgeContra/productive-client** (JS) — thin v2 API client wrapper (generic request interface, auth, pagination).
- **Official Productive MCP connector** (the thing we're replacing) — full domain, but token-limited on bulk pulls.

**Finding:** No existing tool is a CLI, and none targets the **financial / revenue-recognition** domain. All are task/PM/time oriented. The financial resources (invoices, line_items, invoice_attributions, financial_item_reports) are effectively unserved by a purpose-built tool. Our differentiator is uncontested.

## Absorbed (match or beat everything relevant that exists)
Scope: financial/revenue-reconciliation domain (deliberate — see brief). Read-only.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List deals/budgets w/ filters | productive-client, MCPs | `productive-pp-cli deals list` | filter stage_type/company/status/dates; `--json`/`--csv`; paginates to exhaustion; SQL-queryable offline |
| 2 | Get deal/budget by id | productive-client | `(generated endpoint) deals get` | typed, `--select` |
| 3 | List invoices w/ filters | MCPs | `productive-pp-cli invoices list` | filter type/status/company/date; `--csv` |
| 4 | Get invoice by id | productive-client | `(generated endpoint) invoices get` | typed |
| 5 | List line items (paginated) | official MCP (15–20 id batches) | `productive-pp-cli line-items list` | **free pagination, no token-limit batching** — the core MCP pain removed |
| 6 | List invoice attributions | (none purpose-built) | `productive-pp-cli invoice-attributions list` | budget↔invoice join, `--csv` |
| 7 | List payments | (none) | `productive-pp-cli payments list` | AR data |
| 8 | Auth via token + org headers | productive-client | `(behavior in productive-pp-cli doctor)` | validates `X-Auth-Token` + `X-Organization-Id` live, cents/currency note |
| 9 | JSON:API filter/sort/page passthrough | productive-client | `(behavior in productive-pp-cli deals list)` | root `--filter`/`--sort`/`--page-size`, max 200 enforced |
| 10 | Financial item report (raw) | (none) | `(generated endpoint) reports financial-item` | `group` + `filter` passthrough |
| 11 | Invoice report | (none) | `(generated endpoint) reports invoice` | grouped invoice aggregates |
| 12 | Line item report | (none) | `(generated endpoint) reports line-item` | grouped line-item aggregates |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|------------------|
| 1 | Recognized revenue by budget × month | `recognized-revenue --from 2026-01-01 --to 2026-06-30` | hand-code | Wraps `/reports/financial_item_reports` with `group=budget,date:month`, date-range filter, `financial_item_type` selector, and report rate-limit throttle. No wrapper/MCP models this report; it's the pipeline's headline input. | Use for recognized (earned) revenue per budget per month. For invoiced/billed amounts use `invoiced`; to compare the two use `reconcile`. |
| 2 | Invoiced by budget × month (net, finalized) | `invoiced --from 2026-01-01 --to 2026-06-30` | hand-code | Same report with `financial_item_type=invoice_attribution` + `total_invoiced` (net, finalized only). Distinguishes finalized vs `total_draft_invoiced`. | Net finalized invoiced amount attributed per budget per month. Drafts excluded (shown separately). Not the same as recognized revenue — use `reconcile` to compare. |
| 3 | Reconcile recognized vs invoiced | `reconcile --from 2026-01-01 --to 2026-06-30` | hand-code | Local join of recognized-revenue and invoiced series per budget×month with signed deltas + threshold flags. The entire revenue-reconciliation workflow purpose in one command; impossible via a single API call. | The reconciliation itself: recognized − invoiced per budget×month, flag rows over `--threshold`. |
| 4 | Raw CSV export for the reconciliation workflow | `export --resource line_items --from ... --to ... > raw.csv` | hand-code | Streams full paginated pulls to CSV shaped for `revenue-reconciliation`'s `raw_productive_data/*.csv`. Removes the MCP token ceiling entirely. | Bulk raw export (invoices, line_items, invoice_attributions, deals) to CSV for the reconciliation workflow. |
| 5 | Offline sync + SQL + search | `sync` / `sql` / `search` | spec-emits + hand-code | Pull once into local SQLite, then query offline — sidesteps the 10-req/30s report cap and lets the pipeline re-run without re-hitting the API. | Local mirror of financial resources; `sql` for arbitrary SELECT, `search` for FTS over invoice/deal/line-item text. |
| 6 | AR aging / unpaid invoices | `aging --as-of 2026-07-04` | hand-code | Buckets `amount_unpaid` by days past `pay_on` from local or live data — an accounts-receivable view no PM-focused tool offers. | Unpaid invoice amounts bucketed by age (0-30/31-60/61-90/90+). |

Transcendence rows: **6 planned, 6 shipping-scope; hand-code count = 5** (rows 1–4, 6). Row 5 is mostly framework (`sync`/`sql`/`search` emitted) + light wiring. No stubs.

## Notes / risks for the gate
- **Report rate limit (10 req/30s)** is the tightest constraint; report commands throttle + prefer offline store for re-runs.
- Exact `group=budget,date:month` raw-wire syntax confirmed on first live call (Phase 5); commands built to make adjustment trivial.
- All money is integer cents — CLI divides by the currency subunit ratio on human/CSV output, preserves raw cents in `--json`.
- Read-only tool: no create/update/delete commands (data-pull purpose).
