# Luma CLI — Phase 3 Build Log

Manifest transcendence rows: 5 planned, 5 built. Phase 3 passes — all 5 ship.

## Built
### Generator-emitted (Priority 0/1)
- Data layer: SQLite store + sync (events/categories/calendars/discover) + framework `search` (FTS), `analytics`, `tail`, `doctor`, `import`, `profile`, `workflow`.
- Endpoint commands: `discover home`, `discover categories`, `places get`, `places calendars`, `places map`, `events list` (`--city`/`--place-id`/`--category`), `events get`, `calendars` (get).

### Hand-coded novel (Priority 2) — all 5
| Command | Data source | Status | Live-verified |
|---|---|---|---|
| `agenda` | live (fan-out) | built | ✓ merges city+category, dedupe, window, exit-2 no-filter |
| `near` | local (store + haversine) | built | ✓ 25 events ≤3km of SF center, nearest 0.3km |
| `ics` | live | built | ✓ valid RFC5545 VCALENDAR, escaped LOCATION |
| `watch` | live + local history | built | ✓ baseline capture → "no changes" diff |
| `calendars compare` | live (fan-out) | built | ✓ ranked 3 calendars by upcoming events; exit-2 <2 args |

Shared helpers in `internal/cli/luma_events.go` (parse, haversine, window, dedupe, ICS, fan-out fetch). Pure-logic tests in `luma_events_test.go` (pass).

## Critical fix: event ID extraction
The generator's generic ID-fallback list (`id/ID/uuid/slug/name`, plus `gid/sid/...`) does **not** include `api_id`, which Luma uses universally. Result: sync stored 0 events ("all_items_failed_id_extraction"), breaking the local store (search/near/watch).

Fix: separate hand-authored `init()` files populate the mutable `resourceIDFieldOverrides` map in both `internal/store/luma_id_overrides.go` and `internal/cli/luma_id_overrides.go` (events/categories/calendars/discover → `api_id`). This is the skill's sanctioned durable-extension pattern (whole files survive regen-merge; no edits to generated code). After the fix sync stores 50 events and `search "summit"` returns local FTS results.

**Retro candidate (generator gap):** the ID-fallback heuristic should include `api_id`. Many reverse-engineered APIs (Luma, others) use `api_id` as the canonical identifier. Without it, every sniffed/internal-spec CLI for such APIs silently stores nothing.

## Intentionally deferred / notes
- No public free-text search endpoint on Luma → framework `search` is local-FTS-over-synced only (the headline value-add). Documented in SKILL/README.
- `places get` accepts `--city` (slug) or `--place-id`; slug resolution confirmed.
- Quirk preserved: event uses `event_api_id`, calendar uses `api_id`.
- No stubs.
