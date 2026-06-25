# Costco Receipts CLI Brief

## API Identity
- Domain: Costco member purchase/receipt history. No official public consumer API.
- Real surface: internal SPA backend `POST https://ecom-api.costco.com/ebusiness/order/v1/orders/graphql`.
- Users: Costco members who want their own in-warehouse + gas + online purchase history as structured data (taxes, budgeting, warranty/return windows, household spend analysis).
- Data profile: per-transaction receipts with full line items (UPC, description, dept, unit price, qty, tax/refund/void flags, fuel grades), coupons, sub-taxes, and tender (payment) details.

## User Vision
- "I would like access to my receipt history. The website UI only shows past two years and I want to understand if we can go further back."
- Translation: headline feature = pull receipts over an arbitrary date range; flagship transcendence feature = empirically probe how far back the API actually serves for the user's account.

## Reachability Risk
- [Low–Medium] Homepage returns HTTP 200 (3.9MB real HTML), `probe-reachability` = `standard_http`. Akamai Bot Manager + reCAPTCHA strings are embedded scripts, not a challenge page.
- The receipts GraphQL endpoint requires `Costco-X-Authorization: Bearer <idToken>`; without it -> 401 (expected for an auth-gated API). Real reachability validation requires a live idToken from the user's logged-in session.
- The same endpoint is used by a public Chrome extension (`nnalnbomehfogoleegpfegaeoofheemn`) and the open-source TCRDD tool, both currently working -> the contract is live and stable.

## The 2-Year Question (core research finding)
- Website Orders & Returns UI shows receipts in 6-month increments **up to 2 years** (confirmed by Costco customer-service docs).
- The GraphQL `receipts(startDate, endDate)` query takes an **explicit date range** — the 2-year limit is a UI date-picker cap, not necessarily an API cap.
- TCRDD (open-source) fetches a **rolling 3 years** (`startDate = now - 3yr - 1mo`) and documents it as "the maximum allowed history."
- => API demonstrably serves at least 3 years (one year past the UI). Whether it serves beyond 3 years is UNTESTED. Costco's own stance: data older than ~2 years requires contacting customer service / membership desk — implying a backend retention horizon somewhere past 2yr. The exact floor is account-specific and empirically discoverable by stepping startDate backward.

## Endpoint Contract (ground truth from TCRDD source + web research)
- POST `https://ecom-api.costco.com/ebusiness/order/v1/orders/graphql`
- Headers:
  - `Content-Type: application/json`
  - `Costco.Env: ecom`
  - `Costco.Service: restOrders`
  - `Costco-X-Wcs-Clientid: <clientID>`        (localStorage `clientID`)
  - `Client-Identifier: 481b1aec-aa3b-454b-b81b-48187e28f205`  (static app constant)
  - `Costco-X-Authorization: Bearer <idToken>`  (localStorage `idToken`, a JWT)
- Body: `{ query: "query receipts($startDate:String!,$endDate:String!){ receipts(startDate,endDate){ ... } }", variables: { startDate, endDate } }`
- A sibling query `receiptsWithCounts` returns category counts: `inWarehouse`, `gasStation`, `carWash`, `gasAndCarWash` + the receipts array. Useful for a "summary/counts" command.
- documentType / receiptType distinguish warehouse vs gas vs online vs car-wash.

### Receipt fields (top-level)
documentType, receiptType, membershipNumber, transactionType, transactionDateTime, transactionDate, warehouseShortName/Number/Name/Address1/Address2/City/State/Country/PostalCode/AreaCode/Phone, companyNumber, invoiceNumber, sequenceNumber, transactionBarcode, totalItemCount, instantSavings, subTotal, taxes, total, currencyCode, registerNumber, transactionNumber, operatorNumber.

### Nested arrays
- itemArray: itemNumber, itemUPCNumber, itemDescription01/02, frenchItemDescription1/2, itemIdentifier, itemDepartmentNumber, transDepartmentNumber, itemUnitPriceAmount, unit, amount, taxFlag, refundFlag, resaleFlag, voidFlag, merchantID, entryMethod, fuel* (fuelUnitQuantity, fuelUomCode/Description, fuelGradeCode/Description).
- couponArray: couponNumber, upcnumberCoupon, associatedItemNumber, unitCoupon, amountCoupon, tax/void/refund flags.
- subTaxes: tax1..4 + a/b/c/d/u tax legends/percents/amounts.
- tenderArray: tenderTypeCode/Name, amountTender, displayAccountNumber (masked), approvalNumber, transactionID, walletType, etc.

