# OpenSnow CLI Build Log

## Priority 0: Foundation
- SQLite store with WAL mode, FTS5 search (generated)
- Tables: resources, resources_fts, content, forecast, sync_state, favorites
- Config: TOML with access_token, base_url fields

## Priority 1: Absorbed Features (22 commands)
All generated from spec:
- `forecast get-by-point` — Any lng,lat point forecast
- `forecast get-detail` — Named location 5-day forecast
- `forecast get-snow-detail` — Day+night snow forecast
- `snow-report` — Resort snow report
- `daily-reads content get-daily-snow` — Daily Snow posts
- `sync` — Full data sync to SQLite
- `doctor` — Health check
- `auth` — Token management
- `profile` — Flag presets
- Plus: api, agent-context, import, which, workflow, feedback, version, completion

## Priority 2: Transcendence Features (10 commands, all hand-built)
1. `favorites` — Add/remove/list favorite locations (SQLite-backed)
2. `dashboard` — Parallel snow-report fetch for all favorites, summary table
3. `overnight` — Overnight snow forecast from semi-daily data
4. `compare` — Side-by-side multi-resort comparison
5. `powder-score` — Rate days 1-10 for powder quality (weighted formula)
6. `powder-rank` — Rank favorites by best upcoming powder day
7. `storm-track` — Find contiguous snow periods in hourly forecast
8. `diff` — Compare current vs cached snow report
9. `digest` — Daily Snow posts as clean terminal text
10. `history` — Historical snowfall from cached data

## Deferred
- Forecast accuracy tracker (requires season of data accumulation)
- Season snapshot (requires season of data)

## Generator Notes
- Filtered global query params api_key and v from commands (handled by client)
- All commands support --json, --compact, --agent, --select, --dry-run
