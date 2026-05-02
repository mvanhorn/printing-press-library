# Shipcheck Results

Command:

```bash
./printing-press shipcheck \
  --dir library/commerce/shopify \
  --spec catalog/specs/shopify-2026-04-wrapper.yaml \
  --json \
  --no-live-check \
  --no-fix
```

Result: PASS

| Leg | Result |
| --- | --- |
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS |

The first shipcheck attempt in the sandbox failed before validation because dogfood could not write `dogfood-results.json` into the library repo. The same command passed after rerunning with filesystem permission.
