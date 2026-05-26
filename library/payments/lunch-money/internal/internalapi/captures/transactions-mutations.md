# /transactions/* — mutation endpoints (internal API)

Captured 2026-05-13 by driving the web UI in Chrome MCP while observing:
- `PerformanceObserver` for URL paths
- `read_network_requests` for HTTP methods + status codes
- `main.bundle.js` static analysis for request body shapes (Chrome MCP redacts URL/body strings containing query params or base64, so the JS interceptor could not surface bodies; the bundle is the source of truth for shapes).

**Two parallel client layers exist in the bundle.** The legacy hand-rolled client (in module `r` with paths like `transactions/group/${id}`) routes requests through `apiClient`, which prepends `/v2` to the path on the wire. The newer typed (openapi-fetch) client uses templated paths directly. Both observed on the wire — `POST /v2/transactions/group`, `DELETE /v2/transactions/group/{id}`, `POST /v2/transactions/split/{id}`. Treat the captured wire paths as canonical; the bundle's "transactions/group" string is the API-relative path the typed client passes to its `/v2` base.

---

## GROUP

### POST /v2/transactions/group (create a transaction group)
```
POST https://api.lunchmoney.app/v2/transactions/group
Body:
{
  "ids": [12345, 12345],        // transaction IDs to group
  "date": "2026-05-05",                    // YYYY-MM-DD
  "payee": "Apple",
  "category_id": 12345,                    // nullable
  "notes": null,                           // nullable
  "tag_ids": []                            // empty array OK
}
→ 200
Response: {parent: {...}, children: [...]} per `e.parent, e.children` consumers
```
Bundle confirmation:
```js
t.createTransactionGroup = e => apiClient.transactions.group(e)
// where group: async e => client.POST("/transactions/group", {body: e})
```

Real consumer call shape:
```js
createTransactionGroup({
  ids: Object.keys(I).map(Number),
  date: U.date, payee: U.payee, category_id: U.category_id,
  notes: U.notes, tag_ids: U.tags
})
```

### DELETE /v2/transactions/group/{group_id} (ungroup)
```
DELETE https://api.lunchmoney.app/v2/transactions/group/12345
→ 204 No Content
```
Captured live: `method: DELETE, statusCode: 204` against `/v2/transactions/group/12345` (the parent transaction ID). After delete, the two original children become unparented again (`in_group:false, group_id:null`).

Bundle: `ungroup: async e => client.DELETE("/transactions/group/{id}", {params:{path:{id:e}}})` → wire path is `/v2/transactions/group/{id}`.

### PUT /transactions/group/{group_id} (update group metadata)
```
PUT https://api.lunchmoney.app/transactions/group/{id}
Body: {date, payee, category_id, notes, tag_ids, ...}
→ 200, returns updated group
```
Used by the "Edit transaction group" panel's SAVE CHANGES button when you do NOT click UNGROUP. Note: the legacy hand-rolled client (`(0,r.put)('transactions/group/${id}', t)`) likely hits unversioned `/transactions/group/{id}` while the typed client hits `/v2/transactions/group/{id}`. Not separately verified.

### PUT /transactions/group/bulk_ungroup (bulk variant)
```
PUT https://api.lunchmoney.app/transactions/group/bulk_ungroup
Body: {transaction_ids: [id1, id2, ...]}
→ 200, returns {orphaned: [...], asset_update?: [...], background_job?: ...}
```
Bundle: `t.ungroupTransactions = e => (0,r.put)("transactions/group/bulk_ungroup", e)`.

Reversal sequence: create with POST /v2/transactions/group → store the returned `parent.id` → call `DELETE /v2/transactions/group/{parent.id}`.

---

## SPLIT

