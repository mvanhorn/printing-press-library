# Settings, profile, suggested-rules, export probes

Captured 2026-05-14 with `lunch-money-pp-cli internal request`.

## Profile update

Read paths:

```
GET /me     -> 200, full profile keys
GET /v2/me  -> 200, slim profile keys: account_id, api_key_label, budget_name,
               email, id, name, primary_currency
```

Write probes:

```
PUT /me {}
-> 500 {"name":"Error","message":"Error updating profile, please try again."}

PUT /me {"name": <current>, "budget_name": <current>, "primary_currency": <current>}
-> 500

PUT /me {"name": <current>, "account_display_name": <current>,
         "primary_currency": <current>, "phone": <current>}
-> 500

PUT /v2/me {"name": <current>, "budget_name": <current>,
            "primary_currency": <current>}
-> 404 Cannot PUT /v2/me
```

No profile value was intentionally changed; these were same-value or empty-body
probes. Body shape remains unknown.

## Settings / preferences

```
PUT /me/preferences {}
-> 404

PUT /settings {}
-> 404

PUT /preferences {}
-> 404
```

Notification/date-format write paths remain uncaptured.

## Suggested rules

The capture plan's `GET /rules/suggested?criteria_id={id}` did not behave as
previously expected in this session:

```
GET /rules/suggested?criteria_id=12345
-> 400 {"name":"ValidateError","message":"Missing required fields: criteria_id"}

GET /rules/suggested/12345
-> 404

GET /rules/suggestions?criteria_id=12345
-> 400 Missing required fields: criteria_id

POST /rules/suggested {"criteria_id":12345}
-> 404
```

No suggested rule was accepted or deleted.

## Transaction export

Small-date-range export guesses:

```
GET /transactions/export?format=csv&start_date=2026-05-01&end_date=2026-05-01
-> 404

GET /v2/transactions/export?format=csv&start_date=2026-05-01&end_date=2026-05-01
-> 400 route collision with /v2/transactions/{id}; "export" parsed as id

GET /transactions/download?format=csv&start_date=2026-05-01&end_date=2026-05-01
-> 404

GET /exports/transactions?format=csv&start_date=2026-05-01&end_date=2026-05-01
-> 404
```

CSV/JSON download path remains unknown.
