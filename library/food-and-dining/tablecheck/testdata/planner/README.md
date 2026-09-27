# Planner fixture provenance

`manifest.json` records the source sample, whether each fixture is live-derived or synthetic, its intended edge case, and its byte size. Live samples come from the fourteen explicitly authorized anonymous public endpoint probes in `../discovery/contract-probe-ledger.json`; preparing these fixtures made no network requests.

Only English/Japanese translation objects and relevant venue/course/availability fields are retained. Image fields are removed. Empty arrays, explicit nulls, missing fields, false flags, unknown statuses, decimal strings, source conditions and timezone data remain distinct. The real calendars are reduced to the contiguous six-day Sep 28–Oct 3 horizon; they retain actual timestamp objects and one closed date.

The synthetic files deliberately exercise cases that the bounded live observations did not contain. They must never be presented as actual restaurant inventory or as evidence that the upstream supports invented status enums. In particular, `vendor_future_status` checks conservative handling, while synthetic course `sold_out` and `unpublished` strings only check preserving explicit source values.

Planned deterministic checks use an injected local HTTP transport/server and a clock. They verify the public result contract through actual search, course, calendar, cache, scan and handoff methods; they do not depend on implementation-private helper names.

- Search checks explicit source query spellings, opaque pagination cursors, no speculative second-page request, decimal venue budgets, and discovery scope.
- Course checks preserve decimal strings and fine print, distinguish missing/null/empty rules, keep source stock/status, and never derive all-in totals or price basis from group-order metadata.
- Calendar checks distinguish false, missing, closed and source failures; convert UTC timestamps to the declared timezone; check exact requested time separately from alternatives; retain missing/failed scan days and venue scope.
- HTTP/cache checks count actual attempts, cap one transient retry and total requests, enforce two concurrent requests, propagate context cancellation, honor exact locale/party/date keys, use original freshness timestamps, and bypass/read/expire cache correctly. Failed inventory is not cached as an empty calendar.
- CLI checks required explicit arguments, invalid bounds and dates/times, compact JSON/selection, dry-run request plans without network or cache writes, and bounded scan behavior.
- Handoff checks both canonical booking modes and source prefill keys `start_date`, `start_time`, `num_people`; no cart, hold, booking or account mutation occurs.
