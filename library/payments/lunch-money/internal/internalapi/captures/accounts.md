# /accounts (internal API) — uses `/assets` not `/manual_accounts`

Internal API renames "manual_accounts" to "assets". Functionally equivalent but with
a richer set of fields (including Plaid access tokens — see Privacy below).

## LIST (all accounts: manual + Plaid + crypto)
```
GET https://api.lunchmoney.app/assets
→ 200, array of full account objects
```

⚠ **PRIVACY:** This response includes `access_token` and `item_id` for every Plaid-linked
account. Treat as secret-tier. The CLI must:
- never log this field to disk
- redact it in any --json output (drop the key before printing)
- never persist it to the local SQLite store

## CREATE manual account
```
POST https://api.lunchmoney.app/assets
Body:
{
  "type_id": 1,           // numeric type (1 = cash; others undocumented)
  "type_name": "cash",    // also send by name
  "subtype_id": null,
  "subtype_name": "Other",
  "name": "...",
  "display_name": "...",
  "balance": 123.45,
  "currency": "usd",
  "institution_name": "..."
}
→ 200, returns full account object
```

## UPDATE
```
PUT https://api.lunchmoney.app/assets/{id}
Body:
{
  "type_name":"cash", "type_id":null,
  "subtype_name":"Other", "subtype_id":null,
  "name":"...", "display_name":"...",
  "balance": 999.99, "currency":"usd",
  "institution_name":"...",
  "exclude_transactions": false,
  "status": "active",
  "closed_on": "YYYY-MM-DD"
}
→ 200, response: {"updated":[{...full account...}]}
```

## DELETE (two-step)
```
GET  https://api.lunchmoney.app/assets/{id}/status
→ 200, {"hasTransaction": bool, "hasRecurring": bool, "hasBalanceHistory": bool}

PUT  https://api.lunchmoney.app/assets/{id}/delete
Body: {"keep_items": false}    // true = unlink Plaid item but keep transactions in store
→ 204 No Content
```

## Account types (inferred from /assets/subtypes)
The `GET /assets/subtypes` call on every page load returns the list of valid type/subtype
pairs (cash → checking/savings/digital-wallet/physical-cash/gift-card/store-credit/other,
credit → credit-card, investment → brokerage/retirement/..., etc.).

## Plaid account fetch trigger

Captured 2026-05-14 with a single account id:

```
POST https://api.lunchmoney.app/v2/plaid_accounts/fetch?id=295207
Body: {}
→ 202 Accepted
```

The unversioned internal-host path is not mounted:

```
POST https://api.lunchmoney.app/plaid_accounts/fetch?id=295207
→ 404 Cannot POST /plaid_accounts/fetch
```

This matches the public-v2 endpoint shape, mounted under `/v2` on
`api.lunchmoney.app`. The endpoint queues a background Plaid fetch; no reversal
is needed.

## Auth refresh
The internal API uses short-lived JWT access tokens. On 401, the web UI calls:
```
POST https://api.lunchmoney.app/auth/token/refresh
Body: {}
→ 204 (sets new access cookie)
```
CLI must handle this on 401 with `errMsg: "Access token expired."`.

## CLI proposal
- `assets list` → `GET /assets` (redacts access_token by default; `--show-secrets` opt-in)
- `assets create --name X --balance N ...` → `POST /assets`
- `assets update <id> --balance N` → `PUT /assets/{id}`
- `assets delete <id> [--keep-items]` → status check + `PUT /assets/{id}/delete`
- `assets status <id>` → `GET /assets/{id}/status`
