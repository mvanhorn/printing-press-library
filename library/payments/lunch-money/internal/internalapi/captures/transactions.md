# /transactions (internal API) — review flow

The internal API exposes richer search + bulk operations than the public API.

## LIST with unreviewed filter
```
GET https://api.lunchmoney.app/transactions
    ?is_unreviewed=true
    &start_date=YYYY-MM-DD
    &end_date=YYYY-MM-DD
    &match=all
    &paginate=true
→ 200, JSON {transactions: [...]}
```

## Snapshot endpoint (one-shot page load)
```
GET https://api.lunchmoney.app/snapshot/transactions_page
→ 200
```
Probably bundles transactions + categories + accounts + tags for fast initial render.
Worth probing for an offline-snapshot CLI command.

## UPDATE single transaction status / fields
```
PUT https://api.lunchmoney.app/transactions/{tx_id}
Body: {"status": "cleared" | "uncleared" | ...}
       (or any combination of writable fields: payee, category_id, notes, tags, amount, date, etc.)
→ 200
Response:
{
  "criteriaId": null,                  // rule that fired, if any
  "isSuggestedRule": false,
  "transactionRuleCount": 0,
  "asset_update": [],
  "updated_fields": {...},
  "num_applied_rules": 0
}
```

Note: status enum is `cleared` (reviewed) and `uncleared` (unreviewed).

## BULK UPDATE (the killer feature)
```
PUT https://api.lunchmoney.app/transactions/bulk_update
Body:
{
  "transactionIds": ["12345", "12345"],
  "updateObj": {"status": "cleared"}
}
→ 200
Response: {"asset_update":[...], "recurring_transactions":[...]}
```

The `updateObj` can contain ANY writable field — so this is universal bulk-edit:
- Mark N transactions reviewed: `{"status":"cleared"}`
- Bulk recategorize: `{"category_id": 12345}`
- Bulk tag: `{"tag_ids":[262245]}`
- Bulk exclude from totals: `{"exclude_from_totals": true}`

Public API has `transactions update` which is similar but with different payload shape.
The internal `bulk_update` is the better target for review workflows because the response
already names which transactions changed, including recurring linkage.

## CLI proposal — review workflow
```
lunch-money-pp-cli review list --since 2026-01-01 --status unreviewed
lunch-money-pp-cli review mark <tx_id>... --reviewed
lunch-money-pp-cli review mark --filter unreviewed --since 30d --reviewed   # bulk all matching
lunch-money-pp-cli review bulk-edit <tx_id>... --category-id 12345
lunch-money-pp-cli review bulk-edit <tx_id>... --tag <name>
```

The `triage` command already exists in the CLI and overlaps with this — it suggests
categories for unreviewed transactions. The new `review` family complements it with
the actual mark-reviewed action.
