Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Autom CLI build log

## Slice 1: foundation (done)
- internal/client/autom_hooks.go + 2 one-line edits in client.go: 401 "Rate limit exceeded" retried like 429 (key quotas excluded); ResponseObserver hook after each successful network response.
- internal/cli/helpers.go: one case in classifyAPIErrorOnly so a 401 rate limit or quota exits 7 with a hint instead of a bad-key auth error.
- internal/serp (engines, parsing, domains, overlap, diff, source merge, news keys) with table-driven tests.
- internal/store/extras.go (serp_snapshots, serp_results, news_seen) + autom_serp.go queries with tests.
- internal/cli/autom_common.go: credit ledger (JSONL in the data dir, fed by the client hook for every command), live search helpers, store helpers.

## Slice 2: absorbed commands (done)
- rank check (live, saves the page, previous position from the store), rank history (local), rank competitors (local, diff vs previous page), serp diff + serp list (local), ground (live fan-out, sources merged by canonical URL), batch run (file/stdin/--keywords, CSV columns, balance check on /v1/usage before spending, --budget cap, curtailed to 1 keyword under live dogfood), jobs get --wait (free polling).
- Root --fields restores Autom's server-side projection that the generator filters (client.ExtraParams hook, one line in doInternal). Parsing commands neutralize it with fullResponse().
- Manifest rows 19 and 28 reworded to resolvable command paths (no scope change): --fields is a root flag shown on google search; the MCP mirror lives in the autom-pp-mcp binary, row now points at agent-context.

## Slice 3: transcendence (done, 6/6)
- engines compare, geo compare (country codes checked with the free finder, --no-validate), rank movers, credits (live usage + local ledger, auto), visibility, news watch (dedupe by canonical URL or outlet+title, --peek).
- Partial failures: compare/ground keep fetch_failures in JSON, warn on stderr with the remaining denominator, and fail only when every source failed.

## Tests
- internal/serp: engines, params (Bing never sends page), parsing, domains, overlap, diff (identical pages report no drift), source merge, news keys.
- internal/store: snapshot round trip, cutoff lookup, news dedupe.
- internal/cli: movers (gain, exit, steady keyword silent, no-baseline note), visibility stats, serp diff (and unknown keyword = no changes + note), history exit 2 without domain, batch input parsing, ledger billing rules (200 billed, usage free, 202 pending), 401 quota vs bad key classification.

## Deliberate deviations
- Local novel commands do not call hintIfUnsynced/hintIfStale: the generated sync is a no-op for this API (every endpoint needs a query), so its hint ("run autom-pp-cli sync --resources brave,...") would mislead. They print their own hint naming rank check / batch run instead.
- dogfood WARN "defaultSyncResources empty": expected; the store is populated by rank check, batch run and compare --save.

## Deferred / limitations
- The ledger only sees calls made by this CLI. Async jobs are recorded as pending; Autom bills them on completion.
- Bing never receives `page` (Autom answers 500); news watch has no date filter (Google News dates are free text).

## Phase 4.8 review fixes and a generator incident
- 3 errors + 7 warnings from the SKILL review fixed at the source (research.json: novel descriptions now say which commands read saved pages, that credits only sees this CLI's calls, that news watch marks articles as seen with --peek, recipe costs, rank check and batch run recipes, no superlatives in the headline, group renamed "Rank tracking from saved pages"). SKILL and README command references now list the hand-written commands.
- Retro candidate (generator): `generate --force` on the same spec after a research.json narrative change did NOT preserve hand work, contrary to the hand-edit durability contract. It deleted every standalone hand-authored file (autom_common.go, rank_check.go, rank_history.go, serp_diff.go, batch_run.go, ground.go, jobs_get.go, autom_fields.go, autom_commands_test.go), reset the six implemented novel files to TODO scaffolds, and reverted the one-line edits in client.go, helpers.go and extras.go. Recovered from a pre-regen backup; only the narrative-derived files (README, SKILL, root.go, which.go, mcp/tools.go, agent_context.go, manifests, agentcookie.toml, .goreleaser.yaml) were taken from the regen output.
- Retro candidate (template): SKILL "Output Delivery" lists three --deliver sinks while --help also lists spacefast:<name>.

## Phase 4.9 audit fixes (10 errors, 11 warnings)
- Code: AUTOM_NO_LOCAL_DATA=1 now also stops page saving (rank check, batch run, compare --save), news watch memory and the credit ledger, and never creates the store. promoted_stats.go example now passes --engine google (the API requires engine or path).
- Docs (README, SKILL, AGENTS): read-only statement; quickstart comment for doctor --dry-run; correct command names (bing, --is-async, auth status, job-results, web-search); real queries instead of example-value and <command>; offline claims now name the saved-pages commands; CRUD boilerplate (--idempotent, --stdin) removed; doctor described as not validating the key (usage does); no echo of the secret; four --deliver sinks; ground +1 credit with --news; TLS note moved to Auth Setup; SERP examples instead of sports examples; purge note on saved history and the kept ledger; SKILL Troubleshooting section with a "not useful for this API" note on sync/search/analytics/import/export.
- research.json narrative and manifest rows 10, 12, 13, 18, 26 reworded to match (no scope change; row 26 no longer claims a doctor key probe).
- Not fixed (generated, out of reach without a regen, which is unsafe here): root help line "search: Full-text search across synced data" and the sync help text pointing to search.

## Phase 4.85 / 4.95
- Output review: 2 warnings fixed (visibility null positions, credits meta.source local when the balance fails). Coverage partial without a key.
- Local code review: 3 rounds, converged; details in proofs/phase-4.95-findings.md.

## Phase 5 live dogfood, loop 1 (run by the user with their key)
- Full matrix: 152/154 passed. Failures: ground and visibility error_path (exit 0 on "__printing_press_invalid__"); hollow: news watch (treated as mutating, dry-run only).
- CLI fixes: domainArg validates every domain argument and flag (rank check/history, visibility and --vs, movers/compare --domain, batch rows) with exit 2; ground carries pp:no-error-path-probe (any text is a valid question); news watch and rank check carry pp:preview-happy-path with --peek / --no-save happy args so the matrix runs them for real without writing.
- Shipcheck after the fixes (no key): 7/7 PASS, 92/100.
- Printing Press note (retro): mcp:local-write makes the live runner treat a command as mutating and skip its real happy path; a local-only write could be exercised in the sandbox HOME.

## Phase 5 live dogfood, loop 2
- Full matrix after loop-1 fixes: 153/153 PASS, no hollow feature.
- Live outputs showed Google returns redirect links (google.com/goto?url=...&gsessionid=...) that change on every call, which would make serp diff, rank competitors, rank movers (URL mode) and ground dedupe treat every Google result as new. Fixed in internal/serp: Identity() compares redirect links by domain + title (Diff, MergeSources, NewsKeys), with a test. Bing answered 500 during the run (Autom-side, reported in fetch_failures as designed).
- Source changed after the passing marker, so the live matrix must run once more to mint a marker for the final tree.
