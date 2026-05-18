# Bandsintown CLI — Absorb Manifest

## Absorbed (match or beat every feature that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get artist info by name (id, mbid, image, tracker_count, upcoming_event_count, FB page) | subdigital/intown, aroscoe/bandsintown, chrisforrette/python-bandsintown, @datafire/bandsintown | `artists get <name>` typed-by-spec, --json/--select/--csv/--compact | Agent-native, offline cache, retry/backoff, special-char escaping built-in |
| 2 | Get artist events — upcoming | All wrappers | `events list <artist> --date upcoming` | Local store, FTS, --json --select for piping |
| 3 | Get artist events — past | All wrappers | `events list <artist> --date past` | Same + historical retention |
| 4 | Get artist events — all | All wrappers | `events list <artist> --date all` | Same |
| 5 | Get artist events — date range YYYY-MM-DD,YYYY-MM-DD | bandsintown/api-gem (Ruby), @datafire | `events list <artist> --date 2026-01-01,2026-06-30` | Same |
| 6 | Print/format event with venue + offers + lineup | TappNetwork/php-sdk, node wrappers | Default human + --json structured | Typed offers table, typed venue lat/lng |
| 7 | Filter events by lat/lng locality | bandsintown-gig-map (Vue map) | `events list ... --near 'Jakarta,ID' --radius-km 500` | Local geocoding cache via venue table |
| 8 | URL-escape artist names with special chars (`/`→%252F, `?`→%253F, `*`→%252A, `"`→%27C) | bandsintown/api-gem | Built into client transport | Per-spec, handled automatically |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why only we can do this |
|---|---------|---------|-------|-------------------------|
| 1 | Tour-routing feasibility | `route --to <city> --on <date> --window <Nd> --tracked [--score]` | 9/10 | Cross-entity local join across events × venues × watchlist; haversine + date-window math; no API endpoint exists for this |
| 2 | Calendar-gap detection | `gaps <artist> --min <Nd> --max <Nd> [--in <region>]` | 8/10 | SQL window function (LEAD/LAG) over an artist's synced events; gap analysis is impossible from a stateless API call |
| 3 | Lineup co-bill mining | `lineup co-bill <artist> --since <date> --min-shared <N>` | 9/10 | Mines `lineup[]` arrays across many events into a junction table; aggregates co-bill counts; Bandsintown returns lineups one event at a time |
| 4 | Tracker trend over time | `snapshot` + `trend --top <N> --period <Nd>` | 9/10 | Turns the stateless `tracker_count` scalar into a time-series via local `artist_snapshots` table; LAG() computes deltas; Bandsintown has no history endpoint |
| 5 | Idempotent sync with diff | `sync --tracked [--since-stale <h>]` | 8/10 | Synthesizes a cursor from staleness windows; emits structured diff (added/removed/changed events); agent-shaped exit codes; no API cursor exists |
| 6 | SEA routing radar | `sea-radar --date <range> [--tier <t>]` | 8/10 | Composed query over events × watchlist × `tracker_count` percentile, scoped to a SEA bounding box; Aisha's Monday-morning briefing in one command |
| 7 | Watchlist primitive | `track add <names...>` / `track list` / `track remove <name>` | 7/10 | First-class `tracked_artists` table not present in the API; drives sync, snapshot, route, sea-radar |

## Notes

- All transcendence features will ship as real commands. No stubs.
- Live API testing in Phase 5 will be skipped per the Phase 0 reachability decision (partner-only access). Quick Check matrix will exercise help, --dry-run, --json, and offline store commands; live-API rows will report `unverified-needs-auth`.
- Phase 5 acceptance gate downgrades from `pass` to `unverified-needs-auth` for live rows — this is the documented path for partner-gated APIs.
