Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.

Manifest transcendence rows: 6 planned, 6 built.

## Built
- P0/P1 (generator): 5 API verbs promoted to top-level — find, near, show, reviews, photos — + framework search/sql/sync/doctor over local SQLite. Auth: api_key in query param `key` (TRIPADVISOR_API_KEY).
- P2 (hand-coded transcendence, internal/cli/*.go as hand-authored files):
  - best <query> — search + bounded detail fan-out + rank by rating/reviews/ranking
  - compare <id> <id>... — 2-5 IDs side by side (subratings, trip-type mix), partial-failure accounting
  - nearby-best <lat,long> — near + bounded fan-out + min-rating filter + rank
  - drift <id> — fresh fetch vs stored snapshot (internal/store/tripadvisor_snapshots.go: rating_snapshots table), flags drops
  - digest <id> — details + reviews(UGC-labeled) + photos in one payload
  - fit <query> --traveler — rank by trip_type share for a traveler profile
- Shared helpers: internal/cli/tripadvisor_novel.go (parse/search/bounded fan-out/sort/emit). Real table-driven tests: internal/cli/tripadvisor_novel_test.go.

## Design notes
- `search` is a reserved framework command (offline FTS); live API search exposed as `find`.
- All fan-out commands have --max-scan caps (metered API, 5k/mo free) + IsDogfoodEnv curtailment to 3.
- All novel commands mcp:read-only, agent-native (--json/--select/--agent), reviews labeled UGC.
- Verify-friendly RunE: help-only / dry-run / required-input branches; missing-arg returns exit 2.

## Deferred
- None. All 6 approved transcendence rows shipped.
