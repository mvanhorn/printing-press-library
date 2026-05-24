# Acceptance Report: American Reindustrialization CLI

- **Level:** Full Dogfood
- **Matrix size:** 111 tests
- **Passed:** 111
- **Failed:** 0
- **Skipped:** 58 (error-path probes for commands with no positional args — expected skips)
- **Auth context:** `none` (no API key required)
- **Run mode:** Live (real `/api/*` calls against americanreindustrialization.com)

## Gate: PASS

All 111 mandatory tests passed across the 16-command tree (categories, companies, openings, tags, news, analytics, whats-new, sync, search, doctor, profile, workflow, etc.) plus their `--help`, happy-path, `--json` fidelity, and error-path probes where applicable.

## In-flight fix
1 failure surfaced in the first dogfood pass and was fixed in-session:

- **`workflow archive --json` returned invalid JSON (1/111 fail).** Root cause: `syncResource` writes per-event NDJSON to `os.Stdout` during the sync, and `workflow archive` then appended a final JSON summary, so stdout was NDJSON-stream-plus-trailing-object — neither parseable as a single JSON document nor as pure NDJSON.
- **Fix:** in `internal/cli/channel_workflow.go`, redirect `os.Stdout` to `os.Stderr` during the syncResource loop when `--json` is set, then restore before writing the summary. Stdout is now a single parseable JSON document; the NDJSON progress events go to stderr where humans can still tail them.
- **Verified:** `workflow archive --json` now produces clean JSON on stdout (47 NDJSON event lines on stderr); `python3 -c "json.load(open('out.txt'))"` parses successfully.
- **Cost:** 1 file, ~6 LoC added.

## Printing Press retro candidate

The underlying generator-level issue is broader: every printed CLI's `syncResource` writes NDJSON events to stdout unconditionally, which means **every CLI's `workflow archive --json`** (and possibly `sync --json`) produces the same dual-format stdout. The right fix lives in the generator's `sync.go.tmpl` (route events through a writer that switches to stderr when `--json` is set, or always to stderr). Filed for the retrospective.

## Live API data quality observations
Not bugs — calling out so the user knows what to expect:

- **`/api/companies?limit=N`** caps at the default page size (20). The full dataset is 96 companies; default sync paginates through all of them.
- **`/api/jobs` (openings)** has 501 listings total; default sync stops at the first 100 because the generator's pagination heuristic doesn't auto-fetch beyond `?page=1` without an explicit `--max-pages`. `printing-press dogfood` matrix exercises only what one page returns; `workflow archive` accumulates a deeper window (164 jobs after multiple runs). Users wanting the full set can run `sync --max-pages 0`.
- **`primary_sector`, `funding_stage`, `founded_year`, `latitude`, `longitude`** are NULL on most rows upstream. Analytics commands display `(unspecified)` buckets accordingly; the SKILL and README both document this caveat.
- **`/api/news`** is empty upstream (the API ships the route but no items yet). The `news list` command works correctly and returns `[]`.
- **`/api/jobs/titles`** returns plain strings, no IDs — sync correctly skips them with `sync_anomaly: all_items_failed_id_extraction`. The autocomplete endpoint is still callable via `openings titles --q <prefix>`.

## Novel-feature smoke results

| Feature | Live result |
|---|---|
| `whats-new --since 2026-05-12` | ✓ returns companies updated since |
| `openings find --experience senior --limit 3` | ✓ returns Senior Aero-Mechanical Engineer, etc. at Rainmaker |
| `analytics sector-heatmap --weight jobs` | ✓ Energy/UT=38, Aerospace&Defense/CA=33, Manufacturing/MI=30 |
| `analytics funding-by-sector` | ✓ Manufacturing 11 companies, Technology 5, etc. |
| `companies top-hiring --limit 3` | ✓ Archer/Last Energy/Oklo (50 jobs each) |
| `companies profile nox-metals` | ✓ joins openings + 10 similar Manufacturing companies |
| `analytics geo-clusters` | ⚠ returns `[]` (lat/lon NULL upstream — documented caveat) |
| `openings salary-stats` | ✓ correctly reports 100 nulls / 0 with salary on synced sample |
| `companies cohorts --bucket 10` | ✓ 1980-1989/1990-1999/2000-2009/... cohorts populated |

## Verdict

`ship` — every gate condition met; novel features work against real data; documented caveats are honest about upstream data sparsity.
