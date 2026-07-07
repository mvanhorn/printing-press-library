---
name: pp-pokerdb
description: Search and inspect local PokerDB/Hendon Mob-style CSV or JSON exports without using an API.
---

# pp-pokerdb

Use `pokerdb-pp-cli` when the user wants to search local PokerDB-style exports.
This skill is local-only: it does not call Hendon Mob, PokerDB, or any network
API.

## Examples

```bash
pokerdb-pp-cli players search negreanu --file ./pokerdb.local.json --json
pokerdb-pp-cli events search wsop --file ./results.csv --year 2025
pokerdb-pp-cli results list --file ./results.csv --country Canada --limit 50
pokerdb-pp-cli import ./results.csv --out ./pokerdb.local.json
```

## Notes

- Set `--file` or `POKERDB_FILE`.
- There is no API key.
- If the user asks for live Hendon Mob data, explain that this CLI requires a
  user-provided local export.
