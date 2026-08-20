# /rules (internal API)

Base: `https://api.lunchmoney.app` (cookie auth, separate from `api.lunchmoney.dev/v2`).

## LIST
```
GET /rules?offset=0&limit=100
→ 200, JSON array of rule objects (~95KB on a 60-rule account)
```

## CREATE
```
POST /rules
Body:
{
  "conditions": {
    "on_plaid": true,
    "on_api":  true,
    "on_csv":  true,
    "payee": {"name":"ZZZZ_TEST", "match":"contain"}
  },
  "actions": {
    "description": "free text label",
    "category_id": 12345
  }
}
→ 200, returns full rule object including rule_id + rule_criteria_id
```

### Action shapes — full enum surface (from main.bundle.js)

The `actions` object accepts any combination of these keys (all `true` or value-typed):

| Key | Type | UI label | Notes |
|---|---|---|---|
| `category_id` | number | "Set category" | Set category to category_id |
| `set_uncategorized` | `true` | "Set uncategorized" | Strip category. Cannot combine with `category_id`. |
| `payee` | string | "Set payee" | Rename payee on match |
| `description` | string | (free-text label) | Internal label for the rule |
| `notes` | string | "Set notes" | Set transaction notes |
| `add_tag_ids` | number[] | "Add tags" | Adds tags (does not replace) |
| `mark_as_reviewed` | `true` | "Mark as reviewed" | Sets status: cleared |
| `mark_as_unreviewed` | `true` | "Mark as unreviewed" | Sets status: uncleared |
| `should_send_email` | `true` | "Send me an email" | Email notification |
| `should_delete` | `true` | "Delete transaction" | Hard-delete on match |
| `should_split` | object | "Split transaction" | Auto-split with template |
| `skip_recurring` | `true` | "Don't link to recurring item" | Suppress recurring auto-link |
| `dont_run_rules` | `true` | "Don't create a rule" / "Skip suggestion" | Suppress suggested-rule promotion |
| `stop_processing_others` | `true` | "Stop processing other rules" | Halts the chain |

### CREATE — example: "Mark unreviewed AND clear category"

Captured live via PerformanceObserver as `POST /rules → 200`.

```
POST /rules
Body:
{
  "conditions": {
    "payee": {"name": "ZZZZ_TEMP_TEST_RULE", "match": "contain"}
  },
  "actions": {
    "mark_as_unreviewed": true,
    "set_uncategorized": true
  }
}
→ 200
```

The UI exposes "Mark as unreviewed" and "Set uncategorized" as **separate AND-ed actions**, both stored as boolean flags in the same `actions` object. Confirm the rule list shows the result as "Do not set category, Mark as unreviewed" — matching the bundle's `set_uncategorized → "set uncategorized"` and `mark_as_unreviewed → "Mark as unreviewed"` mappings.

Reversal: select rule's checkbox in /rules list, click DELETE SELECTED, confirm → fires `POST /rules/bulk_delete` with `{criteria_ids:[...]}` → 204.

## UPDATE
```
PUT /rules/{criteria_id}        ← uses rule_criteria_id, NOT rule_id
Body:
{
  "conditions": {
    "payee": {"name":"ZZZZ_TEST", "match":"contain"},
    "on_plaid": true, "on_manual": true, "on_csv": true, "on_api": true,
    "priority": "5"
  },
  "actions": {
    "category_id": 12345,
    "description": "..."
  }
}
→ 200, returns updated rule object (rule_id may change but criteria_id is stable)
```

CLI note: `lunch-money-pp-cli rules update` fetches the existing rule first and
overlays only supplied flags before sending this full PUT body. This avoids
requiring callers to retype the payee/match/priority fields for action-only
edits.

## APPLY (dry-run / preview)
```
POST /rules/apply
Body:
{
  "criteria_ids": [12345],
  "dry_run": true,
  "include_transaction_ids": []
}
→ 200, array of transactions that would be affected (empty if no matches)
```
The web UI's "Apply Selected" button is the dry-run preview. To actually mutate transactions
the body likely needs `dry_run: false` — verify on a real apply before shipping.

### Batching caveat

Captured operationally on 2026-05-14: committing multiple `criteria_ids` in one
`POST /rules/apply` request can misroute the combined matching transaction set
through the last rule's action. The CLI wrapper must serialize multi-rule apply
operations into one backend request per `criteria_id`.

## DELETE (bulk)
```
POST /rules/bulk_delete
Body: {"criteria_ids":[12345, ...]}
→ 204 No Content
```

## Response object shape (partial)
```json
{
  "rule_id": 12345,
  "rule_criteria_id": 12345,
  "criteria_payee_name": "ZZZZ_TEST",
  "criteria_payee_name_match": "contain",
  "criteria_notes": null,
  "criteria_notes_match": null,
  "criteria_amount": null,
  "criteria_amount_2": null,
  "criteria_amount_currency": null,
  "criteria_amount_match": null,
  "criteria_day": null,
  "criteria_day_2": null,
  "criteria_day_match": null,
  "criteria_asset_id": null,
  "criteria_plaid_account_id": null,
  "criteria_priority": 5,
  "criteria_source": null,
  "criteria_suggested": false,
  "criteria_on_plaid": true,
  "criteria_on_csv": true,
  "criteria_on_manual": true,
  "criteria_on_api": true,
  ...
}
```
