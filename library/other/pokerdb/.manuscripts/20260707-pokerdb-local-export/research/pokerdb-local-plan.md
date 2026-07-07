# PokerDB Local CLI Plan

## Goal

Create a local-only command line tool for exploring user-supplied PokerDB/Hendon Mob-style CSV and JSON exports without using an API, scraping the website, or bypassing access controls.

## Constraints

- No PokerDB or Hendon Mob API is used.
- No network calls are made by the CLI.
- No scraping, browser automation, Cloudflare bypass, or live-site mirroring is included.
- The only data source is a local CSV or JSON file supplied by the user through `--file`, `POKERDB_FILE`, or `import`.

## CLI Surface

- `players search <query>` and `players list` inspect rows with player names.
- `events search <query>` and `events list` inspect rows with event names.
- `results search <query>` and `results list` inspect rows that contain both player and event fields.
- `import <csv-or-json> --out <path>` normalizes a local export into compact JSON.
- `schema` documents accepted formats and field aliases.
- `doctor` reports local-only mode, selected data file, and disabled network/API status.

## Data Handling

The importer accepts CSV files with headers or JSON arrays/objects with `rows`, `results`, `players`, or `events` arrays. Common aliases are normalized into portable fields such as `player`, `country`, `earnings`, `event`, `date`, `place`, `venue`, and `source_url`. Unknown source fields remain in the in-memory raw row for loss-aware processing.

## Safety

`import` requires an explicit `--out` destination and writes through a temporary file in the target directory before renaming it into place. This avoids truncating an existing local cache when the input cannot be loaded or the normalized export cannot be written completely.
