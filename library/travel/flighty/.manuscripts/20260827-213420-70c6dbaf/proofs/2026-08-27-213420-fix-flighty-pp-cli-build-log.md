Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Flighty CLI Build Log

## What was built

### Foundation (Priority 0)
- `internal/rsc/` — Next.js RSC flight-chunk extractor (`ExtractChunks`, anchored `FindObject`/`FindArray` balanced scanners honoring string state). 8 table-driven tests.
- `internal/cli/flighty_extract.go` — kind-based extraction: catalog (regions flattened, RSC `$f:...:All:airports:N` references resolved to inline rows, region attribution, IATA dedup), detail (props + today-performance merge), board (`initialFlights[]`).
- `internal/cli/flighty_catalog.go` — store-or-live catalog with --data-source strategy, identifier resolution (IATA/slug/name/city/K-ICAO), haversine.
- `internal/cli/flighty_models.go` — typed board-flight models, board/detail fetch helpers, sync-hint-wired local catalog reader.
- `internal/store/flighty_migrations.go` — `airport_snapshots` history table (sync records one row per airport per run).
- sync.go hook — records post-sync catalog snapshots for `airports diff`.

### Absorbed features (Priority 1) — generated endpoint commands rewired to RSC extraction
- `airports list` (+ `--region`, `--status` filters) — live-verified: 155 unique airports.
- `airports tv` — same catalog payload from the TV dashboard.
- `airports show <iata|slug|name|city>` — slug resolution + detail extraction (weather incl. raw METAR, today performance). Live-verified with raw METAR output.
- `airports arrivals|departures` — slug resolution + board extraction. Live-verified: 25 flights per board with gates/belts/status.
- Write-through resource scoping fixed: tv→`airports-tv`, show→`airport_detail`, boards→`flight` so board/detail rows cannot pollute the catalog store.
- sync of `airports` extracts via RSC (157 records synced live).

### Transcendence features (Priority 2) — all 7 hand-coded, all live-verified
1. `airports worst --region --status --limit` (local) — ranks by cumulativeDelay then canceled%. Live: Manchester tops (4385m, MAJOR_ISSUES).
2. `airports find-flight <airport> <flight#>` (auto) — parallel fan-out to both boards, airline-prefix aware (`UA5072`/`5072`), partial-failure accounting. Live: UA2381 → "2h 45m Late", 09:45→12:30, Gate B10.
3. `airports airline <code> --top --region` (auto) — scans top-N disrupted airports, aggregates one airline's footprint with fetch-failure accounting. Live: UA at 5 airports → 49 delayed/2 canceled of 51.
4. `airports compare <A> <B>` (auto) — parallel detail fetches, side-by-side JSON + human table. Live: sfo/lax.
5. `airports route <origin> <dest>` (auto) — both directions of route disruption. Live: MAN→CDG forward 86% delayed (6/7), reverse found too.
6. `airports nearby <iata> --healthy-only --limit --max-km` (local) — haversine ranking. Live: SFO→LAX 544km etc. Zero-match note names the widening flag (catalog is sparse: nearest airport to DEN is 628km).
7. `airports diff` (local) — snapshot history diff: status transitions, added/cleared warnings, delay deltas. Live: 2-sync run surfaced ADD NORMAL→MINOR_ISSUES plus deltas across the network.

### Priority 3 polish
- Fixed `today.today` double-nesting in detail extraction.
- Fixed typed-nil interface gotcha in route note condition.
- Zero-match JSON notes on nearby (flag widening guidance).
- Default `--max-km` raised to 1000 (156-airport catalog is geographically sparse; nearest tracked airport to SFO is 544km).
- Behavioral tests: ranking, nearby distance/health/absence, flight-number split, snapshot diff (incl. identical-snapshot zero changes), RSC catalog reference resolution, detail merge, board extraction — all passing (`go test -count=1`).

## Intentionally deferred
- None.

## Skipped body fields
- None.

## Generator limitations found
- The generic `embedded-json` HTML extractor assumes `__NEXT_DATA__`/plain script JSON and cannot parse Next.js App Router RSC flight chunks (`self.__next_f.push`); a per-CLI extractor was required. (Retro candidate.)
- Endpoint commands for one spec resource share the resource's write-through type; hand-scoping per endpoint was needed to avoid store pollution. (Retro candidate.)
