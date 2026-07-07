# pokerdb-pp-cli

Local-only PokerDB export explorer.

This CLI does not call an API. It reads CSV/JSON files supplied by the user,
normalizes common PokerDB/Hendon Mob-style columns, and supports fast terminal
search across players, events, and results.

## Usage

```bash
pokerdb-pp-cli players search negreanu --file ../examples/sample-results.csv
pokerdb-pp-cli results search triton --file ../examples/sample-results.csv --json
pokerdb-pp-cli import ../examples/sample-results.csv --out ./pokerdb.local.json
POKERDB_FILE=./pokerdb.local.json pokerdb-pp-cli players search canada
```

## Commands

- `players search <query>`
- `players list`
- `events search <query>`
- `events list`
- `results search <query>`
- `results list`
- `import <csv-or-json>`
- `schema`
- `doctor`
- `version`

## Data Contract

Accepted formats:

- CSV with a header row
- JSON array
- JSON object containing `rows`, `results`, `players`, or `events`

Common aliases are normalized, including `player`, `player_name`, `name`,
`event`, `tournament`, `country`, `earnings`, `rank`, `date`, `place`, `venue`,
and `source_url`.

## Compliance

This tool does not scrape `pokerdb.thehendonmob.com`, does not bypass
Cloudflare, and does not use credentials or API keys. Users provide their own
local data exports.
