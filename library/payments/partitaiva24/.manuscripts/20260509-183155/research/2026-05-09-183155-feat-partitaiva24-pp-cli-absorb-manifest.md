# Partitaiva24 CLI — Absorb Manifest

## Context
Partitaiva24 has **zero** third-party tooling. There is no community SDK, MCP server, npm/PyPI wrapper, or competing CLI to absorb features from. The "absorb" half of this manifest therefore covers every endpoint the platform itself exposes, organized into command groups. The "transcend" half is where most of the value sits — features that only become possible once data is mirrored to a local SQLite store.

Adjacent prior art was reviewed for cross-pollination ideas: Fatture in Cloud SDKs (PHP/JS/C#/Python), `italia/fatturapa-python`, `taocomp/php-sdicoop-client`. The shapes that map to this API (invoice CRUD, SDI status, F24, corrispettivi, customer registry) are all reflected in the transcendence list below.

## Absorbed (every endpoint reachable on partitaiva24.cloud/api/v1)

| # | Feature | Source | Our Implementation | Added Value |
|---|---------|--------|--------------------|-------------|
| 1 | List invoices | `GET /user/invoices` | `invoices list` + sync into store | Offline FTS, --since / --status / --paid filters |
| 2 | Get invoice | `GET /user/invoices/{id}` | `invoices get <id>` | --json --select for narrowing 30+ fields |
| 3 | Create invoice | `POST /user/invoices` | `invoices create --stdin` | Idempotent via local key, --dry-run shows the request |
| 4 | Update invoice | `PUT /user/invoices/{id}` | `invoices update <id> --patch` | JSON-merge-patch from stdin |
| 5 | Delete invoice | `DELETE /user/invoices/{id}` | `invoices delete <id>` | Confirm prompt unless --yes |
| 6 | Mark invoice paid | `POST /user/invoices/{id}/paid` | `invoices mark-paid <id>` | Bulk via `--ids` CSV |
| 7 | Send invoice PDF | `POST /user/invoices/{id}/sendpdf` | `invoices send-pdf <id>` | Override recipient via --to |
| 8 | Download invoice PDF | `GET /user/invoices/{id}/file` | `invoices file <id> -o <path>` | Streams to file |
| 9 | Transmit to SDI | `POST /user/invoices/{id}/transmit` | `invoices transmit <id>` | Bulk via `--ids` |
| 10 | Sign invoice | `POST /user/invoices/{id}/sign` | `invoices sign <id>` | |
| 11 | SDI notification | `GET /user/invoices/{id}/s_d_i_notification` | `invoices sdi-status <id>` | Used by `sdi watch` (transcend) |
| 12 | List attachments | `GET /user/invoices/{id}/attachment` | `invoices attachments <id>` | |
| 13 | Add attachment | `POST /user/invoices/{id}/attachment` | `invoices attach <id> --file <path>` | |
| 14 | Remove attachment | `DELETE /user/invoices/{id}/attachment` | `invoices detach <id> --attachment-id <n>` | |
| 15 | Invoice skeleton | `GET /user/invoice/skeleton` | `invoices skeleton --json` | Default starter doc shape |
| 16 | Invoice defaults | `GET /user/invoice/defaults` | `invoices defaults --json` | VATs, doc types, pension funds, witholdings |
| 17 | Invoice import | `POST /user/invoices-import` | `invoices import --file <xml>` | |
| 18 | Invoice export | `POST /user/invoices-export` | `invoices export --year 2025 -o file.zip` | |
| 19 | SDI accept (refused docs) | `POST /user/invoices-sdi-accept` | `invoices sdi-accept` | |
| 20 | Invoice stats | `GET /user/invoices-stats` | `invoices stats --json` | Used by `turnover` and `tax-due` |
| 21 | List income invoices | `GET /user/income-invoices` | `income list` | --paid/--unread filters |
| 22 | Get income invoice | `GET /user/income-invoices/{id}` | `income get <id>` | |
| 23 | Mark income paid | `POST /user/income-invoices/{id}/paid` | `income mark-paid <id>` | |
| 24 | Mark income read | `POST /user/income-invoices/{id}/read` | `income mark-read <id>` | Bulk via `--ids` |
| 25 | Income attachment | `GET /user/income-invoices/{id}/attachment/{idx}` | `income attachment <id> <idx> -o file` | |
| 26 | Income import | `POST /user/income-invoices-import` | `income import --file <xml>` | |
| 27 | List customers | `GET /user/customers` | `customers list` + sync | FTS over name/p_iva/codfis |
| 28 | Get customer | `GET /user/customers/{id}` | `customers get <id>` | |
| 29 | Create customer | `POST /user/customers` | `customers create --stdin` | |
| 30 | Update customer | `PUT /user/customers/{id}` | `customers update <id>` | |
| 31 | Delete customer | `DELETE /user/customers/{id}` | `customers delete <id>` | |
| 32 | List corrispettivi | `GET /user/corrispettivi` | `corrispettivi list` | |
| 33 | Corrispettivi draft | `GET /user/corrispettivi/draft` | `corrispettivi draft` | |
| 34 | CRUD corrispettivo | `GET/POST/PUT/DELETE /user/corrispettivi/{id}` | `corrispettivi {get,create,update,delete}` | |
| 35 | Attach corrispettivo | `POST /user/corrispettivi/{id}/attach` | `corrispettivi attach` | |
| 36 | Detach corrispettivo | `DELETE /user/corrispettivi/{id}/detach/{aid}` | `corrispettivi detach` | |
| 37 | List F24 | `GET /user/f24` | `f24 list` + sync | --year / --paid / --archived filters |
| 38 | Mark F24 paid/read/archived | `POST /user/f24/{id}/{paid,read,archive}` | `f24 mark-paid/read/archive <id>` | |
| 39 | F24 ravvedimento request | `POST /user/f24/{id}/ravv_request` | `f24 ravvedimento <id>` | |
| 40 | List foreign-mgmt | `GET /user/foreign-mgmt` | `esterometro list` | Renamed for clarity (Italian tax term) |
| 41 | CRUD foreign-mgmt | `GET/POST/PUT/DELETE /user/foreign-mgmt/{id}` | `esterometro {get,create,update,delete}` | |
| 42 | Mark foreign paid | `POST /user/foreign-mgmt/{id}/paid` | `esterometro mark-paid <id>` | |
| 43 | List docs | `GET /user/docs` | `docs list` + sync | |
| 44 | Mark doc read | `POST /user/docs/{id}/read` | `docs mark-read <id>` | |
| 45 | Doc upload | `POST /user/doc-upload` | `docs upload --file` | |
| 46 | List attachments | `GET /user/attachments` | `attachments list` | |
| 47 | Add attachment | `POST /user/attachments` | `attachments add --file` | |
| 48 | Delete attachment | `DELETE /user/attachment/{id}` | `attachments delete <id>` | |
| 49 | List notifications | `GET /user/notifications` | `notifications list` | Includes badge counts (f24, docs, incoming, tickets) |
| 50 | Broadcast notifications | `GET /user/notifications/broadcast` | `notifications broadcast` | |
| 51 | Get profile | `GET /user/profile` | `profile show --json` | |
| 52 | Update profile | `PUT /user/profile` | `profile update --patch` | |
| 53 | Get fiscal year | `GET /user/fiscal_year/{year}` | `fiscal-year <year>` | |
| 54 | Get settings | `GET /user/settings` | `settings list` | |
| 55 | Update setting | `POST /user/settings/{name}` | `settings set <name>` | |
| 56 | Delete setting | `DELETE /user/settings/{name}` | `settings unset <name>` | |
| 57 | Change password | `POST /user/password` | `password change --stdin` | |
| 58 | List events | `GET /user/events` | `events list` + sync | Used by `f24 ical` (transcend) |
| 59 | Data export request | `GET/POST /user/d-data-request` | `data-request {status,submit}` | GDPR data dump |
| 60 | List tickets | `GET /user/tickets` | `tickets list` | |
| 61 | Ticket categories | `GET /user/tickets/categories` | `tickets categories` | |
| 62 | Get ticket | `GET /user/ticket/{id}` | `tickets get <id>` | |
| 63 | Create / reply ticket | `POST /user/ticket` + `POST /user/ticket/{id}` | `tickets create` / `tickets reply <id>` | |
| 64 | Submit questionario | `POST /user/questionario/{year}` | `questionario submit <year>` | Annual tax questionnaire |
| 65 | Get questionario | `GET /user/questionario/{year}` | `questionario show <year>` | |
| 66 | Questionario config | `GET /user/questionario-config` | `questionario config` | |
| 67 | Attach to questionario | `POST /user/questionario/{year}/attachfile` | `questionario attach <year> --file` | |
| 68 | List subscriptions | `GET /user/subscriptions` | `subscriptions list` | |
| 69 | Subscription gateways | `GET /user/subscription/gateways` | `subscriptions gateways` | |
| 70 | CRUD subscription | `GET/POST/PUT/DELETE /user/subscription/{id}` | `subscriptions {get,update,cancel}` | |
| 71 | VIES VAT check | `GET /tools/check-vies/{cc}/{vat}` | `tools check-vies <cc> <vat>` | Used by `vies bulk` (transcend) |
| 72 | PA registry search | `POST /tools/search-pa/{coduni}` | `tools search-pa <coduni>` | |
| 73 | XML → invoice | `POST /tools/invoice/xml2invoice` | `tools xml2invoice --file` | |
| 74 | PDF sign | `POST /tools/sign` | `tools sign --file` | |
| 75 | Tools ping | `POST /tools/ping` | `doctor` (composed) | Healthcheck path |
| 76 | Tools info | `GET /tools/info` | `doctor` | API version |
| 77 | Tools system info | `GET /tools/system_info` | `tools system-info` | |

**77/77 endpoints absorbed.** No stubs.

## Transcendence (only possible with our local store + agent surface)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | **Forfettario turnover meter** | `turnover [--year]` | Joins live `invoices-stats` with `fiscal_year.turnover_limit` and projects days-to-limit at current run-rate. The platform shows a static % bar; we project. | 9 |
| 2 | **Quarterly tax-due projection** | `tax-due [--quarter Q3]` | Computes IVA/IRPEF/INPS owed at quarter-end from synced invoices + `regime_fiscale` + `tax_rate`. Reads zero endpoints at run time once synced. | 9 |
| 3 | **AR aging report** | `aging` | Buckets unpaid invoices by 0-30/31-60/61-90/90+ days from `date` to today, grouped by customer. Local SQL only. | 8 |
| 4 | **Client revenue concentration** | `clients top [--limit 10]` | Pareto by revenue per customer; flags forfettari at risk of over-concentration (one client > 80% of turnover triggers AdE attention). | 8 |
| 5 | **VIES bulk validate** | `vies bulk [--country IT]` | For every customer with `country_type=eu` or non-IT P.IVA, calls `/tools/check-vies` and flags invalid/expired. Web UI is one-by-one. | 7 |
| 6 | **Reconcile in vs out** | `reconcile [--period 2025-Q1]` | Net = invoices.taxable - income-invoices.taxable per period. Web UI fragments active vs passive across two pages. | 7 |
| 7 | **F24 calendar export** | `f24 ical [-o due.ics]` | Emits upcoming F24 due-dates as iCal so the user's normal calendar warns them before deadline (notoriously the most-missed Italian deadline). | 6 |
| 8 | **SDI status sweep** | `sdi watch [--older-than 7d]` | Lists every transmitted invoice without an SDI ack older than threshold. Catches stuck transmissions before AdE penalties kick in. | 7 |
| 9 | **Esterometro export** | `esterometro export <year>` | Pulls foreign-mgmt + invoices flagged as `country_type=eu/extra` and outputs the AdE-ready CSV. Manual today. | 7 |
| 10 | **Bollo (stamp duty) accumulator** | `stamp-due [--year]` | Sums stamp duty owed by quarter from invoices with `stamp` set. AdE pre-fills from SDI but discrepancies happen — this is the cross-check. | 6 |
| 11 | **Invoice numbering audit** | `numbering audit [--year]` | Italian law requires sequential numbering per fiscal year; flags gaps, duplicates, out-of-order dates within a number stream. | 8 |
| 12 | **Portable backup snapshot** | `backup [-o backup.zip]` | Sync everything into local SQLite then dump as portable archive (CSV + JSON + signed-URL fetched PDFs). Switching-cost killer; the platform offers no equivalent export. | 8 |

**12 transcendence features, all scoring ≥6.** No stubs.

## Auth Mode
**`composed`** — two related credentials harvested together:
- `cookie`: `p24_logged_in_<hash>=<value>` (full Cookie header), `kind: harvested` (captured during login).
- `header`: `X-WP-Nonce: <value>`, `kind: harvested` (scraped from any logged-in page after login).
- Both required on every authenticated call. Stored together in the user's config; CLI prompts for nonce-only refresh on `rest_cookie_invalid_nonce`.

## Decisions to flag at the gate
- **Cookie auth flow:** v1 uses **manual capture** — `auth set --cookie <full-cookie> --nonce <nonce>` reads pasted values. Automated `auth login --user --password` would require posting to `wp-login.php` and scraping nonce from `partitaiva24.cloud/dashboard`, which is fragile. Manual capture is the floor; automated login can be added in a v0.2.
- **Mutating live tests:** Phase 5 dogfood will exercise read endpoints in full, but mutating endpoints (create/update/delete invoices, mark-paid, transmit-to-SDI) carry real-world consequences. Default plan: `--dry-run` validation for every mutator, plus a single approved create→read→delete lifecycle on a clearly-test customer entity. **No invoice writes** in Phase 5 — these emit fiscally-binding documents.
