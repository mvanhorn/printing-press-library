# Budget Reference

Use this file for Wanderlog budget summaries, CSV export, budget totals, expenses, settlement payments, categories, split fields, and undo/redo of budget edits.

## Commands

- `plan budget summary`: Read budget amount, expense count, payment count, totals by category, totals by currency, and debt simplification state.
- `plan budget csv`: Export expenses through Wanderlog's `expensesAsCSV` endpoint.
- `plan budget set`: Set total amount, currency, or `simplifyDebt`.
- `plan budget expense list/add/edit/remove`: Manage expenses. Use `--block-id` to associate a cost with an itinerary block/place card; nonzero block ids are validated against the plan before apply.
- `plan budget payment list/add/remove`: Manage settlement payments.

## Expense Shape

Wanderlog stores budget data at `itinerary.budget`:

```json
{
  "amount": {"amount": 0, "currencyCode": "SGD"},
  "expenses": [],
  "payments": [],
  "simplifyDebt": false
}
```

A CLI-created expense uses this shape:

```json
{
  "id": 488194167,
  "amount": {"amount": 12.34, "currencyCode": "SGD"},
  "category": "food",
  "description": "Lunch",
  "date": "2026-08-30",
  "blockId": null,
  "paidByUserId": 123,
  "paidByUser": {"type": "registered", "id": 123},
  "splitWith": {"type": "noOne", "users": []},
  "associatedDate": "2026-08-30"
}
```

Important: `splitWith.users` must exist and be an array for every split mode, including `noOne` and `everyone`. Omitting it can make the plan fail to subscribe with `expense.splitWith.users is not iterable`.

Dry-run reports for add/edit/remove include the `expense` payload so agents can verify amount, currency, category, date, split, and `blockId` before applying.

## Categories

Supported categories are: `flights`, `lodging`, `carRental`, `publicTransit`, `food`, `drinks`, `sightseeing`, `activities`, `shopping`, `gas`, `groceries`, `other`.

## Split Modes

- `--split-with noOne`: Use `{"type":"noOne","users":[]}`.
- `--split-with everyone`: Use `{"type":"everyone","users":[]}`.
- `--split-with individuals --split-user-id <id>`: Use registered user refs in `users`.

## Payment Shape

A settlement payment uses:

```json
{
  "id": 123,
  "amount": {"amount": 24, "currencyCode": "SGD"},
  "paidAt": "2026-06-21T00:00:00Z",
  "fromUser": {"type": "registered", "id": 1},
  "toUser": {"type": "registered", "id": 2}
}
```

## Safe Budget Workflow

1. Read `plan budget summary` and `plan budget expense list`.
2. Resolve the itinerary block id before linking a place-card cost with `--block-id`.
3. Dry-run add/edit/remove first unless the user already approved the exact change.
4. Apply and read back the expense or summary.
5. Use `plan undo --apply` if the budget edit is wrong.
6. For live testing, use a disposable clone and remove smoke-test journal records after the final cleanup so `redo` cannot reinsert them.
