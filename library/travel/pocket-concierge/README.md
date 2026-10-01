# Pocket Concierge CLI

Public, read-only restaurant discovery, course and date/party availability, and canonical booking handoff from Pocket Concierge. No account or paid key required for discovery. Only the provider's English and Japanese variants are used.

## Quick Start

Go 1.26.6 or newer. Build standalone; no browser or database service required.

```sh
go build -o bin/pocket-concierge-pp-cli ./cmd/pocket-concierge-pp-cli
bin/pocket-concierge-pp-cli doctor --dry-run
bin/pocket-concierge-pp-cli filters
bin/pocket-concierge-pp-cli restaurants search --query Murase --limit 5
bin/pocket-concierge-pp-cli restaurants get --id 245672
bin/pocket-concierge-pp-cli courses list --id 245672
bin/pocket-concierge-pp-cli availability dates --id 245672 --limit 10
```

## Agent Usage

Every data command emits compact JSON by default; `--json`, `--agent` and `--compact` select the same output. Diagnostics and JSON errors go to stderr. No prompts. `agent-context` routes tasks; `schema` explains fields and exits. `--dry-run` performs no HTTP/cache access and describes the fixed operation. No arbitrary GraphQL or provider writes are exposed.

```sh
bin/pocket-concierge-pp-cli restaurants search --query sushi --limit 5 --agent --select items.id,items.name,items.name_ja,items.url
```

Projection uses dotted fields and retains `meta` with freshness, request count, latency, response bytes and partial coverage. Missing facts are null; successful empty collections are `[]`. Source IDs are strings. English results include first-party Japanese names joined by ID, usually costing a second request. `--lang ja` uses Japanese content directly.

## Cookbook

Get source area/cuisine IDs from `filters`; geographic aliases are not guessed. Prices are JPY per guest. Search returns summaries; detail and policies are retrieved only when requested.

```sh
bin/pocket-concierge-pp-cli restaurants search --area-id 19 --cuisine-id 1 --service DINNER --max-price 30000 --limit 5
bin/pocket-concierge-pp-cli restaurants search --query sushi --date 2026-10-05 --party 2 --instant --pages 1
bin/pocket-concierge-pp-cli availability slots --id 245672 --date 2026-10-05 --party 2
bin/pocket-concierge-pp-cli booking handoff --id 245672 --course-id 182402
```

Dates in examples are illustrative inputs and may become unavailable. Choose an actual current date from `availability dates`. A handoff with a source session additionally requires its date:

```sh
bin/pocket-concierge-pp-cli booking handoff --id 245672 --course-id 182402 --session-id 6871760 --date 2026-10-05 --party 2
```

The handoff verifies public course/session ownership and party bounds when known, but returns only the canonical restaurant page. Selection is not prepopulated. Recheck and complete any booking yourself on Pocket Concierge.

## Source semantics and limits

- `realTimeBooking=true` means an instant-confirmation option; false means a reservation request. Neither means a reservation exists. Waitlists are separate and have a null source session ID.
- Calendars are restaurant-level date signals, not party suitability. Sessions include course identity, start/end offset timestamps, seating type and min/max party bounds. Unknown bounds remain null and mark partial coverage.
- Course `price.per_guest` and `price.fixed_per_group` are separate JPY values. `all_in_total` is always null. Exact fee statements are retained from course text and restaurant policies, including contradictions. Check both before booking.
- Dietary, child and language conditions contain only verbatim source evidence. English content does not guarantee English staff. No accommodation or age eligibility is inferred.
- Coverage is the provider's public listings, not all Japan restaurants or account-exclusive inventory. Undocumented first-party GraphQL may change; HTTP/schema/partial GraphQL errors fail explicitly.

## Cache, bounds and freshness

Catalog/search/detail cache: 15 minutes; filters: 24 hours. Date-filtered search, calendars and sessions are fresh by default. `--cache-availability` on dates/slots permits 30 seconds of cached inventory. Inspect every `meta.observations[].fetched_at` and `cache_hit`; `oldest_fetched_at` is the oldest component.

`--refresh` explicitly bypasses cache reads and replaces queried entries. `--no-cache` bypasses reads and writes. `--cache-dir` controls location (default OS user cache under `pocket-concierge-pp-cli`). Cache is capped at 128 entries / 16 MiB per-process pruning. No inventory-wide automatic refresh or hidden crawl.

Search defaults: one page, ten results; caps: three pages, 50 per page. Follow `pagination.next_page`. Course/calendar/session lists use `--offset`, `--limit`, and `next_offset`. Calendar default 100 entries; sessions 50. Truncated pages mark `meta.partial`. Snapshot pagination can move between invocations.

Requests are serial at at most two per second. One retry for 429 (only short Retry-After) or 5xx; 2 MiB maximum response. Whole-command default deadline 30s, adjustable 1s–120s; each request also has a 15s ceiling. No resident browser, SQLite, authentication or shared configuration.

## Health Check

```sh
bin/pocket-concierge-pp-cli doctor --refresh
bin/pocket-concierge-pp-cli version
```

Doctor verifies useful public filter JSON, not merely HTTP status. For a fresh source probe use `--refresh` or `--no-cache`.

## Troubleshooting

Exit codes: 0 success; 1 internal; 2 invalid input/selection; 3 restaurant/course/session not found; 4 access denied; 5 exhausted rate limit; 6 network/deadline; 7 HTTP/schema/GraphQL contract failure. Empty results are success, not a network failure.

For code 5 retry later. For code 6 check connectivity or increase `--timeout` within the cap. For code 7 try `--refresh`; if persistent, inspect provider changes before relying on output. A stale session ID can disappear; refresh dates/slots and choose a currently returned ID. Unknown flags, invalid dates, negative prices, and missing required IDs fail without source requests.

## Validation

Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./...`. Build the binary before `python3 tools/live_e2e.py`; the live runner uses an isolated provider cache. See the bundled manuscripts for generation and publish-time live evidence.

Created by [@zjsng-trav](https://github.com/zjsng-trav) (zjsng). Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) v4.32.5; source semantics and domain commands were curated during the recorded build.
