# Zoho Expense CLI Absorb Manifest

## Source Tools Surveyed

| Tool | Coverage |
|---|---|
| **Ramp CLI** (github.com/ramp-public/ramp-cli) — gold standard for agent-friendly expense CLI | Full surface: `accounting`, `bills`, `funds`, `purchase_orders`, `receipts {upload, attach}`, `reimbursements`, `requests`, `transactions {list, get, approve, edit, missing, flag-missing, memo-suggestions, trips}`, `travel`, `users {me, search, org-chart}` |
| **schmorrison/Zoho (Go)** | Partial Expense coverage; multi-Zoho-product wrapper; no India helper |
| **tdesposito/pyZohoAPI** | Expense on roadmap, not yet shipped |
| **Airbyte source-zoho-expense (PR #47406)** | Read-only: expenses, reports, users, categories, currencies, approval_history |
| **dltHub zoho-expense** | Read-only: organizations, expensereports, expensereport_detail, expenses_summary, users, currencies, expense_categories, approval_history |
| **Zoho first-party SDKs** | **No Expense SDK published** (CRM/Books/Inventory/Subscriptions/Desk/Creator only) |
| **Zoho MCP** | **No Zoho Expense MCP** (Books only) |
| **Competing CLIs for Zoho Expense** | **None found.** Greenfield. |

Total absorbed features (drawn from Ramp CLI surface + Zoho native API + Airbyte/dltHub read patterns): **27**.
Novel/transcendence features (Hermes-first workflow design): **6**.

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | OAuth login | Ramp CLI, Zoho self-client docs | `zoho-expense-pp-cli auth login --client-id=... --client-secret=... --code=...` | India-region self-client flow; persists refresh token; auto-refreshes access token |
| 2 | Token refresh | Ramp CLI | `zoho-expense-pp-cli auth refresh` | Force-refresh from refresh token; transparent refresh on 401 |
| 3 | Auth status | Ramp CLI `users me` | `zoho-expense-pp-cli auth status` | Token validity + active org + region |
| 4 | List organizations | Zoho API | (generated endpoint) organizations list | Cached; org_id auto-discovery on first auth |
| 5 | Get organization | Zoho API | (generated endpoint) organizations get | |
| 6 | Switch active organization | Ramp-style org context | `zoho-expense-pp-cli org use <id>` | Persist org_id in config; used as the header for every subsequent request |
| 7 | Current user (me) | Ramp `users me` | (generated endpoint) users me | Header `users/me` resolves the authenticated identity |
| 8 | List users | Ramp `users search`, dltHub | (generated endpoint) users list | Page/per_page; cached locally |
| 9 | Get user | Zoho API | (generated endpoint) users get | |
| 10 | Invite/activate/deactivate user | Zoho API | (generated endpoint) users invite/activate/deactivate | |
| 11 | List expenses with filters | Ramp `transactions list`, Airbyte, dltHub | (generated endpoint) expenses list | Filter by date range, status, user, category, project, merchant, customer; JSON-by-default when piped |
| 12 | Get a single expense | Ramp `transactions get` | (generated endpoint) expenses get | |
| 13 | Create expense (JSON) | Zoho API | (generated endpoint) expenses create | Tax-inclusive, billable, project, customer, merchant_name in one call |
| 14 | Update expense (tagging) | Zoho API, Ramp `transactions edit` | (behavior in zoho-expense-pp-cli expense tag) | Wraps the generated update endpoint with ergonomic flags: `--category`, `--project`, `--customer`, `--tag name=option`, `--billable`, `--gst-inclusive` |
| 15 | Merge expenses (dedupe) | Zoho API | (generated endpoint) expenses merge | Used by `receipt upload --dedupe` and `invoice ingest` |
| 16 | Upload receipt for autoscan | Ramp `receipts upload` | `zoho-expense-pp-cli receipt upload <file>` | multipart POST + polling loop until autoscan_status=Processed; returns the populated expense; **the Hermes hot path** |
| 17 | List untagged / scanned expenses | Ramp `transactions missing` | `zoho-expense-pp-cli expense untagged` | Local query: scanned receipts awaiting category/project |
| 18 | List expense categories | Zoho API, Airbyte, dltHub | (generated endpoint) expense_categories list | Synced offline |
| 19 | CRUD expense categories | Zoho API | (generated endpoint) expense_categories create/update/delete/enable/disable | |
| 20 | List reporting tags + options | Zoho API | (generated endpoint) reporting_tags list, list_options | The user's tagging schema — critical for auto-tag |
| 21 | CRUD reporting tags | Zoho API | (generated endpoint) reporting_tags create/update/delete/activate/deactivate | |
| 22 | CRUD projects | Zoho API | (generated endpoint) projects list/get/create/update/delete/activate/deactivate | |
| 23 | CRUD customers | Zoho API | (generated endpoint) customers list/get/create/update/delete | |
| 24 | List expense reports | Ramp `bills list`, Airbyte | (generated endpoint) expense_reports list | Filter by status |
| 25 | CRUD reports + approve/reject/reimburse | Ramp `bills approve/reject`, Zoho API | (generated endpoint) expense_reports create/update/approve/reject/reimburse + approval_history | |
| 26 | CRUD trips | Zoho API | (generated endpoint) trips list/get/create/update/delete/approve/reject/cancel/close | |
| 27 | Currencies + taxes | Zoho API | (generated endpoint) currencies + taxes CRUD | India GST configuration |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---|---|---|---|
| 1 | **Invoice batch ingest with hash dedup + auto-tag** (Hermes hot path) | `invoice ingest <dir>` | hand-code | Walks a directory, SHA256-hashes each file against the local store, skips already-uploaded, posts new ones in parallel, polls autoscan for all, and auto-tags from learned merchant→category/project map. No competitor wraps the upload-poll-tag-dedup loop because no competitor has a local store. |
| 2 | **Merchant→tag memory** trained from local history | `merchant list`, `merchant map <merchant>` | hand-code | Zoho only auto-categorizes a merchant after the org has seen + categorized it once. We bypass this by training the mapping locally from sync history and pre-populating tags on first-sight merchants via the generated PUT /expenses/{id} call. |
| 3 | **Receipt hash gate** (perceptual + exact) | (behavior in `receipt upload`) | hand-code | SHA256 of receipt file + cached at upload time in SQLite. `receipt upload` refuses duplicate hashes (`--force` to override). Solves Zoho's weak server-side dedup. |
| 4 | **Monthly close**: bundle + audit + submit in one shot | `close --month=YYYY-MM` | hand-code | Lists unreported expenses for the month, flags any still-processing autoscans, flags untagged items, creates an expense report named "{Month YYYY}", attaches everything, optionally auto-approves. One command for what the Zoho web UI takes 8 clicks to do. |
| 5 | **GST/CGST/SGST split** for Indian expenses | `gst-split <expense_id>` | hand-code | India-specific: parses `tax_id` and line items, computes CGST/SGST/IGST shares (typically 9/9 for intra-state, 18 IGST for inter-state), updates the expense or emits CSV. Only relevant to .in customers and only possible with local enrichment. |
| 6 | **Untagged-expense audit + auto-fix** | `expense untagged --auto-fix` | hand-code | Lists expenses missing `category_id` or `project_id`, applies the merchant→tag memory map, PUTs back to Zoho. The thing the user actually wants to run at month-end before `close`. |

---

## Skipped / Not Building

- **Advances API** — undocumented public surface; plan-gated (Premium only); not in Hermes workflow.
- **Mileage GPS sync** — mobile-app only; irrelevant for invoice ingestion.
- **Per-diem rates admin** — undocumented public CRUD; org-admin scope only.
- **Custom field definitions admin** — undocumented public CRUD; would need browser-sniff.
- **Approval workflow definitions** — undocumented public CRUD.
- **Submit-for-approval endpoint** — undocumented; covered by `expense_reports update` body field changes.