### POST /v2/transactions/split/{parent_id} (split into child transactions)
```
POST https://api.lunchmoney.app/v2/transactions/split/12345
Body:
{
  "child_transactions": [
    {
      "date": "2026-05-09",
      "payee": "Amazon",
      "amount": "11",                        // strings or numbers — verify; we sent percentage-driven so 50% of -$22 ⇒ -$11
      "currency": "usd",
      "category_id": 12345,                  // nullable
      "notes": null,                         // nullable
      "tag_ids": []                          // nullable, may be omitted
    },
    {... second child ...}
  ]
}
→ 200
Response: {split: [...children with new ids...], parent: {...}}
```
Captured live: PerformanceObserver showed `/v2/transactions/split/12345` followed by a GET `/transactions/split/12345`. After split, the parent gets `has_children: true` and the children are real new transactions inheriting from the parent's account.

Bundle:
```js
split: async (e, t) => client.POST("/transactions/split/{id}", {params:{path:{id:e}}, body:t})
// caller:
splitTransaction(z.id, {child_transactions: a})
```

Where `a` is built from the panel's form state:
```js
{
  date: q.date,
  payee: q.payee,
  category_id: q.category_id ?? undefined,
  notes: q.notes ?? undefined,
  tag_ids: q.tag_ids,
  amount: q.amount,           // optional if amount_pct
  amount_pct: q.amount_pct    // optional alternative to amount
}
```

So each child can be specified by **either** absolute `amount` **or** `amount_pct` (0–100). The web UI's "Split Evenly 2 Ways" sends `amount_pct: 50, amount: 11` for a -$22 parent. The "Custom Split" path sends only `amount`.

### PUT /transactions/split/bulk_unsplit (unsplit)
```
PUT https://api.lunchmoney.app/transactions/split/bulk_unsplit
Body:
{
  "transaction_ids": [12345],      // parent IDs to unsplit
  "remove_parents": false               // false = keep parent transaction, drop children; true = also delete parent
}
→ 200
Response: data is array of orphaned child IDs that were promoted back to roots
```
Captured live via PerformanceObserver. The web UI's "Unsplit" button sends `transaction_ids:[parent_id], remove_parents: false` for the standard unsplit-this-transaction case. When unsplitting from inside the parent (the "Confirm unsplit" modal) the UI passes the parent's ID.

Bundle:
```js
t.unsplitTransactions = e => (0,r.put)("transactions/split/bulk_unsplit", e)
// callers:
unsplitTransactions({transaction_ids:[R.parent_id], remove_parents:false})
```

### DELETE /v2/transactions/split/{id} (single-transaction unsplit, typed-client variant)
Per the typed client `unsplit: async e => client.DELETE("/transactions/split/{id}", ...)`. Not observed live (the web UI used `bulk_unsplit` for our single-transaction unsplit), but available.

Reversal sequence: POST /v2/transactions/split/{id} returns `parent.id` and `split[].id` → call `PUT /transactions/split/bulk_unsplit` with `{transaction_ids:[parent.id], remove_parents:false}`.

---

## CREATE single transaction (web UI's "ADD TO CASH" flow)

### POST /transactions (NOT /transactions/new)
```
POST https://api.lunchmoney.app/transactions
Body:
{
  "transaction": {
    "date": "2026-05-13",
    "payee": "ZZZZ_TEST_DELETE_ME",
    "notes": null,
    "amount": -0.01,                        // negative = outflow, positive = inflow
    "currency": "usd",
    "category_id": null,                    // or category ID
    "asset_id": 12345,                      // for manual accounts; OR plaid_account_id
    "status": "cleared" | "uncleared"       // optional; respects user's auto_review_transaction_on_creation setting
  },
  "opts": {
    "should_convert": true                  // convert from foreign currency? Plaid → true; manual transfer → false
  }
}
→ 200
Response: {transactions: [{id, ...}], asset_update?: [...]}
```

**WARNING — captured behavior, not from a clean test.** The web UI's "ADD TO CASH" inline editor row sits visually adjacent to the pending-transactions row. Tab navigation can move focus from the editor into a pending row's fields, triggering a `PUT /transactions/{tx_id}` update on an existing pending bank transaction instead of `POST /transactions`. Observed once during this capture session and immediately reverted. **The CLI should never replicate the web UI's inline-edit collision behavior** — only use `POST /transactions` with explicit `transaction.asset_id`.

Bundle:
```js
t.createTransaction = (e, t) => (0,r.post)("transactions", {transaction:e, opts:t})
```

