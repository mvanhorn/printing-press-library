# InfoTBM CLI Build Log

Manifest transcendence rows: 8 planned, 0 built. Phase 3 will not pass until all 8 ship.

## Priority 0: Foundation
- Data layer: Generated store with routes, stops, agencies, alerts, fares synced to SQLite
- Sync path: 5 resources (agencies, alerts, fares, routes, stops) sync from Mecatran REST API
- FTS search: Generated resources_fts with porter tokenizer

## Priority 1: Absorbed features (generated)
- All 13 absorbed features generated as promoted commands or API endpoints
- realtime stop, realtime vehicles, siri endpoints, routes, stops, alerts, etc.

## Phase 3 Transcendence Build
- schedule diff: pending
- trips last-departure: pending
- trips reroute: pending
- alerts impact: pending
- lines stops: pending
- schedule changes: pending
- trips plan: pending
- lines frequency: pending
