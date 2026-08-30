# amazon-jobs-pp-cli Build Log

Manifest transcendence rows: 6 planned (sync spec-emits + 5 hand-code), 6 built. Phase 3 gate requires all 6 ship.

## What was built (hand-written, package `internal/cli`)
- find    — live search: keyword + location (server-side normalized_* filters) + client-side
            NULL-safe facet filters (--category/--schedule/--intern/--manager/--university),
            scan-and-filter with --max-scan-pages, --json/--select/--plain. (pp:data-source live)
- get     — fetch one job by id (or first keyword match), full detail, --plain HTML->text. (live)
- sync    — page /en/search.json, UpsertBatch into the typed `postings` store, --max-pages,
            --saved <name>, IsDogfoodEnv curtail to 1 page. (live)
- new     — diff a saved search's live current ids vs stored cursor; baseline on first run;
            advances cursor; keeps mirror fresh. (live)
- save    — persist a named search (query+filters) into saved_searches table. (local)
- searches— list / --delete saved searches. (local)
- stats   — GROUP BY city/state/country/team/category/schedule over synced store; true total. (local)
- skills  — keyword LIKE-scan of qualifications, ranked by team/city/category; true match total. (local)

Shared: internal/cli/amazonjobs_common.go (Job type, buildSearchValues with bracket wire keys,
searchPage, client-side filters, saved_searches CRUD via db.DB(), plainText HTML->text,
emitResult output helper, data-source guard, missing-mirror hint).
Tests: internal/cli/amazonjobs_common_test.go (buildSearchValues, filters, effectiveBool, boolFlag, cleanJob).

## Data-layer decision
The v4.29 generator emits the store (`internal/store` with typed `postings` table + FTS) but NO
framework sync/search/sql commands, and the promoted `postings search` uses the paginated read
path which ignores `response_path`. So the entire query/store surface is hand-written against
store.UpsertBatch("postings", ...) and store.DB() raw queries. The generated `postings search`
remains as the raw live escape hatch.

## Confirmed contract nuances handled
- result_limit forced >= 1 (0 + filter => false zero hits).
- Location filters use bracketed wire keys normalized_*[] (percent-encoded %5B%5D accepted).
- Nullable is_intern/is_manager/university_job => *bool, NULL-safe client filters.
- HTML <br/> in descriptions => plainText() converts to newlines for --plain/human output.

## Intentionally deferred / not built
- None. No stubs. Category/schedule/intern/manager filters are honest client-side (documented).
