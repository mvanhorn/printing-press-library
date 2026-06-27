Manifest transcendence rows: 6 planned, 6 built. All shipping-scope.

Built (hand-authored, regen-durable — generated headers removed):
- millard: Millard County sweep across 10 towns, dedup, land-use body filter (--all, --days, --limit)
- landuse: body OR agenda land-use filter with reason (--location optional -> millard sweep, --days, --limit)
- since: local-store new-since diff (pmn_seen_notices table; --peek, --landuse, --db, --days)
- watch: parent with add/remove/list/check (pmn_watch_bodies table)
- agenda scan <term>: live agenda/title keyword scan with context snippet
- locations: curated Millard County town/ZIP registry

Shared: internal/cli/pmn_helpers.go (notice DTO, fetcher, sweep+dedup, matchers, registry),
internal/cli/pmn_store.go (seen + watchlist tables).

Surfaces: getUpcomingNotices.json (auth-free) is the workhorse for all novel commands.
Generated endpoint commands: notices (upcoming), notice (detail HTML).

Deferred / known gap: searchresult.html power-search is server-side flaky; not used.
