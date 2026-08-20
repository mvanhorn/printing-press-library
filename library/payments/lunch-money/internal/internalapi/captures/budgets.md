# /v2/budgets (internal host) — budget set/clear

Captured 2026-05-14 via `lunch-money-pp-cli internal request` against
`api.lunchmoney.app` with cookie auth.

## Set / update a budget

```
PUT https://api.lunchmoney.app/v2/budgets
Body:
{
  "category_id": 12345,
  "start_date": "2026-05-01",
  "amount": "0.01",
  "currency": "usd",
  "notes": "ZZZZ_CAPTURE_DELETE_ME"
}
-> 200
Response:
{
  "category_id": 12345,
  "start_date": "2026-05-01",
  "amount": "0.0100",
  "currency": "usd",
  "to_base": 0.01,
  "notes": "ZZZZ_CAPTURE_DELETE_ME"
}
```

## Clear a budget

```
DELETE https://api.lunchmoney.app/v2/budgets?category_id=12345&start_date=2026-05-01
-> 204 No Content
```

Reversal was verified by re-reading `/summary` for the same period/category:
the current occurrence returned `budgeted: null`, `budgeted_amount: null`, and
`budgeted_currency: null`.