The legacy hand-rolled client uses unversioned `/transactions` (not `/v2/transactions`). The typed client probably mirrors this at `/v2/transactions`.

### DELETE /transactions/{id} (delete single)
```
DELETE https://api.lunchmoney.app/transactions/{id}
Body: opts? (maybe `{force_delete: true}`)
→ 200
```
Bundle: `t.deleteTransaction = (e, t) => (0,r.del)('transactions/${e}', t)`.

### POST /transactions/bulk_delete (delete many)
```
POST https://api.lunchmoney.app/transactions/bulk_delete
Body: {transaction_ids: [...]}
→ 200, returns {asset_update?: [...], background_job?: ...}
```
Bundle: `t.deleteTransactions = e => ...(POST "transactions/bulk_delete")`.

Reversal sequence: create with POST /transactions → grab `transactions[0].id` → `DELETE /transactions/{id}`.

---

## ATTACHMENTS / FILES

The bundle exposes BOTH a legacy and a typed-client family. The currently-active "SELECT FILES" button in the transaction-details panel routes through the legacy family. Chrome MCP refused our file upload (`code:-32000, "Not allowed"`) so neither family was exercised live this session.

### Legacy family (active in transaction-details panel)
```
POST   https://api.lunchmoney.app/transactions/file/{transaction_id}
Body:  multipart FormData with a single "file" field
→ 200, response: {data: {id, file_url, ...}}     // file_url is the storage URL

GET    https://api.lunchmoney.app/transactions/file/{file_id}
→ 200, response: {data: {file_url, ...}}         // signed-URL fetch

PUT    https://api.lunchmoney.app/transactions/file/{file_id}
Body:  {transaction_id: <new_tx_id>}             // re-assign file to a different transaction (used in ungroup-with-file flow)
→ 200

DELETE https://api.lunchmoney.app/transactions/file/{file_id}
→ 204
```
Bundle:
```js
t.uploadFile = (e, t) => (0,r.post)('transactions/file/${e}', t)  // e = tx_id, t = FormData
t.updateFile = (e, t) => (0,r.put)('transactions/file/${e}', t)   // e = file_id, t = {transaction_id}
t.deleteFile = e => (0,r.del)('transactions/file/${e}')           // e = file_id
t.getFileUrl = e => (0,r.get)('transactions/file/${e}')           // e = file_id
```

### Typed-client family (exists in bundle, possibly newer)
```
POST   https://api.lunchmoney.app/v2/transactions/{transaction_id}/attachments
Body:  multipart with file
→ 200

GET    https://api.lunchmoney.app/v2/transactions/attachments/{file_id}
→ 200, returns download URL

DELETE https://api.lunchmoney.app/v2/transactions/attachments/{file_id}
→ 204
```
Bundle:
```js
attachFile: async (e, t) => client.POST("/transactions/{transaction_id}/attachments", {params:{path:{transaction_id:e}}, body:t})
getAttachmentUrl: async e => client.GET("/transactions/attachments/{file_id}", ...)
deleteAttachment: async e => client.DELETE("/transactions/attachments/{file_id}", ...)
```

### PDF upload (statement parsing)
```
POST https://api.lunchmoney.app/transactions/file/pdf
Body: multipart (PDF) + form fields {asset_id?, plaid_account_id?}
→ 200, {data: {uuids: [...], processing: true}}     // async processing

POST https://api.lunchmoney.app/transactions/file/pdf/status
Body: {uuids: [...]}
→ 200, {data: {processing: false, transactions: [...]}}  // poll until processing:false
```
Bundle:
```js
t.uploadPDF    = e => (0,r.post)("transactions/file/pdf", e)
t.getPDFStatus = e => (0,r.post)("transactions/file/pdf/status", e)
```

Reversal sequence: POST upload → response includes `id` → `DELETE /transactions/file/{id}`.

---

## CSV IMPORT (bulk_insert)

### Discovered on import dialog open:
```
GET https://api.lunchmoney.app/import_configs
→ 200, list of saved column-mapping configs
```