## Auth model
- Shape: **composed / bearer-from-localStorage**. Login (cookie+OIDC) -> SPA stores `idToken` (JWT) + `clientID` in localStorage -> sent as headers on every receipts call.
- CLI v1 auth: `auth set-token` accepting idToken + clientID pasted from DevTools (`localStorage.idToken`, `localStorage.clientID`), or imported via a browser-sniff/press-auth localStorage extraction.
- idToken is short-lived (JWT exp). Document this: the CLI works for the life of a captured token; full OIDC refresh is out of v1 scope (would require replicating Costco's identity flow). `doctor` should decode the JWT `exp` and warn when expired.
- PII: receipts contain membership number, masked payment account numbers, addresses. Never write token values or raw receipt dumps into artifacts; redact in any proof.

## Top Workflows
1. Pull all receipts in a date range -> structured JSON/CSV (the core).
2. Probe how far back history goes for this account (the vision).
3. Filter/search receipts by item (UPC/description), warehouse, amount, date.
4. Spend analytics: total by month, by warehouse, by department, instant-savings captured.
5. Export a full local archive (SQLite) and incrementally sync new receipts (dedup by membershipNumber+transactionBarcode).

## Table Stakes (absorbed from existing tools)
- TCRDD: rolling-3yr fetch, JSON export, dedup by barcode, multi-member merge, smart incremental merge.
- Chrome extension: one-click receipt download.
- costco-importer / CostcoWrapped: parse receipts to CSV for sheets/budgeting.
- Costco_Scraping: per-item product-link enrichment.

## Data Layer
- Primary entity: `receipt` (keyed by membershipNumber+transactionBarcode). Child: line items, tenders, coupons, taxes.
- Sync cursor: transactionDate (incremental: fetch since last synced date).
- FTS/search: item descriptions + UPC + warehouse name.
- Local store unlocks: cross-receipt analytics, item price-history over time, dept rollups, spend trends — none of which any single API call returns.

## Why install this over the UI/extension
- Goes past the 2-year UI cap (date-range param) and **measures** the true history floor.
- Structured, scriptable, agent-native output (JSON/CSV/--select), not a one-shot download.
- Local SQLite archive that compounds: price history, spend analytics, incremental sync, household merge.
- Read-only, member-scoped, runs from terminal/cron.

## Product Thesis
- Name: costco-pp-cli (Costco Receipts)
- Why it should exist: Costco hides your own purchase history behind a 2-year UI wall and a per-6-month picker. The backend serves more. This CLI gives you your complete receipt history as data, proves how far back it actually goes, and builds a local archive that answers spend/price/warranty questions the website never will.

## Build Priorities
1. Composed/bearer auth (set-token + doctor JWT-exp check) and the receipts GraphQL client.
2. `receipts` (date-range fetch), `sync` (SQLite archive, dedup, incremental), `search`/`sql`.
3. Flagship: `history-depth` probe — step startDate backward, report the real floor.
4. Analytics: spend by month/warehouse/department; item price-history; instant-savings total.
5. counts/summary command (receiptsWithCounts), CSV export, household merge.

## LIVE VALIDATION (this run, authenticated probe)
- Working request confirmed via DevTools Copy-as-cURL. Critical fix: **`Content-Type: application/json-patch+json`** (standard `application/json` -> gateway 401 "Invalid credentials", a misleading error).
- **No cookies required** — pure bearer-token auth. Headers needed:
  - `costco-x-authorization: Bearer <idToken>`
  - `costco-x-wcs-clientId: <clientID>`   (localStorage clientID, e.g. 4900eb1f-...)
  - `client-identifier: 481b1aec-aa3b-454b-b81b-48187e28f205`  (static)
  - `costco.env: ecom`, `costco.service: restOrders`
  - `Content-Type: application/json-patch+json`, `Origin/Referer: https://www.costco.com`
- **2-year cap DISPROVEN as an API limit:** `receipts(startDate:"2010-01-01", endDate:today)` returned rows back to 2023-09-17 (~2y9m) — older than the 2-year UI window. API returns the full account history, not a 2-year slice. (Tested account's own data started 2023-09, so the API's true retention floor is >= that; `history-depth` command finds the real floor per account by stepping startDate back until earliest stops moving.)
- `receipts` returns the FULL set in ONE call (no pagination). `getOnlineOrders` IS paginated (pageNumber/pageSize/warehouseNumber).
- idToken is a JWT with ~15-min `exp`. No refresh-token flow exposed -> re-paste model.
- Two queries at the same endpoint:
  - `receipts(startDate,endDate)` -> in-warehouse + gas + carwash receipts (documentType differentiates).
  - `getOnlineOrders(startDate,endDate,pageNumber,pageSize,warehouseNumber)` -> online .com orders, paginated, with shipment/tracking.
  - `receiptsWithCounts(...)` -> category counts (inWarehouse/gasStation/carWash/gasAndCarWash) + receipts.
