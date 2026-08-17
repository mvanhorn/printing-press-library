# FRED CLI Build Log

Manifest transcendence rows: 5 planned, 5 built.
- dashboard (hand-code) — curated macro snapshot, fans out 6 indicators
- series compare (hand-code) — N series aligned by date, partial-failure accounting
- series latest (hand-code) — most recent observation one-liner
- watchlist add/list/sync (hand-code) — JSON-backed local state, reports what changed
- release calendar (hand-code) — windowed release schedule, dogfood-curtailed

Generated: 27 endpoint commands across 6 resources (series, category, release, source, tags) + framework (sync, search, doctor, export, etc.).
Auth: api_key query param + file_type=json (FRED shape).
