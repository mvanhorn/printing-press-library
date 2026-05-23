# Partitaiva24 CLI Brief

## API Identity
- **Domain:** Italian e-invoicing & accounting platform for freelancers and small businesses (Partita IVA holders), competing with Fatture in Cloud and Aruba Fatturazione.
- **Users:** Forfettari, professionisti, PMI, and their commercialisti. The user driving this run is one such Partita IVA holder.
- **Data profile:** Per-tenant SDI-bound documents — invoices (issued + received), corrispettivi, F24 tax payments, esterometro foreign transactions, customer registry, attachments, fiscal-year metadata, support tickets, subscription billing.
- **Surface:** WordPress REST namespace at `/api/v1/`. 77 routes self-described via `GET /api/v1/`. Auth = WordPress session cookie (`p24_logged_in_<hash>`) plus `X-WP-Nonce` header. Cookie pulled from browser session; nonce harvested from any logged-in page.

## Reachability Risk
- **None.** Live probe `GET /api/v1/` returned 200 with full route catalog (77 routes). All 26 unauthenticated-path-but-auth-required GETs returned 200 with the supplied cookie + nonce. No 403/429/Cloudflare/WAF in the path.
- **Caveat:** WP nonces rotate (~24h). The CLI must re-prompt for `X-WP-Nonce` when stale; an expired nonce returns `rest_cookie_invalid_nonce` and is the dominant failure mode in production.

## Top Workflows
1. **List & search invoices**, mark paid, transmit to SDI, send PDF to customer.
2. **Pull income invoices** (passive — vendors invoicing me), mark paid/read, sync to local accounting.
3. **Customer CRUD** — add new buyers/sellers, look up by codice fiscale or P.IVA, validate VIES.
4. **F24 tax payments** — list outstanding, mark paid, archive after submission, request ravvedimento.
5. **Quarterly stats / turnover monitoring** — for forfettari, the live "am I about to blow past €85k" question is the most-asked.

## Codebase Intelligence
- WordPress REST under a custom namespace; payloads are flat JSON.
- Discovered schemas (live, sanitized): `Invoice` (id UUID, fp_id, number, date, status, type, paid, taxRate, from, to, products[], ddt[], payment, pension_funds[], witholdings[], stamp, e_doc_type, e_rf, taxable, total, vat, nettopay, fiscalrelevance), `Customer` (id, type, companyname, pf, pa, surname, name, p_iva, codfis, address, cap, city, prov, country, country_type, tel, email, cod_dest_pec), `IncomeInvoice` (Invoice + sdi_id, sdi_date, sender, filename, body_num, source, is_read, paid), `F24` (collection currently empty for this user — schema TBD via sync), `FiscalYear` (id, label, year, valid_until, turnover_limit, tax_rate, default_vat_id, e_rf, years_limit, is_vat), `InvoicesStats` (fiscal_profile, type, inv_stats, cor_stats, excedence), `Doc` (id, status, year, label, is_read, category, created, path, size, mime, hash, signed_url), `Attachment` (id, parent_id, parent_type, mime_type, path, title, tag, size, url), `Subscription` (id, plan_id, status, gateway, name, price, recurrence), `Notification` + badges{f24, docs, incoming_invoices, tickets}, `TicketCategory`, `Event` (calendar with start/end_dt, recurrence, location).
- Stored sample shapes (types only, no values): `discovery/sample-*.json`.
- Auth: `p24_logged_in_<hash>` cookie + `X-WP-Nonce` header. Both required on every authenticated call. The cookie carries email|exp|hash|hmac (WordPress auth_cookie format).
- Tools subnamespace exposes `check-vies/{cc}/{vat}` (EU VAT validation), `search-pa/{coduni}` (PA registry by IPA code), `invoice/xml2invoice` (parse SDI XML), `sign` (PDF signing), `ping`, `info`, `system_info`.

## Data Layer
- **Primary entities:** invoices, income-invoices, customers, corrispettivi, f24, foreign-mgmt, docs, attachments, notifications, events, tickets, subscriptions.
- **Sync cursor:** invoices and income-invoices use UUID ids and string `date` (YYYY-MM-DD); upserts on (id) plus `modified` timestamp where present. F24/corrispettivi/foreign keyed by integer ids.
- **FTS/search:** generated FTS5 over (number, date, status, to.companyname, to.surname, to.name, to.p_iva, to.codfis, products[].title) for invoices; analogous for income-invoices and customers.

## User Vision (provided in briefing)
- The user manages their own Partita IVA via this platform; the CLI is for personal accounting workflow automation.
- They have provided live cookie + nonce — Phase 5 live testing is mandatory, not optional.

## Source Priority
- Single-source CLI. No combo, no inversion risk.

## Product Thesis
- **Name:** `partitaiva24-pp-cli` (binary), `partitaiva24` (slug)
- **Why it should exist:** Partitaiva24 has zero third-party tooling, no SDK, no published API docs. Their web UI is functional but every freelancer ends up hand-pasting numbers between the platform, the AdE (Agenzia delle Entrate) portal, and a personal spreadsheet to answer "where do I stand?". A local-store CLI inverts that: sync once, then run quarter-end projections, AR aging, forfettario turnover meters, VIES batch validation, and SDI status sweeps offline. It also gives users a portable backup the platform doesn't offer — important because switching costs (export customer registries, historical invoices) are a known pain point in this space.
- **Why agent-native matters:** the user's commercialista or an LLM assistant can ask "quanto IVA devo a fine trimestre" or "quali clienti EU hanno P.IVA non valida" and get a structured JSON answer without scraping the web UI.

## Build Priorities
1. **Foundation:** sync of invoices, income-invoices, customers, F24, corrispettivi, foreign-mgmt, docs, notifications, attachments, subscriptions, events into a local SQLite store with FTS5.
2. **Absorb (P1):** every read endpoint as a typed command; mutating endpoints for paid/read/archive/transmit/sign/send-pdf; tools (VIES check, PA search, XML→invoice).
3. **Transcend (P2):** the 12 power-user features in the absorb manifest (turnover meter, AR aging, tax-due projection, VIES batch, reconcile, F24 ical, SDI sweep, esterometro export, bollo accumulator, numbering audit, client concentration, portable backup).
