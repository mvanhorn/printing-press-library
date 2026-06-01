# Restaurant365 OData Live Validation

Validated against a private Restaurant365 OData tenant using local-only credentials. No credential values, tenant identifiers, restaurant names, row IDs, amounts, or row values were written to this package.

## Commands Run

- `doctor --agent`
  - Result: `$metadata` reachable, credentials verified, 19 metadata entity types detected.
- `list-views --agent`
  - Result: documented CLI views resolved with field counts and refresh patterns.
- `describe-view Location --agent`
  - Result: returned schema field names and OData types only.
- `sample --view Location --limit 5 --agent`
  - Result: returned row count and columns only, `values_redacted: true`.
- `sample --view SalesDetail --limit 5 --filter "date ge 2026-05-01T00:00:00Z and date le 2026-05-01T23:59:59Z" --agent`
  - Result: returned row count and columns only, `values_redacted: true`.
- `deleted-records --entity TransactionDetail --since-row-version 0 --limit 5 --agent`
  - Result: returned counts by entity only, `values_redacted: true`.

## Notes

- Restaurant365 date filters were validated with DateTimeOffset boundaries, not bare `YYYY-MM-DD` values.
- Metadata lookup was verified against live casing differences such as `GlAccount` and `PosEmployee`.
- The contribution uses documented Restaurant365 OData views only. It does not include custom customer API endpoints or client pipeline code.
