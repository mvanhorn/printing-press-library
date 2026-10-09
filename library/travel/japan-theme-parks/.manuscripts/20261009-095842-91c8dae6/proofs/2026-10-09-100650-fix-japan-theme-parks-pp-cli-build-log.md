Manifest transcendence rows: 4 planned, 4 built. (dates, typical, snapshot, waits — order folded into waits per the absorb gate.)

# Build log: japan-theme-parks-pp-cli (2026-10-09)

## Built
- `parks` (computed): 5 Japan parks; JA/EN names, Queue-Times id, timezone, official URL, crowd-calendar link (link only), coverage block (hours/tickets source or unknown reason).
- `waits` (live, Queue-Times): absorbed rows 2-4 and transcendence 4. `--open-only`, `--max-wait`, `--sort wait|delta|name`, `--limit`, `--min-samples`, `--db`, `--no-history`. wait_minutes null when closed. Typical/delta/n only when local history has samples for the current weekday x hour (Asia/Tokyo); otherwise history_note. Credit "Powered by Queue-Times.com" + link in meta and human output.
- `dates` (live): absorbed rows 5-7 and transcendence 1. TDR JA+EN ticket calendar month pages (Chrome TLS via surf), per-ticket status (on_sale, few_left, sold_out, not_yet_on_sale, no_online_sale, unknown), legend, adult/junior/child yen, official hours. Unpublished month = explicit unknown + rule-derived sale_opens_at (observed=false, basis text). USJ/Fuji-Q hours from api.themeparks.wiki labelled third-party with fetched_at and credit; null + "beyond third-party source horizon" past the schedule end. USJ dated price/Express Pass, Fuji-Q tickets, Legoland hours/tickets = null with reason. Structure change = ErrTDRStructure -> apiErr with coverage reason. Range max 92 days; past dates rejected.
- `snapshot` (live -> local SQLite `wait_snapshots`): INSERT OR IGNORE on (ride_id, source_updated_at); per-park counts; partial failures in fetch_failures + stderr warning; all-fail -> typed error. No scheduler installed; README gives a cron example only.
- `typical` (local): median/p75 per ride x weekday x hour (Asia/Tokyo), n, closed count, distinct days, first/last date, "insufficient" under --min-samples (default 6). Empty store = exit 0 with note.
- Sibling clients: `internal/sources` (AdaptiveLimiter per source, 429 -> RateLimitError, one retry on 5xx/network, 8 MiB cap). Parsers/stats: `internal/parks` with table-driven tests (fixtures trimmed from discovery). Store: `internal/store/japan_theme_parks_migrations.go` with dedupe tests.

## Intentionally deferred / not built
- `tickets`, `hours`, `order`: cut at the absorb gate (covered by dates/waits).
- No `sync`: the generator emitted none for this spec; `snapshot` owns the local store.
- USJ mobile-service Hours endpoint: removed (401, needs OAuth client token). Never use client credentials from USJ code.
- USJ WEB ticket store: waiting room + robot check; not read.
- Queue-Times crowd calendar: link only, not scraped.

## Skipped body fields
- None (all read-only GET).

## Generator limitations found
- go directive 1.26.6 fails govulncheck (stdlib GO-2026-66xx); fixed with `go 1.26.9`.
- surf v1.0.202+ needs go 1.27; pinned surf v1.0.201 + enetx/http v1.0.29.
- Spec transport is `standard`; TDR Chrome-TLS client is hand-written in `internal/sources`.
