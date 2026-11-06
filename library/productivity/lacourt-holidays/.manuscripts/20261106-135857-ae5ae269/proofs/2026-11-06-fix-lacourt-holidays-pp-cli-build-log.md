Manifest transcendence rows: 7 planned (6 + user-added all-years ICS, built into `ics`), 7 built. Phase 3 will not pass until all 7 ship.

# Build log

## Built
- internal/courtdays: pure court-day logic (ParseHolidays, Check, AddCourtDays, CalendarDeadline, CourtDaysBetween, NextHolidays, ICS) with table tests incl. year crossing, round trip add/between, unpublished-year guard.
- internal/cli/lasc_calendar.go: lazy per-year loader through the generated client (cache/timeout/verify mock), date parsing (ISO, MM/DD/YYYY, today/tomorrow), --back sign flip, unpublished-year -> exit 3, published-year discovery (walks back up to 40 yrs and forward up to 15).
- Commands: is-court-day, add-court-days (--back), deadline (--back), court-days-between, next-holiday (--count), ics ([year], --out-dir per-year files, all published years by default).
- Client patch: unwrapJSONStringBody (API double-encodes the array when Accept: application/json). Store patch: holidays keyed by date. Recorded in .printing-press-patches/.
- govulncheck fix: brotli 1.2.7, klauspost/compress 1.20.2.

## Findings
- API publishes 2006-2030 (page tabs show only 2026-2030). Docs/spec corrected.
- Negative positional N is parsed by pflag as a shorthand flag; replaced with --back.

## Deferred / limitations
- No `sync` resource: spec response schema is a string, so the generator emitted no syncable resource; commands are live with the client response cache. Offline-only use is not supported.
- Dogfood warnings (generator-owned): dead flag maxAge; dead helpers endpointPositionalArgs, retainCLIQueryParams.
- No statute tables (CCP 1005/1013) by design.
