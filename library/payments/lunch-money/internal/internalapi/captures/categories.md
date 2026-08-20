# /categories + autocategorization (internal API)

## Categories CRUD — same as public API
- `POST   /v2/categories` (CREATE)
- `PUT    /v2/categories/{id}` (UPDATE)
- `DELETE /v2/categories/{id}` (DELETE) → 204
- `GET    /v2/categories` (LIST)

No new CLI work needed for basic category CRUD.

## Autocategorization (NOT in public API)

The "Set autocategorizations" page maps Plaid taxonomy → Lunch Money categories.

### Populate / refresh
```
POST https://api.lunchmoney.app/plaid/categories/populate
Body: {}
→ 204 No Content
```
Fires when entering the autocategorization page; ensures Plaid taxonomy is loaded
server-side.

### List Plaid taxonomy + current mappings
```
GET https://api.lunchmoney.app/plaid/categories
→ 200, array:
[
  {
    "primary": "INCOME",
    "detailed": "INCOME_DIVIDENDS",
    "description": "Dividends from investment accounts",
    "primary_display_name": "Income",
    "detailed_display_name": "Dividends",
    "tx": null,             // currently mapped Lunch Money category id (null = unmapped)
    "count": 0              // uncategorized tx count waiting for this rule
  },
  ...
]
```

### Set / change a mapping
The original REST-convention guess was wrong. Verified 2026-05-14:

```
PUT https://api.lunchmoney.app/plaid/categories/{detailed-slug}
Body: {"category_id": <lm_category_id>}
→ 404 Cannot PUT /plaid/categories/INCOME_DIVIDENDS
```

Also attempted an immediate clear body for the same zero-count Plaid category:

```
PUT https://api.lunchmoney.app/plaid/categories/INCOME_DIVIDENDS
Body: {"category_id": null}
→ 404
```

No mapping was changed; `GET /plaid/categories` still returned
`{detailed:"INCOME_DIVIDENDS", tx:null, count:0}`.

### Re-run categorization (not captured — destructive)
The "Re-run categorization" button in the UI presumably POSTs to something like
`/plaid/categories/run` or `/plaid/categorize` and applies all mappings to the
9 currently-uncategorized transactions. Capture later when the user wants to run it.

### Create missing categories button
"Create missing categories (N)" auto-creates Lunch Money categories for each
unmapped Plaid taxonomy node. Endpoint not yet captured.

## CLI proposal
- `autocategorize list` → `GET /plaid/categories` (show mappings + uncategorized counts)
- `autocategorize set <detailed-slug> --category-id <id>` → `PUT /plaid/categories/{slug}`
- `autocategorize run` → run the categorizer endpoint
- `autocategorize create-missing` → auto-create LM categories for unmapped Plaid types
