# SEEK CLI — Phase 3 Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

Run: 20260908-143250-21abe609 · binary v4.32.0 · single-session build.

## Priority 0 — foundation

- Generator emitted `internal/store/store.go` with a typed `listings` table
  (id, data, synced_at + 14 columns) and a `me` table, plus `sync_state`.
- **No framework `sync` / `search` / `analytics` commands.** SEEK is a
  search-only API — `/api/jobsearch/v5/search` is a filtered slice, not a
  full-resource collection — so v4.32's profiler (#4524 "do not silently sync a
  default-filtered slice") correctly excludes it from default sync. The store
  table still exists; the novel commands populate it themselves via
  `store.UpsertListings` after every live fetch.
- Shared helper `internal/cli/seek_novel.go` (hand-authored, not generator-owned):
  `scanSearch` (bounded pagination over the search endpoint), `cacheJobs`
  (upsert to `listings`), `fetchJobDetails` / `fetchSavedSearches` (GraphQL
  `POST /graphql`), `knownJobIDs` (anti-join set from the local table), and
  `seekJob` field accessors (classification / region / work type / arrangement).
- Pure-logic package `internal/seekparse/` (+ `seekparse_test.go`, 4 table-driven
  tests, all passing): `ParseSalary` (annualises `$/hr`, `$/day`, `$/mo`, `k`/`m`
  suffixes; domain clamp 15k–2M), `Percentile`, `Histogram`,
  `ParseSavedSearchQuery` (handles both the `?keywords=…` and SEO-path shapes
  SEEK has shipped for saved-search `query`).

## Priority 1 — absorbed (generator-emitted endpoint commands)

| Command | Endpoint | Status |
|---|---|---|
| `listings search` | `GET /api/jobsearch/v5/search` | emitted, live-verified |
| `listings get <id>` | `POST /graphql` jobDetails | emitted |
| `listings count` | `POST /graphql` JobCountV7 | emitted |
| `me saved-searches` | `POST /graphql` viewer.apacSavedSearches (cookie) | emitted |
| `me saved-jobs` | `POST /graphql` viewer.savedJobs (cookie) | emitted |
| `me job-status` | `POST /graphql` viewer.searchSaved/AppliedJobs (cookie) | emitted |

Absorbed behaviour rows (structured salary columns, delta tracking via
`listings` upsert + `first-seen` history, compact/agent output, AU+NZ via
`--site`, CSV/JSON) ride on the generated `listings search` command and the
novel commands' shared caching path.

## Priority 2 — transcendence (hand-built, all 6)

| # | Command | Data source | Notes |
|---|---|---|---|
| 1 | `salary [role] --where --site --classification --job <id>` | `live` | Scans up to `--max-scan-pages` (8) of search results, annualises salary text, computes p10-p90 + histogram + disclosure rate, caches listings. `--job` ranks one listing. **Live-verified**: RN/Melbourne → 1409 matches, 200 sampled, 45% disclosed, p50 $124k. |
| 2 | `trends --by classification\|subclassification\|region\|worktype\|workarrangement --since --granularity` | `local` | Reads the cached `listings` table, buckets by the job's own `listingDate` (month/week), period-over-period deltas. Missing-mirror hint when the store is empty. **Live-verified** after salary/company populated the store. |
| 3 | `company <advertiser> --site --active --limit` | `live` | Scans search (keyword = name, or broad when a numeric ID is given), filters by advertiser, attaches GraphQL `companyProfile` from the first opening. **Live-verified**: "Woolworths" → advertiser 23240035, 126 openings. |
| 4 | `listings facets --keywords --where --group-by --max-scan-pages` | `live` | Tallies the scanned result pages by classification / subclass / region / worktype / workarrangement / salary-band / advertiser. `facets` in the raw API response is empty, so this samples `data[]` and notes the sample size. **Live-verified**: "software engineer"/Sydney → ICT 182, Engineering 10. |
| 5 | `classifications [term] --sub` | `computed` | Curated `// pp:novel-static-reference` — 28 top-level classifications + 251 subclassifications harvested from live search responses. Fuzzy resolve by name or ID. **Verified**: `classifications software` → subclass 6290 Engineering - Software under 6281 ICT. |
| 6 | `me new-jobs --since --search --max-scan-pages` | `live` (cookie auth) | `fetchSavedSearches` → parse each `query` → `scanSearch` → anti-join `knownJobIDs` → cache new → tag by search name. Returns `authErr` (exit 4) without a session. |

All 6 follow the verify-friendly RunE shape (help-only branch, `dryRunOK`
short-circuit before IO, `usageErr` for missing input). `--help` and
`--dry-run --json` verified for all 6. `mcp:read-only` set on the five reads;
`me new-jobs` left unhinted (writes to the local store). `cliutil.IsDogfoodEnv()`
caps `--max-scan-pages` on every scanning command.

## Intentionally deferred

- `--data-source` incompatibility rejection on the `local`/`live`/`computed`
  commands — not wired; dogfood did not flag it. Polish candidate.
- `me new-jobs` saved-search `query` parsing is best-effort (two known shapes);
  a shape SEEK has not shipped yet would cause that search to be skipped and
  listed under `skipped_searches`, not fail the run.
- Company reviews sub-shape is passed through as raw `companyProfile` JSON
  rather than typed — the sniff never captured the reviews payload.

## Generator limitations / WARN-level dogfood findings (not novel-code)

- `dead_flags: [maxAge]`, `dead_functions: [handleBinaryResponseDelivery,
  hasChangedLocalFlags, isDryRunResponseForClient]` — all generator-emitted,
  dead because there is no framework sync/store command. Polish/shipcheck territory.
- `sync uses generic Upsert only` — expected; no typed sync path exists.
- `config inconsistency` on the cookie-auth `config.go` — generator-emitted.

## Phase 3 Completion Gate

- Per-row Cobra resolution: all 6 resolve as `seek-pp-cli <leaf> [flags]`. ✅
- Deterministic backstop: `dogfood … novel_features_check` → `{planned: 6, found: 6}`,
  no `missing`, `skipped` absent. ✅
- Test presence: `internal/seekparse` ships `seekparse_test.go` with real
  assertions (4 test funcs). ✅
- `go build ./...`, `go vet ./...`, `go test ./...` all green.