### Upload + check + commit cycle:
```
GET https://api.lunchmoney.app/import_configs
PUT https://api.lunchmoney.app/import_configs
Body: {name, columns_mapping, ...}                 // save user's column mapping for reuse
→ 200

PUT https://api.lunchmoney.app/transactions/bulk_insert/check
Body: {
  transactions: [{date, payee, amount, ...}, ...],
  apply_rules: true|false,
  skip_duplicates: true|false,
  create_new_categories: true
}
→ 200, returns {transactions: [...prepared...], errors: [...]}

PUT https://api.lunchmoney.app/transactions/bulk_insert
Body: {transactions: [...same shape as check...]}
→ 200, returns {transactions: [...committed with ids...], total_count, asset_update?}
   or {background_job: ...}     // for very large imports
```
Bundle:
```js
t.bulkInsertTransactions      = e => (0,r.put)("transactions/bulk_insert", e)
t.checkBulkInsertTransactions = e => (0,r.put)("transactions/bulk_insert/check", e)
t.getImportConfigs            = ()  => (0,r.get)("import_configs", {})
t.saveImportConfig            = e   => (0,r.put)("import_configs", e)
```

Reversal sequence: import is non-trivial to reverse — captured but not exercised. Recommended for CLI: always run `/transactions/bulk_insert/check` first as a dry-run preview before `/transactions/bulk_insert`.

---

## Other transactions endpoints surfaced in the bundle (not exercised this session)

| Method | Path | Bundle binding |
|---|---|---|
| POST | `/transactions/clear` | `t.clearAllByMonth` (mark all txns in a month as reviewed) |
| POST | `/transactions/fetch` | account refresh trigger (Plaid) |
| POST | `/transactions/multiple` | `getTransactionsMultiple({ids:[...]})` — batch fetch |
| GET  | `/transactions/years` | `t.getYears` — list years with any transactions |
| GET  | `/transactions/merchant_names` | autocomplete source |
| POST | `/transactions/recurring` | `linkRecurring` / `unlinkRecurring` |
| POST | `/transactions/search/string` | full-text search |

---

## Capture caveats

1. **Chrome MCP redaction**: any request body or URL fragment containing what looked like a token, cookie, or query string was returned to the JS interceptor as the literal string `[BLOCKED: Cookie/query string data]` or `[BLOCKED: Base64 encoded data]`. The `read_network_requests` panel similarly only surfaces a subset of mutations (we saw the DELETE 204 but not the POST/PUT round-trips). **PerformanceObserver was the only signal that reliably surfaced the URLs of every mutation**; method + status came from the network panel; body shapes came from `main.bundle.js` static analysis.

2. **`__cap` JS interceptor failures**: a hand-installed `window.fetch` + `XMLHttpRequest.prototype.send` interceptor captured `/system/status` and `/summary` GETs but missed the POSTs entirely. We tried four variants (closure capture, prototype patch, `Object.defineProperty` getter trap, `globalThis.fetch =`). The React app must hold a pre-import reference to the original `fetch` that bypasses all of them. **PerformanceObserver is the correct interception surface for this app.**

3. **`/v2` prefix**: the bundle's hand-rolled client (module `r` with `r.post`, `r.put`, etc.) writes paths *without* the `/v2` prefix (`"transactions/group"`). The typed openapi-fetch client writes them the same way but with templated path params. On the wire, both clients ended up calling the `/v2/`-prefixed path for `group` and `split`. For `bulk_unsplit` and the legacy `file` endpoints, the wire path was unversioned. The safest CLI implementation is to **read the wire paths from the captures column, not the bundle path strings**.

4. **File upload was blocked**: Chrome MCP's `file_upload` tool returned `{"code":-32000,"message":"Not allowed"}` for both `.txt` and `.pdf` paths. Attachment upload + delete were not exercised live.

5. **Accidental real-data mutation risk**: the "ADD TO CASH" inline editor on /transactions/{year}/{month} sits one Tab-stop away from pending-transaction rows. During testing, focus jumped into a pending row's amount field and sent a `PUT /transactions/{id}` instead of the intended create request. The change was immediately reverted. Documenting here so the CLI design avoids the same UI pattern.
